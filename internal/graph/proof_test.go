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

package graph

import (
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// backpressuredSource is the common upstream: busy and blocked writing to the
// slow operator downstream, so it is never itself the choke point.
func backpressuredSource() model.Operator {
	return model.Operator{ID: "src", Name: "Source",
		Subtasks: []model.Subtask{{Index: 0, BusyMsPerSec: 900, BackpressuredMsPerSec: 800}}}
}

// TestDeliberateBottlenecks is the proof suite required by the conventions:
// each job has a known, deliberate bottleneck that must be located and
// classified correctly through the full New → Locate → Classify path.
func TestDeliberateBottlenecks(t *testing.T) {
	cases := []struct {
		name      string
		job       model.Job
		wantOp    string
		wantCause CauseKind
	}{
		{
			name: "key-skew",
			job: model.Job{ID: "skew", Operators: []model.Operator{
				backpressuredSource(),
				{ID: "agg", Name: "Window Agg", Upstreams: []string{"src"}, Subtasks: []model.Subtask{
					{Index: 0, BusyMsPerSec: 300, BackpressuredMsPerSec: 20},
					{Index: 1, BusyMsPerSec: 980, BackpressuredMsPerSec: 30},
				}},
				{ID: "sink", Upstreams: []string{"agg"}, Subtasks: []model.Subtask{{Index: 0, BusyMsPerSec: 100}}},
			}},
			wantOp:    "agg",
			wantCause: CauseSkew,
		},
		{
			name: "slow-sink",
			job: model.Job{ID: "sink-slow", Operators: []model.Operator{
				backpressuredSource(),
				{ID: "sink", Name: "Kafka Sink", Upstreams: []string{"src"}, Subtasks: []model.Subtask{
					{Index: 0, BusyMsPerSec: 800},
					{Index: 1, BusyMsPerSec: 820},
				}},
			}},
			wantOp:    "sink",
			wantCause: CauseSlowSink,
		},
		{
			name: "heavy-state",
			job: model.Job{ID: "state", Operators: []model.Operator{
				backpressuredSource(),
				{ID: "agg", Name: "Keyed Agg", Upstreams: []string{"src"},
					State: []model.StateDescriptor{{Name: "windowContents", Backend: "rocksdb"}},
					Subtasks: []model.Subtask{
						{Index: 0, BusyMsPerSec: 800},
						{Index: 1, BusyMsPerSec: 820},
					}},
				{ID: "sink", Upstreams: []string{"agg"}, Subtasks: []model.Subtask{{Index: 0, BusyMsPerSec: 100}}},
			}},
			wantOp:    "agg",
			wantCause: CauseStateAccess,
		},
		{
			name: "cpu-bound",
			job: model.Job{ID: "cpu", Operators: []model.Operator{
				backpressuredSource(),
				{ID: "map", Name: "Heavy Map", Upstreams: []string{"src"}, Subtasks: []model.Subtask{
					{Index: 0, BusyMsPerSec: 900},
					{Index: 1, BusyMsPerSec: 910},
				}},
				{ID: "sink", Upstreams: []string{"map"}, Subtasks: []model.Subtask{{Index: 0, BusyMsPerSec: 100}}},
			}},
			wantOp:    "map",
			wantCause: CauseCPUBound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := New(tc.job)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			b, found := g.Locate(DefaultThresholds())
			if !found {
				t.Fatal("no bottleneck located")
			}
			if b.Operator.ID != tc.wantOp {
				t.Errorf("located %q, want %q", b.Operator.ID, tc.wantOp)
			}
			if c := g.Classify(b); c.Kind != tc.wantCause {
				t.Errorf("cause = %q, want %q (evidence: %s)", c.Kind, tc.wantCause, c.Evidence)
			}
		})
	}
}
