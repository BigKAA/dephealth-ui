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
| `group` | string | Нет | Фильтр по логической группе (SDK v0.5.0+, пусто = все) |
| `time` | string | Нет | ISO8601/RFC3339 метка времени для исторических запросов (например, `2026-02-15T12:00:00Z`). Возвращает состояние топологии на указанный момент |

**Кэширование:** Нефильтрованные live-запросы (`namespace`, `group` и `time` пусты) кэшируются на сервере. Поддерживаются заголовки `ETag` / `If-None-Match` — возвращает `304 Not Modified`, если данные не изменились. Исторические запросы (`time` задан) полностью обходят кэш.

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
      "group": "payment-team",
      "isEntry": true,
      "dependencyCount": 3,
      "grafanaUrl": "https://grafana.example.com/d/dephealth-service-status?var-service=order-service",
      "alertCount": 0,
      "alertSeverity": ""
    },
    {
      "id": "order-service/postgres-main",
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
      "target": "order-service/postgres-main",
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
    "errors": [],
    "time": "2026-02-10T09:00:00Z",
    "isHistory": true
  }
}
```

**Поля узла (Node):**

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | string | Уникальный идентификатор узла. Для сервис-узлов — имя сервиса (например, `order-service`). Для dependency-узлов — формат `{source}/{dependency}` (например, `order-service/postgres-main`). Если имя зависимости совпадает с известным сервисом, используется узел этого сервиса |
| `label` | string | Отображаемое имя. Для сервис-узлов — имя сервиса. Для dependency-узлов — логическое имя зависимости (например, `postgres-main`) |
| `state` | string | `ok`, `degraded`, `down`, `unknown` |
| `type` | string | `service` (инструментированное приложение) или тип зависимости (`postgres`, `redis`, `http`, `ldap` и т.д.) |
| `namespace` | string | Kubernetes namespace |
| `group` | string | Логическая группа сервиса (SDK v0.5.0+, пропускается если пуста) |
| `isEntry` | bool | `true`, если узел является точкой входа для внешнего трафика (задаётся через метку `isentry` в метриках dephealth SDK). Пропускается, если `false` |
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
| `time` | string | RFC3339 метка запрошенного исторического момента (пропускается в live-режиме) |
| `isHistory` | bool | `true` при просмотре исторических данных (пропускается в live-режиме) |

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

### `GET /api/v1/cascade-analysis`

Выполняет BFS-анализ каскадных сбоев по графу зависимостей. Возвращает первопричины, затронутые сервисы и полные каскадные цепочки с неограниченной глубиной.

**Query-параметры:**

| Параметр | Тип | Обязателен | Описание |
|----------|-----|:----------:|----------|
| `service` | string | Нет | Анализ каскада для конкретного сервиса (пусто = все) |
| `namespace` | string | Нет | Фильтр по Kubernetes namespace |
| `group` | string | Нет | Фильтр по логической группе (SDK v0.5.0+) |
| `depth` | int | Нет | Максимальная глубина BFS (`0` = без ограничений) |
| `time` | string | Нет | ISO8601/RFC3339 метка времени для исторического анализа каскадов |

**Ответ:** `200 OK`

```json
{
  "rootCauses": [
    {
      "id": "postgres-main.db.svc:5432",
      "label": "postgres-main",
      "state": "down",
      "namespace": "production"
    }
  ],
  "affectedServices": [
    {
      "service": "order-service",
      "namespace": "production",
      "dependsOn": "payment-api",
      "rootCauses": ["postgres-main.db.svc:5432"]
    }
  ],
  "allFailures": [
    {
      "service": "payment-api",
      "dependency": "postgres-main.db.svc:5432",
      "health": 0,
      "critical": true
    }
  ],
  "cascadeChains": [
    {
      "affectedService": "order-service",
      "namespace": "production",
      "dependsOn": "postgres-main",
      "path": ["order-service", "payment-api", "postgres-main"],
      "depth": 2
    }
  ],
  "summary": {
    "totalServices": 10,
    "rootCauseCount": 1,
    "affectedServiceCount": 3,
    "totalFailureCount": 2,
    "maxDepth": 2
  }
}
```

**Поля Root Cause:**

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | string | Идентификатор зависимости (может включать host:port) |
| `label` | string | Человекочитаемое имя |
| `state` | string | Текущее состояние (`down`, `degraded` и т.д.) |
| `namespace` | string | Kubernetes namespace |

**Поля Cascade Chain:**

| Поле | Тип | Описание |
|------|-----|----------|
| `affectedService` | string | Затронутый сервис |
| `namespace` | string | Namespace затронутого сервиса |
| `dependsOn` | string | Терминальная зависимость (первопричина) |
| `path` | string[] | Полный путь от затронутого сервиса до первопричины |
| `depth` | int | Количество хопов в цепочке |

---

### `GET /api/v1/cascade-graph`

Возвращает топологию каскадных сбоев в формате [Grafana Node Graph panel](https://grafana.com/docs/grafana/latest/panels-visualizations/visualizations/node-graph/). Предназначен для прямого использования через Grafana Infinity datasource.

**Query-параметры:**

| Параметр | Тип | Обязателен | Описание |
|----------|-----|:----------:|----------|
| `service` | string | Нет | Фильтр графа каскада для конкретного сервиса (пусто = все) |
| `namespace` | string | Нет | Фильтр по Kubernetes namespace |
| `group` | string | Нет | Фильтр по логической группе (SDK v0.5.0+) |
| `depth` | int | Нет | Максимальная глубина BFS (`0` = без ограничений) |

**Ответ:** `200 OK`

```json
{
  "nodes": [
    {
      "id": "order-service",
      "title": "order-service",
      "subTitle": "production",
      "mainStat": "ok",
      "arc__failed": 0,
      "arc__degraded": 0,
      "arc__ok": 1,
      "arc__unknown": 0
    },
    {
      "id": "postgres-main",
      "title": "postgres-main",
      "subTitle": "production",
      "mainStat": "down",
      "arc__failed": 1,
      "arc__degraded": 0,
      "arc__ok": 0,
      "arc__unknown": 0
    }
  ],
  "edges": [
    {
      "id": "order-service--postgres-main",
      "source": "order-service",
      "target": "postgres-main",
      "mainStat": ""
    }
  ]
}
```

**Поля узла (Node):**

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | string | Уникальный идентификатор узла |
| `title` | string | Отображаемое имя |
| `subTitle` | string | Kubernetes namespace |
| `mainStat` | string | Состояние узла: `ok`, `degraded`, `down`, `unknown` |
| `arc__failed` | float | Сегмент дуги для состояния failed (0.0–1.0) |
| `arc__degraded` | float | Сегмент дуги для состояния degraded (0.0–1.0) |
| `arc__ok` | float | Сегмент дуги для здорового состояния (0.0–1.0) |
| `arc__unknown` | float | Сегмент дуги для неизвестного состояния (0.0–1.0) |

Поля `arc__*` управляют цветным кольцом вокруг каждого узла в панели Grafana Node Graph. Ровно одно поле установлено в `1` для каждого узла в зависимости от его состояния.

**Поля ребра (Edge):**

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | string | Идентификатор ребра (`source--target`) |
| `source` | string | ID исходного узла |
| `target` | string | ID целевого узла |
| `mainStat` | string | Зарезервировано для будущего использования |

---

### `GET /api/v1/timeline/events`

Возвращает события переходов состояний за временной диапазон. Используется фронтендом для отображения маркеров событий на слайдере таймлайна. Запрашивает `app_dependency_status` через Prometheus `query_range` API, детектирует смены состояний и возвращает хронологический список событий.

**Query-параметры:**

| Параметр | Тип | Обязателен | Описание |
|----------|-----|:----------:|----------|
| `start` | string | Да | RFC3339 метка начала диапазона (например, `2026-02-15T00:00:00Z`) |
| `end` | string | Да | RFC3339 метка конца диапазона (должна быть позже `start`) |

Шаг запроса вычисляется автоматически по длительности диапазона:

| Диапазон | Шаг |
|----------|-----|
| ≤ 1ч | 15с |
| ≤ 6ч | 1м |
| ≤ 24ч | 5м |
| ≤ 7д | 15м |
| ≤ 30д | 1ч |
| ≤ 90д | 3ч |
| > 90д | 6ч |

**Ответ:** `200 OK`

```json
[
  {
    "timestamp": "2026-02-15T08:32:15Z",
    "service": "payment-api",
    "fromState": "ok",
    "toState": "timeout",
    "kind": "degradation"
  },
  {
    "timestamp": "2026-02-15T08:45:00Z",
    "service": "payment-api",
    "fromState": "timeout",
    "toState": "ok",
    "kind": "recovery"
  }
]
```

**Поля события:**

| Поле | Тип | Описание |
|------|-----|----------|
| `timestamp` | string | RFC3339 метка времени смены состояния |
| `service` | string | Имя сервиса, где произошёл переход |
| `namespace` | string | Kubernetes namespace (пропускается, если пуст) |
| `fromState` | string | Предыдущий статус зависимости |
| `toState` | string | Новый статус зависимости |
| `kind` | string | `degradation` (ухудшение), `recovery` (восстановление) или `change` (изменение) |

**Ошибки:**
- `400 Bad Request` — отсутствуют `start`/`end`, неверный формат или `start` ≥ `end`

---

### `GET /api/v1/export/{format}`

Экспортирует граф топологии в указанном формате. Поддерживает как форматы данных (JSON, CSV, DOT), так и отрендеренные изображения (PNG, SVG через Graphviz).

**Path-параметры:**

| Параметр | Тип | Обязателен | Описание |
|----------|-----|:----------:|----------|
| `format` | string | Да | Формат экспорта: `json`, `csv`, `dot`, `png`, `svg` |

**Query-параметры:**

| Параметр | Тип | Обязателен | Описание |
|----------|-----|:----------:|----------|
| `scope` | string | Нет | `full` (по умолчанию) — вся топология, `current` — с фильтрацией по namespace/group |
| `namespace` | string | Нет | Фильтр по Kubernetes namespace (только при `scope=current`) |
| `group` | string | Нет | Фильтр по логической группе (только при `scope=current`) |
| `time` | string | Нет | ISO8601/RFC3339 метка времени для исторического экспорта |
| `scale` | int | Нет | Масштаб PNG, 1–4 (по умолчанию `2`). Большие значения дают изображения выше разрешением |

**Заголовки ответа:**

| Формат | Content-Type | Content-Disposition |
|--------|-------------|---------------------|
| `json` | `application/json` | `attachment; filename="dephealth-topology-YYYYMMDD-HHMMSS.json"` |
| `csv` | `application/zip` | `attachment; filename="dephealth-topology-YYYYMMDD-HHMMSS.zip"` |
| `dot` | `text/vnd.graphviz` | `attachment; filename="dephealth-topology-YYYYMMDD-HHMMSS.dot"` |
| `png` | `image/png` | `attachment; filename="dephealth-topology-YYYYMMDD-HHMMSS.png"` |
| `svg` | `image/svg+xml` | `attachment; filename="dephealth-topology-YYYYMMDD-HHMMSS.svg"` |

**Ответ:** `200 OK` — бинарное содержимое файла

**Схема JSON-экспорта:**

```json
{
  "version": "1.0",
  "timestamp": "2026-02-20T12:00:00Z",
  "scope": "full",
  "filters": {},
  "nodes": [
    {
      "id": "order-service",
      "name": "order-service",
      "namespace": "production",
      "group": "payment-team",
      "type": "service",
      "state": "ok",
      "alerts": 0
    }
  ],
  "edges": [
    {
      "source": "order-service",
      "target": "postgres-main",
      "dependency": "postgres-main",
      "type": "postgres",
      "host": "pg-master.db.svc",
      "port": "5432",
      "critical": true,
      "health": 1,
      "status": "",
      "detail": "",
      "latency_ms": 0.0052
    }
  ]
}
```

**CSV-экспорт:** Возвращает ZIP-архив с двумя файлами:
- `nodes.csv` — колонки: `id`, `name`, `namespace`, `group`, `type`, `state`, `alerts`
- `edges.csv` — колонки: `source`, `target`, `dependency`, `type`, `host`, `port`, `critical`, `health`, `status`, `detail`, `latency_ms`

Оба CSV-файла содержат UTF-8 BOM для автоматического определения кодировки в Excel.

**DOT-экспорт:** Возвращает текст в формате [Graphviz DOT](https://graphviz.org/doc/info/lang.html) с подграфами-кластерами по namespace/group, узлами, окрашенными по состоянию, и цветами рёбер, соответствующими легенде подключений в UI.

**PNG/SVG-экспорт:** Требует установленный Graphviz на сервере (включён в Docker-образ). Внутренне генерирует DOT и рендерит его через движок `dot`. Параметр `scale` управляет разрешением PNG через DPI (scale=1 → 72dpi, scale=2 → 144dpi, scale=3 → 216dpi, scale=4 → 288dpi).

**Примеры:**

```bash
# Экспорт всей топологии в JSON
curl -o topology.json https://dephealth.example.com/api/v1/export/json

# Экспорт отфильтрованной топологии в CSV
curl -o topology.zip https://dephealth.example.com/api/v1/export/csv?scope=current&namespace=production

# Экспорт PNG высокого разрешения
curl -o topology.png https://dephealth.example.com/api/v1/export/png?scale=3

# Экспорт исторической топологии в SVG
curl -o topology.svg https://dephealth.example.com/api/v1/export/svg?time=2026-02-15T12:00:00Z
```

**Ошибки:**

| HTTP Status | Условие |
|-------------|---------|
| 400 | Неподдерживаемый формат, неверный scope, неверный формат time, scale вне диапазона (1–4) |
| 502 | Prometheus/VictoriaMetrics недоступен |
| 503 | Graphviz не установлен (только PNG/SVG) |

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
      "cascadeOverview": "dephealth-cascade-overview",
      "rootCause": "dephealth-root-cause",
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
    "enabled": true,
    "severityLevels": [
      {"value": "critical", "color": "#f44336"},
      {"value": "warning", "color": "#ff9800"},
      {"value": "info", "color": "#2196f3"}
    ]
  }
}
```

**Конфигурация алертов:**

| Поле | Тип | Описание |
|------|-----|----------|
| `alerts.enabled` | bool | `true`, если AlertManager настроен (`datasources.alertmanager.url` не пуст), `false` в противном случае. При `false` фронтенд скрывает все алерт-элементы UI (кнопка алертов неактивна, нет бейджей алертов на узлах/рёбрах, нет секций алертов в сайдбарах, нет счётчиков алертов в статус-баре) |
| `alerts.severityLevels` | array | Уровни severity алертов с цветами отображения |

**UID дашбордов:**

| Ключ | Назначение | URL-параметры |
|------|-----------|---------------|
| `cascadeOverview` | Обзор каскадных сбоев | `?var-namespace=<ns>` |
| `rootCause` | Анализ первопричин | `?var-service=<name>&var-namespace=<ns>` |
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
