# Metrics Format Specification | Спецификация формата метрик

**Language:** [English](#english) | [Русский](#русский)

---

## English

### Overview

**dephealth-ui** requires Prometheus-compatible metrics that describe service dependencies and their health status. These metrics are collected by applications instrumented with the [dephealth SDK](https://github.com/BigKAA/topologymetrics).

This document specifies:
- Required metric names and types
- Mandatory and optional labels
- Value formats and constraints
- PromQL queries used by the application
- Integration examples

---

### Required Metrics

#### 1. `app_dependency_health`

**Type:** Gauge  
**Description:** Health status of a service dependency endpoint.

**Values:**
- `1` — dependency is healthy (responding successfully)
- `0` — dependency is down (unreachable or failing health checks)

**Required Labels:**

| Label | Required | Description | Example Values |
|-------|:--------:|-------------|----------------|
| `name` | ✅ | Application name (service identifier) | `order-service`, `payment-api`, `user-backend` |
| `namespace` | ✅ | Kubernetes namespace or logical grouping | `production`, `staging`, `team-alpha` |
| `dependency` | ✅ | Logical name of the dependency | `postgres-main`, `redis-cache`, `auth-service` |
| `type` | ✅ | Connection protocol/type | `http`, `grpc`, `tcp`, `postgres`, `mysql`, `redis`, `mongodb`, `amqp`, `kafka` |
| `host` | ✅ | Target endpoint hostname/IP | `pg-master.db.svc.cluster.local`, `10.0.1.5` |
| `port` | ✅ | Target endpoint port | `5432`, `6379`, `8080` |
| `critical` | ✅ | Dependency criticality flag | `yes`, `no` |

**Optional Labels:**

| Label | Description | Example Values |
|-------|-------------|----------------|
| `role` | Instance role (for replicated systems) | `primary`, `replica`, `standby` |
| `shard` | Shard identifier (for sharded systems) | `shard-01`, `shard-02` |
| `vhost` | AMQP virtual host | `/`, `/app` |
| `pod` | Kubernetes pod name | `order-service-7d9f8b-xyz12` |
| `instance` | Prometheus instance label | `10.244.1.5:9090` |
| `job` | Prometheus job label | `order-service` |

**Example:**
```prometheus
app_dependency_health{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",role="primary"} 1
app_dependency_health{name="order-service",namespace="production",dependency="redis-cache",type="redis",host="redis.cache.svc",port="6379",critical="no"} 1
app_dependency_health{name="payment-api",namespace="production",dependency="auth-service",type="http",host="auth.svc",port="8080",critical="yes"} 0
```

---

#### 2. `app_dependency_latency_seconds`

**Type:** Histogram  
**Description:** Health check latency for dependency endpoints (in seconds).

**Buckets:** `0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0`

**Required Labels:** Same as `app_dependency_health` (all required labels must match).

**Generated Time Series:**
- `app_dependency_latency_seconds_bucket{..., le="0.001"}` — count of requests ≤ 1ms
- `app_dependency_latency_seconds_bucket{..., le="0.005"}` — count of requests ≤ 5ms
- `app_dependency_latency_seconds_bucket{..., le="+Inf"}` — total count
- `app_dependency_latency_seconds_sum{...}` — sum of all latencies
- `app_dependency_latency_seconds_count{...}` — total number of health checks

**Example:**
```prometheus
app_dependency_latency_seconds_bucket{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",le="0.001"} 45
app_dependency_latency_seconds_bucket{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",le="0.005"} 98
app_dependency_latency_seconds_bucket{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",le="+Inf"} 100
app_dependency_latency_seconds_sum{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes"} 0.152
app_dependency_latency_seconds_count{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes"} 100
```

---

### PromQL Queries Used by dephealth-ui

The application executes the following queries against Prometheus/VictoriaMetrics:

#### 1. **Topology Discovery** — extract all unique edges
```promql
group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health)
```
**Purpose:** Discover all service→dependency relationships in the system.

#### 2. **Health State** — current health value per edge
```promql
app_dependency_health
```
**Purpose:** Determine if each dependency endpoint is currently UP (1) or DOWN (0).

#### 3. **Average Latency** — mean latency per edge
```promql
rate(app_dependency_latency_seconds_sum[5m]) / rate(app_dependency_latency_seconds_count[5m])
```
**Purpose:** Calculate rolling 5-minute average latency for each dependency.

#### 4. **P99 Latency** — 99th percentile latency per edge
```promql
histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket[5m]))
```
**Purpose:** Calculate P99 latency to identify slow dependencies.

#### 5. **Service Instances** — list all instances for a service
```promql
group by (instance, pod, job) (app_dependency_health{name="<service-name>"})
```
**Purpose:** Display all running instances/pods for a selected service in the sidebar.

---

### Graph Model

- **Nodes (Vertices):** Unique values of `name` label → represent services/applications
- **Edges (Directed):** Unique combinations of `{name, namespace, dependency, type, host, port, critical}` → represent service→dependency connections
- **Edge Properties:**
  - **critical:** visual thickness (critical dependencies are displayed thicker)
  - **latency:** displayed as label on edge
  - **health:** affects edge color (green=OK, yellow=degraded, red=down)

**Example Topology:**
```
order-service (node)
  ├─→ postgres-main (edge: critical=yes, type=postgres, latency=5ms)
  ├─→ redis-cache (edge: critical=no, type=redis, latency=1ms)
  └─→ payment-api (edge: critical=yes, type=http, latency=15ms)
```

---

### State Calculation Rules

**Node State (Service):**
- **DOWN:** All critical dependencies are down
- **DEGRADED:** Some critical dependencies are down OR high latency/alerts present
- **OK:** All critical dependencies are healthy

**Edge State (Dependency Link):**
- **DOWN:** `app_dependency_health = 0`
- **DEGRADED:** Mixed health states (0 and 1) across multiple endpoints, OR active AlertManager alerts
- **OK:** `app_dependency_health = 1`, no alerts

---

### AlertManager Integration

dephealth-ui queries AlertManager API v2 for active alerts:

**Expected Alert Labels:**
- `alertname` — alert rule name (e.g., `DependencyDown`, `DependencyHighLatency`)
- `severity` — `critical`, `warning`, `info`
- `name` — service name (matches metric label)
- `dependency` — dependency name (matches metric label)

**Common Alerts** (from topologymetrics project):
- `DependencyDown` — all endpoints down for 1min (critical)
- `DependencyDegraded` — mixed UP/DOWN states for 2min (warning)
- `DependencyHighLatency` — P99 > 1s for 5min (warning)
- `DependencyFlapping` — >4 state changes in 15min (info)
- `DependencyAbsent` — metrics missing for 5min (warning)

---

### Integration Guide

#### Step 1: Instrument Your Application

Use the [dephealth SDK](https://github.com/BigKAA/topologymetrics) to automatically emit metrics:

**Go Example:**
```go
import "github.com/BigKAA/topologymetrics/sdk-go"

// Initialize SDK
sdk, _ := dephealth.New(dephealth.Config{
    ServiceName: "order-service",
    Namespace:   "production",
    MetricsAddr: ":9090",
})

// Register dependencies
pgDep := sdk.RegisterDependency(dephealth.Dependency{
    Name:     "postgres-main",
    Type:     "postgres",
    Host:     "pg-master.db.svc",
    Port:     5432,
    Critical: true,
    HealthCheck: func(ctx context.Context) error {
        return db.PingContext(ctx)
    },
})

// SDK automatically exports metrics to /metrics endpoint
```

#### Step 2: Configure Prometheus Scraping

Ensure Prometheus scrapes your application's `/metrics` endpoint:

```yaml
scrape_configs:
  - job_name: 'order-service'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: [production]
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
```

#### Step 3: Deploy AlertManager Rules

Install alerting rules (example from topologymetrics Helm chart):

```yaml
groups:
  - name: dephealth
    rules:
      - alert: DependencyDown
        expr: |
          (count by (name, namespace, dependency) (app_dependency_health == 0) > 0)
          and
          (count by (name, namespace, dependency) (app_dependency_health == 1) == 0)
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Dependency {{ $labels.dependency }} is completely down"
```

#### Step 4: Configure dephealth-ui

Point the application to your Prometheus and AlertManager:

```yaml
# config.yaml
datasources:
  prometheus:
    url: "http://victoriametrics.monitoring.svc:8428"
  alertmanager:
    url: "http://alertmanager.monitoring.svc:9093"
```

---

### Validation Checklist

✅ Metrics `app_dependency_health` and `app_dependency_latency_seconds` are exposed  
✅ All required labels are present: `name`, `namespace`, `dependency`, `type`, `host`, `port`, `critical`  
✅ Label values are consistent (same labels for health + latency metrics)  
✅ Health values are exactly `0` or `1` (not strings, not other numbers)  
✅ Latency histogram has standard buckets  
✅ Prometheus successfully scrapes metrics (check `/targets` page)  
✅ AlertManager is configured and reachable  

**Test Query:**
```promql
# Should return your service topology
group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health)
```

---

### Troubleshooting

**Problem:** Topology graph is empty  
**Solution:** Verify metrics are present in Prometheus:
```promql
count(app_dependency_health)
```
If zero, check Prometheus scrape configuration.

**Problem:** Edges missing in topology  
**Solution:** Ensure all required labels are present and non-empty. Query:
```promql
app_dependency_health{name="", namespace="", dependency="", type="", host="", port=""}
```
Should return 0 results (no metrics with empty required labels).

**Problem:** Latency not displayed  
**Solution:** Check histogram metrics:
```promql
rate(app_dependency_latency_seconds_count[5m])
```
If zero, health checks are not recording latency.

**Problem:** Wrong node states  
**Solution:** Verify AlertManager integration and alert labels match metric labels.

---

## Русский

### Обзор

**dephealth-ui** требует метрики, совместимые с Prometheus, которые описывают зависимости сервисов и их состояние здоровья. Эти метрики собираются приложениями, инструментированными с помощью [dephealth SDK](https://github.com/BigKAA/topologymetrics).

Данный документ определяет:
- Обязательные имена и типы метрик
- Обязательные и опциональные метки
- Форматы значений и ограничения
- PromQL-запросы, используемые приложением
- Примеры интеграции

---

### Обязательные метрики

#### 1. `app_dependency_health`

**Тип:** Gauge  
**Описание:** Состояние здоровья endpoint'а зависимости сервиса.

**Значения:**
- `1` — зависимость здорова (успешно отвечает)
- `0` — зависимость недоступна (не отвечает или проваливает health check)

**Обязательные метки:**

| Метка | Обязательна | Описание | Примеры значений |
|-------|:-----------:|----------|------------------|
| `name` | ✅ | Имя приложения (идентификатор сервиса) | `order-service`, `payment-api`, `user-backend` |
| `namespace` | ✅ | Kubernetes namespace или логическая группа | `production`, `staging`, `team-alpha` |
| `dependency` | ✅ | Логическое имя зависимости | `postgres-main`, `redis-cache`, `auth-service` |
| `type` | ✅ | Протокол/тип подключения | `http`, `grpc`, `tcp`, `postgres`, `mysql`, `redis`, `mongodb`, `amqp`, `kafka` |
| `host` | ✅ | Hostname/IP целевого endpoint'а | `pg-master.db.svc.cluster.local`, `10.0.1.5` |
| `port` | ✅ | Порт целевого endpoint'а | `5432`, `6379`, `8080` |
| `critical` | ✅ | Флаг критичности зависимости | `yes`, `no` |

**Опциональные метки:**

| Метка | Описание | Примеры значений |
|-------|----------|------------------|
| `role` | Роль инстанса (для реплицированных систем) | `primary`, `replica`, `standby` |
| `shard` | Идентификатор шарда (для шардированных систем) | `shard-01`, `shard-02` |
| `vhost` | AMQP virtual host | `/`, `/app` |
| `pod` | Имя Kubernetes pod'а | `order-service-7d9f8b-xyz12` |
| `instance` | Prometheus instance label | `10.244.1.5:9090` |
| `job` | Prometheus job label | `order-service` |

**Пример:**
```prometheus
app_dependency_health{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",role="primary"} 1
app_dependency_health{name="order-service",namespace="production",dependency="redis-cache",type="redis",host="redis.cache.svc",port="6379",critical="no"} 1
app_dependency_health{name="payment-api",namespace="production",dependency="auth-service",type="http",host="auth.svc",port="8080",critical="yes"} 0
```

---

#### 2. `app_dependency_latency_seconds`

**Тип:** Histogram  
**Описание:** Latency health check'ов для endpoint'ов зависимостей (в секундах).

**Buckets:** `0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0`

**Обязательные метки:** Те же, что и у `app_dependency_health` (все обязательные метки должны совпадать).

**Генерируемые time series:**
- `app_dependency_latency_seconds_bucket{..., le="0.001"}` — количество запросов ≤ 1ms
- `app_dependency_latency_seconds_bucket{..., le="0.005"}` — количество запросов ≤ 5ms
- `app_dependency_latency_seconds_bucket{..., le="+Inf"}` — общее количество
- `app_dependency_latency_seconds_sum{...}` — сумма всех latency
- `app_dependency_latency_seconds_count{...}` — общее количество health check'ов

**Пример:**
```prometheus
app_dependency_latency_seconds_bucket{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",le="0.001"} 45
app_dependency_latency_seconds_bucket{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",le="0.005"} 98
app_dependency_latency_seconds_bucket{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes",le="+Inf"} 100
app_dependency_latency_seconds_sum{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes"} 0.152
app_dependency_latency_seconds_count{name="order-service",namespace="production",dependency="postgres-main",type="postgres",host="pg-master.db.svc",port="5432",critical="yes"} 100
```

---

### PromQL-запросы, используемые dephealth-ui

Приложение выполняет следующие запросы к Prometheus/VictoriaMetrics:

#### 1. **Обнаружение топологии** — извлечение всех уникальных рёбер
```promql
group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health)
```
**Назначение:** Обнаружить все связи сервис→зависимость в системе.

#### 2. **Состояние здоровья** — текущее значение health для каждого ребра
```promql
app_dependency_health
```
**Назначение:** Определить, доступен ли каждый endpoint зависимости (UP=1, DOWN=0).

#### 3. **Средний latency** — среднее значение latency для каждого ребра
```promql
rate(app_dependency_latency_seconds_sum[5m]) / rate(app_dependency_latency_seconds_count[5m])
```
**Назначение:** Вычислить скользящее среднее latency за 5 минут для каждой зависимости.

#### 4. **P99 Latency** — 99-й перцентиль latency для каждого ребра
```promql
histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket[5m]))
```
**Назначение:** Вычислить P99 latency для выявления медленных зависимостей.

#### 5. **Инстансы сервиса** — список всех инстансов сервиса
```promql
group by (instance, pod, job) (app_dependency_health{name="<service-name>"})
```
**Назначение:** Отобразить все запущенные инстансы/pod'ы для выбранного сервиса в боковой панели.

---

### Модель графа

- **Узлы (Vertices):** Уникальные значения метки `name` → представляют сервисы/приложения
- **Рёбра (Directed):** Уникальные комбинации `{name, namespace, dependency, type, host, port, critical}` → представляют связи сервис→зависимость
- **Свойства рёбер:**
  - **critical:** визуальная толщина (критичные зависимости отображаются толще)
  - **latency:** отображается как подпись на ребре
  - **health:** влияет на цвет ребра (зелёный=OK, жёлтый=degraded, красный=down)

**Пример топологии:**
```
order-service (узел)
  ├─→ postgres-main (ребро: critical=yes, type=postgres, latency=5ms)
  ├─→ redis-cache (ребро: critical=no, type=redis, latency=1ms)
  └─→ payment-api (ребро: critical=yes, type=http, latency=15ms)
```

---

### Правила вычисления состояний

**Состояние узла (сервиса):**
- **DOWN:** Все критичные зависимости недоступны
- **DEGRADED:** Часть критичных зависимостей недоступна ИЛИ высокий latency/алерты
- **OK:** Все критичные зависимости здоровы

**Состояние ребра (связи с зависимостью):**
- **DOWN:** `app_dependency_health = 0`
- **DEGRADED:** Смешанные состояния (0 и 1) на нескольких endpoint'ах, ИЛИ активные алерты AlertManager
- **OK:** `app_dependency_health = 1`, нет алертов

---

### Интеграция с AlertManager

dephealth-ui запрашивает AlertManager API v2 для получения активных алертов:

**Ожидаемые метки алертов:**
- `alertname` — имя правила алерта (например, `DependencyDown`, `DependencyHighLatency`)
- `severity` — `critical`, `warning`, `info`
- `name` — имя сервиса (соответствует метке метрики)
- `dependency` — имя зависимости (соответствует метке метрики)

**Типичные алерты** (из проекта topologymetrics):
- `DependencyDown` — все endpoint'ы недоступны в течение 1 мин (critical)
- `DependencyDegraded` — смешанные состояния UP/DOWN в течение 2 мин (warning)
- `DependencyHighLatency` — P99 > 1с в течение 5 мин (warning)
- `DependencyFlapping` — >4 смен состояния за 15 мин (info)
- `DependencyAbsent` — метрики отсутствуют в течение 5 мин (warning)

---

### Руководство по интеграции

#### Шаг 1: Инструментировать приложение

Используйте [dephealth SDK](https://github.com/BigKAA/topologymetrics) для автоматического экспорта метрик:

**Пример на Go:**
```go
import "github.com/BigKAA/topologymetrics/sdk-go"

// Инициализация SDK
sdk, _ := dephealth.New(dephealth.Config{
    ServiceName: "order-service",
    Namespace:   "production",
    MetricsAddr: ":9090",
})

// Регистрация зависимостей
pgDep := sdk.RegisterDependency(dephealth.Dependency{
    Name:     "postgres-main",
    Type:     "postgres",
    Host:     "pg-master.db.svc",
    Port:     5432,
    Critical: true,
    HealthCheck: func(ctx context.Context) error {
        return db.PingContext(ctx)
    },
})

// SDK автоматически экспортирует метрики на endpoint /metrics
```

#### Шаг 2: Настроить scraping в Prometheus

Убедитесь, что Prometheus собирает метрики с endpoint'а `/metrics` вашего приложения:

```yaml
scrape_configs:
  - job_name: 'order-service'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: [production]
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
```

#### Шаг 3: Развернуть правила AlertManager

Установите правила алертинга (пример из Helm chart topologymetrics):

```yaml
groups:
  - name: dephealth
    rules:
      - alert: DependencyDown
        expr: |
          (count by (name, namespace, dependency) (app_dependency_health == 0) > 0)
          and
          (count by (name, namespace, dependency) (app_dependency_health == 1) == 0)
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Зависимость {{ $labels.dependency }} полностью недоступна"
```

#### Шаг 4: Настроить dephealth-ui

Укажите приложению адреса Prometheus и AlertManager:

```yaml
# config.yaml
datasources:
  prometheus:
    url: "http://victoriametrics.monitoring.svc:8428"
  alertmanager:
    url: "http://alertmanager.monitoring.svc:9093"
```

---

### Чеклист проверки

✅ Метрики `app_dependency_health` и `app_dependency_latency_seconds` экспортируются  
✅ Все обязательные метки присутствуют: `name`, `namespace`, `dependency`, `type`, `host`, `port`, `critical`  
✅ Значения меток консистентны (одинаковые метки для метрик health + latency)  
✅ Значения health — строго `0` или `1` (не строки, не другие числа)  
✅ Histogram latency имеет стандартные buckets  
✅ Prometheus успешно собирает метрики (проверьте страницу `/targets`)  
✅ AlertManager настроен и доступен  

**Тестовый запрос:**
```promql
# Должен вернуть вашу топологию сервисов
group by (name, namespace, dependency, type, host, port, critical) (app_dependency_health)
```

---

### Решение проблем

**Проблема:** Граф топологии пустой  
**Решение:** Проверьте наличие метрик в Prometheus:
```promql
count(app_dependency_health)
```
Если ноль — проверьте конфигурацию scrape в Prometheus.

**Проблема:** Отсутствуют рёбра в топологии  
**Решение:** Убедитесь, что все обязательные метки присутствуют и не пустые. Запрос:
```promql
app_dependency_health{name="", namespace="", dependency="", type="", host="", port=""}
```
Должен вернуть 0 результатов (нет метрик с пустыми обязательными метками).

**Проблема:** Не отображается latency  
**Решение:** Проверьте метрики histogram:
```promql
rate(app_dependency_latency_seconds_count[5m])
```
Если ноль — health check'и не записывают latency.

**Проблема:** Неверные состояния узлов  
**Решение:** Проверьте интеграцию с AlertManager и соответствие меток алертов меткам метрик.

---

## See Also | См. также

- [Application Design](./application-design.md) — Full architecture overview | Полный обзор архитектуры
- [API Documentation](./API.md) — REST API endpoints | REST API endpoints
- [Deployment Guide](./DEPLOYMENT.md) — Kubernetes & Helm | Kubernetes & Helm
- [dephealth SDK](https://github.com/BigKAA/topologymetrics) — Official instrumentation library | Официальная библиотека инструментирования
