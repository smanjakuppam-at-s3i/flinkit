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

// classify builds the graph, wraps the named operator as a located bottleneck,
// and classifies it.
func classify(t *testing.T, job model.Job, opID string) Cause {
	t.Helper()
	g, err := New(job)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var op model.Operator
	found := false
	for _, o := range job.Operators {
		if o.ID == opID {
			op, found = o, true
		}
	}
	if !found {
		t.Fatalf("operator %q not in job", opID)
	}
	busy, worst := operatorBusy(op)
	return g.Classify(Bottleneck{
		Operator:              op,
		WorstSubtask:          worst,
		BusyMsPerSec:          busy,
		BackpressuredMsPerSec: operatorBackpressure(op),
	})
}

func TestClassifySkew(t *testing.T) {
	job := model.Job{ID: "j", Operators: []model.Operator{
		{ID: "src"},
		{ID: "agg", Upstreams: []string{"src"}, Subtasks: []model.Subtask{
			{Index: 0, BusyMsPerSec: 100},
			{Index: 1, BusyMsPerSec: 100},
			{Index: 2, BusyMsPerSec: 100},
			{Index: 3, BusyMsPerSec: 980},
		}},
		{ID: "sink", Upstreams: []string{"agg"}},
	}}
	c := classify(t, job, "agg")
	if c.Kind != CauseSkew {
		t.Fatalf("kind = %q, want key-skew (%s)", c.Kind, c.Evidence)
	}
}

func TestClassifySlowSink(t *testing.T) {
	job := model.Job{ID: "j", Operators: []model.Operator{
		{ID: "src"},
		{ID: "sink", Upstreams: []string{"src"}, Subtasks: []model.Subtask{
			{Index: 0, BusyMsPerSec: 700},
			{Index: 1, BusyMsPerSec: 700},
		}},
	}}
	if c := classify(t, job, "sink"); c.Kind != CauseSlowSink {
		t.Fatalf("kind = %q, want slow-sink", c.Kind)
	}
}

func TestClassifyStateAccess(t *testing.T) {
	job := model.Job{ID: "j", Operators: []model.Operator{
		{ID: "src"},
		{ID: "agg", Upstreams: []string{"src"},
			State:    []model.StateDescriptor{{Name: "windowContents", Backend: "rocksdb"}},
			Subtasks: []model.Subtask{{Index: 0, BusyMsPerSec: 800}, {Index: 1, BusyMsPerSec: 820}},
		},
		{ID: "sink", Upstreams: []string{"agg"}},
	}}
	if c := classify(t, job, "agg"); c.Kind != CauseStateAccess {
		t.Fatalf("kind = %q, want state-access", c.Kind)
	}
}

func TestClassifyCPUBound(t *testing.T) {
	job := model.Job{ID: "j", Operators: []model.Operator{
		{ID: "src"},
		{ID: "map", Upstreams: []string{"src"}, Subtasks: []model.Subtask{
			{Index: 0, BusyMsPerSec: 900},
			{Index: 1, BusyMsPerSec: 910},
		}},
		{ID: "sink", Upstreams: []string{"map"}},
	}}
	if c := classify(t, job, "map"); c.Kind != CauseCPUBound {
		t.Fatalf("kind = %q, want cpu-bound", c.Kind)
	}
}
