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

package fake

import (
	"context"
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

func fixture() model.Inventory {
	inv := model.NewInventory()
	inv.Source = "fake"
	inv.Jobs = []model.Job{{
		ID:   "job-1",
		Name: "demo",
		Operators: []model.Operator{{
			ID:   "op-agg",
			Name: "Window Agg",
			Subtasks: []model.Subtask{
				{Index: 0, BusyMsPerSec: 100},
				{Index: 1, BusyMsPerSec: 980},
			},
		}},
		Config: map[string]string{"parallelism.default": "4"},
	}}
	inv.Statements = []model.Statement{{Name: "s1", SQL: "SELECT 1"}}
	return inv
}

func TestFakeServesFixture(t *testing.T) {
	ctx := context.Background()
	a := New("fake", fixture())

	jobs, err := a.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "job-1" {
		t.Fatalf("ListJobs = %+v, want one job job-1", jobs)
	}
	if jobs[0].Operators != nil {
		t.Errorf("ListJobs should return lightweight jobs without operators")
	}

	graph, err := a.GetJobGraph(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJobGraph: %v", err)
	}
	if len(graph.Operators) != 1 {
		t.Fatalf("GetJobGraph operators = %d, want 1", len(graph.Operators))
	}

	subs, err := a.GetSubtaskMetrics(ctx, "job-1", "op-agg")
	if err != nil {
		t.Fatalf("GetSubtaskMetrics: %v", err)
	}
	if len(subs) != 2 || subs[1].BusyMsPerSec != 980 {
		t.Fatalf("GetSubtaskMetrics = %+v, want 2 subtasks with busy 980", subs)
	}

	cfg, err := a.GetConfig(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg["parallelism.default"] != "4" {
		t.Errorf("GetConfig = %v, want parallelism.default=4", cfg)
	}
}

func TestFakeNotFound(t *testing.T) {
	a := New("fake", fixture())
	if _, err := a.GetJob(context.Background(), "nope"); err == nil {
		t.Fatal("GetJob for missing job: expected error, got nil")
	}
}

// TestFakeRegisters confirms the fake plugs into the adapter registry like any
// real adapter.
func TestFakeRegisters(t *testing.T) {
	reg := adapters.NewRegistry()
	reg.Register("fake", func(cfg adapters.Config) (adapters.PlatformAdapter, error) {
		return New("fake", fixture()), nil
	})

	a, err := reg.New("fake", adapters.Config{})
	if err != nil {
		t.Fatalf("registry New: %v", err)
	}
	if a.Name() != "fake" {
		t.Errorf("Name = %q, want fake", a.Name())
	}
	if a.Capabilities().Platform != model.PlatformOSS {
		t.Errorf("Capabilities.Platform = %q, want oss", a.Capabilities().Platform)
	}
}
