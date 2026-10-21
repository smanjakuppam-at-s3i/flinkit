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
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// NewInventory builds the scan job-table browser: a navigable list of jobs with
// a detail pane for the selected one. Static (render once, navigable).
func NewInventory(inv model.Inventory) tea.Model {
	return inventoryModel{theme: DefaultTheme(), inv: inv, width: 80}
}

type inventoryModel struct {
	theme  Theme
	inv    model.Inventory
	cursor int
	width  int
}

func (m inventoryModel) Init() tea.Cmd { return nil }

func (m inventoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.inv.Jobs)-1 {
				m.cursor++
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}

func (m inventoryModel) View() string {
	return renderInventoryView(m.theme, m.inv, m.cursor, m.width)
}

func renderInventoryView(t Theme, inv model.Inventory, cursor, width int) string {
	inner := width - 4
	if inner < 60 {
		inner = 60
	}
	var b strings.Builder
	b.WriteString(t.Title.Render("flinkit ▸ scan ▸ "+dash(inv.Source)) + "   " +
		t.Faint.Render(fmt.Sprintf("%d job(s)", len(inv.Jobs))) + "\n\n")

	if len(inv.Jobs) == 0 {
		b.WriteString("  " + t.Faint.Render("no jobs"))
		return t.Border.Width(inner).Render(b.String())
	}

	const rowFmt = "%-20s %-22s %-5s %-9s %-9s %s"
	b.WriteString("  " + t.Faint.Render(fmt.Sprintf(rowFmt, "ID", "NAME", "PLAT", "VERSION", "STATE", "PAR")) + "\n")
	for i, j := range inv.Jobs {
		row := fmt.Sprintf(rowFmt,
			trunc(j.ID, 20), trunc(dash(j.Name), 22), dash(string(j.Platform)),
			dash(j.FlinkVersion.String()), dash(string(j.State)), intStr(j.Parallelism))
		prefix := "  "
		if i == cursor {
			prefix = t.Title.Render("▸ ")
			row = t.Title.Render(row)
		}
		b.WriteString(prefix + row + "\n")
	}

	if cursor >= 0 && cursor < len(inv.Jobs) {
		j := inv.Jobs[cursor]
		b.WriteString("\n" + t.Panel.Render("job") + "\n")
		b.WriteString(fmt.Sprintf("  %-11s %s\n", "name:", dash(j.Name)))
		b.WriteString(fmt.Sprintf("  %-11s %s\n", "state:", dash(string(j.State))))
		b.WriteString(fmt.Sprintf("  %-11s %s\n", "flink:", dash(j.FlinkVersion.String())))
		b.WriteString(fmt.Sprintf("  %-11s %d\n", "operators:", len(j.Operators)))
		if len(j.Config) > 0 {
			b.WriteString("  config:\n")
			for _, k := range sortedKeys(j.Config) {
				b.WriteString(fmt.Sprintf("    %s = %s\n", k, j.Config[k]))
			}
		}
	}

	b.WriteString("\n" + t.Faint.Render("↑↓ select · q quit"))
	return t.Border.Width(inner).Render(b.String())
}

func intStr(n int) string {
	if n == 0 {
		return "-"
	}
	return strconv.Itoa(n)
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
