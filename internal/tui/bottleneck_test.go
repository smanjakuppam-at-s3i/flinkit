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
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
)

func TestRenderBottleneckFound(t *testing.T) {
	rep := report.BottleneckReport{
		JobID: "j1", Found: true, OperatorID: "op-agg", OperatorName: "Window Agg",
		WorstSubtask: 7, BusyMsPerSec: 980, BackpressuredMsPerSec: 30,
		Cause: "key-skew", Summary: "work is concentrated on one subtask", Fix: "salt hot keys",
	}
	out := renderBottleneckView(DefaultTheme(), rep, 90)
	for _, want := range []string{"bottleneck", "j1", "choke point", "Window Agg", "subtask 7", "cause", "key-skew", "fix", "salt hot keys"} {
		if !strings.Contains(out, want) {
			t.Errorf("bottleneck view missing %q:\n%s", want, out)
		}
	}
}

func TestRenderBottleneckNotFound(t *testing.T) {
	out := renderBottleneckView(DefaultTheme(), report.BottleneckReport{JobID: "j1", Found: false}, 90)
	if !strings.Contains(out, "No bottleneck") {
		t.Errorf("not-found view should say so:\n%s", out)
	}
}

func TestBottleneckModelQuit(t *testing.T) {
	m := NewBottleneck(report.BottleneckReport{JobID: "j1"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should quit")
	}
}
