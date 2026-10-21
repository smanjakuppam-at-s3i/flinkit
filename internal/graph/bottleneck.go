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

import "github.com/smanjakuppam-at-s3i/flinkit/internal/model"

// Thresholds define when an operator counts as busy or backpressured. Rates are
// milliseconds of activity per second (0–1000), as the Flink REST API reports.
type Thresholds struct {
	BusyMsPerSec          float64
	BackpressuredMsPerSec float64
}

// DefaultThresholds treats ≥50% busy as busy and ≥10% backpressured as
// backpressured.
func DefaultThresholds() Thresholds {
	return Thresholds{BusyMsPerSec: 500, BackpressuredMsPerSec: 100}
}

// Bottleneck is the located choke point.
type Bottleneck struct {
	Operator              model.Operator
	WorstSubtask          int     // index of the busiest subtask (-1 if none)
	BusyMsPerSec          float64 // operator busy (max over subtasks)
	BackpressuredMsPerSec float64 // operator backpressure (max over subtasks)
}

// Locate finds the choke point: walking sources→sinks, the first operator that
// is busy but not itself backpressured. It returns false when the job shows no
// backpressure anywhere — nothing is constrained, so there is no bottleneck.
func (g *Graph) Locate(th Thresholds) (Bottleneck, bool) {
	backpressuredSomewhere := false
	for _, op := range g.byID {
		if operatorBackpressure(op) >= th.BackpressuredMsPerSec {
			backpressuredSomewhere = true
			break
		}
	}
	if !backpressuredSomewhere {
		return Bottleneck{}, false
	}

	for _, op := range g.Order() {
		busy, worst := operatorBusy(op)
		bp := operatorBackpressure(op)
		if busy >= th.BusyMsPerSec && bp < th.BackpressuredMsPerSec {
			return Bottleneck{
				Operator:              op,
				WorstSubtask:          worst,
				BusyMsPerSec:          busy,
				BackpressuredMsPerSec: bp,
			}, true
		}
	}
	return Bottleneck{}, false
}

// operatorBusy returns the operator's busy rate (max over subtasks) and the
// index of the busiest subtask (-1 when there are no subtasks).
func operatorBusy(op model.Operator) (float64, int) {
	busiest, idx := 0.0, -1
	for _, s := range op.Subtasks {
		if s.BusyMsPerSec > busiest {
			busiest, idx = s.BusyMsPerSec, s.Index
		}
	}
	return busiest, idx
}

// operatorBackpressure returns the operator's backpressure rate (max over
// subtasks).
func operatorBackpressure(op model.Operator) float64 {
	m := 0.0
	for _, s := range op.Subtasks {
		if s.BackpressuredMsPerSec > m {
			m = s.BackpressuredMsPerSec
		}
	}
	return m
}
