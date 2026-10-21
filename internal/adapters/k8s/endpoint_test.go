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

package k8s

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResolveEndpointApplicationMode(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"kind":     "FlinkDeployment",
		"metadata": map[string]any{"name": "orders", "namespace": "flink"},
		"status":   map[string]any{"jobStatus": map[string]any{"jobId": "abc123"}},
	}}
	ep, err := resolveEndpoint(u)
	if err != nil {
		t.Fatalf("resolveEndpoint: %v", err)
	}
	want := RESTEndpoint{Namespace: "flink", Service: "orders-rest", Port: 8081, JobID: "abc123", Session: false}
	if ep != want {
		t.Errorf("endpoint = %+v, want %+v", ep, want)
	}
}

func TestResolveEndpointSessionMode(t *testing.T) {
	// Many session jobs share one session cluster's JobManager, so the service
	// resolves to the parent deploymentName, not the job's own name.
	u := &unstructured.Unstructured{Object: map[string]any{
		"kind":     "FlinkSessionJob",
		"metadata": map[string]any{"name": "enrich-job", "namespace": "flink"},
		"spec":     map[string]any{"deploymentName": "shared-session"},
		"status":   map[string]any{"jobStatus": map[string]any{"jobId": "def456"}},
	}}
	ep, err := resolveEndpoint(u)
	if err != nil {
		t.Fatalf("resolveEndpoint: %v", err)
	}
	want := RESTEndpoint{Namespace: "flink", Service: "shared-session-rest", Port: 8081, JobID: "def456", Session: true}
	if ep != want {
		t.Errorf("endpoint = %+v, want %+v", ep, want)
	}
}

func TestResolveEndpointRestPortOverride(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"kind":     "FlinkDeployment",
		"metadata": map[string]any{"name": "orders", "namespace": "flink"},
		"spec":     map[string]any{"flinkConfiguration": map[string]any{"rest.port": "9091"}},
	}}
	ep, err := resolveEndpoint(u)
	if err != nil {
		t.Fatalf("resolveEndpoint: %v", err)
	}
	if ep.Port != 9091 {
		t.Errorf("port = %d, want 9091 from rest.port override", ep.Port)
	}
}

func TestResolveEndpointBadRestPortFallsBack(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"kind":     "FlinkDeployment",
		"metadata": map[string]any{"name": "orders"},
		"spec":     map[string]any{"flinkConfiguration": map[string]any{"rest.port": "not-a-number"}},
	}}
	ep, err := resolveEndpoint(u)
	if err != nil {
		t.Fatalf("resolveEndpoint: %v", err)
	}
	if ep.Port != defaultRESTPort {
		t.Errorf("port = %d, want default %d on unparseable rest.port", ep.Port, defaultRESTPort)
	}
}

// A FlinkSessionJob with no spec.deploymentName cannot resolve a JobManager.
func TestResolveEndpointSessionMissingDeployment(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"kind":     "FlinkSessionJob",
		"metadata": map[string]any{"name": "orphan-job"},
	}}
	if _, err := resolveEndpoint(u); err == nil {
		t.Fatal("expected an error for a session job with no deploymentName")
	}
}

// An unnamed resource cannot resolve a service name.
func TestResolveEndpointNoName(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{"kind": "FlinkDeployment"}}
	if _, err := resolveEndpoint(u); err == nil {
		t.Fatal("expected an error for a resource with no name")
	}
}

// deploymentName present without an explicit kind still resolves as a session
// job — the fake dynamic client does not always set Kind on fetched objects.
func TestResolveEndpointSessionByDeploymentName(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "enrich-job", "namespace": "flink"},
		"spec":     map[string]any{"deploymentName": "shared-session"},
	}}
	ep, err := resolveEndpoint(u)
	if err != nil {
		t.Fatalf("resolveEndpoint: %v", err)
	}
	if ep.Service != "shared-session-rest" || !ep.Session {
		t.Errorf("endpoint = %+v, want shared-session-rest/session", ep)
	}
}
