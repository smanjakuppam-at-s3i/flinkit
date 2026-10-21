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

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// Graph is a job's operator graph with a source→sink topological order, built
// from the upstream edges in the model.
type Graph struct {
	job        model.Job
	byID       map[string]model.Operator
	downstream map[string][]string
	order      []string // topological, sources first
}

// New builds a graph from a job, returning an error on a duplicate operator,
// an unknown upstream, or a cycle.
func New(job model.Job) (*Graph, error) {
	g := &Graph{
		job:        job,
		byID:       make(map[string]model.Operator, len(job.Operators)),
		downstream: make(map[string][]string),
	}
	for _, op := range job.Operators {
		if _, dup := g.byID[op.ID]; dup {
			return nil, fmt.Errorf("graph: duplicate operator id %q", op.ID)
		}
		g.byID[op.ID] = op
	}

	indeg := make(map[string]int, len(job.Operators))
	for _, op := range job.Operators {
		indeg[op.ID] = 0
	}
	for _, op := range job.Operators {
		for _, up := range op.Upstreams {
			if _, ok := g.byID[up]; !ok {
				return nil, fmt.Errorf("graph: operator %q has unknown upstream %q", op.ID, up)
			}
			g.downstream[up] = append(g.downstream[up], op.ID)
			indeg[op.ID]++
		}
	}

	// Kahn's algorithm, seeding sources in input order for a stable result.
	var queue []string
	for _, op := range job.Operators {
		if indeg[op.ID] == 0 {
			queue = append(queue, op.ID)
		}
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		g.order = append(g.order, n)
		for _, d := range g.downstream[n] {
			indeg[d]--
			if indeg[d] == 0 {
				queue = append(queue, d)
			}
		}
	}
	if len(g.order) != len(job.Operators) {
		return nil, fmt.Errorf("graph: job %q has a cycle", job.ID)
	}
	return g, nil
}

// Order returns the operators source→sink in topological order.
func (g *Graph) Order() []model.Operator {
	out := make([]model.Operator, 0, len(g.order))
	for _, id := range g.order {
		out = append(out, g.byID[id])
	}
	return out
}

// Sources returns the operators with no upstreams, in input order.
func (g *Graph) Sources() []model.Operator {
	var out []model.Operator
	for _, op := range g.job.Operators {
		if len(op.Upstreams) == 0 {
			out = append(out, op)
		}
	}
	return out
}

// Sinks returns the operators that feed no one, in input order.
func (g *Graph) Sinks() []model.Operator {
	var out []model.Operator
	for _, op := range g.job.Operators {
		if len(g.downstream[op.ID]) == 0 {
			out = append(out, op)
		}
	}
	return out
}

// IsSink reports whether the operator feeds no downstream operator.
func (g *Graph) IsSink(id string) bool {
	return len(g.downstream[id]) == 0
}
