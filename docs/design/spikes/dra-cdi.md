# Spike Report: Dynamic Resource Allocation (DRA) + Container Device Interface (CDI) Hooks as the Interception Mechanism

* **Issue Reference:** Sub-issue of [#159](https://github.com/gke-labs/service-portals/issues/159), Issue [#160](https://github.com/gke-labs/service-portals/issues/160)
* **Prototype Branch:** `spike/dra-cdi`
* **Status:** Prototype implemented and verified on kind; deny suite passing.

---

## 1. Executive Summary & Verdict

This spike investigates replacing the in-pod mutating webhook injector and `initContainers` with Kubernetes **Dynamic Resource Allocation (DRA)** and **Container Device Interface (CDI)** OCI runtime hooks.

In this model:
1. Workload egress interception is modeled as an abstract "device" (`portal-egress.portals.gke.io`).
2. Workload pods opt in by referencing a `ResourceClaimTemplate` in `spec.resourceClaims` and claiming it in `spec.containers[*].resources.claims`.
3. The node-local DRA driver (a DaemonSet) handles `NodePrepareResources` by generating a root CA certificate and writing a dynamic **CDI specification** to `/etc/cdi/portal-egress.json`.
4. When containerd creates the container, CDI automatically applies `containerEdits`:
   - Mounts the public CA certificate into `/etc/service-portal/ca/tls.crt`.
   - Injects `SSL_CERT_FILE=/etc/service-portal/ca/tls.crt` into the container environment.
   - Registers an OCI **`createRuntime` hook** (`/usr/local/bin/portal-cdi-hook`).
5. The `createRuntime` hook executes on the host before any workload process starts, joins the container's network namespace (`/proc/<pid>/ns/net`), and configures the iptables redirect and default-deny egress rules.

### Verdict: Is Non-Device DRA Workable or an Abuse?

* **Workable:** **Yes.** The end-to-end mechanism is technically sound, highly deterministic, and verified by our automated e2e test suite (`TestDRAEgressDenied`). CDI injection of mounts, environment variables, and OCI lifecycle hooks works cleanly in containerd without mutating the container's filesystem image or requiring init containers.
* **Is it an Abuse?** **Philosophically yes, practically feasible.** DRA was designed to allocate physical hardware accelerators (GPUs, TPUs, FPGAs, SR-IOV VFs). Using DRA to model network egress capability stretches the concept of a "device" to an abstraction. However, Kubernetes DRA Structured Parameters (KEP-4381) intentionally decoupled device claims from hardware vendors, allowing software-defined device drivers. The primary awkwardness is scheduling semantics: "portal-egress" is not a scarce hardware resource, so the driver must publish high device counts (or capacity) in its `ResourceSlice` to avoid blocking scheduling.
* **Key Strengths:**
  1. **Strictly Fail-Closed:** If the node DRA plugin is crashed or unregistered, kubelet blocks container creation with `FailedPrepareDynamicResources`. The container **never** starts unprotected.
  2. **100% Race-Free:** OCI `createRuntime` hooks run after the pod's network namespace is created but **before** the container entrypoint process is executed. There is zero window for egress leak.
  3. **Zero Pod Privileges:** The workload container requires zero capabilities (no `NET_ADMIN`, no `NET_RAW`), runs with `allowPrivilegeEscalation: false` and `NoNewPrivs: 1`, and requires no init container.

---

## 2. Kubernetes and Runtime Prerequisites

### 2.1 Kubernetes Version & Feature Gates

* **Kubernetes 1.30 (Alpha) / 1.31–1.32 (Beta):**
  - DRA was substantially overhauled in Kubernetes 1.30/1.31 with **Structured Parameters** (KEP-4381) under API group `resource.k8s.io/v1beta1`.
  - Feature Gate: `DynamicResourceAllocation=true` (enabled on API server, scheduler, and kubelet).
  - Runtime Config: `resource.k8s.io/v1beta1=true` on the API server.

### 2.2 kind Configuration

To test DRA and CDI on kind, the cluster must be provisioned with feature gates, API runtime configuration, and containerd CRI patches:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
featureGates:
  DynamicResourceAllocation: true
runtimeConfig:
  "resource.k8s.io/v1beta1": "true"
nodes:
- role: control-plane
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]
```

### 2.3 Containerd CDI & Hook Support Verification

In kind node images (`kindest/node:v1.32.2` with containerd v2.3.6 / runc 1.5.2):
* `crictl info` confirms containerd CDI support:
  ```json
  "config": {
    "cdiSpecDirs": ["/etc/cdi", "/var/run/cdi"],
    "enableCDI": true
  }
  ```
* Containerd natively translates CDI `containerEdits.hooks` into OCI runtime `config.json` hooks (`createRuntime`).
* When runc executes the `createRuntime` hook, it provides container state on `stdin` containing:
  ```json
  {
    "ociVersion": "1.2.0",
    "id": "<container-id>",
    "status": "creating",
    "pid": 2672,
    "bundle": "/run/containerd/io.containerd.runtime.v2.task/k8s.io/<container-id>",
    "annotations": {
      "io.kubernetes.cri.container-name": "workload",
      "io.kubernetes.cri.sandbox-name": "test-client-dra-deny",
      "io.kubernetes.cri.sandbox-namespace": "default",
      "io.kubernetes.cri.sandbox-uid": "8b5145b4-661b-46bc-a304-3a1d847593cd"
    }
  }
  ```

### 2.4 Google Kubernetes Engine (GKE) Support

* **GKE Standard:**
  - Dynamic Resource Allocation (DRA) is available as an Alpha/Beta feature on GKE Standard clusters running GKE 1.31+ with cluster feature flags or Alpha clusters (`--enable-kubernetes-alpha`).
  - Containerd on GKE nodes has CDI enabled by default for GPU and accelerator drivers.
  - A node-local DaemonSet running with hostPath and `privileged: true` can register as a kubelet plugin on GKE Standard.
* **GKE Autopilot:**
  - **Not currently supported on Autopilot.** GKE Autopilot prohibits `privileged: true`, `hostPath` volume mounts to `/var/lib/kubelet` or `/etc/cdi`, and `hostPID`/`hostNetwork` on workload or system daemons without Google-managed exceptions. Unless Google provides DRA drivers as a managed platform feature, third-party DRA DaemonSets cannot run on Autopilot.

---

## 3. Prototype Architecture & Implementation

### 3.1 Components Created

1. **`cmd/dra-portal-driver`:**
   - Registers with kubelet via `/var/lib/kubelet/plugins_registry/portal-egress.portals.gke.io-reg.sock`.
   - Implements gRPC services `v1beta1.DRAPlugin` and `v1alpha3.Node` on `/var/lib/kubelet/plugins/portal-egress.portals.gke.io/dra.sock`.
   - Generates the CA keypair in `/var/run/service-portal/ca`.
   - Copies `/usr/local/bin/portal-cdi-hook` to the node's `/usr/local/bin/portal-cdi-hook`.
   - Writes the CDI specification `/etc/cdi/portal-egress.json`.
   - On `NodePrepareResources`, resolves claims and returns CDI device `portals.gke.io/portal-egress=portal-egress`.
2. **`cmd/portal-cdi-hook`:**
   - Standalone OCI `createRuntime` hook binary executed by runc.
   - Reads OCI JSON state from `stdin` to obtain the container's host `pid`.
   - Automatically discovers the Kubernetes cluster DNS nameserver IP (reading `/var/lib/kubelet/pods/*/resolv.conf` or defaulting to `10.96.0.10`).
   - Uses `nsenter -t <pid> -n` to configure `iptables-restore` and `ip6tables-restore` inside the container network namespace.
3. **`k8s/dra-portal-driver.yaml`:**
   - Declares `DeviceClass` `portal-egress.portals.gke.io`.
   - Declares `ResourceClaimTemplate` `portal-egress-template`.
   - Declares the `dra-portal-driver` DaemonSet.
4. **`tests/e2e/e2e_test.go` (`TestDRAEgressDenied`):**
   - Automated end-to-end test verifying all positive and negative deny suite semantics on kind.

### 3.2 The Pod Opt-In Contract

A workload pod opts in with a resource claim in its spec:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-client-dra-deny
spec:
  resourceClaims:
  - name: portal-egress
    resourceClaimTemplateName: portal-egress-template
  containers:
  - name: workload
    image: toolbox:e2e
    securityContext:
      runAsUser: 1000
      allowPrivilegeEscalation: false
    resources:
      claims:
      - name: portal-egress
```

**Key Observation:** The `workload` container spec has **zero** portal-specific configuration:
- No `initContainers` in the pod.
- No `NET_ADMIN` or privileged capabilities.
- No volume mount for CA certificate.
- No environment variables for proxy or certificates.
Everything is injected dynamically by CDI when the claim is fulfilled.

### 3.3 Exploring the "No-Pod-Change" Path: Namespace Defaulting

Can a `ResourceClaim` be defaulted per namespace without changing pod specs?

* **Native Kubernetes Core:** Kubernetes has **no built-in defaulting controller** for `spec.resourceClaims`. Unlike PVCs, which cannot be defaulted onto pods without a pod spec change, DRA claims must be explicitly listed in `spec.resourceClaims` and referenced by containers.
* **CEL MutatingAdmissionPolicy (Alpha in 1.32):**
  - MutatingAdmissionPolicy (`mutating.admissionregistration.k8s.io/v1alpha1`) allows in-tree CEL mutations.
  - While CEL mutation can add `spec.resourceClaims` and `spec.containers[*].resources.claims` to incoming pods matching a namespace label, MutatingAdmissionPolicy is currently Alpha and restricted in expression power.
* **Mutating Admission Webhook:**
  - A traditional mutating webhook can inject `spec.resourceClaims` and `spec.containers[*].resources.claims` on pod creation.
  - *Irony:* If an admission webhook is required to default the claim onto the pod, we have reintroduced the admission webhook that we sought to avoid by using DRA. However, the webhook would only inject 6 lines of pure declarative metadata (`resourceClaims`), rather than injecting certificates, init containers, volumes, and network privileges.

---

## 4. Key Questions Answered

### Q1: Is the CDI Hook Race-Free vs Workload Start?
**Yes, 100% race-free.**
In the OCI runtime lifecycle:
1. `createRuntime` hooks run **after** the container namespaces (network, PID, cgroups) are created, but **before** `pivot_root` and before the container's entrypoint process is executed.
2. The hook executes `nsenter -t <pid> -n iptables-restore`, which installs the redirection and default-deny firewall rules atomically inside the container's network namespace.
3. Only after the hook exits with return code 0 does runc launch the container process.
4. If the hook fails or exits non-zero, runc terminates the container immediately; the workload process never runs. There is zero window where packets can bypass the rules.

### Q2: Is DRA Fail-Closed When the Driver is Down?
**Yes, strictly fail-closed.**
We experimentally tested this in `TestDRAEgressDenied/FailClosed_UnregisteredPlugin`:
1. When a pod references a claim for an unregistered or offline driver, the Kubernetes scheduler binds the pod to the node based on `ResourceSlice`.
2. When kubelet reconciles the pod, it attempts to call `NodePrepareResources` on the driver's gRPC socket.
3. Because the driver is offline or unregistered, kubelet logs:
   ```
   Warning FailedPrepareDynamicResources: failed to get gRPC client for driver <name>: plugin name not found in the list of registered DRA plugins
   ```
4. The pod remains in `Pending` / `ContainerCreating` status indefinitely.
5. Kubelet **refuses to invoke CRI `CreateContainer` or start the workload**. The container is never executed unprotected.

---

## 5. Evaluation Against Issue #159 Criteria

| Criterion | Evaluation / Observed Result |
| :--- | :--- |
| **Invisibility** | **High within the pod:** The workload container contains only `resources.claims: [{ name: portal-egress }]`. No init container, no volume mounts, no env vars. At the pod level, `spec.resourceClaims` is required unless defaulted by an admission webhook or CEL MutatingAdmissionPolicy. |
| **Privileges** | **Workload: Zero privileges.** Workload runs as unprivileged user (UID 1000) with `allowPrivilegeEscalation: false` (`NoNewPrivs: 1`). Workload has zero Linux capabilities (no `NET_ADMIN`, no `NET_RAW`).<br>**Node: High privileges.** The DRA driver DaemonSet runs as a privileged container with hostPath mounts (`/var/lib/kubelet`, `/etc/cdi`, `/usr/local/bin`). Autopilot does not currently permit third-party DRA plugins. |
| **Bypass Resistance** | **Complete:** Even if a process in the workload runs as root (UID 0), without `CAP_NET_ADMIN` it cannot modify iptables rules. UID 1337 bypass is blocked by `NoNewPrivs: 1`. IP literals, unlisted hostnames, non-intercepted ports, and arbitrary external DNS (e.g. 8.8.8.8) are rejected. |
| **Pod Identity** | In sidecar mode (used in this spike), pod identity is implicitly 1:1 local. In per-node DaemonSet mode, the driver and proxy would map pod IP or container ID to pod identity via CRI metadata available in the hook. |
| **CA Distribution** | **Dynamic CDI mount:** The DRA driver writes the CA certificate to `/var/run/service-portal/ca/tls.crt` on the node. The CDI spec mounts this into `/etc/service-portal/ca/tls.crt` and injects `SSL_CERT_FILE=/etc/service-portal/ca/tls.crt`. No Secret or ConfigMap projection is needed in the pod spec. |
| **Failure Mode** | **Fail-Closed:** Pod stays in `Pending` (`ContainerCreating`) if the driver is offline or `NodePrepareResources` fails. If the OCI hook fails, container creation aborts. |
| **Testability on kind** | **Full:** Tested and passed 100% on kind v0.27.0 with containerd v2.3.6 and Kubernetes 1.32.2. |
| **Operational Complexity** | **Moderate-High:** Requires cluster-level feature gates, containerd CDI enablement in `config.toml`, a node DaemonSet, and custom DRA API objects (`DeviceClass`, `ResourceSlice`). |

---

## 6. Credentials & Security Analysis

### 6.1 `opaque` Device Config Parameters
In Kubernetes DRA, a `ResourceClaim` can include `spec.devices.config`:
```yaml
config:
- opaque:
    driver: portal-egress.portals.gke.io
    parameters:
      policy: "gemini-portal-policy"
```
When `NodePrepareResources` is called, kubelet passes the opaque parameters to the node driver. The driver can read the named `PortalPolicy` or rules for that pod.

### 6.2 What Could a Pod Learn About Other Pods' Claims?
1. **`ResourceClaim` Scope:** `ResourceClaim` is a namespaced Kubernetes resource. A pod with standard default service account permissions cannot list or read `ResourceClaim` objects outside its own namespace.
2. **`ResourceSlice` Scope:** `ResourceSlice` is cluster-scoped. If RBAC allows a pod to read `ResourceSlice`, it can see allocated device names (`egress-0`) and node assignments, but `ResourceSlice` does **not** contain claim secrets or policy configurations.
3. **CDI Specs on Disk:** CDI files in `/etc/cdi` are owned by root on the host. Unprivileged workload containers cannot see or read `/etc/cdi` because they are isolated in their own mount namespace.
4. **Secret Isolation Mandate:** Plaintext credentials should **never** be passed inside opaque claim parameters. Claims should only pass references (e.g. Secret name in the pod's namespace). The driver must verify that the requesting pod belongs to the namespace that owns the referenced Secret.

---

## 7. What Was Faked or Prototyped

To ensure complete transparency regarding this spike:
1. **Proxy Placement:** The spike evaluated the interception mechanism using a sidecar proxy container (`all-in-one-portal:e2e`) injected alongside the workload, running as UID 1337. The workload container had **no** init container, **no** `NET_ADMIN`, and only the claim opt-in. A per-node DaemonSet proxy mode was designed and analyzed (see section 5), but the 1:1 sidecar proxy avoided needing multi-tenant secret routing in the proxy for this interception spike.
2. **ResourceSlice Provisioning:** The prototype driver in the e2e test uses a pre-provisioned `ResourceSlice` pool on the node (`egress-0`, `egress-1`, etc.). In a production driver, the driver controller would dynamically reconcile the `ResourceSlice` based on node capacity.

---

## 8. What Would Be Needed to Productionise

To bring DRA + CDI interception to production readiness:
1. **Dynamic ResourceSlice Controller:** Implement dynamic generation of `ResourceSlice` instances in `cmd/dra-portal-driver` using `client-go`, adjusting device count and capacity automatically.
2. **Claim Cleanup on Deletion:** Enhance `NodeUnprepareResources` to clean up ephemeral pod certificates and per-claim CDI spec files on container destruction.
3. **Containerd Configuration Automation:** Node provisioning scripts (cloud-init or GKE node initializers) must configure `/etc/containerd/config.toml` to enable `enable_cdi = true` and specify `cdi_spec_dirs`.
4. **GKE Autopilot Platform Feature Request:** Advocate with GKE engineering for managed DRA egress driver support, or pursue sibling mechanisms (NRI or eBPF) that can be packaged into managed platform runtimes.
