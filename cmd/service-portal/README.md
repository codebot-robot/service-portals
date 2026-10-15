# Service Portal (`cmd/service-portal`)

`service-portal` is the generic single-target reverse proxy and HTTP proxy server. It is typically deployed as a standalone Kubernetes Deployment and `Service` (explicit service mode) or used as a standard HTTP/HTTPS proxy.

---

## Features

- **Reverse Proxy**: Forwards requests to a single upstream target configured by `TARGET_URL`, injecting `UPSTREAM_AUTH_TOKEN` into `UPSTREAM_AUTH_HEADER`.
- **Explicit HTTP Proxy & CONNECT Support**: When configured with `CA_CERT_PATH` and `CA_KEY_PATH`, handles `HTTP CONNECT` tunnels from clients using `HTTPS_PROXY`. The proxy intercepts the tunnel, dynamically mints a TLS certificate for the target host, and injects authentication headers if the host matches `TARGET_URL`.
- **Response Caching**: Optional response caching with configurable `CACHE_TTL` and cleanup interval.
- **Rules Support**: Can optionally load `PortalRule` YAML files from `--rules-dir` / `RULES_DIR` to support multi-host routing.
- **OpenTelemetry Instrumentation**: Supports W3C trace context extraction and propagation when `OTEL_INSTRUMENTATION_ENABLED=true`.

---

## Configuration

| Environment Variable / Flag | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | Port to listen on. |
| `HTTPS_PORT` | `8443` | Port for TLS connections (requires `CA_CERT_PATH` and `CA_KEY_PATH` to sign certificates). |
| `TARGET_URL` | `https://generativelanguage.googleapis.com` | Target upstream URL. |
| `UPSTREAM_AUTH_TOKEN` | `""` | Authentication token injected into upstream requests. |
| `UPSTREAM_AUTH_HEADER` | `Authorization` | Header used for injecting the authentication token. |
| `RULES_DIR` / `--rules-dir` | `""` | Directory containing `PortalRule` YAML files. |
| `CA_CERT_PATH` | `""` | Path to Root CA certificate PEM file. |
| `CA_KEY_PATH` | `""` | Path to Root CA private key PEM file. |
| `CACHE_TTL` | `""` | Cache TTL duration string. |
| `CACHE_CLEANUP_INTERVAL` | `1m` | Cache cleanup janitor interval. |
| `OTEL_INSTRUMENTATION_ENABLED` | `""` | Set to `"true"` to enable OpenTelemetry tracing. |
