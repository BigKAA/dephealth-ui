# dephealth-ui: Development Plan

## Overview

Full development plan for dephealth-ui — a web application for real-time
microservice topology and dependency health visualization.

**Stack:** Go (net/http + chi) backend + Vanilla JS (Vite + Cytoscape.js + dagre) frontend
**Deployment:** Kubernetes (Helm), multi-arch Docker images (amd64 + arm64)
**Test environment:** topologymetrics Helm charts (dephealth-infra, dephealth-services, dephealth-monitoring)

---

## Phase 0: Project Setup & Test Environment

**Objective:** Initialize project structure, build tooling, and deploy the test
environment in Kubernetes so that real Prometheus metrics and alerts are
available for development.

**Estimated effort:** 2-3 days

### 0.1 Go project initialization

- [ ] `go mod init github.com/BigKAA/dephealth-ui`
- [ ] Create directory structure:

```text
cmd/dephealth-ui/main.go        — entry point (placeholder)
internal/config/config.go       — configuration types
internal/server/server.go       — HTTP server stub
frontend/                       — empty dir for Phase 2
deploy/helm/dephealth-ui/       — empty dir for Phase 4
```

- [ ] Add `.gitignore` (Go + Node.js + IDE artifacts)
- [ ] Add `config.example.yaml` with documented defaults

### 0.2 Makefile

- [ ] Create `Makefile` with targets:

| Target | Description |
|---|---|
| `build` | `go build` local binary |
| `lint` | `golangci-lint run` + `markdownlint` |
| `test` | `go test ./...` |
| `frontend-build` | `npm --prefix frontend ci && npm --prefix frontend run build` |
| `docker-build` | Multi-arch build via `docker buildx` (linux/amd64, linux/arm64) |
| `docker-push` | Tag + push to `$(REGISTRY)` |
| `helm-deploy` | `helm upgrade --install` |
| `helm-undeploy` | `helm uninstall` |
| `dev` | `docker-build` → `docker-push` → `helm-deploy` |

- [ ] Variables at the top: `REGISTRY`, `IMAGE_NAME`, `TAG`, `PLATFORMS`, `NAMESPACE`
- [ ] Default `PLATFORMS = linux/amd64,linux/arm64`

### 0.3 Dockerfile (multi-stage, multi-arch)

- [ ] Create `Dockerfile`:

```dockerfile
# Stage 1 — frontend build
ARG REGISTRY=docker.io
FROM ${REGISTRY}/node:22-alpine AS frontend
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2 — Go build
FROM ${REGISTRY}/golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=frontend /frontend/dist internal/server/static
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /dephealth-ui ./cmd/dephealth-ui

# Stage 3 — runtime
FROM ${REGISTRY}/alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /dephealth-ui /dephealth-ui
EXPOSE 8080
ENTRYPOINT ["/dephealth-ui"]
```

- [ ] Verify build: `docker buildx build --platform linux/amd64,linux/arm64 -t test .`

### 0.4 Deploy test environment in Kubernetes

Deploy the topologymetrics stack that generates real `app_dependency_health`
and `app_dependency_latency_seconds` metrics.

**Source charts:** `~/Projects/personal/topologymetrics/topologymetrics/deploy/helm/`

- [ ] Build and push stub images (http-stub, grpc-stub) — multi-arch

```bash
# From topologymetrics project root
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg REGISTRY=harbor.kryukov.lan/docker \
  -t harbor.kryukov.lan/library/dephealth-http-stub:latest \
  -f conformance/stubs/http-stub/Dockerfile \
  --push conformance/stubs/http-stub/

docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg REGISTRY=harbor.kryukov.lan/docker \
  -t harbor.kryukov.lan/library/dephealth-grpc-stub:latest \
  -f conformance/stubs/grpc-stub/Dockerfile \
  --push conformance/stubs/grpc-stub/
```

- [ ] Build and push test service images (go, python, java, csharp) — multi-arch

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg REGISTRY=harbor.kryukov.lan/docker \
  -t harbor.kryukov.lan/library/dephealth-test-go:latest \
  -f test-services/go-service/Dockerfile --push .

# Similarly for python, java, csharp services
```

- [ ] Deploy infrastructure:

```bash
helm upgrade --install dephealth-infra \
  ~/Projects/personal/topologymetrics/topologymetrics/deploy/helm/dephealth-infra/ \
  -f ~/Projects/personal/topologymetrics/topologymetrics/deploy/helm/dephealth-infra/values-homelab.yaml
```

- [ ] Deploy test services:

```bash
helm upgrade --install dephealth-services \
  ~/Projects/personal/topologymetrics/topologymetrics/deploy/helm/dephealth-services/ \
  -f ~/Projects/personal/topologymetrics/topologymetrics/deploy/helm/dephealth-services/values-homelab.yaml
```

- [ ] Deploy monitoring stack:

```bash
helm upgrade --install dephealth-monitoring \
  ~/Projects/personal/topologymetrics/topologymetrics/deploy/helm/dephealth-monitoring/ \
  -f ~/Projects/personal/topologymetrics/topologymetrics/deploy/helm/dephealth-monitoring/values-homelab.yaml
```

### 0.5 Verify test environment

- [ ] All pods running: `kubectl get pods -n dephealth-test`
- [ ] All pods running: `kubectl get pods -n dephealth-monitoring`
- [ ] Metrics visible in VictoriaMetrics:

```bash
kubectl port-forward -n dephealth-monitoring svc/victoriametrics 8428:8428
curl 'http://localhost:8428/api/v1/query?query=app_dependency_health'
```

- [ ] Alerts configured in AlertManager:

```bash
kubectl port-forward -n dephealth-monitoring svc/alertmanager 9093:9093
curl 'http://localhost:9093/api/v2/alerts'
```

- [ ] Note the internal service URLs for dephealth-ui config:
  - VictoriaMetrics: `http://victoriametrics.dephealth-monitoring.svc:8428`
  - AlertManager: `http://alertmanager.dephealth-monitoring.svc:9093`

### Checkpoint

- Go project compiles (empty placeholder main)
- Makefile works for local build
- Docker multi-arch build succeeds
- Test environment deployed and generating real metrics
- VictoriaMetrics and AlertManager accessible from within the cluster

---

## Phase 1: Go Backend — Configuration & Prometheus Client

**Objective:** Implement the Go backend that connects to VictoriaMetrics,
executes PromQL queries, builds the topology graph, and exposes it via
REST API.

**Estimated effort:** 4-5 days

### 1.1 Configuration module

**Files:** `internal/config/config.go`

- [ ] Define config struct matching `config.example.yaml`:

```go
type Config struct {
    Server      ServerConfig      `yaml:"server"`
    Datasources DatasourcesConfig `yaml:"datasources"`
    Cache       CacheConfig       `yaml:"cache"`
    Auth        AuthConfig        `yaml:"auth"`
    Grafana     GrafanaConfig     `yaml:"grafana"`
}
```

- [ ] Load from YAML file (flag `-config`)
- [ ] Override with environment variables (`DEPHEALTH_SERVER_LISTEN`, etc.)
- [ ] Validate required fields (at minimum `datasources.prometheus.url`)
- [ ] Unit tests for config loading and validation

### 1.2 HTTP server skeleton

**Files:** `internal/server/server.go`, `internal/server/routes.go`

- [ ] Create chi router with middleware:
  - `middleware.Logger`
  - `middleware.Recoverer`
  - `middleware.RequestID`
  - `middleware.RealIP`
  - CORS headers for development
- [ ] Register routes:
  - `GET /api/v1/topology`
  - `GET /api/v1/alerts`
  - `GET /api/v1/config`
  - `GET /healthz` — liveness probe
  - `GET /readyz` — readiness probe (checks datasource connectivity)
  - `GET /*` — SPA static files (placeholder, returns 200 for now)
- [ ] Graceful shutdown on SIGINT/SIGTERM
- [ ] Unit tests for route registration

### 1.3 Prometheus / VictoriaMetrics client

**Files:** `internal/topology/prometheus.go`

- [ ] Use official `github.com/prometheus/client_golang/api` or plain HTTP client
- [ ] Implement query methods:

```go
type PrometheusClient interface {
    // Returns all unique topology edges: {job, dependency, type, host, port}
    QueryTopologyEdges(ctx context.Context) ([]TopologyEdge, error)

    // Returns current health state per edge
    QueryHealthState(ctx context.Context) (map[EdgeKey]float64, error)

    // Returns average latency per edge (rate over 5m)
    QueryAvgLatency(ctx context.Context) (map[EdgeKey]float64, error)

    // Returns P99 latency per edge
    QueryP99Latency(ctx context.Context) (map[EdgeKey]float64, error)
}
```

- [ ] PromQL queries (from design doc):
  - Topology edges: `group by (job, dependency, type, host, port) (app_dependency_health)`
  - Health state: `app_dependency_health`
  - Avg latency: `rate(app_dependency_latency_seconds_sum[5m]) / rate(app_dependency_latency_seconds_count[5m])`
  - P99 latency: `histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket[5m]))`
- [ ] Optional Basic auth for Prometheus connection
- [ ] Configurable timeout
- [ ] Unit tests with mock HTTP server

### 1.4 Graph builder

**Files:** `internal/topology/graph.go`, `internal/topology/models.go`

- [ ] Define models:

```go
type Node struct {
    ID              string `json:"id"`
    Label           string `json:"label"`
    State           string `json:"state"`    // "ok", "degraded", "down", "unknown"
    Type            string `json:"type"`     // "service" or dependency type
    DependencyCount int    `json:"dependencyCount"`
    GrafanaURL      string `json:"grafanaUrl,omitempty"`
}

type Edge struct {
    Source     string  `json:"source"`
    Target     string  `json:"target"`
    Latency    string  `json:"latency"`      // human-readable "5.2ms"
    LatencyRaw float64 `json:"latencyRaw"`
    Health     float64 `json:"health"`       // 0 or 1
    State      string  `json:"state"`        // "ok", "degraded", "down"
    GrafanaURL string  `json:"grafanaUrl,omitempty"`
}

type TopologyResponse struct {
    Nodes  []Node           `json:"nodes"`
    Edges  []Edge           `json:"edges"`
    Alerts []AlertInfo      `json:"alerts"`
    Meta   TopologyMeta     `json:"meta"`
}
```

- [ ] Build graph from Prometheus data:
  1. Query topology edges → create Node set (from `job` labels) + Edge set
  2. Query health state → set `health` on edges
  3. Query latency → set `latency`/`latencyRaw` on edges
  4. Calculate node state:
     - All edges healthy → `ok`
     - Mix of healthy/unhealthy → `degraded`
     - All edges unhealthy → `down`
     - No edges → `unknown`
  5. Generate Grafana URLs from config
- [ ] Unit tests for graph building logic (with sample data)

### 1.5 API handler — GET /api/v1/topology

**Files:** `internal/server/handlers.go`

- [ ] Call graph builder → return JSON TopologyResponse
- [ ] Error handling: return 502 if Prometheus unavailable, 500 for internal errors
- [ ] Set `Content-Type: application/json`
- [ ] Unit tests

### 1.6 Entry point

**Files:** `cmd/dephealth-ui/main.go`

- [ ] Parse flags (`-config`)
- [ ] Load config
- [ ] Create Prometheus client
- [ ] Create graph builder
- [ ] Create and start HTTP server
- [ ] Graceful shutdown

### 1.7 Deploy and test in Kubernetes

- [ ] Build multi-arch image and push to Harbor
- [ ] Create a minimal Kubernetes manifest (Deployment + Service) for quick
      testing (not full Helm chart yet)
- [ ] Set environment or ConfigMap with VictoriaMetrics URL
- [ ] Port-forward and test: `curl localhost:8080/api/v1/topology`
- [ ] Verify the graph JSON contains correct nodes and edges from test services

### Checkpoint

- `GET /api/v1/topology` returns JSON graph with real data
- Nodes correspond to test services (go, python, java, csharp)
- Edges show health status and latency
- Node states (ok/degraded/down) calculated correctly
- Application runs in Kubernetes, connects to VictoriaMetrics

---

## Phase 2: Frontend — Graph Visualization

**Objective:** Create a Vite-based SPA that fetches the topology from the
backend API and renders an interactive directed graph with Cytoscape.js.

**Estimated effort:** 3-4 days

### 2.1 Frontend project initialization

**Files:** `frontend/`

- [ ] Initialize Vite project:

```bash
npm create vite@latest frontend -- --template vanilla
cd frontend && npm install
```

- [ ] Install dependencies:

```bash
npm install cytoscape cytoscape-dagre
```

- [ ] Configure Vite (`frontend/vite.config.js`):
  - Dev server proxy: `/api` → `http://localhost:8080`
  - Build output: `frontend/dist`
- [ ] Verify dev server starts: `npm run dev`

### 2.2 API client

**Files:** `frontend/src/api.js`

- [ ] `fetchTopology()` — GET /api/v1/topology → parse JSON
- [ ] `fetchConfig()` — GET /api/v1/config → parse JSON
- [ ] Error handling: show user-friendly error on network/HTTP errors
- [ ] Auto-retry with backoff on transient errors

### 2.3 Graph renderer

**Files:** `frontend/src/graph.js`

- [ ] Initialize Cytoscape instance with dagre layout:

```javascript
const cy = cytoscape({
    container: document.getElementById('cy'),
    layout: { name: 'dagre', rankDir: 'TB', nodeSep: 80, rankSep: 120 },
    style: [/* see 2.4 */],
    elements: []
});
```

- [ ] `renderGraph(topologyData)`:
  1. Convert API nodes → Cytoscape nodes
  2. Convert API edges → Cytoscape edges
  3. Use `cy.batch()` for efficient update
  4. Run layout
- [ ] `updateGraph(topologyData)`:
  - Diff current elements vs new data
  - Add/remove/update elements without full re-layout
  - Only re-layout if topology structure changed (new/removed nodes/edges)

### 2.4 Graph styles

**Files:** `frontend/src/styles.js`

- [ ] Node styles:
  - **ok:** green background (`#4caf50`)
  - **degraded:** amber background (`#ff9800`)
  - **down:** red background (`#f44336`)
  - **unknown:** gray background (`#9e9e9e`)
  - Label: node name
  - Shape: round-rectangle for services, ellipse for dependencies
- [ ] Edge styles:
  - **ok:** green line
  - **degraded:** dashed amber line
  - **down:** dotted red line
  - Label: latency value (persistent, not hover-only)
  - Arrow: triangle target
  - Font size: 10px, edge-label positioning

### 2.5 Main application

**Files:** `frontend/src/main.js`, `frontend/index.html`

- [ ] HTML structure:
  - Header bar with app title
  - Full-screen Cytoscape container (`#cy`)
  - Status bar (last update time, node/edge count, connection status)
  - Error overlay (shown on API errors)
- [ ] On load:
  1. Fetch config
  2. Fetch topology
  3. Render graph
  4. Start polling interval (configurable, default 15s)
- [ ] Toolbar buttons:
  - Refresh now
  - Fit to screen
  - Toggle auto-refresh

### 2.6 Grafana click-through

- [ ] On node click → open `grafanaUrl` in new tab (if present)
- [ ] On edge click → open `grafanaUrl` in new tab (if present)
- [ ] Visual hover feedback (highlight, cursor pointer)

### 2.7 CSS / layout

**Files:** `frontend/src/style.css`

- [ ] Fullscreen layout (100vh)
- [ ] Header bar (fixed top)
- [ ] Status bar (fixed bottom)
- [ ] Cytoscape container fills remaining space
- [ ] Basic responsive: collapse header on narrow screens
- [ ] Light theme as default

### 2.8 Embed frontend in Go binary

**Files:** `internal/server/static.go`

- [ ] Use `embed.FS` to embed `frontend/dist`:

```go
//go:embed static/*
var staticFiles embed.FS
```

- [ ] Serve embedded files from chi router (`/*`)
- [ ] SPA fallback: serve `index.html` for all non-API, non-static routes
- [ ] Set correct MIME types and cache headers for static assets

### 2.9 Deploy and test in Kubernetes

- [ ] Build multi-arch image (frontend + backend embedded)
- [ ] Push to Harbor
- [ ] Deploy to Kubernetes
- [ ] Port-forward and open in browser
- [ ] Verify graph renders with real test service data
- [ ] Verify auto-refresh works
- [ ] Verify Grafana links work (if Grafana is accessible)

### Checkpoint

- SPA loads in browser and shows topology graph
- Nodes color-coded by state
- Edges show latency labels
- Graph auto-refreshes
- Click on node/edge opens Grafana
- Frontend embedded in single Go binary

---

## Phase 3: AlertManager Integration

**Objective:** Integrate AlertManager API to enrich the topology graph with
active alert information and improve state calculation.

**Estimated effort:** 2-3 days

### 3.1 AlertManager client

**Files:** `internal/alerts/alertmanager.go`

- [ ] Implement AlertManager API v2 client:

```go
type AlertManagerClient interface {
    // Fetch active alerts (firing state)
    FetchAlerts(ctx context.Context) ([]Alert, error)
}
```

- [ ] GET `/api/v2/alerts` with optional filters
- [ ] Parse AlertManager JSON response
- [ ] Map alerts to topology entities (match by `job`, `dependency` labels)
- [ ] Optional Basic auth for AlertManager connection
- [ ] Unit tests with mock HTTP server

### 3.2 Enrich graph with alerts

**Files:** `internal/topology/graph.go` (extend)

- [ ] Add alerts to `TopologyResponse.Alerts` array
- [ ] Cross-reference alerts with nodes/edges:
  - `DependencyDown` / `DependencyDegraded` → affect edge and target node state
  - `DependencyHighLatency` → informational on edge
  - `DependencyFlapping` → informational on edge
  - `DependencyAbsent` → mark node as `unknown`
- [ ] Alert-based state override (alerts are more authoritative than instant query):
  - If `DependencyDown` alert firing → edge state = `down`
  - If `DependencyDegraded` alert firing → edge state = `degraded`
- [ ] Unit tests for alert enrichment

### 3.3 API — GET /api/v1/alerts

**Files:** `internal/server/handlers.go` (extend)

- [ ] Return aggregated alerts:

```json
{
    "alerts": [
        {
            "alertname": "DependencyDown",
            "service": "order-service",
            "dependency": "postgres-main",
            "severity": "critical",
            "state": "firing",
            "since": "2026-02-08T08:30:00Z",
            "summary": "..."
        }
    ],
    "meta": {
        "total": 3,
        "critical": 1,
        "warning": 2,
        "fetchedAt": "..."
    }
}
```

### 3.4 Frontend — alert display

**Files:** `frontend/src/graph.js`, `frontend/src/main.js` (extend)

- [ ] Alert badge on nodes (small icon/counter for active alerts)
- [ ] Edge tooltip with alert details (on hover)
- [ ] Alert summary in status bar (total alerts: X critical, Y warning)
- [ ] Optional: alert panel (sidebar/drawer) listing all active alerts

### 3.5 Deploy and test

- [ ] Build, push, deploy to Kubernetes
- [ ] Trigger test alert (e.g., scale down a dependency in dephealth-infra)
- [ ] Verify alert appears on graph
- [ ] Verify state changes reflected in colors

### Checkpoint

- Alerts fetched from AlertManager and merged into graph
- Alert badges visible on affected nodes
- Alert details available on hover/click
- State calculation considers both metrics and alerts
- `GET /api/v1/alerts` returns structured alert data

---

## Phase 4: Caching, Auth & Helm Chart

**Objective:** Add server-side caching, authentication middleware, and create
a production-ready Helm chart for dephealth-ui.

**Estimated effort:** 3-4 days

### 4.1 Cache layer

**Files:** `internal/cache/cache.go`

- [ ] In-memory TTL cache for topology responses:

```go
type Cache struct {
    mu       sync.RWMutex
    data     *TopologyResponse
    cachedAt time.Time
    ttl      time.Duration
}
```

- [ ] Cache key = "topology" (single global cache, all users see same data)
- [ ] `Get()` → return cached if not expired
- [ ] `Set()` → store with timestamp
- [ ] Background refresh goroutine (optional: pre-fetch before TTL expires)
- [ ] TTL from config (default 15s)
- [ ] Add `cachedAt` and `ttl` to `TopologyResponse.Meta`
- [ ] Unit tests

### 4.2 Auth middleware — `none`

**Files:** `internal/auth/auth.go`, `internal/auth/none.go`

- [ ] Auth middleware interface:

```go
type Authenticator interface {
    Middleware() func(http.Handler) http.Handler
}
```

- [ ] `none` implementation: pass-through (no authentication)

### 4.3 Auth middleware — `basic`

**Files:** `internal/auth/basic.go`

- [ ] HTTP Basic Authentication
- [ ] Users from config (username + bcrypt password hash)
- [ ] Protect `/api/*` routes
- [ ] Allow `/healthz`, `/readyz` without auth
- [ ] Return 401 with `WWW-Authenticate` header on failure
- [ ] Unit tests

### 4.4 Helm chart for dephealth-ui

**Files:** `deploy/helm/dephealth-ui/`

- [ ] `Chart.yaml`:

```yaml
apiVersion: v2
name: dephealth-ui
description: Microservice topology and dependency health visualization
type: application
version: 0.1.0
appVersion: "0.1.0"
```

- [ ] `values.yaml`:

```yaml
global:
  imageRegistry: docker.io
  pushRegistry: ""
  namespace: dephealth-ui

image:
  name: dephealth-ui
  tag: latest
  pullPolicy: IfNotPresent

replicaCount: 1

service:
  type: ClusterIP
  port: 8080

route:
  enabled: false
  hostname: dephealth.example.com
  gateway:
    name: gateway
    namespace: default
  tls:
    enabled: false
    clusterIssuer: ""

config:
  server:
    listen: ":8080"
  datasources:
    prometheus:
      url: "http://victoriametrics.dephealth-monitoring.svc:8428"
    alertmanager:
      url: "http://alertmanager.dephealth-monitoring.svc:9093"
  cache:
    ttl: 15s
  auth:
    type: "none"
  grafana:
    baseUrl: ""
    dashboards:
      serviceStatus: "dephealth-service-status"
      linkStatus: "dephealth-link-status"

resources:
  requests:
    cpu: 50m
    memory: 32Mi
  limits:
    cpu: 200m
    memory: 64Mi

probes:
  liveness:
    path: /healthz
    initialDelaySeconds: 5
    periodSeconds: 10
  readiness:
    path: /readyz
    initialDelaySeconds: 5
    periodSeconds: 10
```

- [ ] `values-homelab.yaml`:

```yaml
global:
  pushRegistry: harbor.kryukov.lan/library
  namespace: dephealth-ui

route:
  enabled: true
  hostname: dephealth.kryukov.lan
  gateway:
    name: gateway
    namespace: default
  tls:
    enabled: true
    clusterIssuer: dev-ca-issuer

config:
  datasources:
    prometheus:
      url: "http://victoriametrics.dephealth-monitoring.svc:8428"
    alertmanager:
      url: "http://alertmanager.dephealth-monitoring.svc:9093"
  grafana:
    baseUrl: "http://grafana.dephealth.local"
```

- [ ] Templates:
  - `namespace.yml` — Namespace
  - `configmap.yml` — YAML config mounted as file
  - `deployment.yml` — Deployment with probes, resource limits, config mount
  - `service.yml` — ClusterIP Service
  - `httproute.yml` — HTTPRoute (Gateway API), optional
  - `_helpers.tpl` — Image path helpers (consistent with topologymetrics charts)

### 4.5 Deploy and test full Helm chart

- [ ] Build, push multi-arch image
- [ ] `helm upgrade --install dephealth-ui deploy/helm/dephealth-ui/ -f deploy/helm/dephealth-ui/values-homelab.yaml`
- [ ] Verify pods running
- [ ] Verify HTTPRoute created
- [ ] Add `dephealth.kryukov.lan` to hosts file (ask user)
- [ ] Access via browser at `https://dephealth.kryukov.lan`
- [ ] Verify full functionality: graph renders, auto-refreshes, alerts shown
- [ ] Test basic auth (change config, redeploy, verify login prompt)

### Checkpoint

- Topology responses cached (15s TTL)
- Basic auth works when enabled
- Full Helm chart deploys cleanly
- Application accessible via Gateway API
- TLS certificate provisioned by cert-manager

---

## Phase 5: OIDC Auth & UI Refinements

**Objective:** Add OIDC/SSO authentication and polish the user interface.

**Estimated effort:** 3-4 days

### 5.1 Auth middleware — `oidc`

**Files:** `internal/auth/oidc.go`

- [ ] OIDC Authorization Code flow:
  - Discovery endpoint (`.well-known/openid-configuration`)
  - Redirect to IdP for login
  - Handle callback (`/auth/callback`)
  - Validate ID token (JWT)
  - Session management (cookie-based)
- [ ] Config: `issuer`, `clientId`, `clientSecret`, `redirectUrl`
- [ ] Use `github.com/coreos/go-oidc/v3` library
- [ ] Logout endpoint (`/auth/logout`)
- [ ] Frontend: show logged-in user in header, logout button
- [ ] Unit tests with mock OIDC provider

### 5.2 Dark theme

**Files:** `frontend/src/style.css`, `frontend/src/main.js`

- [ ] CSS custom properties for theming:

```css
:root {
    --bg-primary: #ffffff;
    --bg-secondary: #f5f5f5;
    --text-primary: #212121;
    --text-secondary: #757575;
}

[data-theme="dark"] {
    --bg-primary: #1e1e1e;
    --bg-secondary: #2d2d2d;
    --text-primary: #e0e0e0;
    --text-secondary: #9e9e9e;
}
```

- [ ] Theme toggle button in header
- [ ] Persist theme preference in `localStorage`
- [ ] Respect `prefers-color-scheme` media query as default
- [ ] Adjust Cytoscape node/edge colors for dark theme

### 5.3 Responsive layout

- [ ] Mobile-friendly touch interactions (pinch-zoom, pan)
- [ ] Collapsible header on small screens
- [ ] Responsive status bar
- [ ] Touch-friendly node/edge tap targets

### 5.4 Error handling improvements

- [ ] Connection lost overlay with retry countdown
- [ ] Partial data indicator (if some queries fail)
- [ ] Toast notifications for transient errors
- [ ] Empty state: message when no services found

### 5.5 Performance optimization

- [ ] Graph rendering:
  - Limit initial viewport to visible nodes
  - Virtual rendering for 100+ node graphs
  - Debounce layout recalculation
- [ ] API:
  - Conditional requests (ETag / If-None-Match)
  - Gzip compression middleware
- [ ] Frontend:
  - Bundle size audit
  - Lazy loading for non-critical features

### 5.6 Deploy and test

- [ ] Build, push, deploy
- [ ] Test OIDC with Keycloak (if available) or skip to future
- [ ] Test dark theme
- [ ] Test on mobile device / narrow viewport
- [ ] Load test with simulated large topology

### Checkpoint

- OIDC authentication works end-to-end
- Dark theme toggleable and persisted
- UI responsive on mobile devices
- Error states handled gracefully
- Performance adequate for 100+ nodes

---

## Phase 6: Testing & Documentation

**Objective:** Comprehensive test coverage, documentation, and preparation for
production use.

**Estimated effort:** 2-3 days

### 6.1 Go unit tests

- [ ] Config loading and validation
- [ ] Prometheus client (with mock HTTP responses)
- [ ] Graph builder (with sample metric data)
- [ ] AlertManager client (with mock HTTP responses)
- [ ] Cache layer
- [ ] Auth middleware (none, basic)
- [ ] API handlers (with httptest)
- [ ] Target: >70% coverage on business logic

### 6.2 Go integration tests

- [ ] Test against real VictoriaMetrics (use test environment)
- [ ] Test full request cycle: API → Prometheus → graph → JSON
- [ ] Tag with `//go:build integration`

### 6.3 Frontend tests (optional)

- [ ] Basic smoke test: graph renders with mock data
- [ ] API client error handling tests
- [ ] Consider Playwright for E2E testing in future

### 6.4 Linting

- [ ] `golangci-lint` configuration (`.golangci.yml`)
- [ ] ESLint configuration for frontend (`frontend/.eslintrc.js`)
- [ ] `markdownlint` for documentation (already configured)
- [ ] Add lint targets to Makefile

### 6.5 Documentation

- [ ] Update `README.md` with:
  - Project description
  - Architecture overview
  - Quick start guide
  - Configuration reference
  - Screenshots
- [ ] API documentation (OpenAPI spec or markdown)
- [ ] Helm chart `README.md` with values reference

### Checkpoint

- All tests pass
- Linting clean
- Documentation complete
- Ready for production deployment

---

## Appendix A: Project Directory Structure (final)

```text
dephealth-ui/
├── cmd/
│   └── dephealth-ui/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── server/
│   │   ├── server.go
│   │   ├── routes.go
│   │   ├── handlers.go
│   │   ├── static.go
│   │   └── handlers_test.go
│   ├── topology/
│   │   ├── models.go
│   │   ├── prometheus.go
│   │   ├── prometheus_test.go
│   │   ├── graph.go
│   │   └── graph_test.go
│   ├── alerts/
│   │   ├── alertmanager.go
│   │   └── alertmanager_test.go
│   ├── auth/
│   │   ├── auth.go
│   │   ├── none.go
│   │   ├── basic.go
│   │   ├── basic_test.go
│   │   ├── oidc.go
│   │   └── oidc_test.go
│   └── cache/
│       ├── cache.go
│       └── cache_test.go
├── frontend/
│   ├── src/
│   │   ├── main.js
│   │   ├── api.js
│   │   ├── graph.js
│   │   ├── styles.js
│   │   └── style.css
│   ├── index.html
│   ├── package.json
│   └── vite.config.js
├── deploy/
│   └── helm/
│       └── dephealth-ui/
│           ├── Chart.yaml
│           ├── values.yaml
│           ├── values-homelab.yaml
│           └── templates/
│               ├── _helpers.tpl
│               ├── namespace.yml
│               ├── configmap.yml
│               ├── deployment.yml
│               ├── service.yml
│               └── httproute.yml
├── plans/
│   └── development-plan.md
├── docs/
│   └── application-design.md
├── Dockerfile
├── Makefile
├── .golangci.yml
├── .gitignore
├── config.example.yaml
├── go.mod
├── go.sum
├── CLAUDE.md
├── AGENTS.md
├── GIT-WORKFLOW.md
└── README.md
```

## Appendix B: Multi-arch Docker Build Reference

```bash
# Create buildx builder (one-time setup)
docker buildx create --name multiarch --driver docker-container --use

# Build and push multi-arch image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg REGISTRY=harbor.kryukov.lan/docker \
  -t harbor.kryukov.lan/library/dephealth-ui:latest \
  -t harbor.kryukov.lan/library/dephealth-ui:v0.1.0 \
  --push .
```

Key points:
- Go cross-compilation is native (no emulation needed for Go build stage)
- Node.js stage produces platform-independent JavaScript (no arch issues)
- Alpine runtime stage supports both amd64 and arm64
- `CGO_ENABLED=0` ensures static binary (no libc dependency)

## Appendix C: Test Environment Namespaces

| Namespace | Components | Source Chart |
|-----------|-----------|-------------|
| `dephealth-test` | PostgreSQL, Redis, stubs, test services | dephealth-infra + dephealth-services |
| `dephealth-monitoring` | VictoriaMetrics, AlertManager, Grafana, VMAlert | dephealth-monitoring |
| `dephealth-ui` | dephealth-ui application | dephealth-ui (this project) |

Cross-namespace access:
- dephealth-ui → VictoriaMetrics: `http://victoriametrics.dephealth-monitoring.svc:8428`
- dephealth-ui → AlertManager: `http://alertmanager.dephealth-monitoring.svc:9093`

## Appendix D: Development Cycle

```text
┌─────────────────────────────────────────────────────────┐
│  Developer workstation (Mac M / amd64)                  │
│                                                         │
│  1. Edit code (Go / JS)                                 │
│  2. make lint test           ← local checks             │
│  3. make dev                 ← build + push + deploy    │
│     ├── docker buildx build  ← multi-arch image         │
│     ├── docker push          ← push to Harbor           │
│     └── helm upgrade         ← deploy to K8s            │
│  4. Browser → https://dephealth.kryukov.lan             │
│                                                         │
│  For frontend dev:                                      │
│  - npm --prefix frontend run dev  ← Vite HMR            │
│  - Proxy /api → K8s pod (port-forward or separate Go)   │
└─────────────────────────────────────────────────────────┘
```
