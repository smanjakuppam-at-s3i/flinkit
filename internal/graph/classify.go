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
	"fmt"
	"strings"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// skewFactor is how far the busiest subtask must exceed the subtask average
// before the bottleneck is attributed to key/partition skew.
const skewFactor = 1.5

// CauseKind names why an operator is the choke point.
type CauseKind string

const (
	CauseUnknown     CauseKind = "unknown"
	CauseSkew        CauseKind = "key-skew"
	CauseSlowSink    CauseKind = "slow-sink"
	CauseStateAccess CauseKind = "state-access"
	CauseCPUBound    CauseKind = "cpu-bound"
)

// Cause explains the bottleneck and how to fix it.
type Cause struct {
	Kind     CauseKind `json:"kind"`
	Summary  string    `json:"summary"`
	Evidence string    `json:"evidence"`
	Fix      string    `json:"fix"`
}

// Classify attributes a located bottleneck to a cause. Skew is checked first
// (the sharpest, most actionable diagnosis); then slow sink, state access, and
// CPU-bound; otherwise unknown.
func (g *Graph) Classify(b Bottleneck) Cause {
	op := b.Operator

	if skewed, worst, worstBusy, ratio := busySkew(op.Subtasks); skewed {
		return Cause{
			Kind:     CauseSkew,
			Summary:  "Key or partition skew: work is concentrated on one subtask.",
			Evidence: fmt.Sprintf("subtask %d is %.0f ms/s busy, %.1fx the subtask average", worst, worstBusy, ratio),
			Fix:      "Salt hot keys or repartition so load spreads evenly across subtasks.",
		}
	}

	ev := fmt.Sprintf("%.0f ms/s busy, %.0f ms/s backpressured", b.BusyMsPerSec, b.BackpressuredMsPerSec)

	switch {
	case g.IsSink(op.ID):
		return Cause{
			Kind:     CauseSlowSink,
			Summary:  "The sink cannot keep up with upstream throughput.",
			Evidence: ev,
			Fix:      "Raise sink parallelism or tune batching (linger, batch size, in-flight requests).",
		}
	case hasRocksDBState(op):
		return Cause{
			Kind:     CauseStateAccess,
			Summary:  "State access is the constraint (RocksDB).",
			Evidence: ev,
			Fix:      "Tune RocksDB (block cache, write buffers) or shrink state via TTL; use the heap backend if state is small.",
		}
	case b.BusyMsPerSec > 0:
		return Cause{
			Kind:     CauseCPUBound,
			Summary:  "CPU-bound operator logic.",
			Evidence: ev,
			Fix:      "Raise parallelism, or optimize the operator logic (UDFs, serialization, allocations).",
		}
	default:
		return Cause{
			Kind:     CauseUnknown,
			Summary:  "Could not classify the bottleneck from the available metrics.",
			Evidence: ev,
			Fix:      "Collect more signals (GC, network buffers, upstream Kafka lag) and re-run.",
		}
	}
}

// busySkew reports whether one subtask dominates. It returns the busiest
// subtask's index and busy rate and the max/average ratio.
func busySkew(subs []model.Subtask) (skewed bool, worst int, worstBusy, ratio float64) {
	if len(subs) < 2 {
		return false, -1, 0, 0
	}
	var sum float64
	worst = -1
	for _, s := range subs {
		sum += s.BusyMsPerSec
		if s.BusyMsPerSec > worstBusy {
			worstBusy, worst = s.BusyMsPerSec, s.Index
		}
	}
	mean := sum / float64(len(subs))
	if mean <= 0 {
		return false, worst, worstBusy, 0
	}
	ratio = worstBusy / mean
	return ratio >= skewFactor, worst, worstBusy, ratio
}

func hasRocksDBState(op model.Operator) bool {
	for _, s := range op.State {
		if strings.EqualFold(s.Backend, "rocksdb") {
			return true
		}
	}
	return false
}
