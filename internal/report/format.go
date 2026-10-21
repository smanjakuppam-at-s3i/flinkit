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
	"fmt"
	"strings"
)

// Format is an output format selected by the --json and --format flags.
type Format string

const (
	FormatTerminal Format = "terminal"
	FormatJSON     Format = "json"
	FormatMarkdown Format = "md"
	FormatHTML     Format = "html"
)

// ParseFormat parses a --format value, accepting "markdown" as an alias for md.
func ParseFormat(s string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(s))) {
	case FormatTerminal:
		return FormatTerminal, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatMarkdown, "markdown":
		return FormatMarkdown, nil
	case FormatHTML:
		return FormatHTML, nil
	default:
		return "", fmt.Errorf("report: unknown format %q (want terminal|json|md|html)", s)
	}
}

// Resolve picks the effective format from the global flags: --json wins and
// forces JSON; otherwise --format applies, defaulting to terminal when empty.
func Resolve(jsonFlag bool, format string) (Format, error) {
	if jsonFlag {
		return FormatJSON, nil
	}
	if strings.TrimSpace(format) == "" {
		return FormatTerminal, nil
	}
	return ParseFormat(format)
}
