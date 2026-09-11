# Plan: Endpoint Grouping via `dep_namespace` / `dep_group` Labels

## Metadata

- **Plan version**: 1.2.0
- **Created**: 2026-09-11
- **Last updated**: 2026-09-11
- **Status**: Completed

---

## Version History

- **v1.1.0** (2026-09-11): Phases 1–3 and item 4.1 implemented
- **v1.0.0** (2026-09-11): Initial plan

---

## Current Status

- **Active phase**: none — plan complete
- **Active item**: none
- **Last updated**: 2026-09-11
- **Note**: All phases complete. Released as **v0.22.0** (tag + GitHub Release
  - multi-arch image in Yandex CR). Final lookback observation confirmed: the
  temporary conflicting label self-cleared when the stale series aged out of
  the 1h window (15:14:35, `meta.warnings` empty, postgresql back in `db`/`data`),
  and the untouched UI re-grouped postgresql into the `db` compound container
  via auto-refresh alone (no page reload) — closing the Phase 2 manual
  criterion. Homelab runs the feature (dev image v0.22.0-1; release image
  v0.22.0 identical code).

---

## Table of Contents

- [x] [Phase 1: Backend — Label Ingestion and Node Resolution](#phase-1-backend--label-ingestion-and-node-resolution)
- [x] [Phase 2: Frontend — Dynamic Re-grouping](#phase-2-frontend--dynamic-re-grouping)
- [x] [Phase 3: Documentation](#phase-3-documentation)
- [x] [Phase 4: Build, Deploy and Verification](#phase-4-build-deploy-and-verification)

---

## Background

Dependency ("endpoint") nodes in the topology graph have no group/namespace of
their own: the `namespace` and `group` metric labels describe the **reporting
service**, not the target. Current resolution is heuristic:

- `namespace`: FQDN parsing of `host` (`resolveDepNamespace`) → inheritance from
  a sole source service → empty (node rendered outside any group).
- `group`: inherited from the **first** edge encountered (non-deterministic for
  multi-source dependencies and misleading).

The agreed solution (Variant A) standardizes two reserved optional metric labels
that describe the **location of the dependency target**:

- `dep_namespace` — namespace (or virtual bucket, e.g. `external`) of the target.
- `dep_group` — logical group of the target.

Resolution precedence for dependency nodes (service-to-service targets are not
affected — a target that is itself a service keeps its own labels):

1. Explicit `dep_namespace`/`dep_group` — unanimous across all incoming edges
   (conflicting non-empty values are ignored, a warning is emitted).
2. FQDN extraction from `host` (namespace only, as today).
3. Inheritance from a sole source service (namespace **and** group — fixes the
   first-edge-wins behavior for `group`).
4. Otherwise ungrouped.

SDK-side, the labels ride on the existing custom-label mechanism
(`WithLabel`, env `DEPHEALTH_<DEP>_LABEL_<KEY>`), so Go services and uniproxy
can emit them **today** without SDK changes.

---

## Phase 1: Backend — Label Ingestion and Node Resolution

**Dependencies**: None
**Status**: Completed

### Description

Read `dep_namespace` / `dep_group` from Prometheus, carry them through
`TopologyEdge`, and apply the resolution precedence in `buildGraph`. Emit
conflict warnings via `TopologyMeta`.

### Items

- [x] **1.1 PromQL ingestion and model fields**
  - **Dependencies**: None
  - **Description**: Extend the topology edge queries to select the new labels
    and parse them into `TopologyEdge`.
  - **Creates/Modifies**:
    - `internal/topology/prometheus.go`
    - `internal/topology/models.go`
    - `internal/topology/prometheus_test.go`
  - **Details**:
    - `queryTopologyEdges` and `queryTopologyEdgesLookback`: add `dep_namespace`,
      `dep_group` to the `group by (...)` clause.
    - `TopologyEdge`: add `DepNamespace string` and `DepGroup string`.
    - Parse `r.Metric["dep_namespace"]` / `r.Metric["dep_group"]` in both
      `QueryTopologyEdges` and `QueryTopologyEdgesLookback`.
    - Tests: mock prom results with/without the labels (absent label → empty
      value, present → parsed).

- [x] **1.2 Node resolution precedence in `buildGraph`**
  - **Dependencies**: 1.1
  - **Description**: Apply the agreed precedence for dependency nodes and fix
    `group` inheritance.
  - **Creates/Modifies**:
    - `internal/topology/graph.go`
    - `internal/topology/graph_test.go`
  - **Details**:
    - During edge collection, gather per-dep-node sets of non-empty
      `dep_namespace` / `dep_group` values. Skip edges whose target is a known
      service (`serviceNames[e.Dependency]`) — explicit labels must not affect
      service nodes.
    - Remove first-edge group assignment at dep-node creation (`group: e.Group`).
    - Second pass per dependency node:
      - namespace: explicit unanimous → `resolveDepNamespace(host)` → sole-source
        inheritance → empty;
      - group: explicit unanimous → sole-source inheritance → empty.
    - Table-driven tests for the resolution matrix:
      - explicit single source (others empty) → explicit value wins;
      - explicit unanimous multi-source → explicit value;
      - explicit conflicting multi-source → fallback + warning;
      - explicit vs FQDN → explicit wins;
      - sole-source inheritance for namespace and group;
      - multi-source without explicit labels → ungrouped;
      - service target unaffected by dep labels;
      - lookback variant of the above.

- [x] **1.3 Conflict warnings in `TopologyMeta`**
  - **Dependencies**: 1.2
  - **Description**: Surface label conflicts so misconfiguration is visible.
  - **Creates/Modifies**:
    - `internal/topology/models.go`
    - `internal/topology/graph.go`
    - `internal/topology/graph_test.go`
  - **Details**:
    - Add `Warnings []string` (`json:"warnings,omitempty"`) to `TopologyMeta`.
    - `buildGraph` returns warnings (e.g.
      `dependency "postgres/pg:5432": conflicting dep_namespace values (db, infra), explicit labels ignored`);
      `Build` attaches them to the response. Do not mix with `Errors` (which
      marks partial data failures).

### ✅ Phase 1 Completion Criteria

- [x] All items completed (1.1, 1.2, 1.3)
- [x] `go test ./internal/topology/... -v -race` passes
- [x] No changes to node identity (`dependency/host:port` IDs unchanged)
- [x] Resolution matrix covered by tests

> Implementation note: "sole-source inheritance" follows the existing
> namespace-inheritance semantics — a value is inherited when all sources
> agree on it (a sole source is the special case). This preserves current
> namespace behavior exactly and applies it symmetrically to `group`.

---

## Phase 2: Frontend — Dynamic Re-grouping

**Dependencies**: Phase 1
**Status**: Completed (manual E2E check deferred to Phase 4.2)

### Description

Make the graph re-group when a node's namespace/group changes without a
structural change (label added/removed at runtime), and verify existing UI
surfaces display the resolved values.

### Items

- [x] **2.1 Include namespace/group in the render signature**
  - **Dependencies**: None
  - **Description**: `computeSignature` in `frontend/src/graph.js` currently
    hashes only node ids/types and edges, so a namespace/group-only change never
    triggers a rebuild during auto-refresh. Extend the node signature with
    `namespace` and `group`.
  - **Creates/Modifies**:
    - `frontend/src/graph.js`
  - **Details**: `${n.id}:${n.type}:${n.namespace || ''}:${n.group || ''}`;
    missing values serialize stably to empty strings.

- [x] **2.2 UI surface verification**
  - **Dependencies**: 2.1
  - **Description**: Verify that node tooltip and sidebar show the resolved
    namespace/group for dependency nodes and that compound grouping picks them
    up in both dimensions. Fix gaps if found (no changes expected —
    `buildCompoundElements` already consumes `node.namespace`/`node.group`).
  - **Creates/Modifies**:
    - `frontend/src/tooltip.js`, `frontend/src/sidebar.js` (only if gaps found)

### ✅ Phase 2 Completion Criteria

- [x] All items completed (2.1, 2.2)
- [x] Manual check: changing a label on a test uniproxy instance re-groups the
      node on the next auto-refresh (no page reload) — verified in the homelab:
      the UI tab was left untouched in namespace grouping; after the
      conflicting label reverted and the lookback window cleared, postgresql
      moved from the `dephealth-postgresql` (FQDN fallback) container into the
      `db` container via auto-refresh alone

> 2.2 result: no gaps found. `tooltip.js` and `sidebar.js` render
> `namespace`/`group` for dependency nodes; `buildCompoundElements`
> (grouping.js) consumes `node.namespace`/`node.group` in both dimensions.
> No code changes were needed.

---

## Phase 3: Documentation

**Dependencies**: Phase 1
**Status**: Completed

### Description

Document the new labels and the resolution rules for both operators and SDK
users. All docs in English (RU versions where they exist).

### Items

- [x] **3.1 Metrics specification**
  - **Dependencies**: None
  - **Description**: Add `dep_namespace` / `dep_group` to the label lists and
    describe semantics (target location, optional, unanimity rule, virtual
    values allowed) in:
    - `docs/METRICS.md`
    - `docs/METRICS.ru.md`
  - Include an example of setting the labels via uniproxy env vars and via SDK
    `WithLabel`.

- [x] **3.2 API and design docs**
  - **Dependencies**: None
  - **Description**: Document the resolution precedence and `meta.warnings` in:
    - `docs/API.md` (Topology response: `meta.warnings`)
    - `docs/application-design.md` (grouping resolution rules), RU version if
      present in `docs/`

- [x] **3.3 CHANGELOG**
  - **Dependencies**: None
  - **Description**: Add an Unreleased entry: new labels, resolution precedence,
    `meta.warnings`, fixed `group` first-edge inheritance (visible behavior
    change), frontend signature fix.

### ✅ Phase 3 Completion Criteria

- [x] All items completed (3.1, 3.2, 3.3)
- [ ] `make lint` (markdownlint) passes — blocked by a pre-existing repo-wide
      markdownlint failure on master (922 errors, tool version regression);
      the new/edited docs introduce zero new lint errors (CHANGELOG's MD024
      duplicates match the file's established Keep-a-Changelog pattern)

> 3.2 note: RU versions of API.md and application-design.md exist and were
> updated alongside the EN docs.

---

## Phase 4: Build, Deploy and Verification

**Dependencies**: Phase 1, Phase 2, Phase 3
**Status**: In Progress

### Description

Full validation and deployment to the homelab test environment per the standard
release flow.

### Items

- [x] **4.1 Local validation**
  - **Dependencies**: None
  - **Description**:
    - `make test` (Go tests with `-race`) — pass (11 packages)
    - `make lint` — golangci-lint pass (0 issues); markdownlint fails
      repo-wide on master (pre-existing, see Phase 3 criteria)
    - `make frontend-build`, copy `frontend/dist` → `internal/server/static/`,
      `make build` — pass (binary smoke-tested)

- [x] **4.2 Deploy and end-to-end verification**
  - **Dependencies**: 4.1
  - **Status**: Deployed and verified; one lookback self-clear observation
    pending (see below)
  - **Deploy**:
    - Dev image `v0.22.0-1` (multi-arch) pushed to Yandex CR, verified via
      `yc container image list`
    - `make uniproxy-deploy` (rev bumps: ns1→2, ns2→3→5, ns3→2) and
      `make helm-deploy` (dephealth-ui rev 4, image v0.22.0-1)
    - Recovered a missing `custom-ca` ConfigMap in namespace `dephealth-ui`
      (extracted `ca.crt` from cert-manager secret `dev-ca`) — it was
      referenced by homelab values but never created, blocking the rollout
  - **Verified** (via `/api/v1/topology` and visually in the UI):
    - single-source endpoint with explicit labels → ldap → `infra`/`identity`
      (compound containers visible in both dimensions) ✅
    - shared endpoint, unanimous labels → postgresql → `db`/`data` ✅
    - shared endpoint, conflicting labels (temporary flip of uniproxy-08 to
      `legacy`/`old-data`, then reverted) → namespace fell back to FQDN
      (`dephealth-postgresql`), group empty, and `meta.warnings` carried both
      sorted messages ✅
    - endpoint without labels → previous behavior: redis/grpc-stub →
      namespace from FQDN, group inherited from the sole source ✅
      (uniproxy-pr1 with a bare-IP host → sole-source inheritance) ✅
    - grouping works in both dimensions (namespace / group): confirmed
      visually (containers `infra`, `identity`, `proxy-cluster-1/2/3`,
      FQDN-fallback containers) ✅
  - **Lookback effect observed and confirmed**: after reverting the conflicting
    label, the instant query was unanimous again immediately, but the topology
    kept the conflict until the stale series aged out of the 1h lookback
    window (`last_over_time[1h]` still saw the `legacy` samples) — expected
    behavior, not a bug. Final check (15:14): `meta.warnings` empty,
    postgresql back to `db`/`data`, and the untouched open UI re-grouped it
    into the `db` container on auto-refresh without a page reload (closes the
    Phase 2 manual criterion).

### ✅ Phase 4 Completion Criteria

- [x] All items completed (4.1, 4.2)
- [x] All tests and linters pass (golangci-lint ✅; markdownlint blocked by
      pre-existing repo-wide failure on master — tracked separately)
- [x] E2E scenarios verified in the homelab environment
- [x] CHANGELOG updated; released as v0.22.0 (tag, GitHub Release,
      multi-arch image in Yandex CR)

---

## Notes

- **External repositories** (issues drafted, to be filed on GitHub):
  - `tmp/issue-topologymetrics.md` — spec: reserve `dep_namespace`/`dep_group`,
    convenience options in all 4 SDKs, conformance scenario, docs.
  - `tmp/issue-uniproxy.md` — per-dependency `depNamespace`/`depGroup` config
    (YAML + env), global defaults, Helm values, README EN/RU.
  - `tmp/uniproxy-dep-grouping-guide.md` — user-facing configuration guide for
    uniproxy (Russian draft; final home: uniproxy repo docs, EN + RU).
- **Rollout order**: dephealth-ui can ship first — missing labels degrade
  gracefully to current heuristics. The interim env-var mechanism
  (`DEPHEALTH_<DEP>_LABEL_DEP_NAMESPACE`) already works with the current Go SDK
  and current uniproxy, so early adopters are unblocked immediately.
- **Visible behavior change**: `group` inheritance switches from
  first-edge-wins to sole-source-only. Multi-source endpoints that previously
  landed in an arbitrary group will render ungrouped until explicit
  `dep_group` labels are set. Called out in CHANGELOG.
- **Deferred (not in this plan)**: server-side endpoint overrides in
  dephealth-ui `config.yaml` (Variant B) as an optional follow-up for endpoints
  whose owners cannot adopt the labels.

---

**✅ Plan complete — released as v0.22.0.**
