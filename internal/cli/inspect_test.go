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
	"strings"
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters/fake"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/report"
)

// TestInspectFusedIO checks a single fused source→sink operator yields ONE I/O
// endpoint labeled "source+sink", not two.
func TestInspectFusedIO(t *testing.T) {
	inv := model.NewInventory()
	inv.Jobs = []model.Job{{
		ID:   "job-1",
		Name: "fused",
		Operators: []model.Operator{{
			ID:   "op",
			Name: "Source -> Writer -> Committer",
			Subtasks: []model.Subtask{
				{Index: 0, RecordsIn: 100, RecordsOut: 100},
			},
		}},
	}}

	snap, err := buildSnapshot(context.Background(), fake.New("fake", inv), "job-1")
	if err != nil {
		t.Fatalf("buildSnapshot: %v", err)
	}
	if len(snap.IO) != 1 {
		t.Fatalf("I/O endpoints = %d, want 1 (deduped)", len(snap.IO))
	}
	if snap.IO[0].Direction != "source+sink" {
		t.Errorf("direction = %q, want source+sink", snap.IO[0].Direction)
	}
}

// TestInspectSnapshotFindings confirms buildSnapshot runs the health rules so
// the cockpit's findings panel has real data.
func TestInspectSnapshotFindings(t *testing.T) {
	inv := model.NewInventory()
	inv.Jobs = []model.Job{{
		ID:   "job-1",
		Name: "demo",
		Operators: []model.Operator{{
			ID:    "op-agg",
			Name:  "Window Agg",
			State: []model.StateDescriptor{{Name: "windowContents"}}, // stateful, no uid, no ttl
		}},
	}}

	snap, err := buildSnapshot(context.Background(), fake.New("fake", inv), "job-1")
	if err != nil {
		t.Fatalf("buildSnapshot: %v", err)
	}
	if len(snap.Findings) == 0 {
		t.Fatal("expected findings from the health rules")
	}
	var hasHyg bool
	for _, f := range snap.Findings {
		if f.RuleID == "DOC-HYG-001" {
			hasHyg = true
		}
	}
	if !hasHyg {
		t.Errorf("expected DOC-HYG-001 (stateful operator without uid): %+v", snap.Findings)
	}
}

func runInspectCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCommand("test")
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"inspect"}, args...))
	err = root.Execute()
	return out.String(), errb.String(), err
}

// inspect uses the same skewServer as bottleneck: v-src → v-agg with a hot
// subtask on v-agg, so inspect should show both operators, both I/O endpoints,
// and the located bottleneck.
func TestInspectTerminal(t *testing.T) {
	writeProfile(t, skewServer(t).URL)
	out, _, err := runInspectCmd(t, "j1", "--profile", "it")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	for _, want := range []string{"Job:", "demo", "Operators:", "Window Agg", "I/O:", "source", "sink", "key-skew"} {
		if !strings.Contains(out, want) {
			t.Errorf("inspect output missing %q:\n%s", want, out)
		}
	}
}

func TestInspectLogs(t *testing.T) {
	writeProfile(t, skewServer(t).URL)
	out, _, err := runInspectCmd(t, "j1", "--profile", "it")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !strings.Contains(out, "Recent logs:") || !strings.Contains(out, "something bad happened") {
		t.Errorf("inspect output missing JobManager logs:\n%s", out)
	}

	// JSON carries the log lines too.
	jout, _, err := runInspectCmd(t, "j1", "--profile", "it", "--json")
	if err != nil {
		t.Fatalf("inspect --json: %v", err)
	}
	var snap report.InspectSnapshot
	if err := json.Unmarshal([]byte(jout), &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(snap.Logs) == 0 || snap.Logs[0].Source != "jobmanager" {
		t.Errorf("snapshot logs = %+v, want jobmanager lines", snap.Logs)
	}
}

func TestInspectJSON(t *testing.T) {
	writeProfile(t, skewServer(t).URL)
	out, _, err := runInspectCmd(t, "j1", "--profile", "it", "--json")
	if err != nil {
		t.Fatalf("inspect --json: %v", err)
	}
	var snap report.InspectSnapshot
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatalf("stdout not an inspect snapshot: %v\n%s", err, out)
	}
	if snap.JobID != "j1" || len(snap.Operators) != 2 {
		t.Fatalf("snapshot = %+v, want j1 with 2 operators", snap)
	}
	if snap.Bottleneck == nil || !snap.Bottleneck.Found || snap.Bottleneck.Cause != "key-skew" {
		t.Errorf("snapshot bottleneck = %+v, want found key-skew", snap.Bottleneck)
	}
	// v-src is a source, v-agg is a sink (no downstream in this two-op job).
	var haveSource, haveSink bool
	for _, io := range snap.IO {
		if io.Direction == "source" && io.Operator == "v-src" {
			haveSource = true
		}
		if io.Direction == "sink" && io.Operator == "v-agg" {
			haveSink = true
		}
	}
	if !haveSource || !haveSink {
		t.Errorf("I/O endpoints = %+v, want v-src source and v-agg sink", snap.IO)
	}
}
