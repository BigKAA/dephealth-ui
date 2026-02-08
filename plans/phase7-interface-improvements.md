# Phase 7: Interface Improvements — Implementation Workflow

> Based on: `.tasks/requirements-interface-improvements.md`
> Requirements: R1 (endpoint coloring), R2 (dedup nodes), R3 (namespace filter),
> R4 (frontend filters), R5 (enriched model)

## Overview

Three implementation phases, each deployable independently:
- **Phase 7.1** — Data model + dedup + coloring (R5 + R2 + R1) — backend focus
- **Phase 7.2** — Namespace filtering (R3) — backend + API + frontend
- **Phase 7.3** — Frontend filters (R4) — pure frontend

## Design Decisions

### Cache strategy with namespace filtering (R3)
- Unfiltered requests (`/api/v1/topology`): use existing TTL cache as-is.
- Namespace-filtered requests (`?namespace=X`): bypass cache, query Prometheus
  directly. Namespace filtering is an infrequent analytical operation; caching
  per-namespace adds complexity with little benefit.

### Alert matching after EdgeKey change (R2)
- AlertManager alerts reference `service` (=job) and `dependency` (=dep name).
- After R2, EdgeKey becomes `{Job, Host, Port}`.
- Solution: build a reverse lookup map `(job, dependency) → (host, port)` from
  rawEdges during `buildGraph()`. Use it in `enrichWithAlerts()` to translate
  alert labels to the new EdgeKey.

### Grafana URLs after R2
- Current `linkGrafanaURL(job, dep)` is **broken**: generates `var-dep=X` but the
  `dephealth-link-status` dashboard expects `var-dependency`, `var-host`, `var-port`.
- After R2, change signature to `linkGrafanaURL(job, dependency, host, port string)`.
- Generate URL: `?var-dependency={dependency}&var-host={host}&var-port={port}`.
- This aligns with the actual Grafana template variables in `link-status.json`.

### Namespace for non-K8s services
- Prometheus `group by (namespace, ...)` naturally assigns `namespace=""` to
  series without this label. Use empty string as-is — no artificial values.
- Go model: `Namespace string` (without `omitempty`) — always present in JSON.
- Frontend displays empty namespace as "— (no namespace)" or similar label.

### Filter persistence strategy
- **Namespace filter** → URL query parameter `?namespace=X` (shareable, backend-synced).
- **Frontend-only filters** (type, state, job) → `localStorage` key `dephealth-filters`.
- "Reset filters" button clears all.
- Precedent: theme already stored in localStorage.

---

## Phase 7.1 — Data Model, Dedup, Coloring

**Branch**: `feature/topology-dedup-coloring`
**Scope**: R5 + R2 + R1 — tightly coupled, done together.

### Step 1: Update models (R5 + R2)

**File**: `internal/topology/models.go`

Changes:
- [x] `Node` — add fields: `Namespace string`, `Host string`, `Port string`
  ```go
  type Node struct {
      ID              string `json:"id"`
      Label           string `json:"label"`
      State           string `json:"state"`
      Type            string `json:"type"`
      Namespace       string `json:"namespace"`
      Host            string `json:"host,omitempty"`
      Port            string `json:"port,omitempty"`
      DependencyCount int    `json:"dependencyCount"`
      GrafanaURL      string `json:"grafanaUrl,omitempty"`
  }
  ```
- [x] `Edge` — add field: `Type string`
  ```go
  type Edge struct {
      Source     string  `json:"source"`
      Target     string  `json:"target"`
      Type       string  `json:"type,omitempty"` // grpc, http, postgres, redis
      Latency    string  `json:"latency"`
      LatencyRaw float64 `json:"latencyRaw"`
      Health     float64 `json:"health"`
      State      string  `json:"state"`
      GrafanaURL string  `json:"grafanaUrl,omitempty"`
  }
  ```
- [x] `EdgeKey` — change from `{Job, Dependency}` to `{Job, Host, Port}`
  ```go
  type EdgeKey struct {
      Job  string
      Host string
      Port string
  }
  ```
- [x] `TopologyEdge` — add `Namespace` field
  ```go
  type TopologyEdge struct {
      Job        string
      Namespace  string
      Dependency string
      Type       string
      Host       string
      Port       string
  }
  ```

### Step 2: Update Prometheus queries

**File**: `internal/topology/prometheus.go`

Changes:
- [x] `QueryTopologyEdges()` — also extract `namespace` label into TopologyEdge.
  Add `namespace` to group-by in PromQL constant:
  ```
  queryTopologyEdges = `group by (job, namespace, dependency, type, host, port) (app_dependency_health)`
  ```
- [x] `parseEdgeValues()` — construct `EdgeKey{Job, Host, Port}` instead of
  `EdgeKey{Job, Dependency}`. Metric still has `host` and `port` labels, so:
  ```go
  key := EdgeKey{
      Job:  r.Metric["job"],
      Host: r.Metric["host"],
      Port: r.Metric["port"],
  }
  ```

### Step 3: Update GraphBuilder

**File**: `internal/topology/graph.go`

#### 3a: Rewrite `buildGraph()` for dedup (R2)

- [x] Dependency node ID = `host:port`, label = `host`
- [x] Service node ID = `job` (unchanged)
- [x] Edge keyed by `{Job, Host, Port}` (new EdgeKey)
- [x] Track `depNamesByEndpoint` map for alert matching:
  `map[string][]string` — `"host:port" → ["redis", "redis-cache"]`
- [x] Build reverse lookup: `map[depAlertKey]EdgeKey` where
  `depAlertKey = {Job, Dependency}` → used by enrichWithAlerts()
- [x] Populate new Node fields: `Namespace`, `Host`, `Port`
- [x] Populate new Edge field: `Type`
- [x] Service nodes get Namespace from first encountered raw edge
- [x] `nodeInfo` struct: add `namespace`, `host`, `port` fields

#### 3b: Fix endpoint coloring (R1)

- [x] Collect incoming edge health per target node:
  ```go
  nodeIncomingHealth := make(map[string][]float64)
  // in edge loop:
  nodeIncomingHealth[targetID] = append(nodeIncomingHealth[targetID], h)
  ```
- [x] For dependency nodes (type != "service"), use incoming health:
  ```go
  if info.typ == "service" {
      state = calcNodeState(nodeEdgeHealth[id])  // outgoing (existing)
  } else {
      state = calcNodeState(nodeIncomingHealth[id])  // incoming (new)
  }
  ```

#### 3c: Update `enrichWithAlerts()`

- [x] Accept additional param: reverse lookup map from step 3a
  (`depLookup map[depAlertKey]EdgeKey`).
- [x] When matching alerts: translate `(a.Service, a.Dependency)` → EdgeKey
  via the reverse lookup, then find edge by EdgeKey.
- [x] Update `Build()` to pass reverse lookup to `enrichWithAlerts()`.

#### 3d: Update Grafana URLs

- [x] `linkGrafanaURL(job, dependency, host, port string)` — pass all three params:
  ```go
  GrafanaURL: b.linkGrafanaURL(raw.Job, raw.Dependency, raw.Host, raw.Port),
  ```
  Generates: `?var-dependency={dependency}&var-host={host}&var-port={port}`
- [x] Fix bug: rename `var-dep` → `var-dependency` to match actual dashboard template vars

### Step 4: Update tests

**File**: `internal/topology/graph_test.go`
- [x] Update test data: EdgeKey fields, expected node IDs (`host:port`)
- [x] Add test case: two services with different dependency names to same
  host:port → single dependency node
- [x] Add test case: dependency node state from incoming edges (R1)
- [x] Add test case: all incoming edges down → dependency state "down"
- [x] Add test case: mixed incoming edges → dependency state "degraded"

**File**: `internal/topology/prometheus_test.go`
- [x] Update EdgeKey assertions to use `{Job, Host, Port}`
- [x] Add namespace field to TopologyEdge test assertions

### Step 5: Update frontend edge IDs

**File**: `frontend/src/graph.js`
- [x] Edge ID: currently `${edge.source}->${edge.target}`.
  Target is now `host:port`. The format still works, just the value changes.
  No code change needed — verified.

### Step 6: Build, test, deploy

- [x] `go test ./internal/topology/...`
- [x] `go test ./...`
- [x] Docker build + push (`v0.6.0`)
- [x] Helm upgrade dephealth-ui
- [x] Verify in browser: 4 endpoint nodes instead of 8, correct coloring

### Checkpoint

After Phase 7.1:
- [x] Graph shows 4 unique dependency nodes (redis, grpc-stub, http-stub, postgres-primary)
- [x] Dependency nodes reflect incoming edge health (green/orange/red)
- [x] Service nodes still reflect outgoing edge health
- [x] Alerts still match correctly
- [x] All Go tests pass

---

## Phase 7.2 — Backend Namespace Filtering

**Branch**: `feature/namespace-filter`
**Scope**: R3

### Step 1: Parameterize PromQL queries

**File**: `internal/topology/prometheus.go`

- [x] Change PrometheusClient interface: all methods accept optional
  `namespace string` parameter (or use a `QueryOptions` struct).
  Preferred: `QueryOptions` struct for extensibility.
  ```go
  type QueryOptions struct {
      Namespace string
  }
  ```
- [x] Update all 4 methods: `QueryTopologyEdges(ctx, opts)`,
  `QueryHealthState(ctx, opts)`, etc.
- [x] In `query()` method or in each Query method: if `opts.Namespace != ""`,
  append `{namespace="<value>"}` to PromQL.
  For constants, use `fmt.Sprintf`:
  ```go
  q := queryTopologyEdges
  if opts.Namespace != "" {
      q = fmt.Sprintf(`group by (job, namespace, dependency, type, host, port) (app_dependency_health{namespace="%s"})`, opts.Namespace)
  }
  ```

### Step 2: Update GraphBuilder

**File**: `internal/topology/graph.go`

- [x] `Build(ctx, opts QueryOptions)` — accept options and pass to prom methods
- [x] No other logic change — namespace filtering happens at PromQL level

### Step 3: Update server handler

**File**: `internal/server/server.go`

- [x] `handleTopology()` — parse `r.URL.Query().Get("namespace")`
- [x] If namespace is set: bypass cache, call `builder.Build(ctx, opts)` directly
- [x] If namespace is empty: use existing cache logic (no change)
- [x] Return filtered result

### Step 4: Update cache interface

**File**: `internal/cache/cache.go`

- [x] No change to cache itself — namespace-filtered requests bypass cache.

### Step 5: Update frontend API

**File**: `frontend/src/api.js`

- [x] `fetchTopology(namespace)` — accept optional namespace param
- [x] If namespace provided: append `?namespace=<value>` to URL
- [x] If namespace provided: skip ETag logic (always fresh fetch)

### Step 6: Add namespace selector to frontend

**File**: `frontend/index.html`
- [x] Add namespace dropdown/select in toolbar (or header area)

**File**: `frontend/src/main.js`
- [x] Fetch available namespaces (from topology data or separate endpoint)
- [x] On namespace change: call `fetchTopology(namespace)`, re-render
- [x] When namespace is selected: disable auto-refresh ETag optimization
  (or keep auto-refresh but always pass namespace)

### Step 7: Update tests

- [x] `internal/topology/prometheus_test.go` — test namespace filter in PromQL
- [x] `internal/topology/graph_test.go` — test Build with QueryOptions
- [x] `internal/server/server_test.go` — test namespace query param handling

### Step 8: Build, test, deploy

- [x] `go test ./...`
- [x] Docker build + push (`v0.6.0`)
- [x] Helm upgrade
- [x] Verify: `?namespace=dephealth-test` filters correctly

### Checkpoint

After Phase 7.2:
- [x] API accepts `?namespace=X` parameter
- [x] Filtered requests bypass cache and query Prometheus directly
- [x] Unfiltered requests use cache as before
- [x] Frontend has namespace selector
- [x] All tests pass

---

## Phase 7.3 — Frontend Filters

**Branch**: `feature/frontend-filters`
**Scope**: R4

### Step 1: Filter UI components

**File**: `frontend/index.html`
- [x] Add filter panel between header and `#cy` container (with Reset button)
- [x] Add filter toggle button in toolbar

### Step 2: Filter styles

**File**: `frontend/src/style.css`
- [x] `#filter-panel` — horizontal bar below header, flexbox wrap
- [x] `.filter-group` — inline group with label + chips
- [x] `.filter-chip` — toggle button style (active/inactive states)
- [x] `.filter-chip.active` — highlighted when filter is active
- [x] Dark theme support via CSS custom properties
- [x] Responsive: wrap on small screens

### Step 3: Filter module

**File**: `frontend/src/filter.js` (new file)
- [x] `initFilters(data)` — populate filter chips, restore from localStorage
- [x] `applyFilters(cy)` — show/hide Cytoscape elements (AND logic)
  - Service nodes: match by job + state
  - Dependency nodes: match by type + state
  - Edges: visible only if both endpoints visible
  - Orphan nodes: hidden when all connected edges hidden
- [x] `getActiveFilters()` — return current filter state
- [x] `updateFilters(data)` — refresh chips, preserve selections
- [x] `hasActiveFilters()` — check if any filter active
- [x] `resetFilters()` — clear all + remove from localStorage

### Step 4: Integrate filters into main.js

**File**: `frontend/src/main.js`
- [x] Import filter functions from filter.js
- [x] `refresh()`: call `updateFilters(data)` + `applyFilters(cy)` after render
- [x] `setupFilters()`: toggle button, reset button, `filters-changed` event
- [x] Panel visibility persisted in localStorage
- [x] Status bar shows "Filtered" indicator when filters active

### Step 5: Preserve filters across data refresh

- [x] Filters re-applied after every `renderGraph()` cycle
- [x] Filter selections persist as JS `Set` per dimension (not lost on poll)

### Step 6: Hybrid filter persistence

- [x] **Namespace** → URL query param `?namespace=X` (Phase 7.2, unchanged)
- [x] **Type, State, Job** → `localStorage` key `dephealth-filters`
- [x] Restore on page load from localStorage
- [x] "Reset" button clears localStorage + removes namespace from URL

### Step 7: Build and deploy

- [x] `npm run build` (Vite) — OK
- [x] Docker build + push (`v0.6.0`)
- [x] Helm upgrade
- [x] Verify: filter chips appear, toggling hides/shows elements correctly

### Checkpoint

After Phase 7.3:
- [x] Filter panel toggleable from toolbar
- [x] Type filter: multi-select chips for grpc, http, postgres, redis, etc.
- [x] State filter: ok, degraded, down, unknown
- [x] Job filter: per-service toggle
- [x] Filters combine with AND logic
- [x] Filters survive data refresh
- [x] Connected orphan nodes hidden correctly
- [x] Dark/light theme supported

---

## Summary

| Phase | Scope | Effort | Dependencies |
|-------|-------|--------|-------------|
| 7.1 | Models + Dedup + Coloring | Medium | None |
| 7.2 | Namespace filter | Medium | 7.1 (enriched model) |
| 7.3 | Frontend filters | Medium | 7.1 (enriched data in API) |

Each phase ends with: tests pass, Docker build, Helm deploy, browser verify.
