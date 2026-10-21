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
	"encoding/json"
	"fmt"
	"io"
)

// InventorySchemaVersion is the version of the inventory file format. Bump it
// only on a breaking change to the serialized shape.
const InventorySchemaVersion = 1

// Inventory is the normalized result of `flinkit scan`: everything discovered
// about one or more environments, written to .flinkit/inventory.json. Every
// other command reads it.
//
// GeneratedAt is an RFC 3339 timestamp stamped by the command layer; the model
// package itself holds no clock so it stays deterministic and testable.
type Inventory struct {
	SchemaVersion int         `json:"schemaVersion"`
	GeneratedAt   string      `json:"generatedAt,omitempty"`
	Source        string      `json:"source,omitempty"` // adapter the data came from: cc|cp|oss|k8s|repo
	Jobs          []Job       `json:"jobs,omitempty"`
	Statements    []Statement `json:"statements,omitempty"`
	Connectors    []Connector `json:"connectors,omitempty"`
}

// NewInventory returns an empty inventory stamped with the current schema
// version.
func NewInventory() Inventory {
	return Inventory{SchemaVersion: InventorySchemaVersion}
}

// WriteJSON writes the inventory as indented JSON.
func (inv Inventory) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(inv); err != nil {
		return fmt.Errorf("encoding inventory: %w", err)
	}
	return nil
}

// ReadInventory decodes an inventory from JSON, rejecting unknown fields so a
// schema drift surfaces loudly rather than silently dropping data.
func ReadInventory(r io.Reader) (Inventory, error) {
	var inv Inventory
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&inv); err != nil {
		return Inventory{}, fmt.Errorf("decoding inventory: %w", err)
	}
	return inv, nil
}
