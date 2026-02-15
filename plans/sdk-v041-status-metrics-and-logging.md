# Plan: SDK v0.4.1 Status Metrics + Frontend Integration + Grafana Dashboards + Configurable Logging

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-02-15
- **Last updated**: 2026-02-15
- **Status**: Pending

---

## Version History

- **v1.0.0** (2026-02-15): Initial plan

---

## Current Status

- **Active phase**: Phase 1
- **Active item**: —
- **Last updated**: 2026-02-15
- **Note**: Plan created, awaiting approval

---

## Context

SDK topologymetrics v0.4.1 adds two new Prometheus metrics (`app_dependency_status`, `app_dependency_status_detail`) that expose **why** a connection fails (timeout, dns_error, tls_error, etc.), not just **whether** it fails. This enriches our topology graph with diagnostic information. Additionally, the uniproxy project has a clean `log/slog` logging pattern we want to adopt for dephealth-ui backend.

**Goal:** Full integration of new metrics into backend API, frontend visualization (colors, labels, filters, sidebar, tooltip), Grafana dashboards, plus configurable structured logging.

**Target version:** v0.15.0

---

## Phase Dependencies

```
Phase 1: Logging (independent)        ─────────────────────►
Phase 2: Backend SDK metrics           ─────────────────────►
Phase 3: Frontend integration          ── depends on Phase 2 ►
Phase 4: Grafana dashboards            ── depends on Phase 2 ►
Phase 5: Uniproxy + E2E testing       ── depends on 2,3,4 ──►
```

Phases 1 and 2 run in parallel. Phases 3 and 4 run in parallel after Phase 2.

---

## Table of Contents

- [ ] [Phase 1: Configurable Logging](#phase-1-configurable-logging)
- [ ] [Phase 2: Backend — New Status Metrics](#phase-2-backend--new-status-metrics)
- [ ] [Phase 3: Frontend — Full Integration](#phase-3-frontend--full-integration)
- [ ] [Phase 4: Grafana Dashboards](#phase-4-grafana-dashboards)
- [ ] [Phase 5: Uniproxy + E2E Testing](#phase-5-uniproxy--e2e-testing)

---

## Phase 1: Configurable Logging

**Dependencies**: None (fully independent)
**Status**: Pending

### Description

Create `internal/logging/` package adopting the pattern from uniproxy, add `LogConfig` to config, replace chi's `middleware.Logger` with a custom slog-based HTTP request logger, update Helm values.

### Items

- [ ] **1.1 Create logging package**
  - **Dependencies**: None
  - **Description**: Create `internal/logging/` with `NewLogger(cfg)` factory, `parseLevel`, `buildReplaceAttr`, `newHandler`. Supports text/JSON output, custom JSON keys for log aggregator compatibility (ECS, GCP, Splunk), configurable timestamp formats (rfc3339, rfc3339nano, unix, unixmilli), source location. Copy pattern from uniproxy `internal/logging/logging.go`.
  - **Creates**:
    - `internal/logging/logging.go`
    - `internal/logging/logging_test.go`
  - **Links**:
    - Reference: `~/Projects/personal/topologymetrics/uniproxy/internal/logging/logging.go`

- [ ] **1.2 Create HTTP request logging middleware**
  - **Dependencies**: 1.1
  - **Description**: Create `RequestLogger(logger)` middleware that replaces chi's `middleware.Logger`. Logs: method, path, status code, duration_ms, remote_addr, request_id (from chi's `middleware.RequestID`). Uses chi's `middleware.WrapResponseWriter` to capture status code.
  - **Creates**:
    - `internal/logging/middleware.go`
    - `internal/logging/middleware_test.go`
  - **Links**: N/A

- [ ] **1.3 Add LogConfig to config**
  - **Dependencies**: None
  - **Description**: Add `LogConfig` struct with 8 fields (Format, Level, TimeFormat, AddSource, TimeKey, LevelKey, MessageKey, SourceKey) to `internal/config/config.go`. Add `Log LogConfig` field to `Config`. Add env overrides for `LOG_FORMAT`, `LOG_LEVEL`, `LOG_TIME_FORMAT`, `LOG_ADD_SOURCE`, `LOG_TIME_KEY`, `LOG_LEVEL_KEY`, `LOG_MESSAGE_KEY`, `LOG_SOURCE_KEY`. Add validation (format: text/json, valid level, valid time format). Defaults: format=json, level=info, timeFormat=rfc3339nano.
  - **Creates**:
    - Modified `internal/config/config.go`
    - Modified `internal/config/config_test.go`
  - **Links**:
    - Reference: `~/Projects/personal/topologymetrics/uniproxy/internal/config/config.go` (LogConfig parsing)

- [ ] **1.4 Wire logging into application**
  - **Dependencies**: 1.1, 1.2, 1.3
  - **Description**: In `cmd/dephealth-ui/main.go`, replace hardcoded `slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})` with `logging.NewLogger(cfg.Log)`. Add bootstrap logger (text/stderr) for pre-config error logging. In `internal/server/server.go`, replace `middleware.Logger` with `logging.RequestLogger(s.logger)` in `setupMiddleware()`.
  - **Creates**:
    - Modified `cmd/dephealth-ui/main.go`
    - Modified `internal/server/server.go`
  - **Links**: N/A

- [ ] **1.5 Update Helm chart and example config**
  - **Dependencies**: 1.3
  - **Description**: Add `log:` section to `deploy/helm/dephealth-ui/templates/configmap.yml` and `deploy/helm/dephealth-ui/values.yaml`. Add log configuration section with comments to `config.example.yaml`.
  - **Creates**:
    - Modified `deploy/helm/dephealth-ui/templates/configmap.yml`
    - Modified `deploy/helm/dephealth-ui/values.yaml`
    - Modified `config.example.yaml`
  - **Links**: N/A

### Completion Criteria Phase 1

- [ ] All items completed (1.1–1.5)
- [ ] `go test ./internal/logging/... ./internal/config/... -v -race` passes
- [ ] Image `v0.15.0-1` built and deployed
- [ ] JSON logs with correct keys/levels verified in K8s logs
- [ ] HTTP request logging shows method, path, status, duration_ms

---

## Phase 2: Backend — New Status Metrics

**Dependencies**: None (can run in parallel with Phase 1)
**Status**: Pending

### Description

Query two new SDK v0.4.1 Prometheus metrics (`app_dependency_status`, `app_dependency_status_detail`), extend the Edge model with `status` and `detail` fields, and return them in API responses. Backward compatible with old SDK services.

### New Metrics Reference

- `app_dependency_status` (Gauge, enum pattern): labels = name, namespace, dependency, type, host, port, critical, **status**. 8 status values: ok, timeout, connection_error, dns_error, auth_error, tls_error, unhealthy, error. Exactly one series per endpoint has value = 1, rest = 0.
- `app_dependency_status_detail` (Gauge, info pattern): labels = name, namespace, dependency, type, host, port, critical, **detail**. Detail examples: http_503, grpc_not_serving, connection_refused. Always value = 1 when present.

### Items

- [ ] **2.1 Extend Edge model**
  - **Dependencies**: None
  - **Description**: Add two optional fields to `Edge` struct in `internal/topology/models.go`:
    ```go
    Status string `json:"status,omitempty"`
    Detail string `json:"detail,omitempty"`
    ```
    `omitempty` ensures backward compatibility — old SDK services without new metrics have empty fields omitted from JSON.
  - **Creates**:
    - Modified `internal/topology/models.go`
  - **Links**: N/A

- [ ] **2.2 Add PromQL queries and PrometheusClient methods**
  - **Dependencies**: None
  - **Description**: Add query constants to `internal/topology/prometheus.go`:
    ```go
    queryDependencyStatus       = `app_dependency_status%s == 1`
    queryDependencyStatusDetail = `app_dependency_status_detail%s == 1`
    ```
    Add two methods to `PrometheusClient` interface:
    ```go
    QueryDependencyStatus(ctx context.Context, opts QueryOptions) (map[EdgeKey]string, error)
    QueryDependencyStatusDetail(ctx context.Context, opts QueryOptions) (map[EdgeKey]string, error)
    ```
    Create helper `parseEdgeStringValues(results []promResult, label string) map[EdgeKey]string` that extracts a named label from promResult per EdgeKey (name, host, port).
  - **Creates**:
    - Modified `internal/topology/prometheus.go`
    - Modified `internal/topology/prometheus_test.go`
  - **Links**: N/A

- [ ] **2.3 Update graph builder**
  - **Dependencies**: 2.1, 2.2
  - **Description**: In `internal/topology/graph.go`, update `Build()` to call two new queries (non-fatal: log warn + empty map on error, add to queryErrors for partial response). Pass status/detail maps to `buildGraph()`. In edge construction loop, set:
    ```go
    if s, ok := depStatus[key]; ok { edge.Status = s }
    if d, ok := depStatusDetail[key]; ok { edge.Detail = d }
    ```
    Stale edges (early continue block) leave Status/Detail empty.
  - **Creates**:
    - Modified `internal/topology/graph.go`
  - **Links**: N/A

- [ ] **2.4 Update tests**
  - **Dependencies**: 2.2, 2.3
  - **Description**: Extend `mockPrometheusClient` in `graph_test.go` with `QueryDependencyStatus` and `QueryDependencyStatusDetail` methods. Add test cases: edges with status/detail (new SDK), edges without (old SDK), mixed, query failures with partial data. Add `TestQueryDependencyStatus` and `TestQueryDependencyStatusDetail` to `prometheus_test.go`.
  - **Creates**:
    - Modified `internal/topology/graph_test.go`
    - Modified `internal/topology/prometheus_test.go`
  - **Links**: N/A

### Completion Criteria Phase 2

- [ ] All items completed (2.1–2.4)
- [ ] `go test ./internal/topology/... -v -race` passes
- [ ] Image `v0.15.0-2` built and deployed
- [ ] `curl /api/v1/topology | jq '.edges[] | select(.status != null)'` returns edges with status/detail
- [ ] Edges from old SDK services have no status/detail fields (backward compat)

---

## Phase 3: Frontend — Full Integration

**Dependencies**: Phase 2
**Status**: Pending

### Description

Full visual integration of new status/detail data: edge color coding by status category, thick critical edges, status abbreviation labels on edges, sidebar enrichment, tooltip enhancement, new status filter dimension.

### Color Mapping

| Status | Color | Hex |
|--------|-------|-----|
| ok | Green | `#4caf50` |
| timeout | Orange | `#ff9800` |
| connection_error | Red | `#f44336` |
| error | Red | `#f44336` |
| dns_error | Purple | `#9c27b0` |
| auth_error | Yellow | `#ffeb3b` |
| tls_error | Dark Red | `#b71c1c` |
| unhealthy | Orange-Red | `#ff5722` |

### Status Abbreviations

| Status | Abbreviation |
|--------|-------------|
| timeout | TMO |
| connection_error | CONN |
| dns_error | DNS |
| auth_error | AUTH |
| tls_error | TLS |
| unhealthy | UNH |
| error | ERR |

### Items

- [ ] **3.1 Edge color coding + thickness**
  - **Dependencies**: None
  - **Description**: In `frontend/src/graph.js`, add `STATUS_COLORS` map. Update edge `line-color` and `target-arrow-color` to use `status` when available, fallback to existing `state`-based colors. Critical edge width: start with 8px (font height 12px), adjust visually. Non-critical: 1.5px. Include `status` and `detail` in edge data during `renderGraph()` (both full rebuild and smart-diff update paths).
  - **Creates**:
    - Modified `frontend/src/graph.js`
  - **Links**: N/A

- [ ] **3.2 Edge labels — status abbreviations**
  - **Dependencies**: 3.1
  - **Description**: In `frontend/src/graph.js`, add `STATUS_ABBREVIATIONS` map. Update edge `label` to show `"TMO 15.2ms"` for timeout edges, plain latency for ok/no-status edges.
  - **Creates**:
    - Modified `frontend/src/graph.js`
  - **Links**: N/A

- [ ] **3.3 Edge sidebar — status + detail**
  - **Dependencies**: None
  - **Description**: In `frontend/src/sidebar.js`, update `renderEdgeDetails()` to add color-coded status badge and detail text after the state row. Show only for non-ok statuses. Add `formatStatus(status)` helper that renders a colored badge.
  - **Creates**:
    - Modified `frontend/src/sidebar.js`
  - **Links**: N/A

- [ ] **3.4 Node sidebar — dependency status summary**
  - **Dependencies**: 3.3
  - **Description**: In `frontend/src/sidebar.js`, add `renderDependencyStatusSummary(node, cy)` for service nodes. Counts statuses of outgoing edges, renders as mini color-coded pills (e.g., "3 ok, 1 TMO, 1 DNS"). Show in node details section before connected edges list.
  - **Creates**:
    - Modified `frontend/src/sidebar.js`
  - **Links**: N/A

- [ ] **3.5 Tooltip — status on hover**
  - **Dependencies**: None
  - **Description**: In `frontend/src/tooltip.js`, add status and detail rows to edge hover tooltip for non-ok statuses.
  - **Creates**:
    - Modified `frontend/src/tooltip.js`
  - **Links**: N/A

- [ ] **3.6 Filter by error type**
  - **Dependencies**: 3.1
  - **Description**: In `frontend/src/filter.js`, add 4th filter dimension `status` to `activeFilters`. Add `STATUS_VALUES` array. Initialize Tom Select multiselect for status filtering in `initFilters()`. In `applyFilters()`, add new edge-level filtering pass: when status filter active, hide edges whose status doesn't match, then re-evaluate node visibility. In `frontend/index.html`, add `<select id="status-select" multiple>` in the filter panel.
  - **Creates**:
    - Modified `frontend/src/filter.js`
    - Modified `frontend/index.html`
  - **Links**: N/A

- [ ] **3.7 i18n + CSS + legend**
  - **Dependencies**: 3.1, 3.3
  - **Description**: Add ~15 new i18n keys to `frontend/src/locales/{en,ru}.js` (sidebar.edge.status, sidebar.edge.detail, tooltip.status, tooltip.detail, filter.allStatuses, legend entries). Add CSS styles for `.sidebar-status-badge`, `.sidebar-edge-status` to `frontend/src/style.css`. Update legend in `frontend/index.html` with status color entries.
  - **Creates**:
    - Modified `frontend/src/locales/en.js`
    - Modified `frontend/src/locales/ru.js`
    - Modified `frontend/src/style.css`
    - Modified `frontend/index.html`
  - **Links**: N/A

### Completion Criteria Phase 3

- [ ] All items completed (3.1–3.7)
- [ ] `npm --prefix frontend run build` succeeds
- [ ] Image `v0.15.0-3` built and deployed
- [ ] Edge colors match status categories visually
- [ ] Edge labels show abbreviations for non-ok statuses
- [ ] Critical edges rendered thick (adjust width if needed)
- [ ] Sidebar shows status + detail for edges
- [ ] Node sidebar shows dependency status summary
- [ ] Tooltip shows status on hover
- [ ] Status filter works (edges filtered, nodes update)
- [ ] Backward compatibility: old SDK edges display normally with state-based coloring

---

## Phase 4: Grafana Dashboards

**Dependencies**: Phase 2
**Status**: Pending

### Description

Update existing dashboards with status/detail panels. Create new "Connection Diagnostics" dashboard with full status analytics.

### Items

- [ ] **4.1 Update existing dashboards**
  - **Dependencies**: None
  - **Description**: In `deploy/helm/dephealth-monitoring/dashboards/`:
    - `link-status.json` — add Status Category stat panel (`app_dependency_status == 1`), Status Detail stat panel (`app_dependency_status_detail == 1`), Status Timeline (state-timeline)
    - `links-status.json` — add status/detail columns to overview table
    - `service-status.json` — add "Dependency Statuses" sub-panel with status distribution for selected service
  - **Creates**:
    - Modified `deploy/helm/dephealth-monitoring/dashboards/link-status.json`
    - Modified `deploy/helm/dephealth-monitoring/dashboards/links-status.json`
    - Modified `deploy/helm/dephealth-monitoring/dashboards/service-status.json`
  - **Links**: N/A

- [ ] **4.2 Create Connection Diagnostics dashboard**
  - **Dependencies**: None
  - **Description**: Create new dashboard with UID `dephealth-connection-diagnostics`. Panels:
    1. Status Distribution (pie chart): `count by (status) (app_dependency_status == 1)`
    2. Status Timeline (state-timeline): `app_dependency_status == 1` filtered by variables
    3. Detail Table: `app_dependency_status_detail == 1` — columns: name, dependency, host, port, detail
    4. Error Heatmap: non-ok status counts over time bucketed by status category
    5. Drill-down links to Link Status dashboard
    Variables: `$namespace`, `$service`, `$dependency`, `$status`
  - **Creates**:
    - `deploy/helm/dephealth-monitoring/dashboards/connection-diagnostics.json`
  - **Links**: N/A

- [ ] **4.3 Register dashboard in Helm**
  - **Dependencies**: 4.2
  - **Description**: In `deploy/helm/dephealth-monitoring/templates/grafana.yml`:
    - Add ConfigMap `grafana-dashboard-connection-diagnostics` for new dashboard
    - Add volume and volume mount in Grafana deployment spec
  - **Creates**:
    - Modified `deploy/helm/dephealth-monitoring/templates/grafana.yml`
  - **Links**: N/A

- [ ] **4.4 Add dashboard UID to config + frontend links**
  - **Dependencies**: 4.2
  - **Description**: Add `ConnectionDiagnostics string` to `DashboardsConfig` in `internal/config/config.go`. Add to `configDashboards` response in `internal/server/server.go`. Add dashboard UID to `deploy/helm/dephealth-ui/values.yaml`. Add Grafana link for Connection Diagnostics in `frontend/src/sidebar.js` edge rendering. Add i18n key `sidebar.grafana.connectionDiagnostics`.
  - **Creates**:
    - Modified `internal/config/config.go`
    - Modified `internal/server/server.go`
    - Modified `deploy/helm/dephealth-ui/values.yaml`
    - Modified `frontend/src/sidebar.js`
    - Modified `frontend/src/locales/en.js`
    - Modified `frontend/src/locales/ru.js`
  - **Links**: N/A

### Completion Criteria Phase 4

- [ ] All items completed (4.1–4.4)
- [ ] `make env-deploy` succeeds (monitoring stack updated)
- [ ] Image `v0.15.0-4` built and deployed
- [ ] All updated dashboards display status/detail panels in Grafana
- [ ] New Connection Diagnostics dashboard shows data
- [ ] Drill-down links work between dashboards
- [ ] Frontend sidebar shows Connection Diagnostics Grafana link

---

## Phase 5: Uniproxy + E2E Testing

**Dependencies**: Phase 2, Phase 3, Phase 4
**Status**: Pending

### Description

Verify/rebuild uniproxy test service with SDK v0.4.1 metrics, run full E2E verification, update documentation.

### Items

- [ ] **5.1 Verify uniproxy metrics**
  - **Dependencies**: None
  - **Description**: Check if uniproxy already exposes `app_dependency_status` and `app_dependency_status_detail` in VictoriaMetrics. The uniproxy project already has SDK v0.4.1 — verify the running instance has these metrics. If not, rebuild: `make uniproxy-build TAG=v0.4.1` and redeploy.
  - **Creates**:
    - Possibly rebuilt uniproxy image
  - **Links**: N/A

- [ ] **5.2 Full E2E verification**
  - **Dependencies**: 5.1
  - **Description**: Complete verification checklist:
    - [ ] API returns status/detail for edges from SDK v0.4.1 services
    - [ ] Edge colors match status categories in UI
    - [ ] Edge labels show abbreviations for non-ok statuses
    - [ ] Critical edges rendered thick
    - [ ] Edge sidebar shows status + detail
    - [ ] Node sidebar shows dependency status summary
    - [ ] Tooltip shows status on hover
    - [ ] Status filter works (edges filtered, nodes update)
    - [ ] Backward compatibility: old SDK edges display normally
    - [ ] HTTP request logging works with configurable format/level
    - [ ] All Grafana dashboards display correct data
    - [ ] Connection Diagnostics dashboard fully functional
    - [ ] Drill-down links work
  - **Creates**: N/A
  - **Links**: N/A

- [ ] **5.3 Update documentation**
  - **Dependencies**: 5.2
  - **Description**: Update docs:
    - `docs/API.md` + `.ru.md` — document new `status` and `detail` fields in Edge response
    - `docs/METRICS.md` + `.ru.md` — document new `app_dependency_status` and `app_dependency_status_detail` metrics
    - `docs/application-design.md` + `.ru.md` — update state model with status categories
    - `CHANGELOG.md` — add v0.15.0 section
    - `config.example.yaml` — add log section + connectionDiagnostics dashboard UID
  - **Creates**:
    - Modified documentation files
  - **Links**: N/A

### Completion Criteria Phase 5

- [ ] All items completed (5.1–5.3)
- [ ] E2E checklist fully passed
- [ ] Documentation updated (API, METRICS, CHANGELOG)
- [ ] Release image `v0.15.0` ready

---

## Image Tagging Plan

| Build | Tag | Content |
|-------|-----|---------|
| 1 | v0.15.0-1 | Phase 1 (logging) |
| 2 | v0.15.0-2 | Phase 2 (backend metrics) |
| 3 | v0.15.0-3 | Phase 3 (frontend) |
| 4 | v0.15.0-4 | Phase 4 (Grafana) |
| Release | v0.15.0 | All phases complete |

---

## Risk Mitigations

1. **Backward compatibility:** `omitempty` + non-fatal queries (log warn, continue with empty maps) — services on SDK v0.3.0 work seamlessly
2. **Critical edge width:** Start with 8px, adjust visually after testing on real graph
3. **Status filter complexity:** New edge-level filter pattern — integrate carefully with existing multi-pass `applyFilters()`
4. **Logger init order:** Bootstrap logger (text/stderr) for pre-config errors, then configurable logger after config load
5. **Grafana dashboard JSON:** Test changes in Grafana UI first, then export JSON for Helm

## Notes

- `STATUS_COLORS` and `STATUS_ABBREVIATIONS` maps exported from `graph.js` for reuse in sidebar and tooltip modules
- Edge `line-style` logic preserved: ok=solid, degraded=dashed, down=dotted, unknown=dashed (from state). Status only affects color, not line style
- uniproxy at `~/Projects/personal/topologymetrics/uniproxy` already has SDK v0.4.1 — only need to verify/rebuild deployed image
