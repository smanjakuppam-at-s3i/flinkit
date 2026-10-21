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

// Statement is a SQL statement discovered on a platform or in a repo. JobID
// links it to the Job it produced when that relationship is known.
type Statement struct {
	Name     string   `json:"name,omitempty"`
	SQL      string   `json:"sql"`
	Platform Platform `json:"platform,omitempty"`
	Catalog  string   `json:"catalog,omitempty"`
	Database string   `json:"database,omitempty"`
	JobID    string   `json:"jobId,omitempty"`
}
