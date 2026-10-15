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

resource "google_container_cluster" "primary" {
  name     = "kellnr-cluster"
  location = var.region

  # Enable Autopilot
  enable_autopilot = true

  network    = google_compute_network.kellnr_network.name
  subnetwork = google_compute_subnetwork.kellnr_subnet.name

  ip_allocation_policy {
    # Autopilot manages cluster pod/services IP allocation automatically
  }

  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false # Keep the control plane public endpoint open so we can actuate K8s resources from the runner
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }

  # Necessary to wait for VPC and Subnets to be fully provisioned
  depends_on = [
    google_compute_subnetwork.kellnr_subnet,
    google_compute_router_nat.kellnr_nat
  ]
}

# Google Service Account (GSA) used by Kellnr pods
resource "google_service_account" "kellnr_gsa" {
  account_id   = "kellnr-gsa"
  display_name = "Kellnr GKE Workload Identity Service Account"
  project      = var.project_id
}

# Grant GSA access to connect to Cloud SQL
resource "google_project_iam_member" "gsa_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.kellnr_gsa.email}"
}

# Bind GSA to GKE KSA via Workload Identity User role
resource "google_service_account_iam_member" "gsa_workload_identity" {
  service_account_id = google_service_account.kellnr_gsa.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[kellnr/kellnr-ksa]"
}
