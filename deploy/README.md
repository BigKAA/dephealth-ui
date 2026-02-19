# Test Environment

**Language:** English | [Русский](./README.ru.md)

---

## Overview

This directory contains everything needed to deploy a complete **dephealth-ui** test environment: infrastructure services, test microservices, monitoring stack, and the application itself.

The test environment creates a realistic microservice topology with multiple namespaces, dependency types (HTTP, gRPC, PostgreSQL, Redis), authentication scenarios, and a bare metal host — allowing full end-to-end testing of dephealth-ui visualization capabilities.

> **Important:** All hostnames, IP addresses, and registry URLs in `values-homelab.yaml` files are specific to the author's home lab. You **must** adapt them to your own environment. See [Adapting for Your Environment](#adapting-for-your-environment).

---

## Prerequisites

### Required Software

| Tool | Version | Purpose |
| ------ | --------- | --------- |
| **Kubernetes** | 1.28+ | Cluster with `kubectl` access |
| **Helm** | 3.0+ | Chart deployment |
| **Docker** | 24+ | Container builds, bare metal deployment |
| **docker buildx** | 0.10+ | Multi-arch builds (amd64/arm64) |
| **SSH** | any | Bare metal host deployment |
| **make** | any | Build automation |

### Kubernetes Cluster Requirements

- **Gateway API** (preferred) or Ingress controller
- **StorageClass** for persistent volumes (e.g. `nfs-client`, `local-path`)
- **MetalLB** or equivalent LoadBalancer provider (for bare metal clusters)
- **cert-manager** with a `ClusterIssuer` (for TLS certificates)
- Network connectivity from cluster pods to bare metal hosts (for external scraping)

### Container Registry

The test environment pulls images from a private Harbor registry. You need either:

- Your own container registry, or
- Direct access to Docker Hub (modify `values.yaml` to use default registries)

---

## Directory Structure

```text
deploy/
├── docker/
│   └── uniproxy-pr1/             # Bare metal host deployment
│       └── docker-compose.yaml   # uniproxy + PostgreSQL + Redis
├── helm/
│   ├── dephealth-infra/          # Infrastructure services
│   │   ├── values.yaml           # Default values
│   │   └── values-homelab.yaml   # Home lab overrides
│   ├── dephealth-monitoring/     # Monitoring stack
│   │   ├── dashboards/           # Grafana dashboard JSONs
│   │   ├── values.yaml           # Default values
│   │   └── values-homelab.yaml   # Home lab overrides
│   ├── dephealth-ui/             # Application chart
│   │   ├── README.md             # Helm chart documentation
│   │   ├── values.yaml           # Default values
│   │   └── values-homelab.yaml   # Home lab overrides
│   └── dephealth-uniproxy/       # Test proxy instances
│       ├── instances/
│       │   ├── ns1-homelab.yaml  # NS1 topology (3 instances)
│       │   └── ns2-homelab.yaml  # NS2 topology (5 instances, auth scenarios)
│       ├── values.yaml           # Default values
│       └── values-homelab.yaml   # Home lab overrides
└── k8s-dev/
    └── dephealth-ui.yaml         # Raw K8s manifest (development)
```

---

## Helm Charts

### dephealth-infra

Infrastructure services shared by test microservices.

| Service | Namespace | Default | Description |
| --------- | ----------- | --------- | ------------- |
| PostgreSQL | `dephealth-postgresql` | Enabled | v17-alpine, credentials: `dephealth/dephealth-test-pass` |
| Redis | `dephealth-redis` | Enabled | v7-alpine, in-memory cache |
| gRPC Stub | `dephealth-grpc-stub` | Enabled | Simple gRPC health check responder |
| Dex (OIDC) | `dephealth-test` | Disabled | OIDC identity provider for auth testing |
| Kafka | — | Disabled | Reserved for future use |
| RabbitMQ | — | Disabled | Reserved for future use |

### dephealth-uniproxy

[uniproxy](https://github.com/BigKAA/uniproxy) test proxy instances built with [dephealth SDK](https://github.com/BigKAA/topologymetrics). Creates a multi-service topology across two namespaces.

**Namespace 1** (`dephealth-uniproxy`):

| Instance | Replicas | Dependencies | Notes |
| ---------- | ---------- | ------------- | ------- |
| uniproxy-01 | 2 | uniproxy-02 (critical), uniproxy-03 (critical) | Entry point, NodePort 30080 |
| uniproxy-02 | 2 | redis, grpc-stub, uniproxy-04 (critical), uniproxy-pr1 (critical) | Cross-namespace + bare metal |
| uniproxy-03 | 3 | postgresql (critical) | Database dependency |

**Namespace 2** (`dephealth-uniproxy-2`) — authentication test scenarios:

| Instance | Replicas | Dependencies | Auth |
| ---------- | ---------- | ------------- | ------ |
| uniproxy-04 | 2 | uniproxy-05 (Bearer), uniproxy-06 | Client auth |
| uniproxy-05 | 1 | — | Server: Bearer token |
| uniproxy-06 | 2 | uniproxy-07, uniproxy-08 (Basic), uniproxy-05 (wrong token) | Mixed auth |
| uniproxy-07 | 1 | postgresql (critical) | No auth |
| uniproxy-08 | 1 | postgresql (critical) | Server: Basic auth |

### dephealth-monitoring

Full monitoring stack for metrics collection and visualization.

| Component | Version | Description |
| ----------- | --------- | ------------- |
| VictoriaMetrics | v1.108.1 | Prometheus-compatible TSDB, 7d retention |
| VMAlert | v1.108.1 | Alert rule evaluation engine |
| AlertManager | v0.28.1 | Alert routing and grouping |
| Grafana | v11.6.0 | Dashboards (8 pre-built), admin/dephealth |

**Metrics collection:**

- Kubernetes pods auto-discovered via `prometheus.io/scrape=true` + `app.kubernetes.io/part-of=dephealth`
- External targets (bare metal hosts) via `victoriametrics.externalTargets` in values

### dephealth-ui

The application itself. See [deploy/helm/dephealth-ui/README.md](./helm/dephealth-ui/README.md) for detailed chart documentation.

---

## Bare Metal Host

The `deploy/docker/uniproxy-pr1/` directory contains a Docker Compose setup for running `uniproxy-pr1` on a physical host outside the Kubernetes cluster. This tests dephealth-ui's ability to visualize mixed K8s + bare metal topologies.

**Services:**

- `uniproxy-pr1` — test proxy with dephealth SDK (port 8080)
- `postgresql` — local PostgreSQL 17 (critical dependency)
- `redis` — local Redis 7 (non-critical dependency)

**Prerequisites for the host:**

- Docker with Compose plugin
- Network access from the K8s cluster (for Prometheus scraping)
- Trust to the private CA certificate (if using a private registry)

---

## Test Topology

```text
                           ┌─ NS: dephealth-uniproxy ──────────────────────────────────┐
                           │                                                            │
                           │  uniproxy-01 ──critical──► uniproxy-02 ──► redis           │
                           │       │                         │          ──► grpc-stub    │
                           │       │                         │                           │
                           │       └──critical──► uniproxy-03 ──critical──► postgresql   │
                           │                         │                                   │
                           └─────────────────────────┼───────────────────────────────────┘
                                                     │
                              ┌───────────────────critical────────────────────────┐
                              │                      │                            │
                              ▼                      ▼                            │
┌─ NS: dephealth-uniproxy-2 ─────────────────────────────────────────────────┐   │
│                                                                             │   │
│  uniproxy-04 ──Bearer──► uniproxy-05 ◄──wrong token── uniproxy-06         │   │
│       │                                                     │    │          │   │
│       └──critical──► uniproxy-06 ──► uniproxy-07 ──► postgresql │          │   │
│                                      ──Basic──► uniproxy-08 ──► postgresql │   │
│                                                                             │   │
└─────────────────────────────────────────────────────────────────────────────┘   │
                                                                                  │
┌─ Host: 192.168.218.168 (NS: hostpr1) ──────────────────────────────────────┐   │
│                                                                             │   │
│  uniproxy-pr1 ──critical──► postgresql                              ◄──────┘   │
│       │                                                                     │
│       └──────────────────► redis                                            │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Quick Start

### Deploy Everything

```bash
make env-deploy    # Infrastructure + uniproxy + bare metal host + monitoring
make helm-deploy   # dephealth-ui application
```

### Check Status

```bash
make env-status    # All namespaces + bare metal host
```

### Tear Down

```bash
make helm-undeploy  # Remove dephealth-ui
make env-undeploy   # Remove all test components
```

### Individual Components

```bash
make uniproxy-deploy    # Deploy/upgrade uniproxy instances only
make uniproxy-undeploy  # Remove uniproxy instances

make host-deploy        # Deploy Docker Compose on bare metal host
make host-undeploy      # Stop Docker Compose on bare metal host
make host-status        # Check containers on bare metal host
```

---

## Adapting for Your Environment

The `values-homelab.yaml` files contain environment-specific settings. To use your own infrastructure, create your own override files or modify the existing ones.

### 1. Container Registry

**Release:** `container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst` (Yandex Container Registry)

**Development:** `harbor.kryukov.lan/library` (images), `harbor.kryukov.lan/docker` (Docker Hub proxy)

**Options:**

- Use Docker Hub directly: remove `global.imageRegistry` overrides from `values-homelab.yaml`
- Use your own registry: set `global.pushRegistry` to your registry URL
- For private registries with self-signed CA: install the CA certificate on all K8s nodes and bare metal hosts

### 2. Storage Class

**Current:** `nfs-client`

Replace with your cluster's StorageClass:

```yaml
global:
  storageClass: "local-path"  # or "standard", "gp3", etc.
```

### 3. DNS and Hostnames

**Current hostnames** (must resolve to your cluster's Gateway/LoadBalancer IP):

| Hostname | Service | Port |
| ---------- | --------- | ------ |
| `dephealth.kryukov.lan` | dephealth-ui | HTTPS |
| `grafana.kryukov.lan` | Grafana | HTTP |
| `dex.kryukov.lan` | Dex OIDC (optional) | HTTPS |

**To adapt:**

1. Choose your own domain (e.g., `dephealth.mylab.local`)
2. Update all `values-homelab.yaml` files with new hostnames
3. Add DNS records pointing to your Gateway/LoadBalancer IP
4. Or add entries to `/etc/hosts` on your development machine

### 4. Gateway API

**Current:** Envoy Gateway (`eg` in `envoy-gateway-system` namespace)

If you use a different Gateway controller or Ingress:

- Modify `route.gateway.name` and `route.gateway.namespace` in values
- Or switch to Ingress: set `ingress.enabled=true` and `route.enabled=false`

### 5. Bare Metal Host

**Current:** `192.168.218.168` (Rocky Linux, SSH as root)

To use your own host:

```bash
make host-deploy HOST_PR1_IP=10.0.0.50
```

Or set permanently in Makefile:

```makefile
HOST_PR1_IP ?= 10.0.0.50
```

**Requirements for the host:**

- Docker with Compose plugin installed
- SSH access (key-based recommended)
- Port 8080 accessible from the K8s cluster
- Access to your container registry (with CA trust if needed)

### 6. TLS Certificates

**Current:** cert-manager with `ClusterIssuer: dev-ca-issuer` (self-signed CA)

For your environment:

- Use cert-manager with Let's Encrypt for public clusters
- Use your own CA and configure `customCA` in dephealth-ui values
- Or disable TLS for development (not recommended)

### Example: Minimal Custom Configuration

Create `values-myenv.yaml` for each chart:

```yaml
# dephealth-infra/values-myenv.yaml
global:
  storageClass: "local-path"

# dephealth-monitoring/values-myenv.yaml
global:
  storageClass: "local-path"
grafana:
  rootUrl: "http://grafana.mylab.local"
  route:
    enabled: true
    hostname: grafana.mylab.local
    gateway:
      name: my-gateway
      namespace: gateway-system

# dephealth-ui/values-myenv.yaml
image:
  registry: "my-registry.example.com"
  tag: "v0.16.0"
route:
  enabled: true
  hostname: dephealth.mylab.local
  gateway:
    name: my-gateway
    namespace: gateway-system
config:
  grafana:
    baseUrl: "http://grafana.mylab.local"
```

Then deploy:

```bash
helm upgrade --install dephealth-infra deploy/helm/dephealth-infra -f deploy/helm/dephealth-infra/values-myenv.yaml
```

---

## Troubleshooting

### Pods stuck in ImagePullBackOff

Your cluster cannot access the container registry. Check:

- Registry URL in `values-homelab.yaml` (or your override)
- Network connectivity from cluster nodes to registry
- CA certificate trust (for private registries with self-signed certificates)

### VictoriaMetrics not scraping external targets

After updating the monitoring Helm chart, VictoriaMetrics needs a pod restart to reload the scrape config:

```bash
kubectl delete pod victoriametrics-0 -n dephealth-monitoring
```

### Bare metal host: TLS certificate error

Docker on the host does not trust your private CA. Install the CA certificate:

```bash
# Rocky Linux / CentOS / RHEL
scp ca.crt root@<HOST_IP>:/etc/pki/ca-trust/source/anchors/
ssh root@<HOST_IP> 'update-ca-trust && systemctl restart docker'

# Ubuntu / Debian
scp ca.crt root@<HOST_IP>:/usr/local/share/ca-certificates/
ssh root@<HOST_IP> 'update-ca-certificates && systemctl restart docker'
```

### dephealth-ui shows no topology

1. Check VictoriaMetrics has metrics: `curl http://victoriametrics:8428/api/v1/query?query=app_dependency_health`
2. Check dephealth-ui can reach VictoriaMetrics: verify `config.datasources.prometheus.url` in values
3. Check uniproxy pods are running: `make env-status`

### Custom CA for dephealth-ui

Before deploying dephealth-ui, create the ConfigMap with your CA certificate:

```bash
kubectl create configmap custom-ca \
  --from-file=ca.crt=/path/to/your/ca.crt \
  -n dephealth-ui
```

---

## Related Documentation

| Document | Description |
| ---------- | ------------- |
| [Helm Chart Guide](./helm/dephealth-ui/README.md) | dephealth-ui Kubernetes deployment |
| [Metrics Specification](../docs/METRICS.md) | Required Prometheus metrics format |
| [API Reference](../docs/API.md) | REST API endpoints |
| [Application Design](../docs/application-design.md) | Architecture and design decisions |
