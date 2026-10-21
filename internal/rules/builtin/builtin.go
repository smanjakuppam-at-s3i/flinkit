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

// Package builtin embeds the rule definitions that ship with the binary, so
// Flinkit needs no external rule files to run. Rules are grouped by category
// in subdirectories (doctor, bottleneck, lint, port, upgrade).
package builtin

import (
	"embed"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

//go:embed doctor
var files embed.FS

// Rules returns the validated, duplicate-free set of all built-in rules.
func Rules() (*rules.RuleSet, error) {
	return rules.LoadFS(files, ".")
}
