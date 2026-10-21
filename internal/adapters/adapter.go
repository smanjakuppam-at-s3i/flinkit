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

package adapters

import (
	"context"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// Metrics is a flat set of named numeric metrics, as the Flink REST API
// exposes them. Adapters omit metrics they cannot provide, so consumers must
// tolerate missing keys — this is how bottleneck analysis degrades gracefully
// on platforms that expose fewer metrics.
type Metrics map[string]float64

// PlatformAdapter is the single seam between Flinkit's core and a Flink
// platform. Every platform — OSS, Kubernetes, CP, CC, local repos — is an
// adapter behind this interface, and platform-specific types never leak past
// it. All methods are read-only.
type PlatformAdapter interface {
	// Name is the adapter's short identifier: oss, k8s, cp, cc, or repo.
	Name() string

	// ListJobs returns the jobs visible to this adapter with lightweight
	// metadata and no operator graph.
	ListJobs(ctx context.Context) ([]model.Job, error)

	// GetJob returns a single job's metadata by ID.
	GetJob(ctx context.Context, jobID string) (model.Job, error)

	// GetJobGraph returns the job with its operator graph populated (operators
	// and upstream edges) — the input to bottleneck analysis.
	GetJobGraph(ctx context.Context, jobID string) (model.Job, error)

	// GetMetrics returns job-scoped aggregate metrics.
	GetMetrics(ctx context.Context, jobID string) (Metrics, error)

	// GetSubtaskMetrics returns the per-subtask timings for one operator.
	GetSubtaskMetrics(ctx context.Context, jobID, operatorID string) ([]model.Subtask, error)

	// GetConfig returns a job's effective configuration.
	GetConfig(ctx context.Context, jobID string) (map[string]string, error)

	// ListStatements returns the SQL statements visible to this adapter.
	ListStatements(ctx context.Context) ([]model.Statement, error)

	// Capabilities declares what this platform supports.
	Capabilities() Capabilities
}

// Config carries the connection settings an adapter needs to reach a platform.
// It is intentionally small; profile loading in a later rung populates it.
type Config struct {
	Endpoint string            `json:"endpoint,omitempty"`
	Options  map[string]string `json:"options,omitempty"`
}

// Factory constructs an adapter from a Config.
type Factory func(cfg Config) (PlatformAdapter, error)
