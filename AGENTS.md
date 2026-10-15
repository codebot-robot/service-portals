# Agent Guide: Service Portals

`service-portals` is a credential-swapping, layer-7 intercepting proxy designed to secure untrusted and semi-trusted sandboxes (such as AI agents, CI jobs, and code execution runners). It ensures that **real credentials never enter the sandbox container** ("tokens that aren't there can't leak"). Outbound traffic is transparently intercepted, decrypted using an ephemeral pod-local Root CA, matched against routing rules, and forwarded to external APIs with real authentication headers injected.

---

## Codebase Map

### Core Libraries (`pkg/`)
- **`pkg/proxy/`**: The foundational layer-7 proxy (`HTTPProxy`). Handles reverse proxying, request/response header sanitization (RFC 7230 hop-by-hop removal), explicit `HTTP CONNECT` hijacking, dynamic X.509 certificate generation signed by CA key pairs, and OpenTelemetry client transport wrapping.
- **`pkg/portals/`**: The multi-tenant routing engine. Implements `PortalRule` YAML unmarshaling, directory walking, lower-cased host routing, `SIGHUP` live rule reloading, dynamic SNI `GetCertificate` dispatch, and the unified server runner (`Run`).
- **`pkg/cache/`**: Caching infrastructure. Provides `InMemoryCache` and filesystem-backed `DiskCache` with SHA-256 key hashing, streaming file I/O, atomic temporary file replacement, and background janitor expiration sweeps.
- **`pkg/gitproxy/`**: Protocol-aware Git caching. Implements Git Smart HTTP protocols (v0, v1, v2), packfile and delta chain (`OFS_DELTA`, `REF_DELTA`) parsing, commit DAG reachability analysis, and dynamic packfile generation from cache based on client `have` lines.

### Applications (`cmd/` and service directories)
- **`cmd/all-in-one-portal/`**: The primary sidecar proxy binary. Uses `pkg/portals` to load `PortalRule` secrets from `--rules-dir`, listening on `:8080` (HTTP) and `:8443` (HTTPS) with SNI-based dynamic TLS minting.
- **`cmd/init-service-portals/`**: Setup binary for the pod `initContainer`. Generates ephemeral pod-local 2048-bit RSA Root CA key pairs, writes public certs to `/etc/service-portal/ca`, and writes private keys to `/etc/service-portal/ca-private`; executed by `entrypoint.sh` before it sets up iptables redirection and egress filtering.
- **`cmd/gitproxy-server/`**: Standalone caching Git proxy binary supporting disk/memory caches and upstream token injection.
- **`cmd/service-portal/`**: Single-target reverse proxy and HTTP proxy binary.
- **`gemini-portal/`**: Specialized portal for Google Gemini API (`generativelanguage.googleapis.com`) with JSON and gRPC protobuf request/response payload parsing and logging.
- **`anthropic-portal/`**: Specialized portal for Anthropic Messages API (`api.anthropic.com`) with system prompt and message logging, and gzip decompression.
- **`github-portal/`**: Specialized portal for GitHub API (`api.github.com`) with token injection and header redaction.

### Deployments, Tests, & Tasks
- **`k8s/sidecar.yaml`**: Authoritative production manifest for the transparent sidecar pattern (init container + sidecar proxy + workload + Secret rules).
- **`k8s/manifests.yaml`**: Manifest for deploying standalone cluster-wide service mode.
- **`tests/e2e/`**: KinD-based integration tests verifying both standalone service and transparent sidecar TLS interception and credential injection.
- **`dev/ci/presubmits/`**: CI validation scripts: `ap-build`, `ap-test`, `ap-lint`, `ap-verify-generate`, and `ap-e2e`.
- **`dev/tasks/`**: Local development tasks: `deploy-to-kube`, `create-test-certificates`, `test-e2e`.

---

## Essential Conventions & Architecture Principles

1. **Defense in Depth**: Real tokens live exclusively in Secret volume mounts attached to the proxy UID (`1337`). Workload containers run as non-root (e.g. UID `1000`) with `allowPrivilegeEscalation: false` to ensure they cannot run as UID `1337` and bypass iptables.
2. **Deterministic Interception**: Port 80 traffic redirects to 8080 (HTTP); port 443 redirects to 8443 (HTTPS). Egress filter blocks all other outbound TCP and UDP except DNS (port 53) and proxy UID 1337.
3. **No Code Changes to Applications**: Workloads use standard hostnames (via `hostAliases`) and standard client libraries, trusting only the pod-local public CA certificate.
4. **Verification**: Run `go test ./...` to verify all Go packages. Ensure code passes `dev/ci/presubmits/ap-lint` and `dev/ci/presubmits/ap-test`.
