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

variable "enable_oauth2" {
  type        = bool
  default     = false
  description = "Enable Google OIDC OAuth2 authentication in Kellnr."
}

variable "kellnr_oauth2_client_id" {
  type        = string
  default     = "dummy-client-id"
  description = "Google OAuth2 Client ID (required if enable_oauth2 is true)."
}

variable "kellnr_oauth2_client_secret" {
  type        = string
  default     = "dummy-client-secret"
  description = "Google OAuth2 Client Secret (required if enable_oauth2 is true)."
}

variable "kellnr_oauth2_issuer" {
  type        = string
  default     = "https://accounts.google.com"
  description = "The OIDC issuer URL."
}
