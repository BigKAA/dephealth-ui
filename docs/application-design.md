# dephealth-ui — Application Design

**Language:** English | [Русский](./application-design.ru.md)

---

## Purpose

dephealth-ui is a web application for visualizing microservice topologies and monitoring dependency health in real-time. It displays a directed service graph with color-coded states (OK, DEGRADED, DOWN, Unknown), latency values on edges, and links to Grafana dashboards.

## Data Sources

The application consumes data from two sources:

- **Prometheus / VictoriaMetrics** — metrics collected by the [topologymetrics](https://github.com/BigKAA/topologymetrics) project (dephealth SDK)
- **AlertManager** — active dependency alerts

### topologymetrics Metrics

| Metric | Type | Values | Description |
|--------|------|--------|-------------|
| `app_dependency_health` | Gauge | `1` (healthy) / `0` (unhealthy) | Dependency state |
| `app_dependency_latency_seconds` | Histogram | seconds | Dependency health check latency |

Histogram buckets: `0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0`

### Labels (same for both metrics)

| Label | Required | Description | Example |
|-------|:--------:|-------------|---------|
| `name` | yes | Application name (from SDK) | `uniproxy-01` |
| `dependency` | yes | Logical dependency name | `postgres-main` |
| `type` | yes | Connection type | `http`, `grpc`, `tcp`, `postgres`, `mysql`, `redis`, `amqp`, `kafka` |
| `host` | yes | Endpoint address | `pg-master.db.svc.cluster.local` |
| `port` | yes | Endpoint port | `5432` |
| `critical` | yes | Dependency criticality | `yes`, `no` |
| `role` | no | Instance role | `primary`, `replica` |
| `shard` | no | Shard identifier | `shard-01` |
| `vhost` | no | AMQP virtual host | `/` |

### Graph Model

- **Nodes** = Prometheus label `name` (application name from dephealth SDK)
- **Edges** = combination `{name → dependency, type, host, port, critical}`
- Each unique combination `{name, dependency, host, port}` = one directed edge
- The `critical` flag determines edge visual thickness on the graph

### Alert Rules (from topologymetrics Helm chart)

| Alert | Condition | Severity |
|-------|-----------|----------|
| `DependencyDown` | All dependency endpoints = 0 for 1 min | critical |
| `DependencyDegraded` | Mixed 0 and 1 values for a dependency for 2 min | warning |
| `DependencyHighLatency` | P99 > 1s for 5 min | warning |
| `DependencyFlapping` | >4 state changes in 15 min | info |
| `DependencyAbsent` | Metrics completely absent for 5 min | warning |

---

## Deployment Constraints

- **Network isolation:** the application is deployed **separately** from the monitoring stack. Prometheus/VictoriaMetrics and AlertManager are in a different network, inaccessible from user browsers.
- **Scale:** 100+ services with dephealth SDK, thousands of dependency edges.
- **Authentication:** configurable — no auth (internal tool), Basic auth, or OIDC/SSO (Keycloak, LDAP).

**Consequence:** a pure SPA with Nginx proxying to Prometheus is **not possible**. A server-side backend is required to query Prometheus/AlertManager and deliver ready-to-use graph data to the frontend.

---

## Architecture

Combined application: Go backend + JS frontend, shipped as a single Docker image.

```
┌─────────────────────┐
│  Browser (JS SPA)   │  ← Cytoscape.js, receives ready JSON graph
│  Cytoscape.js       │  ← No PromQL knowledge, no Prometheus access
└────────┬────────────┘
         │ HTTPS (JSON REST API)
         ▼
┌─────────────────────────────────────┐
│  dephealth-ui (Go binary)           │  ← Single binary, single Docker image
│                                     │
│  ┌─ HTTP Server ──────────────────┐ │
│  │  GET /              → SPA      │ │  ← Serves embedded static files
│  │  GET /api/v1/topology → handler│ │  ← Ready topology graph
│  │  GET /api/v1/alerts   → handler│ │  ← Aggregated alerts
│  │  GET /api/v1/config   → handler│ │  ← Frontend configuration
│  │  GET /api/v1/export/* → handler│ │  ← Graph export (JSON/CSV/DOT/PNG/SVG)
│  └────────────────────────────────┘ │
│                                     │
│  ┌─ Topology Service ─────────────┐ │
│  │  Prometheus/VM API queries     │ │  ← Server-side
│  │  AlertManager API v2 queries   │ │
│  │  Graph construction            │ │  ← OK/DEGRADED/DOWN computation
│  │  Caching (15-60s TTL)         │ │  ← One query cycle serves all users
│  └────────────────────────────────┘ │
│                                     │
│  ┌─ Auth Module (pluggable) ──────┐ │
│  │  type: "none"  → open access   │ │  ← Configured via YAML/env
│  │  type: "basic" → user/password │ │
│  │  type: "oidc"  → SSO/Keycloak │ │
│  └────────────────────────────────┘ │
└──────────┬──────────────┬───────────┘
           │              │
           ▼              ▼
┌──────────────────┐ ┌────────────────┐
│ VictoriaMetrics  │ │  AlertManager  │
│ (separate net)   │ │ (separate net) │
└──────────────────┘ └────────────────┘
```

---

## Tech Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| **Backend** | Go (`net/http` + `chi`) | Single binary; official Prometheus client library; minimal Docker image (~15-20MB); fits K8s ecosystem |
| **Frontend** | Vanilla JS + Vite | Compact SPA; Cytoscape.js works natively; minimal bundle; can migrate to React if needed |
| **Graph visualization** | Cytoscape.js + dagre + fcose | Native persistent edge labels; CSS-like styles; `cy.batch()` for efficient updates; rich layout ecosystem |
| **Layout** | dagre (flat) / fcose (grouped) | dagre — optimal for DAG-like topology in flat mode; fcose — force-directed layout for dimension grouping (namespace or group) with compound nodes |
| **Frontend build** | Vite | Fast dev server, optimal builds, HMR |
| **Containerization** | Docker (multi-stage) + Helm chart | Single image: Go binary with embedded SPA static files |

---

## Backend Responsibilities

| Responsibility | Details |
|----------------|---------|
| **Prometheus queries** | `app_dependency_health`, latency histogram via `prometheus/client_golang/api/v1` |
| **AlertManager queries** | `GET /api/v2/alerts` with filters, standard HTTP client |
| **Graph construction** | Nodes from label `name`, edges from labels `dependency/type/host/port/critical` |
| **State computation** | Metrics + alerts correlation → OK / DEGRADED / DOWN for each node and edge |
| **Caching** | In-memory cache with configurable TTL (default 15s). One query cycle to Prometheus serves all connected users |
| **Grafana URL generation** | Dashboard URLs with correct query parameters from configuration |
| **Auth middleware** | Pluggable: none (passthrough), Basic (bcrypt), OIDC (redirect flow + token validation) |
| **Static file serving** | SPA assets embedded via Go `embed` package, served at `/` |
| **Graph export** | Multi-format export (JSON, CSV, DOT, PNG, SVG) via `internal/export` package; Graphviz integration for image rendering |

---

## State Model

dephealth-ui uses a 4-state model for both nodes and edges:

| State | Color | Description |
|-------|-------|-------------|
| **ok** | Green (`#4caf50`) | All dependencies healthy |
| **degraded** | Yellow (`#ff9800`) | Some dependencies unhealthy, service still alive |
| **down** | Red (`#f44336`) | Service unavailable (all edges stale or truly down) |
| **unknown** | Gray (`#9e9e9e`) | No data available (no edges, or all edges stale) |

### Service Node State (Backend)

The backend computes service node state in `calcServiceNodeState()` based on outgoing edge health:

- **No edges** → `unknown`
- **Any edge with health=0** → `degraded`
- **All edges healthy (health=1)** → `ok`

> **Important:** `calcServiceNodeState` never returns `"down"`. The backend only assigns `ok`, `degraded`, or `unknown` to service nodes. The `down` state for service nodes is set only when **all** outgoing edges are stale (metrics disappeared), handled by the stale detection logic.

### Dependency Node State (Backend)

Dependency nodes derive state from incoming edges:

- **All incoming edges stale** → `down` (with `stale=true`)
- **Mixed stale/live** → state from non-stale edges only
- **health=1** → `ok`
- **health=0** → `down`

### Edge State

| Condition | State |
|-----------|-------|
| health=1 | `ok` |
| health=0 | `down` |
| Stale (metrics disappeared) | `unknown` |

### Frontend State Extensions

The frontend extends the state model with cascade warnings (see [Cascade Warnings](#cascade-warnings)). Nodes receiving cascade propagation show a `⚠ N` badge and appear in the virtual `warning` filter state.

---

## REST API

### `GET /api/v1/topology`

Returns the full topology graph with pre-computed states:

```json
{
  "nodes": [
    {
      "id": "order-service",
      "label": "Order Service",
      "state": "ok",
      "type": "service",
      "dependencyCount": 3,
      "grafanaUrl": "https://grafana.example.com/d/dephealth-service-status?var-service=order-service"
    },
    {
      "id": "postgres-main",
      "label": "postgres-main",
      "state": "degraded",
      "type": "postgres"
    }
  ],
  "edges": [
    {
      "source": "order-service",
      "target": "postgres-main",
      "latency": "5.2ms",
      "latencyRaw": 0.0052,
      "health": 1,
      "state": "ok",
      "critical": true,
      "grafanaUrl": "https://grafana.example.com/d/dephealth-link-status?var-dependency=postgres-main&var-host=pg-host&var-port=5432"
    }
  ],
  "alerts": [
    {
      "service": "postgres-main",
      "alertname": "DependencyDegraded",
      "severity": "warning",
      "since": "2026-02-08T08:30:00Z"
    }
  ],
  "meta": {
    "cachedAt": "2026-02-08T09:15:30Z",
    "ttl": 15,
    "nodeCount": 42,
    "edgeCount": 187
  }
}
```

### `GET /api/v1/config`

Returns configuration needed by the frontend (Grafana base URL, dashboard UIDs, display settings).

```json
{
  "grafana": {
    "baseUrl": "https://grafana.example.com",
    "dashboards": {
      "serviceStatus": "dephealth-service-status",
      "linkStatus": "dephealth-link-status",
      "serviceList": "dephealth-service-list",
      "servicesStatus": "dephealth-services-status",
      "linksStatus": "dephealth-links-status"
    }
  },
  "cache": { "ttl": 15 },
  "auth": { "type": "oidc" },
  "alerts": {
    "severityLevels": [
      {"value": "critical", "color": "#f44336"},
      {"value": "warning", "color": "#ff9800"},
      {"value": "info", "color": "#2196f3"}
    ]
  }
}
```

**Dashboards:**

| UID | Purpose | Query parameters |
|-----|---------|------------------|
| `serviceStatus` | Single service status | `?var-service=<name>` |
| `linkStatus` | Single dependency status | `?var-dependency=<dep>&var-host=<host>&var-port=<port>` |
| `serviceList` | All services list | — |
| `servicesStatus` | All services overview | — |
| `linksStatus` | All links overview | — |

---

## Application Configuration

```yaml
# dephealth-ui.yaml
server:
  listen: ":8080"

datasources:
  prometheus:
    url: "http://victoriametrics.monitoring.svc:8428"
    # username: "reader"
    # password: "secret"
  alertmanager:
    url: "http://alertmanager.monitoring.svc:9093"

cache:
  ttl: 15s

auth:
  type: "none"   # "none" | "basic" | "oidc"

grafana:
  baseUrl: "https://grafana.example.com"
  dashboards:
    serviceStatus: "dephealth-service-status"
    linkStatus: "dephealth-link-status"
    serviceList: "dephealth-service-list"
    servicesStatus: "dephealth-services-status"
    linksStatus: "dephealth-links-status"

topology:
  lookback: "0"  # "0" = disabled, "1h", "6h", "24h"
```

---

## Frontend Behavior

The frontend is a thin visualization layer. All data transformation happens on the backend.

### Main Loop

1. Frontend requests `GET /api/v1/topology` at the interval specified in `meta.ttl`
2. Receives ready JSON with nodes, edges, alerts, and meta information
3. Updates the Cytoscape.js graph via `cy.batch()` (efficient batch update)

### Visualization

- **Nodes:** color depends on `state` — green (OK), yellow (DEGRADED), red (DOWN), gray (Unknown/stale); dynamic size based on label length; colored namespace stripe
- **Edges:** directed arrows with persistent latency labels; edge color by `state`; edge thickness by `critical` (critical = thicker)
- **Stale nodes:** gray background (`#9e9e9e`), dashed border, hidden latency; tooltip "Metrics disappeared"
- **Click on node/edge:** opens sidebar with details (state, namespace, instances, edges, alerts) and Grafana dashboard links
- **Context menu (right-click):** Open in Grafana, Copy Grafana URL, Show Details
- **Layout:** dagre (flat mode, LR/TB) or fcose (dimension grouping mode)

![Context menu on a service node](./images/context-menu-grafana.png)

### Visual Grouping (Dimensions)

Grouping visually combines services into Cytoscape.js compound nodes by a selected **dimension**:
- **Namespace** — Kubernetes namespace (default, available for all services)
- **Group** — logical group label from SDK v0.5.0+ (`group` label on metrics)

A toolbar toggle button (**NS** / **GRP**) switches the active dimension. The choice is persisted in `localStorage` and reflected in the URL (`?dimension=group` or `?dimension=namespace`).

**Group dimension details:**
- Service nodes display a colored stripe and `gr: <group>` label when in group mode
- Dependency nodes (Redis, PostgreSQL, etc.) do not have a `group` label — they show no stripe in group mode
- The filter dropdown switches between "Namespace" and "Group" values depending on the active dimension
- The namespace/group legend panel updates to show the active dimension's values and colors
- If no nodes in the topology have a `group` field, the toggle is hidden and namespace mode is used automatically

**Modes:**
- **Flat mode (dagre):** all nodes displayed at the same level, dagre layout
- **Grouped mode (fcose):** nodes grouped in dimension containers, fcose layout

**Collapse/Expand:**
- Double-click on a group or "Expand" button in sidebar → collapse/expand
- Collapsed group shows: worst child state, service count, total alerts
- Edges between collapsed groups are automatically aggregated (showing count `×N`)
- Collapse/expand state is persisted in `localStorage`
- During data refresh (auto-refresh) — collapsed groups remain collapsed

**Click-to-expand navigation:**
- In collapsed group sidebar — clickable service list with colored state indicators
- Click on a service → group expands → camera centers on selected service → sidebar shows service details
- In edge sidebar — click on a node from a collapsed group also expands and navigates to the original service

![Main view with collapsed namespaces](./images/dephealth-main-view.png)

![Collapsed namespace sidebar with clickable services](./images/sidebar-collapsed-namespace.png)

### Sidebar

Three types of sidebars:

**1. Node Sidebar** — on clicking a service node:
- Basic info (state, type, namespace, group)
- Active alerts (with severity)
- Instance list (pod name, IP:port) — for service nodes
- Connected edges (incoming/outgoing with latency and navigation)
- "Open in Grafana" button (opens serviceStatus dashboard)
- **Grafana Dashboards** section — links to all dashboards with context-aware query parameters

![Node sidebar with alerts and Grafana links](./images/sidebar-grafana-section.png)

![Stale/unknown node sidebar](./images/sidebar-stale-node.png)

**2. Edge Sidebar** — on clicking an edge:
- State, type, latency, criticality
- Active alerts for this link
- Connected nodes (source/target) with clickable navigation
- "Open in Grafana" button (opens linkStatus dashboard)
- Grafana Dashboards section

![Edge sidebar with alerts and connected nodes](./images/sidebar-edge-details.png)

**3. Collapsed Namespace Sidebar** — on clicking a collapsed namespace:
- Worst state, service count, total alerts
- Clickable service list with colored state dots and "Go to node →" arrow
- "Expand namespace" button

### Internationalization (i18n)

The frontend supports EN and RU. Language toggle button in the toolbar. All UI elements, filters, legend, status bar, sidebar, and context menu are localized. Language is saved in `localStorage`.

| EN | RU |
|----|----|
| ![UI in English](./images/dephealth-main-view.png) | ![UI in Russian](./images/dephealth-russian-ui.png) |

### Cascade Warnings

Cascade warnings visualize failure propagation through critical dependencies. When a service goes down, all upstream services that critically depend on it (directly or transitively) receive cascade warning indicators showing the root cause.

**Key principle:** Only edges with `critical=true` propagate cascade warnings. Non-critical dependency failures do not trigger cascade propagation.

#### Algorithm

Cascade computation runs entirely on the frontend (`cascade.js`) after each data refresh:

**Phase 1 — Find root causes** (`findRealRootCauses`):
For each `down` service node, trace downstream through critical edges to find the actual unavailable dependency (the terminal root cause). Example: if `A(down) → B(critical) → C(unknown)`, the root cause is `C`, not `A`.

**Phase 2 — BFS upstream** (`computeCascadeWarnings`):
From each `down` service node, BFS upstream through incoming critical edges. Each upstream node (that is not itself `down`) receives cascade warning data referencing the real root cause(s).

```
fetchTopology() → renderGraph() → computeCascadeWarnings(cy) → updateBadges()
```

#### Node Data Properties

| Property | Type | Description |
|----------|------|-------------|
| `cascadeCount` | number | Count of distinct root-cause sources affecting this node |
| `cascadeSources` | string[] | Array of root-cause node IDs |
| `inCascadeChain` | boolean | `true` for nodes in the failure chain (down + root causes) — used by filter system |

#### Visual Representation

- **Cascade badge:** `⚠ N` pill-shaped badge on the top-left corner of affected nodes (N = number of root cause sources)
- **Tooltip:** shows "Cascade warning: ↳ service-name (state)" for each root cause source
- **Down nodes** do not show cascade badges (they are the root cause, not a warning recipient)

![Topology with cascade warning badges on upstream nodes](./images/cascade-warnings-main.png)

![Tooltip showing cascade warning root cause](./images/cascade-warning-tooltip.png)

#### State Filters

The state filter bar includes the virtual `warning` state alongside backend states:

![State filter chips: ok, degraded, down, unknown, warning](./images/state-filters.png)

#### Filter Integration

The filter system includes a virtual `warning` state (not a backend state):

- A node matches `warning` if `cascadeCount > 0` and `state !== 'down'`
- Nodes with `inCascadeChain=true` also match the `warning` filter (shows the full failure chain)

**Pass 1.5 — Degraded/down chain visibility:**
When `degraded` or `down` state filter is active, the filter system also reveals downstream non-ok dependencies so the user can see WHY a node is degraded or down.

---

## History Mode

History mode enables time-travel through the topology graph, allowing users to view the dependency state at any point in the past.

### Architecture

```
Browser                          Go Backend                    VictoriaMetrics
  │                                  │                              │
  │  /api/v1/topology?time=T         │                              │
  ├─────────────────────────────────►│  query(promql, at=T)         │
  │                                  ├─────────────────────────────►│
  │                                  │  /api/v1/query?time=T        │
  │  {meta: {isHistory:true, time:T}}│◄─────────────────────────────┤
  │◄─────────────────────────────────┤                              │
  │                                  │                              │
  │  /api/v1/timeline/events         │  query_range(start,end,step) │
  │  ?start=S&end=E                  │                              │
  ├─────────────────────────────────►├─────────────────────────────►│
  │  [{timestamp,service,kind}]      │◄─────────────────────────────┤
  │◄─────────────────────────────────┤  detect transitions          │
```

**Backend:**
- All Prometheus queries accept an optional `time` parameter. When set, the Prometheus `/api/v1/query?time=<unix_ts>` parameter is used instead of the current time
- Historical alerts are reconstructed from the `ALERTS{alertstate="firing"}` metric at the requested timestamp (AlertManager is not queried for historical data)
- Historical requests bypass the in-memory cache entirely (no Get, no Set)
- The `lookback` window is applied relative to `opts.Time` for stale node detection
- The `/api/v1/timeline/events` endpoint uses `query_range` to detect `app_dependency_status` transitions over a time window, with auto-calculated step size

**Frontend:**
- Timeline panel: bottom panel with time range presets (1h–90d), custom datetime inputs, and a range slider
- Event markers: colored markers on the slider showing state transitions (red=degradation, green=recovery, orange=change)
- URL synchronization: `?time=`, `?from=`, `?to=` query parameters are maintained via `history.replaceState()` for shareable links
- Grafana links: in history mode, all Grafana dashboard URLs include `&from=<ts-1h>&to=<ts+1h>` (Unix ms) to navigate to the relevant time window
- Auto-refresh is paused in history mode and resumed when returning to live mode

### User Flow

1. Click the History button (clock icon) in the toolbar
2. Select a time range via preset buttons or custom datetime inputs
3. Slider appears with event markers; drag to select a point in time
4. Graph updates on slider release showing the historical topology state
5. Click an event marker to snap to that specific transition
6. Copy the URL to share the historical view with colleagues
7. Click "Live" to return to real-time mode

---

## PromQL Queries (executed on the backend)

```promql
# All topology edges (instant)
group by (name, namespace, group, dependency, type, host, port, critical) (app_dependency_health)

# All edges within lookback window (for stale node retention)
group by (name, namespace, group, dependency, type, host, port, critical) (last_over_time(app_dependency_health[LOOKBACK]))

# Current state of all dependencies
app_dependency_health

# Average latency
rate(app_dependency_latency_seconds_sum[5m]) / rate(app_dependency_latency_seconds_count[5m])

# P99 latency
histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket[5m]))

# Degraded: some endpoints up, some down
(count by (name, namespace, dependency, type) (app_dependency_health == 0) > 0)
and
(count by (name, namespace, dependency, type) (app_dependency_health == 1) > 0)
```

> **Note:** The `group` label is optional. When present (SDK v0.5.0+), it enables the group dimension toggle in the UI. When absent, the system falls back to namespace-only grouping.

### Stale Node Retention (lookback window)

When a service stops sending metrics (crash, scale-down, network issues), its time series become "stale" in Prometheus after ~5 minutes and disappear from instant queries. By default, this causes the node to vanish from the topology graph.

The **lookback window** feature (`topology.lookback`) retains disappeared nodes on the graph with `state="unknown"` for a configurable duration.

**How it works:**

1. **Topology query** uses `last_over_time(metric[LOOKBACK])` — returns ALL edges seen in the lookback window (current + stale)
2. **Health query** uses an instant query — returns ONLY current edges (live time series)
3. Edges present in topology but NOT in health → marked as **stale** (`state="unknown"`, `Stale=true`)
4. Nodes where ALL edges are stale → `state="unknown"`; mixed nodes use non-stale edges for state calculation

**Frontend visualization:**
- Stale nodes: gray background (`#9e9e9e`), dashed border
- Stale edges: gray dashed lines, latency hidden
- Tooltip shows "Metrics disappeared" / "Метрики пропали"

**Configuration:** `topology.lookback` (default: `0` = disabled). Recommended values: `1h`, `6h`, `24h`. Minimum: `1m`. Env: `DEPHEALTH_TOPOLOGY_LOOKBACK`.

Compatible with both Prometheus and VictoriaMetrics (`last_over_time()` is supported by both).

---

## Graph Export

The export feature allows users to download the topology graph in multiple formats for external analysis, documentation, or sharing.

### Supported Formats

| Format | Type | Description |
|--------|------|-------------|
| **JSON** | Data | Structured export with nodes, edges, and metadata (version, timestamp, scope, filters) |
| **CSV** | Data | ZIP archive containing `nodes.csv` + `edges.csv` with UTF-8 BOM |
| **DOT** | Data | Graphviz DOT language with namespace/group subgraph clusters and status colors |
| **PNG** | Image | Graphviz-rendered raster image with configurable DPI (scale 1–4) |
| **SVG** | Image | Graphviz-rendered vector image |

### Architecture

Export uses a dual-path approach:

- **"Current view"** (frontend): `cy.png()` and `cy.svg()` — captures the exact Cytoscape.js canvas as the user sees it, preserving layout, zoom, and collapsed groups
- **"Full graph"** (backend): `GET /api/v1/export/{format}` — generates a complete, server-side representation of the topology via the `internal/export` package and Graphviz

```
┌────────────────────────────────────────┐
│  Export Modal (frontend)               │
│                                        │
│  Format: [PNG] [SVG] [JSON] [CSV] [DOT]│
│  Scope:  ○ Current view  ○ Full graph  │
│                                        │
│  Current view + PNG/SVG                │
│    → cy.png({full:true, scale:2})      │
│    → cy.svg({full:true})               │
│                                        │
│  Full graph + any format               │
│    → fetch /api/v1/export/{format}     │
│    → Blob → download                   │
└──────────────────┬─────────────────────┘
                   │ (backend formats)
                   ▼
┌────────────────────────────────────────┐
│  Export Handler (Go backend)           │
│                                        │
│  TopologyResponse                      │
│    → ConvertTopology() → ExportData    │
│    → ExportJSON / ExportCSV / ExportDOT│
│    → RenderDOT (png/svg via Graphviz)  │
└──────────────────┬─────────────────────┘
                   │ (PNG/SVG only)
                   ▼
┌────────────────────────────────────────┐
│  Graphviz CLI (dot -Tpng/-Tsvg)       │
│  Installed in Docker image (Alpine)    │
└────────────────────────────────────────┘
```

### Backend Export Package (`internal/export/`)

| File | Purpose |
|------|---------|
| `model.go` | `ExportData`, `ExportNode`, `ExportEdge` structs; `ConvertTopology()` converter |
| `json.go` | `ExportJSON()` — indented JSON serialization |
| `csv.go` | `ExportCSV()` — ZIP archive with `nodes.csv` + `edges.csv` |
| `dot.go` | `ExportDOT()` — Graphviz DOT with clusters, colors, shapes |
| `render.go` | `RenderDOT()` — invokes `dot` CLI with 10s timeout; `GraphvizAvailable()` check |

### Graphviz Integration

The Docker image includes the Alpine `graphviz` package (~55–65 MB) for server-side rendering. The `dot` layout engine is used for all graph rendering. If Graphviz is not installed, PNG/SVG exports return HTTP 503; other formats (JSON, CSV, DOT) work without Graphviz.

---

## Deployment

### Docker

Multi-stage build:
1. **Stage 1 (frontend):** Node.js + Vite → builds SPA into `dist/`
2. **Stage 2 (backend):** Go → compiles binary with embedded static files from Stage 1
3. **Stage 3 (runtime):** Alpine-based image with Graphviz for graph export rendering

Result: Docker image ~80 MB (Graphviz adds ~55–65 MB to the base image).

### Helm Chart

- Deployment with one container
- ConfigMap for `dephealth-ui.yaml`
- Secret for auth credentials (basic passwords, OIDC client secret)
- Service (ClusterIP or LoadBalancer)
- HTTPRoute (Gateway API) for external access
- Optional Certificate (cert-manager) for TLS

### Environment Variable Override

All YAML parameters can be overridden via environment variables:
- `DEPHEALTH_SERVER_LISTEN`
- `DEPHEALTH_DATASOURCES_PROMETHEUS_URL`
- `DEPHEALTH_DATASOURCES_ALERTMANAGER_URL`
- `DEPHEALTH_CACHE_TTL`
- `DEPHEALTH_AUTH_TYPE`
- `DEPHEALTH_GRAFANA_BASEURL`
- `DEPHEALTH_TOPOLOGY_LOOKBACK`

---

## See Also

- [REST API Reference](./API.md) — All endpoints and response formats
- [Metrics Specification](./METRICS.md) — Required metrics and integration guide
- [Deployment Guide](../deploy/helm/dephealth-ui/README.md) — Kubernetes & Helm
