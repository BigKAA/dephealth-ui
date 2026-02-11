# Plan: Stale Node Retention (lookback window)

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-02-11
- **Last updated**: 2026-02-11
- **Status**: Pending

---

## Version history

- **v1.0.0** (2026-02-11): Initial plan version

---

## Current status

- **Active phase**: Phase 4
- **Active step**: 4.1
- **Last updated**: 2026-02-11
- **Note**: Phase 1+2+3 complete — config, PromQL, stale detection, frontend visualization

---

## Summary

When a service stops sending metrics (crash, scale-down, network issues), its time series become "stale" in Prometheus after ~5 minutes and disappear from instant queries. This causes the node to vanish from the topology graph.

**Solution**: Use `last_over_time()` PromQL function to fetch topology structure from a configurable time window, while using instant queries for current health. The difference reveals stale (disappeared) nodes which are displayed with `state="unknown"`.

### Key design decisions

1. **Zero backend state** — Prometheus serves as the time-series store; no in-memory graph history needed
2. **Configurable lookback window** — single config parameter (`topology.lookback`)
3. **Backward compatible** — `lookback: 0` preserves current behavior exactly
4. **Natural cleanup** — stale nodes automatically disappear after the lookback window expires

### Data flow

```
┌──────────────────────────────────────────────────────────────┐
│  Topology query (structure):                                 │
│  last_over_time(app_dependency_health{...}[LOOKBACK])        │
│  → ALL edges seen in the lookback window (current + stale)   │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│  Health query (instant):                                     │
│  app_dependency_health{...}                                  │
│  → ONLY current edges (live time series)                     │
└──────────────────────────────────────────────────────────────┘

Edge in topology but NOT in health → STALE → state="unknown"
Edge in both → CURRENT → use health value (ok/degraded/down)
```

---

## Table of contents

- [x] [Phase 1: Backend — config & PromQL](#phase-1-backend--config--promql)
- [x] [Phase 2: Backend — graph builder stale detection](#phase-2-backend--graph-builder-stale-detection)
- [x] [Phase 3: Frontend — unknown state visualization](#phase-3-frontend--unknown-state-visualization)
- [ ] [Phase 4: Build, test & deploy](#phase-4-build-test--deploy)
- [ ] [Phase 5: Documentation](#phase-5-documentation)

---

## Phase 1: Backend — config & PromQL

**Dependencies**: None
**Status**: Pending

### Description

Add the `topology.lookback` configuration parameter and extend `PrometheusClient` to support lookback-based topology queries using `last_over_time()`.

### Steps

- [ ] **1.1 Add `TopologyConfig` to configuration**
  - **Dependencies**: None
  - **Description**: Add a new `topology` section to `Config` with a `lookback` field (`time.Duration`). Default value: `0` (disabled — current behavior). Add env override `DEPHEALTH_TOPOLOGY_LOOKBACK`. Add validation: if set, must be >= 1m. Update `config.example.yaml`.
  - **Modifies**:
    - `internal/config/config.go` — add `TopologyConfig` struct, field in `Config`, default, env override, validation
    - `internal/config/config_test.go` — add test cases for lookback parsing and validation
    - `config.example.yaml` — add `topology.lookback` with comment

- [ ] **1.2 Extend `PrometheusClient` interface with lookback query**
  - **Dependencies**: 1.1
  - **Description**: Add a new method `QueryTopologyEdgesLookback(ctx, opts, lookback)` to the `PrometheusClient` interface. It uses the PromQL template: `group by (name, namespace, dependency, type, host, port, critical) (last_over_time(app_dependency_health{ns_filter}[LOOKBACK]))`. The existing `QueryTopologyEdges` stays unchanged (backward compatibility). Add a set of `EdgeKey` to the return value of `QueryHealthState` to identify which edges are currently live (or derive this from the health map keys).
  - **Modifies**:
    - `internal/topology/prometheus.go` — add `queryTopologyEdgesLookback` PromQL template, add `QueryTopologyEdgesLookback` method to interface and implementation
    - `internal/topology/prometheus_test.go` — add test for the new method (mock HTTP response)

- [ ] **1.3 Pass lookback duration into `GraphBuilder`**
  - **Dependencies**: 1.1
  - **Description**: Add a `lookback time.Duration` field to `GraphBuilder`. Pass it from `main.go` using the config value. If `lookback > 0`, `Build()` will call `QueryTopologyEdgesLookback` instead of `QueryTopologyEdges`.
  - **Modifies**:
    - `internal/topology/graph.go` — add `lookback` field to `GraphBuilder`, update `NewGraphBuilder` signature
    - `cmd/dephealth-ui/main.go` — pass `cfg.Topology.Lookback` to `NewGraphBuilder`

### Completion criteria Phase 1

- [ ] All steps completed (1.1, 1.2, 1.3)
- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/config/...` passes
- [ ] `go test ./internal/topology/...` passes
- [ ] `lookback: 0` produces identical behavior to current version

---

## Phase 2: Backend — graph builder stale detection

**Dependencies**: Phase 1
**Status**: Pending

### Description

Modify `Build()` and `buildGraph()` to detect stale edges (present in lookback topology but absent in current health) and mark them as `state="unknown"`. Add `Stale` field to `Node` and `Edge` models for frontend use.

### Steps

- [ ] **2.1 Add `Stale` field to models**
  - **Dependencies**: None
  - **Description**: Add `Stale bool` field to `Node` and `Edge` structs in `models.go`. JSON tag: `"stale,omitempty"`. This allows the frontend to easily style stale elements without parsing the state string.
  - **Modifies**:
    - `internal/topology/models.go` — add `Stale bool` to `Node` and `Edge`

- [ ] **2.2 Implement stale detection in `Build()` and `buildGraph()`**
  - **Dependencies**: 2.1, Phase 1
  - **Description**:
    In `Build()`, when `lookback > 0`:
    1. Call `QueryTopologyEdgesLookback()` to get ALL topology edges (current + stale)
    2. Call `QueryHealthState()` (instant) to get current health values
    3. Build a `currentEdgeKeys` set from the health map keys
    4. Pass `currentEdgeKeys` to `buildGraph()`

    In `buildGraph()`, when processing edges:
    - If edge's `EdgeKey` is NOT in `currentEdgeKeys` AND `currentEdgeKeys` is not nil:
      - Set edge `State = "unknown"`, `Health = -1`, `Latency = ""`, `LatencyRaw = 0`, `Stale = true`
      - Do NOT include in `nodeOutgoingHealth` / `nodeIncomingHealth` (stale health is meaningless)
    - Track a per-node `staleCount` and `totalCount`
    - For node state calculation:
      - If ALL edges are stale → `state = "unknown"`, `Stale = true`
      - If SOME edges are stale and remaining are mixed → use `calcNodeState` on non-stale edges only

    When `lookback == 0` (default), `currentEdgeKeys` is nil and all logic is skipped — exact current behavior.

  - **Modifies**:
    - `internal/topology/graph.go` — modify `Build()` flow, modify `buildGraph()` signature and logic

- [ ] **2.3 Unit tests for stale detection**
  - **Dependencies**: 2.2
  - **Description**: Add test cases to `graph_test.go`:
    1. **All edges current** — no stale nodes/edges, same as today
    2. **One service disappears** — its edges become stale, node state="unknown"
    3. **Partial stale** — service has 2 deps, one stale, one current → service state from non-stale edges only
    4. **Connected graph stale** — service-to-service edge becomes stale, through-node stays with unknown
    5. **Lookback disabled (0)** — exact current behavior
    6. **All edges stale** — everything is unknown (edge case: Prometheus returns lookback data but health query returns nothing)
  - **Modifies**:
    - `internal/topology/graph_test.go` — add `mockPrometheusClient` support for `QueryTopologyEdgesLookback`, add 6 test cases

### Completion criteria Phase 2

- [ ] All steps completed (2.1, 2.2, 2.3)
- [ ] `go test ./internal/topology/... -v` — all tests pass, including 6 new stale-detection tests
- [ ] With `lookback: 0` all existing tests pass unchanged
- [ ] Edge cases handled: all-stale, partial-stale, connected-graph-stale

---

## Phase 3: Frontend — unknown state visualization

**Dependencies**: Phase 2
**Status**: Pending

### Description

Update the frontend to visually distinguish stale (unknown) nodes and edges from active ones using dashed borders, gray coloring, and appropriate tooltips.

### Steps

- [ ] **3.1 Add unknown/stale styles to Cytoscape graph**
  - **Dependencies**: None
  - **Description**:
    - Add `unknown` entry to `EDGE_STYLES`: `{ lineStyle: 'dashed', color: '#9e9e9e' }` (gray dashed line)
    - Stale nodes: add `border-style: 'dashed'` for nodes where `data('stale')` is true
    - Service nodes with `state='unknown'`: gray background (`#9e9e9e`), dashed border
    - Dependency nodes with `state='unknown'`: gray background, dashed border
    - Ensure the existing `STATE_COLORS.unknown = '#9e9e9e'` is used consistently
  - **Modifies**:
    - `frontend/src/graph.js` — add `unknown` to `EDGE_STYLES`, add conditional `border-style` for stale nodes

- [ ] **3.2 Update tooltip for stale elements**
  - **Dependencies**: 3.1
  - **Description**:
    - When node or edge is stale, show a tooltip line: "Status: Unknown (metrics disappeared)" / "Статус: Неизвестно (метрики пропали)"
    - For stale edges, hide latency (show "—" or omit)
    - Add i18n keys: `state.unknown.detail` → "Metrics disappeared" / "Метрики пропали"
  - **Modifies**:
    - `frontend/src/tooltip.js` — handle stale flag in tooltip rendering
    - `frontend/src/locales/en.js` — add `state.unknown.detail` key
    - `frontend/src/locales/ru.js` — add `state.unknown.detail` key

- [ ] **3.3 Update sidebar details for stale nodes**
  - **Dependencies**: 3.1
  - **Description**:
    - In sidebar node details, show stale badge/indicator
    - State badge for unknown should use the existing `.sidebar-state-badge.unknown` CSS class (already exists)
    - For stale edges in edge detail view, show "—" for latency
  - **Modifies**:
    - `frontend/src/sidebar.js` — handle stale flag in sidebar rendering

- [ ] **3.4 Update stats counter in header**
  - **Dependencies**: 3.1
  - **Description**:
    - The header stats bar already counts `unknown` nodes (line ~121 in `main.js`). Verify it works correctly with stale nodes. No changes expected, just verification.
  - **Modifies**: None (verification only)

### Completion criteria Phase 3

- [ ] All steps completed (3.1, 3.2, 3.3, 3.4)
- [ ] Stale nodes render with gray dashed borders
- [ ] Stale edges render with gray dashed lines
- [ ] Tooltips show appropriate stale/unknown text
- [ ] Stats counter correctly shows unknown count
- [ ] No visual regressions for non-stale nodes/edges

---

## Phase 4: Build, test & deploy

**Dependencies**: Phase 1, Phase 2, Phase 3
**Status**: Pending

### Description

Build Docker image, deploy to Kubernetes, and verify stale node retention works end-to-end.

### Steps

- [ ] **4.1 Run all Go tests**
  - **Dependencies**: None
  - **Description**: `make test` — all tests pass with `-race` flag
  - **Creates**: Test results

- [ ] **4.2 Build frontend**
  - **Dependencies**: None
  - **Description**: `make frontend-build` — Vite build succeeds
  - **Creates**: `frontend/dist/`

- [ ] **4.3 Build Docker image**
  - **Dependencies**: 4.1, 4.2
  - **Description**: Build multi-arch image with dev tag: `make docker-build TAG=v0.11.4-1`
  - **Creates**: Docker image `harbor.kryukov.lan/library/dephealth-ui:v0.11.4-1`

- [ ] **4.4 Update Helm chart and deploy**
  - **Dependencies**: 4.3
  - **Description**:
    - Update `deploy/helm/dephealth-ui/values.yaml` — add `topology.lookback: 1h` to config section
    - Update image tag to `v0.11.4-1`
    - Deploy with `make helm-deploy`
  - **Modifies**:
    - `deploy/helm/dephealth-ui/values.yaml` — add topology config, update image tag

- [ ] **4.5 End-to-end verification**
  - **Dependencies**: 4.4
  - **Description**:
    1. Verify graph displays normally with all services running
    2. Scale down one uniproxy instance (`kubectl scale deploy ... --replicas=0`)
    3. Wait for Prometheus staleness (~5 min)
    4. Verify: the scaled-down service remains on graph with `state=unknown`, gray dashed styling
    5. Verify: edges to/from the service are shown as dashed gray with state=unknown
    6. Scale the service back up → verify it transitions back to normal state
    7. Wait for lookback window to expire → verify the node disappears naturally
  - **Creates**: Test results

### Completion criteria Phase 4

- [ ] All steps completed (4.1–4.5)
- [ ] Docker image built and pushed
- [ ] Helm deploy successful
- [ ] E2E: stale node displayed as unknown after metrics disappear
- [ ] E2E: node returns to normal after metrics reappear
- [ ] E2E: node disappears after lookback window expires

---

## Phase 5: Documentation

**Dependencies**: Phase 4
**Status**: Pending

### Description

Update project documentation to describe the stale node retention feature.

### Steps

- [ ] **5.1 Update `docs/application-design.md`**
  - **Dependencies**: None
  - **Description**: Add section describing the lookback/stale retention mechanism, PromQL queries used, and configuration.
  - **Modifies**:
    - `docs/application-design.md`

- [ ] **5.2 Update `config.example.yaml` comments**
  - **Dependencies**: None
  - **Description**: Ensure `topology.lookback` is documented with clear comments explaining the feature and recommended values.
  - **Modifies**:
    - `config.example.yaml` (done in Phase 1, verify completeness)

- [ ] **5.3 Update Helm chart README**
  - **Dependencies**: None
  - **Description**: Add `topology.lookback` to the Helm values documentation table.
  - **Modifies**:
    - `deploy/helm/dephealth-ui/README.md`

### Completion criteria Phase 5

- [ ] All steps completed (5.1, 5.2, 5.3)
- [ ] Documentation accurately describes the feature
- [ ] Configuration example is clear and has good defaults

---

## Notes

- **PromQL compatibility**: `last_over_time()` is supported by both Prometheus and VictoriaMetrics
- **Performance**: The lookback query uses `group by` + `last_over_time`, which is lightweight. No significant overhead expected.
- **Recommended lookback values**:
  - `1h` — good default for most environments
  - `6h` — for environments with infrequent deployments
  - `24h` — maximum practical value (longer windows increase query cost)
  - `0` — disabled (current behavior)
- **No breaking changes**: The feature is opt-in (`lookback: 0` by default). Existing deployments are unaffected.
- **Health value for stale edges**: Set to `-1` (sentinel) to distinguish from healthy (1) and down (0). Frontend uses the `stale` boolean flag for styling, not the health value.
