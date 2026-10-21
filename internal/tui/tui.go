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

// Package tui is the shared terminal-UI layer: one lipgloss theme and reusable
// helpers that every command's view renders with, so the tool looks like one
// product. bubbletea drives the loop; views stay pure (snapshot -> string) so
// they are testable without a terminal.
package tui

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// IsTerminal reports whether f is an interactive terminal. Commands default to
// the TUI when stdout is a terminal and fall back to plain text otherwise.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// Run starts a bubbletea program in the alternate screen and blocks until exit.
func Run(m tea.Model) error {
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// Theme holds the shared styles used across command views.
type Theme struct {
	Title  lipgloss.Style
	Panel  lipgloss.Style
	Faint  lipgloss.Style
	High   lipgloss.Style
	Med    lipgloss.Style
	OK     lipgloss.Style
	Border lipgloss.Style
}

// DefaultTheme is the standard flinkit palette.
func DefaultTheme() Theme {
	return Theme{
		Title:  lipgloss.NewStyle().Bold(true),
		Panel:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")),
		Faint:  lipgloss.NewStyle().Faint(true),
		High:   lipgloss.NewStyle().Foreground(lipgloss.Color("9")),  // bright red
		Med:    lipgloss.NewStyle().Foreground(lipgloss.Color("11")), // bright yellow
		OK:     lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // bright green
		Border: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
	}
}

// gauge renders a 0–100% value as a colored block bar (green/yellow/red).
func (t Theme) gauge(pct int) string {
	const w = 10
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := (pct*w + 50) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", w-filled)
	return t.level(pct).Render(bar)
}

// level picks the color style for a percentage.
func (t Theme) level(pct int) lipgloss.Style {
	switch {
	case pct >= 80:
		return t.High
	case pct >= 50:
		return t.Med
	default:
		return t.OK
	}
}

// sparkRunes are the eight block heights used for sparklines.
var sparkRunes = []rune("▁▂▃▄▅▆▇█")

// sparkline renders values (each 0–100) as a block-character trend, showing at
// most the last width samples.
func sparkline(vals []float64, width int) string {
	if len(vals) == 0 {
		return ""
	}
	if width > 0 && len(vals) > width {
		vals = vals[len(vals)-width:]
	}
	var b strings.Builder
	for _, v := range vals {
		idx := int(v/100*float64(len(sparkRunes)-1) + 0.5)
		if idx < 0 {
			idx = 0
		}
		if idx > len(sparkRunes)-1 {
			idx = len(sparkRunes) - 1
		}
		b.WriteRune(sparkRunes[idx])
	}
	return b.String()
}

// dash renders an empty string as a dash.
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// trunc shortens s to at most n runes with an ellipsis.
func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
