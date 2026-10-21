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
	"fmt"

	"github.com/spf13/cobra"
)

// GlobalOptions holds the flags shared by every command. A single instance is
// bound to the root command's persistent flags and passed to subcommands.
type GlobalOptions struct {
	Profile string // --profile: named connection profile from ~/.flinkit/config.yaml
	JSON    bool   // --json: emit machine-readable JSON
	Format  string // --format: terminal|json|md|html
	Verbose bool   // --verbose
	Apply   bool   // --apply: mutate instead of printing a plan (mutating commands only)
	Via     string // --via: k8s JobManager REST transport: auto|direct|proxy|port-forward
}

// Execute builds the root command, runs it, and returns a process exit code.
func Execute(version string) int {
	if err := newRootCommand(version).Execute(); err != nil {
		return 1
	}
	return 0
}

func newRootCommand(version string) *cobra.Command {
	opts := &GlobalOptions{}

	root := &cobra.Command{
		Use:   "flinkit",
		Short: "Diagnose, assess, migrate, and generate Apache Flink workloads",
		Long: `Flinkit is a vendor-neutral CLI for running Apache Flink in production.

It diagnoses running jobs for health and performance, assesses portability
across platforms (Confluent Cloud, Confluent Platform, open-source) and
versions, migrates code to Flink and proves equivalence through replay, and
generates new jobs from templates.`,
		Version: version,
		// Print the error but not the usage text on a runtime failure; usage is
		// only helpful for argument/flag mistakes, which cobra still shows.
		SilenceUsage: true,
	}

	f := root.PersistentFlags()
	f.StringVar(&opts.Profile, "profile", "", "connection profile from ~/.flinkit/config.yaml")
	f.BoolVar(&opts.JSON, "json", false, "emit machine-readable JSON output")
	f.StringVar(&opts.Format, "format", "terminal", "output format: terminal|json|md|html")
	f.BoolVarP(&opts.Verbose, "verbose", "v", false, "verbose output")
	f.BoolVar(&opts.Apply, "apply", false, "apply changes instead of printing a plan (mutating commands only)")
	f.StringVar(&opts.Via, "via", "", "k8s JobManager REST transport: auto|direct|proxy|port-forward (for --source k8s)")

	root.AddCommand(
		newScanCommand(opts),
		newDoctorCommand(opts),
		newBottleneckCommand(opts),
		newInspectCommand(opts),
		newVersionCommand(version),
	)

	return root
}

// notImplemented is the placeholder action for scaffolded commands. It writes
// to the command's configured output so tests can capture it.
func notImplemented(cmd *cobra.Command, name string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "flinkit %s: not implemented yet (scaffold)\n", name)
	return nil
}
