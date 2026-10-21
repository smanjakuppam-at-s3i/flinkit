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

import (
	"strconv"
	"strings"
)

// Version is a Flink version such as 1.20.1. Raw preserves the original text;
// Major, Minor, and Patch are parsed for comparison. A component that could
// not be parsed is -1.
type Version struct {
	Raw   string `json:"raw"`
	Major int    `json:"major"`
	Minor int    `json:"minor"`
	Patch int    `json:"patch"`
}

// ParseVersion parses a Flink version string. It tolerates a leading "v" and a
// trailing qualifier (for example "1.20.1-SNAPSHOT"). Missing or unparseable
// numeric components become -1, while Raw always retains the input.
func ParseVersion(s string) Version {
	v := Version{Raw: s, Major: -1, Minor: -1, Patch: -1}

	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "v")
	// The Flink Kubernetes Operator reports versions as an enum like "v1_20" or
	// "v2_0"; normalize the underscore to a dot so it parses like "1.20"/"2.0".
	t = strings.ReplaceAll(t, "_", ".")
	if t == "" {
		return v
	}
	// Drop any qualifier after the numeric core (e.g. "-SNAPSHOT", "-rc1").
	if i := strings.IndexAny(t, "-+"); i >= 0 {
		t = t[:i]
	}

	parts := strings.Split(t, ".")
	dst := []*int{&v.Major, &v.Minor, &v.Patch}
	for i := 0; i < len(parts) && i < len(dst); i++ {
		if n, err := strconv.Atoi(parts[i]); err == nil {
			*dst[i] = n
		}
	}
	return v
}

// String returns the original text when present, otherwise a reconstructed
// major.minor.patch form.
func (v Version) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	// An unset version (zero value, Major 0) or an unparseable one (Major -1)
	// has no meaningful string — render nothing rather than a fake "0.0.0".
	if v.Major <= 0 {
		return ""
	}
	parts := []string{strconv.Itoa(v.Major)}
	if v.Minor >= 0 {
		parts = append(parts, strconv.Itoa(v.Minor))
	}
	if v.Patch >= 0 {
		parts = append(parts, strconv.Itoa(v.Patch))
	}
	return strings.Join(parts, ".")
}

// Compare returns -1, 0, or +1 as v is older than, equal to, or newer than w,
// comparing Major then Minor then Patch. Unparsed components (-1) sort before 0.
func (v Version) Compare(w Version) int {
	for _, pair := range [][2]int{{v.Major, w.Major}, {v.Minor, w.Minor}, {v.Patch, w.Patch}} {
		switch {
		case pair[0] < pair[1]:
			return -1
		case pair[0] > pair[1]:
			return 1
		}
	}
	return 0
}
