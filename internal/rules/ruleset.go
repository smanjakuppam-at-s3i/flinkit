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

// RuleSet is a validated, duplicate-free collection of rules with lookup by ID
// and category. It backs the rules command and feeds the evaluator.
type RuleSet struct {
	rules []Rule
	byID  map[string]Rule
}

// NewRuleSet validates each rule and rejects duplicate IDs.
func NewRuleSet(rules []Rule) (*RuleSet, error) {
	rs := &RuleSet{byID: make(map[string]Rule, len(rules))}
	for _, r := range rules {
		if err := r.Validate(); err != nil {
			return nil, err
		}
		if _, dup := rs.byID[r.ID]; dup {
			return nil, fmt.Errorf("rules: duplicate rule id %q", r.ID)
		}
		rs.byID[r.ID] = r
		rs.rules = append(rs.rules, r)
	}
	return rs, nil
}

// All returns the rules in load order.
func (rs *RuleSet) All() []Rule { return rs.rules }

// Len returns the number of rules.
func (rs *RuleSet) Len() int { return len(rs.rules) }

// ByID looks a rule up by its stable ID.
func (rs *RuleSet) ByID(id string) (Rule, bool) {
	r, ok := rs.byID[id]
	return r, ok
}

// ByCategory returns the rules in one category, in load order.
func (rs *RuleSet) ByCategory(c Category) []Rule {
	var out []Rule
	for _, r := range rs.rules {
		if r.Category == c {
			out = append(out, r)
		}
	}
	return out
}
