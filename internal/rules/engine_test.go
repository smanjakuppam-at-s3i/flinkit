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
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// uidCheck flags operators without a UID. It uses the rule's metadata to mint
// findings, so severity/message come from the rule, not the check.
func uidCheck() Check {
	return CheckFunc{
		RuleID: "DOC-HYG-001",
		Fn: func(rule Rule, ctx EvalContext) Findings {
			var out Findings
			for _, op := range ctx.Job.Operators {
				if op.UID == "" {
					out = append(out, rule.Finding(ctx.Job.ID+"/"+op.ID, "operator "+op.Name+" has no uid"))
				}
			}
			return out
		},
	}
}

func testRuleSet(t *testing.T) *RuleSet {
	t.Helper()
	rs, err := NewRuleSet([]Rule{
		{ID: "DOC-HYG-001", Category: CategoryDoctor, Severity: SeverityMedium, Message: "missing uid", Fix: "add uid()"},
	})
	if err != nil {
		t.Fatalf("NewRuleSet: %v", err)
	}
	return rs
}

func TestEngineEvaluateJob(t *testing.T) {
	eng, err := NewEngine(testRuleSet(t), []Check{uidCheck()})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// A job with one operator missing a UID fires once, carrying rule metadata.
	bad := model.Job{ID: "job-1", Operators: []model.Operator{
		{ID: "op-src", Name: "Source", UID: "src"},
		{ID: "op-agg", Name: "Window Agg"}, // no UID
	}}
	fs := eng.EvaluateJob(bad)
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.RuleID != "DOC-HYG-001" || f.Severity != SeverityMedium || f.Fix != "add uid()" {
		t.Errorf("finding did not inherit rule metadata: %+v", f)
	}
	if f.Target != "job-1/op-agg" {
		t.Errorf("target = %q, want job-1/op-agg", f.Target)
	}

	// A clean job yields nothing.
	clean := model.Job{ID: "job-2", Operators: []model.Operator{{ID: "op", Name: "op", UID: "op"}}}
	if n := len(eng.EvaluateJob(clean)); n != 0 {
		t.Errorf("clean job produced %d findings, want 0", n)
	}
}

func TestNewEngineRejectsCheckWithoutRule(t *testing.T) {
	rs, _ := NewRuleSet([]Rule{{ID: "DOC-A", Category: CategoryDoctor, Severity: SeverityLow, Message: "m"}})
	_, err := NewEngine(rs, []Check{CheckFunc{RuleID: "DOC-ORPHAN", Fn: nil}})
	if err == nil {
		t.Fatal("expected error for a check with no matching rule")
	}
}

func TestCheckRegistry(t *testing.T) {
	reg := NewCheckRegistry()
	reg.Register(uidCheck())
	if len(reg.All()) != 1 {
		t.Fatalf("registry size = %d, want 1", len(reg.All()))
	}

	defer func() {
		if recover() == nil {
			t.Error("duplicate Register should panic")
		}
	}()
	reg.Register(uidCheck())
}
