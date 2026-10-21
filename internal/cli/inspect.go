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

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/graph"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/tui"
)

func newInspectCommand(opts *GlobalOptions) *cobra.Command {
	var (
		source   string
		watch    bool
		interval string
	)

	cmd := &cobra.Command{
		Use:   "inspect <job>",
		Short: "Live single-job triage: health, I/O, and logs in one view",
		Long: `Inspect assembles a single-pane triage view of one job — its operators and
their busy/backpressure, checkpoints and restarts, inputs/outputs, recent
logs, and the located bottleneck — from the platform. With --watch it refreshes
live until interrupted. It always fetches from the platform.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(cmd, opts, args, source, watch, interval)
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "source to inspect: cc|cp|oss|k8s|repo (else from the profile)")
	cmd.Flags().BoolVar(&watch, "watch", false, "live view: refresh the snapshot until interrupted")
	cmd.Flags().StringVar(&interval, "interval", "2s", "refresh interval for --watch (e.g. 2s)")

	return cmd
}

func runInspect(cmd *cobra.Command, opts *GlobalOptions, args []string, sourceFlag string, watch bool, intervalStr string) error {
	if len(args) != 1 {
		return fmt.Errorf("inspect requires exactly one <job> argument")
	}
	jobID := args[0]

	format, err := report.Resolve(opts.JSON, opts.Format)
	if err != nil {
		return err
	}

	source, acfg, err := resolveSource(opts.Profile, sourceFlag, opts.Via)
	if err != nil {
		return err
	}
	adapter, err := adapters.Default.New(source, acfg)
	if err != nil {
		return err
	}

	// TUI is the default view in an interactive terminal; fall back to plain
	// text / JSON when piped, redirected, or --json is set.
	if format == report.FormatTerminal && tui.IsTerminal(os.Stdout) {
		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			return fmt.Errorf("inspect: invalid --interval %q: %w", intervalStr, err)
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
		defer stop()
		fetch := func(c context.Context) (report.InspectSnapshot, error) {
			return buildSnapshot(c, adapter, jobID)
		}
		return tui.Run(tui.NewInspect(ctx, fetch, watch, interval))
	}

	renderer, err := report.For(format)
	if err != nil {
		return err
	}
	snap, err := buildSnapshot(cmd.Context(), adapter, jobID)
	if err != nil {
		return err
	}
	return renderer.RenderInspect(cmd.OutOrStdout(), snap)
}

// buildSnapshot assembles a point-in-time triage snapshot for one job: the
// operator graph with a single read of per-subtask rates, job-level metrics,
// source/sink I/O, and the located bottleneck.
func buildSnapshot(ctx context.Context, a adapters.PlatformAdapter, jobID string) (report.InspectSnapshot, error) {
	job, err := a.GetJobGraph(ctx, jobID)
	if err != nil {
		return report.InspectSnapshot{}, err
	}
	job = sampleSubtasks(ctx, a, job) // one read of per-subtask busy/backpressure/records

	metrics, _ := a.GetMetrics(ctx, jobID) // best-effort

	snap := report.InspectSnapshot{
		JobID:        job.ID,
		JobName:      job.Name,
		State:        string(job.State),
		FlinkVersion: job.FlinkVersion.String(),
		Checkpoints: report.Checkpoints{
			Completed: int64(metrics["checkpoints.completed"]),
			Failed:    int64(metrics["checkpoints.failed"]),
		},
		Restarts: int64(metrics["numRestarts"]),
	}

	g, gerr := graph.New(job)
	for _, op := range job.Operators {
		busy, bp := operatorRates(op)
		snap.Operators = append(snap.Operators, report.InspectOperator{
			ID:                    op.ID,
			Name:                  op.Name,
			Parallelism:           op.Parallelism,
			BusyMsPerSec:          busy,
			BackpressuredMsPerSec: bp,
		})

		// Sources have no upstream; sinks have no downstream. A fused operator
		// that is both (common in Flink) gets a single "source+sink" entry
		// rather than being listed twice.
		isSource := len(op.Upstreams) == 0
		isSink := gerr == nil && g.IsSink(op.ID)
		if isSource || isSink {
			dir := "source+sink"
			switch {
			case isSource && !isSink:
				dir = "source"
			case isSink && !isSource:
				dir = "sink"
			}
			recIn, recOut := operatorRecords(op)
			snap.IO = append(snap.IO, report.IOEndpoint{
				Operator: op.ID, Direction: dir, Resource: op.Name, RecordsIn: recIn, RecordsOut: recOut,
			})
		}
	}

	if gerr == nil {
		if rep, err := analyzeBottleneck(jobID, job); err == nil && rep.Found {
			snap.Bottleneck = &rep
		}
	}

	// Run the health rules so the cockpit's findings panel is real.
	if engine, err := newDoctorEngine(); err == nil {
		snap.Findings = engine.Evaluate(rules.EvalContext{Job: job, Metrics: metrics})
	}

	// Logs are an optional adapter capability; degrade silently when absent.
	if lp, ok := a.(adapters.LogProvider); ok {
		if logs, err := lp.GetLogs(ctx, jobID, inspectLogLines); err == nil {
			for _, l := range logs {
				snap.Logs = append(snap.Logs, report.LogLine{Source: l.Source, Text: l.Text})
			}
		}
	}

	return snap, nil
}

// inspectLogLines is how many recent log lines the snapshot carries.
const inspectLogLines = 20

func operatorRates(op model.Operator) (busy, bp float64) {
	for _, s := range op.Subtasks {
		if s.BusyMsPerSec > busy {
			busy = s.BusyMsPerSec
		}
		if s.BackpressuredMsPerSec > bp {
			bp = s.BackpressuredMsPerSec
		}
	}
	return busy, bp
}

func operatorRecords(op model.Operator) (in, out int64) {
	for _, s := range op.Subtasks {
		in += s.RecordsIn
		out += s.RecordsOut
	}
	return in, out
}
