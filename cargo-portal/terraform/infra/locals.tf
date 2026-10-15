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

locals {
  options = {
    gke_cdn_gcs = {
      enable_cdn        = true
      storage_backend   = "gcs"
      database_tier     = "db-custom-1-3840"
      kellnr_replicas   = 3
      kellnr_cpu        = "200m"
      kellnr_memory     = "512Mi"
    }
    gke_cdn_filestore = {
      enable_cdn        = true
      storage_backend   = "filestore"
      database_tier     = "db-custom-1-3840"
      kellnr_replicas   = 3
      kellnr_cpu        = "200m"
      kellnr_memory     = "512Mi"
    }
    gke_only_2node_gcs = {
      enable_cdn        = false
      storage_backend   = "gcs"
      database_tier     = "db-custom-16-61440"
      kellnr_replicas   = 22
      kellnr_cpu        = "4"
      kellnr_memory     = "8Gi"
    }
    gke_only_2node_filestore = {
      enable_cdn        = false
      storage_backend   = "filestore"
      database_tier     = "db-custom-16-61440"
      kellnr_replicas   = 22
      kellnr_cpu        = "4"
      kellnr_memory     = "8Gi"
    }
    gke_only_1node_gcs = {
      enable_cdn        = false
      storage_backend   = "gcs"
      database_tier     = "db-custom-16-61440"
      kellnr_replicas   = 11
      kellnr_cpu        = "4"
      kellnr_memory     = "8Gi"
    }
    gke_only_1node_filestore = {
      enable_cdn        = false
      storage_backend   = "filestore"
      database_tier     = "db-custom-16-61440"
      kellnr_replicas   = 11
      kellnr_cpu        = "4"
      kellnr_memory     = "8Gi"
    }
  }

  cfg = local.options[var.deployment_option]
}
