# Справочник REST API

**Язык:** [English](./API.md) | Русский

---

## Обзор

dephealth-ui предоставляет REST API для визуализации топологии и мониторинга здоровья. Все endpoint'ы возвращают JSON и поддерживают CORS для браузерных клиентов.

**Базовый URL:** `https://dephealth.example.com`
**Префикс API:** `/api/v1`

---

## Аутентификация

Режим аутентификации настраивается в `config.yaml`:

- **`none`** — Без аутентификации (открытый доступ)
- **`basic`** — HTTP Basic Authentication (username/password)
- **`oidc`** — OpenID Connect (перенаправление на SSO провайдер)

Для OIDC фронтенд автоматически обрабатывает OAuth2 flow. API-запросы после аутентификации включают session cookies.

---

## Endpoint'ы

### `GET /api/v1/topology`

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

**Состояния узлов (сервис-ноды):**
- `ok` — все исходящие рёбра здоровы (health=1)
- `degraded` — любое исходящее ребро имеет health=0
- `down` — все исходящие рёбра stale (метрики пропали)
- `unknown` — нет исходящих рёбер / нет данных

> Примечание: Backend `calcServiceNodeState` никогда не возвращает `"down"` напрямую — возвращает только `ok`, `degraded` или `unknown`. Состояние `down` устанавливается логикой stale detection, когда все рёбра stale.

**Состояния рёбер:**
- `ok` — health = 1
- `down` — health = 0
- `unknown` — stale (метрики пропали в пределах lookback window)

**Каскадные предупреждения (только фронтенд):**
Вычисление каскадного распространения сбоев выполняется целиком на фронтенде. API-ответ не содержит каскадных данных (`cascadeCount`, `cascadeSources`, `inCascadeChain`). Поле `critical` на рёбрах определяет, будут ли сбои распространяться вверх как каскадные предупреждения. Подробнее см. [Проектирование приложения — Каскадные предупреждения](./application-design.ru.md#каскадные-предупреждения).

---

### `GET /api/v1/alerts`

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

### `GET /api/v1/config`

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

### `GET /api/v1/instances`

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

### `GET /healthz`

Kubernetes liveness проба. Всегда возвращает `200 OK` с `{"status":"ok"}`.

### `GET /readyz`

Kubernetes readiness проба. Всегда возвращает `200 OK` с `{"status":"ok"}`.

---

### `GET /auth/login`

Инициирует OIDC-аутентификацию (только при `auth.type=oidc`).

**Ответ:** `302 Found`
Перенаправляет на authorization endpoint OIDC-провайдера.

---

### `GET /auth/callback`

Callback endpoint для OIDC (только при `auth.type=oidc`).

**Ответ:** `302 Found`
Устанавливает session cookie и перенаправляет на корень приложения.

---

### `GET /auth/logout`

Завершает сессию пользователя (только при `auth.type=oidc`).

**Ответ:** `302 Found`
Удаляет session cookie и перенаправляет на страницу входа.

---

### `GET /auth/userinfo`

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

## Ответы с ошибками

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

## CORS

CORS включён для всех источников со следующими настройками:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, OPTIONS
Access-Control-Allow-Headers: Accept, Content-Type, If-None-Match
Access-Control-Max-Age: 300
```

---

## Кэширование и ETag

Endpoint `/api/v1/topology` (без фильтров) поддерживает HTTP-кэширование:

- Сервер возвращает заголовок `ETag` с каждым ответом
- Клиенты могут отправить `If-None-Match` с предыдущим значением ETag
- Если данные не изменились, сервер возвращает `304 Not Modified` (пустое тело)
- Рекомендуемый интервал опроса: использовать значение `meta.ttl` (по умолчанию 15с)

Остальные endpoint'ы (`/api/v1/alerts`, `/api/v1/instances`) не кэшируются и всегда возвращают свежие данные.

---

## Rate Limiting

Явное ограничение частоты запросов не применяется. Кэширование на стороне сервера снижает нагрузку на Prometheus/AlertManager.

Рекомендуемые интервалы опроса клиента:
- `/api/v1/topology` — каждые 15-30 секунд (используйте `meta.ttl`)
- `/api/v1/alerts` — каждые 30-60 секунд
- `/api/v1/config` — один раз при запуске

---

## См. также

- [Спецификация метрик](./METRICS.ru.md) — Формат обязательных метрик
- [Проектирование приложения](./application-design.ru.md) — Обзор архитектуры
- [Руководство по развёртыванию](../deploy/helm/dephealth-ui/README.ru.md) — Kubernetes & Helm
