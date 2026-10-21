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
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/graph"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/tui"
)

// bottleneckSamples is how many reads are averaged across the window.
const bottleneckSamples = 3

func newBottleneckCommand(opts *GlobalOptions) *cobra.Command {
	var (
		window string
		source string
	)

	cmd := &cobra.Command{
		Use:   "bottleneck <job>",
		Short: "Find the choke point in a running job",
		Long: `Bottleneck samples busy, backpressured, and idle time per subtask over a
short window, walks the job graph from sources to sinks to find where
backpressure stops, classifies the cause, and suggests a fix. It is a live
analysis and always fetches from the platform.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBottleneck(cmd, opts, args, source, window)
		},
	}

	cmd.Flags().StringVar(&window, "window", "30s", "sampling window (e.g. 30s)")
	cmd.Flags().StringVar(&source, "source", "", "source to analyze: cc|cp|oss|k8s|repo (else from the profile)")

	return cmd
}

func runBottleneck(cmd *cobra.Command, opts *GlobalOptions, args []string, sourceFlag, window string) error {
	if len(args) != 1 {
		return fmt.Errorf("bottleneck requires exactly one <job> argument")
	}
	jobID := args[0]

	format, err := report.Resolve(opts.JSON, opts.Format)
	if err != nil {
		return err
	}
	win, err := time.ParseDuration(window)
	if err != nil {
		return fmt.Errorf("bottleneck: invalid --window %q: %w", window, err)
	}

	source, acfg, err := resolveSource(opts.Profile, sourceFlag, opts.Via)
	if err != nil {
		return err
	}
	adapter, err := adapters.Default.New(source, acfg)
	if err != nil {
		return err
	}

	interval := win / bottleneckSamples
	job, err := sampleJob(cmd.Context(), adapter, jobID, bottleneckSamples, interval, time.Sleep)
	if err != nil {
		return err
	}

	rep, err := analyzeBottleneck(jobID, job)
	if err != nil {
		return err
	}

	// Focused choke-point TUI by default in a terminal; plain/JSON otherwise.
	if format == report.FormatTerminal && tui.IsTerminal(os.Stdout) {
		return tui.Run(tui.NewBottleneck(rep))
	}

	renderer, err := report.For(format)
	if err != nil {
		return err
	}
	return renderer.RenderBottleneck(cmd.OutOrStdout(), rep)
}

// analyzeBottleneck builds the graph, locates the choke point, classifies it,
// and flattens the result into a report.
func analyzeBottleneck(jobID string, job model.Job) (report.BottleneckReport, error) {
	g, err := graph.New(job)
	if err != nil {
		return report.BottleneckReport{}, err
	}
	b, found := g.Locate(graph.DefaultThresholds())
	if !found {
		return report.BottleneckReport{JobID: jobID, Found: false}, nil
	}
	cause := g.Classify(b)
	return report.BottleneckReport{
		JobID:                 jobID,
		Found:                 true,
		OperatorID:            b.Operator.ID,
		OperatorName:          b.Operator.Name,
		WorstSubtask:          b.WorstSubtask,
		BusyMsPerSec:          b.BusyMsPerSec,
		BackpressuredMsPerSec: b.BackpressuredMsPerSec,
		Cause:                 string(cause.Kind),
		Summary:               cause.Summary,
		Evidence:              cause.Evidence,
		Fix:                   cause.Fix,
	}, nil
}

// sampleJob fetches the job graph and averages each subtask's busy/backpressure/
// idle rates over the given number of samples spaced by interval. sleep is
// injected so tests can run with one sample and no delay.
func sampleJob(ctx context.Context, a adapters.PlatformAdapter, jobID string, samples int, interval time.Duration, sleep func(time.Duration)) (model.Job, error) {
	if samples < 1 {
		samples = 1
	}
	job, err := a.GetJobGraph(ctx, jobID)
	if err != nil {
		return model.Job{}, err
	}

	type accum struct {
		busy, bp, idle float64
		recIn, recOut  int64
		n              int
	}
	totals := make([]map[int]*accum, len(job.Operators))
	for i := range totals {
		totals[i] = map[int]*accum{}
	}

	for s := 0; s < samples; s++ {
		if s > 0 {
			sleep(interval)
		}
		for i, op := range job.Operators {
			subs, err := a.GetSubtaskMetrics(ctx, jobID, op.ID)
			if err != nil {
				return model.Job{}, fmt.Errorf("sampling operator %s: %w", op.ID, err)
			}
			for _, st := range subs {
				acc := totals[i][st.Index]
				if acc == nil {
					acc = &accum{}
					totals[i][st.Index] = acc
				}
				acc.busy += st.BusyMsPerSec
				acc.bp += st.BackpressuredMsPerSec
				acc.idle += st.IdleMsPerSec
				acc.recIn = st.RecordsIn
				acc.recOut = st.RecordsOut
				acc.n++
			}
		}
	}

	for i := range job.Operators {
		idxs := make([]int, 0, len(totals[i]))
		for idx := range totals[i] {
			idxs = append(idxs, idx)
		}
		sort.Ints(idxs)

		subs := make([]model.Subtask, 0, len(idxs))
		for _, idx := range idxs {
			acc := totals[i][idx]
			subs = append(subs, model.Subtask{
				Index:                 idx,
				BusyMsPerSec:          acc.busy / float64(acc.n),
				BackpressuredMsPerSec: acc.bp / float64(acc.n),
				IdleMsPerSec:          acc.idle / float64(acc.n),
				RecordsIn:             acc.recIn,
				RecordsOut:            acc.recOut,
			})
		}
		job.Operators[i].Subtasks = subs
	}
	return job, nil
}
