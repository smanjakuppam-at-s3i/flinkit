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

package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

func sampleInventory() model.Inventory {
	inv := model.NewInventory()
	inv.Source = "oss"
	inv.Jobs = []model.Job{{
		ID:           "job-1",
		Name:         "orders-enrichment",
		Platform:     model.PlatformOSS,
		FlinkVersion: model.ParseVersion("1.20.1"),
		State:        model.JobStateRunning,
		Parallelism:  4,
	}}
	return inv
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		jsonFlag bool
		format   string
		want     Format
		wantErr  bool
	}{
		{"json flag wins", true, "terminal", FormatJSON, false},
		{"empty defaults terminal", false, "", FormatTerminal, false},
		{"explicit md", false, "md", FormatMarkdown, false},
		{"markdown alias", false, "markdown", FormatMarkdown, false},
		{"unknown errors", false, "pdf", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.jsonFlag, tc.format)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Resolve(%v, %q): expected error", tc.jsonFlag, tc.format)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Resolve = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSameInventoryBothFormats renders one inventory as JSON and as a terminal
// table through the seam, checking each carries the job.
func TestSameInventoryBothFormats(t *testing.T) {
	inv := sampleInventory()

	jr, err := For(FormatJSON)
	if err != nil {
		t.Fatalf("For(json): %v", err)
	}
	var jbuf bytes.Buffer
	if err := jr.RenderInventory(&jbuf, inv); err != nil {
		t.Fatalf("render json: %v", err)
	}
	// The JSON must decode back into an equivalent inventory.
	got, err := model.ReadInventory(&jbuf)
	if err != nil {
		t.Fatalf("decode json report: %v", err)
	}
	if len(got.Jobs) != 1 || got.Jobs[0].ID != "job-1" {
		t.Fatalf("json inventory = %+v, want one job job-1", got)
	}

	tr, err := For(FormatTerminal)
	if err != nil {
		t.Fatalf("For(terminal): %v", err)
	}
	var tbuf bytes.Buffer
	if err := tr.RenderInventory(&tbuf, inv); err != nil {
		t.Fatalf("render terminal: %v", err)
	}
	out := tbuf.String()
	for _, want := range []string{"orders-enrichment", "job-1", "1.20.1", "RUNNING", "source=oss"} {
		if !strings.Contains(out, want) {
			t.Errorf("terminal output missing %q:\n%s", want, out)
		}
	}
}

func TestMarkdownAndHTMLStubbed(t *testing.T) {
	for _, f := range []Format{FormatMarkdown, FormatHTML} {
		r, err := For(f)
		if err != nil {
			t.Fatalf("For(%q): %v", f, err)
		}
		if err := r.RenderInventory(&bytes.Buffer{}, sampleInventory()); err != ErrFormatNotImplemented {
			t.Errorf("RenderInventory(%q) err = %v, want ErrFormatNotImplemented", f, err)
		}
		if err := r.RenderFindings(&bytes.Buffer{}, sampleFindings()); err != ErrFormatNotImplemented {
			t.Errorf("RenderFindings(%q) err = %v, want ErrFormatNotImplemented", f, err)
		}
	}
}

func sampleFindings() rules.Findings {
	return rules.Findings{
		{RuleID: "DOC-STATE-001", Category: rules.CategoryDoctor, Severity: rules.SeverityHigh,
			Target: "orders/op-agg", Message: "Unbounded state without TTL.", Fix: "Set a TTL."},
		{RuleID: "DOC-HYG-001", Category: rules.CategoryDoctor, Severity: rules.SeverityMedium,
			Target: "orders/op-src", Message: "Missing operator UID."},
	}
}

func TestRenderFindingsTerminal(t *testing.T) {
	r, _ := For(FormatTerminal)
	var buf bytes.Buffer
	if err := r.RenderFindings(&buf, sampleFindings()); err != nil {
		t.Fatalf("RenderFindings: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"2 finding(s)", "1 high", "DOC-STATE-001", "HIGH", "orders/op-agg", "fix: Set a TTL."} {
		if !strings.Contains(out, want) {
			t.Errorf("terminal findings missing %q:\n%s", want, out)
		}
	}
}

func TestRenderFindingsJSON(t *testing.T) {
	r, _ := For(FormatJSON)
	var buf bytes.Buffer
	if err := r.RenderFindings(&buf, sampleFindings()); err != nil {
		t.Fatalf("RenderFindings: %v", err)
	}
	var got struct {
		Findings rules.Findings `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode findings json: %v\n%s", err, buf.String())
	}
	if len(got.Findings) != 2 || got.Findings[0].RuleID != "DOC-STATE-001" {
		t.Errorf("decoded findings = %+v", got.Findings)
	}
}
