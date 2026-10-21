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

	"k8s.io/client-go/rest"
)

func TestParseTransport(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    Transport
		wantErr bool
	}{
		{"", TransportAuto, false},
		{"auto", TransportAuto, false},
		{"direct", TransportDirect, false},
		{"proxy", TransportProxy, false},
		{"port-forward", TransportPortForward, false},
		{" direct ", TransportDirect, false},
		{"bogus", "", true},
	} {
		got, err := parseTransport(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseTransport(%q) = nil error, want error", tc.in)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("parseTransport(%q) = (%q, %v), want (%q, nil)", tc.in, got, err, tc.want)
		}
	}
}

func TestResolveTransport(t *testing.T) {
	if got := resolveTransport(TransportAuto, true); got != TransportDirect {
		t.Errorf("auto in-cluster = %q, want direct", got)
	}
	if got := resolveTransport(TransportAuto, false); got != TransportProxy {
		t.Errorf("auto out-of-cluster = %q, want proxy", got)
	}
	// Explicit modes are never overridden by cluster location.
	if got := resolveTransport(TransportProxy, true); got != TransportProxy {
		t.Errorf("explicit proxy in-cluster = %q, want proxy", got)
	}
}

func TestDirectBaseURL(t *testing.T) {
	ep := RESTEndpoint{Namespace: "flink", Service: "orders-rest", Port: 8081}
	if got := directBaseURL(ep); got != "http://orders-rest.flink:8081" {
		t.Errorf("directBaseURL = %q", got)
	}
	// No namespace: bare service name (resolves inside its own namespace).
	ep.Namespace = ""
	if got := directBaseURL(ep); got != "http://orders-rest:8081" {
		t.Errorf("directBaseURL without namespace = %q", got)
	}
}

func TestProxyBaseURL(t *testing.T) {
	ep := RESTEndpoint{Namespace: "flink", Service: "orders-rest", Port: 8081}
	want := "https://api.example.com/api/v1/namespaces/flink/services/http:orders-rest:8081/proxy"
	if got := proxyBaseURL("https://api.example.com", ep); got != want {
		t.Errorf("proxyBaseURL = %q, want %q", got, want)
	}
	// A trailing slash on the apiserver host is trimmed.
	if got := proxyBaseURL("https://api.example.com/", ep); got != want {
		t.Errorf("proxyBaseURL with trailing slash = %q, want %q", got, want)
	}
}

func TestPortForwardBaseURL(t *testing.T) {
	if got := portForwardBaseURL("", 8081); got != "http://localhost:8081" {
		t.Errorf("portForwardBaseURL default = %q", got)
	}
	if got := portForwardBaseURL("http://127.0.0.1:9000/", 8081); got != "http://127.0.0.1:9000" {
		t.Errorf("portForwardBaseURL explicit = %q", got)
	}
}

func TestRestBaseURLAndClientDirect(t *testing.T) {
	a := &Adapter{transport: TransportDirect}
	ep := RESTEndpoint{Namespace: "flink", Service: "orders-rest", Port: 8081}
	base, client, err := a.restBaseURLAndClient(ep)
	if err != nil {
		t.Fatalf("restBaseURLAndClient: %v", err)
	}
	if base != "http://orders-rest.flink:8081" || client == nil {
		t.Errorf("direct = (%q, %v)", base, client)
	}
}

func TestRestBaseURLAndClientProxy(t *testing.T) {
	a := &Adapter{transport: TransportProxy, restConfig: &rest.Config{Host: "https://api.example.com"}}
	ep := RESTEndpoint{Namespace: "flink", Service: "orders-rest", Port: 8081}
	base, client, err := a.restBaseURLAndClient(ep)
	if err != nil {
		t.Fatalf("restBaseURLAndClient proxy: %v", err)
	}
	want := "https://api.example.com/api/v1/namespaces/flink/services/http:orders-rest:8081/proxy"
	if base != want || client == nil {
		t.Errorf("proxy = (%q, %v), want base %q", base, client, want)
	}
}

func TestRestBaseURLAndClientProxyNeedsConfig(t *testing.T) {
	a := &Adapter{transport: TransportProxy} // no restConfig
	if _, _, err := a.restBaseURLAndClient(RESTEndpoint{Service: "x-rest", Port: 8081}); err == nil {
		t.Fatal("proxy without a cluster connection should error")
	}
}

func TestRestBaseURLAndClientPortForward(t *testing.T) {
	a := &Adapter{transport: TransportPortForward, pfEndpoint: "http://localhost:8082"}
	base, _, err := a.restBaseURLAndClient(RESTEndpoint{Service: "x-rest", Port: 8081})
	if err != nil {
		t.Fatalf("restBaseURLAndClient port-forward: %v", err)
	}
	if base != "http://localhost:8082" {
		t.Errorf("port-forward base = %q, want the configured endpoint", base)
	}
}
