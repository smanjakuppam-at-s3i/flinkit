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
	"github.com/charmbracelet/lipgloss"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

// severityStyle maps a severity to its color.
func (t Theme) severityStyle(sev rules.Severity) lipgloss.Style {
	switch sev {
	case rules.SeverityHigh:
		return t.High
	case rules.SeverityMedium:
		return t.Med
	default:
		return t.Faint
	}
}

// renderFindingsList renders findings as one line each. selected >= 0 marks that
// row; maxRows > 0 caps the number shown (with a "… N more" note).
func (t Theme) renderFindingsList(fs rules.Findings, selected, maxRows int) string {
	if len(fs) == 0 {
		return "  " + t.OK.Render("no findings")
	}
	n := len(fs)
	if maxRows > 0 && n > maxRows {
		n = maxRows
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		f := fs[i]
		prefix := "  "
		if i == selected {
			prefix = t.Title.Render("▸ ")
		}
		line := fmt.Sprintf("%-6s %-14s %s", strings.ToUpper(string(f.Severity)), f.RuleID, f.Message)
		b.WriteString(prefix + t.severityStyle(f.Severity).Render(line) + "\n")
	}
	if maxRows > 0 && len(fs) > maxRows {
		b.WriteString(t.Faint.Render(fmt.Sprintf("  … %d more", len(fs)-maxRows)))
		return b.String()
	}
	return strings.TrimRight(b.String(), "\n")
}

// severitySummary is a one-line count by severity.
func severitySummary(fs rules.Findings) string {
	var high, med, low, info int
	for _, f := range fs {
		switch f.Severity {
		case rules.SeverityHigh:
			high++
		case rules.SeverityMedium:
			med++
		case rules.SeverityLow:
			low++
		case rules.SeverityInfo:
			info++
		}
	}
	return fmt.Sprintf("%d finding(s): %d high, %d medium, %d low, %d info", len(fs), high, med, low, info)
}

// NewFindings builds the doctor findings browser: a navigable list with a detail
// pane for the selected finding. Static (render once, navigable).
func NewFindings(title string, findings rules.Findings) tea.Model {
	return findingsModel{theme: DefaultTheme(), title: title, findings: findings, width: 80}
}

type findingsModel struct {
	theme    Theme
	title    string
	findings rules.Findings
	cursor   int
	width    int
}

func (m findingsModel) Init() tea.Cmd { return nil }

func (m findingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.findings)-1 {
				m.cursor++
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}

func (m findingsModel) View() string {
	return renderFindingsView(m.theme, m.title, m.findings, m.cursor, m.width)
}

func renderFindingsView(t Theme, title string, fs rules.Findings, cursor, width int) string {
	inner := width - 4
	if inner < 48 {
		inner = 48
	}
	var b strings.Builder
	b.WriteString(t.Title.Render("flinkit ▸ doctor ▸ "+dash(title)) + "   " + t.Faint.Render(severitySummary(fs)) + "\n\n")
	b.WriteString(t.renderFindingsList(fs, cursor, 0) + "\n")

	if cursor >= 0 && cursor < len(fs) {
		f := fs[cursor]
		b.WriteString("\n" + t.Panel.Render("detail") + "\n")
		for _, line := range []struct{ label, val string }{
			{"target", f.Target},
			{"evidence", f.Evidence},
			{"fix", f.Fix},
			{"docs", f.Docs},
		} {
			if line.val != "" {
				b.WriteString(fmt.Sprintf("  %-9s %s\n", line.label+":", line.val))
			}
		}
	}

	b.WriteString("\n" + t.Faint.Render("↑↓ select · q quit"))
	return t.Border.Width(inner).Render(b.String())
}
