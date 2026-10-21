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
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters/fake"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// TestSourcesRegistered confirms the in-core adapters are wired into the
// default registry via the CLI's blank imports.
func TestSourcesRegistered(t *testing.T) {
	names := adapters.Default.Names()
	has := func(n string) bool {
		for _, x := range names {
			if x == n {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"oss", "k8s"} {
		if !has(want) {
			t.Errorf("adapter %q not registered; have %v", want, names)
		}
	}
}

func TestApplyNamespace(t *testing.T) {
	var cfg adapters.Config
	applyNamespace(&cfg, "flink")
	if cfg.Options["namespace"] != "flink" {
		t.Errorf("namespace override not applied: %+v", cfg)
	}

	var empty adapters.Config
	applyNamespace(&empty, "")
	if empty.Options != nil {
		t.Errorf("empty namespace should not initialize options: %+v", empty)
	}
}

func TestApplyVia(t *testing.T) {
	var cfg adapters.Config
	applyVia(&cfg, "proxy")
	if cfg.Options["via"] != "proxy" {
		t.Errorf("via override not applied: %+v", cfg)
	}

	var empty adapters.Config
	applyVia(&empty, "")
	if empty.Options != nil {
		t.Errorf("empty via should not initialize options: %+v", empty)
	}
}

func TestBuildInventoryWithFake(t *testing.T) {
	src := model.NewInventory()
	src.Jobs = []model.Job{{
		ID:        "job-1",
		Name:      "demo",
		Operators: []model.Operator{{ID: "op1", Name: "Source"}},
		Config:    map[string]string{"state.backend": "rocksdb"},
	}}
	src.Statements = []model.Statement{{Name: "s1", SQL: "SELECT 1"}}

	got, err := buildInventory(context.Background(), fake.New("fake", src))
	if err != nil {
		t.Fatalf("buildInventory: %v", err)
	}
	if len(got.Jobs) != 1 || len(got.Jobs[0].Operators) != 1 {
		t.Fatalf("jobs/operators = %+v, want one job with one operator", got.Jobs)
	}
	if got.Jobs[0].Config["state.backend"] != "rocksdb" {
		t.Errorf("config not carried through: %v", got.Jobs[0].Config)
	}
	if len(got.Statements) != 1 {
		t.Errorf("statements = %+v, want 1", got.Statements)
	}
}

// ossServer serves a minimal recorded Flink REST surface for the scan slice.
func ossServer(t *testing.T) *httptest.Server {
	t.Helper()
	responses := map[string]string{
		"/jobs/overview":  `{"jobs":[{"jid":"j1","name":"demo","state":"RUNNING"}]}`,
		"/jobs/j1":        `{"jid":"j1","name":"demo","state":"RUNNING","vertices":[{"id":"v1","name":"Source","parallelism":2}],"plan":{"nodes":[{"id":"v1"}]}}`,
		"/jobs/j1/config": `{"jid":"j1","name":"demo","execution-config":{"job-parallelism":2,"user-config":{"state.backend":"hashmap"}}}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// writeProfile writes a temp config with one profile pointed at endpoint and
// makes it the active config via FLINKIT_CONFIG.
func writeProfile(t *testing.T, endpoint string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "profiles:\n  it:\n    source: oss\n    endpoint: " + endpoint + "\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	t.Setenv("FLINKIT_CONFIG", path)
}

func TestScanEndToEndTerminal(t *testing.T) {
	writeProfile(t, ossServer(t).URL)
	outPath := filepath.Join(t.TempDir(), "inventory.json")

	root := newRootCommand("test")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"scan", "--profile", "it", "--out", outPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("scan: %v\nstderr: %s", err, stderr.String())
	}

	// The inventory file must exist and decode with the discovered job.
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("opening inventory: %v", err)
	}
	defer f.Close()
	inv, err := model.ReadInventory(f)
	if err != nil {
		t.Fatalf("decoding inventory: %v", err)
	}
	if inv.Source != "oss" || inv.GeneratedAt == "" {
		t.Errorf("inventory meta = source %q, generatedAt %q", inv.Source, inv.GeneratedAt)
	}
	if len(inv.Jobs) != 1 || inv.Jobs[0].ID != "j1" {
		t.Fatalf("jobs = %+v, want one job j1", inv.Jobs)
	}
	if len(inv.Jobs[0].Operators) != 1 || inv.Jobs[0].Config["state.backend"] != "hashmap" {
		t.Errorf("job graph/config not populated: %+v", inv.Jobs[0])
	}

	// Terminal output carries the job, and the write note went to stderr.
	if out := stdout.String(); !strings.Contains(out, "demo") || !strings.Contains(out, "j1") {
		t.Errorf("terminal output missing job:\n%s", out)
	}
	if !strings.Contains(stderr.String(), "wrote "+outPath) {
		t.Errorf("stderr missing write note:\n%s", stderr.String())
	}
}

func TestScanEndToEndJSON(t *testing.T) {
	writeProfile(t, ossServer(t).URL)
	outPath := filepath.Join(t.TempDir(), "inventory.json")

	root := newRootCommand("test")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"scan", "--profile", "it", "--out", outPath, "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("scan --json: %v\nstderr: %s", err, stderr.String())
	}

	// Stdout must be clean JSON (the write note is on stderr) that decodes back.
	inv, err := model.ReadInventory(bytes.NewReader(stdout.Bytes()))
	if err != nil {
		t.Fatalf("stdout is not a valid inventory JSON: %v\n%s", err, stdout.String())
	}
	if len(inv.Jobs) != 1 || inv.Jobs[0].ID != "j1" {
		t.Errorf("json inventory = %+v, want one job j1", inv.Jobs)
	}
}

func TestScanNoSourceErrors(t *testing.T) {
	// Empty config, no --source: scan has nothing to talk to.
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\n"), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	t.Setenv("FLINKIT_CONFIG", path)

	root := newRootCommand("test")
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"scan"})

	if err := root.Execute(); err == nil {
		t.Fatal("expected an error when no source or profile is available")
	}
}
