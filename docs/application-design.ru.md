# dephealth-ui — Проектирование приложения

**Язык:** [English](./application-design.md) | Русский

---

## Назначение

dephealth-ui — веб-приложение для визуализации топологии микросервисов и состояния их зависимостей в реальном времени. Отображает направленный граф сервисов с цветовой индикацией состояний (OK, DEGRADED, DOWN, Unknown), значениями latency на связях и ссылками на dashboards в Grafana.

## Источники данных

Приложение получает данные из двух источников:

- **Prometheus / VictoriaMetrics** — метрики, собираемые проектом [topologymetrics](https://github.com/BigKAA/topologymetrics) (dephealth SDK)
- **AlertManager** — активные алерты по зависимостям

### Метрики topologymetrics

| Метрика | Тип | Значения | Описание |
|---------|-----|----------|----------|
| `app_dependency_health` | Gauge | `1` (здоров) / `0` (недоступен) | Состояние зависимости |
| `app_dependency_latency_seconds` | Histogram | секунды | Latency health check зависимости |

Histogram buckets: `0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0`

### Labels (одинаковые для обеих метрик)

| Label | Обязательный | Описание | Пример |
|-------|:---:|----------|--------|
| `name` | да | Имя приложения (из SDK) | `uniproxy-01` |
| `dependency` | да | Логическое имя зависимости | `postgres-main` |
| `type` | да | Тип подключения | `http`, `grpc`, `tcp`, `postgres`, `mysql`, `redis`, `amqp`, `kafka` |
| `host` | да | Адрес endpoint | `pg-master.db.svc.cluster.local` |
| `port` | да | Порт endpoint | `5432` |
| `critical` | да | Критичность зависимости | `yes`, `no` |
| `role` | нет | Роль экземпляра | `primary`, `replica` |
| `shard` | нет | Идентификатор shard | `shard-01` |
| `vhost` | нет | AMQP virtual host | `/` |

### Модель графа

- **Узлы (nodes)** = Prometheus label `name` (имя приложения из dephealth SDK)
- **Рёбра (edges)** = комбинация `{name → dependency, type, host, port, critical}`
- Каждая уникальная комбинация `{name, dependency, host, port}` = одно направленное ребро
- Флаг `critical` определяет визуальную толщину ребра на графе

### Правила алертов (из Helm chart topologymetrics)

| Алерт | Условие | Severity |
|-------|---------|----------|
| `DependencyDown` | Все endpoints зависимости = 0 в течение 1 мин | critical |
| `DependencyDegraded` | Смешанные значения 0 и 1 для одной зависимости в течение 2 мин | warning |
| `DependencyHighLatency` | P99 > 1с в течение 5 мин | warning |
| `DependencyFlapping` | >4 смены состояния за 15 мин | info |
| `DependencyAbsent` | Метрики отсутствуют полностью в течение 5 мин | warning |

---

## Ограничения развёртывания

- **Сетевая изоляция:** приложение развёртывается **отдельно** от стека мониторинга. Prometheus/VictoriaMetrics и AlertManager находятся в другой сети, недоступной из браузеров пользователей.
- **Масштаб:** 100+ сервисов с dephealth SDK, тысячи рёбер зависимостей.
- **Аутентификация:** настраивается в конфигурации — без auth (внутренний инструмент), Basic auth или OIDC/SSO (Keycloak, LDAP).

**Следствие:** чистое SPA-приложение с Nginx-проксированием к Prometheus **невозможно**. Необходим серверный backend, который обращается к Prometheus/AlertManager и отдаёт фронтенду готовые данные графа.

---

## Архитектура

Комбинированное приложение: Go backend + JS frontend, поставляется как единый Docker-образ.

```
┌─────────────────────┐
│  Браузер (JS SPA)   │  ← Cytoscape.js, получает готовый JSON-граф
│  Cytoscape.js       │  ← Не знает про PromQL, не обращается к Prometheus
└────────┬────────────┘
         │ HTTPS (JSON REST API)
         ▼
┌─────────────────────────────────────┐
│  dephealth-ui (Go binary)           │  ← Единый binary, единый Docker image
│                                     │
│  ┌─ HTTP Server ──────────────────┐ │
│  │  GET /              → SPA      │ │  ← Раздаёт встроенные static-файлы
│  │  GET /api/v1/topology → handler│ │  ← Готовый граф топологии
│  │  GET /api/v1/alerts   → handler│ │  ← Агрегированные алерты
│  │  GET /api/v1/config   → handler│ │  ← Конфигурация для фронтенда
│  └────────────────────────────────┘ │
│                                     │
│  ┌─ Topology Service ─────────────┐ │
│  │  Запросы к Prometheus/VM API   │ │  ← Серверная сторона
│  │  Запросы к AlertManager API v2 │ │
│  │  Построение графа              │ │  ← Вычисление OK/DEGRADED/DOWN
│  │  Кэширование (15-60с TTL)     │ │  ← Один запрос обслуживает всех
│  └────────────────────────────────┘ │
│                                     │
│  ┌─ Auth Module (pluggable) ──────┐ │
│  │  type: "none"  → открытый      │ │  ← Настраивается через YAML/env
│  │  type: "basic" → user/password │ │
│  │  type: "oidc"  → SSO/Keycloak │ │
│  └────────────────────────────────┘ │
└──────────┬──────────────┬───────────┘
           │              │
           ▼              ▼
┌──────────────────┐ ┌────────────────┐
│ VictoriaMetrics  │ │  AlertManager  │
│ (отдельная сеть) │ │ (отдельная     │
│                  │ │  сеть)         │
└──────────────────┘ └────────────────┘
```

---

## Стек технологий

| Компонент | Выбор | Обоснование |
|-----------|-------|-------------|
| **Backend** | Go (`net/http` + `chi`) | Единый binary; официальная библиотека Prometheus client; минимальный Docker-образ (~15-20MB); соответствует K8s-экосистеме |
| **Frontend** | Vanilla JS + Vite | Компактное SPA; Cytoscape.js работает нативно; минимальный bundle; при росте — миграция на React |
| **Визуализация графа** | Cytoscape.js + dagre + fcose | Нативные постоянные подписи на рёбрах; CSS-подобные стили; `cy.batch()` для эффективного обновления; богатая экосистема layout |
| **Layout** | dagre (flat) / fcose (grouped) | dagre — оптимален для DAG-подобной топологии в плоском режиме; fcose — force-directed layout для группировки по namespace с compound nodes |
| **Сборка frontend** | Vite | Быстрый dev server, оптимальный build, HMR |
| **Контейнеризация** | Docker (multi-stage) + Helm chart | Единый образ: Go binary со встроенными SPA static-файлами |

---

## Backend: зоны ответственности

| Ответственность | Детали |
|-----------------|--------|
| **Запросы к Prometheus** | `app_dependency_health`, latency histogram через `prometheus/client_golang/api/v1` |
| **Запросы к AlertManager** | `GET /api/v2/alerts` с фильтрами, стандартный HTTP client |
| **Построение графа** | Узлы из label `name`, рёбра из labels `dependency/type/host/port/critical` |
| **Вычисление состояний** | Корреляция метрик + алертов → OK / DEGRADED / DOWN для каждого узла и ребра |
| **Кэширование** | In-memory cache с настраиваемым TTL (по умолчанию 15с). Один цикл запросов к Prometheus обслуживает всех подключённых пользователей |
| **Генерация Grafana URL** | Формирование URL dashboards с правильными query-параметрами из конфигурации |
| **Auth middleware** | Pluggable: none (passthrough), Basic (bcrypt), OIDC (redirect flow + token validation) |
| **Раздача static-файлов** | SPA-ассеты встроены через Go `embed` package, раздаются по `/` |

---

## Модель состояний

dephealth-ui использует модель из 4 состояний для узлов и рёбер:

| Состояние | Цвет | Описание |
|-----------|------|----------|
| **ok** | Зелёный (`#4caf50`) | Все зависимости здоровы |
| **degraded** | Жёлтый (`#ff9800`) | Некоторые зависимости недоступны, сервис работает |
| **down** | Красный (`#f44336`) | Сервис недоступен (все рёбра stale или действительно down) |
| **unknown** | Серый (`#9e9e9e`) | Нет данных (нет рёбер или все рёбра stale) |

### Состояние сервис-ноды (Backend)

Backend вычисляет состояние сервис-ноды в `calcServiceNodeState()` по здоровью исходящих рёбер:

- **Нет рёбер** → `unknown`
- **Любое ребро с health=0** → `degraded`
- **Все рёбра здоровы (health=1)** → `ok`

> **Важно:** `calcServiceNodeState` никогда не возвращает `"down"`. Backend присваивает сервис-нодам только `ok`, `degraded` или `unknown`. Состояние `down` устанавливается только когда **все** исходящие рёбра stale (метрики пропали) — это обрабатывается логикой stale detection.

### Состояние dependency-ноды (Backend)

Dependency-ноды определяют состояние по входящим рёбрам:

- **Все входящие рёбра stale** → `down` (с `stale=true`)
- **Смешанные stale/live** → состояние из non-stale рёбер
- **health=1** → `ok`
- **health=0** → `down`

### Состояние ребра

| Условие | Состояние |
|---------|-----------|
| health=1 | `ok` |
| health=0 | `down` |
| Stale (метрики пропали) | `unknown` |

### Расширения состояний на фронтенде

Фронтенд расширяет модель состояний каскадными предупреждениями (см. [Каскадные предупреждения](#каскадные-предупреждения)). Узлы, получающие каскадное распространение, показывают бейдж `⚠ N` и попадают в виртуальный фильтр `warning`.

---

## REST API

### `GET /api/v1/topology`

Возвращает полный граф топологии с предвычисленными состояниями:

```json
{
  "nodes": [
    {
      "id": "order-service",
      "label": "Order Service",
      "state": "ok",
      "type": "service",
      "dependencyCount": 3,
      "grafanaUrl": "https://grafana.example.com/d/dephealth-service-status?var-service=order-service"
    },
    {
      "id": "postgres-main",
      "label": "postgres-main",
      "state": "degraded",
      "type": "postgres"
    }
  ],
  "edges": [
    {
      "source": "order-service",
      "target": "postgres-main",
      "latency": "5.2ms",
      "latencyRaw": 0.0052,
      "health": 1,
      "state": "ok",
      "critical": true,
      "grafanaUrl": "https://grafana.example.com/d/dephealth-link-status?var-dependency=postgres-main&var-host=pg-host&var-port=5432"
    }
  ],
  "alerts": [
    {
      "service": "postgres-main",
      "alertname": "DependencyDegraded",
      "severity": "warning",
      "since": "2026-02-08T08:30:00Z"
    }
  ],
  "meta": {
    "cachedAt": "2026-02-08T09:15:30Z",
    "ttl": 15,
    "nodeCount": 42,
    "edgeCount": 187
  }
}
```

### `GET /api/v1/config`

Возвращает конфигурацию, необходимую фронтенду (Grafana base URL, dashboard UID, настройки отображения).

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
  "cache": { "ttl": 15 },
  "auth": { "type": "oidc" },
  "alerts": {
    "severityLevels": [
      {"value": "critical", "color": "#f44336"},
      {"value": "warning", "color": "#ff9800"},
      {"value": "info", "color": "#2196f3"}
    ]
  }
}
```

**Dashboards:**

| UID | Назначение | Query-параметры |
|-----|-----------|-----------------|
| `serviceStatus` | Состояние одного сервиса | `?var-service=<name>` |
| `linkStatus` | Состояние одной зависимости | `?var-dependency=<dep>&var-host=<host>&var-port=<port>` |
| `serviceList` | Список всех сервисов | — |
| `servicesStatus` | Обзор состояния всех сервисов | — |
| `linksStatus` | Обзор состояния всех связей | — |

---

## Конфигурация приложения

```yaml
# dephealth-ui.yaml
server:
  listen: ":8080"

datasources:
  prometheus:
    url: "http://victoriametrics.monitoring.svc:8428"
    # username: "reader"
    # password: "secret"
  alertmanager:
    url: "http://alertmanager.monitoring.svc:9093"

cache:
  ttl: 15s

auth:
  type: "none"   # "none" | "basic" | "oidc"

grafana:
  baseUrl: "https://grafana.example.com"
  dashboards:
    serviceStatus: "dephealth-service-status"
    linkStatus: "dephealth-link-status"
    serviceList: "dephealth-service-list"
    servicesStatus: "dephealth-services-status"
    linksStatus: "dephealth-links-status"

topology:
  lookback: "0"  # "0" = отключено, "1h", "6h", "24h"
```

---

## Frontend: поведение

Frontend — тонкий слой визуализации. Вся трансформация данных происходит на backend.

### Основной цикл

1. Frontend запрашивает `GET /api/v1/topology` с интервалом, указанным в `meta.ttl`
2. Получает готовый JSON с узлами, рёбрами, алертами и meta-информацией
3. Обновляет граф Cytoscape.js через `cy.batch()` (эффективное массовое обновление)

### Визуализация

- **Узлы:** цвет зависит от `state` — зелёный (OK), жёлтый (DEGRADED), красный (DOWN), серый (Unknown/stale); динамический размер по длине текста; цветная полоска namespace
- **Рёбра:** направленные стрелки с постоянными подписями latency; цвет ребра по `state`; толщина ребра по `critical` (критичные — толще)
- **Stale-ноды:** серый фон (`#9e9e9e`), пунктирная рамка, скрытая latency; tooltip «Метрики пропали»
- **Клик по узлу/ребру:** открывает боковую панель с деталями (состояние, namespace, инстансы, связи, алерты) и секцией ссылок на Grafana dashboards
- **Контекстное меню (правый клик):** Открыть в Grafana, Копировать URL, Детали
- **Layout:** dagre (плоский режим, LR/TB) или fcose (режим группировки по namespace)

![Контекстное меню на узле сервиса](./images/context-menu-grafana.png)

### Группировка по namespace

Группировка позволяет визуально объединить сервисы по Kubernetes namespace в составные узлы (compound nodes) Cytoscape.js.

**Режимы:**
- **Flat mode (dagre):** все узлы отображаются на одном уровне, layout dagre
- **Grouped mode (fcose):** узлы сгруппированы в namespace-контейнеры, layout fcose

**Collapse/Expand:**
- Двойной клик по группе или кнопка «Развернуть namespace» в sidebar → сворачивание/разворачивание
- Свёрнутый namespace показывает: наихудшее состояние детей, количество сервисов, суммарные алерты
- Рёбра между свёрнутыми namespace автоматически агрегируются (показывают count `×N`)
- Состояние collapse/expand сохраняется в `localStorage`
- При обновлении данных (auto-refresh) — свёрнутые namespace остаются свёрнутыми

**Click-to-expand навигация:**
- В sidebar свёрнутого namespace — кликабельный список сервисов с цветными индикаторами состояния
- Клик по сервису → namespace разворачивается → камера центрируется на выбранном сервисе → sidebar показывает детали сервиса
- В sidebar ребра — клик по узлу из свёрнутого namespace также разворачивает и навигирует к оригинальному сервису

![Основной вид со свёрнутыми namespace](./images/dephealth-main-view.png)

![Sidebar свёрнутого namespace с кликабельными сервисами](./images/sidebar-collapsed-namespace.png)

### Боковая панель (Sidebar)

Три типа боковых панелей:

**1. Node Sidebar** — при клике по узлу-сервису:
- Основная информация (state, type, namespace)
- Активные алерты (с severity)
- Список инстансов (pod name, IP:port) — для service-узлов
- Связанные рёбра (входящие/исходящие с latency и навигацией)
- Кнопка «Open in Grafana» (открывает serviceStatus dashboard)
- Секция **Grafana Dashboards** — ссылки на все dashboards с контекстно-зависимыми query-параметрами

![Sidebar узла с алертами и ссылками Grafana](./images/sidebar-grafana-section.png)

![Sidebar stale/unknown узла](./images/sidebar-stale-node.png)

**2. Edge Sidebar** — при клике по ребру:
- Состояние, тип, latency, критичность
- Активные алерты для данной связи
- Связанные узлы (source/target) с кликабельной навигацией
- Кнопка «Open in Grafana» (открывает linkStatus dashboard)
- Секция Grafana Dashboards

![Sidebar ребра с алертами и связанными узлами](./images/sidebar-edge-details.png)

**3. Collapsed Namespace Sidebar** — при клике по свёрнутому namespace:
- Наихудшее состояние, количество сервисов, суммарные алерты
- Кликабельный список сервисов с цветными точками состояния и стрелкой «Перейти к узлу →»
- Кнопка «Развернуть namespace»

### Интернационализация (i18n)

Фронтенд поддерживает EN и RU. Кнопка переключения языка в тулбаре. Все элементы UI, фильтры, легенда, статусбар, боковая панель и контекстное меню локализованы. Язык сохраняется в `localStorage`.

| EN | RU |
|----|----|
| ![Интерфейс на английском](./images/dephealth-main-view.png) | ![Интерфейс на русском](./images/dephealth-russian-ui.png) |

### Каскадные предупреждения

Каскадные предупреждения визуализируют распространение сбоев через критические зависимости. Когда сервис падает, все вышестоящие сервисы, критически зависящие от него (прямо или транзитивно), получают индикаторы каскадного предупреждения с указанием корневой причины.

**Ключевой принцип:** Каскадные предупреждения распространяются только через рёбра с `critical=true`. Сбои некритических зависимостей не вызывают каскадное распространение.

#### Алгоритм

Каскадные вычисления выполняются целиком на фронтенде (`cascade.js`) после каждого обновления данных:

**Фаза 1 — Поиск корневых причин** (`findRealRootCauses`):
Для каждой `down` сервис-ноды — прослеживание вниз по критическим рёбрам до реально недоступной зависимости (терминальная корневая причина). Пример: если `A(down) → B(critical) → C(unknown)`, корневая причина — `C`, а не `A`.

**Фаза 2 — BFS вверх** (`computeCascadeWarnings`):
От каждой `down` сервис-ноды — BFS вверх по входящим критическим рёбрам. Каждый вышестоящий узел (не находящийся в состоянии `down`) получает данные каскадного предупреждения с указанием реальной корневой причины.

```
fetchTopology() → renderGraph() → computeCascadeWarnings(cy) → updateBadges()
```

#### Свойства данных узла

| Свойство | Тип | Описание |
|----------|-----|----------|
| `cascadeCount` | number | Количество различных корневых причин, влияющих на узел |
| `cascadeSources` | string[] | Массив ID узлов — корневых причин |
| `inCascadeChain` | boolean | `true` для узлов в цепочке сбоя (down-ноды + корневые причины) — используется системой фильтров |

#### Визуальное представление

- **Бейдж каскада:** `⚠ N` в форме pill на верхнем левом углу затронутых узлов (N = количество корневых причин)
- **Tooltip:** показывает «Каскадное предупреждение: ↳ service-name (state)» для каждой корневой причины
- **Down-ноды** не показывают каскадные бейджи (они сами являются причиной сбоя, а не получателем предупреждения)

![Топология с каскадными бейджами на вышестоящих узлах](./images/cascade-warnings-main.png)

![Tooltip с корневой причиной каскадного предупреждения](./images/cascade-warning-tooltip.png)

#### Фильтры состояний

Панель фильтров включает виртуальное состояние `warning` наряду с backend-состояниями:

![Фильтры состояний: ok, degraded, down, unknown, warning](./images/state-filters.png)

#### Интеграция с фильтрами

Система фильтров включает виртуальное состояние `warning` (не является backend-состоянием):

- Узел соответствует `warning`, если `cascadeCount > 0` и `state !== 'down'`
- Узлы с `inCascadeChain=true` также соответствуют фильтру `warning` (показывает полную цепочку сбоя)

**Pass 1.5 — Видимость цепочки degraded/down:**
При активном фильтре `degraded` или `down` система фильтров также показывает нижележащие non-ok зависимости, чтобы пользователь видел ПОЧЕМУ узел degraded или down.

---

## PromQL-запросы (выполняются на backend)

```promql
# Все рёбра топологии (instant)
group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health)

# Все рёбра за lookback-окно (для stale node retention)
group by (name, namespace, dependency, type, host, port, critical) (last_over_time(app_dependency_health[LOOKBACK]))

# Текущее состояние всех зависимостей
app_dependency_health

# Средняя latency
rate(app_dependency_latency_seconds_sum[5m]) / rate(app_dependency_latency_seconds_count[5m])

# P99 latency
histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket[5m]))

# Degraded: часть endpoints up, часть down
(count by (name, namespace, dependency, type) (app_dependency_health == 0) > 0)
and
(count by (name, namespace, dependency, type) (app_dependency_health == 1) > 0)
```

### Удержание stale-нод (lookback window)

Когда сервис перестаёт отправлять метрики (crash, scale-down, сетевые проблемы), его time series становятся «stale» в Prometheus через ~5 минут и исчезают из instant-запросов. По умолчанию узел просто пропадает с графа.

Функция **lookback window** (`topology.lookback`) сохраняет исчезнувшие узлы на графе в состоянии `state="unknown"` на настраиваемый период.

**Как это работает:**

1. **Topology query** использует `last_over_time(metric[LOOKBACK])` — возвращает ВСЕ рёбра за окно lookback (текущие + stale)
2. **Health query** использует instant-запрос — возвращает ТОЛЬКО текущие рёбра (живые time series)
3. Рёбра, присутствующие в topology, но НЕ в health → помечаются как **stale** (`state="unknown"`, `Stale=true`)
4. Узлы, где ВСЕ рёбра stale → `state="unknown"`; смешанные узлы используют non-stale рёбра для вычисления состояния

**Визуализация на фронтенде:**
- Stale-ноды: серый фон (`#9e9e9e`), пунктирная рамка
- Stale-рёбра: серые пунктирные линии, latency скрыта
- Tooltip показывает «Метрики пропали» / «Metrics disappeared»

**Конфигурация:** `topology.lookback` (по умолчанию: `0` = отключено). Рекомендуемые значения: `1h`, `6h`, `24h`. Минимум: `1m`. Env: `DEPHEALTH_TOPOLOGY_LOOKBACK`.

Совместимо с Prometheus и VictoriaMetrics (`last_over_time()` поддерживается обоими).

---

## Развёртывание

### Docker

Multi-stage build:
1. **Stage 1 (frontend):** Node.js + Vite → собирает SPA в `dist/`
2. **Stage 2 (backend):** Go → компилирует binary со встроенными static-файлами из Stage 1
3. **Stage 3 (runtime):** Минимальный образ (scratch / distroless) с единственным binary

Результат: Docker-образ ~15-20MB.

### Helm Chart

- Deployment с одним контейнером
- ConfigMap для `dephealth-ui.yaml`
- Secret для auth credentials (basic passwords, OIDC client secret)
- Service (ClusterIP или LoadBalancer)
- HTTPRoute (Gateway API) для внешнего доступа
- Опциональный Certificate (cert-manager) для TLS

### Конфигурация через environment

Все параметры из YAML можно переопределить через переменные окружения:
- `DEPHEALTH_SERVER_LISTEN`
- `DEPHEALTH_DATASOURCES_PROMETHEUS_URL`
- `DEPHEALTH_DATASOURCES_ALERTMANAGER_URL`
- `DEPHEALTH_CACHE_TTL`
- `DEPHEALTH_AUTH_TYPE`
- `DEPHEALTH_GRAFANA_BASEURL`
- `DEPHEALTH_TOPOLOGY_LOOKBACK`

---

## См. также

- [Справочник REST API](./API.ru.md) — Все endpoint'ы и форматы ответов
- [Спецификация метрик](./METRICS.ru.md) — Формат обязательных метрик и руководство по интеграции
- [Руководство по развёртыванию](../deploy/helm/dephealth-ui/README.ru.md) — Kubernetes & Helm
