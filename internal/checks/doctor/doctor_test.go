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

package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules/builtin"
)

func loadJob(t *testing.T, ruleID, name string) model.Job {
	t.Helper()
	path := filepath.Join("testdata", "rules", ruleID, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var job model.Job
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	return job
}

func countByRule(fs rules.Findings, ruleID string) int {
	n := 0
	for _, f := range fs {
		if f.RuleID == ruleID {
			n++
		}
	}
	return n
}

// TestDoctorChecksAgainstFixtures runs the real engine (built-in rules + the
// registered doctor checks) over each rule's positive and negative fixture.
// Every rule must fire on its positive input and stay silent on its negative.
func TestDoctorChecksAgainstFixtures(t *testing.T) {
	rs, err := builtin.Rules()
	if err != nil {
		t.Fatalf("builtin.Rules: %v", err)
	}
	eng, err := rules.NewEngine(rs, rules.DefaultChecks.All())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	for _, ruleID := range []string{"DOC-HYG-001", "DOC-STATE-001"} {
		t.Run(ruleID, func(t *testing.T) {
			pos := eng.EvaluateJob(loadJob(t, ruleID, "positive"))
			if countByRule(pos, ruleID) == 0 {
				t.Errorf("%s did not fire on its positive fixture; findings: %+v", ruleID, pos)
			}
			neg := eng.EvaluateJob(loadJob(t, ruleID, "negative"))
			if n := countByRule(neg, ruleID); n != 0 {
				t.Errorf("%s fired %d times on its negative fixture; findings: %+v", ruleID, n, neg)
			}
		})
	}
}

// TestMetricChecks exercises the live-metric checks via an EvalContext carrying
// metrics, with no network.
func TestMetricChecks(t *testing.T) {
	rs, _ := builtin.Rules()
	eng, err := rules.NewEngine(rs, rules.DefaultChecks.All())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	withMetrics := eng.Evaluate(rules.EvalContext{
		Job:     model.Job{ID: "job-1"},
		Metrics: map[string]float64{"checkpoints.failed": 2, "numRestarts": 3},
	})
	if countByRule(withMetrics, "DOC-CKPT-001") != 1 {
		t.Errorf("DOC-CKPT-001 should fire on failed checkpoints: %+v", withMetrics)
	}
	if countByRule(withMetrics, "DOC-STAB-001") != 1 {
		t.Errorf("DOC-STAB-001 should fire on restarts: %+v", withMetrics)
	}

	// No metrics -> metric checks are silent (graceful offline degradation).
	offline := eng.Evaluate(rules.EvalContext{Job: model.Job{ID: "job-1"}})
	if countByRule(offline, "DOC-CKPT-001") != 0 || countByRule(offline, "DOC-STAB-001") != 0 {
		t.Errorf("metric checks should be silent without metrics: %+v", offline)
	}
}

// TestBackpressureCheck exercises DOC-BP-001 over a job with sampled subtask
// backpressure.
func TestBackpressureCheck(t *testing.T) {
	rs, _ := builtin.Rules()
	eng, err := rules.NewEngine(rs, rules.DefaultChecks.All())
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	bp := model.Job{ID: "j", Operators: []model.Operator{
		{ID: "src", Name: "Source", UID: "src",
			Subtasks: []model.Subtask{{Index: 0, BackpressuredMsPerSec: 800}}},
	}}
	if countByRule(eng.Evaluate(rules.EvalContext{Job: bp}), "DOC-BP-001") != 1 {
		t.Error("DOC-BP-001 should fire on a backpressured subtask")
	}

	// Below the threshold -> silent.
	low := model.Job{ID: "j", Operators: []model.Operator{
		{ID: "src", UID: "src", Subtasks: []model.Subtask{{Index: 0, BackpressuredMsPerSec: 100}}},
	}}
	if countByRule(eng.Evaluate(rules.EvalContext{Job: low}), "DOC-BP-001") != 0 {
		t.Error("low backpressure should not fire DOC-BP-001")
	}
}

// TestFindingsCarryRuleMetadata confirms a check's finding inherits severity
// and fix from the rule, not from the check code.
func TestFindingsCarryRuleMetadata(t *testing.T) {
	rs, _ := builtin.Rules()
	eng, _ := rules.NewEngine(rs, rules.DefaultChecks.All())

	fs := eng.EvaluateJob(loadJob(t, "DOC-STATE-001", "positive"))
	var found bool
	for _, f := range fs {
		if f.RuleID == "DOC-STATE-001" {
			found = true
			if f.Severity != rules.SeverityHigh {
				t.Errorf("severity = %q, want high (from the rule)", f.Severity)
			}
			if f.Fix == "" {
				t.Error("fix not carried from the rule")
			}
			if f.Target != "job-1/op-agg" {
				t.Errorf("target = %q, want job-1/op-agg", f.Target)
			}
		}
	}
	if !found {
		t.Fatal("DOC-STATE-001 did not fire")
	}
}
