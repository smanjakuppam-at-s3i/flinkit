// Copyright 2026 S3I Groups LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
)

func sampleSnap() report.InspectSnapshot {
	return report.InspectSnapshot{
		JobID:        "j1",
		JobName:      "orders",
		State:        "RUNNING",
		FlinkVersion: "1.20.1",
		Operators: []report.InspectOperator{
			{ID: "op-agg", Name: "Window Agg", Parallelism: 4, BusyMsPerSec: 980, BackpressuredMsPerSec: 50},
		},
		Checkpoints: report.Checkpoints{Completed: 10, Failed: 2},
		Restarts:    1,
		IO: []report.IOEndpoint{
			{Operator: "op-agg", Direction: "source+sink", Resource: "Window Agg", RecordsIn: 100, RecordsOut: 60},
		},
		Bottleneck: &report.BottleneckReport{
			Found: true, OperatorID: "op-agg", OperatorName: "Window Agg",
			WorstSubtask: 7, Cause: "key-skew", Fix: "salt hot keys",
		},
		Logs: []report.LogLine{{Source: "jobmanager", Text: "checkpoint 119 failed"}},
	}
}

func loaded(snap report.InspectSnapshot) inspectModel {
	return inspectModel{theme: DefaultTheme(), snap: snap, loaded: true, width: 90, height: 30, history: map[string][]float64{}}
}

func TestInspectViewContent(t *testing.T) {
	out := loaded(sampleSnap()).View()
	for _, want := range []string{
		"orders", "1.20.1", "operators", "Window Agg", "i/o",
		"bottleneck", "key-skew", "salt hot keys", "logs", "checkpoint 119 failed",
		"selected:", "p pause",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}
}

func TestInspectViewStates(t *testing.T) {
	notLoaded := inspectModel{theme: DefaultTheme(), width: 90, history: map[string][]float64{}}
	if out := notLoaded.View(); !strings.Contains(out, "connecting") {
		t.Errorf("not-loaded should show connecting:\n%s", out)
	}
	errModel := inspectModel{theme: DefaultTheme(), loaded: true, err: errors.New("boom"), width: 90, history: map[string][]float64{}}
	if out := errModel.View(); !strings.Contains(out, "fetch error") {
		t.Errorf("error state should show fetch error:\n%s", out)
	}
	live := loaded(sampleSnap())
	live.live = true
	if out := live.View(); !strings.Contains(out, "live") {
		t.Errorf("live mode should show ⟳live:\n%s", out)
	}
	p := loaded(sampleSnap())
	p.paused = true
	if out := p.View(); !strings.Contains(out, "paused") {
		t.Errorf("paused mode should show the paused marker:\n%s", out)
	}
}

func TestInspectModelQuitKey(t *testing.T) {
	m := NewInspect(context.Background(),
		func(context.Context) (report.InspectSnapshot, error) { return sampleSnap(), nil },
		false, time.Second)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should quit the program")
	}
}

func TestInspectHistoryPauseAndSparkline(t *testing.T) {
	m := NewInspect(context.Background(), nil, true, time.Second)

	// Two snapshots accumulate a 2-point busy history for the operator.
	m1, _ := m.Update(inspectSnapMsg{snap: sampleSnap()})
	m2, _ := m1.Update(inspectSnapMsg{snap: sampleSnap()})
	im := m2.(inspectModel)
	if got := len(im.history["op-agg"]); got != 2 {
		t.Errorf("history len = %d, want 2", got)
	}
	// 98% busy renders a near-full block in the sparkline.
	if !strings.Contains(im.View(), "█") {
		t.Errorf("expected a sparkline block in the view:\n%s", im.View())
	}

	// p toggles pause.
	paused, _ := im.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if !paused.(inspectModel).paused {
		t.Error("p should pause")
	}
}

func TestInspectCursorClampsSingleOperator(t *testing.T) {
	im := loaded(sampleSnap()) // one operator
	down, _ := im.Update(tea.KeyMsg{Type: tea.KeyDown})
	if down.(inspectModel).cursor != 0 {
		t.Errorf("with one operator the cursor should stay at 0, got %d", down.(inspectModel).cursor)
	}
}
