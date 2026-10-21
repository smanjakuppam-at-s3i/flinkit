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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

// flinkDeployment builds an unstructured FlinkDeployment custom resource.
func flinkDeployment(name, version string, parallelism int64, state string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "flink.apache.org/v1beta1",
		"kind":       "FlinkDeployment",
		"metadata":   map[string]any{"name": name, "namespace": "default"},
		"spec": map[string]any{
			"flinkVersion":       version,
			"job":                map[string]any{"parallelism": parallelism},
			"flinkConfiguration": map[string]any{"state.backend": "rocksdb"},
		},
		"status": map[string]any{
			"jobStatus": map[string]any{"state": state},
		},
	}}
}

// flinkSessionJob builds an unstructured FlinkSessionJob custom resource.
func flinkSessionJob(name string, parallelism int64, state string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "flink.apache.org/v1beta1",
		"kind":       "FlinkSessionJob",
		"metadata":   map[string]any{"name": name, "namespace": "default"},
		"spec": map[string]any{
			"deploymentName": "session-cluster",
			"job":            map[string]any{"parallelism": parallelism},
		},
		"status": map[string]any{
			"jobStatus": map[string]any{"state": state},
		},
	}}
}

// fakeAdapter returns a K8s adapter backed by a fake dynamic client seeded with
// the given custom resources.
func fakeAdapter(objs ...*unstructured.Unstructured) *Adapter {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		flinkDeploymentGVR: "FlinkDeploymentList",
		flinkSessionJobGVR: "FlinkSessionJobList",
	}
	runtimeObjs := make([]runtime.Object, 0, len(objs))
	for _, o := range objs {
		runtimeObjs = append(runtimeObjs, o)
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, runtimeObjs...)
	return newWithClient(client, "default")
}

func TestListJobs(t *testing.T) {
	a := fakeAdapter(
		flinkDeployment("orders", "1.20.1", 4, "RUNNING"),
		flinkDeployment("payments", "1.19.0", 2, "RUNNING"),
	)
	jobs, err := a.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}

	byID := map[string]model.Job{}
	for _, j := range jobs {
		byID[j.ID] = j
	}
	orders, ok := byID["orders"]
	if !ok {
		t.Fatal("missing orders deployment")
	}
	if orders.Platform != model.PlatformOSS || orders.State != model.JobStateRunning ||
		orders.Parallelism != 4 || orders.FlinkVersion.String() != "1.20.1" {
		t.Errorf("orders = %+v, want oss/RUNNING/par4/1.20.1", orders)
	}
}

func TestGetJob(t *testing.T) {
	a := fakeAdapter(flinkDeployment("orders", "1.20.1", 4, "RUNNING"))

	job, err := a.GetJob(context.Background(), "orders")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.Name != "orders" || job.Parallelism != 4 {
		t.Errorf("job = %+v, want orders/par4", job)
	}

	if _, err := a.GetJob(context.Background(), "missing"); err == nil {
		t.Error("GetJob for a missing deployment: expected error")
	}
}

func TestNameAndRegistration(t *testing.T) {
	if a := fakeAdapter(); a.Name() != "k8s" {
		t.Errorf("Name = %q, want k8s", a.Name())
	}
}

func TestGetConfig(t *testing.T) {
	a := fakeAdapter(flinkDeployment("orders", "1.20.1", 4, "RUNNING"))
	cfg, err := a.GetConfig(context.Background(), "orders")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg["state.backend"] != "rocksdb" {
		t.Errorf("config = %v, want state.backend=rocksdb", cfg)
	}
}

func TestSessionJobsDiscovered(t *testing.T) {
	a := fakeAdapter(
		flinkDeployment("orders", "1.20.1", 4, "RUNNING"),
		flinkSessionJob("enrich", 2, "RUNNING"),
	)
	jobs, err := a.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2 (deployment + session job)", len(jobs))
	}
	// The session job is reachable by GetJob too (falls back past FlinkDeployment).
	sj, err := a.GetJob(context.Background(), "enrich")
	if err != nil {
		t.Fatalf("GetJob(enrich): %v", err)
	}
	if sj.Name != "enrich" || sj.Parallelism != 2 {
		t.Errorf("session job = %+v, want enrich/par2", sj)
	}
}

func TestCapabilitiesLiveMetricsViaREST(t *testing.T) {
	caps := fakeAdapter().Capabilities()
	if caps.Metric("busyTimeMsPerSecond") != adapters.SupportYes {
		t.Errorf("live metrics should be supported via REST, got %q", caps.Metric("busyTimeMsPerSecond"))
	}
}

// jmServer is a minimal JobManager REST server for the graph + overview paths.
func jmServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/jobs/flinkjob-1":
			io.WriteString(w, `{"jid":"flinkjob-1","name":"orders","state":"RUNNING",
				"vertices":[{"id":"v1","name":"Source","parallelism":2},
				            {"id":"v2","name":"Window Agg","parallelism":4}],
				"plan":{"nodes":[{"id":"v2","inputs":[{"id":"v1"}]},{"id":"v1","inputs":[]}]}}`)
		case "/jobs/overview":
			io.WriteString(w, `{"jobs":[{"jid":"flinkjob-1","name":"orders","state":"RUNNING"}]}`)
		case "/config":
			io.WriteString(w, `{"flink-version":"1.20.1"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// deploymentWithJobID builds a FlinkDeployment carrying a running Flink job id.
func deploymentWithJobID(name, flinkJobID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "flink.apache.org/v1beta1",
		"kind":       "FlinkDeployment",
		"metadata":   map[string]any{"name": name, "namespace": "default"},
		"spec":       map[string]any{"flinkVersion": "1.20.1"},
		"status":     map[string]any{"jobStatus": map[string]any{"state": "RUNNING", "jobId": flinkJobID}},
	}}
}

// restAdapter wires a fake-client adapter to a fake JobManager over the
// port-forward transport, so the REST delegation runs end to end.
func restAdapter(dep *unstructured.Unstructured, jmURL string) *Adapter {
	a := fakeAdapter(dep)
	a.transport = TransportPortForward
	a.pfEndpoint = jmURL
	return a
}

// TestGetJobGraphViaREST proves the graph is fetched from the JobManager REST
// API while the Kubernetes resource name stays the job identity.
func TestGetJobGraphViaREST(t *testing.T) {
	a := restAdapter(deploymentWithJobID("orders", "flinkjob-1"), jmServer(t).URL)

	graph, err := a.GetJobGraph(context.Background(), "orders")
	if err != nil {
		t.Fatalf("GetJobGraph: %v", err)
	}
	if graph.ID != "orders" || graph.Name != "orders" {
		t.Errorf("identity = %s/%s, want orders/orders (the k8s resource name)", graph.ID, graph.Name)
	}
	if len(graph.Operators) != 2 {
		t.Fatalf("got %d operators from REST, want 2", len(graph.Operators))
	}
	var agg model.Operator
	for _, o := range graph.Operators {
		if o.ID == "v2" {
			agg = o
		}
	}
	if len(agg.Upstreams) != 1 || agg.Upstreams[0] != "v1" {
		t.Errorf("v2 upstreams = %v, want [v1] (from the job plan)", agg.Upstreams)
	}
}

// TestGetJobGraphResolvesIDFromREST covers the fallback: when the CRD has no
// jobId yet, the single running job on the JobManager is used.
func TestGetJobGraphResolvesIDFromREST(t *testing.T) {
	dep := flinkDeployment("orders", "1.20.1", 4, "RUNNING") // no status.jobStatus.jobId
	a := restAdapter(dep, jmServer(t).URL)

	graph, err := a.GetJobGraph(context.Background(), "orders")
	if err != nil {
		t.Fatalf("GetJobGraph with id fallback: %v", err)
	}
	if len(graph.Operators) != 2 {
		t.Errorf("got %d operators, want 2 (id resolved from /jobs/overview)", len(graph.Operators))
	}
}

// TestSessionJobWithoutIDErrors covers the session-mode guard: a shared cluster
// runs many jobs, so a session job with no jobId cannot be resolved.
func TestSessionJobWithoutIDErrors(t *testing.T) {
	sj := flinkSessionJob("enrich", 2, "RUNNING") // no jobId in the helper
	a := restAdapter(sj, jmServer(t).URL)
	if _, err := a.GetJobGraph(context.Background(), "enrich"); err == nil {
		t.Fatal("expected an error resolving a session job with no jobId")
	}
}
