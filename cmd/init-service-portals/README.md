# Init Service Portals (`cmd/init-service-portals`)

`init-service-portals` is the setup utility packaged into the `init-service-portals` container image. It runs as an `initContainer` inside the pod to establish pod-local TLS trust and configure network packet redirection.

---

## Responsibilities

1. **Pod-Local Root CA Generation**: Generates an ephemeral 2048-bit RSA Root Certificate Authority (`tls.crt` and `tls.key`) with a 10-year validity period.
2. **Key Privilege Separation**:
   - Writes the public certificate (`tls.crt`) into the cert directory (`-cert-dir`, default `/etc/service-portal/ca`), which is mounted read-only to the workload container.
   - Writes the private key (`tls.key`) with `0600` permissions and certificate into the key directory (`-key-dir`, default `/etc/service-portal/ca-private`), and chowns both directory and key file to the proxy user (`-chown-uid` and `-chown-gid`, typically `1337`).
3. **iptables Interception & Egress Enforcement (`entrypoint.sh`)**:
   - Redirects outbound TCP port 80 to proxy port 8080 and port 443 to proxy port 8443.
   - Excludes proxy UID 1337 and loopback from redirection.
   - Drops all other outbound TCP and UDP packets from the pod (allowing only DNS on port 53).

---

## CLI Flags

```
init-service-portals [flags]
```

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-cert-dir` | `/etc/service-portal/ca` | Destination directory for public CA certificate (`tls.crt`). |
| `-key-dir` | `/etc/service-portal/ca-private` | Destination directory for CA certificate and private key (`tls.key`). |
| `-chown-uid` | `-1` | UID to chown private key and directory to (-1 leaves unchanged). |
| `-chown-gid` | `-1` | GID to chown private key and directory to (-1 leaves unchanged). |

---

## Environment Variables (`entrypoint.sh`)

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PROXY_PORT` | `8080` | Local port for HTTP traffic redirection. |
| `PROXY_HTTPS_PORT` | `8443` | Local port for HTTPS traffic redirection. |
| `PROXY_UID` | `1337` | UID of the proxy container, excluded from redirection and allowed outbound. |
| `INTERCEPT_PORTS` | `80` | Ports to redirect (e.g. `80,443`), or `*` for all ports. |
| `CA_DIR` | `/etc/service-portal/ca` | Passed to `-cert-dir`. |
| `CA_PRIVATE_DIR` | `/etc/service-portal/ca-private` | Passed to `-key-dir`. |
| `CHOWN_UID` | `-1` | Passed to `-chown-uid`. |
| `CHOWN_GID` | `-1` | Passed to `-chown-gid`. |
| `DNS_SERVER_IPS` | `/etc/resolv.conf` nameservers | Comma-separated list of DNS server IPs allowed for outbound port 53 traffic. If unset, automatically reads nameservers from `/etc/resolv.conf`. |
