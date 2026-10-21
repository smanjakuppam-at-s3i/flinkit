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

// The DTOs below mirror the Flink REST API wire format. They stay unexported so
// Flink-specific shapes never leak past the adapter; methods map them into the
// vendor-neutral model.

// jobsOverviewDTO is GET /jobs/overview.
type jobsOverviewDTO struct {
	Jobs []jobOverviewDTO `json:"jobs"`
}

type jobOverviewDTO struct {
	JID   string `json:"jid"`
	Name  string `json:"name"`
	State string `json:"state"`
}

// jobDetailDTO is GET /jobs/<jobid>.
type jobDetailDTO struct {
	JID      string      `json:"jid"`
	Name     string      `json:"name"`
	State    string      `json:"state"`
	Vertices []vertexDTO `json:"vertices"`
	Plan     planDTO     `json:"plan"`
}

type vertexDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Parallelism int    `json:"parallelism"`
}

// planDTO carries the dataflow edges used to reconstruct upstream links.
type planDTO struct {
	Nodes []planNodeDTO `json:"nodes"`
}

type planNodeDTO struct {
	ID     string         `json:"id"`
	Inputs []planInputDTO `json:"inputs"`
}

type planInputDTO struct {
	ID string `json:"id"` // upstream node id feeding this node
}

// jobConfigDTO is GET /jobs/<jobid>/config.
type jobConfigDTO struct {
	JID             string `json:"jid"`
	Name            string `json:"name"`
	ExecutionConfig struct {
		JobParallelism int               `json:"job-parallelism"`
		UserConfig     map[string]string `json:"user-config"`
	} `json:"execution-config"`
}

// vertexDetailDTO is GET /jobs/<jobid>/vertices/<vertexid>.
type vertexDetailDTO struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Subtasks []subtaskDTO `json:"subtasks"`
}

type subtaskDTO struct {
	Subtask int               `json:"subtask"`
	Metrics subtaskMetricsDTO `json:"metrics"`
}

type subtaskMetricsDTO struct {
	ReadRecords  int64 `json:"read-records"`
	WriteRecords int64 `json:"write-records"`
}

// checkpointsDTO is GET /jobs/<jobid>/checkpoints (the counts block only).
type checkpointsDTO struct {
	Counts struct {
		Completed int64 `json:"completed"`
		Failed    int64 `json:"failed"`
	} `json:"counts"`
}

// jobMetricDTO is one entry of GET /jobs/<jobid>/metrics?get=<names>.
type jobMetricDTO struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// clusterConfigDTO is the relevant slice of GET /config (cluster-level).
type clusterConfigDTO struct {
	FlinkVersion string `json:"flink-version"`
}
