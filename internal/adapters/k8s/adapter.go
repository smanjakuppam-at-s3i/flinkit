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

// Package k8s implements PlatformAdapter against the Flink Kubernetes Operator.
// It discovers jobs from FlinkDeployment and FlinkSessionJob custom resources
// via the Kubernetes API, read with a dynamic, unstructured client — no
// dependency on the operator's Go module. Live data not in the CRD (the job
// graph, metrics, and logs) is fetched from each job's JobManager REST API,
// reusing the OSS REST adapter over a selectable transport (see transport.go):
// in-cluster service DNS, the apiserver proxy out-of-cluster, or a port-forward.
package k8s

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters/oss"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
)

const (
	crdGroup   = "flink.apache.org"
	crdVersion = "v1beta1"
)

// flinkDeploymentGVR and flinkSessionJobGVR are the operator's custom
// resources. A FlinkDeployment is a cluster+job; a FlinkSessionJob is a job on
// a shared session cluster.
var (
	flinkDeploymentGVR = schema.GroupVersionResource{
		Group:    crdGroup,
		Version:  crdVersion,
		Resource: "flinkdeployments",
	}
	flinkSessionJobGVR = schema.GroupVersionResource{
		Group:    crdGroup,
		Version:  crdVersion,
		Resource: "flinksessionjobs",
	}
)

// Adapter discovers Flink jobs from the Kubernetes API and reaches their
// JobManager REST APIs over a selectable transport (see transport.go).
type Adapter struct {
	client    dynamic.Interface
	namespace string

	// restConfig is the cluster connection; the proxy transport reuses its host
	// and auth/TLS to reach JobManager services through the apiserver. nil for
	// test adapters built with a fake client.
	restConfig *rest.Config
	// transport selects how JobManager REST is reached: auto, direct, proxy, or
	// port-forward.
	transport Transport
	// pfEndpoint is the explicit base URL for the port-forward transport (from
	// Config.Endpoint); empty means http://localhost:<port>.
	pfEndpoint string
}

var (
	_ adapters.PlatformAdapter = (*Adapter)(nil)
	_ adapters.LogProvider     = (*Adapter)(nil)
)

// init registers the adapter so `--source k8s` resolves it.
func init() { adapters.Register("k8s", New) }

// New builds a K8s adapter from a Config. Connection comes from a kubeconfig
// (Options["kubeconfig"] or the default rules, with Options["context"]) or, as
// a fallback, in-cluster config. The namespace is Options["namespace"] (default
// "default"). Options["via"] selects the JobManager REST transport:
// auto|direct|proxy|port-forward (default auto — see transport.go).
func New(cfg adapters.Config) (adapters.PlatformAdapter, error) {
	rc, err := restConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8s: building client config: %w", err)
	}
	dc, err := dynamic.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("k8s: building dynamic client: %w", err)
	}
	ns := cfg.Options["namespace"]
	if ns == "" {
		ns = "default"
	}
	transport, err := parseTransport(cfg.Options["via"])
	if err != nil {
		return nil, err
	}
	a := newWithClient(dc, ns)
	a.restConfig = rc
	a.transport = transport
	a.pfEndpoint = strings.TrimSpace(cfg.Endpoint)
	return a, nil
}

// newWithClient builds an adapter around an already-constructed dynamic client,
// used by tests with a fake client.
func newWithClient(client dynamic.Interface, namespace string) *Adapter {
	return &Adapter{client: client, namespace: namespace}
}

func restConfig(cfg adapters.Config) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kc := cfg.Options["kubeconfig"]; kc != "" {
		rules.ExplicitPath = kc
	}
	overrides := &clientcmd.ConfigOverrides{}
	if ctx := cfg.Options["context"]; ctx != "" {
		overrides.CurrentContext = ctx
	}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	if rc, err := cc.ClientConfig(); err == nil {
		return rc, nil
	}
	// No usable kubeconfig: fall back to in-cluster config.
	return rest.InClusterConfig()
}

// Name returns the adapter identifier.
func (a *Adapter) Name() string { return "k8s" }

// ListJobs returns every FlinkDeployment and FlinkSessionJob in the namespace
// as a job. FlinkDeployment is the primary resource (its errors surface);
// FlinkSessionJob is best-effort, since that CRD may not be installed.
func (a *Adapter) ListJobs(ctx context.Context) ([]model.Job, error) {
	deployments, err := a.client.Resource(flinkDeploymentGVR).Namespace(a.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("k8s: listing flinkdeployments in %q: %w", a.namespace, err)
	}
	var jobs []model.Job
	for i := range deployments.Items {
		jobs = append(jobs, mapResource(&deployments.Items[i]))
	}

	if sessionJobs, err := a.client.Resource(flinkSessionJobGVR).Namespace(a.namespace).List(ctx, metav1.ListOptions{}); err == nil {
		for i := range sessionJobs.Items {
			jobs = append(jobs, mapResource(&sessionJobs.Items[i]))
		}
	}
	return jobs, nil
}

// GetJob returns one job by name, trying FlinkDeployment then FlinkSessionJob.
func (a *Adapter) GetJob(ctx context.Context, jobID string) (model.Job, error) {
	u, err := a.resourceFor(ctx, jobID)
	if err != nil {
		return model.Job{}, err
	}
	return mapResource(u), nil
}

// resourceFor fetches a job's custom resource, trying FlinkDeployment then
// FlinkSessionJob.
func (a *Adapter) resourceFor(ctx context.Context, jobID string) (*unstructured.Unstructured, error) {
	if u, err := a.getResource(ctx, flinkDeploymentGVR, jobID); err == nil {
		return u, nil
	}
	if u, err := a.getResource(ctx, flinkSessionJobGVR, jobID); err == nil {
		return u, nil
	}
	return nil, fmt.Errorf("k8s: job %q not found as a FlinkDeployment or FlinkSessionJob in %q", jobID, a.namespace)
}

func (a *Adapter) getResource(ctx context.Context, gvr schema.GroupVersionResource, name string) (*unstructured.Unstructured, error) {
	return a.client.Resource(gvr).Namespace(a.namespace).Get(ctx, name, metav1.GetOptions{})
}

// mapResource maps a FlinkDeployment or FlinkSessionJob into the model. Fields
// absent on one kind (e.g. flinkVersion on a session job) map to zero values.
func mapResource(u *unstructured.Unstructured) model.Job {
	name := u.GetName()
	flinkVersion, _, _ := unstructured.NestedString(u.Object, "spec", "flinkVersion")
	parallelism, _, _ := unstructured.NestedInt64(u.Object, "spec", "job", "parallelism")
	state, _, _ := unstructured.NestedString(u.Object, "status", "jobStatus", "state")

	return model.Job{
		ID:           name,
		Name:         name,
		Platform:     model.PlatformOSS,
		FlinkVersion: model.ParseVersion(flinkVersion),
		State:        model.JobState(state),
		Parallelism:  int(parallelism),
	}
}

// GetJobGraph returns the job with its operator graph. The CRD carries no graph,
// so this fetches it from the JobManager REST API over the configured transport,
// keeping the Kubernetes identity (resource name) for the job.
func (a *Adapter) GetJobGraph(ctx context.Context, jobID string) (model.Job, error) {
	u, err := a.resourceFor(ctx, jobID)
	if err != nil {
		return model.Job{}, err
	}
	meta := mapResource(u)
	client, ep, err := a.restClientForResource(u)
	if err != nil {
		return model.Job{}, err
	}
	fid, err := a.resolveFlinkJobID(ctx, client, ep, jobID)
	if err != nil {
		return model.Job{}, err
	}
	graph, err := client.GetJobGraph(ctx, fid)
	if err != nil {
		return model.Job{}, fmt.Errorf("k8s: job graph for %q via REST: %w", jobID, err)
	}
	// Prefer the Kubernetes identity; keep REST values where the CRD has none.
	graph.ID = meta.ID
	graph.Name = meta.Name
	graph.Platform = meta.Platform
	if graph.State == "" {
		graph.State = meta.State
	}
	if graph.FlinkVersion.String() == "" {
		graph.FlinkVersion = meta.FlinkVersion
	}
	return graph, nil
}

// GetMetrics returns job-scoped metrics from the JobManager REST API. The CRD
// carries none, so this delegates over the configured transport.
func (a *Adapter) GetMetrics(ctx context.Context, jobID string) (adapters.Metrics, error) {
	client, fid, err := a.restForJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return client.GetMetrics(ctx, fid)
}

// GetSubtaskMetrics returns per-subtask rates for one operator from the
// JobManager REST API. operatorID is a Flink vertex id (from GetJobGraph).
func (a *Adapter) GetSubtaskMetrics(ctx context.Context, jobID, operatorID string) ([]model.Subtask, error) {
	client, fid, err := a.restForJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return client.GetSubtaskMetrics(ctx, fid, operatorID)
}

// GetLogs returns recent JobManager log lines via the REST adapter, implementing
// adapters.LogProvider so `inspect` works against k8s jobs. The JobManager log is
// cluster-wide, so jobID is only used to resolve which JobManager to read.
func (a *Adapter) GetLogs(ctx context.Context, jobID string, maxLines int) ([]adapters.LogLine, error) {
	client, _, err := a.restClient(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return client.GetLogs(ctx, jobID, maxLines)
}

// restClient resolves a job's JobManager REST endpoint from its CRD and returns
// an OSS REST adapter bound to it over the configured transport.
func (a *Adapter) restClient(ctx context.Context, jobID string) (*oss.Adapter, RESTEndpoint, error) {
	u, err := a.resourceFor(ctx, jobID)
	if err != nil {
		return nil, RESTEndpoint{}, err
	}
	return a.restClientForResource(u)
}

// restClientForResource builds the OSS REST adapter for an already-fetched CRD,
// resolving its endpoint and transport. The namespace falls back to the
// adapter's when the resource carries none.
func (a *Adapter) restClientForResource(u *unstructured.Unstructured) (*oss.Adapter, RESTEndpoint, error) {
	ep, err := resolveEndpoint(u)
	if err != nil {
		return nil, RESTEndpoint{}, err
	}
	if ep.Namespace == "" {
		ep.Namespace = a.namespace
	}
	base, httpClient, err := a.restBaseURLAndClient(ep)
	if err != nil {
		return nil, RESTEndpoint{}, err
	}
	return oss.NewWithClient(base, httpClient), ep, nil
}

// restForJob resolves both the REST client and the Flink job id to query within
// its JobManager.
func (a *Adapter) restForJob(ctx context.Context, jobID string) (*oss.Adapter, string, error) {
	client, ep, err := a.restClient(ctx, jobID)
	if err != nil {
		return nil, "", err
	}
	fid, err := a.resolveFlinkJobID(ctx, client, ep, jobID)
	if err != nil {
		return nil, "", err
	}
	return client, fid, nil
}

// resolveFlinkJobID determines the Flink job id to query. The CRD's
// status.jobStatus.jobId is authoritative when present. Otherwise, for an
// application-mode JobManager (one job), it falls back to the single running
// job the REST API reports; a shared session cluster runs many jobs, so the id
// is required there.
func (a *Adapter) resolveFlinkJobID(ctx context.Context, client *oss.Adapter, ep RESTEndpoint, jobID string) (string, error) {
	if ep.JobID != "" {
		return ep.JobID, nil
	}
	if ep.Session {
		return "", fmt.Errorf("k8s: session job %q has no status.jobStatus.jobId yet", jobID)
	}
	jobs, err := client.ListJobs(ctx)
	if err != nil {
		return "", fmt.Errorf("k8s: locating the running job for %q: %w", jobID, err)
	}
	var running []model.Job
	for _, j := range jobs {
		if j.State == model.JobStateRunning {
			running = append(running, j)
		}
	}
	switch {
	case len(running) == 1:
		return running[0].ID, nil
	case len(jobs) == 1:
		return jobs[0].ID, nil
	default:
		return "", fmt.Errorf("k8s: could not determine the Flink job id for %q (%d jobs on the JobManager)", jobID, len(jobs))
	}
}

// GetConfig returns the job's spec.flinkConfiguration from the CRD — the
// operator-applied configuration, which needs no REST call.
func (a *Adapter) GetConfig(ctx context.Context, jobID string) (map[string]string, error) {
	u, err := a.resourceFor(ctx, jobID)
	if err != nil {
		return nil, err
	}
	cfg, _, _ := unstructured.NestedStringMap(u.Object, "spec", "flinkConfiguration")
	return cfg, nil
}

// ListStatements returns nothing: SQL statements are not in the CRD.
func (a *Adapter) ListStatements(ctx context.Context) ([]model.Statement, error) {
	return nil, nil
}

// Capabilities declares the live runtime metrics supported. Discovery comes
// from the CRD, but the adapter reaches each job's JobManager REST API over the
// configured transport, so it exposes the same live metrics as the OSS adapter.
func (a *Adapter) Capabilities() adapters.Capabilities {
	return adapters.Capabilities{
		Platform: model.PlatformOSS,
		Metrics: map[string]adapters.Support{
			"busyTimeMsPerSecond":          adapters.SupportYes,
			"backPressuredTimeMsPerSecond": adapters.SupportYes,
			"checkpoints":                  adapters.SupportYes,
		},
	}
}
