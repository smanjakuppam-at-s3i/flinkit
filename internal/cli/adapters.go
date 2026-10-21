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

package cli

// Blank imports register the in-core adapters with the default registry so
// --source can resolve them. Additional adapters (k8s, repo) and out-of-core
// plugins are added here as they land.
import (
	_ "github.com/smanjakuppam-at-s3i/flinkit/internal/adapters/k8s"
	_ "github.com/smanjakuppam-at-s3i/flinkit/internal/adapters/oss"
)
