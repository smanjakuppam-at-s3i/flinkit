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
	"fmt"
	"strings"
)

// Severity ranks a finding's importance. The order (info < low < medium <
// high) drives ranking in reports and the doctor --fail-on gate.
type Severity string

const (
	SeverityInfo   Severity = "info"
	SeverityLow    Severity = "low"
	SeverityMedium Severity = "medium"
	SeverityHigh   Severity = "high"
)

var severityRank = map[Severity]int{
	SeverityInfo:   0,
	SeverityLow:    1,
	SeverityMedium: 2,
	SeverityHigh:   3,
}

// Valid reports whether s is a known severity.
func (s Severity) Valid() bool {
	_, ok := severityRank[s]
	return ok
}

// rank returns the ordering weight; an unknown severity ranks lowest.
func (s Severity) rank() int { return severityRank[s] }

// AtLeast reports whether s is as severe as, or more severe than, min.
func (s Severity) AtLeast(min Severity) bool { return s.rank() >= min.rank() }

// ParseSeverity parses a severity string, as used by --fail-on.
func ParseSeverity(v string) (Severity, error) {
	s := Severity(strings.ToLower(strings.TrimSpace(v)))
	if !s.Valid() {
		return "", fmt.Errorf("rules: unknown severity %q (want info|low|medium|high)", v)
	}
	return s, nil
}
