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
	"io"

	"github.com/smanjakuppam-at-s3i/flinkit/internal/model"
	"github.com/smanjakuppam-at-s3i/flinkit/internal/rules"
)

// markdownRenderer is a placeholder. Shareable Markdown reports (via Mustache
// or an equivalent) arrive with the P1 doctor reports; the type exists now so
// --format md resolves to a renderer rather than an unknown-format error.
type markdownRenderer struct{}

func (markdownRenderer) RenderInventory(w io.Writer, inv model.Inventory) error {
	return ErrFormatNotImplemented
}

func (markdownRenderer) RenderFindings(w io.Writer, findings rules.Findings) error {
	return ErrFormatNotImplemented
}

func (markdownRenderer) RenderFleet(w io.Writer, results []JobFindings) error {
	return ErrFormatNotImplemented
}

func (markdownRenderer) RenderBottleneck(w io.Writer, report BottleneckReport) error {
	return ErrFormatNotImplemented
}

func (markdownRenderer) RenderInspect(w io.Writer, snap InspectSnapshot) error {
	return ErrFormatNotImplemented
}

// htmlRenderer is a placeholder, like markdownRenderer.
type htmlRenderer struct{}

func (htmlRenderer) RenderInventory(w io.Writer, inv model.Inventory) error {
	return ErrFormatNotImplemented
}

func (htmlRenderer) RenderFindings(w io.Writer, findings rules.Findings) error {
	return ErrFormatNotImplemented
}

func (htmlRenderer) RenderFleet(w io.Writer, results []JobFindings) error {
	return ErrFormatNotImplemented
}

func (htmlRenderer) RenderBottleneck(w io.Writer, report BottleneckReport) error {
	return ErrFormatNotImplemented
}

func (htmlRenderer) RenderInspect(w io.Writer, snap InspectSnapshot) error {
	return ErrFormatNotImplemented
}
