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
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
)

// inspectHistory is how many busy samples per operator the sparkline keeps.
const inspectHistory = 30

// SnapshotFunc fetches a fresh inspect snapshot.
type SnapshotFunc func(context.Context) (report.InspectSnapshot, error)

// NewInspect builds the inspect dashboard model. With live=true it refreshes
// every interval (until paused); otherwise it renders once and stays navigable.
func NewInspect(ctx context.Context, fetch SnapshotFunc, live bool, interval time.Duration) tea.Model {
	return inspectModel{
		ctx:      ctx,
		fetch:    fetch,
		live:     live,
		interval: interval,
		theme:    DefaultTheme(),
		width:    80,
		height:   24,
		history:  map[string][]float64{},
	}
}

type inspectModel struct {
	ctx      context.Context
	fetch    SnapshotFunc
	live     bool
	interval time.Duration
	theme    Theme

	snap    report.InspectSnapshot
	err     error
	loaded  bool
	width   int
	height  int
	cursor  int                  // selected operator
	paused  bool                 // live refresh paused
	history map[string][]float64 // operator id -> recent busy% (sparkline)
}

type inspectTickMsg time.Time

type inspectSnapMsg struct {
	snap report.InspectSnapshot
	err  error
}

func (m inspectModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.fetchCmd()}
	if m.live {
		cmds = append(cmds, m.tickCmd())
	}
	return tea.Batch(cmds...)
}

func (m inspectModel) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		s, err := m.fetch(m.ctx)
		return inspectSnapMsg{snap: s, err: err}
	}
}

func (m inspectModel) tickCmd() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg { return inspectTickMsg(t) })
}

func (m inspectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.cursor < len(m.snap.Operators)-1 {
				m.cursor++
			}
		case "p":
			m.paused = !m.paused
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case inspectSnapMsg:
		m.snap, m.err, m.loaded = msg.snap, msg.err, true
		m.record()
		switch {
		case len(m.snap.Operators) == 0:
			m.cursor = 0
		case m.cursor > len(m.snap.Operators)-1:
			m.cursor = len(m.snap.Operators) - 1
		}
	case inspectTickMsg:
		if m.live {
			if m.paused {
				return m, m.tickCmd()
			}
			return m, tea.Batch(m.fetchCmd(), m.tickCmd())
		}
	}
	return m, nil
}

// record appends the latest busy% per operator to the sparkline history. The
// history map is shared across model copies (maps are references), so the trend
// accumulates across ticks.
func (m inspectModel) record() {
	for _, op := range m.snap.Operators {
		h := append(m.history[op.ID], op.BusyMsPerSec/10)
		if len(h) > inspectHistory {
			h = h[len(h)-inspectHistory:]
		}
		m.history[op.ID] = h
	}
}

func (m inspectModel) View() string {
	t := m.theme
	inner := m.width - 4
	if inner < 48 {
		inner = 48
	}

	var b strings.Builder

	meta := strings.TrimSpace(fmt.Sprintf("%s · Flink %s", dash(string(m.snap.State)), dash(m.snap.FlinkVersion)))
	switch {
	case m.paused:
		meta += " · ⏸ paused"
	case m.live:
		meta += " · ⟳live"
	}
	b.WriteString(t.Title.Render("flinkit ▸ "+dash(m.snap.JobName)) + "   " + t.Faint.Render(meta) + "\n")

	switch {
	case !m.loaded:
		b.WriteString(t.Faint.Render("connecting…"))
		return t.Border.Width(inner).Render(b.String())
	case m.err != nil:
		b.WriteString(t.High.Render("fetch error: " + m.err.Error()))
		return t.Border.Width(inner).Render(b.String())
	}

	b.WriteString(fmt.Sprintf("checkpoints %d✓ %d✗    restarts %d\n",
		m.snap.Checkpoints.Completed, m.snap.Checkpoints.Failed, m.snap.Restarts))

	b.WriteString("\n" + t.Panel.Render("operators") + "\n")
	for i, op := range m.snap.Operators {
		busy := int(op.BusyMsPerSec / 10)
		bp := int(op.BackpressuredMsPerSec / 10)
		name := fmt.Sprintf("%-26s", trunc(dash(op.Name), 26))
		prefix := "  "
		if i == m.cursor {
			prefix = t.Title.Render("▸ ")
			name = t.Title.Render(name)
		}
		spark := t.level(busy).Render(sparkline(m.history[op.ID], 24))
		b.WriteString(fmt.Sprintf("%s%s  %s %3d%%  bp %3d%%\n", prefix, name, spark, busy, bp))
	}
	if m.cursor < len(m.snap.Operators) {
		op := m.snap.Operators[m.cursor]
		b.WriteString("  " + t.Faint.Render(fmt.Sprintf("selected: %s · parallelism %d · busy %d%% · bp %d%%",
			dash(op.Name), op.Parallelism, int(op.BusyMsPerSec/10), int(op.BackpressuredMsPerSec/10))) + "\n")
	}

	if len(m.snap.IO) > 0 {
		b.WriteString("\n" + t.Panel.Render("i/o") + "\n")
		for _, io := range m.snap.IO {
			b.WriteString(fmt.Sprintf("  %-11s %-26s  in %d / out %d\n",
				io.Direction, trunc(dash(io.Resource), 26), io.RecordsIn, io.RecordsOut))
		}
	}

	b.WriteString("\n" + t.Panel.Render("findings") + "\n")
	b.WriteString(t.renderFindingsList(m.snap.Findings, -1, 5) + "\n")

	b.WriteString("\n" + t.Panel.Render("bottleneck") + "\n")
	if m.snap.Bottleneck != nil && m.snap.Bottleneck.Found {
		bn := m.snap.Bottleneck
		b.WriteString(fmt.Sprintf("  %s (%s) subtask %d — %s\n",
			dash(bn.OperatorName), dash(bn.OperatorID), bn.WorstSubtask, t.Med.Render(dash(bn.Cause))))
		if bn.Fix != "" {
			b.WriteString("  " + t.Faint.Render("fix: "+bn.Fix) + "\n")
		}
	} else {
		b.WriteString("  " + t.Faint.Render("none — job is not backpressured") + "\n")
	}

	if len(m.snap.Logs) > 0 {
		b.WriteString("\n" + t.Panel.Render("logs") + "\n")
		logs := m.snap.Logs
		if n := logLines(m.height); len(logs) > n {
			logs = logs[len(logs)-n:]
		}
		for _, l := range logs {
			b.WriteString("  " + trunc(l.Text, inner-2) + "\n")
		}
	}

	b.WriteString("\n" + t.Faint.Render("↑↓ operator · p pause · q quit"))
	return t.Border.Width(inner).Render(b.String())
}

// logLines returns how many log lines to show given the terminal height.
func logLines(height int) int {
	switch {
	case height <= 0:
		return 6
	case height < 24:
		return 3
	case height < 40:
		return 6
	default:
		return 10
	}
}
