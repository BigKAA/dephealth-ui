# dephealth-ui: Development Plan

## Overview

Full development plan for dephealth-ui — a web application for real-time
microservice topology and dependency health visualization.

**Stack:** Go (net/http + chi) backend + Vanilla JS (Vite + Cytoscape.js + dagre) frontend
**Deployment:** Kubernetes (Helm), multi-arch Docker images (amd64 + arm64)
**Test environment:** topologymetrics Helm charts (dephealth-infra, dephealth-services, dephealth-monitoring)

---

## Phase 0: Project Setup & Test Environment [COMPLETED]

**Objective:** Initialize project structure, build tooling, and deploy the test
environment in Kubernetes so that real Prometheus metrics and alerts are
available for development.

**Estimated effort:** 2-3 days

### 0.1 Go project initialization

- [x] `go mod init github.com/BigKAA/dephealth-ui`
- [x] Create directory structure:

```text
cmd/dephealth-ui/main.go        — entry point (placeholder)
internal/config/config.go       — configuration types
internal/server/server.go       — HTTP server stub
frontend/                       — empty dir for Phase 2
deploy/helm/dephealth-ui/       — empty dir for Phase 4
```

- [x] Add `.gitignore` (Go + Node.js + IDE artifacts)
- [x] Add `config.example.yaml` with documented defaults

### 0.2 Makefile

- [x] Create `Makefile` with targets:

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

- [x] Variables at the top: `REGISTRY`, `IMAGE_NAME`, `TAG`, `PLATFORMS`, `NAMESPACE`
- [x] Default `PLATFORMS = linux/amd64,linux/arm64`

### 0.3 Dockerfile (multi-stage, multi-arch)

- [x] Create `Dockerfile`:

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

- [x] Verify build: `docker buildx build --platform linux/amd64,linux/arm64 -t test .`

### 0.4 Deploy test environment in Kubernetes

Deploy the topologymetrics stack that generates real `app_dependency_health`
and `app_dependency_latency_seconds` metrics.

**Source charts:** `deploy/helm/` (local copies from topologymetrics project)

- [x] Build and push stub images (http-stub, grpc-stub) — multi-arch

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

- [x] Build and push test service images (go, python, java, csharp) — multi-arch

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg REGISTRY=harbor.kryukov.lan/docker \
  -t harbor.kryukov.lan/library/dephealth-test-go:latest \
  -f test-services/go-service/Dockerfile --push .

# Similarly for python, java, csharp services
```

- [x] Deploy infrastructure:

```bash
helm upgrade --install dephealth-infra \
  deploy/helm/dephealth-infra/ \
  -f deploy/helm/dephealth-infra/values-homelab.yaml
```

- [x] Deploy test services:

```bash
helm upgrade --install dephealth-services \
  deploy/helm/dephealth-services/ \
  -f deploy/helm/dephealth-services/values-homelab.yaml
```

- [x] Deploy monitoring stack:

```bash
helm upgrade --install dephealth-monitoring \
  deploy/helm/dephealth-monitoring/ \
  -f deploy/helm/dephealth-monitoring/values-homelab.yaml
```

### 0.5 Verify test environment

- [x] All pods running: `kubectl get pods -n dephealth-test`
- [x] All pods running: `kubectl get pods -n dephealth-monitoring`
- [x] Metrics visible in VictoriaMetrics:

```bash
kubectl port-forward -n dephealth-monitoring svc/victoriametrics 8428:8428
curl 'http://localhost:8428/api/v1/query?query=app_dependency_health'
```

- [x] Alerts configured in AlertManager:

```bash
kubectl port-forward -n dephealth-monitoring svc/alertmanager 9093:9093
curl 'http://localhost:9093/api/v2/alerts'
```

- [x] Note the internal service URLs for dephealth-ui config:
  - VictoriaMetrics: `http://victoriametrics.dephealth-monitoring.svc:8428`
  - AlertManager: `http://alertmanager.dephealth-monitoring.svc:9093`

### Checkpoint

- Go project compiles (empty placeholder main)
- Makefile works for local build
- Docker multi-arch build succeeds
- Test environment deployed and generating real metrics
- VictoriaMetrics and AlertManager accessible from within the cluster

---

## Phase 1: Go Backend — Configuration & Prometheus Client [COMPLETED]

**Objective:** Implement the Go backend that connects to VictoriaMetrics,
executes PromQL queries, builds the topology graph, and exposes it via
REST API.

**Estimated effort:** 4-5 days

### 1.1 Configuration module

**Files:** `internal/config/config.go`

- [x] Define config struct matching `config.example.yaml`:

```go
type Config struct {
    Server      ServerConfig      `yaml:"server"`
    Datasources DatasourcesConfig `yaml:"datasources"`
    Cache       CacheConfig       `yaml:"cache"`
    Auth        AuthConfig        `yaml:"auth"`
    Grafana     GrafanaConfig     `yaml:"grafana"`
}
```

- [x] Load from YAML file (flag `-config`)
- [x] Override with environment variables (`DEPHEALTH_SERVER_LISTEN`, etc.)
- [x] Validate required fields (at minimum `datasources.prometheus.url`)
- [x] Unit tests for config loading and validation

### 1.2 HTTP server skeleton

**Files:** `internal/server/server.go`, `internal/server/routes.go`

- [x] Create chi router with middleware:
  - `middleware.Logger`
  - `middleware.Recoverer`
  - `middleware.RequestID`
  - `middleware.RealIP`
  - CORS headers for development
- [x] Register routes:
  - `GET /api/v1/topology`
  - `GET /api/v1/alerts`
  - `GET /api/v1/config`
  - `GET /healthz` — liveness probe
  - `GET /readyz` — readiness probe (checks datasource connectivity)
  - `GET /*` — SPA static files (placeholder, returns 200 for now)
- [x] Graceful shutdown on SIGINT/SIGTERM
- [x] Unit tests for route registration

### 1.3 Prometheus / VictoriaMetrics client

**Files:** `internal/topology/prometheus.go`

- [x] Use official `github.com/prometheus/client_golang/api` or plain HTTP client
- [x] Implement query methods:

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

- [x] PromQL queries (from design doc):
  - Topology edges: `group by (job, dependency, type, host, port) (app_dependency_health)`
  - Health state: `app_dependency_health`
  - Avg latency: `rate(app_dependency_latency_seconds_sum[5m]) / rate(app_dependency_latency_seconds_count[5m])`
  - P99 latency: `histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket[5m]))`
- [x] Optional Basic auth for Prometheus connection
- [x] Configurable timeout
- [x] Unit tests with mock HTTP server

### 1.4 Graph builder

**Files:** `internal/topology/graph.go`, `internal/topology/models.go`

- [x] Define models:

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

- [x] Build graph from Prometheus data:
  1. Query topology edges → create Node set (from `job` labels) + Edge set
  2. Query health state → set `health` on edges
  3. Query latency → set `latency`/`latencyRaw` on edges
  4. Calculate node state:
     - All edges healthy → `ok`
     - Mix of healthy/unhealthy → `degraded`
     - All edges unhealthy → `down`
     - No edges → `unknown`
  5. Generate Grafana URLs from config
- [x] Unit tests for graph building logic (with sample data)

### 1.5 API handler — GET /api/v1/topology

**Files:** `internal/server/handlers.go`

- [x] Call graph builder → return JSON TopologyResponse
- [x] Error handling: return 502 if Prometheus unavailable, 500 for internal errors
- [x] Set `Content-Type: application/json`
- [x] Unit tests

### 1.6 Entry point

**Files:** `cmd/dephealth-ui/main.go`

- [x] Parse flags (`-config`)
- [x] Load config
- [x] Create Prometheus client
- [x] Create graph builder
- [x] Create and start HTTP server
- [x] Graceful shutdown

### 1.7 Deploy and test in Kubernetes

- [x] Build multi-arch image and push to Harbor
- [x] Create a minimal Kubernetes manifest (Deployment + Service) for quick
      testing (not full Helm chart yet)
- [x] Set environment or ConfigMap with VictoriaMetrics URL
- [x] Port-forward and test: `curl localhost:8080/api/v1/topology`
- [x] Verify the graph JSON contains correct nodes and edges from test services

### Checkpoint

- `GET /api/v1/topology` returns JSON graph with real data
- Nodes correspond to test services (go, python, java, csharp)
- Edges show health status and latency
- Node states (ok/degraded/down) calculated correctly
- Application runs in Kubernetes, connects to VictoriaMetrics

---

## Phase 2: Frontend — Graph Visualization [COMPLETED]

**Objective:** Create a Vite-based SPA that fetches the topology from the
backend API and renders an interactive directed graph with Cytoscape.js.

**Estimated effort:** 3-4 days

### 2.1 Frontend project initialization

**Files:** `frontend/`

- [x] Initialize Vite project:

```bash
npm create vite@latest frontend -- --template vanilla
cd frontend && npm install
```

- [x] Install dependencies:

```bash
npm install cytoscape cytoscape-dagre
```

- [x] Configure Vite (`frontend/vite.config.js`):
  - Dev server proxy: `/api` → `http://localhost:8080`
  - Build output: `frontend/dist`
- [x] Verify dev server starts: `npm run dev`

### 2.2 API client

**Files:** `frontend/src/api.js`

- [x] `fetchTopology()` — GET /api/v1/topology → parse JSON
- [x] `fetchConfig()` — GET /api/v1/config → parse JSON
- [x] Error handling: show user-friendly error on network/HTTP errors
- [x] Auto-retry with backoff on transient errors

### 2.3 Graph renderer

**Files:** `frontend/src/graph.js`

- [x] Initialize Cytoscape instance with dagre layout:

```javascript
const cy = cytoscape({
    container: document.getElementById('cy'),
    layout: { name: 'dagre', rankDir: 'TB', nodeSep: 80, rankSep: 120 },
    style: [/* see 2.4 */],
    elements: []
});
```

- [x] `renderGraph(topologyData)`:
  1. Convert API nodes → Cytoscape nodes
  2. Convert API edges → Cytoscape edges
  3. Use `cy.batch()` for efficient update
  4. Run layout
- [x] `updateGraph(topologyData)`:
  - Diff current elements vs new data
  - Add/remove/update elements without full re-layout
  - Only re-layout if topology structure changed (new/removed nodes/edges)

### 2.4 Graph styles

**Files:** `frontend/src/styles.js`

- [x] Node styles:
  - **ok:** green background (`#4caf50`)
  - **degraded:** amber background (`#ff9800`)
  - **down:** red background (`#f44336`)
  - **unknown:** gray background (`#9e9e9e`)
  - Label: node name
  - Shape: round-rectangle for services, ellipse for dependencies
- [x] Edge styles:
  - **ok:** green line
  - **degraded:** dashed amber line
  - **down:** dotted red line
  - Label: latency value (persistent, not hover-only)
  - Arrow: triangle target
  - Font size: 10px, edge-label positioning

### 2.5 Main application

**Files:** `frontend/src/main.js`, `frontend/index.html`

- [x] HTML structure:
  - Header bar with app title
  - Full-screen Cytoscape container (`#cy`)
  - Status bar (last update time, node/edge count, connection status)
  - Error overlay (shown on API errors)
- [x] On load:
  1. Fetch config
  2. Fetch topology
  3. Render graph
  4. Start polling interval (configurable, default 15s)
- [x] Toolbar buttons:
  - Refresh now
  - Fit to screen
  - Toggle auto-refresh

### 2.6 Grafana click-through

- [x] On node click → open `grafanaUrl` in new tab (if present)
- [x] On edge click → open `grafanaUrl` in new tab (if present)
- [x] Visual hover feedback (highlight, cursor pointer)

### 2.7 CSS / layout

**Files:** `frontend/src/style.css`

- [x] Fullscreen layout (100vh)
- [x] Header bar (fixed top)
- [x] Status bar (fixed bottom)
- [x] Cytoscape container fills remaining space
- [x] Basic responsive: collapse header on narrow screens
- [x] Light theme as default

### 2.8 Embed frontend in Go binary

**Files:** `internal/server/static.go`

- [x] Use `embed.FS` to embed `frontend/dist`:

```go
//go:embed static/*
var staticFiles embed.FS
```

- [x] Serve embedded files from chi router (`/*`)
- [x] SPA fallback: serve `index.html` for all non-API, non-static routes
- [x] Set correct MIME types and cache headers for static assets

### 2.9 Deploy and test in Kubernetes

- [x] Build multi-arch image (frontend + backend embedded)
- [x] Push to Harbor
- [x] Deploy to Kubernetes
- [x] Port-forward and open in browser
- [x] Verify graph renders with real test service data
- [x] Verify auto-refresh works
- [x] Verify Grafana links work (if Grafana is accessible)

### Checkpoint

- SPA loads in browser and shows topology graph
- Nodes color-coded by state
- Edges show latency labels
- Graph auto-refreshes
- Click on node/edge opens Grafana
- Frontend embedded in single Go binary

---

## Phase 3: AlertManager Integration [COMPLETED]

**Objective:** Integrate AlertManager API to enrich the topology graph with
active alert information and improve state calculation.

**Estimated effort:** 2-3 days

### 3.1 AlertManager client

**Files:** `internal/alerts/alertmanager.go`

- [x] Implement AlertManager API v2 client:

```go
type AlertManagerClient interface {
    // Fetch active alerts (firing state)
    FetchAlerts(ctx context.Context) ([]Alert, error)
}
```

- [x] GET `/api/v2/alerts` with optional filters
- [x] Parse AlertManager JSON response
- [x] Map alerts to topology entities (match by `job`, `dependency` labels)
- [x] Optional Basic auth for AlertManager connection
- [x] Unit tests with mock HTTP server

### 3.2 Enrich graph with alerts

**Files:** `internal/topology/graph.go` (extend)

- [x] Add alerts to `TopologyResponse.Alerts` array
- [x] Cross-reference alerts with nodes/edges:
  - `DependencyDown` / `DependencyDegraded` → affect edge and target node state
  - `DependencyHighLatency` → informational on edge
  - `DependencyFlapping` → informational on edge
  - `DependencyAbsent` → mark node as `unknown`
- [x] Alert-based state override (alerts are more authoritative than instant query):
  - If `DependencyDown` alert firing → edge state = `down`
  - If `DependencyDegraded` alert firing → edge state = `degraded`
- [x] Unit tests for alert enrichment

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

- [x] Alert badge on nodes (small icon/counter for active alerts)
- [x] Edge tooltip with alert details (on hover)
- [x] Alert summary in status bar (total alerts: X critical, Y warning)
- [ ] Optional: alert panel (sidebar/drawer) listing all active alerts (deferred to Phase 5)

### 3.5 Deploy and test

- [x] Build, push, deploy to Kubernetes
- [ ] Trigger test alert (e.g., scale down a dependency in dephealth-infra) — no active alerts at test time
- [x] Verify alert appears on graph
- [x] Verify state changes reflected in colors

### Checkpoint

- Alerts fetched from AlertManager and merged into graph
- Alert badges visible on affected nodes
- Alert details available on hover/click
- State calculation considers both metrics and alerts
- `GET /api/v1/alerts` returns structured alert data

---

## Phase 4: Caching, Auth & Helm Chart [COMPLETED]

**Objective:** Add server-side caching, authentication middleware, and create
a production-ready Helm chart for dephealth-ui.

**Estimated effort:** 3-4 days

### 4.1 Cache layer

**Files:** `internal/cache/cache.go`

- [x] In-memory TTL cache for topology responses
- [x] Cache key = "topology" (single global cache, all users see same data)
- [x] `Get()` → return cached if not expired
- [x] `Set()` → store with timestamp
- [x] Background refresh goroutine (optional: pre-fetch before TTL expires)
- [x] TTL from config (default 15s)
- [x] Add `cachedAt` and `ttl` to `TopologyResponse.Meta`
- [x] Unit tests

### 4.2 Auth middleware — `none`

**Files:** `internal/auth/auth.go`, `internal/auth/none.go`

- [x] Auth middleware interface (`Authenticator`)
- [x] `none` implementation: pass-through (no authentication)

### 4.3 Auth middleware — `basic`

**Files:** `internal/auth/basic.go`

- [x] HTTP Basic Authentication
- [x] Users from config (username + bcrypt password hash)
- [x] Protect `/api/*` routes
- [x] Allow `/healthz`, `/readyz` without auth
- [x] Return 401 with `WWW-Authenticate` header on failure
- [x] Unit tests

### 4.4 Helm chart for dephealth-ui

**Files:** `deploy/helm/dephealth-ui/`

- [x] `Chart.yaml`
- [x] `values.yaml`
- [x] `values-homelab.yaml`
- [x] Templates:
  - `namespace.yml` — Namespace
  - `configmap.yml` — YAML config mounted as file
  - `deployment.yml` — Deployment with probes, resource limits, config mount
  - `service.yml` — ClusterIP Service
  - `httproute.yml` — HTTPRoute (Gateway API), optional
  - `_helpers.tpl` — Image path helpers (consistent with topologymetrics charts)

### 4.5 Deploy and test full Helm chart

- [x] Build, push multi-arch image
- [x] `helm upgrade --install dephealth-ui deploy/helm/dephealth-ui/ -f deploy/helm/dephealth-ui/values-homelab.yaml`
- [x] Verify pods running
- [x] Verify HTTPRoute created
- [x] Add `dephealth.kryukov.lan` to hosts file (ask user)
- [x] Access via browser at `https://dephealth.kryukov.lan`
- [x] Verify full functionality: graph renders, auto-refreshes, alerts shown
- [x] Test basic auth (change config, redeploy, verify login prompt)

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

### 5.1 Auth middleware — `oidc` [COMPLETED]

**Files:** `internal/auth/oidc.go`, `internal/auth/session.go`

- [x] OIDC Authorization Code flow with PKCE (S256):
  - Discovery endpoint (`.well-known/openid-configuration`)
  - Redirect to IdP for login (`/auth/login`)
  - Handle callback (`/auth/callback`)
  - Validate ID token (JWT)
  - Session management (cookie-based, in-memory store, 8h TTL)
- [x] Config: `issuer`, `clientId`, `clientSecret`, `redirectUrl`
- [x] Use `github.com/coreos/go-oidc/v3` library
- [x] Logout endpoint (`/auth/logout`)
- [x] User info endpoint (`/auth/userinfo`)
- [x] Frontend: show logged-in user in header, logout button
- [x] `authenticatedFetch()` wrapper (401 → redirect to login)
- [x] `Routes() http.Handler` added to `Authenticator` interface
- [x] `auth.type` exposed in config API response
- [x] Unit tests with mock OIDC provider (RSA keypair, JWKS, full flow)

### 5.2 Dark theme [COMPLETED]

**Files:** `frontend/src/style.css`, `frontend/src/graph.js`, `frontend/src/main.js`, `frontend/index.html`

- [x] CSS custom properties for theming (`:root` light + `html[data-theme="dark"]`)
- [x] Theme toggle button in header
- [x] Persist theme preference in `localStorage`
- [x] Respect `prefers-color-scheme` media query as default
- [x] Adjust Cytoscape edge label colors for dark theme (function-based styles + `updateGraphTheme`)

### 5.3 Responsive layout [COMPLETED]

- [x] Mobile-friendly touch interactions (pinch-zoom, pan)
- [x] Collapsible header on small screens
- [x] Responsive status bar
- [x] Touch-friendly node/edge tap targets

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
│       ├── dephealth-ui/          ← application chart (Phase 4)
│       │   ├── Chart.yaml
│       │   ├── values.yaml
│       │   ├── values-homelab.yaml
│       │   └── templates/
│       ├── dephealth-infra/       ← test infrastructure (PostgreSQL, Redis, stubs)
│       │   ├── Chart.yaml
│       │   ├── values.yaml
│       │   ├── values-homelab.yaml
│       │   └── templates/
│       ├── dephealth-services/    ← test microservices (go, python, java, csharp)
│       │   ├── Chart.yaml
│       │   ├── values.yaml
│       │   ├── values-homelab.yaml
│       │   └── templates/
│       └── dephealth-monitoring/  ← monitoring (VictoriaMetrics, AlertManager, Grafana)
│           ├── Chart.yaml
│           ├── values.yaml
│           ├── values-homelab.yaml
│           ├── dashboards/
│           └── templates/
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
