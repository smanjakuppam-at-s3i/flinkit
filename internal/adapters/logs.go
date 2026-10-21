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

import "context"

// LogLine is one recent log line with its source (e.g. "jobmanager").
type LogLine struct {
	Source string `json:"source"`
	Text   string `json:"text"`
}

// LogProvider is an optional capability: an adapter that can return recent log
// lines for a job implements it. Callers type-assert for it and degrade
// gracefully when an adapter cannot provide logs, so the base PlatformAdapter
// interface stays lean.
type LogProvider interface {
	GetLogs(ctx context.Context, jobID string, maxLines int) ([]LogLine, error)
}
