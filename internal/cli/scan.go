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
	"time"

	"github.com/spf13/cobra"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/config"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/tui"
)

func newScanCommand(opts *GlobalOptions) *cobra.Command {
	var (
		source    string
		out       string
		namespace string
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Discover jobs, statements, configs, and connectors into an inventory",
		Long: `Scan discovers jobs, SQL statements, compute pools, configs, connectors,
catalogs, and Flink versions from a platform or local repo, normalizes them
into the Flinkit Model, and writes .flinkit/inventory.json. Every other
command reads this inventory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(cmd, opts, source, out, namespace)
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "source to scan: cc|cp|oss|k8s|repo")
	cmd.Flags().StringVar(&out, "out", filepath.Join(".flinkit", "inventory.json"), "path to write the inventory")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace (for --source k8s; overrides the profile)")

	return cmd
}

func runScan(cmd *cobra.Command, opts *GlobalOptions, sourceFlag, outPath, namespace string) error {
	format, err := report.Resolve(opts.JSON, opts.Format)
	if err != nil {
		return err
	}

	source, acfg, err := resolveSource(opts.Profile, sourceFlag, opts.Via)
	if err != nil {
		return err
	}
	applyNamespace(&acfg, namespace)

	adapter, err := adapters.Default.New(source, acfg)
	if err != nil {
		return err
	}

	inv, err := buildInventory(cmd.Context(), adapter)
	if err != nil {
		return err
	}
	inv.Source = source
	inv.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	if err := writeInventory(outPath, inv); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (%d jobs)\n", outPath, len(inv.Jobs))

	// Browsable job table by default in a terminal; plain/JSON otherwise.
	if format == report.FormatTerminal && tui.IsTerminal(os.Stdout) {
		return tui.Run(tui.NewInventory(inv))
	}

	renderer, err := report.For(format)
	if err != nil {
		return err
	}
	return renderer.RenderInventory(cmd.OutOrStdout(), inv)
}

// applyNamespace injects a --namespace override into the adapter config's
// options (used by the k8s adapter; ignored by others).
func applyNamespace(acfg *adapters.Config, namespace string) {
	if namespace == "" {
		return
	}
	if acfg.Options == nil {
		acfg.Options = map[string]string{}
	}
	acfg.Options["namespace"] = namespace
}

// resolveSource determines which adapter to use and the config to build it
// with. An explicit --source wins; otherwise the resolved profile supplies
// both the source and the connection config. With neither, it is an error. A
// non-empty via overrides the k8s transport (ignored by other adapters).
func resolveSource(profileName, sourceFlag, via string) (string, adapters.Config, error) {
	cfg, err := config.LoadDefault()
	if err != nil {
		return "", adapters.Config{}, err
	}

	var (
		source string
		acfg   adapters.Config
	)
	profile, perr := cfg.Resolve(profileName)
	if perr == nil {
		source = profile.Source
		acfg = profile.AdapterConfig()
	}
	if sourceFlag != "" {
		source = sourceFlag
	}
	if source == "" {
		return "", adapters.Config{}, fmt.Errorf("scan: no source: pass --source or configure a profile (%v)", perr)
	}
	applyVia(&acfg, via)
	return source, acfg, nil
}

// applyVia injects a --via transport override into the adapter config's options
// (used by the k8s adapter; ignored by others).
func applyVia(acfg *adapters.Config, via string) {
	if via == "" {
		return
	}
	if acfg.Options == nil {
		acfg.Options = map[string]string{}
	}
	acfg.Options["via"] = via
}

// buildInventory walks an adapter and normalizes what it finds into the model.
func buildInventory(ctx context.Context, a adapters.PlatformAdapter) (model.Inventory, error) {
	inv := model.NewInventory()

	jobs, err := a.ListJobs(ctx)
	if err != nil {
		return inv, fmt.Errorf("listing jobs: %w", err)
	}
	for _, j := range jobs {
		graph, err := a.GetJobGraph(ctx, j.ID)
		if err != nil {
			return inv, fmt.Errorf("job %s: graph: %w", j.ID, err)
		}
		cfg, err := a.GetConfig(ctx, j.ID)
		if err != nil {
			return inv, fmt.Errorf("job %s: config: %w", j.ID, err)
		}
		graph.Config = cfg
		inv.Jobs = append(inv.Jobs, graph)
	}

	stmts, err := a.ListStatements(ctx)
	if err != nil {
		return inv, fmt.Errorf("listing statements: %w", err)
	}
	inv.Statements = stmts

	return inv, nil
}

// writeInventory writes the inventory as JSON, creating the parent directory.
func writeInventory(path string, inv model.Inventory) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	if err := inv.WriteJSON(f); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", path, err)
	}
	return nil
}
