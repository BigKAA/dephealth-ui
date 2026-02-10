# REST API Reference | Справочник REST API

**Language:** [English](#english) | [Русский](#русский)

---

## English

### Overview

dephealth-ui exposes a REST API for topology visualization and health monitoring. All endpoints return JSON and support CORS for browser-based clients.

**Base URL:** `https://dephealth.example.com`  
**API Prefix:** `/api/v1`

---

### Authentication

Authentication mode is configured in `config.yaml`:

- **`none`** — No authentication (open access)
- **`basic`** — HTTP Basic Authentication (username/password)
- **`oidc`** — OpenID Connect (redirects to SSO provider)

For OIDC, the frontend automatically handles the OAuth2 flow. API calls after authentication include session cookies.

---

### Endpoints

#### `GET /api/v1/topology`

Returns the complete service topology graph with pre-calculated node/edge states.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `namespace` | string | No | Filter by Kubernetes namespace (empty = all) |

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
      "state": "ok",
      "type": "postgres",
      "namespace": "production",
      "host": "pg-master.db.svc",
      "port": "5432",
      "dependencyCount": 0
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
      "grafanaUrl": "https://grafana.example.com/d/dephealth-link-status?var-dependency=postgres-main&var-host=pg-master.db.svc&var-port=5432"
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
    "edgeCount": 187
  }
}
```

**Node States:**
- `ok` — all critical dependencies healthy
- `degraded` — some dependencies unhealthy or high latency
- `down` — all critical dependencies down
- `unknown` — no data available

**Edge States:**
- `ok` — health = 1, no alerts
- `degraded` — mixed health or active alerts
- `down` — health = 0

**Caching:** Responses are cached server-side with TTL specified in `meta.ttl`. Clients should poll with interval = TTL.

---

#### `GET /api/v1/alerts`

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
    "cachedAt": "2026-02-10T09:15:30Z",
    "ttl": 30,
    "totalAlerts": 5
  }
}
```

**Severity Levels:**
- `critical` — service outage, immediate action required
- `warning` — degraded state, monitoring required
- `info` — informational, no action required

---

#### `GET /api/v1/config`

Returns frontend configuration (Grafana URLs, dashboard UIDs, severity colors, display settings).

**Response:** `200 OK`

```json
{
  "grafana": {
    "baseUrl": "https://grafana.example.com",
    "dashboards": {
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
| `serviceStatus` | Single service health | `?var-service=<name>` |
| `linkStatus` | Single dependency health | `?var-dependency=<dep>&var-host=<host>&var-port=<port>` |
| `serviceList` | All services table | — |
| `servicesStatus` | All services overview | — |
| `linksStatus` | All links overview | — |

If `grafana.baseUrl` is empty, Grafana integration features are hidden in the UI.

---

#### `GET /api/v1/instances`

Returns all running instances (pods/containers) for a specific service.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|:--------:|-------------|
| `service` | string | Yes | Service name (from `name` label) |

**Example:** `GET /api/v1/instances?service=order-service`

**Response:** `200 OK`

```json
{
  "instances": [
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
  ],
  "meta": {
    "cachedAt": "2026-02-10T09:15:30Z",
    "ttl": 60,
    "instanceCount": 2
  }
}
```

---

#### `GET /auth/login`

Initiates OIDC authentication flow (only when `auth.type=oidc`).

**Response:** `302 Found`  
Redirects to OIDC provider's authorization endpoint.

---

#### `GET /auth/callback`

OIDC callback endpoint (only when `auth.type=oidc`).

**Response:** `302 Found`  
Sets session cookie and redirects to application root.

---

#### `GET /auth/logout`

Terminates user session (only when `auth.type=oidc` or `auth.type=basic`).

**Response:** `302 Found`  
Clears session cookie and redirects to login page.

---

#### `GET /auth/userinfo`

Returns current authenticated user information.

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

### Error Responses

All error responses follow this format:

```json
{
  "error": "error message description",
  "code": "ERROR_CODE"
}
```

**Common Error Codes:**

| HTTP Status | Code | Description |
|-------------|------|-------------|
| 400 | `BAD_REQUEST` | Invalid query parameters |
| 401 | `UNAUTHORIZED` | Authentication required |
| 403 | `FORBIDDEN` | Access denied |
| 404 | `NOT_FOUND` | Resource not found |
| 500 | `INTERNAL_ERROR` | Server error |
| 502 | `DATASOURCE_ERROR` | Prometheus/AlertManager unreachable |
| 503 | `SERVICE_UNAVAILABLE` | Service temporarily unavailable |

---

### CORS

CORS is enabled by default with these headers:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

To restrict origins, configure in `config.yaml`:

```yaml
server:
  cors:
    allowedOrigins: ["https://dephealth.example.com"]
```

---

### Rate Limiting

No explicit rate limiting is enforced. Caching at server-side reduces load on Prometheus/AlertManager.

Recommended client polling intervals:
- `/api/v1/topology` — every 15-30 seconds (use `meta.ttl`)
- `/api/v1/alerts` — every 30-60 seconds
- `/api/v1/config` — once at startup

---

## Русский

### Обзор

dephealth-ui предоставляет REST API для визуализации топологии и мониторинга здоровья. Все endpoint'ы возвращают JSON и поддерживают CORS для браузерных клиентов.

**Базовый URL:** `https://dephealth.example.com`  
**Префикс API:** `/api/v1`

---

### Аутентификация

Режим аутентификации настраивается в `config.yaml`:

- **`none`** — Без аутентификации (открытый доступ)
- **`basic`** — HTTP Basic Authentication (username/password)
- **`oidc`** — OpenID Connect (перенаправление на SSO провайдер)

Для OIDC фронтенд автоматически обрабатывает OAuth2 flow. API-запросы после аутентификации включают session cookies.

---

### Endpoint'ы

#### `GET /api/v1/topology`

Возвращает полный граф топологии сервисов с предвычисленными состояниями узлов/рёбер.

**Query-параметры:**

| Параметр | Тип | Обязателен | Описание |
|----------|-----|:----------:|----------|
| `namespace` | string | Нет | Фильтр по Kubernetes namespace (пусто = все) |

**Ответ:** `200 OK`

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
      "critical": true
    }
  ],
  "alerts": [],
  "meta": {
    "cachedAt": "2026-02-10T09:15:30Z",
    "ttl": 15,
    "nodeCount": 42,
    "edgeCount": 187
  }
}
```

**Состояния узлов:**
- `ok` — все критичные зависимости здоровы
- `degraded` — часть зависимостей недоступна или высокий latency
- `down` — все критичные зависимости недоступны
- `unknown` — нет данных

**Состояния рёбер:**
- `ok` — health = 1, нет алертов
- `degraded` — смешанное health или активные алерты
- `down` — health = 0

**Кэширование:** Ответы кэшируются на сервере с TTL, указанным в `meta.ttl`. Клиенты должны опрашивать с интервалом = TTL.

---

#### `GET /api/v1/alerts`

Возвращает все активные алерты из AlertManager, агрегированные по сервису/зависимости.

**Ответ:** `200 OK`

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
      "summary": "Зависимость postgres-main полностью недоступна"
    }
  ],
  "meta": {
    "cachedAt": "2026-02-10T09:15:30Z",
    "ttl": 30,
    "totalAlerts": 5
  }
}
```

**Уровни severity:**
- `critical` — сбой сервиса, требуется немедленное действие
- `warning` — деградированное состояние, требуется мониторинг
- `info` — информационный, действий не требуется

---

#### `GET /api/v1/config`

Возвращает конфигурацию для фронтенда (Grafana URL, UID дашбордов, цвета severity, настройки отображения).

**Ответ:** `200 OK`

```json
{
  "grafana": {
    "baseUrl": "https://grafana.example.com",
    "dashboards": {
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

**UID дашбордов:**

| Ключ | Назначение | URL-параметры |
|------|-----------|---------------|
| `serviceStatus` | Состояние одного сервиса | `?var-service=<name>` |
| `linkStatus` | Состояние одной зависимости | `?var-dependency=<dep>&var-host=<host>&var-port=<port>` |
| `serviceList` | Таблица всех сервисов | — |
| `servicesStatus` | Обзор состояния сервисов | — |
| `linksStatus` | Обзор состояния связей | — |

Если `grafana.baseUrl` пуст, элементы интеграции с Grafana скрыты в UI.

---

#### `GET /api/v1/instances`

Возвращает все запущенные инстансы (pod'ы/контейнеры) для указанного сервиса.

**Query-параметры:**

| Параметр | Тип | Обязателен | Описание |
|----------|-----|:----------:|----------|
| `service` | string | Да | Имя сервиса (из метки `name`) |

**Пример:** `GET /api/v1/instances?service=order-service`

**Ответ:** `200 OK`

```json
{
  "instances": [
    {
      "instance": "10.244.1.5:9090",
      "pod": "order-service-7d9f8b-xyz12",
      "job": "order-service",
      "service": "order-service"
    }
  ],
  "meta": {
    "cachedAt": "2026-02-10T09:15:30Z",
    "ttl": 60,
    "instanceCount": 2
  }
}
```

---

### Ответы с ошибками

Все ответы с ошибками следуют этому формату:

```json
{
  "error": "описание сообщения об ошибке",
  "code": "ERROR_CODE"
}
```

**Типичные коды ошибок:**

| HTTP Status | Code | Описание |
|-------------|------|----------|
| 400 | `BAD_REQUEST` | Неверные query-параметры |
| 401 | `UNAUTHORIZED` | Требуется аутентификация |
| 403 | `FORBIDDEN` | Доступ запрещён |
| 404 | `NOT_FOUND` | Ресурс не найден |
| 500 | `INTERNAL_ERROR` | Ошибка сервера |
| 502 | `DATASOURCE_ERROR` | Prometheus/AlertManager недоступен |
| 503 | `SERVICE_UNAVAILABLE` | Сервис временно недоступен |

---

### CORS

CORS включён по умолчанию с этими заголовками:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

Для ограничения источников настройте в `config.yaml`:

```yaml
server:
  cors:
    allowedOrigins: ["https://dephealth.example.com"]
```

---

### Rate Limiting

Явное ограничение частоты запросов не применяется. Кэширование на стороне сервера снижает нагрузку на Prometheus/AlertManager.

Рекомендуемые интервалы опроса клиента:
- `/api/v1/topology` — каждые 15-30 секунд (используйте `meta.ttl`)
- `/api/v1/alerts` — каждые 30-60 секунд
- `/api/v1/config` — один раз при запуске

---

## See Also | См. также

- [Metrics Specification](./METRICS.md) — Required metrics format | Формат обязательных метрик
- [Deployment Guide](./DEPLOYMENT.md) — Kubernetes & Helm | Kubernetes & Helm
- [Application Design](./application-design.md) — Architecture overview | Обзор архитектуры
