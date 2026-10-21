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

// Package fake provides an in-memory PlatformAdapter backed by a fixed
// Inventory, so commands and the graph walker can be tested without a live
// platform.
package fake

import (
	"context"
	"fmt"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// Adapter is an in-memory PlatformAdapter serving a fixed Inventory.
type Adapter struct {
	name string
	inv  model.Inventory
	caps adapters.Capabilities
}

// Compile-time assertion that Adapter satisfies the interface.
var _ adapters.PlatformAdapter = (*Adapter)(nil)

// New returns a fake adapter serving inv under the given name.
func New(name string, inv model.Inventory) *Adapter {
	return &Adapter{
		name: name,
		inv:  inv,
		caps: adapters.Capabilities{Platform: model.PlatformOSS},
	}
}

// WithCapabilities sets the capabilities the fake reports and returns the
// adapter for chaining.
func (a *Adapter) WithCapabilities(caps adapters.Capabilities) *Adapter {
	a.caps = caps
	return a
}

// Name returns the adapter's identifier.
func (a *Adapter) Name() string { return a.name }

// ListJobs returns the jobs without their operator graphs.
func (a *Adapter) ListJobs(ctx context.Context) ([]model.Job, error) {
	jobs := make([]model.Job, 0, len(a.inv.Jobs))
	for _, j := range a.inv.Jobs {
		j.Operators = nil
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// GetJob returns one job's metadata without its operator graph.
func (a *Adapter) GetJob(ctx context.Context, jobID string) (model.Job, error) {
	j, ok := a.findJob(jobID)
	if !ok {
		return model.Job{}, notFound("job", jobID)
	}
	j.Operators = nil
	return j, nil
}

// GetJobGraph returns the job with its operator graph intact.
func (a *Adapter) GetJobGraph(ctx context.Context, jobID string) (model.Job, error) {
	j, ok := a.findJob(jobID)
	if !ok {
		return model.Job{}, notFound("job", jobID)
	}
	return j, nil
}

// GetMetrics returns an empty metric set; the fixture carries subtask timings
// on the operators instead.
func (a *Adapter) GetMetrics(ctx context.Context, jobID string) (adapters.Metrics, error) {
	if _, ok := a.findJob(jobID); !ok {
		return nil, notFound("job", jobID)
	}
	return adapters.Metrics{}, nil
}

// GetSubtaskMetrics returns the subtasks recorded on the operator.
func (a *Adapter) GetSubtaskMetrics(ctx context.Context, jobID, operatorID string) ([]model.Subtask, error) {
	j, ok := a.findJob(jobID)
	if !ok {
		return nil, notFound("job", jobID)
	}
	for _, op := range j.Operators {
		if op.ID == operatorID {
			return op.Subtasks, nil
		}
	}
	return nil, notFound("operator", operatorID)
}

// GetConfig returns the job's configuration.
func (a *Adapter) GetConfig(ctx context.Context, jobID string) (map[string]string, error) {
	j, ok := a.findJob(jobID)
	if !ok {
		return nil, notFound("job", jobID)
	}
	return j.Config, nil
}

// ListStatements returns the fixture's statements.
func (a *Adapter) ListStatements(ctx context.Context) ([]model.Statement, error) {
	return a.inv.Statements, nil
}

// Capabilities returns the configured capabilities.
func (a *Adapter) Capabilities() adapters.Capabilities { return a.caps }

func (a *Adapter) findJob(id string) (model.Job, bool) {
	for _, j := range a.inv.Jobs {
		if j.ID == id {
			return j, true
		}
	}
	return model.Job{}, false
}

func notFound(kind, id string) error {
	return fmt.Errorf("fake: %s %q not found", kind, id)
}
