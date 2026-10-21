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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters/fake"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
)

// skewServer serves a job v-src → v-agg where v-src is backpressured and v-agg
// is the choke point with a hot subtask 1 (skew).
func skewServer(t *testing.T) *httptest.Server {
	t.Helper()
	responses := map[string]string{
		"/jobs/j1": `{"jid":"j1","name":"demo","state":"RUNNING",
			"vertices":[{"id":"v-src","name":"Source","parallelism":1},{"id":"v-agg","name":"Window Agg","parallelism":2}],
			"plan":{"nodes":[{"id":"v-agg","inputs":[{"id":"v-src"}]},{"id":"v-src"}]}}`,
		"/jobs/j1/config":                            `{"jid":"j1","name":"demo","execution-config":{"user-config":{}}}`,
		"/jobmanager/log":                            "2026-10-16 10:00:00 INFO  job started\n2026-10-16 10:01:00 WARN  checkpoint slow\n2026-10-16 10:02:00 ERROR something bad happened\n",
		"/jobs/j1/vertices/v-src":                    `{"id":"v-src","name":"Source","subtasks":[{"subtask":0,"metrics":{}}]}`,
		"/jobs/j1/vertices/v-agg":                    `{"id":"v-agg","name":"Window Agg","subtasks":[{"subtask":0,"metrics":{}},{"subtask":1,"metrics":{}}]}`,
		"/jobs/j1/vertices/v-src/subtasks/0/metrics": `[{"id":"busyTimeMsPerSecond","value":"900.0"},{"id":"backPressuredTimeMsPerSecond","value":"800.0"},{"id":"idleTimeMsPerSecond","value":"0.0"}]`,
		"/jobs/j1/vertices/v-agg/subtasks/0/metrics": `[{"id":"busyTimeMsPerSecond","value":"300.0"},{"id":"backPressuredTimeMsPerSecond","value":"20.0"},{"id":"idleTimeMsPerSecond","value":"680.0"}]`,
		"/jobs/j1/vertices/v-agg/subtasks/1/metrics": `[{"id":"busyTimeMsPerSecond","value":"980.0"},{"id":"backPressuredTimeMsPerSecond","value":"30.0"},{"id":"idleTimeMsPerSecond","value":"0.0"}]`,
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

func runBottleneckCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCommand("test")
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"bottleneck"}, args...))
	err = root.Execute()
	return out.String(), errb.String(), err
}

func TestBottleneckSkewEndToEnd(t *testing.T) {
	writeProfile(t, skewServer(t).URL)
	// A tiny window keeps the three samples' sleeps negligible.
	out, _, err := runBottleneckCmd(t, "j1", "--profile", "it", "--window", "3ms")
	if err != nil {
		t.Fatalf("bottleneck: %v", err)
	}
	for _, want := range []string{"Window Agg", "v-agg", "key-skew"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestBottleneckJSON(t *testing.T) {
	writeProfile(t, skewServer(t).URL)
	out, _, err := runBottleneckCmd(t, "j1", "--profile", "it", "--window", "3ms", "--json")
	if err != nil {
		t.Fatalf("bottleneck --json: %v", err)
	}
	var rep report.BottleneckReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("stdout not a bottleneck report: %v\n%s", err, out)
	}
	if !rep.Found || rep.OperatorID != "v-agg" || rep.Cause != "key-skew" || rep.WorstSubtask != 1 {
		t.Errorf("report = %+v, want found v-agg key-skew subtask 1", rep)
	}
}

// TestSampleJobAverages checks the windowed averaging: two samples of the same
// rates average back to those rates, keyed per subtask.
func TestSampleJobAverages(t *testing.T) {
	inv := model.NewInventory()
	inv.Jobs = []model.Job{{
		ID: "job-1",
		Operators: []model.Operator{{
			ID: "op", Name: "op",
			Subtasks: []model.Subtask{
				{Index: 0, BusyMsPerSec: 200},
				{Index: 1, BusyMsPerSec: 980},
			},
		}},
	}}
	a := fake.New("fake", inv)

	job, err := sampleJob(context.Background(), a, "job-1", 2, 0, func(time.Duration) {})
	if err != nil {
		t.Fatalf("sampleJob: %v", err)
	}
	if len(job.Operators) != 1 || len(job.Operators[0].Subtasks) != 2 {
		t.Fatalf("job = %+v", job)
	}
	subs := job.Operators[0].Subtasks
	if subs[0].BusyMsPerSec != 200 || subs[1].BusyMsPerSec != 980 {
		t.Errorf("averaged busy = %v / %v, want 200 / 980", subs[0].BusyMsPerSec, subs[1].BusyMsPerSec)
	}
}
