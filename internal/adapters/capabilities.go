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

package adapters

import "github.com/smanjakuppam-at-s3i/flinkit/internal/model"

// Support is a tri-state describing whether a platform supports a feature. The
// Unknown state lets adapters and port-check degrade gracefully and flag
// uncertainty rather than guess, per the capability-matrix convention.
type Support string

const (
	SupportUnknown Support = "unknown"
	SupportYes     Support = "yes"
	SupportNo      Support = "no"
)

// Capabilities declares what a platform supports: SQL features, config
// options, metrics, and connectors. Port-check is largely a diff of two
// Capabilities against the inventory, and bottleneck analysis consults Metrics
// to know which signals it can rely on.
type Capabilities struct {
	Platform    model.Platform     `json:"platform"`
	Version     model.Version      `json:"version,omitempty"`
	SQLFeatures map[string]Support `json:"sqlFeatures,omitempty"`
	Options     map[string]Support `json:"options,omitempty"`
	Metrics     map[string]Support `json:"metrics,omitempty"`
	Connectors  map[string]Support `json:"connectors,omitempty"`
}

// SQLFeature returns the support level for a SQL feature, defaulting to Unknown
// when the adapter does not declare it.
func (c Capabilities) SQLFeature(name string) Support { return lookup(c.SQLFeatures, name) }

// Option returns the support level for a config option.
func (c Capabilities) Option(name string) Support { return lookup(c.Options, name) }

// Metric returns the support level for a metric.
func (c Capabilities) Metric(name string) Support { return lookup(c.Metrics, name) }

// Connector returns the support level for a connector.
func (c Capabilities) Connector(name string) Support { return lookup(c.Connectors, name) }

func lookup(m map[string]Support, key string) Support {
	if s, ok := m[key]; ok {
		return s
	}
	return SupportUnknown
}
