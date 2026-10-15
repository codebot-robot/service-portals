# Deployment Modes

`service-portals` supports three primary deployment architectures to place the proxy in the traffic path between your workload and external services. This document compares the modes, explains how they work, and outlines their respective trade-offs.

---

## Comparison Matrix

| Feature | Transparent Sidecar Mode (Primary) | Explicit Service Mode | Explicit HTTP Proxy Mode |
| :--- | :--- | :--- | :--- |
| **Primary Binary** | `cmd/all-in-one-portal` + `cmd/init-service-portals` | `cmd/service-portal` (or service-specific portal) | `cmd/service-portal` / `cmd/all-in-one-portal` |
| **Application Changes** | **None** (talks to configured hostnames) | **URL change** (`http://gemini-portal/`) | **Env vars** (`HTTP_PROXY`, `HTTPS_PROXY`) |
| **K8s Privileges** | Requires `NET_ADMIN` for init container | Standard unprivileged pod | Standard unprivileged pod |
| **TLS Handling** | Intercepts port 443 via pod-local CA | Plain HTTP in-cluster; HTTPS upstream | Intercepts `CONNECT` via pod-local CA |
| **Egress Enforcement** | **Strict packet-level blocking** of other ports | None at pod level (relies on NetworkPolicy) | None at pod level (relies on NetworkPolicy) |
| **Secret Scope** | Scoped to individual pod | Shared across all pods accessing Service | Scoped to pod or shared |
| **Bypass Resistance** | **High** (iptables drops direct traffic) | Low (workload can still talk externally) | Low (workload can bypass proxy env vars) |

---

## 1. Transparent Sidecar Mode (Primary)

In transparent sidecar mode, each application pod runs two helper containers alongside the workload: an `init-service-portals` init container and an `all-in-one-portal` sidecar container.

```
+---------------------------------------------------------------------------------+
|                                     POD                                         |
|                                                                                 |
|  +--------------------+                               +----------------------+  |
|  | Workload Container |                               | Sidecar Proxy        |  |
|  | (UID 1000)         |                               | (UID 1337)           |  |
|  |                    |  Port 80 (HTTP) -> 8080       |                      |  |
|  | Talks to           |------------------------------>| Injects real secrets |  |
|  | gemini.backend     |  Port 443 (HTTPS) -> 8443     | from Secret mount    |  |
|  | (via hostAliases)  |------------------------------>| and proxies upstream |  |
|  +--------------------+                               +----------+-----------+  |
|                                                                  |              |
+------------------------------------------------------------------|--------------+
                                                                   v External API
```

### How it Works
1. **Init Container (`init-service-portals`)**:
   - Runs with `CAP_NET_ADMIN` privileges before application containers start.
   - Generates an ephemeral RSA-2048 Root CA certificate (`tls.crt`) in a shared `ca-cert` volume, and the CA private key (`tls.key`) in a `ca-private` volume chowned to UID `1337`.
   - Installs `iptables` rules:
     - Redirects port `80` to local port `8080` (HTTP).
     - Redirects port `443` to local port `8443` (HTTPS).
     - Excludes UID `1337` to avoid redirecting the proxy's own outbound requests.
     - Drops/rejects all other outbound TCP and UDP traffic from the pod (except DNS on port 53).
2. **Workload Container**:
   - Runs as non-root (e.g. UID `1000`) with `allowPrivilegeEscalation: false`.
   - Mounts `ca-cert` read-only to trust the pod-local CA.
   - Resolves fake hostnames (e.g. `gemini.backend`) to arbitrary IPs using pod `hostAliases`.
   - Makes requests without API keys or with dummy tokens.
3. **Sidecar Proxy (`all-in-one-portal`)**:
   - Runs as UID `1337`.
   - Mounts `ca-private` read-only to access the CA private key.
   - Loads `PortalRule` definitions from a mounted Secret volume.
   - Mints per-host TLS certificates dynamically when TLS connections arrive on port `8443`.
   - Injects the real credentials and proxies to the upstream service over public TLS.

### Best Used For
- **Sandboxed untrusted code (agents, CI jobs)** where you must guarantee that credentials cannot be stolen or bypassed, even if the code inside the workload is malicious or compromised.

---

## 2. Explicit Service Mode (Shared In-Cluster Gateway)

In explicit service mode, the portal runs as a standalone Kubernetes Deployment exposed via a standard Kubernetes `Service` (e.g., `gemini-portal`).

```
+-----------------------------------+             +----------------------------------+
| Workload Pod                      |             | Service Portal Deployment        |
|                                   |             |                                  |
| Makes plain HTTP request to:      |  HTTP :80   | Listens on :8080                 |
| http://gemini-portal/v1/...       |------------>| Injects UPSTREAM_AUTH_TOKEN      |
|                                   |             | Proxies to TARGET_URL            |
+-----------------------------------+             +-----------------+----------------+
                                                                    | Real TLS
                                                                    v
                                                          +--------------------------+
                                                          | External API             |
                                                          | (Google, Anthropic, etc.)|
                                                          +--------------------------+
```

### How it Works
1. Deploy the portal using `k8s/manifests.yaml` (or the manifests in `gemini-portal/`, `anthropic-portal/`, or `github-portal/`).
2. The portal container runs with environment variables `TARGET_URL` (e.g. `https://generativelanguage.googleapis.com`) and `UPSTREAM_AUTH_TOKEN` (sourced from a Kubernetes Secret).
3. Workload pods address the portal directly over internal cluster DNS:
   ```bash
   curl http://gemini-portal/v1beta/models
   ```
4. The portal receives the request, attaches the configured authentication header, and forwards the request over HTTPS to the external service.

### Best Used For
- Trusted or internal microservices that need centralized API key management without requiring `NET_ADMIN` capabilities or sidecar containers.
- Shared API key consumption across multiple replicas or jobs in a namespace.

---

## 3. Explicit HTTP Proxy Mode (`HTTP_PROXY` / `HTTPS_PROXY`)

In explicit HTTP proxy mode, workloads configure standard proxy environment variables to route their HTTP and HTTPS traffic through the portal.

```
+-------------------------------------------------------+
| Workload Container                                    |
|                                                       |
| export HTTP_PROXY=http://portal:8080                  |
| export HTTPS_PROXY=http://portal:8080                 |
| export SSL_CERT_FILE=/etc/service-portal/ca/tls.crt   |
|                                                       |
| curl https://api.github.com/...                       |
+---------------------------+---------------------------+
                            | HTTP CONNECT api.github.com:443
                            v
+-------------------------------------------------------+
| Portal Proxy (cmd/service-portal or all-in-one-portal)|
|                                                       |
| 1. Accepts CONNECT tunnel                             |
| 2. Mints TLS cert for api.github.com using local CA   |
| 3. Terminates TLS tunnel and inspects request         |
| 4. Injects secret auth header                         |
| 5. Forwards to upstream https://api.github.com        |
+-------------------------------------------------------+
```

### How it Works
1. Deploy the portal either as a sidecar or as an in-cluster service with `CA_CERT_PATH` and `CA_KEY_PATH` configured.
2. In the workload container, set:
   ```bash
   export HTTP_PROXY=http://127.0.0.1:8080
   export HTTPS_PROXY=http://127.0.0.1:8080
   export SSL_CERT_FILE=/etc/service-portal/ca/tls.crt # or trust the CA
   ```
3. For HTTPS connections, standard HTTP client libraries issue an `HTTP CONNECT <host>:443` request to the proxy.
4. `pkg/proxy/proxy.go` intercepts the `CONNECT` request, hijacks the connection, completes the TLS handshake using a dynamically minted certificate signed by the CA, and executes credential injection before proxying to the target host.

### Best Used For
- Workloads using standard CLI utilities (`git`, `curl`, `pip`, `npm`) that natively respect `HTTP_PROXY` and `HTTPS_PROXY` in environments where `CAP_NET_ADMIN` (required for transparent iptables interception) cannot be granted.
