# Plan: isentry Label, LDAP Checker, Test Environment Update

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-02-27
- **Last updated**: 2026-02-27
- **Status**: Pending

---

## Version History

- **v1.0.0** (2026-02-27): Initial plan version

---

## Current Status

- **Active phase**: Phase 2
- **Active item**: 2.1
- **Last updated**: 2026-02-27
- **Note**: Phase 1 complete

---

## Table of Contents

- [x] [Phase 1: Backend — isentry label support](#phase-1-backend--isentry-label-support)
- [ ] [Phase 2: Frontend — IsRoot → IsEntry rename](#phase-2-frontend--isroot--isentry-rename)
- [ ] [Phase 3: LDAP server in test infrastructure](#phase-3-ldap-server-in-test-infrastructure)
- [ ] [Phase 4: Test environment — configuration and restart](#phase-4-test-environment--configuration-and-restart)
- [ ] [Phase 5: Documentation](#phase-5-documentation)

---

## Phase 1: Backend — isentry label support

**Dependencies**: None
**Status**: Complete

### Description

Replace automatic entry point detection (nodes without incoming edges → IsRoot=true) with
explicit `isentry` label from Prometheus metrics. Rename `IsRoot` to `IsEntry` across the
Go codebase. This is the core change — all other phases depend on it.

### Items

- [x] **1.1 Add IsEntry field to TopologyEdge, remove IsRoot from Node**
  - **Dependencies**: None
  - **Description**:
    1. In `internal/topology/models.go`:
       - Add `IsEntry bool` field to `TopologyEdge` struct (line 111)
       - Rename `IsRoot bool` → `IsEntry bool` in `Node` struct (line 17), change JSON tag `isRoot` → `isEntry`
  - **Modifies**:
    - `internal/topology/models.go` (TopologyEdge struct lines 102-111, Node struct lines 6-21)

- [x] **1.2 Update PromQL queries and parsing**
  - **Dependencies**: 1.1
  - **Description**:
    1. In `internal/topology/prometheus.go`:
       - Add `isentry` to `group by` clause in `queryTopologyEdges` (line 82):
         `group by (name, namespace, group, dependency, type, host, port, critical, isentry) (app_dependency_health%s)`
       - Add `isentry` to `group by` clause in `queryTopologyEdgesLookback` (line 88):
         `group by (name, namespace, group, dependency, type, host, port, critical, isentry) (last_over_time(app_dependency_health%s[%s]))`
       - Add `IsEntry: r.Metric["isentry"] == "yes"` to both `QueryTopologyEdges` (line 347) and `QueryTopologyEdgesLookback` (line 371) result parsing
  - **Modifies**:
    - `internal/topology/prometheus.go` (lines 82, 88, 328-349, 351-373)

- [x] **1.3 Update graph builder — replace auto-detection with label**
  - **Dependencies**: 1.1, 1.2
  - **Description**:
    1. In `internal/topology/graph.go`:
       - Remove the auto-detection block (lines 393-402) that marks nodes without incoming edges as root
       - Instead, when building nodes from edges, propagate `IsEntry` flag: if any `TopologyEdge` with `Name == nodeID` has `IsEntry==true`, set `Node.IsEntry = true`
       - The flag is per-service (not per-edge), so check it during node registration (lines 207-237): when registering the source node, if `e.IsEntry` is true, set the node's entry flag
  - **Modifies**:
    - `internal/topology/graph.go` (lines 162-405, specifically 207-237 and 393-402)

- [x] **1.4 Update tests**
  - **Dependencies**: 1.1, 1.2, 1.3
  - **Description**:
    1. In `internal/topology/graph_test.go`:
       - Update IsRoot → IsEntry in all assertions (lines 1684-1710)
       - Modify test data: instead of relying on auto-detection, add `IsEntry: true` to specific test TopologyEdge entries and verify only those nodes get `IsEntry=true`
       - Add test case: when no edges have `IsEntry=true`, no nodes should have `IsEntry=true`
    2. Run `go test ./internal/topology/...` and fix any compilation/test failures
  - **Modifies**:
    - `internal/topology/graph_test.go`
    - Potentially `internal/topology/prometheus_test.go` if PromQL tests exist

### Completion Criteria Phase 1

- [x] All items completed (1.1, 1.2, 1.3, 1.4)
- [x] `go build ./...` succeeds
- [x] `go test ./internal/topology/...` passes
- [x] No references to `IsRoot` remain in Go code

---

## Phase 2: Frontend — IsRoot → IsEntry rename

**Dependencies**: Phase 1
**Status**: Pending

### Description

Update the frontend to use the new `isEntry` JSON field instead of `isRoot`. All visual behavior
(blue badge with ⬇, sidebar "Entry Point" label, legend) stays the same — only the data field name changes.

### Items

- [ ] **2.1 Update graph.js**
  - **Dependencies**: None
  - **Description**:
    1. In `frontend/src/graph.js`:
       - Change CSS selector `'[?isRoot]'` → `'[?isEntry]'` (line 387)
       - Change node data assignment `isRoot: node.isRoot || false` → `isEntry: node.isEntry || false` (line 596)
       - Update any comments referencing "root" to "entry point"
  - **Modifies**:
    - `frontend/src/graph.js`

- [ ] **2.2 Update sidebar.js**
  - **Dependencies**: None
  - **Description**:
    1. In `frontend/src/sidebar.js`:
       - Change `data.isRoot &&` → `data.isEntry &&` (line 373)
  - **Modifies**:
    - `frontend/src/sidebar.js`

- [ ] **2.3 Update CSS class names**
  - **Dependencies**: None
  - **Description**:
    1. In `frontend/src/style.css`:
       - Rename `.root-badge` → `.entry-badge` (CSS class)
       - Rename `.sidebar-root-badge` → `.sidebar-entry-badge`
    2. Update all references to `.root-badge` and `.sidebar-root-badge` in JS files accordingly
  - **Modifies**:
    - `frontend/src/style.css`
    - `frontend/src/graph.js` (class name reference)
    - `frontend/src/sidebar.js` (class name reference)

### Completion Criteria Phase 2

- [ ] All items completed (2.1, 2.2, 2.3)
- [ ] No references to `isRoot` remain in frontend code
- [ ] `make lint` passes (if frontend linting is configured)

---

## Phase 3: LDAP server in test infrastructure

**Dependencies**: None (independent of Phases 1-2)
**Status**: Pending

### Description

Add 389 Directory Server (389ds) to the test infrastructure Helm chart. Use the deployment
pattern from [BigKAA/artds](https://github.com/BigKAA/artds) repository. This is a simplified
single-replica deployment for testing purposes (no replication, no init job).

### Items

- [ ] **3.1 Add 389ds template to dephealth-infra**
  - **Dependencies**: None
  - **Description**:
    1. Create `deploy/helm/dephealth-infra/templates/389ds.yml` following the pattern from `redis.yml`:
       - Deployment (single replica) + Service (ClusterIP)
       - Image: `389ds/dirsrv:3.1`
       - Namespace: `dephealth-389ds` (via `.Values.ds389.namespace`)
       - Environment variables: `DS_SUFFIX_NAME=dc=test,dc=local`, `DS_DM_PASSWORD` from values
       - Ports: 3389 (LDAP), 3636 (LDAPS)
       - Probes: TCP socket on port 3389
       - PVC for `/data` (persistent storage)
       - Conditional on `.Values.ds389.enabled`
       - Labels: `app.kubernetes.io/part-of: dephealth`
    2. Add namespace to `templates/namespace.yml` if needed
  - **Creates**:
    - `deploy/helm/dephealth-infra/templates/389ds.yml`
  - **Links**:
    - [artds kubernetes manifests](https://github.com/BigKAA/artds)

- [ ] **3.2 Add 389ds values**
  - **Dependencies**: 3.1
  - **Description**:
    1. In `deploy/helm/dephealth-infra/values.yaml`:
       - Add `ds389` section: `enabled: false`, image, tag, namespace, resources, dmPassword, suffix
    2. In `deploy/helm/dephealth-infra/values-homelab.yaml`:
       - Enable 389ds: `ds389.enabled: true`
       - Override image registry to Harbor if needed
       - Set storage class and size
  - **Modifies**:
    - `deploy/helm/dephealth-infra/values.yaml`
    - `deploy/helm/dephealth-infra/values-homelab.yaml`

### Completion Criteria Phase 3

- [ ] All items completed (3.1, 3.2)
- [ ] `helm template dephealth-infra deploy/helm/dephealth-infra/ -f deploy/helm/dephealth-infra/values-homelab.yaml` renders without errors
- [ ] 389ds template follows the same pattern as redis.yml

---

## Phase 4: Test environment — configuration and restart

**Dependencies**: Phase 1, Phase 2, Phase 3
**Status**: Pending

### Description

Update test environment configuration: add `isentry` label to uniproxy-01, add LDAP connection
to uniproxy-03, build and deploy new version, restart test environment with clean data.

### Items

- [ ] **4.1 Add isentry to uniproxy-01**
  - **Dependencies**: None
  - **Description**:
    1. In `deploy/helm/dephealth-uniproxy/instances/ns1-homelab.yaml`:
       - Add `isentry: "yes"` to uniproxy-01 instance configuration
    2. Verify the uniproxy Helm chart deployment template passes `DEPHEALTH_ISENTRY` environment variable from the `isentry` value
       - If not, update `deploy/helm/dephealth-uniproxy/templates/deployment.yml` to include the env var
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/instances/ns1-homelab.yaml`
    - Potentially `deploy/helm/dephealth-uniproxy/templates/deployment.yml`

- [ ] **4.2 Add LDAP connection to uniproxy-03**
  - **Dependencies**: None
  - **Description**:
    1. In `deploy/helm/dephealth-uniproxy/instances/ns1-homelab.yaml`:
       - Add LDAP connection to uniproxy-03:
         ```yaml
         - name: ldap
           type: ldap
           host: "389ds.dephealth-389ds.svc"
           port: "3389"
           critical: "no"
         ```
    2. Verify the uniproxy deployment template handles `type: ldap` connections properly
       (LDAP uses host+port, not URL, with `DEPHEALTH_<NAME>_HOST` and `DEPHEALTH_<NAME>_PORT`)
  - **Modifies**:
    - `deploy/helm/dephealth-uniproxy/instances/ns1-homelab.yaml`

- [ ] **4.3 Build and push new dephealth-ui image**
  - **Dependencies**: Phase 1, Phase 2
  - **Description**:
    1. Build dev image: `make docker-dev TAG=<next-dev-tag>`
    2. Push to Harbor dev registry
    3. Update dephealth-ui Helm chart values with new image tag
  - **Modifies**:
    - `deploy/helm/dephealth-ui/values-homelab.yaml` (image tag)

- [ ] **4.4 Restart test environment**
  - **Dependencies**: 4.1, 4.2, 4.3
  - **Description**:
    1. `make env-undeploy`
    2. Delete VictoriaMetrics PVC: `kubectl delete pvc -n dephealth-monitoring -l app=victoriametrics` (or identify the exact PVC name)
    3. `make env-deploy`
    4. `make env-status` — verify all pods are Running
    5. Wait for metrics to accumulate (~2-3 minutes)
    6. Verify in dephealth-ui:
       - uniproxy-01 shows entry point badge
       - uniproxy-03 shows LDAP dependency node
       - No other nodes show entry point badge
  - **Modifies**: (runtime only — Kubernetes resources)

### Completion Criteria Phase 4

- [ ] All items completed (4.1, 4.2, 4.3, 4.4)
- [ ] All pods Running (`make env-status`)
- [ ] uniproxy-01 marked as entry point in UI
- [ ] LDAP dependency visible from uniproxy-03
- [ ] VictoriaMetrics database is clean (no stale data)

---

## Phase 5: Documentation

**Dependencies**: Phase 1, Phase 2
**Status**: Pending

### Description

Update project documentation to reflect the isentry change and describe how to use the label.

### Items

- [ ] **5.1 Update API.md**
  - **Dependencies**: None
  - **Description**:
    1. In `docs/API.md`:
       - Rename `isRoot` → `isEntry` in the Node field table
       - Update description: "Indicates the node is an entry point for traffic (set via `isentry` label in dephealth SDK metrics)"
       - Update example JSON response
  - **Modifies**:
    - `docs/API.md`

- [ ] **5.2 Update application-design.md**
  - **Dependencies**: None
  - **Description**:
    1. In `docs/application-design.md`:
       - Add a section about entry points explaining:
         - What entry points are (first nodes receiving external traffic)
         - How to mark them (set `isentry=yes` label in dephealth SDK / set `DEPHEALTH_ISENTRY=yes` in uniproxy)
         - How they display in UI (blue badge ⬇, sidebar indicator)
         - That entry points are no longer auto-detected
  - **Modifies**:
    - `docs/application-design.md`

- [ ] **5.3 Update CHANGELOG.md**
  - **Dependencies**: 5.1, 5.2
  - **Description**:
    1. Add new section to `CHANGELOG.md` with:
       - **Changed**: Entry points now use explicit `isentry` label from dephealth SDK instead of auto-detection
       - **Changed**: API field renamed `isRoot` → `isEntry` (breaking change)
       - **Added**: LDAP dependency type support in test environment
       - **Removed**: Automatic entry point detection algorithm
  - **Modifies**:
    - `CHANGELOG.md`

### Completion Criteria Phase 5

- [ ] All items completed (5.1, 5.2, 5.3)
- [ ] Documentation accurately reflects the new behavior
- [ ] No references to `isRoot` in documentation (except changelog noting the rename)

---

## Notes

- **Breaking API change**: `isRoot` → `isEntry` rename is a breaking change. Since the project is pre-1.0, this is acceptable without a major version bump.
- **Phase 3 is independent**: LDAP infrastructure can be developed in parallel with Phases 1-2.
- **Uniproxy chart may need updates**: Phase 4.1 depends on the uniproxy Helm chart supporting `isentry` env var passthrough. If it doesn't, the deployment template needs updating.
- **389ds image**: Using `389ds/dirsrv:3.1` — verify the image is accessible from the homelab cluster (may need to pull through Harbor proxy).
