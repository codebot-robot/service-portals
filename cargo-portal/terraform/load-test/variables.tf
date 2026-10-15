# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

variable "concurrency" {
  type        = number
  default     = 90
  description = "Number of concurrent workers per stress pod."
}

variable "duration" {
  type        = number
  default     = 45
  description = "Duration of the stress test run in seconds."
}

variable "parallelism" {
  type        = number
  default     = 16
  description = "Number of stress generator pods to run concurrently."
}

variable "completions" {
  type        = number
  default     = 16
  description = "Total number of successful pod completions for the Job."
}

variable "registry_path" {
  type        = string
  default     = "/api/v1/cratesio"
  description = "The target registry path to test (e.g. /api/v1/crates for private registry, or /api/v1/cratesio for crates.io proxy fallback)."
}

variable "node_selector" {
  type        = map(string)
  default     = {
    "cloud.google.com/machine-family" = "n2"
  }
  description = "Additional node selectors for load generator pods (e.g. {'cloud.google.com/machine-family' = 'n2'})."
}
