# Phase 10: Migration to dephealth SDK v0.2.0

## Overview

Migrate the uniproxy test application from a standalone implementation to the dephealth
Go SDK v0.2.0, and update dephealth-ui's topology graph builder to handle the new metric
labels (`name`, `critical`). The SDK v0.2.0 introduces breaking changes: mandatory `name`
(application identifier) and `critical` (dependency criticality) labels in Prometheus metrics.

## Goals

1. Rewrite uniproxy to use dephealth SDK instead of custom health check code
2. Switch uniproxy configuration from YAML files to environment variables
3. Update dephealth-ui graph builder to use `name` label instead of `job` for source nodes
4. Add `critical` support throughout the topology pipeline (backend → frontend)
5. Update Helm charts for env var-based uniproxy configuration
6. Build, deploy, and verify the complete test environment

---

## Label Changes (v0.1.0 → v0.2.0)

```
Old metric format (custom uniproxy):
  app_dependency_health{dependency="...", type="...", host="...", port="...", hostname="..."}

New metric format (SDK v0.2.0):
  app_dependency_health{name="...", dependency="...", type="...", host="...", port="...", critical="yes|no"}
```

| Label | Old (v0.1.0) | New (v0.2.0) | Notes |
|-------|-------------|-------------|-------|
| `name` | absent | **mandatory** | Application identifier |
| `dependency` | present | present | Dependency logical name |
| `type` | present | present | `http`, `redis`, `postgres`, `grpc` |
| `host` | present | present | Target hostname |
| `port` | present | present | Target port |
| `critical` | absent | **mandatory** | `yes` or `no` |
| `hostname` | present | **removed** | Was custom, not in SDK |

---

## Phase 10.1: Rewrite uniproxy to use dephealth SDK

### Task 10.1.1: Update `go.mod`

- [ ] Add dependency `github.com/BigKAA/topologymetrics` v0.2.x
- [ ] Import `github.com/BigKAA/topologymetrics/dephealth` and `dephealth/checks`
- [ ] Keep `chi/v5` (HTTP router), `pgx/v5` (PostgreSQL driver), `go-redis/v9` (Redis client)
- [ ] Remove direct `prometheus/client_golang` dependency (SDK handles metrics)
- [ ] Remove `gopkg.in/yaml.v3` (no more YAML config)
- [ ] Run `go mod tidy`

### Task 10.1.2: Rewrite `internal/config/config.go` — env var configuration

Replace YAML-based config with environment variables.

**Application-level env vars:**

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DEPHEALTH_NAME` | yes | — | Application name (e.g. `uniproxy-01`) |
| `LISTEN_ADDR` | no | `:8080` | HTTP server listen address |
| `LOG_LEVEL` | no | `info` | Log level: `debug` or `info` |
| `DEPHEALTH_CHECK_INTERVAL` | no | `10` | Check interval in seconds |

**Dependency definition:**

| Variable | Required | Description |
|----------|----------|-------------|
| `DEPHEALTH_DEPS` | yes | Comma-separated `name:type` pairs |

Example: `DEPHEALTH_DEPS=uniproxy-02:http,redis:redis,postgresql:postgres`

**Per-dependency env vars** (SDK naming convention):

| Variable | Description |
|----------|-------------|
| `DEPHEALTH_<NAME>_URL` | Connection URL |
| `DEPHEALTH_<NAME>_HOST` | Host (alternative to URL) |
| `DEPHEALTH_<NAME>_PORT` | Port (alternative to URL) |
| `DEPHEALTH_<NAME>_CRITICAL` | `yes` or `no` |
| `DEPHEALTH_<NAME>_HEALTH_PATH` | HTTP health check path |

Name mapping: `uniproxy-02` → env prefix `UNIPROXY_02` (hyphens → underscores, uppercase).

**Config struct:**

```go
type Config struct {
    Name          string
    ListenAddr    string
    LogLevel      string
    CheckInterval time.Duration
    Dependencies  []Dependency
}

type Dependency struct {
    Name       string
    Type       string // http, redis, postgres, grpc
    URL        string
    Host       string
    Port       string
    Critical   bool
    HealthPath string // HTTP only
}
```

- [ ] Implement `Load() (*Config, error)` — parse all env vars
- [ ] Implement `parseDeps(depsStr string) ([]Dependency, error)`
- [ ] Implement `envName(depName string) string` — name to ENV_PREFIX conversion
- [ ] Validate: name required, at least one dependency, critical required per dep
- [ ] Write unit tests for config parsing

### Task 10.1.3: Delete `internal/checker/` package

Remove all custom health check code — SDK handles this:

- [ ] Delete `internal/checker/checker.go` (Manager, Checker interface, Result)
- [ ] Delete `internal/checker/http.go` (HTTPChecker)
- [ ] Delete `internal/checker/redis.go` (RedisChecker)
- [ ] Delete `internal/checker/postgres.go` (PostgresChecker)
- [ ] Delete `internal/checker/grpc.go` (GRPCChecker)

### Task 10.1.4: Delete `internal/metrics/` package

Remove custom Prometheus metrics — SDK handles registration:

- [ ] Delete `internal/metrics/metrics.go`

### Task 10.1.5: Rewrite `main.go`

New application flow:

```
1. config.Load()           — parse env vars
2. slog.New()              — initialize logger
3. buildOptions()          — build []dephealth.Option from config
4. dephealth.New()         — create SDK instance
5. dh.Start(ctx)           — start health checks
6. server.New().Start()    — HTTP server (status, probes, metrics)
7. signal.NotifyContext()  — wait for shutdown
8. dh.Stop()               — graceful stop
```

- [ ] Implement `buildOptions(cfg, logger) []dephealth.Option`
- [ ] Implement `buildDependencyOption(dep) dephealth.Option` — factory switch
- [ ] Wire graceful shutdown: SIGINT/SIGTERM → dh.Stop() + server shutdown
- [ ] Import `_ "github.com/BigKAA/topologymetrics/dephealth/checks"` for checker factories

### Task 10.1.6: Rewrite `internal/server/server.go`

Simplify — remove dependency on checker.Manager:

- [ ] `GET /` — JSON: `{name, namespace, podName, health: dh.Health()}`
- [ ] `GET /healthz` — `200 ok` (liveness)
- [ ] `GET /readyz` — `200 ok` after `dh.Start()` (readiness)
- [ ] `GET /metrics` — `promhttp.Handler()` (SDK-registered metrics)
- [ ] Constructor takes `*dephealth.DepHealth` instead of `*checker.Manager`

### Task 10.1.7: Update `Dockerfile`

- [ ] Keep multi-stage, multi-arch build
- [ ] Remove `CMD ["-config", "/etc/uniproxy/config.yaml"]` — no config file
- [ ] Keep `ENTRYPOINT ["uniproxy"]`
- [ ] Ensure `CGO_ENABLED=0`

### Task 10.1.8: Validate build

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`

---

## Phase 10.2: Update dephealth-ui topology package

### Task 10.2.1: Update `internal/topology/models.go`

- [ ] Add `Critical bool` field to `TopologyEdge`
- [ ] Add `critical` to JSON serialization (edge response)
- [ ] Update `TopologyNode` if needed (e.g. display info)

### Task 10.2.2: Update `internal/topology/prometheus.go`

**PromQL query changes:**

Old:
```promql
group by (job, namespace, dependency, type, host, port) (app_dependency_health)
```

New:
```promql
group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health)
```

- [ ] Update topology edges query — `group by` with `name` and `critical`
- [ ] Update health state query — key by `{name, host, port}` instead of `{job, host, port}`
- [ ] Update latency query — key by `{name, host, port}`
- [ ] Update `parseEdgeValues()` — extract `name` and `critical` from metric labels
- [ ] Parse `critical`: `"yes"` → `true`, `"no"` → `false`

### Task 10.2.3: Update `internal/topology/graph.go`

- [ ] Rename `EdgeKey.Job` → `EdgeKey.Name`
- [ ] Service node ID = `name` label value (was `job`)
- [ ] Set `edge.Critical` from parsed metric
- [ ] Update `buildGraph()` to use `name` label for source nodes
- [ ] Update deduplication logic — key by `{name, host, port}`

### Task 10.2.4: Update topology tests

- [ ] `prometheus_test.go` — update mock Prometheus responses (add `name`, `critical`)
- [ ] `graph_test.go` — update test data, add assertions for `critical` field
- [ ] Run `go test ./internal/topology/...`

### Task 10.2.5: Update frontend

- [ ] Display `critical` flag on edges (e.g. visual distinction: thick/thin line, color)
- [ ] Add `critical` to edge tooltip/info panel
- [ ] Optionally: add `critical` filter in Tom Select filters

### Task 10.2.6: Update documentation

- [ ] Update `docs/application-design.md` — new PromQL queries, new label format
- [ ] Update metric format tables

---

## Phase 10.3: Update Helm charts

### Task 10.3.1: Update `deployment.yml` template

Replace ConfigMap volume mount with env vars:

```yaml
env:
  - name: DEPHEALTH_NAME
    value: {{ $instance.name }}
  - name: LISTEN_ADDR
    value: ":8080"
  - name: DEPHEALTH_CHECK_INTERVAL
    value: {{ $.Values.checkInterval | quote }}
  - name: DEPHEALTH_DEPS
    value: {{ $instance.deps | quote }}
  {{- range $instance.connections }}
  - name: DEPHEALTH_{{ envName .name }}_URL
    value: {{ .url | quote }}
  - name: DEPHEALTH_{{ envName .name }}_CRITICAL
    value: {{ .critical | default "yes" | quote }}
  {{- end }}
  - name: POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
  - name: NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
```

- [ ] Remove volume and volumeMount for ConfigMap
- [ ] Add env var block with DEPHEALTH_* variables
- [ ] Generate `DEPHEALTH_DEPS` from connections list

### Task 10.3.2: Delete `configmap.yml` template

- [ ] Remove `templates/configmap.yml` — no longer needed

### Task 10.3.3: Add Helm helper for env name conversion

In `_helpers.tpl`, add helper to convert `uniproxy-02` → `UNIPROXY_02`:

```
{{- define "dephealth-uniproxy.envName" -}}
{{- . | upper | replace "-" "_" -}}
{{- end -}}
```

- [ ] Add `envName` helper template
- [ ] Use in deployment template

### Task 10.3.4: Update instance files

Update `instances/ns1-homelab.yaml`:

```yaml
instances:
  - name: uniproxy-01
    replicas: 2
    nodePort: 30080
    connections:
      - name: uniproxy-02
        type: http
        url: "http://uniproxy-02.dephealth-uniproxy.svc:8080"
        critical: "yes"
      - name: uniproxy-03
        type: http
        url: "http://uniproxy-03.dephealth-uniproxy.svc:8080"
        critical: "yes"

  - name: uniproxy-02
    replicas: 2
    connections:
      - name: redis
        type: redis
        url: "redis://redis.dephealth-redis.svc:6379"
        critical: "no"
      - name: grpc-stub
        type: grpc
        host: "grpc-stub.dephealth-grpc-stub.svc"
        port: "9090"
        critical: "no"
      - name: uniproxy-04
        type: http
        url: "http://uniproxy-04.dephealth-uniproxy-2.svc:8080"
        critical: "yes"

  - name: uniproxy-03
    replicas: 3
    connections:
      - name: postgresql
        type: postgres
        url: "postgres://dephealth:dephealth-test-pass@postgresql.dephealth-postgresql.svc:5432/dephealth"
        critical: "yes"
```

- [ ] Update `instances/ns1-homelab.yaml` — add `url`, `critical`, remove `host`/`port` where URL used
- [ ] Update `instances/ns2-homelab.yaml` — same format

### Task 10.3.5: Update `values.yaml` and `values-homelab.yaml`

- [ ] Update default image tag
- [ ] Review/simplify checkInterval handling

---

## Phase 10.4: Build and Deploy

### Task 10.4.1: Build uniproxy image

```bash
make uniproxy-build TAG=v0.2.0
```

- [ ] Build multi-arch image (amd64 + arm64)
- [ ] Push to `harbor.kryukov.lan/library/uniproxy:v0.2.0`
- [ ] Verify image in registry

### Task 10.4.2: Build dephealth-ui image

```bash
make docker-build TAG=v0.8.0
```

- [ ] Build multi-arch image
- [ ] Push to `harbor.kryukov.lan/library/dephealth-ui:v0.8.0`
- [ ] Verify image in registry

### Task 10.4.3: Deploy test environment

```bash
make env-undeploy
make env-deploy
```

- [ ] Undeploy existing environment
- [ ] Deploy with new image versions
- [ ] Wait for all pods to become Ready

### Task 10.4.4: Verify pods

```bash
make env-status
```

- [ ] All uniproxy pods Running in dephealth-uniproxy
- [ ] All uniproxy pods Running in dephealth-uniproxy-2
- [ ] dephealth-ui pod Running
- [ ] No CrashLoopBackOff or error states

---

## Phase 10.5: Verification

### Task 10.5.1: Verify uniproxy metrics format

```bash
kubectl port-forward -n dephealth-uniproxy svc/uniproxy-01 8080:8080
curl -s http://localhost:8080/metrics | grep app_dependency_health
```

Expected:
```
app_dependency_health{name="uniproxy-01",dependency="uniproxy-02",type="http",host="uniproxy-02.dephealth-uniproxy.svc",port="8080",critical="yes"} 1
```

- [ ] `name` label present and correct
- [ ] `critical` label present (`yes` or `no`)
- [ ] `hostname` label absent (removed)
- [ ] Both `app_dependency_health` and `app_dependency_latency_seconds` present

### Task 10.5.2: Verify VictoriaMetrics scraping

Check that metrics appear in VictoriaMetrics with correct labels:

```promql
group by (name, dependency, type, host, port, critical) (app_dependency_health)
```

- [ ] All 8 uniproxy instances report metrics
- [ ] Labels match expected format
- [ ] Latency histogram data present

### Task 10.5.3: Verify topology graph in dephealth-ui

Open `https://dephealth.kryukov.lan`:

- [ ] All uniproxy nodes visible with correct names (uniproxy-01..08)
- [ ] Dependency nodes visible (redis, postgresql, grpc-stub)
- [ ] Edges between correct source/target pairs
- [ ] Health states displayed (OK/DEGRADED/DOWN)
- [ ] Latency values on edges
- [ ] Critical flag visible on edges

### Task 10.5.4: Test failure scenarios

- [ ] Scale down `uniproxy-02` → `uniproxy-01` edge to `uniproxy-02` goes DOWN
- [ ] Verify `uniproxy-01` state becomes DEGRADED
- [ ] Scale back up → state recovers to OK
- [ ] Verify cross-namespace edges (uniproxy → uniproxy-2) work correctly

---

## Dependency Graph

```
Phase 10.1 (uniproxy rewrite) ─────────┐
                                        │
Phase 10.2 (dephealth-ui topology) ─────┼──► Phase 10.4 (build & deploy) ──► Phase 10.5 (verify)
                                        │
Phase 10.3 (Helm charts) ──────────────┘
```

Phases 10.1, 10.2, 10.3 can be developed in parallel.
Phase 10.4 requires all three to be complete.
Phase 10.5 requires Phase 10.4.

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|-----------|
| SDK module path or version unavailable | Build failure | Check `sdk-go/go.mod`; use `replace` directive if needed |
| VictoriaMetrics pod SD doesn't preserve `name` label | Missing label in queries | Check relabeling config, verify metric labels after scrape |
| Old cached metrics in VictoriaMetrics | Stale graph data | Wait for scrape interval + TTL, or restart VM |
| `critical` label breaks existing PromQL aggregations | Incorrect grouping | Test queries in VM UI before deploying UI changes |
| gRPC checker in SDK requires TLS or specific proto | Check fails | Use `FromParams()` for gRPC, verify SDK gRPC checker behavior |

---

## Files Summary

| Phase | Modified | Deleted | Notes |
|-------|----------|---------|-------|
| 10.1 | `go.mod`, `main.go`, `internal/config/config.go`, `internal/server/server.go`, `Dockerfile` | `internal/checker/*` (5 files), `internal/metrics/metrics.go` | Uniproxy rewrite |
| 10.2 | `models.go`, `prometheus.go`, `graph.go`, `*_test.go`, frontend JS/CSS, `application-design.md` | — | Topology updates |
| 10.3 | `deployment.yml`, `_helpers.tpl`, `values.yaml`, `values-homelab.yaml`, `ns1-homelab.yaml`, `ns2-homelab.yaml` | `configmap.yml` | Helm updates |
| 10.4 | — | — | Build & deploy only |
| 10.5 | — | — | Verification only |
| **Total** | **~17 files** | **~7 files** | |
