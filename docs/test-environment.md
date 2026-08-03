# Test Environment Guide

**Language:** English | [Русский](./test-environment.ru.md)

---

## Overview

This guide explains how to deploy a complete, realistic test environment for
**dephealth-ui** on **any Kubernetes cluster**, with no dependency on a specific
home lab. The environment lets you explore topology visualization, dependency
health, authentication scenarios, and alert enrichment end to end.

> A separate, home-lab-specific guide lives in [../deploy/README.md](../deploy/README.md).
> This document is its portable counterpart: every hostname, IP, and registry
> reference is either a placeholder or a publicly readable registry.

### How the pieces fit together

dephealth-ui is a read-only visualization layer. It never talks to
[uniproxy](https://github.com/BigKAA/uniproxy) directly. Instead, uniproxy
instances export Prometheus metrics, a metrics store scrapes them, and
dephealth-ui queries that store with PromQL:

```text
                              scrape                          PromQL
  uniproxy pods  ─────────────────────▶  VictoriaMetrics  ─────────────▶  dephealth-ui  ──▶  Browser
  :8080/metrics      (annotations +      :8428                          :8080 (UI + API)
                      label discovery)
```

Because the data flow is one-way, the four components deploy independently and
only need to agree on the **metrics format** (the `app_dependency_*` family) and
the **scrape configuration** (covered below).

### Components deployed by this guide

| Component | Helm chart | Purpose |
| --------- | ---------- | ------- |
| **dephealth-infra** | `deploy/helm/dephealth-infra` | Real dependencies for the test topology: PostgreSQL, Redis, gRPC stub, LDAP (389ds), and an nginx reverse proxy for the Host-routing scenario |
| **dephealth-monitoring** | `deploy/helm/dephealth-monitoring` | VictoriaMetrics (metrics store + scraper), VMAlert, AlertManager, Grafana |
| **dephealth-uniproxy** | `deploy/helm/dephealth-uniproxy` | Twelve uniproxy instances across three namespaces forming a dependency graph |
| **dephealth-ui** | `deploy/helm/dephealth-ui` | The visualization application (Go backend + embedded SPA) |

---

## Prerequisites

### Tools

| Tool | Version | Purpose |
| ---- | ------- | ------- |
| **kubectl** | any, matching your cluster | Cluster access |
| **Helm** | 3.0+ | Chart deployment |
| **Docker** | 24+ (optional) | Only needed if you build custom images yourself |
| **curl** | any | Verification steps |

### Cluster requirements

- **Kubernetes 1.28+** with working `kubectl` access.
- A **default StorageClass** (or set `global.storageClass` in the overrides to a
  specific class). VictoriaMetrics, PostgreSQL, and 389ds need PersistentVolumes.
- Outbound internet access from cluster nodes to pull images (Docker Hub and the
  Yandex Container Registry), **or** mirror the images into a registry the
  cluster can reach.
- (Optional) a **LoadBalancer** provider (MetalLB, cloud LB) and/or
  **Gateway API** / an **Ingress controller** if you want external access
  instead of `kubectl port-forward`.

---

## Container Images

The environment mixes public images and three custom images.

### Public images (Docker Hub, no configuration needed)

| Component | Image |
| --------- | ----- |
| PostgreSQL | `postgres:17-alpine` |
| Redis | `redis:7-alpine` |
| 389ds (LDAP) | `389ds/dirsrv:3.1` |
| nginx proxy | `nginx:1.27-alpine` |
| VictoriaMetrics | `victoriametrics/victoria-metrics:v1.108.1` |
| VMAlert | `victoriametrics/vmalert:v1.108.1` |
| AlertManager | `prom/alertmanager:v0.28.1` |
| Grafana | `grafana/grafana:11.6.0` |

### Custom images

| Component | Default source (publicly readable) | Build-from-source fallback |
| --------- | ---------------------------------- | -------------------------- |
| **dephealth-ui** | `container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst/dephealth-ui` | `docker buildx build -t <registry>/dephealth-ui:latest --push .` (from the `dephealth-ui/` repo) |
| **uniproxy** | `container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst/uniproxy` | build from the `uniproxy/` repo — see note below |
| **grpc-stub** | `container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst/dephealth-grpc-stub` | build from `topologymetrics/conformance/stubs/grpc-stub/` |

The quickstart override files already point at the Yandex Container Registry,
which is **publicly readable**, so pulling should work without authentication.

> **If the registry is unreachable** from your cluster (e.g. air-gapped
> environment), build the three custom images and push them to a registry your
> cluster can reach, then:
>
> - For **dephealth-ui**: edit `deploy/quickstart/values-dephealth-ui.yaml`
>   and set `image.name`.
> - For **uniproxy** and **grpc-stub**: edit the `global.pushRegistry` field in
>   `deploy/quickstart/values-uniproxy.yaml` and `deploy/quickstart/values-infra.yaml`.
>
> Building **uniproxy** from source: its `Dockerfile` references a private base
> image mirror. Replace the two `FROM` lines with the public equivalents before
> building:
>
> ```dockerfile
> FROM golang:1.25.8-alpine AS builder
> ...
> FROM alpine:3.21
> ```

---

## Deployment

> All commands assume you run them from the **`dephealth-ui/` repository root**.
> The quickstart override files live in `deploy/quickstart/`.

### Step 1 — Infrastructure dependencies

Deploy PostgreSQL, Redis, the gRPC stub, and LDAP. These are the real backing
services that uniproxy instances will health-check.

```bash
helm upgrade --install dephealth-infra deploy/helm/dephealth-infra \
  -f deploy/quickstart/values-infra.yaml
```

Wait for the pods to become ready:

```bash
kubectl get pods -n dephealth-postgresql
kubectl get pods -n dephealth-redis
kubectl get pods -n dephealth-grpc-stub
kubectl get pods -n dephealth-389ds
```

### Step 2 — Monitoring stack

Deploy VictoriaMetrics (the metrics store + scraper), VMAlert, AlertManager,
and Grafana.

```bash
helm upgrade --install dephealth-monitoring deploy/helm/dephealth-monitoring \
  -f deploy/quickstart/values-monitoring.yaml \
  -n dephealth-monitoring --create-namespace
```

```bash
kubectl get pods -n dephealth-monitoring
```

**How scraping works (read this once):** VictoriaMetrics auto-discovers uniproxy
pods using Kubernetes pod discovery, keeping only pods that have **both**:

1. the annotation `prometheus.io/scrape: "true"`, and
2. the label `app.kubernetes.io/part-of: dephealth`.

The uniproxy Helm chart sets both automatically on every instance, so no extra
scrape configuration is needed. The scrape path and port come from the pod
annotations (`/metrics` on `8080`).

### Step 3 — uniproxy instances (the test topology)

Deploy the twelve uniproxy instances across three namespaces. They form the
dependency graph that dephealth-ui will visualize.

```bash
# Namespace 1 — three instances (entry point + chain)
helm upgrade --install uniproxy-ns1 deploy/helm/dephealth-uniproxy \
  -f deploy/quickstart/values-uniproxy.yaml \
  -f deploy/quickstart/instances/ns1.yaml \
  -n dephealth-uniproxy --create-namespace

# Namespace 2 — five instances (authentication scenarios + proxy initiator)
helm upgrade --install uniproxy-ns2 deploy/helm/dephealth-uniproxy \
  -f deploy/quickstart/values-uniproxy.yaml \
  -f deploy/quickstart/instances/ns2.yaml \
  -n dephealth-uniproxy-2 --create-namespace

# Namespace 3 — four instances behind the nginx reverse proxy
helm upgrade --install uniproxy-ns3 deploy/helm/dephealth-uniproxy \
  -f deploy/quickstart/values-uniproxy.yaml \
  -f deploy/quickstart/instances/ns3.yaml \
  -n dephealth-uniproxy-3 --create-namespace
```

```bash
kubectl get pods -n dephealth-uniproxy
kubectl get pods -n dephealth-uniproxy-2
kubectl get pods -n dephealth-uniproxy-3
```

Metrics start appearing in VictoriaMetrics within ~15–30 seconds (one scrape
interval plus check interval).

### Step 4 — dephealth-ui

Deploy the visualization application.

```bash
helm upgrade --install dephealth-ui deploy/helm/dephealth-ui \
  -f deploy/quickstart/values-dephealth-ui.yaml \
  -n dephealth-ui --create-namespace
```

```bash
kubectl get pods -n dephealth-ui
```

### Step 5 — Open the UI

The quickstart disables external routes for portability. Use port-forwarding:

```bash
kubectl port-forward -n dephealth-ui svc/dephealth-ui 8080:8080
```

Then open `http://localhost:8080` in your browser.

---

## Verify

### 1. Metrics are flowing into VictoriaMetrics

Forward the VictoriaMetrics port and query it:

```bash
kubectl port-forward -n dephealth-monitoring svc/victoriametrics 8428:8428 &
```

```bash
# Should return one series per dependency (non-empty "data.result")
curl -s 'http://localhost:8428/api/v1/query?query=app_dependency_health' | jq '.data.result | length'
```

If the count is `0`, see [Troubleshooting](#troubleshooting).

### 2. uniproxy endpoints respond

Forward the entry-point service (NodePort 30080 is mapped to `uniproxy-01`):

```bash
kubectl port-forward -n dephealth-uniproxy svc/uniproxy-01 8081:8080
curl http://localhost:8081/                        # status JSON
curl "http://localhost:8081/?detail=true&depth=2"  # recursive detail
curl http://localhost:8081/metrics | grep app_dependency_health
```

### 3. dephealth-ui shows the topology

Open `http://localhost:8080`. You should see a graph of nodes (services) and
edges (dependencies). Healthy dependencies are green; failing ones are red.

For the JSON the UI consumes:

```bash
curl -s http://localhost:8080/api/v1/topology | jq '.nodes | length'
```

---

## Test Topology

```text
┌─ Namespace: dephealth-uniproxy ─────────────────────────────────────┐
│                                                                     │
│  uniproxy-01 ──critical──► uniproxy-02 ──► redis                    │
│  (entry, NodePort 30080)    │           ──► grpc-stub               │
│       │                     │                                       │
│       └──critical──► uniproxy-03 ──critical──► postgresql           │
│                          │  └──► ldap (389ds)                       │
└──────────────────────────┼──────────────────────────────────────────┘
                           │ cross-namespace
                           ▼
┌─ Namespace: dephealth-uniproxy-2 (auth scenarios) ──────────────────┐
│                                                                     │
│  uniproxy-04 ──Bearer──► uniproxy-05 ◄──wrong token── uniproxy-06   │
│       │                                  │  │  │  │   │           │
│       └──► uniproxy-06 ──► uniproxy-07 ──► postgresql│   │           │
│                          ──Basic──► uniproxy-08 ──► postgresql      │
│                          │  │  │  (Host header routing)             │
└──────────────────────────┼──┼──┼──┼─────────────────────────────────┘
                           │  │  │
              ┌────────────┘  │  └──────────┐ cross-namespace (one proxy)
              ▼               ▼             ▼
┌─ Namespace: dephealth-uniproxy-3 (reverse-proxy scenario) ───────────┐
│                                                                     │
│                  nginx-proxy (single host:port)                      │
│   ──Host: uniproxy-09──► uniproxy-09                                 │
│   ──Host: uniproxy-10──► uniproxy-10                                 │
│   ──Host: uniproxy-11──► uniproxy-11                                 │
│   ──Host: uniproxy-12──► uniproxy-12                                 │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

Highlights this topology exercises:

- **HTTP chain** with recursion (`uniproxy-01 → 02 → 03 → 01`).
- **Cross-namespace** link (`uniproxy-02 → uniproxy-04`).
- **Mixed dependency types**: HTTP, gRPC, PostgreSQL, Redis, LDAP.
- **Critical vs. non-critical** edges.
- **Authentication**: working Bearer token, working Basic auth, and an
  **intentional wrong token** (`uniproxy-06 → uniproxy-05`) so an `auth_error`
  status is visible in the UI.
- **Server-side auth** (`uniproxy-05`, `uniproxy-08`) with `/metrics` left open.
- **Reverse-proxy / Host-header routing** (`uniproxy-05 → nginx-proxy →
  uniproxy-09..12`): four dependencies share one `host:port` and are
  distinguished only by their `Host` header (see [Proxy scenario](#proxy-scenario)).

### Proxy scenario

`uniproxy-05` (ns2) checks four services that all sit behind a single nginx
reverse proxy (`nginx-proxy`, ns3). Because the four targets share the same
TCP destination, the connection list uses `host`/`port` plus a per-connection
`hostHeader` rather than distinct `url`s:

```yaml
# uniproxy-05 connections (excerpt) — deploy/quickstart/instances/ns2.yaml
- name: uniproxy-09
  type: http
  host: "nginx-proxy.dephealth-uniproxy-3.svc"   # same destination for all four
  port: "80"
  hostHeader: "uniproxy-09"                       # selects the backend on nginx
  critical: "yes"
  healthPath: "/"
# ... uniproxy-10, uniproxy-11, uniproxy-12 differ only in name + hostHeader
```

- `host`/`port` are the proxy address — the TCP destination of every check.
- `hostHeader` sets the HTTP `Host` header on the outbound request
  (`DEPHEALTH_<NAME>_HOST_HEADER` → `req.Host` in the SDK). nginx maps the
  inbound `Host` to one of the four backends (`uniproxy-09`..`uniproxy-12`).
- Do **not** put `Host` under `auth.headers`: Go's `net/http` ignores
  `req.Header.Set("Host", ...)`; only `hostHeader` (which sets `req.Host`)
  takes effect, and the SDK rejects combining the two.

This mirrors real-world topology where a shared ingress/proxy front several
apps: in the metrics the four series carry an identical `host:port` and differ
only in the `dependency` label — the shape shown for `nginx-back-app1` in
`tmp/metrics.txt`.

---

## Metrics format (reference)

dephealth-ui queries the dephealth SDK metric family. All four are exported by
uniproxy on `/metrics`:

| Metric | Type | Meaning |
| ------ | ---- | ------- |
| `app_dependency_health` | Gauge | `1` = healthy, `0` = unhealthy |
| `app_dependency_latency_seconds` | Histogram | Health-check latency |
| `app_dependency_status` | Gauge (enum) | Exactly one series = `1`: `ok`, `timeout`, `connection_error`, `dns_error`, `auth_error`, `tls_error`, `unhealthy`, `error` |
| `app_dependency_status_detail` | Gauge (info) | Always `1`; the human-readable reason is in the `detail` label |

Common labels: `name`, `group`, `namespace`, `dependency`, `type`, `host`,
`port`, `critical`. See [METRICS.md](./METRICS.md) for the full specification.

---

## Optional: Bare Metal Host

To test dephealth-ui's ability to visualize a topology that spans Kubernetes
**and** an external host, run an extra uniproxy instance on a VM/bare-metal
host and register it as an external scrape target.

1. On the host (with Docker installed), create a `docker-compose.yaml`:

   ```yaml
   services:
     uniproxy-pr1:
       image: container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst/uniproxy:v0.7.0
       ports:
         - "8080:8080"
       environment:
         DEPHEALTH_NAME: uniproxy-pr1
         DEPHEALTH_GROUP: infra-host
         DEPHEALTH_DEPS: "postgres:postgres,redis:redis"
         DEPHEALTH_POSTGRES_URL: "postgres://dephealth:dephealth-test-pass@<PG_HOST>:5432/dephealth"
         DEPHEALTH_POSTGRES_CRITICAL: "yes"
         DEPHEALTH_REDIS_URL: "redis://<REDIS_HOST>:6379"
         DEPHEALTH_REDIS_CRITICAL: "no"
   ```

2. Start it: `docker compose up -d`. Verify: `curl http://<HOST_IP>:8080/metrics`.

3. Make sure the host's port `8080` is reachable from the Kubernetes cluster
   (VictoriaMetrics will scrape it).

4. Register the target in the monitoring stack. Edit
   `deploy/quickstart/values-monitoring.yaml`:

   ```yaml
   victoriametrics:
     externalTargets:
       - jobName: uniproxy-pr1
         target: "<HOST_IP>:8080"
         labels:
           namespace: hostpr1
           service: uniproxy-pr1
   ```

5. Upgrade monitoring and restart VictoriaMetrics to reload the scrape config:

   ```bash
   helm upgrade --install dephealth-monitoring deploy/helm/dephealth-monitoring \
     -f deploy/quickstart/values-monitoring.yaml \
     -n dephealth-monitoring
   kubectl delete pod -n dephealth-monitoring -l app=victoriametrics
   ```

6. (Optional) Link a cluster-side uniproxy to it by uncommenting the
   `uniproxy-pr1` connection in `deploy/quickstart/instances/ns1.yaml` and
   re-running the ns1 upgrade command.

---

## Optional: External Access (Gateway / Ingress)

Instead of `kubectl port-forward`, expose dephealth-ui and Grafana externally.

### Via Ingress

```yaml
# deploy/quickstart/values-dephealth-ui.yaml
ingress:
  enabled: true
  className: "nginx"          # your Ingress controller
  hostname: dephealth.example.com
```

### Via Gateway API

```yaml
# deploy/quickstart/values-dephealth-ui.yaml
route:
  enabled: true
  hostname: dephealth.example.com
global:
  gateway:
    name: my-gateway
    namespace: gateway-system
```

Apply the change with the `helm upgrade` command from Step 4, then point DNS
(or `/etc/hosts`) for `dephealth.example.com` at your LoadBalancer/Gateway IP.

To also surface Grafana links in the UI, enable Grafana's route in
`values-monitoring.yaml` and set `config.grafana.baseUrl` in the dephealth-ui
overrides to the Grafana URL.

---

## Adapting to Your Environment

The quickstart overrides are intentionally minimal. Common adjustments:

| Need | What to change |
| ---- | -------------- |
| Specific StorageClass | `global.storageClass` in each quickstart values file |
| Private image registry | `global.pushRegistry` (infra, uniproxy) and `image.name` (dephealth-ui); mirror the custom images there |
| Different namespaces | The chart `global.namespace` values and the `.svc` FQDNs in `instances/ns1.yaml` / `ns2.yaml` must match |
| TLS for dephealth-ui | `tls.enabled`, or Ingress TLS — see [Helm chart docs](../deploy/helm/dephealth-ui/README.md) |
| Self-signed CA trust | Create a ConfigMap with the CA and set `customCA` in the dephealth-ui values |

For the full home-lab-specific reference (with concrete IPs, domains, and the
`make env-deploy` automation), see [../deploy/README.md](../deploy/README.md).

---

## Troubleshooting

### dephealth-ui shows an empty topology

Work through the data flow in order:

1. **Are uniproxy pods running and healthy?**

   ```bash
   kubectl get pods -n dephealth-uniproxy -n dephealth-uniproxy-2
   ```

   Pods not `Running`/`Ready` won't export metrics.

2. **Are metrics reaching VictoriaMetrics?**

   ```bash
   kubectl port-forward -n dephealth-monitoring svc/victoriametrics 8428:8428 &
   curl -s 'http://localhost:8428/api/v1/query?query=app_dependency_health' | jq '.data.result | length'
   ```

   Empty result means scraping isn't working — check the next item.

3. **Are pods selected by the scraper?** The pod needs **both** the annotation
   `prometheus.io/scrape=true` **and** the label `app.kubernetes.io/part-of=dephealth`.
   The chart sets them; verify with:

   ```bash
   kubectl get pod <pod> -n dephealth-uniproxy -o jsonpath='{.metadata.annotations}'
   kubectl get pod <pod> -n dephealth-uniproxy -o jsonpath='{.metadata.labels}'
   ```

4. **Can dephealth-ui reach VictoriaMetrics?** Check the configured URL:

   ```bash
   kubectl get configmap -n dephealth-ui dephealth-ui -o yaml | grep -A2 prometheus
   ```

   It must be `http://victoriametrics.dephealth-monitoring.svc:8428`.

5. **Check dephealth-ui logs for connection errors.**

   ```bash
   kubectl logs -n dephealth-ui -l app.kubernetes.io/name=dephealth-ui
   ```

### Pods stuck in `ImagePullBackOff`

The cluster cannot pull an image. Confirm:

- The image reference in the quickstart values is correct.
- The cluster can reach the registry (Docker Hub / Yandex CR) — check node
  egress and any proxy settings.
- For the custom images, either the Yandex CR is reachable, or you mirrored
  them into a registry the cluster can reach (see [Container Images](#container-images)).

### VictoriaMetrics not scraping external targets

After changing `victoriametrics.externalTargets`, VictoriaMetrics must reload its
scrape config:

```bash
kubectl delete pod -n dephealth-monitoring -l app=victoriametrics
```

### Wrong port assumption

uniproxy exposes **everything on a single port `:8080`** — including `/`,
`/healthz`, `/readyz`, and `/metrics`. There is no separate metrics port. The
scrape annotations in the chart already reflect this.

---

## Tear Down

Remove the components in reverse order (optional steps only if you ran them):

```bash
# Application
helm uninstall dephealth-ui -n dephealth-ui

# uniproxy instances
helm uninstall uniproxy-ns1 -n dephealth-uniproxy
helm uninstall uniproxy-ns2 -n dephealth-uniproxy-2

# Monitoring
helm uninstall dephealth-monitoring -n dephealth-monitoring

# Infrastructure
helm uninstall dephealth-infra

# Remove namespaces (ignores ones already gone)
kubectl delete namespace dephealth-ui dephealth-uniproxy dephealth-uniproxy-2 \
  dephealth-monitoring dephealth-postgresql dephealth-redis \
  dephealth-grpc-stub dephealth-389ds --ignore-not-found
```

Stop any bare-metal uniproxy with `docker compose down` on that host.

---

## Related Documentation

| Document | Description |
| -------- | ----------- |
| [../deploy/README.md](../deploy/README.md) | Home-lab-specific deployment guide with `make` automation |
| [METRICS.md](./METRICS.md) | Full Prometheus metrics specification |
| [API.md](./API.md) | dephealth-ui REST API reference |
| [application-design.md](./application-design.md) | Architecture and design decisions |
| [uniproxy README](https://github.com/BigKAA/uniproxy) | uniproxy configuration and use cases |
