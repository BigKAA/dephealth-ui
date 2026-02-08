# Phase 9: New Test Environment with Uniproxy

## Overview

Replace the current multi-language test services (go, python, java, csharp) with a single universal test application **uniproxy** that supports multiple dependency types (HTTP, Redis, PostgreSQL, gRPC). This provides a more realistic and flexible test topology for dephealth-ui.

## Goals

1. Single test application supporting all dependency types
2. Multi-namespace deployment for realistic topology testing
3. Configurable topology via Helm values files
4. Compatible with existing dephealth-ui metrics expectations (`app_dependency_health`, `app_dependency_latency_seconds`)
5. Clean separation of infrastructure (each base service in its own namespace)
6. Annotation-based Prometheus service discovery (replace static scrape targets)

---

## Target Topology

```
dephealth-uniproxy                     dephealth-uniproxy-2
┌─────────────────────────┐            ┌─────────────────────────────┐
│                         │            │                             │
│  uniproxy-01 (r:2)     │            │  uniproxy-04 (r:2)         │
│    ├─► uniproxy-02 ────┼───────────►│    ├─► uniproxy-05 (r:1)   │
│    └─► uniproxy-03     │            │    └─► uniproxy-06 (r:2)   │
│                         │            │          ├─► uniproxy-07    │
│  uniproxy-02 (r:2)     │            │          └─► uniproxy-08    │
│    ├─► redis       ─────┼──► dephealth-redis                      │
│    ├─► grpc-stub   ─────┼──► dephealth-grpc-stub                  │
│    └─► uniproxy-04 ─────┼──►│       │                             │
│                         │   │       │  uniproxy-07 (r:1)          │
│  uniproxy-03 (r:3)     │   │       │    └─► postgresql ──────────┼──► dephealth-postgresql
│    └─► postgresql  ─────┼───┼──────►│                             │
│                         │   │       │  uniproxy-08 (r:1)          │
└─────────────────────────┘   │       │    └─► postgresql ──────────┼──► dephealth-postgresql
                              │       └─────────────────────────────┘
```

### Namespaces

| Namespace | Purpose | Components |
|---|---|---|
| `dephealth-redis` | Redis instance | Redis deployment + ClusterIP service |
| `dephealth-postgresql` | PostgreSQL instance | PostgreSQL StatefulSet + ClusterIP service |
| `dephealth-grpc-stub` | gRPC stub service | gRPC stub deployment + ClusterIP service |
| `dephealth-uniproxy` | Test services (group 1) | uniproxy-01, 02, 03 |
| `dephealth-uniproxy-2` | Test services (group 2) | uniproxy-04, 05, 06, 07, 08 |
| `dephealth-monitoring` | Monitoring stack | VictoriaMetrics, AlertManager, Grafana (unchanged) |
| `dephealth-ui` | Application | dephealth-ui (unchanged) |

### Test Scenarios Covered

- Multi-namespace service topology
- Cross-namespace dependencies
- Multiple dependency types (HTTP, Redis, PostgreSQL, gRPC)
- Shared dependencies (PostgreSQL used by 3 services)
- Varying replica counts (1, 2, 3)
- Leaf nodes with no dependencies (uniproxy-05)
- Deep dependency chains (01 → 02 → 04 → 06 → 07 → postgresql)
- NodePort + ClusterIP service types

---

## Phase 9.1: Uniproxy Application

### Application Design

A Go application that periodically health-checks its configured connections and exposes Prometheus metrics compatible with [topologymetrics](https://github.com/BigKAA/topologymetrics).

#### Project Structure

```
test/uniproxy/
├── main.go                    # Entry point
├── go.mod
├── go.sum
├── Dockerfile
├── internal/
│   ├── config/
│   │   └── config.go          # YAML configuration parsing
│   ├── checker/
│   │   ├── checker.go          # Checker interface + manager
│   │   ├── http.go             # HTTP health check
│   │   ├── redis.go            # Redis PING check
│   │   ├── postgres.go         # PostgreSQL connection check
│   │   └── grpc.go             # gRPC connectivity check
│   ├── metrics/
│   │   └── metrics.go          # Prometheus metrics registration
│   └── server/
│       └── server.go           # HTTP server (chi router)
```

#### Configuration Format

```yaml
server:
  listen: ":8080"              # HTTP listen address
  metricsPath: "/metrics"      # Prometheus metrics endpoint

checkInterval: 10s             # Health check interval

connections:
  - name: "uniproxy-02"                              # → dependency label
    type: "http"                                      # → type label (http|redis|postgres|grpc)
    host: "uniproxy-02.dephealth-uniproxy.svc"       # → host label
    port: "8080"                                      # → port label
    path: "/"                                         # HTTP-specific: GET path

  - name: "redis"
    type: "redis"
    host: "redis.dephealth-redis.svc"
    port: "6379"

  - name: "postgresql"
    type: "postgres"
    host: "postgresql.dephealth-postgresql.svc"
    port: "5432"
    database: "dephealth"                             # Postgres-specific
    username: "dephealth"
    password: "dephealth-test-pass"

  - name: "grpc-stub"
    type: "grpc"
    host: "grpc-stub.dephealth-grpc-stub.svc"
    port: "9090"
```

Environment variable overrides:
- `CONFIG_FILE` — path to config (default: `/etc/uniproxy/config.yaml`)
- `LOG_LEVEL` — logging level (default: `info`)
- `POD_NAME` — Kubernetes pod name (from fieldRef)
- `NAMESPACE` — Kubernetes namespace (from fieldRef)

#### HTTP API

| Endpoint | Method | Description |
|---|---|---|
| `GET /` | GET | JSON: pod info + connection statuses |
| `GET /healthz` | GET | Liveness probe (always 200) |
| `GET /readyz` | GET | Readiness probe (200 if checks initialized) |
| `GET /metrics` | GET | Prometheus metrics |

#### Root endpoint response (`GET /`)

```json
{
  "podName": "uniproxy-01-abc123",
  "namespace": "dephealth-uniproxy",
  "connections": [
    {
      "name": "uniproxy-02",
      "type": "http",
      "host": "uniproxy-02.dephealth-uniproxy.svc",
      "port": "8080",
      "healthy": true,
      "lastCheck": "2026-02-08T10:30:00Z",
      "latencyMs": 2.3
    }
  ]
}
```

#### Prometheus Metrics

Matching topologymetrics SDK format with additional `hostname` label:

```
# HELP app_dependency_health Health status of a dependency (1=healthy, 0=unhealthy)
# TYPE app_dependency_health gauge
app_dependency_health{dependency="uniproxy-02",type="http",host="uniproxy-02.dephealth-uniproxy.svc",port="8080",hostname="uniproxy-01-7f8b9c"} 1
app_dependency_health{dependency="redis",type="redis",host="redis.dephealth-redis.svc",port="6379",hostname="uniproxy-01-7f8b9c"} 1

# HELP app_dependency_latency_seconds Latency of dependency health check
# TYPE app_dependency_latency_seconds histogram
app_dependency_latency_seconds_bucket{dependency="uniproxy-02",type="http",host="...",port="8080",hostname="uniproxy-01-7f8b9c",le="0.001"} 5
app_dependency_latency_seconds_bucket{dependency="uniproxy-02",type="http",host="...",port="8080",hostname="uniproxy-01-7f8b9c",le="0.005"} 12
...
app_dependency_latency_seconds_sum{...} 0.035
app_dependency_latency_seconds_count{...} 15
```

Labels from application:
- `dependency` — connection name
- `type` — connection type (`http`, `redis`, `postgres`, `grpc`)
- `host` — target host
- `port` — target port
- `hostname` — container hostname via `os.Hostname()` (= pod name in K8s)

Labels from Kubernetes SD (see Phase 9.4):
- `job` — from pod label `app.kubernetes.io/name`
- `namespace` — from `__meta_kubernetes_namespace`

Histogram buckets: `0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0`

#### Health Check Implementations

**HTTP checker** (`type: "http"`):
- HTTP GET to `http://{host}:{port}{path}`
- Healthy if status 2xx
- Timeout: 5s

**Redis checker** (`type: "redis"`):
- `PING` command
- Healthy if `PONG` response
- Timeout: 5s

**PostgreSQL checker** (`type: "postgres"`):
- `SELECT 1` query via pgx
- Healthy if no error
- Timeout: 5s
- DSN: `postgres://{username}:{password}@{host}:{port}/{database}?sslmode=disable`

**gRPC checker** (`type: "grpc"`):
- TCP dial to `{host}:{port}`
- Healthy if connection succeeds
- Timeout: 5s
- Note: uses simple TCP connectivity check (no gRPC health protocol required)

#### Go Dependencies

```
github.com/go-chi/chi/v5         # HTTP router (consistent with dephealth-ui)
github.com/prometheus/client_golang  # Prometheus metrics
github.com/redis/go-redis/v9     # Redis client
github.com/jackc/pgx/v5          # PostgreSQL client
google.golang.org/grpc           # gRPC (for TCP dial, optional)
gopkg.in/yaml.v3                 # Config parsing
```

#### Dockerfile

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o uniproxy ./main.go

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/uniproxy /usr/local/bin/uniproxy
EXPOSE 8080
ENTRYPOINT ["uniproxy"]
CMD ["-config", "/etc/uniproxy/config.yaml"]
```

Multi-arch build: `linux/amd64` + `linux/arm64`

---

## Phase 9.2: Infrastructure Changes (dephealth-infra)

Modify `deploy/helm/dephealth-infra/` to deploy base services in separate namespaces.

### Changes to values.yaml

```yaml
redis:
  enabled: true
  namespace: "dephealth-redis"     # NEW: per-component namespace
  # ... rest unchanged

postgres:
  enabled: true
  namespace: "dephealth-postgresql" # NEW: per-component namespace
  # ... rest unchanged

stubs:
  httpStub:
    enabled: false                  # CHANGED: disable (replaced by uniproxy)
  grpcStub:
    enabled: true
    namespace: "dephealth-grpc-stub" # NEW: per-component namespace
    # ... rest unchanged
```

### Template Changes

- Create namespace per component (instead of single `dephealth-test`)
- Update Service/Deployment namespace references
- Remove http-stub template (or keep disabled)

### Namespace resources

Each component template creates its own namespace:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Values.redis.namespace }}
  labels:
    app.kubernetes.io/part-of: dephealth
```

### Dex

Dex remains in its own namespace logic (currently uses `global.namespace`). Move Dex to a separate `dephealth-dex` namespace or keep as is. No change required for Phase 9 scope — can stay as is.

---

## Phase 9.3: Uniproxy Helm Chart

### Chart Structure

```
deploy/helm/dephealth-uniproxy/
├── Chart.yaml
├── templates/
│   ├── _helpers.tpl
│   ├── namespace.yml
│   ├── deployment.yml           # Iterates over .Values.instances
│   ├── service.yml              # ClusterIP per instance
│   ├── service-nodeport.yml     # NodePort (conditional)
│   └── configmap.yml            # Per-instance config
├── values.yaml                  # Chart defaults
├── values-homelab.yaml          # Registry overrides
├── instances/
│   ├── ns1-homelab.yaml         # dephealth-uniproxy instances
│   └── ns2-homelab.yaml         # dephealth-uniproxy-2 instances
```

### Chart Design

Templates iterate over `{{ .Values.instances }}` list, creating per-instance:
- Deployment (with configurable replicas)
- ClusterIP Service
- NodePort Service (conditional, per-instance flag)
- ConfigMap (uniproxy config with connections)

### values.yaml (defaults)

```yaml
global:
  pushRegistry: ""
  namespace: "dephealth-uniproxy"

image:
  name: uniproxy
  tag: latest
  pullPolicy: IfNotPresent

checkInterval: "10s"

resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 100m
    memory: 64Mi

probes:
  readinessDelay: 5
  livenessDelay: 5

instances: []
```

### instances/ns1-homelab.yaml

```yaml
instances:
  - name: uniproxy-01
    replicas: 2
    nodePort: 30080
    connections:
      - name: uniproxy-02
        type: http
        host: uniproxy-02.dephealth-uniproxy.svc
        port: "8080"
        path: "/"
      - name: uniproxy-03
        type: http
        host: uniproxy-03.dephealth-uniproxy.svc
        port: "8080"
        path: "/"

  - name: uniproxy-02
    replicas: 2
    connections:
      - name: redis
        type: redis
        host: redis.dephealth-redis.svc
        port: "6379"
      - name: grpc-stub
        type: grpc
        host: grpc-stub.dephealth-grpc-stub.svc
        port: "9090"
      - name: uniproxy-04
        type: http
        host: uniproxy-04.dephealth-uniproxy-2.svc
        port: "8080"
        path: "/"

  - name: uniproxy-03
    replicas: 3
    connections:
      - name: postgresql
        type: postgres
        host: postgresql.dephealth-postgresql.svc
        port: "5432"
        database: dephealth
        username: dephealth
        password: dephealth-test-pass
```

### instances/ns2-homelab.yaml

```yaml
instances:
  - name: uniproxy-04
    replicas: 2
    connections:
      - name: uniproxy-05
        type: http
        host: uniproxy-05.dephealth-uniproxy-2.svc
        port: "8080"
        path: "/"
      - name: uniproxy-06
        type: http
        host: uniproxy-06.dephealth-uniproxy-2.svc
        port: "8080"
        path: "/"

  - name: uniproxy-05
    replicas: 1
    connections: []

  - name: uniproxy-06
    replicas: 2
    connections:
      - name: uniproxy-07
        type: http
        host: uniproxy-07.dephealth-uniproxy-2.svc
        port: "8080"
        path: "/"
      - name: uniproxy-08
        type: http
        host: uniproxy-08.dephealth-uniproxy-2.svc
        port: "8080"
        path: "/"

  - name: uniproxy-07
    replicas: 1
    connections:
      - name: postgresql
        type: postgres
        host: postgresql.dephealth-postgresql.svc
        port: "5432"
        database: dephealth
        username: dephealth
        password: dephealth-test-pass

  - name: uniproxy-08
    replicas: 1
    connections:
      - name: postgresql
        type: postgres
        host: postgresql.dephealth-postgresql.svc
        port: "5432"
        database: dephealth
        username: dephealth
        password: dephealth-test-pass
```

### Deployment Template (key snippet)

```yaml
{{- range .Values.instances }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .name }}
  namespace: {{ $.Release.Namespace }}
spec:
  replicas: {{ .replicas | default 1 }}
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ .name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ .name }}
        app.kubernetes.io/part-of: dephealth
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - name: uniproxy
          image: {{ include "dephealth-uniproxy.image" $ }}
          args: ["-config", "/etc/uniproxy/config.yaml"]
          ports:
            - containerPort: 8080
          env:
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
          volumeMounts:
            - name: config
              mountPath: /etc/uniproxy
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: {{ .name }}-config
{{- end }}
```

---

## Phase 9.4: Monitoring Updates — Annotation-Based Service Discovery

### Overview

Replace static `scrapeTargets` list with Kubernetes pod service discovery.
VictoriaMetrics will automatically discover and scrape any pod with
`prometheus.io/scrape: "true"` annotation.

**Benefits:**
- No need to update scrape config when adding/removing services
- `namespace` and `job` labels come from K8s metadata automatically
- Services are self-describing via annotations
- Existing `scrapeTargets` / `extraScrapeTargets` values removed

### Required RBAC Resources

VictoriaMetrics needs access to the Kubernetes API to discover pods.
Add to `deploy/helm/dephealth-monitoring/templates/victoriametrics.yml`:

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: victoriametrics
  namespace: {{ include "dephealth-monitoring.namespace" . }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: victoriametrics-sd
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: victoriametrics-sd
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: victoriametrics-sd
subjects:
  - kind: ServiceAccount
    name: victoriametrics
    namespace: {{ include "dephealth-monitoring.namespace" . }}
```

Add `serviceAccountName: victoriametrics` to the StatefulSet pod spec.

### Scrape Config (new)

Replace static `scrape_configs` with `kubernetes_sd_configs`:

```yaml
scrape.yml: |
  global:
    scrape_interval: 15s
    scrape_timeout: 10s

  scrape_configs:
    # --- Kubernetes pod auto-discovery ---
    - job_name: kubernetes-pods
      kubernetes_sd_configs:
        - role: pod

      relabel_configs:
        # 1. Only scrape pods with prometheus.io/scrape=true
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
          action: keep
          regex: "true"

        # 2. Only scrape pods with app.kubernetes.io/part-of=dephealth
        - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_part_of]
          action: keep
          regex: "dephealth"

        # 3. Use scheme from annotation (default: http)
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scheme]
          action: replace
          target_label: __scheme__
          regex: (https?)

        # 4. Use path from annotation (default: /metrics)
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
          action: replace
          target_label: __metrics_path__
          regex: (.+)

        # 5. Use port from annotation
        - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
          action: replace
          regex: ([^:]+)(?::\d+)?;(\d+)
          replacement: $1:$2
          target_label: __address__

        # 6. Set namespace label from K8s metadata
        - source_labels: [__meta_kubernetes_namespace]
          action: replace
          target_label: namespace

        # 7. Set job label from app.kubernetes.io/name
        - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
          action: replace
          target_label: job

        # 8. Set service label (same as job)
        - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
          action: replace
          target_label: service

        # 9. Set pod label
        - source_labels: [__meta_kubernetes_pod_name]
          action: replace
          target_label: pod

    # --- VictoriaMetrics self-monitoring (static) ---
    - job_name: victoriametrics
      static_configs:
        - targets:
            - localhost:8428
```

### Pod Annotations (required on all scraped pods)

```yaml
annotations:
  prometheus.io/scrape: "true"        # Enable scraping
  prometheus.io/port: "8080"          # Metrics port
  prometheus.io/path: "/metrics"      # Optional (default: /metrics)
  prometheus.io/scheme: "http"        # Optional (default: http)
```

### Pod Labels (required for SD relabeling)

```yaml
labels:
  app.kubernetes.io/name: uniproxy-01    # → becomes `job` label
  app.kubernetes.io/part-of: dephealth   # → filter: only dephealth pods
```

### Values Changes

**Remove** from `values.yaml` and `values-homelab.yaml`:
- `scrapeTargets` list
- `extraScrapeTargets` list

**Add** to `values.yaml`:
```yaml
victoriametrics:
  serviceDiscovery:
    enabled: true     # Use kubernetes_sd_configs
```

### Label Flow

| Label | Source | How |
|---|---|---|
| `job` | Pod label `app.kubernetes.io/name` | Relabeling rule #7 |
| `namespace` | K8s pod metadata | Relabeling rule #6 (`__meta_kubernetes_namespace`) |
| `service` | Pod label `app.kubernetes.io/name` | Relabeling rule #8 |
| `pod` | K8s pod metadata | Relabeling rule #9 (`__meta_kubernetes_pod_name`) |
| `dependency` | Application metric | Direct from `app_dependency_health` |
| `type` | Application metric | Direct from `app_dependency_health` |
| `host` | Application metric | Direct from `app_dependency_health` |
| `port` | Application metric | Direct from `app_dependency_health` |
| `hostname` | Application metric | `os.Hostname()` in Go (= pod name in K8s) |

### Backward Compatibility

dephealth-ui reads `job` and `namespace` from `r.Metric["job"]` and `r.Metric["namespace"]`
(`internal/topology/prometheus.go:142-143`). These labels are present regardless of
whether they come from static config or K8s SD — **no changes needed in dephealth-ui**.

### Alert Rules

No changes needed — VMAlert rules use generic label matchers and will work with
new services automatically.

### Grafana Dashboards

No changes needed — dashboards use label-based variables (`$namespace`,
`$dependency_type`, etc.) and will adapt automatically.

---

## Phase 9.5: Makefile Updates

```makefile
# --- Uniproxy Docker build ---
uniproxy-build:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(REGISTRY)/uniproxy:$(TAG) \
		--push \
		test/uniproxy/

# --- Environment deploy ---
env-deploy:
	helm upgrade --install dephealth-infra deploy/helm/dephealth-infra \
		-f deploy/helm/dephealth-infra/values-homelab.yaml
	helm upgrade --install dephealth-uniproxy-ns1 deploy/helm/dephealth-uniproxy \
		-f deploy/helm/dephealth-uniproxy/values-homelab.yaml \
		-f deploy/helm/dephealth-uniproxy/instances/ns1-homelab.yaml \
		-n dephealth-uniproxy --create-namespace
	helm upgrade --install dephealth-uniproxy-ns2 deploy/helm/dephealth-uniproxy \
		-f deploy/helm/dephealth-uniproxy/values-homelab.yaml \
		-f deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml \
		-n dephealth-uniproxy-2 --create-namespace
	helm upgrade --install dephealth-monitoring deploy/helm/dephealth-monitoring \
		-f deploy/helm/dephealth-monitoring/values-homelab.yaml

env-undeploy:
	helm uninstall dephealth-monitoring -n dephealth-monitoring || true
	helm uninstall dephealth-uniproxy-ns1 -n dephealth-uniproxy || true
	helm uninstall dephealth-uniproxy-ns2 -n dephealth-uniproxy-2 || true
	helm uninstall dephealth-infra || true
	kubectl delete namespace dephealth-redis dephealth-postgresql dephealth-grpc-stub \
		dephealth-uniproxy dephealth-uniproxy-2 dephealth-monitoring \
		--ignore-not-found

env-status:
	@echo "=== dephealth-redis ==="
	kubectl get pods -n dephealth-redis
	@echo "=== dephealth-postgresql ==="
	kubectl get pods -n dephealth-postgresql
	@echo "=== dephealth-grpc-stub ==="
	kubectl get pods -n dephealth-grpc-stub
	@echo "=== dephealth-uniproxy ==="
	kubectl get pods -n dephealth-uniproxy
	@echo "=== dephealth-uniproxy-2 ==="
	kubectl get pods -n dephealth-uniproxy-2
	@echo "=== dephealth-monitoring ==="
	kubectl get pods -n dephealth-monitoring
```

---

## Phase 9.6: Cleanup

1. **Delete** `deploy/helm/dephealth-services/` directory entirely
2. **Remove** http-stub from dephealth-infra (or disable by default)
3. **Remove** `scrapeTargets` / `extraScrapeTargets` from monitoring values (replaced by K8s SD)
4. **Update** documentation references

---

## Implementation Order

| Step | Description | Dependencies |
|---|---|---|
| 9.1 | Implement uniproxy Go application | — |
| 9.1b | Build Docker image & push to Harbor | 9.1 |
| 9.2 | Modify dephealth-infra (separate namespaces) | — |
| 9.3 | Create dephealth-uniproxy Helm chart | — |
| 9.4 | Update monitoring scrape config | — |
| 9.5 | Update Makefile | 9.2, 9.3, 9.4 |
| 9.6a | Deploy & test | 9.1b, 9.5 |
| 9.6b | Verify dephealth-ui topology display | 9.6a |
| 9.7 | Delete dephealth-services chart | 9.6b |

Steps 9.1, 9.2, 9.3, 9.4 can be done in parallel.

---

## Docker Image

- **Registry**: `harbor.kryukov.lan/library/uniproxy`
- **Tag**: `v0.1.0` (initial)
- **Platforms**: `linux/amd64`, `linux/arm64`
- **Base**: `alpine:3.21`
- **Size**: ~15-20MB

---

## Verification Checklist

- [ ] Uniproxy application builds and runs locally
- [ ] Docker multi-arch image builds and pushes to Harbor
- [ ] dephealth-infra deploys redis, postgresql, grpc-stub in separate namespaces
- [ ] dephealth-uniproxy chart deploys all 8 instances across 2 namespaces
- [ ] Pod annotations `prometheus.io/scrape: "true"` present on all uniproxy pods
- [ ] VictoriaMetrics RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding) works
- [ ] VictoriaMetrics auto-discovers and scrapes all 8 uniproxy pods via K8s SD
- [ ] `job` label correctly set from `app.kubernetes.io/name` pod label
- [ ] `namespace` label correctly set from K8s pod metadata
- [ ] `app_dependency_health` and `app_dependency_latency_seconds` appear in VictoriaMetrics
- [ ] dephealth-ui shows correct topology graph with all nodes and edges
- [ ] AlertManager rules fire correctly (test by stopping a dependency)
- [ ] Grafana dashboards show data for new services
- [ ] NodePort access to uniproxy-01 works
- [ ] dephealth-services chart deleted with no side effects
- [ ] Adding a new uniproxy instance only requires Helm values + deploy (no scrape config changes)
