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

resource "random_password" "db_password" {
  length  = 16
  special = false
}

resource "google_sql_database_instance" "kellnr_postgres" {
  name             = "kellnr-postgres"
  database_version = "POSTGRES_15"
  region           = var.region

  # Set deletion protection to false for easier teardown of the test lab environment.
  deletion_protection = false

  settings {
    tier = local.cfg.database_tier

    ip_configuration {
      ipv4_enabled = true # Required for Cloud SQL Proxy to connect over public IP securely via IAM
    }


  }
}

resource "google_sql_database" "kellnr_db" {
  name     = "kellnr_db"
  instance = google_sql_database_instance.kellnr_postgres.name
}

resource "google_sql_user" "kellnr_user" {
  name     = "kellnr_user"
  instance = google_sql_database_instance.kellnr_postgres.name
  password = random_password.db_password.result
}
