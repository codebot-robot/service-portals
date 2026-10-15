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

resource "random_id" "bucket_suffix" {
  byte_length = 4
}

resource "google_storage_bucket" "kellnr_bucket" {
  count         = local.cfg.storage_backend == "gcs" ? 1 : 0
  name          = "kellnr-crates-${var.project_id}-${random_id.bucket_suffix.hex}"
  location      = var.region
  force_destroy = true # Allow cleanup of all storage objects on destroy

  public_access_prevention    = "enforced"
  uniform_bucket_level_access = true
}

resource "google_storage_hmac_key" "kellnr_hmac_key" {
  count                 = local.cfg.storage_backend == "gcs" ? 1 : 0
  service_account_email = google_service_account.kellnr_gsa.email
  project               = var.project_id
}

resource "google_storage_bucket_iam_member" "gsa_bucket_access" {
  count  = local.cfg.storage_backend == "gcs" ? 1 : 0
  bucket = google_storage_bucket.kellnr_bucket[0].name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.kellnr_gsa.email}"
}
