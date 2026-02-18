# Metrics Format Specification

**Language:** English | [Русский](./METRICS.ru.md)

---

## Overview

**dephealth-ui** requires Prometheus-compatible metrics that describe service dependencies and their health status. These metrics are collected by applications instrumented with the [dephealth SDK](https://github.com/BigKAA/topologymetrics).

This document specifies:
- Required metric names and types
- Mandatory and optional labels
- Value formats and constraints
- PromQL queries used by the application
- Integration examples

---

## Required Metrics

### 1. `app_dependency_health`

**Type:** Gauge
**Description:** Health status of a service dependency endpoint.

**Values:**
- `1` — dependency is healthy (responding successfully)
- `0` — dependency is down (unreachable or failing health checks)

**Required Labels:**

| Label | Required | Description | Example Values |
|-------|:--------:|-------------|----------------|
| `name` | ✅ | Application name (service identifier) | `order-service`, `payment-api`, `user-backend` |
| `namespace` | ✅ | Kubernetes namespace or logical grouping | `production`, `staging`, `team-alpha` |
| `dependency` | ✅ | Logical name of the dependency | `postgres-main`, `redis-cache`, `auth-service` |
| `type` | ✅ | Connection protocol/type | `http`, `grpc`, `tcp`, `postgres`, `mysql`, `redis`, `mongodb`, `amqp`, `kafka` |
| `host` | ✅ | Target endpoint hostname/IP | `pg-master.db.svc.cluster.local`, `10.0.1.5` |
| `port` | ✅ | Target endpoint port | `5432`, `6379`, `8080` |
| `critical` | ✅ | Dependency criticality flag | `yes`, `no` |

**Optional Labels:**

| Label | Description | Example Values |
|-------|-------------|----------------|
| `group` | Logical service group (SDK v0.5.0+). Enables group dimension toggle in the UI. When absent, namespace-only grouping is used. | `proxy-cluster-1`, `infra-host1`, `payment-team` |
| `role` | Instance role (for replicated systems) | `primary`, `replica`, `standby` |
| `shard` | Shard identifier (for sharded systems) | `shard-01`, `shard-02` |
| `vhost` | AMQP virtual host | `/`, `/app` |
| `pod` | Kubernetes pod name | `order-service-7d9f8b-xyz12` |
| `instance` | Prometheus instance label | `10.244.1.5:9090` |
| `job` | Prometheus job label | `order-service` |

**Example:**
```prometheus
app_dependency_health{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",role="primary"} 1
app_dependency_health{name="order-service",namespace="production",dependency="redis-cache",type="redis",host="redis.cache.svc",port="6379",critical="no"} 1
app_dependency_health{name="payment-api",namespace="production",dependency="auth-service",type="http",host="auth.svc",port="8080",critical="yes"} 0
```

---

### 2. `app_dependency_latency_seconds`

**Type:** Histogram
**Description:** Health check latency for dependency endpoints (in seconds).

**Buckets:** `0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0`

**Required Labels:** Same as `app_dependency_health` (all required labels must match).

**Generated Time Series:**
- `app_dependency_latency_seconds_bucket{..., le="0.001"}` — count of requests ≤ 1ms
- `app_dependency_latency_seconds_bucket{..., le="0.005"}` — count of requests ≤ 5ms
- `app_dependency_latency_seconds_bucket{..., le="+Inf"}` — total count
- `app_dependency_latency_seconds_sum{...}` — sum of all latencies
- `app_dependency_latency_seconds_count{...}` — total number of health checks

**Example:**
```prometheus
app_dependency_latency_seconds_bucket{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",le="0.001"} 45
app_dependency_latency_seconds_bucket{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",le="0.005"} 98
app_dependency_latency_seconds_bucket{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",le="+Inf"} 100
app_dependency_latency_seconds_sum{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes"} 0.152
app_dependency_latency_seconds_count{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes"} 100
```

---

## PromQL Queries Used by dephealth-ui

The application executes the following queries against Prometheus/VictoriaMetrics:

### 1. **Topology Discovery** — extract all unique edges
```promql
group by (name, namespace, group, dependency, type, host, port, critical) (app_dependency_health)
```
**Purpose:** Discover all service→dependency relationships in the system. The `group` label is included when available (SDK v0.5.0+).

### 2. **Health State** — current health value per edge
```promql
app_dependency_health
```
**Purpose:** Determine if each dependency endpoint is currently UP (1) or DOWN (0).

### 3. **Average Latency** — mean latency per edge
```promql
rate(app_dependency_latency_seconds_sum[5m]) / rate(app_dependency_latency_seconds_count[5m])
```
**Purpose:** Calculate rolling 5-minute average latency for each dependency.

### 4. **P99 Latency** — 99th percentile latency per edge
```promql
histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket[5m]))
```
**Purpose:** Calculate P99 latency to identify slow dependencies.

### 5. **Service Instances** — list all instances for a service
```promql
group by (instance, pod, job) (app_dependency_health{name="<service-name>"})
```
**Purpose:** Display all running instances/pods for a selected service in the sidebar.

---

## Graph Model

- **Nodes (Vertices):** Unique values of `name` label → represent services/applications
- **Edges (Directed):** Unique combinations of `{name, namespace, group, dependency, type, host, port, critical}` → represent service→dependency connections
- **Edge Properties:**
  - **critical:** visual thickness (critical dependencies are displayed thicker) + cascade warning propagation (only `critical=yes` edges propagate failure warnings upstream)
  - **latency:** displayed as label on edge
  - **health:** affects edge color (green=OK, yellow=degraded, red=down)

**Example Topology:**
```
order-service (node)
  ├─→ postgres-main (edge: critical=yes, type=postgres, latency=5ms)
  ├─→ redis-cache (edge: critical=no, type=redis, latency=1ms)
  └─→ payment-api (edge: critical=yes, type=http, latency=15ms)
```

---

## State Calculation Rules

**Service Node State** (computed by backend in `calcServiceNodeState`):
- **unknown:** No outgoing edges (no dependency data)
- **degraded:** Any outgoing edge has `health=0`
- **ok:** All outgoing edges have `health=1`
- **down:** Only when all outgoing edges are stale (metrics disappeared) — set by stale detection logic, not by `calcServiceNodeState`

> Note: `calcServiceNodeState` never returns `"down"`. See [Application Design — State Model](./application-design.md#state-model) for details.

**Dependency Node State** (computed by backend):
- **down:** All incoming edges are stale (`stale=true`)
- **ok:** `health=1` (from non-stale incoming edges)
- **down:** `health=0`

**Edge State:**
- **ok:** `app_dependency_health = 1`
- **down:** `app_dependency_health = 0`
- **unknown:** Stale (metrics disappeared within lookback window)

### The `critical` Label and Cascade Warnings

The `critical` label (`yes`/`no`) has two effects in dephealth-ui:

1. **Visual:** Critical edges are displayed thicker on the graph
2. **Cascade warnings:** Only edges with `critical=yes` propagate failure warnings upstream. When a dependency goes down, cascade warnings are sent to all upstream services connected through critical edges.

**Example:** If `order-service → postgres-main (critical=yes)` and `postgres-main` goes down:
- `order-service` receives a cascade warning badge `⚠ 1` with tooltip showing the root cause
- If `order-service → redis-cache (critical=no)` and `redis-cache` goes down — no cascade warning is generated

See [Application Design — Cascade Warnings](./application-design.md#cascade-warnings) for the full algorithm description.

---

## AlertManager Integration

dephealth-ui queries AlertManager API v2 for active alerts:

**Expected Alert Labels:**
- `alertname` — alert rule name (e.g., `DependencyDown`, `DependencyHighLatency`)
- `severity` — `critical`, `warning`, `info`
- `name` — service name (matches metric label)
- `dependency` — dependency name (matches metric label)

**Common Alerts** (from topologymetrics project):
- `DependencyDown` — all endpoints down for 1min (critical)
- `DependencyDegraded` — mixed UP/DOWN states for 2min (warning)
- `DependencyHighLatency` — P99 > 1s for 5min (warning)
- `DependencyFlapping` — >4 state changes in 15min (info)
- `DependencyAbsent` — metrics missing for 5min (warning)

---

## Integration Guide

### Step 1: Instrument Your Application

Use the [dephealth SDK](https://github.com/BigKAA/topologymetrics) to automatically emit metrics:

**Go Example:**
```go
import "github.com/BigKAA/topologymetrics/sdk-go"

// Initialize SDK
sdk, _ := dephealth.New(dephealth.Config{
    ServiceName: "order-service",
    Namespace:   "production",
    MetricsAddr: ":9090",
})

// Register dependencies
pgDep := sdk.RegisterDependency(dephealth.Dependency{
    Name:     "postgres-main",
    Type:     "postgres",
    Host:     "pg-master.db.svc",
    Port:     5432,
    Critical: true,
    HealthCheck: func(ctx context.Context) error {
        return db.PingContext(ctx)
    },
})

// SDK automatically exports metrics to /metrics endpoint
```

### Step 2: Configure Prometheus Scraping

Ensure Prometheus scrapes your application's `/metrics` endpoint:

```yaml
scrape_configs:
  - job_name: 'order-service'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: [production]
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
```

### Step 3: Deploy AlertManager Rules

Install alerting rules (example from topologymetrics Helm chart):

```yaml
groups:
  - name: dephealth
    rules:
      - alert: DependencyDown
        expr: |
          (count by (name, namespace, dependency) (app_dependency_health == 0) > 0)
          and
          (count by (name, namespace, dependency) (app_dependency_health == 1) == 0)
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Dependency {{ $labels.dependency }} is completely down"
```

### Step 4: Configure dephealth-ui

Point the application to your Prometheus and AlertManager:

```yaml
# config.yaml
datasources:
  prometheus:
    url: "http://victoriametrics.monitoring.svc:8428"
  alertmanager:
    url: "http://alertmanager.monitoring.svc:9093"
```

---

## Validation Checklist

- Metrics `app_dependency_health` and `app_dependency_latency_seconds` are exposed
- All required labels are present: `name`, `namespace`, `dependency`, `type`, `host`, `port`, `critical`
- Label values are consistent (same labels for health + latency metrics)
- Health values are exactly `0` or `1` (not strings, not other numbers)
- Latency histogram has standard buckets
- Prometheus successfully scrapes metrics (check `/targets` page)
- AlertManager is configured and reachable

**Test Query:**
```promql
# Should return your service topology
group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health)
```

---

## Troubleshooting

**Problem:** Topology graph is empty
**Solution:** Verify metrics are present in Prometheus:
```promql
count(app_dependency_health)
```
If zero, check Prometheus scrape configuration.

**Problem:** Edges missing in topology
**Solution:** Ensure all required labels are present and non-empty. Query:
```promql
app_dependency_health{name="", namespace="", dependency="", type="", host="", port=""}
```
Should return 0 results (no metrics with empty required labels).

**Problem:** Latency not displayed
**Solution:** Check histogram metrics:
```promql
rate(app_dependency_latency_seconds_count[5m])
```
If zero, health checks are not recording latency.

**Problem:** Wrong node states
**Solution:** Verify AlertManager integration and alert labels match metric labels.

---

## See Also

- [Application Design](./application-design.md) — Full architecture overview
- [API Documentation](./API.md) — REST API endpoints
- [Deployment Guide](../deploy/helm/dephealth-ui/README.md) — Kubernetes & Helm
- [dephealth SDK](https://github.com/BigKAA/topologymetrics) — Official instrumentation library
