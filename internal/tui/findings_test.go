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

	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

func sampleFindings() rules.Findings {
	return rules.Findings{
		{RuleID: "DOC-CKPT-001", Severity: rules.SeverityHigh, Message: "checkpoints failing", Target: "job", Evidence: "3 failed", Fix: "investigate timeouts"},
		{RuleID: "DOC-BP-001", Severity: rules.SeverityMedium, Message: "backpressure on agg", Target: "job/op-agg"},
	}
}

func TestRenderFindingsView(t *testing.T) {
	out := renderFindingsView(DefaultTheme(), "orders", sampleFindings(), 0, 90)
	for _, want := range []string{"doctor", "orders", "2 finding(s)", "DOC-CKPT-001", "checkpoints failing", "detail", "3 failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("findings view missing %q:\n%s", want, out)
		}
	}
}

func TestRenderFindingsListEmpty(t *testing.T) {
	if out := DefaultTheme().renderFindingsList(nil, -1, 0); !strings.Contains(out, "no findings") {
		t.Errorf("empty list should say 'no findings', got %q", out)
	}
}

func TestFindingsModelNavAndQuit(t *testing.T) {
	m := NewFindings("orders", sampleFindings())

	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if down.(findingsModel).cursor != 1 {
		t.Errorf("down should move cursor to 1, got %d", down.(findingsModel).cursor)
	}
	// Clamps at the last finding.
	end, _ := down.Update(tea.KeyMsg{Type: tea.KeyDown})
	if end.(findingsModel).cursor != 1 {
		t.Errorf("cursor should clamp at last, got %d", end.(findingsModel).cursor)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should quit")
	}
}
