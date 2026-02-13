# Plan: Grafana Drill-Down Chain & Cascade Graph Visualization

## Metadata

- **Version**: 1.0.0
- **Created**: 2025-02-13
- **Last updated**: 2025-02-13
- **Status**: Completed

---

## Version History

- **v1.0.0** (2025-02-13): Initial plan

---

## Current Status

- **Active phase**: Completed
- **Active item**: —
- **Last updated**: 2025-02-13
- **Note**: All 4 phases completed. Deployed as v0.14.2-4, Helm revision 41 (ui) / 23 (monitoring).

---

## Table of Contents

- [x] [Phase 1: Backend — Config & Cascade Graph API](#phase-1-backend--config--cascade-graph-api)
- [x] [Phase 2: Frontend — Sidebar & Context Menu](#phase-2-frontend--sidebar--context-menu)
- [x] [Phase 3: Build, Deploy & E2E Testing](#phase-3-build-deploy--e2e-testing)
- [x] [Phase 4: Grafana — Node Graph Panel](#phase-4-grafana--node-graph-panel)

---

## Phase 1: Backend — Config & Cascade Graph API

**Dependencies**: None
**Status**: Pending

### Description

Add two new Grafana dashboard UIDs (`cascadeOverview`, `rootCause`) to the backend config
and create a new `/api/v1/cascade-graph` endpoint that returns topology data in Grafana
Node Graph panel format (`{nodes[], edges[]}`).

### Items

- [ ] **1.1 Add new dashboard UIDs to config**
  - **Dependencies**: None
  - **Description**: Add `CascadeOverview` and `RootCause` fields to `DashboardsConfig` struct
    in `internal/config/config.go`. Add corresponding env var overrides
    (`DEPHEALTH_GRAFANA_DASHBOARDS_CASCADEOVERVIEW`, `DEPHEALTH_GRAFANA_DASHBOARDS_ROOTCAUSE`).
    Update `configDashboards` response struct and `handleConfig()` in `internal/server/server.go`
    to serve new UIDs. Update `config.example.yaml`.
  - **Modifies**:
    - `internal/config/config.go` — `DashboardsConfig` struct (line 88)
    - `internal/server/server.go` — `configDashboards` struct (line ~230), `handleConfig()` (line ~238)
    - `config.example.yaml`
  - **Links**:
    - [internal/config/config.go:88](internal/config/config.go) — DashboardsConfig
    - [internal/server/server.go:218](internal/server/server.go) — configResponse types

- [ ] **1.2 Implement cascade-graph endpoint**
  - **Dependencies**: None (parallel with 1.1)
  - **Description**: New handler `handleCascadeGraph()` in `internal/server/server.go`.
    Route: `GET /api/v1/cascade-graph?service=&namespace=&depth=`.
    Uses existing `cascade.Analyze()` / `cascade.AnalyzeForService()` to get BFS results,
    then transforms into Node Graph format:
    ```json
    {
      "nodes": [
        {
          "id": "service-name",
          "title": "service-name",
          "subTitle": "namespace",
          "mainStat": "down",
          "arc__failed": 1.0,
          "arc__ok": 0.0
        }
      ],
      "edges": [
        {
          "id": "src--tgt",
          "source": "src-service",
          "target": "tgt-service",
          "mainStat": "critical"
        }
      ]
    }
    ```
    Node deduplication: collect unique services from `RootCauses`, `AffectedServices`,
    and `AllFailures`. Color by state: down → arc__failed=1, degraded → arc__degraded=1,
    ok → arc__ok=1, unknown → arc__unknown=1.
    Edges: derive from `CascadeChains` (AffectedService → DependsOn).
    Register on chi router alongside `cascade-analysis`.
  - **Modifies**:
    - `internal/server/server.go` — new handler + route registration (line ~81)
  - **Links**:
    - [internal/cascade/cascade.go:161](internal/cascade/cascade.go) — Analyze()
    - [internal/cascade/cascade.go:334](internal/cascade/cascade.go) — AnalyzeForService()
    - [Grafana Node Graph docs](https://grafana.com/docs/grafana/latest/panels-visualizations/visualizations/node-graph/)

- [ ] **1.3 Unit tests for cascade-graph**
  - **Dependencies**: 1.2
  - **Description**: Add tests for the new `handleCascadeGraph()` handler in
    `internal/server/server_test.go`. Test cases:
    - Empty topology → `{"nodes":[],"edges":[]}`
    - Service with cascade → correct nodes and edges
    - With namespace filter
    - Node deduplication (same service appears as root cause and affected)
  - **Creates**:
    - Test cases in `internal/server/server_test.go`

### Completion Criteria — Phase 1

- [ ] All items completed (1.1, 1.2, 1.3)
- [ ] `go test ./internal/...` passes
- [ ] New endpoint returns valid Node Graph JSON
- [ ] Config API returns new dashboard UIDs

---

## Phase 2: Frontend — Sidebar & Context Menu

**Dependencies**: Phase 1
**Status**: Pending

### Description

Update the frontend to show new Grafana dashboard links (Cascade Overview, Root Cause Analyzer)
in the sidebar and context menu. Reorder sidebar dashboards. Add Root Cause Analyzer to the
context menu for nodes in the cascade chain only.

### Items

- [ ] **2.1 Add i18n translation keys**
  - **Dependencies**: None
  - **Description**: Add new keys to `frontend/src/locales/en.js` and `frontend/src/locales/ru.js`:
    ```
    sidebar.grafana.cascadeOverview → "Cascade Overview" / "Обзор каскадов"
    sidebar.grafana.rootCause → "Root Cause Analyzer" / "Анализ первопричин"
    contextMenu.rootCauseAnalysis → "Root Cause Analysis" / "Анализ первопричины"
    ```
  - **Modifies**:
    - `frontend/src/locales/en.js` (after line 111)
    - `frontend/src/locales/ru.js` (after line 111)

- [ ] **2.2 Update sidebar — service node dashboard order**
  - **Dependencies**: 2.1
  - **Description**: Rewrite `renderGrafanaDashboards()` in `frontend/src/sidebar.js` (line 340)
    to show dashboards in the new order when a service node is selected:
    1. **Cascade Overview** (`var-namespace={ns}`) — new
    2. **Root Cause Analyzer** (`var-service={id}&var-namespace={ns}`) — new
    3. **Service Status** (`var-service={id}`) — existing
    4. **Link Status** — existing
    5. **Service List** — existing
    6. **Services Status** — existing
    7. **Links Status** — existing
  - **Modifies**:
    - `frontend/src/sidebar.js` — `renderGrafanaDashboards()` (line 340)
  - **Links**:
    - [frontend/src/sidebar.js:340](frontend/src/sidebar.js) — renderGrafanaDashboards

- [ ] **2.3 Update sidebar — edge dashboard order**
  - **Dependencies**: 2.1
  - **Description**: Update `renderEdgeGrafanaDashboards()` in `frontend/src/sidebar.js` (line 641)
    to include Cascade Overview at the top (with `var-namespace` from source node).
    New order:
    1. **Cascade Overview** (`var-namespace={sourceNamespace}`) — new
    2. **Link Status** (`var-service=...&var-dependency=...`) — existing
    3. **Service Status** (`var-service={source}`) — existing
    4. **Links Status** — existing
  - **Modifies**:
    - `frontend/src/sidebar.js` — `renderEdgeGrafanaDashboards()` (line 641)

- [ ] **2.4 Context menu — Root Cause Analyzer**
  - **Dependencies**: 2.1
  - **Description**: Add "Root Cause Analysis" item to the right-click context menu for
    service nodes **only when the node has `inCascadeChain` data flag** (set by
    `computeCascadeWarnings()` in `cascade.js`).
    In `contextmenu.js` (line ~28), after the existing Grafana items, add:
    ```javascript
    if (data.inCascadeChain && grafanaConfig?.baseUrl && grafanaConfig?.dashboards?.rootCause) {
      items.push({
        label: t('contextMenu.rootCauseAnalysis'),
        icon: 'bi-search',
        action: () => {
          const url = `${grafanaConfig.baseUrl}/d/${grafanaConfig.dashboards.rootCause}/?var-service=${encodeURIComponent(data.id)}&var-namespace=${encodeURIComponent(data.namespace || '')}`;
          window.open(url, '_blank');
        },
      });
    }
    ```
    Need to import/access `grafanaConfig` in contextmenu.js — use the same pattern as sidebar
    (`setGrafanaConfig()` / module-level variable, or pass via init parameter).
  - **Modifies**:
    - `frontend/src/contextmenu.js` (line ~28)
  - **Links**:
    - [frontend/src/cascade.js](frontend/src/cascade.js) — computeCascadeWarnings sets inCascadeChain
    - [frontend/src/contextmenu.js:28](frontend/src/contextmenu.js) — service node handler

### Completion Criteria — Phase 2

- [ ] All items completed (2.1, 2.2, 2.3, 2.4)
- [ ] Sidebar shows correct dashboard order for service nodes
- [ ] Sidebar shows Cascade Overview for edge selection
- [ ] Context menu shows "Root Cause Analysis" only for nodes in cascade chain
- [ ] All links open correct Grafana dashboards with correct parameters
- [ ] i18n works for both EN and RU

---

## Phase 3: Build, Deploy & E2E Testing

**Dependencies**: Phase 1, Phase 2
**Status**: Pending

### Description

Build Docker image, deploy to Kubernetes, and verify all changes end-to-end.

### Items

- [ ] **3.1 Build Docker image**
  - **Dependencies**: None
  - **Description**: Build and push multi-arch Docker image.
    Tag: `v0.14.2-4` (next dev tag).
    ```bash
    make docker-build TAG=v0.14.2-4
    ```
  - **Creates**:
    - Docker image `harbor.kryukov.lan/library/dephealth-ui:v0.14.2-4`

- [ ] **3.2 Deploy to Kubernetes**
  - **Dependencies**: 3.1
  - **Description**: Update Helm values to use new image tag and add new dashboard UIDs:
    ```yaml
    grafana:
      dashboards:
        cascadeOverview: "dephealth-cascade-overview"
        rootCause: "dephealth-root-cause"
    ```
    Deploy via `helm upgrade`.
  - **Links**:
    - [deploy/helm/dephealth-ui/](deploy/helm/dephealth-ui/) — Helm chart

- [ ] **3.3 E2E verification**
  - **Dependencies**: 3.2
  - **Description**: Verify in the deployed environment:
    1. `GET /api/v1/config` returns new dashboard UIDs
    2. `GET /api/v1/cascade-graph` returns valid Node Graph JSON
    3. `GET /api/v1/cascade-graph?service=...` returns filtered graph
    4. Sidebar shows dashboards in correct order
    5. Context menu shows "Root Cause Analysis" for cascade nodes
    6. All Grafana links open correctly with proper variables

### Completion Criteria — Phase 3

- [ ] All items completed (3.1, 3.2, 3.3)
- [ ] Docker image built and pushed
- [ ] Pod running in Kubernetes
- [ ] All E2E checks pass
- [ ] No regressions in existing functionality

---

## Phase 4: Grafana — Node Graph Panel

**Dependencies**: Phase 3
**Status**: Pending

### Description

Add a Node Graph panel to the "dephealth: Root Cause Analyzer" Grafana dashboard.
The panel uses the Infinity datasource to call `/api/v1/cascade-graph` and visualize
the dependency failure graph. Update dashboard drill-down links.

### Items

- [ ] **4.1 Add Node Graph panel to Root Cause Analyzer**
  - **Dependencies**: None
  - **Description**: Edit `deploy/helm/dephealth-monitoring/dashboards/root-cause-analyzer.json`.
    Add a new panel of type `nodeGraph` with two Infinity queries:
    - **Query A** (nodes): `GET /api/v1/cascade-graph?service=${service}&namespace=${namespace}`,
      `root_selector: "nodes"`, columns: `id` (String), `title` (String), `subTitle` (String),
      `mainStat` (String), `arc__failed` (Number), `arc__ok` (Number),
      `arc__degraded` (Number), `arc__unknown` (Number)
    - **Query B** (edges): same URL, `root_selector: "edges"`,
      columns: `id` (String), `source` (String), `target` (String), `mainStat` (String)
    - Panel title: "Cascade Dependency Graph" or "Dependency Failure Graph"
    - Position: after the stat panels, before tables (row 1-2 area)
    - Arc colors: failed=#F2495C (red), degraded=#FF9830 (orange), ok=#73BF69 (green),
      unknown=#8AB8FF (blue)
    - Node click → drill-down to Service Status: `/d/dephealth-service-status/?var-service=${__data.fields.id}&${__url_time_range}`
  - **Modifies**:
    - `deploy/helm/dephealth-monitoring/dashboards/root-cause-analyzer.json`
  - **Links**:
    - [Grafana Node Graph panel](https://grafana.com/docs/grafana/latest/panels-visualizations/visualizations/node-graph/)
    - [Infinity datasource](https://grafana.com/docs/plugins/yesoreyeram-infinity-datasource/)

- [ ] **4.2 Deploy updated dashboards**
  - **Dependencies**: 4.1
  - **Description**: Redeploy `dephealth-monitoring` Helm chart to update Grafana dashboards:
    ```bash
    helm upgrade dephealth-monitoring deploy/helm/dephealth-monitoring -n dephealth-monitoring
    ```
    Verify Node Graph panel renders correctly in Grafana.

- [ ] **4.3 Verify drill-down links**
  - **Dependencies**: 4.2
  - **Description**: Verify the full drill-down chain works:
    1. UI → click service node → sidebar shows Cascade Overview first
    2. Cascade Overview → click service in table → Root Cause Analyzer
    3. Root Cause Analyzer → Node Graph shows dependency graph
    4. Root Cause Analyzer → click node in graph → Service Status
    5. Service Status → click dependency → Link Status
    Also verify context menu "Root Cause Analysis" opens Root Cause Analyzer correctly.

### Completion Criteria — Phase 4

- [ ] All items completed (4.1, 4.2, 4.3)
- [ ] Node Graph panel renders in Root Cause Analyzer
- [ ] Nodes colored by state
- [ ] Drill-down from graph nodes to Service Status works
- [ ] Full drill-down chain verified end-to-end

---

## Notes

- **Image tag**: v0.14.2-4 (next dev tag after current v0.14.2-3)
- **Backward compatibility**: existing API endpoints and dashboards remain unchanged
- **`inCascadeChain` flag**: set by frontend `computeCascadeWarnings()` in cascade.js, not by backend.
  Context menu must check `node.data('inCascadeChain')` to decide whether to show Root Cause item.
- **Grafana Node Graph arc fields**: `arc__*` fields must be floats 0.0-1.0 and should sum to 1.0
  per node. They control the colored arc ring around the node.
- **Helm chart config**: new dashboard UIDs need to be added to `values.yaml` defaults
  (check if config is passed via env vars or values)

---
