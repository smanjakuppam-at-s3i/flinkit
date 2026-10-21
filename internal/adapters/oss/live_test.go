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

//go:build live

// This file holds tests that reach a real Flink cluster. They are gated behind
// the "live" build tag and skipped in CI. Run them explicitly:
//
//	FLINKIT_OSS_ENDPOINT=http://localhost:8081 go test -tags live ./internal/adapters/oss/...
package oss

import (
	"context"
	"os"
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
)

func TestLiveListJobs(t *testing.T) {
	endpoint := os.Getenv("FLINKIT_OSS_ENDPOINT")
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	a, err := New(adapters.Config{Endpoint: endpoint})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	jobs, err := a.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs against %s: %v", endpoint, err)
	}
	t.Logf("found %d job(s) on %s", len(jobs), endpoint)
}
