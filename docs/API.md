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

**Node States:**
- `ok` — all critical dependencies healthy
- `degraded` — some dependencies unhealthy or high latency
- `down` — all critical dependencies down
- `unknown` — no data available (stale)

**Edge States:**
- `ok` — health = 1, no alerts
- `degraded` — mixed health or active alerts
- `down` — health = 0
- `unknown` — stale (metrics disappeared)

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

#### `GET /api/v1/config`

Returns frontend configuration (Grafana URLs, dashboard UIDs, severity colors, display settings). This endpoint does not require authentication.

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

#### `GET /healthz`

Kubernetes liveness probe. Always returns `200 OK` with `{"status":"ok"}`.

#### `GET /readyz`

Kubernetes readiness probe. Always returns `200 OK` with `{"status":"ok"}`.

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

Terminates user session (only when `auth.type=oidc`).

**Response:** `302 Found`
Clears session cookie and redirects to login page.

---

#### `GET /auth/userinfo`

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

### Error Responses

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

### CORS

CORS is enabled for all origins with these settings:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, OPTIONS
Access-Control-Allow-Headers: Accept, Content-Type, If-None-Match
Access-Control-Max-Age: 300
```

---

### Caching and ETag

The `/api/v1/topology` endpoint (unfiltered) supports HTTP caching:

- Server returns `ETag` header with each response
- Clients can send `If-None-Match` with the previous ETag value
- If data hasn't changed, server returns `304 Not Modified` (empty body)
- Recommended polling interval: use `meta.ttl` value (default 15s)

Other endpoints (`/api/v1/alerts`, `/api/v1/instances`) are not cached and always return fresh data.

---

### Rate Limiting

No explicit rate limiting is enforced. Caching at server-side reduces load on Prometheus/AlertManager.

Recommended client polling intervals:
- `/api/v1/topology` — every 15-30 seconds (use `meta.ttl`)
- `/api/v1/alerts` — every 30-60 seconds
- `/api/v1/config` — once at startup

---

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

**Кэширование:** Нефильтрованные запросы (`namespace` пуст) кэшируются на сервере. Поддерживаются заголовки `ETag` / `If-None-Match` — возвращает `304 Not Modified`, если данные не изменились.

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
      "summary": "Высокая latency на payment-api → auth-service"
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

**Поля узла (Node):**

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | string | Уникальный идентификатор узла |
| `label` | string | Отображаемое имя |
| `state` | string | `ok`, `degraded`, `down`, `unknown` |
| `type` | string | `service` (инструментированное приложение) или тип зависимости (`postgres`, `redis`, `http` и т.д.) |
| `namespace` | string | Kubernetes namespace |
| `host` | string | Hostname endpoint (пропускается для service-узлов) |
| `port` | string | Порт endpoint (пропускается для service-узлов) |
| `dependencyCount` | int | Количество исходящих рёбер |
| `stale` | bool | `true`, если метрики узла пропали (только в режиме lookback) |
| `grafanaUrl` | string | Прямая ссылка на Grafana Service Status dashboard (пропускается, если Grafana не настроена) |
| `alertCount` | int | Количество активных алертов (пропускается, если 0) |
| `alertSeverity` | string | Наивысший severity алертов (пропускается, если нет алертов) |

**Поля ребра (Edge):**

| Поле | Тип | Описание |
|------|-----|----------|
| `source` | string | ID исходного узла |
| `target` | string | ID целевого узла |
| `type` | string | Тип подключения (`http`, `grpc`, `postgres`, `redis` и т.д.) |
| `latency` | string | Человекочитаемая latency (`"5.2ms"`) |
| `latencyRaw` | float64 | Latency в секундах |
| `health` | float64 | `1` = здоров, `0` = недоступен, `-1` = stale |
| `state` | string | `ok`, `degraded`, `down`, `unknown` |
| `critical` | bool | Является ли зависимость критичной |
| `stale` | bool | `true`, если метрики ребра пропали (только в режиме lookback) |
| `grafanaUrl` | string | Прямая ссылка на Grafana Link Status dashboard (пропускается, если Grafana не настроена) |
| `alertCount` | int | Количество активных алертов для этого ребра (пропускается, если 0) |
| `alertSeverity` | string | Наивысший severity алертов для этого ребра (пропускается, если нет алертов) |

**Поля meta:**

| Поле | Тип | Описание |
|------|-----|----------|
| `cachedAt` | string | RFC3339 метка времени кэширования |
| `ttl` | int | TTL кэша в секундах (клиенты должны опрашивать с этим интервалом) |
| `nodeCount` | int | Общее количество узлов |
| `edgeCount` | int | Общее количество рёбер |
| `partial` | bool | `true`, если часть запросов не удалась и данные могут быть неполными |
| `errors` | string[] | Описания ошибок при `partial=true` (пропускается, если пусто) |

**Состояния узлов:**
- `ok` — все критичные зависимости здоровы
- `degraded` — часть зависимостей недоступна или высокий latency
- `down` — все критичные зависимости недоступны
- `unknown` — нет данных (stale)

**Состояния рёбер:**
- `ok` — health = 1, нет алертов
- `degraded` — смешанное health или активные алерты
- `down` — health = 0
- `unknown` — stale (метрики пропали)

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
    "total": 5,
    "critical": 1,
    "warning": 4,
    "fetchedAt": "2026-02-10T09:15:30Z"
  }
}
```

**Поля алертов:**

| Поле | Тип | Описание |
|------|-----|----------|
| `alertname` | string | Имя правила алерта (`DependencyDown`, `DependencyDegraded` и т.д.) |
| `service` | string | Имя исходного сервиса |
| `dependency` | string | Имя целевой зависимости |
| `severity` | string | `critical`, `warning`, `info` |
| `state` | string | `firing` |
| `since` | string | RFC3339 метка начала алерта |
| `summary` | string | Человекочитаемое описание алерта (опционально) |

**Поля meta:**

| Поле | Тип | Описание |
|------|-----|----------|
| `total` | int | Общее количество активных алертов |
| `critical` | int | Количество critical-алертов |
| `warning` | int | Количество warning-алертов |
| `fetchedAt` | string | RFC3339 метка времени получения алертов |

---

#### `GET /api/v1/config`

Возвращает конфигурацию для фронтенда (Grafana URL, UID дашбордов, цвета severity, настройки отображения). Этот endpoint не требует аутентификации.

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

Возвращает JSON-массив инстансов (не обёрнут в объект):

```json
[
  {
    "instance": "10.244.1.5:9090",
    "pod": "order-service-7d9f8b-xyz12",
    "job": "order-service",
    "service": "order-service"
  }
]
```

**Ошибка:** `400 Bad Request` если параметр `service` отсутствует.

---

#### `GET /healthz`

Kubernetes liveness проба. Всегда возвращает `200 OK` с `{"status":"ok"}`.

#### `GET /readyz`

Kubernetes readiness проба. Всегда возвращает `200 OK` с `{"status":"ok"}`.

---

#### `GET /auth/login`

Инициирует OIDC-аутентификацию (только при `auth.type=oidc`).

**Ответ:** `302 Found`
Перенаправляет на authorization endpoint OIDC-провайдера.

---

#### `GET /auth/callback`

Callback endpoint для OIDC (только при `auth.type=oidc`).

**Ответ:** `302 Found`
Устанавливает session cookie и перенаправляет на корень приложения.

---

#### `GET /auth/logout`

Завершает сессию пользователя (только при `auth.type=oidc`).

**Ответ:** `302 Found`
Удаляет session cookie и перенаправляет на страницу входа.

---

#### `GET /auth/userinfo`

Возвращает информацию о текущем аутентифицированном пользователе (только при `auth.type=oidc`).

**Ответ:** `200 OK`

```json
{
  "username": "john.doe",
  "email": "john.doe@example.com",
  "authenticated": true
}
```

**Ответ (неаутентифицирован):** `401 Unauthorized`

---

### Ответы с ошибками

Все ответы с ошибками используют формат:

```json
{
  "error": "описание ошибки"
}
```

**Типичные HTTP-статусы:**

| HTTP Status | Описание |
|-------------|----------|
| 400 | Неверные или отсутствующие query-параметры |
| 401 | Требуется аутентификация |
| 502 | Prometheus/AlertManager недоступен |

---

### CORS

CORS включён для всех источников со следующими настройками:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, OPTIONS
Access-Control-Allow-Headers: Accept, Content-Type, If-None-Match
Access-Control-Max-Age: 300
```

---

### Кэширование и ETag

Endpoint `/api/v1/topology` (без фильтров) поддерживает HTTP-кэширование:

- Сервер возвращает заголовок `ETag` с каждым ответом
- Клиенты могут отправить `If-None-Match` с предыдущим значением ETag
- Если данные не изменились, сервер возвращает `304 Not Modified` (пустое тело)
- Рекомендуемый интервал опроса: использовать значение `meta.ttl` (по умолчанию 15с)

Остальные endpoint'ы (`/api/v1/alerts`, `/api/v1/instances`) не кэшируются и всегда возвращают свежие данные.

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
- [Application Design](./application-design.md) — Architecture overview | Обзор архитектуры
- [Helm Chart](../deploy/helm/dephealth-ui/README.md) — Deployment guide | Руководство по развёртыванию
