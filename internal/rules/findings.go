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

import "sort"

// Finding is one triggered rule against one target: a job, operator, or
// source location. It is the unit every rule-running command reports.
type Finding struct {
	RuleID   string   `json:"ruleId"`
	Category Category `json:"category"`
	Severity Severity `json:"severity"`
	Target   string   `json:"target,omitempty"` // job id, operator id, file:line
	Message  string   `json:"message"`
	Evidence string   `json:"evidence,omitempty"` // the observed value that triggered the rule
	Fix      string   `json:"fix,omitempty"`
	Docs     string   `json:"docs,omitempty"`
}

// Findings is an ordered collection of findings.
type Findings []Finding

// Sort orders findings most severe first, then by rule ID and target so output
// is stable across runs.
func (fs Findings) Sort() {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Severity.rank() != fs[j].Severity.rank() {
			return fs[i].Severity.rank() > fs[j].Severity.rank()
		}
		if fs[i].RuleID != fs[j].RuleID {
			return fs[i].RuleID < fs[j].RuleID
		}
		return fs[i].Target < fs[j].Target
	})
}

// MaxSeverity returns the highest severity present, or "" when empty. It backs
// the doctor --fail-on gate.
func (fs Findings) MaxSeverity() Severity {
	if len(fs) == 0 {
		return ""
	}
	max := fs[0].Severity
	for _, f := range fs[1:] {
		if f.Severity.rank() > max.rank() {
			max = f.Severity
		}
	}
	return max
}

// AtLeast returns the findings at or above min severity.
func (fs Findings) AtLeast(min Severity) Findings {
	out := make(Findings, 0, len(fs))
	for _, f := range fs {
		if f.Severity.AtLeast(min) {
			out = append(out, f)
		}
	}
	return out
}
