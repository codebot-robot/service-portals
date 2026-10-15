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

resource "google_filestore_instance" "kellnr_filestore" {
  count    = local.cfg.storage_backend == "filestore" ? 1 : 0
  name     = "kellnr-filestore"
  location = var.zone
  tier     = "STANDARD"

  file_shares {
    capacity_gb = 1024 # Filestore Standard tier requires a minimum of 1TiB (1024 GB)
    name        = "vol1"
  }

  networks {
    network = google_compute_network.kellnr_network.name
    modes   = ["MODE_IPV4"]
  }
}
