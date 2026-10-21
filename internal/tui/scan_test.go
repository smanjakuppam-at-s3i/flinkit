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

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

func sampleInventory() model.Inventory {
	inv := model.NewInventory()
	inv.Source = "k8s"
	inv.Jobs = []model.Job{
		{ID: "orders", Name: "orders", Platform: model.PlatformOSS, State: model.JobStateRunning,
			Parallelism: 4, FlinkVersion: model.ParseVersion("1.20.1"),
			Config: map[string]string{"state.backend": "rocksdb"}},
		{ID: "pay", Name: "pay", Platform: model.PlatformOSS, State: model.JobStateRunning, Parallelism: 2},
	}
	return inv
}

func TestRenderInventoryView(t *testing.T) {
	out := renderInventoryView(DefaultTheme(), sampleInventory(), 0, 100)
	for _, want := range []string{"scan", "k8s", "2 job(s)", "orders", "1.20.1", "RUNNING", "state.backend"} {
		if !strings.Contains(out, want) {
			t.Errorf("inventory view missing %q:\n%s", want, out)
		}
	}
}

func TestInventoryModelNavAndQuit(t *testing.T) {
	m := NewInventory(sampleInventory())

	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if down.(inventoryModel).cursor != 1 {
		t.Errorf("down should move cursor to 1, got %d", down.(inventoryModel).cursor)
	}
	end, _ := down.Update(tea.KeyMsg{Type: tea.KeyDown})
	if end.(inventoryModel).cursor != 1 {
		t.Errorf("cursor should clamp at last, got %d", end.(inventoryModel).cursor)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should quit")
	}
}
