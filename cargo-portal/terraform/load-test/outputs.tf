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

output "load_test_bucket_name" {
  value = google_storage_bucket.load_test_bucket.name
}

output "stress_instructions" {
  value = <<EOF

Stress testing infrastructure configured!

To run the load simulations:
1. Compile the stress-test binary locally for Linux x86_64:
   cargo build --release --manifest-path ../../stress-test/Cargo.toml

2. Copy the compiled binary and packages.txt into the GCS load-test bucket:
   gcloud storage cp ../../stress-test/target/release/stress-test gs://${google_storage_bucket.load_test_bucket.name}/stress-test

   (Optional) Fetch and copy the top 1000 popular crates to load-test real-world proxy caching:
   python3 ../../stress-test/get_popular_crates.py 1000
   gcloud storage cp packages.txt gs://${google_storage_bucket.load_test_bucket.name}/packages.txt

3. Trigger the stress test by running:
   terraform apply

4. Once the job completes, you can copy the CSV results back from the GCS bucket to analyze them:
   gcloud storage cp -r gs://${google_storage_bucket.load_test_bucket.name}/results/ .

EOF
}
