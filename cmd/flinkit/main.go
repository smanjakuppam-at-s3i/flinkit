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

// Command flinkit is the entry point for the Flinkit CLI. The root command
// and subcommands are wired in internal/cli; this file only hands control
// there and sets the exit code.
package main

import (
	"os"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/cli"
)

// version is the build version, overridden at release time via
// -ldflags "-X main.version=...".
var version = "0.0.0-dev"

func main() {
	os.Exit(cli.Execute(version))
}
