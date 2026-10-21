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
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules/builtin"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/tui"
)

func newDoctorCommand(opts *GlobalOptions) *cobra.Command {
	var (
		all           bool
		failOn        string
		source        string
		inventoryPath string
	)

	cmd := &cobra.Command{
		Use:   "doctor [job]",
		Short: "Run rule-based health checks against a job or the whole fleet",
		Long: `Doctor runs rule-based health checks against a job and reports a ranked
findings list with severity, evidence, and a suggested fix. It reads the job
from .flinkit/inventory.json when present, otherwise fetches it live from the
platform. Use --fail-on to turn findings into a non-zero exit for CI.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd, opts, args, source, inventoryPath, all, failOn)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "check all jobs (fleet mode)")
	cmd.Flags().StringVar(&failOn, "fail-on", "", "exit non-zero on findings at or above low|medium|high")
	cmd.Flags().StringVar(&source, "source", "", "source for a live fetch when no inventory is present: cc|cp|oss|k8s|repo")
	cmd.Flags().StringVar(&inventoryPath, "inventory", filepath.Join(".flinkit", "inventory.json"), "inventory file to read jobs from")

	return cmd
}

func runDoctor(cmd *cobra.Command, opts *GlobalOptions, args []string, sourceFlag, inventoryPath string, all bool, failOn string) error {
	if all && len(args) > 0 {
		return fmt.Errorf("doctor: cannot combine --all with a <job> argument")
	}
	if !all && len(args) != 1 {
		return fmt.Errorf("doctor requires a <job> argument (or --all for the whole fleet)")
	}

	format, err := report.Resolve(opts.JSON, opts.Format)
	if err != nil {
		return err
	}
	var threshold rules.Severity
	if failOn != "" {
		if threshold, err = rules.ParseSeverity(failOn); err != nil {
			return err
		}
	}
	engine, err := newDoctorEngine()
	if err != nil {
		return err
	}
	renderer, err := report.For(format)
	if err != nil {
		return err
	}

	if all {
		return runDoctorFleet(cmd, opts, engine, renderer, sourceFlag, inventoryPath, failOn, threshold)
	}

	job, err := resolveJob(cmd.Context(), opts, args[0], sourceFlag, inventoryPath)
	if err != nil {
		return err
	}
	ctx := rules.EvalContext{Job: job, Metrics: fetchMetrics(cmd.Context(), opts, job.ID, sourceFlag)}
	findings := engine.Evaluate(ctx)

	// Navigable findings TUI by default in a terminal; plain/JSON otherwise.
	// --fail-on keeps the scripted plain path (exit code is the signal).
	if format == report.FormatTerminal && failOn == "" && tui.IsTerminal(os.Stdout) {
		return tui.Run(tui.NewFindings(job.Name, findings))
	}

	if err := renderer.RenderFindings(cmd.OutOrStdout(), findings); err != nil {
		return err
	}
	return gate(failOn, threshold, findings)
}

func runDoctorFleet(cmd *cobra.Command, opts *GlobalOptions, engine *rules.Engine, renderer report.Renderer, sourceFlag, inventoryPath, failOn string, threshold rules.Severity) error {
	jobs, err := resolveAllJobs(cmd.Context(), opts, sourceFlag, inventoryPath)
	if err != nil {
		return err
	}

	results := make([]report.JobFindings, 0, len(jobs))
	var total rules.Findings
	for _, j := range jobs {
		ctx := rules.EvalContext{Job: j, Metrics: fetchMetrics(cmd.Context(), opts, j.ID, sourceFlag)}
		fs := engine.Evaluate(ctx)
		results = append(results, report.JobFindings{JobID: j.ID, JobName: j.Name, Findings: fs})
		total = append(total, fs...)
	}

	if err := renderer.RenderFleet(cmd.OutOrStdout(), results); err != nil {
		return err
	}
	return gate(failOn, threshold, total)
}

// gate turns findings into a non-zero exit when --fail-on is set and the
// threshold is met. The findings themselves are already on stdout; this error
// is the CI signal.
func gate(failOn string, threshold rules.Severity, findings rules.Findings) error {
	if failOn == "" {
		return nil
	}
	if n := len(findings.AtLeast(threshold)); n > 0 {
		return fmt.Errorf("doctor: %d finding(s) at or above %s", n, threshold)
	}
	return nil
}

// resolveJob returns the job to diagnose: from the inventory file when it
// exists (the job must be present), otherwise fetched live via the adapter.
func resolveJob(ctx context.Context, opts *GlobalOptions, jobID, sourceFlag, inventoryPath string) (model.Job, error) {
	if _, err := os.Stat(inventoryPath); err == nil {
		inv, err := readInventoryFile(inventoryPath)
		if err != nil {
			return model.Job{}, err
		}
		for _, j := range inv.Jobs {
			if j.ID == jobID {
				return j, nil
			}
		}
		return model.Job{}, fmt.Errorf("doctor: job %q not in %s (run flinkit scan)", jobID, inventoryPath)
	}

	source, acfg, err := resolveSource(opts.Profile, sourceFlag, opts.Via)
	if err != nil {
		return model.Job{}, err
	}
	adapter, err := adapters.Default.New(source, acfg)
	if err != nil {
		return model.Job{}, err
	}
	job, err := adapter.GetJobGraph(ctx, jobID)
	if err != nil {
		return model.Job{}, err
	}
	cfg, err := adapter.GetConfig(ctx, jobID)
	if err != nil {
		return model.Job{}, err
	}
	job.Config = cfg
	return sampleSubtasks(ctx, adapter, job), nil
}

// sampleSubtasks enriches a live-fetched job with a single read of each
// operator's subtask metrics, so backpressure-based checks have data. It is
// best-effort: an operator whose metrics are unavailable keeps no subtasks.
func sampleSubtasks(ctx context.Context, a adapters.PlatformAdapter, job model.Job) model.Job {
	for i, op := range job.Operators {
		if subs, err := a.GetSubtaskMetrics(ctx, job.ID, op.ID); err == nil {
			job.Operators[i].Subtasks = subs
		}
	}
	return job
}

// resolveAllJobs returns every job to diagnose: all jobs in the inventory file
// when it exists, otherwise every job the adapter lists, enriched with its
// graph and config.
func resolveAllJobs(ctx context.Context, opts *GlobalOptions, sourceFlag, inventoryPath string) ([]model.Job, error) {
	if _, err := os.Stat(inventoryPath); err == nil {
		inv, err := readInventoryFile(inventoryPath)
		if err != nil {
			return nil, err
		}
		return inv.Jobs, nil
	}

	source, acfg, err := resolveSource(opts.Profile, sourceFlag, opts.Via)
	if err != nil {
		return nil, err
	}
	adapter, err := adapters.Default.New(source, acfg)
	if err != nil {
		return nil, err
	}
	listed, err := adapter.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	jobs := make([]model.Job, 0, len(listed))
	for _, j := range listed {
		graph, err := adapter.GetJobGraph(ctx, j.ID)
		if err != nil {
			return nil, fmt.Errorf("job %s: graph: %w", j.ID, err)
		}
		cfg, err := adapter.GetConfig(ctx, j.ID)
		if err != nil {
			return nil, fmt.Errorf("job %s: config: %w", j.ID, err)
		}
		graph.Config = cfg
		jobs = append(jobs, sampleSubtasks(ctx, adapter, graph))
	}
	return jobs, nil
}

// fetchMetrics returns the job's live metrics, best-effort. When no source or
// profile is configured (fully offline), it returns nil and metric-based checks
// degrade to silence.
func fetchMetrics(ctx context.Context, opts *GlobalOptions, jobID, sourceFlag string) map[string]float64 {
	source, acfg, err := resolveSource(opts.Profile, sourceFlag, opts.Via)
	if err != nil {
		return nil
	}
	adapter, err := adapters.Default.New(source, acfg)
	if err != nil {
		return nil
	}
	m, err := adapter.GetMetrics(ctx, jobID)
	if err != nil {
		return nil
	}
	return m
}

func newDoctorEngine() (*rules.Engine, error) {
	ruleSet, err := builtin.Rules()
	if err != nil {
		return nil, err
	}
	return rules.NewEngine(ruleSet, rules.DefaultChecks.All())
}

func readInventoryFile(path string) (model.Inventory, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Inventory{}, err
	}
	defer f.Close()
	return model.ReadInventory(f)
}
