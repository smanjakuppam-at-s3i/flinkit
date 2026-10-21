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

package oss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// fixtureServer serves recorded Flink REST responses from testdata by path.
func fixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	routes := map[string]string{
		"/jobs/overview":              "jobs_overview.json",
		"/jobs/job-1":                 "job_detail.json",
		"/jobs/job-1/config":          "job_config.json",
		"/jobs/job-1/vertices/op-agg": "vertex_detail.json",
		"/jobs/job-1/vertices/op-agg/subtasks/0/metrics": "subtask_0_metrics.json",
		"/jobs/job-1/vertices/op-agg/subtasks/1/metrics": "subtask_1_metrics.json",
		"/jobs/job-1/vertices/op-agg/subtasks/2/metrics": "subtask_2_metrics.json",
		"/jobs/job-1/checkpoints":                        "checkpoints.json",
		"/jobs/job-1/metrics":                            "job_metrics.json",
		"/config":                                        "cluster_config.json",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		b, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Errorf("reading fixture %s: %v", file, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newAdapter(t *testing.T, endpoint string) adapters.PlatformAdapter {
	t.Helper()
	a, err := New(adapters.Config{Endpoint: endpoint})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func TestListJobs(t *testing.T) {
	a := newAdapter(t, fixtureServer(t).URL)
	jobs, err := a.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	if jobs[0].ID != "job-1" || jobs[0].Platform != model.PlatformOSS || jobs[0].State != model.JobStateRunning {
		t.Errorf("job = %+v, want job-1/oss/RUNNING", jobs[0])
	}
}

func TestGetJob(t *testing.T) {
	a := newAdapter(t, fixtureServer(t).URL)
	job, err := a.GetJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.Name != "orders-enrichment" {
		t.Errorf("name = %q, want orders-enrichment", job.Name)
	}
	if job.Operators != nil {
		t.Errorf("GetJob should not populate the operator graph")
	}
}

func TestGetJobGraph(t *testing.T) {
	a := newAdapter(t, fixtureServer(t).URL)
	job, err := a.GetJobGraph(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetJobGraph: %v", err)
	}
	if len(job.Operators) != 2 {
		t.Fatalf("got %d operators, want 2", len(job.Operators))
	}
	if job.FlinkVersion.String() != "1.20.1" {
		t.Errorf("FlinkVersion = %q, want 1.20.1 (from /config)", job.FlinkVersion.String())
	}

	byID := map[string]model.Operator{}
	for _, op := range job.Operators {
		byID[op.ID] = op
	}
	agg, ok := byID["op-agg"]
	if !ok {
		t.Fatal("missing op-agg")
	}
	if len(agg.Upstreams) != 1 || agg.Upstreams[0] != "op-src" {
		t.Errorf("op-agg upstreams = %v, want [op-src]", agg.Upstreams)
	}
	if src := byID["op-src"]; len(src.Upstreams) != 0 {
		t.Errorf("op-src upstreams = %v, want none (source)", src.Upstreams)
	}
}

func TestGetSubtaskMetrics(t *testing.T) {
	a := newAdapter(t, fixtureServer(t).URL)
	subs, err := a.GetSubtaskMetrics(context.Background(), "job-1", "op-agg")
	if err != nil {
		t.Fatalf("GetSubtaskMetrics: %v", err)
	}
	if len(subs) != 3 {
		t.Fatalf("got %d subtasks, want 3", len(subs))
	}
	if subs[2].Index != 2 || subs[2].RecordsIn != 600000 || subs[2].RecordsOut != 360000 {
		t.Errorf("subtask[2] = %+v, want index 2 with 600000/360000 records", subs[2])
	}
}

func TestGetSubtaskMetricsRates(t *testing.T) {
	a := newAdapter(t, fixtureServer(t).URL)
	subs, err := a.GetSubtaskMetrics(context.Background(), "job-1", "op-agg")
	if err != nil {
		t.Fatalf("GetSubtaskMetrics: %v", err)
	}
	if len(subs) != 3 {
		t.Fatalf("got %d subtasks, want 3", len(subs))
	}
	// Subtask 2 is the hot one; records still come from the vertex detail.
	hot := subs[2]
	if hot.Index != 2 || hot.BusyMsPerSec != 980 || hot.BackpressuredMsPerSec != 50 || hot.IdleMsPerSec != 20 {
		t.Errorf("subtask[2] rates = %+v, want busy 980 / bp 50 / idle 20", hot)
	}
	if hot.RecordsIn != 600000 {
		t.Errorf("subtask[2] RecordsIn = %d, want 600000 (from vertex detail)", hot.RecordsIn)
	}
	if subs[0].BusyMsPerSec != 250 {
		t.Errorf("subtask[0] busy = %v, want 250", subs[0].BusyMsPerSec)
	}
}

func TestGetConfig(t *testing.T) {
	a := newAdapter(t, fixtureServer(t).URL)
	cfg, err := a.GetConfig(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg["state.backend"] != "rocksdb" {
		t.Errorf("config = %v, want state.backend=rocksdb", cfg)
	}
}

func TestGetMetrics(t *testing.T) {
	a := newAdapter(t, fixtureServer(t).URL)
	m, err := a.GetMetrics(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if m["checkpoints.completed"] != 10 || m["checkpoints.failed"] != 2 {
		t.Errorf("checkpoint metrics = %v, want completed 10 / failed 2", m)
	}
	if m["numRestarts"] != 3 {
		t.Errorf("numRestarts = %v, want 3", m["numRestarts"])
	}
}

// TestGetMetricsBestEffort checks that missing metric endpoints are tolerated:
// GetMetrics returns what it can rather than failing.
func TestGetMetricsBestEffort(t *testing.T) {
	// A server that serves nothing — every metric endpoint 404s.
	a := newAdapter(t, fixtureServer(t).URL)
	m, err := a.GetMetrics(context.Background(), "job-does-not-exist")
	if err != nil {
		t.Fatalf("GetMetrics should not fail on missing endpoints: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty metrics for unknown job, got %v", m)
	}
}

func TestErrorOnNon200(t *testing.T) {
	a := newAdapter(t, fixtureServer(t).URL)
	if _, err := a.GetJob(context.Background(), "unknown-job"); err == nil {
		t.Fatal("expected error for a 404 job, got nil")
	}
}

// countingTransport records how many requests flow through it, proving an
// injected client is the one actually used.
type countingTransport struct {
	rt    http.RoundTripper
	calls int
}

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.calls++
	return c.rt.RoundTrip(r)
}

// TestNewWithClientReusesTransport confirms the REST logic runs over an injected
// client — the seam the k8s adapter uses to reach a JobManager through the
// apiserver proxy or service DNS.
func TestNewWithClientReusesTransport(t *testing.T) {
	srv := fixtureServer(t)
	ct := &countingTransport{rt: http.DefaultTransport}
	a := NewWithClient(srv.URL, &http.Client{Transport: ct})

	if _, err := a.GetJob(context.Background(), "job-1"); err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if ct.calls == 0 {
		t.Error("injected client saw no calls; REST logic is not using it")
	}
}

// TestNewWithClientNilClientDefaults confirms a nil client still yields a usable
// adapter and that a trailing slash on the base URL is trimmed.
func TestNewWithClientNilClientDefaults(t *testing.T) {
	srv := fixtureServer(t)
	a := NewWithClient(srv.URL+"/", nil)
	if _, err := a.GetJob(context.Background(), "job-1"); err != nil {
		t.Fatalf("GetJob with defaulted client: %v", err)
	}
}

// TestRegisteredInDefaultRegistry confirms init wired the adapter under "oss".
func TestRegisteredInDefaultRegistry(t *testing.T) {
	a, err := adapters.Default.New("oss", adapters.Config{})
	if err != nil {
		t.Fatalf("default registry New(oss): %v", err)
	}
	if a.Name() != "oss" {
		t.Errorf("Name = %q, want oss", a.Name())
	}
}
