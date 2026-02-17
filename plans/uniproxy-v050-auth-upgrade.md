# Plan: Upgrade Test Environment to uniproxy v0.5.0 with Auth Scenarios

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-02-17
- **Last updated**: 2026-02-17
- **Status**: Pending

---

## Version History

- **v1.0.0** (2026-02-17): Initial plan

---

## Current Status

- **Active phase**: Phase 1
- **Active item**: 1.1
- **Last updated**: 2026-02-17
- **Note**: Plan created, awaiting approval

---

## Overview

Upgrade the `dephealth-uniproxy` Helm chart from v0.4.1 to v0.5.0 with authentication
scenarios to exercise new SDK v0.4.2 capabilities (`auth_error` status, client/server auth).

**Image**: `harbor.kryukov.lan/library/uniproxy:v0.5.0` (already in Harbor)

### Target Topology with Auth

```
NS1 (dephealth-uniproxy):
  uniproxy-01 ──→ uniproxy-02 (HTTP, critical)
              ──→ uniproxy-03 (HTTP, critical)

  uniproxy-02 ──→ redis (Redis, not critical)
              ──→ grpc-stub (gRPC, not critical)
              ──→ uniproxy-04 (HTTP, critical)

  uniproxy-03 ──→ postgresql (Postgres, critical)

NS2 (dephealth-uniproxy-2):
  uniproxy-04 ──→ uniproxy-05 (HTTP, critical, CORRECT Bearer)  ✅
              ──→ uniproxy-06 (HTTP, critical)

  uniproxy-05    (leaf node)           SERVER AUTH: Bearer token
                                       /metrics: open (AUTH_METRICS_METHOD=none)

  uniproxy-06 ──→ uniproxy-07 (HTTP, critical)
              ──→ uniproxy-08 (HTTP, not critical, CORRECT Basic)  ✅
              ──→ uniproxy-05 (HTTP, not critical, WRONG Bearer)   ❌ NEW

  uniproxy-07 ──→ postgresql (Postgres, critical)

  uniproxy-08 ──→ postgresql (Postgres, critical)
                                       SERVER AUTH: Basic auth
                                       /metrics: open (AUTH_METRICS_METHOD=none)
```

### Auth Scenarios Summary

| # | Client | Server | Auth Type | Credentials | Expected Status |
|---|--------|--------|-----------|-------------|-----------------|
| 1 | uniproxy-04 | uniproxy-05 | Bearer | Correct token | `ok` |
| 2 | uniproxy-06 | uniproxy-05 | Bearer | **Wrong** token | `auth_error` |
| 3 | uniproxy-06 | uniproxy-08 | Basic | Correct user/pass | `ok` |
| 4 | VictoriaMetrics | uniproxy-05 /metrics | none | No auth needed | scrape ok |
| 5 | VictoriaMetrics | uniproxy-08 /metrics | none | No auth needed | scrape ok |

### Design Decisions

1. **Server auth on `/`, not `/metrics`** — `/metrics` stays open via `AUTH_METRICS_METHOD=none`
   to avoid breaking VictoriaMetrics scraping (no auth support in current scrape config).
2. **uniproxy-05 as Bearer-protected server** — currently a leaf node with no deps, minimal blast radius.
3. **uniproxy-08 as Basic-protected server** — already exists, adds diversity (two auth methods).
4. **Wrong-token edge is non-critical** — prevents cascade propagation from intentional auth_error.
5. **Credentials in values files** — test environment only, no real secrets involved.

---

## Table of Contents

- [ ] [Phase 1: Helm Chart Template Updates](#phase-1-helm-chart-template-updates)
- [ ] [Phase 2: Instance Configuration Updates](#phase-2-instance-configuration-updates)
- [ ] [Phase 3: Deploy and Verify](#phase-3-deploy-and-verify)

---

## Phase 1: Helm Chart Template Updates

**Dependencies**: None
**Status**: Pending

### Description

Update the Helm chart metadata and deployment template to support uniproxy v0.5.0
auth features: per-instance server auth and per-connection client auth env vars.

### Items

- [ ] **1.1 Bump chart version and appVersion**
  - **Dependencies**: None
  - **Description**: Update `Chart.yaml` version to `0.5.0` and appVersion to `"0.5.0"`.
    Update `values-homelab.yaml` image tag to `v0.5.0`.
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/Chart.yaml`
    - `deploy/helm/dephealth-uniproxy/values-homelab.yaml`

- [ ] **1.2 Add server auth env vars to deployment template**
  - **Dependencies**: None
  - **Description**: Add optional `serverAuth` block support per instance.
    When `serverAuth` is defined, emit these env vars:
    ```
    AUTH_METHOD          ← .serverAuth.method
    AUTH_TOKEN           ← .serverAuth.token       (if bearer)
    AUTH_USER            ← .serverAuth.username     (if basic)
    AUTH_PASS            ← .serverAuth.password     (if basic)
    AUTH_API_KEY         ← .serverAuth.apiKey       (if apikey)
    AUTH_METRICS_METHOD  ← .serverAuth.metricsMethod (override for /metrics)
    AUTH_STATUS_METHOD   ← .serverAuth.statusMethod  (override for /)
    ```
    Instance values schema:
    ```yaml
    instances:
      - name: uniproxy-05
        serverAuth:
          method: bearer
          token: "secret-token"
          metricsMethod: "none"    # keep /metrics open
    ```
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/templates/deployment.yml`

- [ ] **1.3 Add client auth env vars to deployment template**
  - **Dependencies**: None
  - **Description**: Add optional `auth` block per connection.
    When `auth` is defined, emit the relevant env vars:
    ```
    DEPHEALTH_<NAME>_BEARER_TOKEN  ← .auth.bearerToken
    DEPHEALTH_<NAME>_BASIC_USER    ← .auth.basicUser
    DEPHEALTH_<NAME>_BASIC_PASS    ← .auth.basicPass
    DEPHEALTH_<NAME>_HEADERS       ← .auth.headers (JSON string)
    DEPHEALTH_<NAME>_METADATA      ← .auth.metadata (JSON string)
    ```
    Connection values schema:
    ```yaml
    connections:
      - name: uniproxy-05
        type: http
        url: "http://..."
        auth:
          bearerToken: "secret-token"
      - name: uniproxy-08
        type: http
        url: "http://..."
        auth:
          basicUser: "monitor"
          basicPass: "monitor-pass"
    ```
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/templates/deployment.yml`

### Completion Criteria Phase 1

- [ ] All items completed (1.1, 1.2, 1.3)
- [ ] `helm template` renders correctly with auth env vars
- [ ] Existing instances without auth render identically to before (backward compatible)

---

## Phase 2: Instance Configuration Updates

**Dependencies**: Phase 1
**Status**: Pending

### Description

Configure auth scenarios in the instance YAML files for NS2 (dephealth-uniproxy-2).

### Items

- [ ] **2.1 Add server auth to uniproxy-05 (Bearer)**
  - **Dependencies**: None
  - **Description**: Add `serverAuth` block to uniproxy-05 instance:
    ```yaml
    - name: uniproxy-05
      replicas: 1
      serverAuth:
        method: bearer
        token: "test-auth-token-05"
        metricsMethod: "none"
      connections: []
    ```
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml`

- [ ] **2.2 Add server auth to uniproxy-08 (Basic)**
  - **Dependencies**: None
  - **Description**: Add `serverAuth` block to uniproxy-08 instance:
    ```yaml
    - name: uniproxy-08
      replicas: 1
      serverAuth:
        method: basic
        username: "monitor"
        password: "monitor-pass-08"
        metricsMethod: "none"
      connections:
        - name: postgresql
          ...  # existing connection unchanged
    ```
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml`

- [ ] **2.3 Add correct Bearer auth: uniproxy-04 → uniproxy-05**
  - **Dependencies**: None
  - **Description**: Add `auth.bearerToken` to existing connection from uniproxy-04
    to uniproxy-05 with the **correct** token `"test-auth-token-05"`:
    ```yaml
    connections:
      - name: uniproxy-05
        type: http
        url: "http://uniproxy-05.dephealth-uniproxy-2.svc:8080"
        critical: "yes"
        healthPath: "/"
        auth:
          bearerToken: "test-auth-token-05"
    ```
    Expected result: `app_dependency_status{status="ok"}` on this edge.
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml`

- [ ] **2.4 Add wrong Bearer auth: uniproxy-06 → uniproxy-05 (NEW connection)**
  - **Dependencies**: None
  - **Description**: Add a **new** connection from uniproxy-06 to uniproxy-05 with
    an **intentionally wrong** Bearer token. Mark as **non-critical** to prevent cascade:
    ```yaml
    - name: uniproxy-06
      connections:
        ... # existing connections
        - name: uniproxy-05
          type: http
          url: "http://uniproxy-05.dephealth-uniproxy-2.svc:8080"
          critical: "no"
          healthPath: "/"
          auth:
            bearerToken: "wrong-token-intentional"
    ```
    Expected result: `app_dependency_status{status="auth_error"}` on this edge.
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml`

- [ ] **2.5 Add correct Basic auth: uniproxy-06 → uniproxy-08**
  - **Dependencies**: None
  - **Description**: Add `auth.basicUser`/`auth.basicPass` to **existing** connection
    from uniproxy-06 to uniproxy-08 with the **correct** credentials:
    ```yaml
    - name: uniproxy-08
      type: http
      url: "http://uniproxy-08.dephealth-uniproxy-2.svc:8080"
      critical: "no"
      healthPath: "/"
      auth:
        basicUser: "monitor"
        basicPass: "monitor-pass-08"
    ```
    Expected result: `app_dependency_status{status="ok"}` on this edge.
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml`

### Completion Criteria Phase 2

- [ ] All items completed (2.1–2.5)
- [ ] `helm template` for NS2 shows correct env vars for all auth scenarios
- [ ] NS1 instances remain unchanged

---

## Phase 3: Deploy and Verify

**Dependencies**: Phase 2
**Status**: Pending

### Description

Deploy the updated Helm chart and verify all auth scenarios work correctly.

### Items

- [ ] **3.1 Deploy NS2 (dephealth-uniproxy-2)**
  - **Dependencies**: None
  - **Description**: Upgrade the NS2 Helm release:
    ```bash
    helm upgrade dephealth-uniproxy-ns2 deploy/helm/dephealth-uniproxy/ \
      -f deploy/helm/dephealth-uniproxy/values-homelab.yaml \
      -f deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml \
      -n dephealth-uniproxy-2
    ```
    Wait for all pods to reach Running state.

- [ ] **3.2 Deploy NS1 (dephealth-uniproxy)**
  - **Dependencies**: None
  - **Description**: Upgrade the NS1 Helm release (image bump only, no auth changes):
    ```bash
    helm upgrade dephealth-uniproxy-ns1 deploy/helm/dephealth-uniproxy/ \
      -f deploy/helm/dephealth-uniproxy/values-homelab.yaml \
      -f deploy/helm/dephealth-uniproxy/instances/ns1-homelab.yaml \
      -n dephealth-uniproxy
    ```

- [ ] **3.3 Verify metrics scraping**
  - **Dependencies**: 3.1, 3.2
  - **Description**: Confirm VictoriaMetrics still collects metrics from all instances,
    including auth-protected uniproxy-05 and uniproxy-08 (via open `/metrics`):
    ```bash
    curl "http://victoriametrics.dephealth-monitoring.svc:8428/api/v1/query?query=up{app_kubernetes_io_name=~'uniproxy-0.'}"
    ```

- [ ] **3.4 Verify auth_error scenario**
  - **Dependencies**: 3.1
  - **Description**: Check that uniproxy-06 → uniproxy-05 shows `auth_error`:
    ```bash
    curl "http://victoriametrics.dephealth-monitoring.svc:8428/api/v1/query?query=app_dependency_status{name='uniproxy-06',dependency='uniproxy-05',status='auth_error'}"
    ```
    Expected: value `1`.

- [ ] **3.5 Verify correct auth scenarios**
  - **Dependencies**: 3.1
  - **Description**: Check that authenticated connections show `ok`:
    ```bash
    # Bearer auth: uniproxy-04 → uniproxy-05
    curl "...query=app_dependency_status{name='uniproxy-04',dependency='uniproxy-05',status='ok'}"
    # Basic auth: uniproxy-06 → uniproxy-08
    curl "...query=app_dependency_status{name='uniproxy-06',dependency='uniproxy-08',status='ok'}"
    ```

- [ ] **3.6 Verify in dephealth-ui**
  - **Dependencies**: 3.3, 3.4, 3.5
  - **Description**: Open `https://dephealth.kryukov.lan` and verify:
    1. Edge uniproxy-06 → uniproxy-05 shows `auth_error` status (yellow AUTH badge)
    2. Edge uniproxy-04 → uniproxy-05 shows `ok` status
    3. Edge uniproxy-06 → uniproxy-08 shows `ok` status
    4. Status filter `auth_error` finds the affected edge
    5. Sidebar shows correct status/detail for auth edges

### Completion Criteria Phase 3

- [ ] All pods Running in both namespaces
- [ ] VictoriaMetrics scraping all instances (including auth-protected ones)
- [ ] `auth_error` visible on uniproxy-06 → uniproxy-05 edge in metrics
- [ ] Correct auth connections show `ok` in metrics
- [ ] dephealth-ui displays auth_error edges correctly
- [ ] No regressions in existing topology visualization

---

## Notes

- **Server auth credentials are test-only** — plaintext in values files is acceptable
  for the test environment. Production would use K8s Secrets.
- **`/metrics` stays open** — VictoriaMetrics scrape config has no auth support.
  If server auth on `/metrics` is needed later, update the VictoriaMetrics scrape config
  in `deploy/helm/dephealth-monitoring/templates/victoriametrics.yml`.
- **Non-critical wrong-auth edge** — uniproxy-06 → uniproxy-05 with wrong token is
  intentionally `critical: "no"` to prevent cascade failure propagation from a deliberate
  test scenario.
