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
	"fmt"
	"os"
	"strings"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/adapters"
)

const (
	// envProfile selects a profile when --profile is not given.
	envProfile = "FLINKIT_PROFILE"
	// envEndpoint overrides the resolved profile's endpoint outright.
	envEndpoint = "FLINKIT_ENDPOINT"
)

// Resolve selects a profile and applies environment overrides.
//
// Selection precedence: the flag, then FLINKIT_PROFILE, then the config's
// default. Field handling: any ${VAR} in the endpoint or option values is
// expanded from the environment (so secrets stay out of the file), and
// FLINKIT_ENDPOINT, if set, replaces the endpoint outright.
func (c *Config) Resolve(flagProfile string) (Profile, error) {
	name := flagProfile
	if name == "" {
		name = os.Getenv(envProfile)
	}
	if name == "" {
		name = c.Default
	}
	if name == "" {
		return Profile{}, fmt.Errorf("config: no profile given and no default set")
	}

	p, ok := c.Profiles[name]
	if !ok {
		// viper lowercases config keys, so a mixed-case --profile may need the
		// lowercased form to match.
		p, ok = c.Profiles[strings.ToLower(name)]
	}
	if !ok {
		return Profile{}, fmt.Errorf("config: profile %q not found", name)
	}

	p.Endpoint = os.ExpandEnv(p.Endpoint)
	if len(p.Options) > 0 {
		opts := make(map[string]string, len(p.Options))
		for k, v := range p.Options {
			opts[k] = os.ExpandEnv(v)
		}
		p.Options = opts
	}
	if ep := os.Getenv(envEndpoint); ep != "" {
		p.Endpoint = ep
	}

	return p, nil
}

// AdapterConfig converts the resolved profile into the connection config an
// adapter factory consumes.
func (p Profile) AdapterConfig() adapters.Config {
	return adapters.Config{Endpoint: p.Endpoint, Options: p.Options}
}
