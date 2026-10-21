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

package report

import (
	"encoding/json"
	"io"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

// jsonRenderer emits machine-readable JSON. It delegates to the model's own
// indented encoder so the serialized inventory shape stays identical whether
// it is written as a report or to .flinkit/inventory.json.
type jsonRenderer struct{}

func (jsonRenderer) RenderInventory(w io.Writer, inv model.Inventory) error {
	return inv.WriteJSON(w)
}

func (jsonRenderer) RenderFindings(w io.Writer, findings rules.Findings) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Findings rules.Findings `json:"findings"`
	}{Findings: findings})
}

func (jsonRenderer) RenderFleet(w io.Writer, results []JobFindings) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Jobs []JobFindings `json:"jobs"`
	}{Jobs: results})
}

func (jsonRenderer) RenderBottleneck(w io.Writer, report BottleneckReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func (jsonRenderer) RenderInspect(w io.Writer, snap InspectSnapshot) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}
