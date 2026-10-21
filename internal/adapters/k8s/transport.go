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
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/client-go/rest"
)

// restTimeout bounds a single JobManager REST call. It is generous enough to
// absorb the extra hop the proxy transport adds through the apiserver.
const restTimeout = 15 * time.Second

// Transport selects how the adapter reaches a JobManager REST API once a job is
// discovered. The REST logic itself is identical across transports (see the OSS
// adapter); only the base URL and HTTP client differ.
type Transport string

const (
	// TransportAuto picks direct in-cluster and proxy out-of-cluster.
	TransportAuto Transport = "auto"
	// TransportDirect talks to the service DNS name (<svc>.<ns>:<port>) — for
	// flinkit running inside the cluster, or any already-reachable endpoint.
	TransportDirect Transport = "direct"
	// TransportProxy tunnels through the Kubernetes apiserver's service proxy,
	// reusing kubeconfig auth/TLS — for flinkit running outside the cluster
	// without a port-forward or ingress.
	TransportProxy Transport = "proxy"
	// TransportPortForward talks to a locally forwarded port (single-job dev).
	TransportPortForward Transport = "port-forward"
)

// parseTransport validates the Options["via"] value, defaulting to auto.
func parseTransport(s string) (Transport, error) {
	switch t := Transport(strings.TrimSpace(s)); t {
	case "", TransportAuto:
		return TransportAuto, nil
	case TransportDirect, TransportProxy, TransportPortForward:
		return t, nil
	default:
		return "", fmt.Errorf("k8s: unknown --via %q (want auto|direct|proxy|port-forward)", s)
	}
}

// resolveTransport turns auto into a concrete transport: direct when running
// inside the cluster (service DNS resolves), proxy otherwise.
func resolveTransport(mode Transport, inCluster bool) Transport {
	if mode == TransportAuto {
		if inCluster {
			return TransportDirect
		}
		return TransportProxy
	}
	return mode
}

// inClusterEnv reports whether flinkit is running inside a Kubernetes pod, using
// the service-account environment the kubelet injects.
func inClusterEnv() bool {
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

// directBaseURL addresses the JobManager REST service by DNS name. Inside the
// cluster "<svc>.<ns>" resolves to the ClusterIP; the operator serves REST over
// plain HTTP.
func directBaseURL(ep RESTEndpoint) string {
	host := ep.Service
	if ep.Namespace != "" {
		host += "." + ep.Namespace
	}
	return fmt.Sprintf("http://%s:%d", host, ep.Port)
}

// proxyBaseURL builds the apiserver service-proxy path for the endpoint. The
// "http:" scheme prefix tells the apiserver to reach the backend service over
// HTTP; the OSS adapter then appends "/jobs/..." after "/proxy".
func proxyBaseURL(apiserverHost string, ep RESTEndpoint) string {
	return fmt.Sprintf("%s/api/v1/namespaces/%s/services/http:%s:%d/proxy",
		strings.TrimRight(apiserverHost, "/"), ep.Namespace, ep.Service, ep.Port)
}

// portForwardBaseURL uses an explicit endpoint if one is configured, otherwise
// localhost on the JobManager port (the usual `kubectl port-forward` target).
func portForwardBaseURL(endpoint string, port int) string {
	if e := strings.TrimSpace(endpoint); e != "" {
		return strings.TrimRight(e, "/")
	}
	return "http://localhost:" + strconv.Itoa(port)
}

// restBaseURLAndClient resolves the base URL and HTTP client for reaching the
// endpoint's JobManager REST API under the adapter's transport. The OSS REST
// adapter (oss.NewWithClient) runs unchanged on top of whatever this returns.
func (a *Adapter) restBaseURLAndClient(ep RESTEndpoint) (string, *http.Client, error) {
	switch mode := resolveTransport(a.transport, inClusterEnv()); mode {
	case TransportDirect:
		return directBaseURL(ep), &http.Client{Timeout: restTimeout}, nil
	case TransportPortForward:
		return portForwardBaseURL(a.pfEndpoint, ep.Port), &http.Client{Timeout: restTimeout}, nil
	case TransportProxy:
		if a.restConfig == nil {
			return "", nil, fmt.Errorf("k8s: proxy transport needs a cluster connection (no kubeconfig loaded)")
		}
		client, err := rest.HTTPClientFor(a.restConfig)
		if err != nil {
			return "", nil, fmt.Errorf("k8s: building apiserver proxy client: %w", err)
		}
		client.Timeout = restTimeout
		return proxyBaseURL(a.restConfig.Host, ep), client, nil
	default:
		return "", nil, fmt.Errorf("k8s: unknown transport %q", mode)
	}
}
