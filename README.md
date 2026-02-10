# dephealth-ui

[![Version](https://img.shields.io/badge/version-0.10.13-blue.svg)](https://github.com/BigKAA/dephealth-ui)
[![Go Version](https://img.shields.io/badge/go-1.25-00ADD8.svg)](https://golang.org/)
[![Helm Chart](https://img.shields.io/badge/helm-0.5.0-0F1689.svg)](./deploy/helm/dephealth-ui)
[![License](https://img.shields.io/badge/license-Apache%202.0-green.svg)](./LICENSE)

**Real-time microservices topology and health visualization tool**

**Language:** [English](#english) | [Русский](#русский)

---

## English

### Overview

**dephealth-ui** is a web application for visualizing microservice topologies and monitoring dependency health in real-time. It displays an interactive directed graph showing service states (OK, DEGRADED, DOWN), connection latency, and provides direct links to Grafana dashboards.

The application consumes metrics collected by the [dephealth SDK](https://github.com/BigKAA/topologymetrics) from Prometheus/VictoriaMetrics and correlates them with AlertManager alerts to provide a unified health view.

### Features

✅ **Real-time Topology Visualization**
- Interactive node-graph diagram with Cytoscape.js
- Automatic layout with dagre (hierarchical/top-to-bottom)
- Color-coded node states (green=OK, yellow=DEGRADED, red=DOWN)
- Dynamic node sizing based on label length

✅ **Comprehensive Monitoring**
- Service health status with alert counts
- Edge latency display (average P99 percentile)
- Critical dependency highlighting (thicker edges)
- Active AlertManager alert integration

✅ **Rich UI Features**
- Smart search with fuzzy matching
- Multi-filter support (namespace, type, state, service)
- Alert drawer with severity-based grouping
- Node detail sidebar with instance information
- Grafana dashboard integration (click-through)
- Legend, statistics, and export to PNG
- Keyboard shortcuts and fullscreen mode
- Dark theme support

✅ **Enterprise-Ready**
- Multiple authentication modes (none, Basic, OIDC/SSO)
- CORS support for browser-based clients
- Server-side caching (configurable TTL)
- Multi-architecture Docker images (amd64, arm64)
- Kubernetes-native with Helm chart
- Gateway API and Ingress support

### Architecture

```
┌─────────────────────┐
│  Browser (SPA)      │  ← Cytoscape.js + Vite
│  Vanilla JS         │
└──────────┬──────────┘
           │ HTTPS (REST API)
           ▼
┌─────────────────────────────────┐
│  dephealth-ui (Go)              │  ← Single binary
│  ┌─────────────────────────┐   │
│  │ REST API                │   │  /api/v1/topology
│  │ /api/v1/alerts          │   │  /api/v1/instances
│  │ /api/v1/config          │   │  /api/v1/config
│  └─────────────────────────┘   │
│  ┌─────────────────────────┐   │
│  │ Topology Service        │   │  ← PromQL queries
│  │ Alert Aggregation       │   │  ← AlertManager API
│  │ In-memory Cache (TTL)   │   │
│  └─────────────────────────┘   │
│  ┌─────────────────────────┐   │
│  │ Auth (none/basic/oidc)  │   │  ← Pluggable
│  └─────────────────────────┘   │
└──────────┬──────────────────────┘
           │
           ▼
┌──────────────────────────────────┐
│ Prometheus/VictoriaMetrics       │  ← app_dependency_health
│ AlertManager                     │  ← app_dependency_latency_seconds
└──────────────────────────────────┘
```

### Tech Stack

| Component | Technology |
|-----------|------------|
| **Backend** | Go 1.25 (net/http + chi router) |
| **Frontend** | Vanilla JS + Vite + Cytoscape.js |
| **Visualization** | Cytoscape.js + cytoscape-dagre |
| **Container** | Docker (multi-stage, multi-arch) |
| **Orchestration** | Kubernetes (Helm 3) |

---

## Quick Start

### Prerequisites

- Kubernetes cluster with Gateway API or Ingress controller
- Prometheus/VictoriaMetrics with [dephealth SDK](https://github.com/BigKAA/topologymetrics) metrics
- AlertManager (optional, for alert integration)
- Helm 3.0+

### Installation

#### 1. Add Helm Repository (if published)

```bash
# If using a Helm repository
helm repo add dephealth https://charts.dephealth.io
helm repo update
```

#### 2. Install with Helm

**Using Gateway API:**
```bash
helm install dephealth-ui ./deploy/helm/dephealth-ui \
  --set route.enabled=true \
  --set route.hostname=dephealth.example.com \
  --set tls.enabled=true \
  --set tls.issuerName=letsencrypt-prod \
  --set config.datasources.prometheus.url=http://victoriametrics:8428 \
  --set config.datasources.alertmanager.url=http://alertmanager:9093 \
  -n dephealth-ui --create-namespace
```

**Using Ingress:**
```bash
helm install dephealth-ui ./deploy/helm/dephealth-ui \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set ingress.hostname=dephealth.example.com \
  --set ingress.tls.enabled=true \
  --set ingress.tls.certManager.enabled=true \
  --set ingress.tls.certManager.issuerName=letsencrypt-prod \
  --set config.datasources.prometheus.url=http://victoriametrics:8428 \
  -n dephealth-ui --create-namespace
```

#### 3. Access the UI

Open your browser and navigate to:
```
https://dephealth.example.com
```

---

## Configuration

### Application Config

Create `config.yaml`:

```yaml
server:
  listen: ":8080"

datasources:
  prometheus:
    url: "http://victoriametrics.monitoring.svc:8428"
    # Optional: Basic auth for Prometheus
    # username: "reader"
    # password: "secret"
  alertmanager:
    url: "http://alertmanager.monitoring.svc:9093"

cache:
  ttl: 15s  # Cache duration for topology data

auth:
  type: "none"  # Options: "none", "basic", "oidc"
  
  # Basic authentication
  # basic:
  #   users:
  #     - username: admin
  #       passwordHash: "$2a$10$..."  # bcrypt hash
  
  # OIDC authentication
  # oidc:
  #   issuer: "https://dex.example.com"
  #   clientId: "dephealth-ui"
  #   clientSecret: "ZGVwaGVhbHRoLXVpLXNlY3JldA"
  #   redirectUrl: "https://dephealth.example.com/auth/callback"

grafana:
  baseUrl: "https://grafana.example.com"
  dashboards:
    serviceStatus: "dephealth-service-status"
    linkStatus: "dephealth-link-status"
```

### Environment Variables

All configuration can be overridden via environment variables:

```bash
DEPHEALTH_SERVER_LISTEN=":8080"
DEPHEALTH_DATASOURCES_PROMETHEUS_URL="http://victoriametrics:8428"
DEPHEALTH_DATASOURCES_ALERTMANAGER_URL="http://alertmanager:9093"
DEPHEALTH_CACHE_TTL="15s"
DEPHEALTH_AUTH_TYPE="none"
DEPHEALTH_GRAFANA_BASEURL="https://grafana.example.com"
```

---

## Required Metrics

dephealth-ui requires two Prometheus metrics from services instrumented with [dephealth SDK](https://github.com/BigKAA/topologymetrics):

### 1. `app_dependency_health` (Gauge)

Health status of dependency endpoints (1=UP, 0=DOWN).

**Required Labels:**
- `name` — service name
- `namespace` — Kubernetes namespace
- `dependency` — logical dependency name
- `type` — connection type (http, grpc, postgres, redis, etc.)
- `host` — target endpoint hostname
- `port` — target endpoint port
- `critical` — criticality flag (yes/no)

**Example:**
```prometheus
app_dependency_health{name="order-service",namespace="prod",dependency="postgres-main",type="postgres",host="pg.svc",port="5432",critical="yes"} 1
```

### 2. `app_dependency_latency_seconds` (Histogram)

Health check latency in seconds with standard buckets: `0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0`

**See [docs/METRICS.md](./docs/METRICS.md) for complete specification.**

---

## Development

### Local Development

#### Prerequisites

- Go 1.25+
- Node.js 22+
- Docker (optional)

#### Build Frontend

```bash
cd frontend
npm install
npm run dev  # Development server with HMR
# or
npm run build  # Production build
```

#### Build Backend

```bash
go mod download
go build -o dephealth-ui ./cmd/dephealth-ui
```

#### Run Locally

```bash
./dephealth-ui -config config.yaml
```

### Docker Build

```bash
# Build multi-arch image
make docker-build TAG=v0.10.13

# Or manually
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t harbor.kryukov.lan/library/dephealth-ui:v0.10.13 \
  --push .
```

### Testing

```bash
# Backend tests
go test ./... -v -race

# Frontend tests
cd frontend
npm test
```

---

## Documentation

| Document | Description |
|----------|-------------|
| **[METRICS.md](./docs/METRICS.md)** | ⭐ **START HERE** — Metrics format, required labels, PromQL queries |
| **[API.md](./docs/API.md)** | REST API reference with all endpoints |
| **[Helm Chart README](./deploy/helm/dephealth-ui/README.md)** | Kubernetes deployment guide |
| **[Application Design](./docs/application-design.md)** | Architecture overview and design decisions |

---

## Project Structure

```
dephealth-ui/
├── cmd/dephealth-ui/          # Application entry point
├── internal/                  # Go packages
│   ├── config/               # Configuration handling
│   ├── server/               # HTTP server + routes
│   ├── topology/             # Topology service (Prometheus queries)
│   ├── alerts/               # AlertManager integration
│   ├── auth/                 # Authentication (none/basic/oidc)
│   └── cache/                # In-memory cache with TTL
├── frontend/                  # Vite + Cytoscape.js SPA
│   ├── src/                  # JavaScript modules
│   ├── public/               # Static assets
│   └── index.html            # SPA entry point
├── deploy/                    # Deployment manifests
│   └── helm/                 # Helm charts
│       ├── dephealth-ui/     # Application chart
│       ├── dephealth-infra/  # Test infrastructure
│       └── dephealth-monitoring/  # Monitoring stack
├── docs/                      # Documentation
└── test/                      # Test helpers and fixtures
```

---

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes using [Conventional Commits](https://www.conventionalcommits.org/)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

**Commit Format:**
```
<type>(<scope>): <subject>

Types: feat, fix, docs, style, refactor, test, chore
```

---

## License

Apache License 2.0 - see [LICENSE](./LICENSE) for details.

---

## Support

- **Issues:** [GitHub Issues](https://github.com/BigKAA/dephealth-ui/issues)
- **Documentation:** [docs/](./docs/)
- **dephealth SDK:** [topologymetrics](https://github.com/BigKAA/topologymetrics)

---

## Related Projects

- [dephealth SDK](https://github.com/BigKAA/topologymetrics) — Instrumentation library for Go, Python, Java, .NET
- [VictoriaMetrics](https://victoriametrics.com/) — High-performance Prometheus-compatible TSDB
- [Cytoscape.js](https://js.cytoscape.org/) — Graph visualization library

---

## Русский

### Обзор

**dephealth-ui** — веб-приложение для визуализации топологии микросервисов и мониторинга состояния зависимостей в реальном времени. Отображает интерактивный направленный граф с состояниями сервисов (OK, DEGRADED, DOWN), latency соединений и предоставляет прямые ссылки на дашборды Grafana.

Приложение потребляет метрики, собранные [dephealth SDK](https://github.com/BigKAA/topologymetrics) из Prometheus/VictoriaMetrics, и коррелирует их с алертами AlertManager для предоставления единого представления о здоровье системы.

### Возможности

✅ **Визуализация топологии в реальном времени**
- Интерактивная диаграмма узлов с Cytoscape.js
- Автоматический layout через dagre (иерархический/сверху-вниз)
- Цветовая индикация состояний (зелёный=OK, жёлтый=DEGRADED, красный=DOWN)
- Динамический размер узлов в зависимости от длины текста

✅ **Полный мониторинг**
- Статус здоровья сервисов с количеством алертов
- Отображение latency на рёбрах (средний и P99 перцентиль)
- Выделение критичных зависимостей (толще рёбра)
- Интеграция с активными алертами AlertManager

✅ **Богатый UI**
- Умный поиск с fuzzy matching
- Множественные фильтры (namespace, тип, состояние, сервис)
- Drawer алертов с группировкой по severity
- Боковая панель с деталями узла и списком инстансов
- Интеграция с Grafana (переход по клику)
- Легенда, статистика, экспорт в PNG
- Горячие клавиши и полноэкранный режим
- Поддержка тёмной темы

✅ **Enterprise-ready**
- Несколько режимов аутентификации (none, Basic, OIDC/SSO)
- CORS для браузерных клиентов
- Серверное кэширование (настраиваемый TTL)
- Multi-arch Docker образы (amd64, arm64)
- Kubernetes-native с Helm chart
- Поддержка Gateway API и Ingress

### Быстрый старт

#### Установка через Helm

**С использованием Gateway API:**
```bash
helm install dephealth-ui ./deploy/helm/dephealth-ui \
  --set route.enabled=true \
  --set route.hostname=dephealth.example.com \
  --set tls.enabled=true \
  --set tls.issuerName=letsencrypt-prod \
  --set config.datasources.prometheus.url=http://victoriametrics:8428 \
  -n dephealth-ui --create-namespace
```

**С использованием Ingress:**
```bash
helm install dephealth-ui ./deploy/helm/dephealth-ui \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set ingress.hostname=dephealth.example.com \
  --set ingress.tls.enabled=true \
  --set ingress.tls.certManager.enabled=true \
  --set ingress.tls.certManager.issuerName=letsencrypt-prod \
  -n dephealth-ui --create-namespace
```

### Необходимые метрики

Приложение требует две Prometheus-метрики от сервисов, инструментированных [dephealth SDK](https://github.com/BigKAA/topologymetrics):

**1. `app_dependency_health`** (Gauge) — состояние здоровья endpoint'ов зависимостей (1=UP, 0=DOWN)

**2. `app_dependency_latency_seconds`** (Histogram) — latency health check'ов в секундах

**Обязательные метки:** `name`, `namespace`, `dependency`, `type`, `host`, `port`, `critical`

**Полная спецификация:** [docs/METRICS.md](./docs/METRICS.md)

### Документация

| Документ | Описание |
|----------|----------|
| **[METRICS.md](./docs/METRICS.md)** | ⭐ **НАЧНИТЕ ОТСЮДА** — Формат метрик, обязательные метки, PromQL-запросы |
| **[API.md](./docs/API.md)** | Справочник REST API со всеми endpoint'ами |
| **[Helm Chart README](./deploy/helm/dephealth-ui/README.md)** | Руководство по развёртыванию в Kubernetes |
| **[Application Design](./docs/application-design.md)** | Обзор архитектуры и проектные решения |

### Разработка

```bash
# Сборка фронтенда
cd frontend && npm install && npm run build

# Сборка backend
go build -o dephealth-ui ./cmd/dephealth-ui

# Запуск
./dephealth-ui -config config.yaml
```

### Лицензия

Apache License 2.0 — см. [LICENSE](./LICENSE) для деталей.

---

**Built with ❤️ for microservices observability**
