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

package model

import (
	"bytes"
	"reflect"
	"testing"
)

// sampleInventory populates every entity and field so the round-trip exercises
// the whole model, not just the common path.
func sampleInventory() Inventory {
	return Inventory{
		SchemaVersion: InventorySchemaVersion,
		GeneratedAt:   "2026-10-10T12:00:00Z",
		Source:        "oss",
		Jobs: []Job{{
			ID:           "job-1",
			Name:         "orders-enrichment",
			Platform:     PlatformOSS,
			FlinkVersion: ParseVersion("1.20.1"),
			State:        JobStateRunning,
			Parallelism:  4,
			Config:       map[string]string{"state.backend": "rocksdb"},
			Operators: []Operator{{
				ID:          "op-src",
				Name:        "Kafka Source",
				Parallelism: 4,
			}, {
				ID:          "op-agg",
				UID:         "window-agg",
				Name:        "Window Agg",
				Parallelism: 4,
				Upstreams:   []string{"op-src"},
				Subtasks: []Subtask{{
					Index:                 7,
					BusyMsPerSec:          980,
					BackpressuredMsPerSec: 0,
					IdleMsPerSec:          20,
					RecordsIn:             1_000_000,
					RecordsOut:            600_000,
				}},
				State: []StateDescriptor{{
					Name:      "windowContents",
					Kind:      "list",
					Backend:   "rocksdb",
					TTL:       "",
					SizeBytes: 4096,
				}},
			}},
		}},
		Statements: []Statement{{
			Name:     "enrich",
			SQL:      "INSERT INTO sink SELECT * FROM src",
			Platform: PlatformOSS,
			Catalog:  "default_catalog",
			Database: "default_database",
			JobID:    "job-1",
		}},
		Connectors: []Connector{{
			Name:    "orders",
			Type:    "kafka",
			Format:  "avro",
			Mode:    ConnectorModeSource,
			Version: "3.7",
		}},
	}
}

func TestInventoryJSONRoundTrip(t *testing.T) {
	want := sampleInventory()

	var buf bytes.Buffer
	if err := want.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	got, err := ReadInventory(&buf)
	if err != nil {
		t.Fatalf("ReadInventory: %v", err)
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip mismatch:\n want %+v\n got  %+v", want, got)
	}
}

func TestReadInventoryRejectsUnknownField(t *testing.T) {
	const junk = `{"schemaVersion":1,"surpriseField":true}`
	if _, err := ReadInventory(bytes.NewReader([]byte(junk))); err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestNewInventoryStampsSchemaVersion(t *testing.T) {
	if got := NewInventory().SchemaVersion; got != InventorySchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", got, InventorySchemaVersion)
	}
}
