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
	"fmt"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// Engine runs a set of checks, each paired with its rule metadata, against a
// job and returns ranked findings.
type Engine struct {
	rules  *RuleSet
	checks []Check
}

// NewEngine pairs checks with their rule metadata. Every check must have a
// matching rule, so a check referring to an unknown rule is a wiring error and
// fails here. Rules without a check are allowed — they are simply not yet
// implemented (or will be evaluated by the declarative matcher later).
func NewEngine(ruleSet *RuleSet, checks []Check) (*Engine, error) {
	if ruleSet == nil {
		return nil, fmt.Errorf("rules: nil rule set")
	}
	for _, c := range checks {
		if _, ok := ruleSet.ByID(c.ID()); !ok {
			return nil, fmt.Errorf("rules: check %q has no rule metadata", c.ID())
		}
	}
	return &Engine{rules: ruleSet, checks: checks}, nil
}

// Evaluate runs every check against the context and returns the findings,
// ranked most severe first.
func (e *Engine) Evaluate(ctx EvalContext) Findings {
	var all Findings
	for _, c := range e.checks {
		rule, _ := e.rules.ByID(c.ID()) // present: NewEngine validated it
		all = append(all, c.Evaluate(rule, ctx)...)
	}
	all.Sort()
	return all
}

// EvaluateJob is a convenience for the offline case: evaluate a job with no
// live metrics.
func (e *Engine) EvaluateJob(job model.Job) Findings {
	return e.Evaluate(EvalContext{Job: job})
}

// Rules returns the engine's rule set.
func (e *Engine) Rules() *RuleSet { return e.rules }
