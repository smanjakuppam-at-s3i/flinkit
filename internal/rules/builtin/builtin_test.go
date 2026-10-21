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

package builtin

import (
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

// TestBuiltinRulesLoad validates the embedded rules parse, validate, and carry
// the expected IDs — guarding against a malformed or duplicate shipped rule.
func TestBuiltinRulesLoad(t *testing.T) {
	rs, err := Rules()
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	if rs.Len() < 2 {
		t.Fatalf("built-in rules = %d, want >= 2", rs.Len())
	}
	for _, id := range []string{"DOC-STATE-001", "DOC-HYG-001"} {
		if _, ok := rs.ByID(id); !ok {
			t.Errorf("built-in rules missing %s", id)
		}
	}
	if n := len(rs.ByCategory(rules.CategoryDoctor)); n != rs.Len() {
		t.Errorf("all built-in rules so far should be doctor category: %d of %d", n, rs.Len())
	}
}
