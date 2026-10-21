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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

// writeInventoryFile writes an inventory with one job and returns its path.
func writeInventoryFile(t *testing.T, job model.Job) string {
	t.Helper()
	return writeInventoryJobs(t, job)
}

// writeInventoryJobs writes an inventory with the given jobs and returns its path.
func writeInventoryJobs(t *testing.T, jobs ...model.Job) string {
	t.Helper()
	inv := model.NewInventory()
	inv.Source = "oss"
	inv.Jobs = jobs

	path := filepath.Join(t.TempDir(), "inventory.json")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create inventory: %v", err)
	}
	defer f.Close()
	if err := inv.WriteJSON(f); err != nil {
		t.Fatalf("write inventory: %v", err)
	}
	return path
}

// cleanJob has a uid and no state, so it trips no doctor rules.
func cleanJob(id string) model.Job {
	return model.Job{ID: id, Name: id + "-name", Operators: []model.Operator{
		{ID: "op", Name: "op", UID: "op"},
	}}
}

// dirtyJob trips both DOC-HYG-001 (no uid) and DOC-STATE-001 (state, no ttl).
func dirtyJob() model.Job {
	return model.Job{
		ID:   "job-1",
		Name: "demo",
		Operators: []model.Operator{{
			ID:    "op-agg",
			Name:  "Window Agg",
			State: []model.StateDescriptor{{Name: "windowContents"}},
		}},
	}
}

func runDoctorCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCommand("test")
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"doctor"}, args...))
	err = root.Execute()
	return out.String(), errb.String(), err
}

func TestDoctorFromInventoryTerminal(t *testing.T) {
	inv := writeInventoryFile(t, dirtyJob())
	out, _, err := runDoctorCmd(t, "job-1", "--inventory", inv)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	for _, want := range []string{"DOC-HYG-001", "DOC-STATE-001", "job-1/op-agg"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorJSON(t *testing.T) {
	inv := writeInventoryFile(t, dirtyJob())
	out, _, err := runDoctorCmd(t, "job-1", "--inventory", inv, "--json")
	if err != nil {
		t.Fatalf("doctor --json: %v", err)
	}
	var got struct {
		Findings rules.Findings `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout not findings JSON: %v\n%s", err, out)
	}
	if len(got.Findings) < 2 {
		t.Errorf("want >= 2 findings, got %d", len(got.Findings))
	}
}

func TestDoctorFailOnGate(t *testing.T) {
	inv := writeInventoryFile(t, dirtyJob())

	// A high finding exists (DOC-STATE-001), so --fail-on high must error...
	out, _, err := runDoctorCmd(t, "job-1", "--inventory", inv, "--fail-on", "high")
	if err == nil {
		t.Error("--fail-on high should exit non-zero when a high finding exists")
	}
	// ...but the findings are still rendered to stdout.
	if !strings.Contains(out, "DOC-STATE-001") {
		t.Errorf("findings should still print on gate trip:\n%s", out)
	}
}

func TestDoctorFailOnClean(t *testing.T) {
	clean := model.Job{ID: "job-1", Name: "demo", Operators: []model.Operator{
		{ID: "op", Name: "op", UID: "op"}, // has uid, no state
	}}
	inv := writeInventoryFile(t, clean)
	if _, _, err := runDoctorCmd(t, "job-1", "--inventory", inv, "--fail-on", "high"); err != nil {
		t.Errorf("clean job should pass the gate, got: %v", err)
	}
}

func TestDoctorJobNotInInventory(t *testing.T) {
	inv := writeInventoryFile(t, dirtyJob())
	if _, _, err := runDoctorCmd(t, "missing-job", "--inventory", inv); err == nil {
		t.Error("expected error for a job not in the inventory")
	}
}

func TestDoctorFleetTerminal(t *testing.T) {
	inv := writeInventoryJobs(t, dirtyJob(), cleanJob("job-2"))
	out, _, err := runDoctorCmd(t, "--all", "--inventory", inv)
	if err != nil {
		t.Fatalf("doctor --all: %v", err)
	}
	for _, want := range []string{"Fleet: 2 job(s)", "demo", "DOC-STATE-001", "job-2-name", "(healthy)"} {
		if !strings.Contains(out, want) {
			t.Errorf("fleet output missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorFleetJSON(t *testing.T) {
	inv := writeInventoryJobs(t, dirtyJob(), cleanJob("job-2"))
	out, _, err := runDoctorCmd(t, "--all", "--inventory", inv, "--json")
	if err != nil {
		t.Fatalf("doctor --all --json: %v", err)
	}
	var got struct {
		Jobs []struct {
			JobID    string         `json:"jobId"`
			Findings rules.Findings `json:"findings"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout not fleet JSON: %v\n%s", err, out)
	}
	if len(got.Jobs) != 2 {
		t.Fatalf("want 2 jobs, got %d", len(got.Jobs))
	}
}

func TestDoctorFleetFailOn(t *testing.T) {
	inv := writeInventoryJobs(t, dirtyJob(), cleanJob("job-2"))
	if _, _, err := runDoctorCmd(t, "--all", "--inventory", inv, "--fail-on", "high"); err == nil {
		t.Error("--all --fail-on high should trip on the dirty job")
	}
}

func TestDoctorAllWithJobArgErrors(t *testing.T) {
	inv := writeInventoryFile(t, dirtyJob())
	if _, _, err := runDoctorCmd(t, "job-1", "--all", "--inventory", inv); err == nil {
		t.Error("combining --all with a job arg should error")
	}
}

// TestDoctorLiveFetch covers the no-inventory path: doctor fetches the job from
// the OSS adapter via a profile and runs cleanly. The REST adapter exposes no
// operator state or UIDs, so DOC-HYG-001 must NOT false-positive here.
func TestDoctorLiveFetch(t *testing.T) {
	writeProfile(t, ossServer(t).URL)
	missing := filepath.Join(t.TempDir(), "none.json") // does not exist -> live fetch

	out, _, err := runDoctorCmd(t, "j1", "--profile", "it", "--inventory", missing)
	if err != nil {
		t.Fatalf("doctor live: %v", err)
	}
	if strings.Contains(out, "DOC-HYG-001") {
		t.Errorf("DOC-HYG-001 should not fire via REST (no state/uid visibility):\n%s", out)
	}
}

// ossServerWithMetrics serves the job surface plus checkpoint and metric
// endpoints, with a failed checkpoint so DOC-CKPT-001 fires.
func ossServerWithMetrics(t *testing.T) *httptest.Server {
	t.Helper()
	responses := map[string]string{
		"/jobs/j1":             `{"jid":"j1","name":"demo","state":"RUNNING","vertices":[{"id":"v1","name":"Source","parallelism":1}],"plan":{"nodes":[{"id":"v1"}]}}`,
		"/jobs/j1/config":      `{"jid":"j1","name":"demo","execution-config":{"user-config":{}}}`,
		"/jobs/j1/checkpoints": `{"counts":{"completed":5,"failed":1}}`,
		"/jobs/j1/metrics":     `[{"id":"numRestarts","value":"0"}]`,
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

// TestDoctorBackpressureLive covers DOC-BP-001: doctor samples subtask metrics
// live, and the backpressured source trips the backpressure rule.
func TestDoctorBackpressureLive(t *testing.T) {
	writeProfile(t, skewServer(t).URL)
	missing := filepath.Join(t.TempDir(), "none.json")

	out, _, err := runDoctorCmd(t, "j1", "--profile", "it", "--inventory", missing)
	if err != nil {
		t.Fatalf("doctor live: %v", err)
	}
	if !strings.Contains(out, "DOC-BP-001") {
		t.Errorf("backpressured source should trip DOC-BP-001:\n%s", out)
	}
}

// TestDoctorLiveMetrics covers the live-metric path: doctor fetches metrics and
// a failed checkpoint trips DOC-CKPT-001.
func TestDoctorLiveMetrics(t *testing.T) {
	writeProfile(t, ossServerWithMetrics(t).URL)
	missing := filepath.Join(t.TempDir(), "none.json")

	out, _, err := runDoctorCmd(t, "j1", "--profile", "it", "--inventory", missing)
	if err != nil {
		t.Fatalf("doctor live metrics: %v", err)
	}
	if !strings.Contains(out, "DOC-CKPT-001") {
		t.Errorf("failed checkpoint should trip DOC-CKPT-001:\n%s", out)
	}
}
