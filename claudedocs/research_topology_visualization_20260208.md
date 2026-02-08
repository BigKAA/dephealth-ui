# Research Report: Microservices Topology Visualization Approaches

**Date:** 2026-02-08 (updated with architecture refinement)
**Confidence:** High (multiple sources cross-validated)

## Deployment Constraints

These constraints were identified during requirements discovery and drive the architecture:

- **Network isolation:** dephealth-ui is deployed **separately from the monitoring stack**. Prometheus/VictoriaMetrics and AlertManager are in a different network, inaccessible from user browsers.
- **Scale:** 100+ services with the dephealth SDK, thousands of dependency edges.
- **Authentication:** must be configurable — none (internal tool), Basic auth, or OIDC/SSO (Keycloak, LDAP).
- **Consequence:** A pure SPA with Nginx proxy to Prometheus is **not viable**. A server-side backend is required to query Prometheus/AlertManager and serve pre-built topology data to the frontend.

## Executive Summary

Four approaches were evaluated for building a microservices health and topology visualization tool that consumes `app_dependency_health` and `app_dependency_latency_seconds` metrics from the [topologymetrics](https://github.com/BigKAA/topologymetrics) project:

| Approach | Effort | Maintenance | Feature Fit | Recommendation |
|----------|--------|-------------|-------------|----------------|
| A. Grafana Node Graph panel (built-in) | 2-5 days | Zero | 70% | Quick start / PoC |
| B. Grafana custom plugin | 22-36 person-days + 2-5 days/year | High | 95% | Not recommended |
| C. **Go backend + JS frontend** | 15-22 person-days | Low | 100% | **Recommended** |
| D. Grafana datasource plugin + Node Graph API | 5-10 days | Medium | 80% | Hybrid alternative |

**Primary recommendation: Option C — combined application** with a Go backend (Prometheus/AlertManager queries, caching, pluggable auth) and a JS frontend (Cytoscape.js + dagre layout). Ships as a single Docker image with Helm chart. The Go backend is the sole access point to monitoring APIs — the browser never touches Prometheus/AlertManager directly.

---

## 1. Data Source Analysis: topologymetrics (dephealth SDK)

### What It Does

topologymetrics (dephealth) is a multi-language SDK (Go, Python, Java, C#) embedded directly in each microservice. Each service periodically health-checks its own dependencies and exports results as Prometheus metrics. The aggregation of per-service metrics across the fleet forms a directed dependency graph.

### Metrics Exported

**Two metric families:**

| Metric | Type | Values | Description |
|--------|------|--------|-------------|
| `app_dependency_health` | Gauge | `1` (healthy) / `0` (unhealthy) | Health status of a dependency |
| `app_dependency_latency_seconds` | Histogram | seconds | Latency of dependency health check |

Histogram buckets: `0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0`

### Labels (both metrics)

| Label | Required | Description | Example |
|-------|----------|-------------|---------|
| `dependency` | Yes | Logical dependency name | `postgres-main` |
| `type` | Yes | Connection type | `http`, `grpc`, `tcp`, `postgres`, `mysql`, `redis`, `amqp`, `kafka` |
| `host` | Yes | Endpoint address | `pg-master.db.svc.cluster.local` |
| `port` | Yes | Endpoint port | `5432` |
| `role` | No | Instance role | `primary`, `replica` |
| `shard` | No | Shard identifier | `shard-01` |
| `vhost` | No | AMQP virtual host | `/` |

### Graph Model

- **Nodes** = Prometheus `job` label (the scraping service name)
- **Edges** = combination of `{job → dependency, type, host, port}`
- Each unique `{job, dependency, host, port}` = one directed edge

### Key PromQL Queries for Topology

```promql
# All edges in topology
group by (job, dependency, type, host, port) (app_dependency_health)

# Current health of all dependencies
app_dependency_health

# P99 latency
histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket[5m]))

# Average latency
rate(app_dependency_latency_seconds_sum[5m]) / rate(app_dependency_latency_seconds_count[5m])

# Degraded: some endpoints up, some down
(count by (job, namespace, dependency, type) (app_dependency_health == 0) > 0)
and
(count by (job, namespace, dependency, type) (app_dependency_health == 1) > 0)
```

### Pre-built Alert Rules (from topologymetrics Helm chart)

| Alert | Condition | Severity |
|-------|-----------|----------|
| `DependencyDown` | All endpoints of a dependency = 0 for 1m | critical |
| `DependencyDegraded` | Mixed 0 and 1 values for same dependency for 2m | warning |
| `DependencyHighLatency` | P99 > 1s for 5m | warning |
| `DependencyFlapping` | >4 state changes in 15m | info |
| `DependencyAbsent` | No metrics exported at all for 5m | warning |

---

## 2. Approach A: Grafana Built-in Node Graph Panel

### Capabilities (Grafana 10.x–12.x)

- **Directed edges:** Yes (source/target fields), but arrow rendering can be inconsistent depending on data formatting
- **Edge labels:** Hover-only (`mainstat`/`secondarystat` displayed on hover, NOT persistently on canvas)
- **Color-coded nodes:** Yes, via `color` field (HTML string) or `arc__*` fields (segmented ring)
- **Click to navigate:** Yes, via Data Links to other dashboards
- **Layout algorithms:** Layered (default), Force, Grid
- **Node limit:** 1,500 nodes maximum

### Assessment

| Requirement | Met? | Notes |
|-------------|------|-------|
| Directed graph with nodes and edges | Yes | Arrow rendering can be buggy |
| Color-coded states (OK, DEGRADED, DOWN) | Yes | Via `color` or `arc__*` fields |
| **Latency labels on edges** | **Partial** | **Hover-only — not persistent** |
| Click-through to Grafana dashboards | Yes | Via Data Links |
| Real-time updates | Yes | Dashboard auto-refresh |

**Verdict:** 70% fit. The hover-only edge labels are a significant limitation for at-a-glance topology monitoring where latency visibility is critical.

**Effort:** 2-5 days (dashboard configuration + data source setup)
**Maintenance:** Zero (built-in panel)

---

## 3. Approach B: Custom Grafana Panel Plugin

### Development Stack

- `@grafana/create-plugin` — CLI scaffolding
- `@grafana/data`, `@grafana/ui`, `@grafana/runtime` — SDK
- `@grafana/plugin-e2e` — Playwright-based E2E testing
- Private signing via `@grafana/sign-plugin` (requires Grafana Cloud account)

### Effort Estimate

| Component | Days |
|-----------|------|
| Scaffolding + setup | 1-2 |
| Graph rendering engine (Cytoscape.js/D3) | 5-8 |
| Node rendering with states | 3-5 |
| Edge rendering with visible labels | 3-5 |
| Data transformation layer | 2-3 |
| Click interactions + data links | 2-3 |
| Options panel / configuration UI | 2-3 |
| Testing (unit + E2E) | 3-5 |
| Signing + CI/CD | 1-2 |
| **Total** | **22-36** |

### Maintenance Burden

- Every Grafana major version (yearly cycle) introduces breaking changes
- Angular-to-React migration destroyed many plugins (Grafana 9-11 era)
- **Grafana 13 (mid-2026): React 19 migration** — will affect all React-based plugins
- Estimated: 2-5 days/year routine + 1-2 weeks for major version transitions
- Real-world example: Novatec Service Dependency Graph had to be completely rewritten for React (v4.0.0)
- algenty/grafana-flowcharting (once very popular) was abandoned due to Angular deprecation

### Existing Relevant Plugins

| Plugin | Status | Notes |
|--------|--------|-------|
| Novatec Service Dependency Graph | Active (v4.2.0, Grafana 10.4+) | Closest fit; community-maintained; abandonment risk |
| Node Graph API datasource | Active (v1.0.0) | Feeds REST API data into built-in Node Graph |
| algenty/grafana-flowcharting | **Abandoned** | Incompatible with Grafana 11+ |

**Verdict:** High effort, high maintenance, Grafana version coupling. Only justified if full Grafana integration is a hard requirement.

---

## 4. Approach C: Go Backend + JS Frontend (Recommended)

### Why a Backend is Required

A pure SPA (browser → Nginx → Prometheus) does not work for this project because:

1. **Network isolation:** Prometheus/VictoriaMetrics and AlertManager are in a separate network, not accessible from user browsers.
2. **Security:** Exposing Prometheus API (even via proxy) allows arbitrary PromQL execution by any browser user.
3. **Performance:** With 100+ services, PromQL queries are heavy. A server-side cache (one query serves all users) is essential.
4. **Authentication:** Pluggable auth (none/basic/OIDC) requires server-side session management and middleware.
5. **Data transformation:** Computing OK/DEGRADED/DOWN states by correlating metrics with alerts is complex logic better handled on the server.

### Frontend Technology Comparison

| Library | Edge Labels | Directed Arrows | Performance (100+ nodes) | Layout | Learning Curve | Maintenance Status |
|---------|-------------|-----------------|--------------------------|--------|---------------|-------------------|
| **Cytoscape.js** | Native, persistent | Native | Excellent | 6 built-in + dagre, ELK, cola | Medium | Active (WebGL preview 2025) |
| D3.js (d3-force) | Manual implementation | Manual implementation | Excellent | Force only | High | Stable, mature |
| vis-network | Native | Native | Excellent | Built-in (hierarchical, force) | Low | **Maintenance mode** |
| React Flow | Custom edge components | Native | Excellent | None built-in (needs dagre/ELK) | Low (React devs) | Active |
| Sigma.js + Graphology | forceLabel: true | Edge types | Overkill (optimized for 100K+) | ForceAtlas2 | Medium | Active |

### Recommendation: Cytoscape.js + dagre

**Why Cytoscape.js:**
- Purpose-built for network/graph visualization (not general-purpose like D3)
- Native persistent edge labels: `{ selector: 'edge', style: { label: 'data(latency)' } }`
- CSS-like styling system separates data from presentation
- `cy.batch()` for efficient bulk real-time updates
- Rich layout ecosystem: dagre (hierarchical), CoSE (force-directed), ELK (advanced)
- Framework-agnostic: works with any frontend or vanilla JS
- Active development, WebGL renderer in preview (Jan 2025)
- ~500K weekly npm downloads, strong community

**Why dagre layout:**
- Microservice topology is typically DAG-like
- Fast, minimal configuration
- Clean hierarchical rendering with `rankDir: 'LR'` or `'TB'`
- Integrates via `cytoscape-dagre` extension

### Recommended Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| **Backend** | Go (net/http + chi) | Single binary; official Prometheus client library; minimal container image (~15-20MB); matches K8s ecosystem |
| **Frontend** | Vanilla JS with Vite | Compact SPA; Cytoscape works natively; minimal bundle; can migrate to React later |
| Graph visualization | Cytoscape.js + cytoscape-dagre | Best balance of features, performance, and simplicity for network topology |
| Layout engine | dagre | Optimal for DAG-like microservice topology |
| Build tool | Vite | Fast dev server, optimal builds, HMR |
| Containerization | Docker (multi-stage) + Helm chart | Single image: Go binary embeds SPA static files |

### Architecture

```
┌─────────────────────┐
│  Browser (JS SPA)   │  ← Cytoscape.js, receives pre-built graph JSON
│  Cytoscape.js       │  ← No PromQL, no direct Prometheus access
└────────┬────────────┘
         │ HTTPS (JSON REST API)
         ▼
┌─────────────────────────────────────┐
│  dephealth-ui (Go binary)           │  ← Single binary, single Docker image
│                                     │
│  ┌─ HTTP Server ──────────────────┐ │
│  │  GET /              → SPA      │ │  ← Serves embedded static files
│  │  GET /api/v1/topology → handler│ │  ← Pre-built topology graph
│  │  GET /api/v1/alerts   → handler│ │  ← Aggregated alert state
│  │  GET /api/v1/config   → handler│ │  ← Frontend configuration
│  └────────────────────────────────┘ │
│                                     │
│  ┌─ Topology Service ─────────────┐ │
│  │  Queries Prometheus/VM API     │ │  ← Server-side, not from browser
│  │  Queries AlertManager API v2   │ │
│  │  Builds graph (nodes + edges)  │ │  ← Computes OK/DEGRADED/DOWN
│  │  Caches result (15-60s TTL)    │ │  ← One query serves all users
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
│ (separate        │ │  (separate     │
│  network)        │ │   network)     │
└──────────────────┘ └────────────────┘
```

### Backend API Contract

The frontend receives **pre-built topology data** — no PromQL, no raw metrics:

```
GET /api/v1/topology

{
  "nodes": [
    {
      "id": "order-service",
      "label": "Order Service",
      "state": "ok",               // computed server-side
      "type": "service",
      "dependencyCount": 3,
      "grafanaUrl": "https://grafana.example.com/d/dephealth-service-status?var-job=order-service"
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
      "grafanaUrl": "https://grafana.example.com/d/dephealth-link-status?var-job=order-service&var-dep=postgres-main"
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

### Configuration

```yaml
# dephealth-ui.yaml
server:
  listen: ":8080"

datasources:
  prometheus:
    url: "http://victoriametrics.monitoring.svc:8428"
    # or external: "https://vm.example.com"
    # optional basic auth for Prometheus itself:
    # username: "reader"
    # password: "secret"
  alertmanager:
    url: "http://alertmanager.monitoring.svc:9093"

cache:
  ttl: 15s       # topology cache TTL

auth:
  type: "none"   # "none" | "basic" | "oidc"
  # basic:
  #   users:
  #     - username: admin
  #       passwordHash: "$2a$..."
  # oidc:
  #   issuer: "https://keycloak.example.com/realms/infra"
  #   clientId: "dephealth-ui"
  #   clientSecret: "..."
  #   redirectUrl: "https://dephealth.example.com/auth/callback"

grafana:
  baseUrl: "https://grafana.example.com"
  dashboards:
    serviceStatus: "dephealth-service-status"
    linkStatus: "dephealth-link-status"
```

### Frontend Data Flow

The frontend is a thin visualization layer — all data transformation happens on the backend:

```javascript
// Polling loop (every 15s)
setInterval(async () => {
  // Single API call — backend handles Prometheus + AlertManager + caching
  const response = await fetch('/api/v1/topology');
  const topology = await response.json();

  // Efficient batch update of the graph
  cy.batch(() => {
    syncNodes(topology.nodes);    // add/remove/update nodes + colors
    syncEdges(topology.edges);    // add/remove/update edges + latency labels
  });
}, topology.meta.ttl * 1000);
```

### Go Backend Responsibilities

| Responsibility | Details |
|----------------|---------|
| **Prometheus queries** | `app_dependency_health`, latency histogram, using official `prometheus/client_golang/api/v1` |
| **AlertManager queries** | `GET /api/v2/alerts` with filters, using standard HTTP client |
| **Graph building** | Constructs nodes from `job` labels, edges from `dependency/type/host/port` labels |
| **State computation** | Correlates health metrics + alerts → OK / DEGRADED / DOWN per node and edge |
| **Caching** | In-memory cache with configurable TTL (default 15s). One Prometheus query cycle serves all connected browsers |
| **Grafana URL generation** | Builds dashboard URLs with correct query parameters from config |
| **Auth middleware** | Pluggable: none (passthrough), Basic (bcrypt), OIDC (redirect flow + token validation) |
| **Static file serving** | Embeds SPA assets via Go `embed` package, serves at `/` |

### Effort Estimate

| Component | Days |
|-----------|------|
| Go project setup + HTTP server + config | 1-2 |
| Prometheus API client + topology builder | 3-4 |
| AlertManager API client + state computation | 2-3 |
| Caching layer | 1 |
| Auth middleware (none + basic + OIDC) | 2-3 |
| Frontend: Vite + Cytoscape.js + graph rendering | 3-4 |
| Frontend: click-through to Grafana | 1 |
| Docker multi-stage build + Helm chart | 1-2 |
| Testing (Go unit + frontend) | 1-2 |
| **Total** | **15-22** |

**Ongoing maintenance:** Minimal. No Grafana SDK dependency, no plugin signing. Go binary has no runtime dependencies. Update only when you want to.

---

## 5. Approach D: Hybrid — Grafana Datasource Plugin + Node Graph API

An intermediate approach using the existing [Node Graph API datasource plugin](https://grafana.com/grafana/plugins/hamedkarbasi93-nodegraphapi-datasource/) with a lightweight Go backend:

1. Build a Go service that queries Prometheus + AlertManager
2. Transform data into the Node Graph API format (JSON with `nodes[]` and `edges[]`)
3. Install the Node Graph API datasource plugin in Grafana
4. Use the built-in Node Graph panel to render

**Pros:** Lower effort than custom plugin; leverages existing Grafana dashboards
**Cons:** Still limited by Node Graph panel (hover-only edge labels); requires Go backend + plugin install; two components to maintain

**Effort:** 5-10 days
**Verdict:** Reasonable hybrid if Grafana integration is important but edge labels are not critical.

Also of note: [nodegraph-provider](https://github.com/opsdis/nodegraph-provider) is an existing Go backend that serves the Node Graph API format using RedisGraph. Could serve as implementation reference.

---

## 6. Comparative Decision Matrix

| Requirement | A: Node Graph | B: Custom Plugin | C: Go + JS | D: Hybrid |
|-------------|:---:|:---:|:---:|:---:|
| Directed graph with nodes/edges | Yes | Yes | **Yes** | Yes |
| Color-coded states (OK/DEGRADED/DOWN) | Yes | Yes | **Yes** | Yes |
| **Persistent latency labels on edges** | **No (hover)** | Yes | **Yes** | **No (hover)** |
| Click-through to Grafana dashboards | Yes | Yes | **Yes** (URL) | Yes |
| Real-time updates (15s) | Yes | Yes | **Yes** | Yes |
| **Works with isolated Prometheus** | No | No | **Yes** | Partial |
| **Pluggable auth (none/basic/OIDC)** | Grafana auth | Grafana auth | **Yes** | Grafana auth |
| **Server-side caching for 100+ services** | No | No | **Yes** | Yes (BFF) |
| No Grafana version dependency | N/A | **No** | **Yes** | Partial |
| Low maintenance burden | **Yes** | No | **Yes** | Medium |
| Full visual control | No | Yes | **Yes** | No |
| Single deployment artifact | N/A | No (plugin + Grafana) | **Yes** (one image) | No (BFF + plugin) |
| Development effort | **2-5 days** | 22-36 days | **15-22 days** | 5-10 days |

---

## 7. Conclusion and Recommendation

**Recommended approach: Option C — Combined Go backend + JS frontend**

Rationale:
1. **Only viable option** for the deployment constraint (Prometheus/AlertManager in a separate network, inaccessible from browsers)
2. **Meets all requirements** including persistent edge labels (the critical gap in Grafana's Node Graph)
3. **Server-side data processing** — caching, state computation, and Grafana URL generation happen on the backend; the frontend is a thin visualization layer
4. **Pluggable authentication** — configurable via YAML: none → basic → OIDC/SSO
5. **Scales to 100+ services** — one Prometheus query cycle (cached 15-60s) serves all connected browsers
6. **Low maintenance** — no Grafana SDK/version coupling; Go binary has no runtime dependencies
7. **Single artifact** — one Docker image (Go binary with embedded SPA), one Helm chart
8. **Click-through to Grafana** — URLs with correct dashboard variables generated server-side from config

**Suggested phased plan:**
1. Phase 1: Go project setup + Prometheus API client + topology builder + basic HTTP API
2. Phase 2: Frontend setup (Vite + Cytoscape.js) + graph rendering from API
3. Phase 3: AlertManager integration + state computation (OK/DEGRADED/DOWN)
4. Phase 4: Auth middleware (none + basic), caching, configuration
5. Phase 5: Docker multi-stage build + Helm chart + deployment to test cluster
6. Phase 6: OIDC auth, polish (dark mode, responsive layout, error handling)

---

## Sources

### topologymetrics / dephealth
- [BigKAA/topologymetrics GitHub](https://github.com/BigKAA/topologymetrics)

### Grafana
- [Grafana Node Graph Documentation](https://grafana.com/docs/grafana/latest/visualizations/panels-visualizations/visualizations/node-graph/)
- [Grafana Plugin Development Tools](https://grafana.com/developers/plugin-tools/)
- [Grafana Breaking Changes](https://grafana.com/docs/grafana/latest/breaking-changes/)
- [Grafana Plugin Signing](https://grafana.com/developers/plugin-tools/publish-a-plugin/sign-a-plugin)
- [Grafana Plugin CI Workflows](https://github.com/grafana/plugin-ci-workflows)
- [Novatec Service Dependency Graph Plugin](https://grafana.com/grafana/plugins/novatec-sdg-panel/)
- [Node Graph API Datasource Plugin](https://grafana.com/grafana/plugins/hamedkarbasi93-nodegraphapi-datasource/)
- [Node Graph Directional Arrow Issue](https://community.grafana.com/t/node-graph-isnt-showing-directional-arrow/159729)
- [Grafana Stricter Plugin Version Checks (2025)](https://grafana.com/whats-new/2025-05-05-enforcing-stricter-version-compatibility-checks-in-plugin-cli-install-commands/)
- [nodegraph-provider (Go backend)](https://github.com/opsdis/nodegraph-provider)

### JavaScript Libraries
- [Cytoscape.js](https://js.cytoscape.org)
- [Cytoscape.js WebGL Preview (Jan 2025)](https://blog.js.cytoscape.org/2025/01/13/webgl-preview/)
- [cytoscape-dagre Extension](https://github.com/cytoscape/cytoscape.js-dagre)
- [D3.js d3-force](https://d3js.org/d3-force)
- [vis-network](https://github.com/visjs/vis-network)
- [React Flow (xyflow)](https://reactflow.dev)
- [Sigma.js](https://www.sigmajs.org/)
- [Graphology](https://graphology.github.io/)
- [ELK.js](https://github.com/kieler/elkjs)

### Go Backend
- [Prometheus Go Client API (v1)](https://pkg.go.dev/github.com/prometheus/client_golang/api/prometheus/v1)
- [go-chi/chi Router](https://github.com/go-chi/chi)
- [Go embed Package](https://pkg.go.dev/embed)
- [k8spacket — Go backend + Node Graph API pattern](https://github.com/k8spacket/k8spacket)
- [nodegraph-provider — Go backend reference implementation](https://github.com/opsdis/nodegraph-provider)

### APIs
- [Prometheus HTTP API](https://prometheus.io/docs/prometheus/latest/querying/api/)
- [AlertManager API v2 OpenAPI Spec](https://github.com/prometheus/alertmanager/blob/main/api/v2/openapi.yaml)
