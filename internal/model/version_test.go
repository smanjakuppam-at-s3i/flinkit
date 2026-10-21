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

package model

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in                  string
		major, minor, patch int
	}{
		{"1.20.1", 1, 20, 1},
		{"1.20", 1, 20, -1},
		{"v1.19.0", 1, 19, 0},
		{"1.20.1-SNAPSHOT", 1, 20, 1},
		{"1.18.0+build.5", 1, 18, 0},
		{"v2_0", 2, 0, -1},   // Flink Operator enum
		{"v1_20", 1, 20, -1}, // Flink Operator enum
		{"", -1, -1, -1},
		{"garbage", -1, -1, -1},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			v := ParseVersion(tc.in)
			if v.Raw != tc.in {
				t.Errorf("Raw = %q, want %q", v.Raw, tc.in)
			}
			if v.Major != tc.major || v.Minor != tc.minor || v.Patch != tc.patch {
				t.Errorf("parsed = %d.%d.%d, want %d.%d.%d",
					v.Major, v.Minor, v.Patch, tc.major, tc.minor, tc.patch)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.20.1", "1.20.1", 0},
		{"1.19.0", "1.20.0", -1},
		{"1.20.0", "1.19.9", 1},
		{"1.20.1", "1.20.0", 1},
		{"1.20", "1.20.0", -1}, // missing patch (-1) sorts before 0
	}
	for _, tc := range tests {
		if got := ParseVersion(tc.a).Compare(ParseVersion(tc.b)); got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestVersionString(t *testing.T) {
	if got := ParseVersion("1.20.1").String(); got != "1.20.1" {
		t.Errorf("String() with Raw = %q, want 1.20.1", got)
	}
	// With no Raw, String reconstructs from components.
	v := Version{Major: 1, Minor: 20, Patch: -1}
	if got := v.String(); got != "1.20" {
		t.Errorf("String() reconstructed = %q, want 1.20", got)
	}
	// An unset version renders empty, not a fake "0.0.0".
	if got := (Version{}).String(); got != "" {
		t.Errorf("unset version String() = %q, want empty", got)
	}
}
