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

package rules

import (
	"sort"
	"sync"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// EvalContext is everything a check may inspect about one job: its structure
// (from the model) and its live metrics (empty when evaluating offline, so
// metric-based checks degrade gracefully to silence).
type EvalContext struct {
	Job     model.Job
	Metrics map[string]float64
}

// Check is the Go-backed detection logic for one rule. The engine hands it the
// rule's metadata so the logic stays free of severities and messages — those
// live in the rule as data, and the check mints findings via Rule.Finding.
type Check interface {
	ID() string
	Evaluate(rule Rule, ctx EvalContext) Findings
}

// CheckFunc adapts a plain function into a Check, so a rule's logic can be a
// single func rather than a type.
type CheckFunc struct {
	RuleID string
	Fn     func(rule Rule, ctx EvalContext) Findings
}

func (c CheckFunc) ID() string { return c.RuleID }

func (c CheckFunc) Evaluate(rule Rule, ctx EvalContext) Findings { return c.Fn(rule, ctx) }

// CheckRegistry holds the Go-backed checks, keyed by rule ID. Checks register
// themselves (typically in an init function); the engine runs those that have
// matching rule metadata.
type CheckRegistry struct {
	mu     sync.RWMutex
	checks map[string]Check
}

// NewCheckRegistry returns an empty registry.
func NewCheckRegistry() *CheckRegistry {
	return &CheckRegistry{checks: make(map[string]Check)}
}

// Register adds a check, panicking on an empty ID or a duplicate so wiring
// mistakes fail loudly at startup.
func (r *CheckRegistry) Register(c Check) {
	id := c.ID()
	if id == "" {
		panic("rules: Register check with empty id")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.checks[id]; dup {
		panic("rules: duplicate check for " + id)
	}
	r.checks[id] = c
}

// All returns the registered checks sorted by ID for deterministic runs.
func (r *CheckRegistry) All() []Check {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Check, 0, len(r.checks))
	for _, c := range r.checks {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// DefaultChecks is the process-wide registry checks register into.
var DefaultChecks = NewCheckRegistry()

// RegisterCheck adds a check to the default registry.
func RegisterCheck(c Check) { DefaultChecks.Register(c) }
