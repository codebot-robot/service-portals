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

FROM golang:1.27.2 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /dra-portal-driver ./cmd/dra-portal-driver
RUN CGO_ENABLED=0 GOOS=linux go build -o /portal-cdi-hook ./cmd/portal-cdi-hook

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    iptables \
    iproute2 \
    util-linux \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /dra-portal-driver /usr/local/bin/dra-portal-driver
COPY --from=builder /portal-cdi-hook /usr/local/bin/portal-cdi-hook

ENTRYPOINT ["/usr/local/bin/dra-portal-driver"]
