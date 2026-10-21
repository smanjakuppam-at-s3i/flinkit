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
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// defaultRESTPort is the JobManager REST port the operator exposes unless
// rest.port is overridden in the job's flinkConfiguration.
const defaultRESTPort = 8081

// RESTEndpoint says how to reach a job's JobManager REST API and which Flink
// job to look at once connected. It is the bridge from a discovered CRD to the
// OSS REST adapter: a transport (R3) turns Namespace/Service/Port into a base
// URL, and JobID selects the job within that JobManager.
type RESTEndpoint struct {
	// Namespace is the Kubernetes namespace the JM REST service lives in.
	Namespace string
	// Service is the JobManager REST service name (always "<cluster>-rest").
	Service string
	// Port is the JM REST port (defaultRESTPort unless overridden).
	Port int
	// JobID is the Flink job id within that JobManager (status.jobStatus.jobId).
	// It may be empty before the job is running; the caller falls back to the
	// single running job the JobManager reports.
	JobID string
	// Session is true for a FlinkSessionJob, which shares one session cluster's
	// JobManager with other jobs — so many jobs resolve to the same Service.
	Session bool
}

// resolveEndpoint maps a FlinkDeployment or FlinkSessionJob to its JobManager
// REST endpoint. Application mode (FlinkDeployment) runs its own JobManager, so
// the service is "<name>-rest". Session mode (FlinkSessionJob) runs on a shared
// session cluster named by spec.deploymentName, so the service is
// "<deploymentName>-rest" and the same JobManager is shared across session jobs.
//
// It is pure — no cluster calls — so every mode is unit-testable from a CRD
// fixture. The namespace comes from the resource; the caller fills it from the
// adapter's namespace when the resource carries none (e.g. fake-client fixtures).
func resolveEndpoint(u *unstructured.Unstructured) (RESTEndpoint, error) {
	deploymentName, _, _ := unstructured.NestedString(u.Object, "spec", "deploymentName")
	session := u.GetKind() == "FlinkSessionJob" || deploymentName != ""

	cluster := u.GetName()
	if session {
		if deploymentName == "" {
			return RESTEndpoint{}, fmt.Errorf("k8s: session job %q has no spec.deploymentName", u.GetName())
		}
		cluster = deploymentName
	}
	if cluster == "" {
		return RESTEndpoint{}, fmt.Errorf("k8s: resource has no name to resolve a JobManager service from")
	}

	jobID, _, _ := unstructured.NestedString(u.Object, "status", "jobStatus", "jobId")

	return RESTEndpoint{
		Namespace: u.GetNamespace(),
		Service:   cluster + "-rest",
		Port:      restPort(u),
		JobID:     jobID,
		Session:   session,
	}, nil
}

// restPort reads rest.port from spec.flinkConfiguration, falling back to the
// default when it is absent or unparseable.
func restPort(u *unstructured.Unstructured) int {
	cfg, _, _ := unstructured.NestedStringMap(u.Object, "spec", "flinkConfiguration")
	if p, err := strconv.Atoi(cfg["rest.port"]); err == nil && p > 0 {
		return p
	}
	return defaultRESTPort
}
