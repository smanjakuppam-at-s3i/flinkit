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

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestSubcommandsRegistered checks the MVP subcommands are wired onto the root.
// Their behavior is covered by their own tests.
func TestSubcommandsRegistered(t *testing.T) {
	root := newRootCommand("test")
	want := map[string]bool{"scan": false, "doctor": false, "bottleneck": false, "inspect": false}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}

// TestVersion checks that the build version is threaded through to --version.
func TestVersion(t *testing.T) {
	root := newRootCommand("1.2.3")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("--version: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "1.2.3") {
		t.Fatalf("--version output = %q, want it to contain the version", got)
	}
}

// TestVersionCommand checks the `version` subcommand reports the build version.
func TestVersionCommand(t *testing.T) {
	root := newRootCommand("1.2.3")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("version: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "1.2.3") {
		t.Fatalf("version output = %q, want it to contain the version", got)
	}
}
