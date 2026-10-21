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

import "testing"

func TestCapabilitiesLookupDefaultsUnknown(t *testing.T) {
	c := Capabilities{
		SQLFeatures: map[string]Support{"MATCH_RECOGNIZE": SupportYes},
		Options:     map[string]Support{"sql.state-ttl": SupportNo},
	}

	if got := c.SQLFeature("MATCH_RECOGNIZE"); got != SupportYes {
		t.Errorf("SQLFeature(MATCH_RECOGNIZE) = %q, want yes", got)
	}
	if got := c.Option("sql.state-ttl"); got != SupportNo {
		t.Errorf("Option(sql.state-ttl) = %q, want no", got)
	}
	// Undeclared keys default to Unknown across every category.
	if got := c.SQLFeature("WINDOW_TVF"); got != SupportUnknown {
		t.Errorf("undeclared SQLFeature = %q, want unknown", got)
	}
	if got := c.Metric("backPressuredTimeMsPerSecond"); got != SupportUnknown {
		t.Errorf("undeclared Metric = %q, want unknown", got)
	}
	if got := c.Connector("kafka"); got != SupportUnknown {
		t.Errorf("undeclared Connector = %q, want unknown", got)
	}
}
