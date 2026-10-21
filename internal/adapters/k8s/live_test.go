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

// This file holds tests that reach a real Kubernetes cluster with the Flink
// Operator installed. They are gated behind the "live" build tag and skipped
// in CI. Run them explicitly:
//
//	FLINKIT_K8S_NAMESPACE=flink go test -tags live ./internal/adapters/k8s/...
package k8s

import (
	"context"
	"os"
	"testing"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
)

func TestLiveListJobs(t *testing.T) {
	ns := os.Getenv("FLINKIT_K8S_NAMESPACE")
	if ns == "" {
		ns = "default"
	}
	a, err := New(adapters.Config{Options: map[string]string{"namespace": ns}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	jobs, err := a.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs in %q: %v", ns, err)
	}
	t.Logf("found %d Flink job(s) in namespace %q", len(jobs), ns)
}
