# dephealth-ui Helm Chart

Helm chart for deploying dephealth-ui application to Kubernetes.

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

### Datasources

```yaml
config:
  datasources:
    prometheus:
      url: "http://victoriametrics.monitoring.svc:8428"
    alertmanager:
      url: "http://alertmanager.monitoring.svc:9093"
```

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

## Requirements

- Kubernetes 1.21+
- Helm 3.0+
- For Gateway API: Gateway API CRDs installed
- For TLS: cert-manager (optional, if using automatic certificates)
- For OIDC: OIDC provider (e.g., Dex, Keycloak)
