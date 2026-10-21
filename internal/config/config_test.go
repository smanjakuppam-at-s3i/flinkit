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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleYAML = `
default: local-oss
profiles:
  local-oss:
    source: oss
    platform: oss
    endpoint: http://localhost:8081
  prod-cp:
    source: cp
    platform: cp
    endpoint: https://cmf.internal:8080
    options:
      token: ${FLINKIT_TEST_TOKEN}
`

func writeConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(sampleYAML), 0o600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

func TestResolveDefaultProfile(t *testing.T) {
	c, err := Load(writeConfig(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p, err := c.Resolve("") // empty flag, no env -> config default
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Source != "oss" || p.Endpoint != "http://localhost:8081" {
		t.Fatalf("default profile = %+v, want the local-oss profile", p)
	}
	if got := p.AdapterConfig().Endpoint; got != "http://localhost:8081" {
		t.Errorf("AdapterConfig endpoint = %q", got)
	}
}

func TestResolveFlagSelectsProfile(t *testing.T) {
	c, err := Load(writeConfig(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p, err := c.Resolve("prod-cp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Platform != "cp" {
		t.Errorf("platform = %q, want cp", p.Platform)
	}
}

func TestResolveSecretsFromEnv(t *testing.T) {
	t.Setenv("FLINKIT_TEST_TOKEN", "s3cr3t")
	c, err := Load(writeConfig(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p, err := c.Resolve("prod-cp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := p.Options["token"]; got != "s3cr3t" {
		t.Errorf("token = %q, want the expanded env value (secret not stored in file)", got)
	}
}

func TestResolveProfileFromEnvAndEndpointOverride(t *testing.T) {
	t.Setenv(envProfile, "prod-cp")
	t.Setenv(envEndpoint, "https://override:9090")
	c, err := Load(writeConfig(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p, err := c.Resolve("") // empty flag -> FLINKIT_PROFILE picks prod-cp
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Platform != "cp" {
		t.Errorf("env-selected profile = %q, want cp", p.Platform)
	}
	if p.Endpoint != "https://override:9090" {
		t.Errorf("endpoint = %q, want the FLINKIT_ENDPOINT override", p.Endpoint)
	}
}

func TestResolveUnknownProfile(t *testing.T) {
	c, err := Load(writeConfig(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := c.Resolve("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestLoadDefaultToleratesMissingFile(t *testing.T) {
	// Point HOME at an empty dir so the default config is absent.
	t.Setenv("HOME", t.TempDir())
	c, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault with no file: %v", err)
	}
	if len(c.Profiles) != 0 {
		t.Errorf("expected empty config, got %+v", c)
	}
}
