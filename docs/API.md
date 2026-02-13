# REST API Reference

**Language:** English | [Русский](./API.ru.md)

---

## Overview

dephealth-ui exposes a REST API for topology visualization and health monitoring. All endpoints return JSON and support CORS for browser-based clients.

**Base URL:** `https://dephealth.example.com`
**API Prefix:** `/api/v1`

---

## Authentication

Authentication mode is configured in `config.yaml`:

- **`none`** — No authentication (open access)
- **`basic`** — HTTP Basic Authentication (username/password)
- **`oidc`** — OpenID Connect (redirects to SSO provider)

For OIDC, the frontend automatically handles the OAuth2 flow. API calls after authentication include session cookies.

---

## Endpoints

### `GET /api/v1/topology`

Returns the complete service topology graph with pre-calculated node/edge states.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `namespace` | string | No | Filter by Kubernetes namespace (empty = all) |

**Caching:** Unfiltered requests (`namespace` empty) are cached server-side. Supports `ETag` / `If-None-Match` headers — returns `304 Not Modified` when data hasn't changed.

**Response:** `200 OK`

```json
{
  "nodes": [
    {
      "id": "order-service",
      "label": "order-service",
      "state": "ok",
      "type": "service",
      "namespace": "production",
      "dependencyCount": 3,
      "grafanaUrl": "https://grafana.example.com/d/dephealth-service-status?var-service=order-service",
      "alertCount": 0,
      "alertSeverity": ""
    },
    {
      "id": "postgres-main",
      "label": "postgres-main",
      "state": "unknown",
      "type": "postgres",
      "namespace": "production",
      "host": "pg-master.db.svc",
      "port": "5432",
      "dependencyCount": 0,
      "stale": true
    }
  ],
  "edges": [
    {
      "source": "order-service",
      "target": "postgres-main",
      "type": "postgres",
      "latency": "5.2ms",
      "latencyRaw": 0.0052,
      "health": 1,
      "state": "ok",
      "critical": true,
      "grafanaUrl": "https://grafana.example.com/d/dephealth-link-status?var-dependency=postgres-main&var-host=pg-master.db.svc&var-port=5432",
      "alertCount": 1,
      "alertSeverity": "warning"
    }
  ],
  "alerts": [
    {
      "alertname": "DependencyHighLatency",
      "service": "payment-api",
      "dependency": "auth-service",
      "severity": "warning",
      "state": "firing",
      "since": "2026-02-10T08:30:00Z",
      "summary": "High latency detected on payment-api → auth-service"
    }
  ],
  "meta": {
    "cachedAt": "2026-02-10T09:15:30Z",
    "ttl": 15,
    "nodeCount": 42,
    "edgeCount": 187,
    "partial": false,
    "errors": []
  }
}
```

**Node fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique node identifier |
| `label` | string | Display label |
| `state` | string | `ok`, `degraded`, `down`, `unknown` |
| `type` | string | `service` (instrumented app) or dependency type (`postgres`, `redis`, `http`, etc.) |
| `namespace` | string | Kubernetes namespace |
| `host` | string | Endpoint hostname (omitted for service nodes) |
| `port` | string | Endpoint port (omitted for service nodes) |
| `dependencyCount` | int | Number of outgoing edges |
| `stale` | bool | `true` if the node's metrics have disappeared (lookback mode only) |
| `grafanaUrl` | string | Direct link to Grafana Service Status dashboard (omitted if Grafana not configured) |
| `alertCount` | int | Number of active alerts (omitted if 0) |
| `alertSeverity` | string | Highest alert severity (omitted if no alerts) |

**Edge fields:**

| Field | Type | Description |
|-------|------|-------------|
| `source` | string | Source node ID |
| `target` | string | Target node ID |
| `type` | string | Connection type (`http`, `grpc`, `postgres`, `redis`, etc.) |
| `latency` | string | Human-readable latency (`"5.2ms"`) |
| `latencyRaw` | float64 | Raw latency in seconds |
| `health` | float64 | `1` = healthy, `0` = unhealthy, `-1` = stale |
| `state` | string | `ok`, `degraded`, `down`, `unknown` |
| `critical` | bool | Whether this is a critical dependency |
| `stale` | bool | `true` if edge metrics have disappeared (lookback mode only) |
| `grafanaUrl` | string | Direct link to Grafana Link Status dashboard (omitted if Grafana not configured) |
| `alertCount` | int | Number of active alerts for this edge (omitted if 0) |
| `alertSeverity` | string | Highest alert severity for this edge (omitted if no alerts) |

**Meta fields:**

| Field | Type | Description |
|-------|------|-------------|
| `cachedAt` | string | RFC3339 timestamp of when the data was cached |
| `ttl` | int | Cache TTL in seconds (clients should poll at this interval) |
| `nodeCount` | int | Total number of nodes |
| `edgeCount` | int | Total number of edges |
| `partial` | bool | `true` if some queries failed and data may be incomplete |
| `errors` | string[] | Error descriptions if `partial=true` (omitted if empty) |

**Node States (service nodes):**
- `ok` — all outgoing edges healthy (health=1)
- `degraded` — any outgoing edge has health=0
- `down` — all outgoing edges are stale (metrics disappeared)
- `unknown` — no outgoing edges / no data

> Note: The backend `calcServiceNodeState` never returns `"down"` directly — it only returns `ok`, `degraded`, or `unknown`. The `down` state is set by stale detection logic when all edges are stale.

**Edge States:**
- `ok` — health = 1
- `down` — health = 0
- `unknown` — stale (metrics disappeared within lookback window)

**Cascade warnings (frontend-only):**
Cascade failure propagation is computed entirely on the frontend. The API response does not include cascade data (`cascadeCount`, `cascadeSources`, `inCascadeChain`). The `critical` field on edges determines whether failures propagate upstream as cascade warnings. See [Application Design — Cascade Warnings](./application-design.md#cascade-warnings).

---

### `GET /api/v1/cascade-analysis`

Performs BFS cascade failure analysis across the dependency graph. Returns root causes, affected services, and full cascade chains with unlimited depth.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `service` | string | No | Analyze cascade for a specific service (empty = analyze all) |
| `namespace` | string | No | Filter by Kubernetes namespace |
| `depth` | int | No | Maximum BFS traversal depth (`0` = unlimited) |

**Response:** `200 OK`

```json
{
  "rootCauses": [
    {
      "id": "postgres-main.db.svc:5432",
      "label": "postgres-main",
      "state": "down",
      "namespace": "production"
    }
  ],
  "affectedServices": [
    {
      "service": "order-service",
      "namespace": "production",
      "dependsOn": "payment-api",
      "rootCauses": ["postgres-main.db.svc:5432"]
    }
  ],
  "allFailures": [
    {
      "service": "payment-api",
      "dependency": "postgres-main.db.svc:5432",
      "health": 0,
      "critical": true
    }
  ],
  "cascadeChains": [
    {
      "affectedService": "order-service",
      "namespace": "production",
      "dependsOn": "postgres-main",
      "path": ["order-service", "payment-api", "postgres-main"],
      "depth": 2
    }
  ],
  "summary": {
    "totalServices": 10,
    "rootCauseCount": 1,
    "affectedServiceCount": 3,
    "totalFailureCount": 2,
    "maxDepth": 2
  }
}
```

**Root Cause fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Dependency identifier (may include host:port) |
| `label` | string | Human-readable label |
| `state` | string | Current state (`down`, `degraded`, etc.) |
| `namespace` | string | Kubernetes namespace |

**Cascade Chain fields:**

| Field | Type | Description |
|-------|------|-------------|
| `affectedService` | string | Service affected by the cascade |
| `namespace` | string | Namespace of the affected service |
| `dependsOn` | string | Terminal dependency (root cause) |
| `path` | string[] | Full path from affected service to root cause |
| `depth` | int | Number of hops in the chain |

---

### `GET /api/v1/cascade-graph`

Returns cascade failure topology in [Grafana Node Graph panel](https://grafana.com/docs/grafana/latest/panels-visualizations/visualizations/node-graph/) format. Designed to be consumed directly by the Grafana Infinity datasource.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `service` | string | No | Filter cascade graph for a specific service (empty = all) |
| `namespace` | string | No | Filter by Kubernetes namespace |
| `depth` | int | No | Maximum BFS traversal depth (`0` = unlimited) |

**Response:** `200 OK`

```json
{
  "nodes": [
    {
      "id": "order-service",
      "title": "order-service",
      "subTitle": "production",
      "mainStat": "ok",
      "arc__failed": 0,
      "arc__degraded": 0,
      "arc__ok": 1,
      "arc__unknown": 0
    },
    {
      "id": "postgres-main",
      "title": "postgres-main",
      "subTitle": "production",
      "mainStat": "down",
      "arc__failed": 1,
      "arc__degraded": 0,
      "arc__ok": 0,
      "arc__unknown": 0
    }
  ],
  "edges": [
    {
      "id": "order-service--postgres-main",
      "source": "order-service",
      "target": "postgres-main",
      "mainStat": ""
    }
  ]
}
```

**Node fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique node identifier |
| `title` | string | Display label |
| `subTitle` | string | Kubernetes namespace |
| `mainStat` | string | Node state: `ok`, `degraded`, `down`, `unknown` |
| `arc__failed` | float | Arc segment for failed state (0.0–1.0) |
| `arc__degraded` | float | Arc segment for degraded state (0.0–1.0) |
| `arc__ok` | float | Arc segment for healthy state (0.0–1.0) |
| `arc__unknown` | float | Arc segment for unknown state (0.0–1.0) |

The `arc__*` fields control the colored ring around each node in the Grafana Node Graph panel. Exactly one field is set to `1` per node, based on the node's state.

**Edge fields:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Edge identifier (`source--target`) |
| `source` | string | Source node ID |
| `target` | string | Target node ID |
| `mainStat` | string | Reserved for future use |

---

### `GET /api/v1/alerts`

Returns all active alerts from AlertManager aggregated by service/dependency.

**Response:** `200 OK`

```json
{
  "alerts": [
    {
      "alertname": "DependencyDown",
      "service": "order-service",
      "dependency": "postgres-main",
      "severity": "critical",
      "state": "firing",
      "since": "2026-02-10T09:00:00Z",
      "summary": "Dependency postgres-main is completely down"
    }
  ],
  "meta": {
    "total": 5,
    "critical": 1,
    "warning": 4,
    "fetchedAt": "2026-02-10T09:15:30Z"
  }
}
```

**Alert fields:**

| Field | Type | Description |
|-------|------|-------------|
| `alertname` | string | Alert rule name (`DependencyDown`, `DependencyDegraded`, etc.) |
| `service` | string | Source service name |
| `dependency` | string | Target dependency name |
| `severity` | string | `critical`, `warning`, `info` |
| `state` | string | `firing` |
| `since` | string | RFC3339 timestamp of alert start |
| `summary` | string | Human-readable alert description (optional) |

**Meta fields:**

| Field | Type | Description |
|-------|------|-------------|
| `total` | int | Total number of active alerts |
| `critical` | int | Number of critical alerts |
| `warning` | int | Number of warning alerts |
| `fetchedAt` | string | RFC3339 timestamp of when alerts were fetched |

---

### `GET /api/v1/config`

Returns frontend configuration (Grafana URLs, dashboard UIDs, severity colors, display settings). This endpoint does not require authentication.

**Response:** `200 OK`

```json
{
  "grafana": {
    "baseUrl": "https://grafana.example.com",
    "dashboards": {
      "cascadeOverview": "dephealth-cascade-overview",
      "rootCause": "dephealth-root-cause",
      "serviceStatus": "dephealth-service-status",
      "linkStatus": "dephealth-link-status",
      "serviceList": "dephealth-service-list",
      "servicesStatus": "dephealth-services-status",
      "linksStatus": "dephealth-links-status"
    }
  },
  "cache": {
    "ttl": 15
  },
  "auth": {
    "type": "oidc"
  },
  "alerts": {
    "severityLevels": [
      {"value": "critical", "color": "#f44336"},
      {"value": "warning", "color": "#ff9800"},
      {"value": "info", "color": "#2196f3"}
    ]
  }
}
```

**Dashboard UIDs:**

| Key | Purpose | URL Parameters |
|-----|---------|----------------|
| `cascadeOverview` | Cascade failure overview | `?var-namespace=<ns>` |
| `rootCause` | Root cause analyzer | `?var-service=<name>&var-namespace=<ns>` |
| `serviceStatus` | Single service health | `?var-service=<name>` |
| `linkStatus` | Single dependency health | `?var-dependency=<dep>&var-host=<host>&var-port=<port>` |
| `serviceList` | All services table | — |
| `servicesStatus` | All services overview | — |
| `linksStatus` | All links overview | — |

If `grafana.baseUrl` is empty, Grafana integration features are hidden in the UI.

---

### `GET /api/v1/instances`

Returns all running instances (pods/containers) for a specific service.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `service` | string | Yes | Service name (from `name` label) |

**Example:** `GET /api/v1/instances?service=order-service`

**Response:** `200 OK`

Returns a JSON array of instances (not wrapped in an object):

```json
[
  {
    "instance": "10.244.1.5:9090",
    "pod": "order-service-7d9f8b-xyz12",
    "job": "order-service",
    "service": "order-service"
  },
  {
    "instance": "10.244.2.8:9090",
    "pod": "order-service-7d9f8b-abc34",
    "job": "order-service",
    "service": "order-service"
  }
]
```

**Error:** `400 Bad Request` if `service` parameter is missing.

---

### `GET /healthz`

Kubernetes liveness probe. Always returns `200 OK` with `{"status":"ok"}`.

### `GET /readyz`

Kubernetes readiness probe. Always returns `200 OK` with `{"status":"ok"}`.

---

### `GET /auth/login`

Initiates OIDC authentication flow (only when `auth.type=oidc`).

**Response:** `302 Found`
Redirects to OIDC provider's authorization endpoint.

---

### `GET /auth/callback`

OIDC callback endpoint (only when `auth.type=oidc`).

**Response:** `302 Found`
Sets session cookie and redirects to application root.

---

### `GET /auth/logout`

Terminates user session (only when `auth.type=oidc`).

**Response:** `302 Found`
Clears session cookie and redirects to login page.

---

### `GET /auth/userinfo`

Returns current authenticated user information (only when `auth.type=oidc`).

**Response:** `200 OK`

```json
{
  "username": "john.doe",
  "email": "john.doe@example.com",
  "authenticated": true
}
```

**Response (unauthenticated):** `401 Unauthorized`

---

## Error Responses

All error responses follow this format:

```json
{
  "error": "error message description"
}
```

**Common HTTP Status Codes:**

| HTTP Status | Description |
|-------------|-------------|
| 400 | Invalid or missing query parameters |
| 401 | Authentication required |
| 502 | Prometheus/AlertManager unreachable |

---

## CORS

CORS is enabled for all origins with these settings:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, OPTIONS
Access-Control-Allow-Headers: Accept, Content-Type, If-None-Match
Access-Control-Max-Age: 300
```

---

## Caching and ETag

The `/api/v1/topology` endpoint (unfiltered) supports HTTP caching:

- Server returns `ETag` header with each response
- Clients can send `If-None-Match` with the previous ETag value
- If data hasn't changed, server returns `304 Not Modified` (empty body)
- Recommended polling interval: use `meta.ttl` value (default 15s)

Other endpoints (`/api/v1/alerts`, `/api/v1/instances`) are not cached and always return fresh data.

---

## Rate Limiting

No explicit rate limiting is enforced. Caching at server-side reduces load on Prometheus/AlertManager.

Recommended client polling intervals:
- `/api/v1/topology` — every 15-30 seconds (use `meta.ttl`)
- `/api/v1/alerts` — every 30-60 seconds
- `/api/v1/config` — once at startup

---

## See Also

- [Metrics Specification](./METRICS.md) — Required metrics format
- [Application Design](./application-design.md) — Architecture overview
- [Deployment Guide](../deploy/helm/dephealth-ui/README.md) — Kubernetes & Helm
