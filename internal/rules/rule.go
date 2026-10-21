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

import "fmt"

// Category groups rules by the command that runs them.
type Category string

const (
	CategoryDoctor     Category = "doctor"
	CategoryBottleneck Category = "bottleneck"
	CategoryLint       Category = "lint"
	CategoryPort       Category = "port"
	CategoryUpgrade    Category = "upgrade"
)

// Valid reports whether c is a known category.
func (c Category) Valid() bool {
	switch c {
	case CategoryDoctor, CategoryBottleneck, CategoryLint, CategoryPort, CategoryUpgrade:
		return true
	default:
		return false
	}
}

// Rule is a declarative, data-defined check. The Match block is interpreted by
// the evaluator (not yet implemented); complex rules the declarative form
// cannot express are backed by Go code instead. IDs are stable once released
// and must never be renumbered (for example DOC-CKPT-001).
type Rule struct {
	ID        string         `yaml:"id" json:"id"`
	Category  Category       `yaml:"category" json:"category"`
	Severity  Severity       `yaml:"severity" json:"severity"`
	AppliesTo []string       `yaml:"applies_to" json:"appliesTo,omitempty"`
	Match     map[string]any `yaml:"match" json:"match,omitempty"`
	Message   string         `yaml:"message" json:"message"`
	Fix       string         `yaml:"fix" json:"fix,omitempty"`
	Docs      string         `yaml:"docs" json:"docs,omitempty"`
}

// Validate checks a rule's required fields and enumerations.
func (r Rule) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("rules: rule missing id")
	}
	if !r.Category.Valid() {
		return fmt.Errorf("rules: rule %s has invalid category %q", r.ID, r.Category)
	}
	if !r.Severity.Valid() {
		return fmt.Errorf("rules: rule %s has invalid severity %q", r.ID, r.Severity)
	}
	if r.Message == "" {
		return fmt.Errorf("rules: rule %s missing message", r.ID)
	}
	return nil
}

// Finding builds a Finding from this rule for a given target and evidence,
// carrying the rule's message, fix, and docs through to the report.
func (r Rule) Finding(target, evidence string) Finding {
	return Finding{
		RuleID:   r.ID,
		Category: r.Category,
		Severity: r.Severity,
		Target:   target,
		Message:  r.Message,
		Evidence: evidence,
		Fix:      r.Fix,
		Docs:     r.Docs,
	}
}
