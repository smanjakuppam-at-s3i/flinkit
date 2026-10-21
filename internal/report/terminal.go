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

package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

// terminalRenderer writes a human-readable summary. It uses the standard
// library's tabwriter for column alignment; richer styling (lipgloss and
// tablewriter, per the tech stack) can replace the internals later without
// touching the Renderer seam.
type terminalRenderer struct{}

func (terminalRenderer) RenderInventory(w io.Writer, inv model.Inventory) error {
	if _, err := fmt.Fprintf(w, "Inventory  source=%s  jobs=%d  statements=%d  connectors=%d\n",
		dash(inv.Source), len(inv.Jobs), len(inv.Statements), len(inv.Connectors)); err != nil {
		return err
	}
	if len(inv.Jobs) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "\nID\tNAME\tPLATFORM\tVERSION\tSTATE\tPARALLELISM"); err != nil {
		return err
	}
	for _, j := range inv.Jobs {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			dash(j.ID), dash(j.Name), dash(string(j.Platform)),
			dash(j.FlinkVersion.String()), dash(string(j.State)), intOrDash(j.Parallelism)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func (terminalRenderer) RenderFindings(w io.Writer, findings rules.Findings) error {
	if _, err := fmt.Fprintf(w, "%s\n", severitySummary(findings)); err != nil {
		return err
	}
	for _, f := range findings {
		if err := writeFinding(w, f); err != nil {
			return err
		}
	}
	return nil
}

func (terminalRenderer) RenderFleet(w io.Writer, results []JobFindings) error {
	var total rules.Findings
	for _, r := range results {
		total = append(total, r.Findings...)
	}
	if _, err := fmt.Fprintf(w, "Fleet: %d job(s), %s\n", len(results), severitySummary(total)); err != nil {
		return err
	}
	for _, r := range results {
		if _, err := fmt.Fprintf(w, "\n=== %s (%s) — %d finding(s) ===\n",
			dash(r.JobName), dash(r.JobID), len(r.Findings)); err != nil {
			return err
		}
		if len(r.Findings) == 0 {
			if _, err := fmt.Fprintln(w, "  (healthy)"); err != nil {
				return err
			}
			continue
		}
		for _, f := range r.Findings {
			if err := writeFinding(w, f); err != nil {
				return err
			}
		}
	}
	return nil
}

func (terminalRenderer) RenderBottleneck(w io.Writer, r BottleneckReport) error {
	if !r.Found {
		_, err := fmt.Fprintf(w, "No bottleneck found in %s: the job is not backpressured.\n", dash(r.JobID))
		return err
	}
	if _, err := fmt.Fprintf(w, "Bottleneck in %s: %s (%s), subtask %d\n",
		dash(r.JobID), dash(r.OperatorName), dash(r.OperatorID), r.WorstSubtask); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  %.0f ms/s busy (%d%%), %.0f ms/s backpressured\n",
		r.BusyMsPerSec, int(r.BusyMsPerSec/10), r.BackpressuredMsPerSec); err != nil {
		return err
	}
	for _, line := range []struct{ label, val string }{
		{"cause", fmt.Sprintf("%s — %s", r.Cause, r.Summary)},
		{"evidence", r.Evidence},
		{"fix", r.Fix},
	} {
		if strings.TrimSpace(line.val) == "" || strings.TrimSpace(line.val) == "—" {
			continue
		}
		if _, err := fmt.Fprintf(w, "  %s: %s\n", line.label, line.val); err != nil {
			return err
		}
	}
	return nil
}

func (terminalRenderer) RenderInspect(w io.Writer, s InspectSnapshot) error {
	if _, err := fmt.Fprintf(w, "Job: %s (%s)  state %s  flink %s\n",
		dash(s.JobName), dash(s.JobID), dash(s.State), dash(s.FlinkVersion)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Checkpoints: %d completed, %d failed   Restarts: %d\n",
		s.Checkpoints.Completed, s.Checkpoints.Failed, s.Restarts); err != nil {
		return err
	}

	if len(s.Operators) > 0 {
		fmt.Fprintln(w, "\nOperators:")
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tPAR\tBUSY%\tBP%")
		for _, op := range s.Operators {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\n",
				dash(op.ID), dash(trunc(op.Name, 48)), intOrDash(op.Parallelism),
				int(op.BusyMsPerSec/10), int(op.BackpressuredMsPerSec/10))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	if len(s.IO) > 0 {
		fmt.Fprintln(w, "\nI/O:")
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		for _, io := range s.IO {
			fmt.Fprintf(tw, "  %s\t%s\t%s\tin %d / out %d\n",
				io.Direction, dash(io.Operator), dash(trunc(io.Resource, 48)), io.RecordsIn, io.RecordsOut)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	if len(s.Findings) > 0 {
		fmt.Fprintf(w, "\nFindings: %s\n", severitySummary(s.Findings))
		for _, f := range s.Findings {
			fmt.Fprintf(w, "  [%s] %s  %s\n", strings.ToUpper(string(f.Severity)), f.RuleID, f.Message)
		}
	}

	if s.Bottleneck != nil && s.Bottleneck.Found {
		b := s.Bottleneck
		fmt.Fprintf(w, "\nBottleneck: %s (%s) subtask %d — %s\n",
			dash(b.OperatorName), dash(b.OperatorID), b.WorstSubtask, dash(b.Cause))
		if b.Fix != "" {
			fmt.Fprintf(w, "  fix: %s\n", b.Fix)
		}
	}

	if len(s.Logs) > 0 {
		fmt.Fprintln(w, "\nRecent logs:")
		for _, l := range s.Logs {
			fmt.Fprintf(w, "  [%s] %s\n", dash(l.Source), l.Text)
		}
	}
	return nil
}

// severitySummary renders a one-line count of findings by severity.
func severitySummary(findings rules.Findings) string {
	var high, medium, low, info int
	for _, f := range findings {
		switch f.Severity {
		case rules.SeverityHigh:
			high++
		case rules.SeverityMedium:
			medium++
		case rules.SeverityLow:
			low++
		case rules.SeverityInfo:
			info++
		}
	}
	return fmt.Sprintf("%d finding(s): %d high, %d medium, %d low, %d info",
		len(findings), high, medium, low, info)
}

// writeFinding renders one finding as an indented block.
func writeFinding(w io.Writer, f rules.Finding) error {
	if _, err := fmt.Fprintf(w, "\n[%s] %s  %s\n  %s\n",
		strings.ToUpper(string(f.Severity)), f.RuleID, dash(f.Target), f.Message); err != nil {
		return err
	}
	for _, line := range []struct{ label, val string }{
		{"evidence", f.Evidence},
		{"fix", f.Fix},
		{"docs", f.Docs},
	} {
		if line.val == "" {
			continue
		}
		if _, err := fmt.Fprintf(w, "  %s: %s\n", line.label, line.val); err != nil {
			return err
		}
	}
	return nil
}

// dash renders an empty string as a dash so blank cells are visible.
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// trunc shortens s to at most n runes, marking truncation with an ellipsis, so
// long operator names don't blow out table widths.
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

func intOrDash(n int) string {
	if n == 0 {
		return "-"
	}
	return strconv.Itoa(n)
}
