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
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
)

// NewBottleneck builds the bottleneck view: a focused choke-point panel for one
// job. Static (render once).
func NewBottleneck(rep report.BottleneckReport) tea.Model {
	return bottleneckModel{theme: DefaultTheme(), rep: rep, width: 80}
}

type bottleneckModel struct {
	theme Theme
	rep   report.BottleneckReport
	width int
}

func (m bottleneckModel) Init() tea.Cmd { return nil }

func (m bottleneckModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}

func (m bottleneckModel) View() string {
	return renderBottleneckView(m.theme, m.rep, m.width)
}

func renderBottleneckView(t Theme, r report.BottleneckReport, width int) string {
	inner := width - 4
	if inner < 48 {
		inner = 48
	}
	var b strings.Builder
	b.WriteString(t.Title.Render("flinkit ▸ bottleneck ▸ "+dash(r.JobID)) + "\n\n")

	if !r.Found {
		b.WriteString("  " + t.OK.Render("No bottleneck — the job is not backpressured.") + "\n")
		b.WriteString("\n" + t.Faint.Render("q quit"))
		return t.Border.Width(inner).Render(b.String())
	}

	busy := int(r.BusyMsPerSec / 10)
	bp := int(r.BackpressuredMsPerSec / 10)

	b.WriteString(t.Panel.Render("choke point") + "\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", t.Title.Render(dash(r.OperatorName)), t.Faint.Render("("+dash(r.OperatorID)+")")))
	b.WriteString(fmt.Sprintf("  subtask %d\n", r.WorstSubtask))
	b.WriteString(fmt.Sprintf("  busy          %s %3d%%\n", t.gauge(busy), busy))
	b.WriteString(fmt.Sprintf("  backpressure  %s %3d%%\n", t.gauge(bp), bp))

	b.WriteString("\n" + t.Panel.Render("cause") + "\n")
	b.WriteString("  " + t.Med.Render(dash(r.Cause)) + "\n")
	if r.Summary != "" {
		b.WriteString("  " + t.Faint.Render(r.Summary) + "\n")
	}
	if r.Evidence != "" {
		b.WriteString("  " + t.Faint.Render("evidence: "+r.Evidence) + "\n")
	}
	if r.Fix != "" {
		b.WriteString("\n" + t.Panel.Render("fix") + "\n  " + r.Fix + "\n")
	}

	b.WriteString("\n" + t.Faint.Render("q quit"))
	return t.Border.Width(inner).Render(b.String())
}
