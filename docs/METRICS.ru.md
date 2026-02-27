# Спецификация формата метрик

**Язык:** [English](./METRICS.md) | Русский

---

## Обзор

**dephealth-ui** требует метрики, совместимые с Prometheus, которые описывают зависимости сервисов и их состояние здоровья. Эти метрики собираются приложениями, инструментированными с помощью [dephealth SDK](https://github.com/BigKAA/topologymetrics).

Данный документ определяет:
- Обязательные имена и типы метрик
- Обязательные и опциональные метки
- Форматы значений и ограничения
- PromQL-запросы, используемые приложением
- Примеры интеграции

---

## Обязательные метрики

### 1. `app_dependency_health`

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
| `isentry` | Отмечает сервис как точку входа для внешнего трафика. При значении `yes` узел отображается с бейджем точки входа (⬇) в UI. | `yes` |
| `group` | Логическая группа сервиса (SDK v0.5.0+). Активирует переключатель группировки в UI. При отсутствии используется только группировка по namespace. | `proxy-cluster-1`, `infra-host1`, `payment-team` |
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

### 2. `app_dependency_latency_seconds`

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

## PromQL-запросы, используемые dephealth-ui

Приложение выполняет следующие запросы к Prometheus/VictoriaMetrics:

### 1. **Обнаружение топологии** — извлечение всех уникальных рёбер
```promql
group by (name, namespace, group, dependency, type, host, port, critical, isentry) (app_dependency_health)
```
**Назначение:** Обнаружить все связи сервис→зависимость в системе. Метки `group` и `isentry` включаются при наличии.

### 2. **Состояние здоровья** — текущее значение health для каждого ребра
```promql
app_dependency_health
```
**Назначение:** Определить, доступен ли каждый endpoint зависимости (UP=1, DOWN=0).

### 3. **Средний latency** — среднее значение latency для каждого ребра
```promql
rate(app_dependency_latency_seconds_sum[5m]) / rate(app_dependency_latency_seconds_count[5m])
```
**Назначение:** Вычислить скользящее среднее latency за 5 минут для каждой зависимости.

### 4. **P99 Latency** — 99-й перцентиль latency для каждого ребра
```promql
histogram_quantile(0.99, rate(app_dependency_latency_seconds_bucket[5m]))
```
**Назначение:** Вычислить P99 latency для выявления медленных зависимостей.

### 5. **Инстансы сервиса** — список всех инстансов сервиса
```promql
group by (instance, pod, job) (app_dependency_health{name="<service-name>"})
```
**Назначение:** Отобразить все запущенные инстансы/pod'ы для выбранного сервиса в боковой панели.

---

## Модель графа

- **Узлы (Vertices):** Уникальные значения метки `name` → представляют сервисы/приложения
- **Рёбра (Directed):** Уникальные комбинации `{name, namespace, group, dependency, type, host, port, critical, isentry}` → представляют связи сервис→зависимость
- **Свойства рёбер:**
  - **critical:** визуальная толщина (критичные зависимости отображаются толще) + распространение каскадных предупреждений (только рёбра с `critical=yes` распространяют предупреждения о сбоях вверх по графу)
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

## Правила вычисления состояний

**Состояние сервис-ноды** (вычисляется backend в `calcServiceNodeState`):
- **unknown:** Нет исходящих рёбер (нет данных о зависимостях)
- **degraded:** Любое исходящее ребро имеет `health=0`
- **ok:** Все исходящие рёбра имеют `health=1`
- **down:** Только когда все исходящие рёбра stale (метрики пропали) — устанавливается логикой stale detection, не `calcServiceNodeState`

> Примечание: `calcServiceNodeState` никогда не возвращает `"down"`. Подробнее см. [Проектирование приложения — Модель состояний](./application-design.ru.md#модель-состояний).

**Состояние dependency-ноды** (вычисляется backend):
- **down:** Все входящие рёбра stale (`stale=true`)
- **ok:** `health=1` (из non-stale входящих рёбер)
- **down:** `health=0`

**Состояние ребра:**
- **ok:** `app_dependency_health = 1`
- **down:** `app_dependency_health = 0`
- **unknown:** Stale (метрики пропали в пределах lookback window)

### Метка `critical` и каскадные предупреждения

Метка `critical` (`yes`/`no`) имеет два эффекта в dephealth-ui:

1. **Визуальный:** Критичные рёбра отображаются толще на графе
2. **Каскадные предупреждения:** Только рёбра с `critical=yes` распространяют предупреждения о сбоях вверх по графу. Когда зависимость падает, каскадные предупреждения отправляются всем вышестоящим сервисам, связанным через критические рёбра.

**Пример:** Если `order-service → postgres-main (critical=yes)` и `postgres-main` падает:
- `order-service` получает бейдж каскадного предупреждения `⚠ 1` с tooltip'ом, показывающим корневую причину
- Если `order-service → redis-cache (critical=no)` и `redis-cache` падает — каскадное предупреждение не генерируется

Подробнее см. [Проектирование приложения — Каскадные предупреждения](./application-design.ru.md#каскадные-предупреждения).

---

## Интеграция с AlertManager

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

## Руководство по интеграции

### Шаг 1: Инструментировать приложение

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

### Шаг 2: Настроить scraping в Prometheus

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

### Шаг 3: Развернуть правила AlertManager

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

### Шаг 4: Настроить dephealth-ui

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

## Чеклист проверки

- Метрики `app_dependency_health` и `app_dependency_latency_seconds` экспортируются
- Все обязательные метки присутствуют: `name`, `namespace`, `dependency`, `type`, `host`, `port`, `critical`
- Значения меток консистентны (одинаковые метки для метрик health + latency)
- Значения health — строго `0` или `1` (не строки, не другие числа)
- Histogram latency имеет стандартные buckets
- Prometheus успешно собирает метрики (проверьте страницу `/targets`)
- AlertManager настроен и доступен

**Тестовый запрос:**
```promql
# Должен вернуть вашу топологию сервисов
group by (name, namespace, dependency, type, host, port, critical, isentry) (app_dependency_health)
```

---

## Решение проблем

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

## См. также

- [Проектирование приложения](./application-design.ru.md) — Полный обзор архитектуры
- [Справочник API](./API.ru.md) — REST API endpoints
- [Руководство по развёртыванию](../deploy/helm/dephealth-ui/README.ru.md) — Kubernetes & Helm
- [dephealth SDK](https://github.com/BigKAA/topologymetrics) — Официальная библиотека инструментирования
