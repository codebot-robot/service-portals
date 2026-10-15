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

variable "project_id" {
  type        = string
  description = "The GCP project ID to deploy resources to."
}

variable "region" {
  type        = string
  default     = "us-central1"
  description = "The GCP region to deploy resources to."
}

variable "zone" {
  type        = string
  default     = "us-central1-a"
  description = "The GCP zone to deploy zonal resources (like Filestore)."
}

variable "deployment_option" {
  type        = string
  default     = "gke_cdn_gcs"
  description = "The deployment option to run: gke_cdn_gcs, gke_cdn_filestore, gke_only_2node_gcs, gke_only_2node_filestore, gke_only_1node_gcs, gke_only_1node_filestore"

  validation {
    condition     = contains(["gke_cdn_gcs", "gke_cdn_filestore", "gke_only_2node_gcs", "gke_only_2node_filestore", "gke_only_1node_gcs", "gke_only_1node_filestore"], var.deployment_option)
    error_message = "Deployment option must be one of: gke_cdn_gcs, gke_cdn_filestore, gke_only_2node_gcs, gke_only_2node_filestore, gke_only_1node_gcs, gke_only_1node_filestore."
  }
}

variable "allowed_ip_ranges" {
  type        = list(string)
  default     = []
  description = "IP ranges allowed to access the Ingress. Used by Cloud Armor. Set to specific corporate ranges to restrict access."
}
