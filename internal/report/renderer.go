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

package report

import (
	"errors"
	"fmt"
	"io"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

// ErrFormatNotImplemented is returned by renderers for formats that are not
// wired up yet. Shareable Markdown and HTML reports arrive with the P1 doctor
// reports; the seam exists now so commands can target them unchanged later.
var ErrFormatNotImplemented = errors.New("report: format not implemented yet")

// JobFindings pairs a job with the findings raised against it, for fleet output.
type JobFindings struct {
	JobID    string         `json:"jobId"`
	JobName  string         `json:"jobName"`
	Findings rules.Findings `json:"findings"`
}

// BottleneckReport is the result of bottleneck analysis for one job. Found is
// false when the job shows no backpressure (nothing is constrained); the cause
// fields are flattened so the report package stays decoupled from the graph
// package.
type BottleneckReport struct {
	JobID                 string  `json:"jobId"`
	Found                 bool    `json:"found"`
	OperatorID            string  `json:"operatorId,omitempty"`
	OperatorName          string  `json:"operatorName,omitempty"`
	WorstSubtask          int     `json:"worstSubtask,omitempty"`
	BusyMsPerSec          float64 `json:"busyMsPerSec,omitempty"`
	BackpressuredMsPerSec float64 `json:"backpressuredMsPerSec,omitempty"`
	Cause                 string  `json:"cause,omitempty"`
	Summary               string  `json:"summary,omitempty"`
	Evidence              string  `json:"evidence,omitempty"`
	Fix                   string  `json:"fix,omitempty"`
}

// InspectSnapshot is a point-in-time triage view of one job: health, I/O, and
// (later) logs, assembled into one structure. It is the contract every inspect
// UI renders over — the --watch refresher now, a web GUI later.
type InspectSnapshot struct {
	JobID        string            `json:"jobId"`
	JobName      string            `json:"jobName,omitempty"`
	State        string            `json:"state,omitempty"`
	FlinkVersion string            `json:"flinkVersion,omitempty"`
	Operators    []InspectOperator `json:"operators,omitempty"`
	Checkpoints  Checkpoints       `json:"checkpoints"`
	Restarts     int64             `json:"restarts,omitempty"`
	IO           []IOEndpoint      `json:"io,omitempty"`
	Findings     rules.Findings    `json:"findings,omitempty"`
	Bottleneck   *BottleneckReport `json:"bottleneck,omitempty"`
	Logs         []LogLine         `json:"logs,omitempty"`
}

// LogLine is one recent log line with its source, for the inspect view.
type LogLine struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

// InspectOperator is one operator's live health in the snapshot.
type InspectOperator struct {
	ID                    string  `json:"id"`
	Name                  string  `json:"name,omitempty"`
	Parallelism           int     `json:"parallelism,omitempty"`
	BusyMsPerSec          float64 `json:"busyMsPerSec,omitempty"`
	BackpressuredMsPerSec float64 `json:"backpressuredMsPerSec,omitempty"`
}

// Checkpoints is the completed/failed checkpoint count.
type Checkpoints struct {
	Completed int64 `json:"completed"`
	Failed    int64 `json:"failed"`
}

// IOEndpoint is a source or sink of the job, connector-agnostic. Type/resource
// and precise backlog are best-effort; records are what Flink reports.
type IOEndpoint struct {
	Operator   string `json:"operator"`
	Direction  string `json:"direction"` // source | sink
	Type       string `json:"type,omitempty"`
	Resource   string `json:"resource,omitempty"`
	RecordsIn  int64  `json:"recordsIn,omitempty"`
	RecordsOut int64  `json:"recordsOut,omitempty"`
}

// Renderer writes a report in one output format. One implementation exists per
// Format; callers obtain one with For. Payload-specific methods are added as
// commands land; every renderer implements all of them so the seam stays
// uniform across formats.
type Renderer interface {
	// RenderInventory writes a scan inventory.
	RenderInventory(w io.Writer, inv model.Inventory) error
	// RenderFindings writes a ranked list of findings (doctor, lint, etc.).
	RenderFindings(w io.Writer, findings rules.Findings) error
	// RenderFleet writes findings grouped by job (doctor --all).
	RenderFleet(w io.Writer, results []JobFindings) error
	// RenderBottleneck writes a bottleneck analysis for one job.
	RenderBottleneck(w io.Writer, report BottleneckReport) error
	// RenderInspect writes a single-job triage snapshot.
	RenderInspect(w io.Writer, snap InspectSnapshot) error
}

// For returns the renderer for a format.
func For(f Format) (Renderer, error) {
	switch f {
	case FormatTerminal:
		return terminalRenderer{}, nil
	case FormatJSON:
		return jsonRenderer{}, nil
	case FormatMarkdown:
		return markdownRenderer{}, nil
	case FormatHTML:
		return htmlRenderer{}, nil
	default:
		return nil, fmt.Errorf("report: no renderer for format %q", f)
	}
}
