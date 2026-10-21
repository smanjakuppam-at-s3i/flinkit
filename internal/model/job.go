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

package model

// JobState mirrors the lifecycle states a Flink job can be in.
type JobState string

const (
	JobStateUnknown    JobState = ""
	JobStateRunning    JobState = "RUNNING"
	JobStateFinished   JobState = "FINISHED"
	JobStateFailed     JobState = "FAILED"
	JobStateCanceled   JobState = "CANCELED"
	JobStateRestarting JobState = "RESTARTING"
	JobStateSuspended  JobState = "SUSPENDED"
)

// Job is a single Flink job: its identity, where it runs, its graph of
// operators, and its effective configuration.
type Job struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Platform     Platform          `json:"platform,omitempty"`
	FlinkVersion Version           `json:"flinkVersion,omitempty"`
	State        JobState          `json:"state,omitempty"`
	Parallelism  int               `json:"parallelism,omitempty"`
	Operators    []Operator        `json:"operators,omitempty"`
	Config       map[string]string `json:"config,omitempty"`
}

// Operator is a vertex in the job graph. Upstreams lists the IDs of the
// operators feeding this one, which the graph walker uses to propagate
// backpressure from sinks toward sources.
type Operator struct {
	ID          string            `json:"id"`
	UID         string            `json:"uid,omitempty"` // operator UID; absence is a hygiene finding
	Name        string            `json:"name"`
	Parallelism int               `json:"parallelism,omitempty"`
	Upstreams   []string          `json:"upstreams,omitempty"`
	Subtasks    []Subtask         `json:"subtasks,omitempty"`
	State       []StateDescriptor `json:"state,omitempty"`
}

// Subtask is one parallel instance of an operator, carrying the per-subtask
// timings the bottleneck analysis samples. Rates are milliseconds of activity
// per second, as exposed by the Flink REST API.
type Subtask struct {
	Index                 int     `json:"index"`
	BusyMsPerSec          float64 `json:"busyMsPerSec,omitempty"`
	BackpressuredMsPerSec float64 `json:"backpressuredMsPerSec,omitempty"`
	IdleMsPerSec          float64 `json:"idleMsPerSec,omitempty"`
	RecordsIn             int64   `json:"recordsIn,omitempty"`
	RecordsOut            int64   `json:"recordsOut,omitempty"`
}

// StateDescriptor describes a piece of keyed or operator state. A missing TTL
// on unbounded state is a common lint and doctor finding.
type StateDescriptor struct {
	Name      string `json:"name"`
	Kind      string `json:"kind,omitempty"`    // value|list|map|aggregating|reducing
	Backend   string `json:"backend,omitempty"` // hashmap|rocksdb
	TTL       string `json:"ttl,omitempty"`     // e.g. "1h"; empty means no TTL
	SizeBytes int64  `json:"sizeBytes,omitempty"`
}
