# dephealth-ui Helm Chart

Helm chart for deploying dephealth-ui application to Kubernetes.

**Language:** English | [Русский](./README.ru.md)

## Installation

```bash
helm install dephealth-ui ./deploy/helm/dephealth-ui \
  -f ./deploy/helm/dephealth-ui/values.yaml \
  -n dephealth-ui --create-namespace
```

## Configuration

### Gateway API (HTTPRoute)

To use Gateway API for ingress traffic:

```yaml
route:
  enabled: true
  hostname: dephealth.example.com

tls:
  enabled: true
  issuerName: letsencrypt-prod
  issuerKind: ClusterIssuer

global:
  gateway:
    name: gateway
    namespace: gateway-system
```

This will create:
- `HTTPRoute` resource pointing to the configured Gateway
- `Certificate` resource (if `tls.enabled: true`) managed by cert-manager

### Ingress (Traditional)

To use traditional Kubernetes Ingress:

```yaml
route:
  enabled: false  # Disable Gateway API

ingress:
  enabled: true
  className: nginx  # Your Ingress controller class
  hostname: dephealth.example.com
  annotations:
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
```

#### Option 1: Use Existing TLS Secret

If you already have a TLS certificate in a Kubernetes Secret:

```yaml
ingress:
  enabled: true
  className: nginx
  hostname: dephealth.example.com
  tls:
    enabled: true
    secretName: my-existing-tls-secret  # Pre-created Secret
    certManager:
      enabled: false
```

The Secret must contain `tls.crt` and `tls.key` keys:

```bash
kubectl create secret tls my-existing-tls-secret \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  -n dephealth-ui
```

#### Option 2: Automatic Certificate with cert-manager

To automatically provision certificates via cert-manager:

```yaml
ingress:
  enabled: true
  className: nginx
  hostname: dephealth.example.com
  tls:
    enabled: true
    secretName: ""  # Leave empty
    certManager:
      enabled: true
      issuerName: letsencrypt-prod
      issuerKind: ClusterIssuer
```

This will create a `Certificate` resource that cert-manager will use to provision the TLS certificate automatically.

### OIDC Authentication

```yaml
config:
  auth:
    type: oidc
    oidc:
      issuer: "https://dex.example.com"
      clientId: "dephealth-ui"
      clientSecret: "base64-encoded-secret"
      redirectUrl: "https://dephealth.example.com/auth/callback"

customCA:
  enabled: true
  configMapName: custom-ca
  key: ca.crt
```

### Stale Node Retention

When a service stops sending metrics, it normally vanishes from the graph. Enable the lookback window to retain disappeared services with `state="unknown"` for a configurable duration:

```yaml
config:
  topology:
    lookback: "1h"  # Keep stale nodes for 1 hour (default: "0" = disabled)
```

Recommended values: `1h` (most environments), `6h` (infrequent deployments), `0` (disabled).

### Datasources

```yaml
config:
  datasources:
    prometheus:
      url: "http://victoriametrics.monitoring.svc:8428"
    alertmanager:
      url: "http://alertmanager.monitoring.svc:9093"
```

### Grafana Integration

Configure Grafana base URL and dashboard UIDs to enable direct links from the UI:

```yaml
config:
  grafana:
    baseUrl: "https://grafana.example.com"
    dashboards:
      cascadeOverview: "dephealth-cascade-overview"  # Cascade failure overview
      rootCause: "dephealth-root-cause"              # Root cause analyzer
      serviceStatus: "dephealth-service-status"      # Single service status
      linkStatus: "dephealth-link-status"            # Single dependency status
      serviceList: "dephealth-service-list"          # All services list
      servicesStatus: "dephealth-services-status"    # All services overview
      linksStatus: "dephealth-links-status"          # All links overview
```

If `grafana.baseUrl` is empty, Grafana links are hidden in the UI.

### Grafana Dashboard Availability Checking

When `grafana.baseUrl` is configured, the application checks dashboard availability at startup via Grafana API. Dashboards that are not found are automatically hidden from the UI.

To authenticate with Grafana API, create a Kubernetes Secret and reference it:

```bash
kubectl create secret generic grafana-creds \
  --from-literal=token="glsa_your_service_account_token" \
  -n dephealth-ui
```

```yaml
grafanaSecret:
  enabled: true
  secretName: grafana-creds
  tokenKey: token        # Key containing API token (priority over basic auth)
  usernameKey: username   # Key containing basic auth username
  passwordKey: password   # Key containing basic auth password
```

Authentication priority: **token** (Bearer) > **basic auth** (username/password) > **none**.

If Grafana is unreachable at startup, all dashboard links are hidden and a warning is logged.

### Grafana Dashboards

The `dephealth-monitoring` Helm chart includes 7 pre-built Grafana dashboards provisioned via ConfigMaps:

| Dashboard | UID | Description | Data Source |
|-----------|-----|-------------|-------------|
| Cascade Overview | `dephealth-cascade-overview` | Cascade failure overview with affected services and root causes | Infinity (API) |
| Root Cause Analyzer | `dephealth-root-cause` | Deep-dive root cause analysis with dependency failure graph | Infinity (API) + Prometheus |
| Service Status | `dephealth-service-status` | Single service health details and dependency timelines | Prometheus |
| Link Status | `dephealth-link-status` | Single dependency connection health and latency | Prometheus |
| Service List | `dephealth-service-list` | All services table with state and dependency count | Prometheus |
| Services Status | `dephealth-services-status` | All services overview with state timelines | Prometheus |
| Links Status | `dephealth-links-status` | All dependency links overview with health timelines | Prometheus |

> **Important: Plugin and API requirements**
>
> The **Cascade Overview** and **Root Cause Analyzer** dashboards require:
>
> 1. **Grafana Infinity datasource plugin** ([yesoreyeram-infinity-datasource](https://grafana.com/grafana/plugins/yesoreyeram-infinity-datasource/)) — must be installed in Grafana. Install via `GF_INSTALL_PLUGINS` environment variable or Grafana CLI.
> 2. **dephealth-ui API** — must be accessible from Grafana over the network (e.g., `http://dephealth-ui.dephealth-ui.svc:8080`). These dashboards call `/api/v1/cascade-analysis` and `/api/v1/cascade-graph` endpoints to fetch cascade failure data.
>
> The **Root Cause Analyzer** dashboard also includes a **Node Graph panel** that visualizes cascade dependency chains. This panel requires the Infinity datasource to be provisioned with UID `infinity` (or update `${DS_INFINITY}` variable in the dashboard JSON).
>
> Without the Infinity plugin or network access to the dephealth-ui API, these two dashboards will show empty panels. The remaining 5 dashboards use only Prometheus and work independently.

## Examples

See `values-homelab.yaml` for a complete homelab example with Gateway API.

See `values-ingress-example.yaml` for Ingress configuration examples.

## Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.pushRegistry` | Container registry for images | `""` |
| `global.namespace` | Target namespace | `dephealth-ui` |
| `image.name` | Image name | `dephealth-ui` |
| `image.tag` | Image tag | `latest` |
| `route.enabled` | Enable Gateway API HTTPRoute | `false` |
| `route.hostname` | Hostname for HTTPRoute | `dephealth.example.com` |
| `ingress.enabled` | Enable Ingress | `false` |
| `ingress.className` | Ingress class name | `""` |
| `ingress.hostname` | Hostname for Ingress | `dephealth.example.com` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.tls.enabled` | Enable TLS for Ingress | `false` |
| `ingress.tls.secretName` | Existing TLS secret name | `""` |
| `ingress.tls.certManager.enabled` | Generate cert via cert-manager | `false` |
| `ingress.tls.certManager.issuerName` | cert-manager Issuer name | `""` |
| `ingress.tls.certManager.issuerKind` | cert-manager Issuer kind | `ClusterIssuer` |
| `config.auth.type` | Auth type: `none`, `basic`, `oidc` | `none` |
| `config.datasources.prometheus.url` | Prometheus/VictoriaMetrics URL | `http://victoriametrics:8428` |
| `config.datasources.alertmanager.url` | AlertManager URL | `""` |
| `config.cache.ttl` | Cache TTL duration | `15s` |
| `config.topology.lookback` | Stale node retention window (`0` = disabled) | `"0"` |
| `config.grafana.baseUrl` | Grafana base URL (empty = links hidden) | `""` |
| `config.grafana.dashboards.cascadeOverview` | Cascade Overview dashboard UID | `dephealth-cascade-overview` |
| `config.grafana.dashboards.rootCause` | Root Cause Analyzer dashboard UID | `dephealth-root-cause` |
| `config.grafana.dashboards.serviceStatus` | Service Status dashboard UID | `dephealth-service-status` |
| `config.grafana.dashboards.linkStatus` | Link Status dashboard UID | `dephealth-link-status` |
| `config.grafana.dashboards.serviceList` | Service List dashboard UID | `dephealth-service-list` |
| `config.grafana.dashboards.servicesStatus` | Services Status dashboard UID | `dephealth-services-status` |
| `config.grafana.dashboards.linksStatus` | Links Status dashboard UID | `dephealth-links-status` |
| `customCA.enabled` | Mount custom CA certificate | `false` |
| `customCA.configMapName` | ConfigMap name with CA cert | `""` |
| `customCA.key` | Key in ConfigMap containing cert | `ca.crt` |
| `grafanaSecret.enabled` | Enable Grafana auth from Secret | `false` |
| `grafanaSecret.secretName` | Name of K8s Secret | `""` |
| `grafanaSecret.tokenKey` | Secret key for API token | `token` |
| `grafanaSecret.usernameKey` | Secret key for username | `username` |
| `grafanaSecret.passwordKey` | Secret key for password | `password` |

## Requirements

- Kubernetes 1.21+
- Helm 3.0+
- For Gateway API: Gateway API CRDs installed
- For TLS: cert-manager (optional, if using automatic certificates)
- For OIDC: OIDC provider (e.g., Dex, Keycloak)
- For group dimension: topologymetrics SDK v0.5.0+ (adds `group` label to metrics)
