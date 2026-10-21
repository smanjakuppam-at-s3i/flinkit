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

// chainJob is source → agg → sink. agg is the choke point: busy and not
// backpressured, while its upstream source is backpressured (blocked writing to
// the slow agg) and the sink is starved.
func chainJob() model.Job {
	return model.Job{
		ID: "job-1",
		Operators: []model.Operator{
			{ID: "op-src", Name: "Source", Subtasks: []model.Subtask{
				{Index: 0, BusyMsPerSec: 900, BackpressuredMsPerSec: 800},
			}},
			{ID: "op-agg", Name: "Window Agg", Upstreams: []string{"op-src"}, Subtasks: []model.Subtask{
				{Index: 0, BusyMsPerSec: 500, BackpressuredMsPerSec: 40},
				{Index: 7, BusyMsPerSec: 980, BackpressuredMsPerSec: 50},
			}},
			{ID: "op-sink", Name: "Sink", Upstreams: []string{"op-agg"}, Subtasks: []model.Subtask{
				{Index: 0, BusyMsPerSec: 100, BackpressuredMsPerSec: 0},
			}},
		},
	}
}

func TestOrderSourceToSink(t *testing.T) {
	g, err := New(chainJob())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	order := g.Order()
	if len(order) != 3 || order[0].ID != "op-src" || order[2].ID != "op-sink" {
		t.Fatalf("order = %v, want src..sink", ids(order))
	}
	if s := g.Sources(); len(s) != 1 || s[0].ID != "op-src" {
		t.Errorf("sources = %v, want [op-src]", ids(s))
	}
	if s := g.Sinks(); len(s) != 1 || s[0].ID != "op-sink" {
		t.Errorf("sinks = %v, want [op-sink]", ids(s))
	}
}

func TestLocateChokePoint(t *testing.T) {
	g, err := New(chainJob())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b, found := g.Locate(DefaultThresholds())
	if !found {
		t.Fatal("expected a bottleneck")
	}
	if b.Operator.ID != "op-agg" {
		t.Errorf("bottleneck = %q, want op-agg", b.Operator.ID)
	}
	if b.WorstSubtask != 7 {
		t.Errorf("worst subtask = %d, want 7", b.WorstSubtask)
	}
	if b.BusyMsPerSec != 980 {
		t.Errorf("busy = %v, want 980", b.BusyMsPerSec)
	}
}

func TestLocateHealthyJobHasNoBottleneck(t *testing.T) {
	// Busy but no backpressure anywhere: nothing is constrained.
	job := model.Job{ID: "job-2", Operators: []model.Operator{
		{ID: "op-src", Subtasks: []model.Subtask{{Index: 0, BusyMsPerSec: 600}}},
		{ID: "op-sink", Upstreams: []string{"op-src"}, Subtasks: []model.Subtask{{Index: 0, BusyMsPerSec: 700}}},
	}}
	g, err := New(job)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, found := g.Locate(DefaultThresholds()); found {
		t.Error("healthy job should have no bottleneck")
	}
}

func TestNewRejectsUnknownUpstream(t *testing.T) {
	job := model.Job{ID: "j", Operators: []model.Operator{
		{ID: "a", Upstreams: []string{"ghost"}},
	}}
	if _, err := New(job); err == nil {
		t.Error("expected error for unknown upstream")
	}
}

func TestNewRejectsCycle(t *testing.T) {
	job := model.Job{ID: "j", Operators: []model.Operator{
		{ID: "a", Upstreams: []string{"b"}},
		{ID: "b", Upstreams: []string{"a"}},
	}}
	if _, err := New(job); err == nil {
		t.Error("expected error for a cycle")
	}
}

func ids(ops []model.Operator) []string {
	out := make([]string, len(ops))
	for i, op := range ops {
		out[i] = op.ID
	}
	return out
}
