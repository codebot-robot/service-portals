# Git Proxy Server (`cmd/gitproxy-server`)

`gitproxy-server` is an intelligent, caching Git HTTP proxy designed to accelerate Git operations (clone, fetch, pull) and inject upstream authentication credentials for sandboxed workloads and CI pipelines.

---

## Features

- **Protocol Support**: Implements Git Smart HTTP protocol versions **v0**, **v1**, and **v2**.
- **Packfile & Delta Resolution**: Parses and resolves Git packfiles including `OFS_DELTA` and `REF_DELTA` objects.
- **Commit DAG & Reachability Analysis**: Tracks commit history and object reachability per repository. When clients send `have` lines during incremental fetches, the proxy prunes objects already reachable by the client, generates a dynamic packfile from cache, and fetches only missing objects from upstream.
- **Cache Storage**: Supports both disk-backed caching (`--cache-dir`) and in-memory caching with configurable TTL and background janitor sweeps.
- **Credential Injection**: Injects upstream credentials into the `Authorization` header (or a custom header).
- **Observability**: Built-in OpenTelemetry HTTP transport instrumentation (`OTEL_INSTRUMENTATION_ENABLED=true`).

---

## Command-Line Flags & Environment Variables

Flags take precedence over environment variables.

| Flag | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- |
| `--port` | `PORT` | `8080` | Port to listen on. |
| `--target-url` | `TARGET_URL` | `""` | Default upstream Git server URL for relative paths. |
| `--cache-dir` | `CACHE_DIR` | `""` | Directory for disk-backed packfile and response cache. Uses in-memory cache if empty. |
| `--cache-ttl` | `CACHE_TTL` | `168h` (7 days) | Cache time-to-live duration. |
| `--cache-cleanup-interval` | `CACHE_CLEANUP_INTERVAL` | `10m` | Interval for purging expired cache entries. |
| `--upstream-auth-token` | `UPSTREAM_AUTH_TOKEN` | `""` | Upstream token injected into Git requests. |
| `--upstream-auth-header` | `UPSTREAM_AUTH_HEADER` | `Authorization` | Header name for token injection. |
| — | `OTEL_INSTRUMENTATION_ENABLED` | `""` | Set to `"true"` to enable OpenTelemetry tracing. |

---

## URL Resolution

`gitproxy-server` resolves the target repository URL from:
1. **Full URL**: `http://gitproxy:8080/https://github.com/org/repo.git`
2. **Host prefix**: `http://gitproxy:8080/github.com/org/repo.git`
3. **Relative path**: `http://gitproxy:8080/org/repo.git` (resolved against `--target-url`)
