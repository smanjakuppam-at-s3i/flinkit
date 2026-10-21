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

// Platform is the Flink flavor a job runs on. Kubernetes and local repos are
// discovery sources for OSS Flink, not platforms in their own right.
type Platform string

const (
	PlatformUnknown Platform = ""    // not yet determined
	PlatformCC      Platform = "cc"  // Confluent Cloud
	PlatformCP      Platform = "cp"  // Confluent Platform
	PlatformOSS     Platform = "oss" // open-source Apache Flink
)

// Valid reports whether p is a recognized platform.
func (p Platform) Valid() bool {
	switch p {
	case PlatformCC, PlatformCP, PlatformOSS:
		return true
	default:
		return false
	}
}
