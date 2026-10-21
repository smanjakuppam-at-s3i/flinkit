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
	"os"
	"path/filepath"
	"testing"
)

func TestParseSeverityAndAtLeast(t *testing.T) {
	if _, err := ParseSeverity("HIGH"); err != nil {
		t.Errorf("ParseSeverity(HIGH): %v", err)
	}
	if _, err := ParseSeverity("critical"); err == nil {
		t.Error("ParseSeverity(critical): expected error")
	}
	if !SeverityHigh.AtLeast(SeverityLow) {
		t.Error("high should be at least low")
	}
	if SeverityLow.AtLeast(SeverityHigh) {
		t.Error("low should not be at least high")
	}
}

func TestNewRuleSetValidatesAndDedupes(t *testing.T) {
	// Missing message fails validation.
	if _, err := NewRuleSet([]Rule{{ID: "X", Category: CategoryDoctor, Severity: SeverityHigh}}); err == nil {
		t.Error("expected validation error for missing message")
	}
	// Invalid category fails validation.
	if _, err := NewRuleSet([]Rule{{ID: "X", Category: "nope", Severity: SeverityHigh, Message: "m"}}); err == nil {
		t.Error("expected validation error for bad category")
	}
	// Duplicate IDs are rejected.
	r := Rule{ID: "DOC-1", Category: CategoryDoctor, Severity: SeverityLow, Message: "m"}
	if _, err := NewRuleSet([]Rule{r, r}); err == nil {
		t.Error("expected duplicate id error")
	}
}

func TestRuleSetLookup(t *testing.T) {
	rs, err := NewRuleSet([]Rule{
		{ID: "DOC-1", Category: CategoryDoctor, Severity: SeverityHigh, Message: "a"},
		{ID: "LINT-1", Category: CategoryLint, Severity: SeverityLow, Message: "b"},
	})
	if err != nil {
		t.Fatalf("NewRuleSet: %v", err)
	}
	if _, ok := rs.ByID("DOC-1"); !ok {
		t.Error("ByID(DOC-1) not found")
	}
	if n := len(rs.ByCategory(CategoryDoctor)); n != 1 {
		t.Errorf("ByCategory(doctor) = %d, want 1", n)
	}
}

func TestFindingsSortSeverityAndFilter(t *testing.T) {
	fs := Findings{
		{RuleID: "B", Severity: SeverityLow, Message: "l"},
		{RuleID: "A", Severity: SeverityHigh, Message: "h"},
		{RuleID: "C", Severity: SeverityMedium, Message: "m"},
	}
	fs.Sort()
	if fs[0].Severity != SeverityHigh || fs[2].Severity != SeverityLow {
		t.Fatalf("sort order wrong: %+v", fs)
	}
	if fs.MaxSeverity() != SeverityHigh {
		t.Errorf("MaxSeverity = %s, want high", fs.MaxSeverity())
	}
	if n := len(fs.AtLeast(SeverityMedium)); n != 2 {
		t.Errorf("AtLeast(medium) = %d, want 2", n)
	}
	if Findings(nil).MaxSeverity() != "" {
		t.Error("MaxSeverity of empty should be \"\"")
	}
}

func TestRuleFinding(t *testing.T) {
	r := Rule{ID: "DOC-STATE-001", Category: CategoryDoctor, Severity: SeverityHigh, Message: "msg", Fix: "fix", Docs: "docs"}
	f := r.Finding("job-1/op-agg", "ttl missing")
	if f.RuleID != r.ID || f.Severity != SeverityHigh || f.Target != "job-1/op-agg" ||
		f.Evidence != "ttl missing" || f.Message != "msg" || f.Fix != "fix" {
		t.Errorf("finding = %+v", f)
	}
}

func TestParseRulesYAML(t *testing.T) {
	const doc = `
rules:
  - id: DOC-STATE-001
    category: doctor
    severity: high
    applies_to: [oss, cp]
    match:
      state_without_ttl: true
    message: Unbounded state without TTL.
    fix: Set a TTL.
`
	got, err := ParseRules([]byte(doc))
	if err != nil {
		t.Fatalf("ParseRules: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rules, want 1", len(got))
	}
	r := got[0]
	if r.ID != "DOC-STATE-001" || r.Severity != SeverityHigh || len(r.AppliesTo) != 2 {
		t.Errorf("parsed rule = %+v", r)
	}
	if r.Match["state_without_ttl"] != true {
		t.Errorf("match not parsed: %v", r.Match)
	}
}

// TestLoadDirFromDisk exercises on-disk loading of a rule tree.
func TestLoadDirFromDisk(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "doctor")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	const doc = "rules:\n  - id: DOC-X-001\n    category: doctor\n    severity: low\n    message: a test rule\n"
	if err := os.WriteFile(filepath.Join(sub, "x.yaml"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	rs, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if _, ok := rs.ByID("DOC-X-001"); !ok {
		t.Error("loaded rules missing DOC-X-001")
	}
}
