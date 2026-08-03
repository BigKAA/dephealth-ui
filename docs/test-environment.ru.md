# Руководство по тестовой среде

**Язык:** [English](./test-environment.md) | Русский

---

## Обзор

Это руководство описывает развёртывание полноценной реалистичной тестовой среды
для **dephealth-ui** в **произвольном Kubernetes-кластере**. Среда позволяет
изучить визуализацию топологии, здоровье зависимостей, сценарии аутентификации
и обогащение алертами от начала до конца.

> Отдельное руководство, ориентированное на хоумлаб автора, находится в
> [../deploy/README.ru.md](../deploy/README.ru.md). Этот документ — его
> переносимый аналог: все домены, IP и реестры здесь либо плейсхолдеры, либо
> публично читаемые реестры.

### Как компоненты связаны между собой

dephealth-ui — слой визуализации только для чтения. Он **никогда** не обращается
к [uniproxy](https://github.com/BigKAA/uniproxy) напрямую. Вместо этого
инстансы uniproxy экспортируют Prometheus-метрики, хранилище метрик их скрейпит,
а dephealth-ui опрашивает это хранилище через PromQL:

```text
                              scrape                          PromQL
  поды uniproxy  ─────────────────────▶  VictoriaMetrics  ─────────────▶  dephealth-ui  ──▶  Браузер
  :8080/metrics     (аннотации +          :8428                          :8080 (UI + API)
                    discovery по лейблу)
```

Поскольку поток данных односторонний, четыре компонента развёртываются
независимо и должны согласовывать только **формат метрик** (семейство
`app_dependency_*`) и **конфигурацию скрейпинга** (описана ниже).

### Компоненты, разворачиваемые этим руководством

| Компонент | Helm-чарт | Назначение |
| --------- | ---------- | ---------- |
| **dephealth-infra** | `deploy/helm/dephealth-infra` | Реальные зависимости для тестовой топологии: PostgreSQL, Redis, gRPC-stub, LDAP (389ds) и nginx reverse proxy для сценария с маршрутизацией по Host |
| **dephealth-monitoring** | `deploy/helm/dephealth-monitoring` | VictoriaMetrics (хранилище метрик + скрейпер), VMAlert, AlertManager, Grafana |
| **dephealth-uniproxy** | `deploy/helm/dephealth-uniproxy` | Двенадцать инстансов uniproxy в трёх неймспейсах, образующих граф зависимостей |
| **dephealth-ui** | `deploy/helm/dephealth-ui` | Приложение визуализации (Go-бэкенд + встроенный SPA) |

---

## Предварительные требования

### Инструменты

| Инструмент | Версия | Назначение |
| ---------- | ------ | ---------- |
| **kubectl** | любая, совместимая с кластером | Доступ к кластеру |
| **Helm** | 3.0+ | Развёртывание чартов |
| **Docker** | 24+ (опционально) | Нужен только если вы сами собираете кастомные образы |
| **curl** | любой | Шаги проверки |

### Требования к кластеру

- **Kubernetes 1.28+** с рабочим доступом через `kubectl`.
- **StorageClass по умолчанию** (или укажите конкретный класс в `global.storageClass`
  в override-файлах). VictoriaMetrics, PostgreSQL и 389ds требуют
  PersistentVolumes.
- Исходящий интернет с узлов кластера для pull образов (Docker Hub и Yandex
  Container Registry), **либо** зеркалирование образов в реестр, доступный
  кластеру.
- (Опционально) провайдер **LoadBalancer** (MetalLB, cloud LB) и/или **Gateway
  API** / **Ingress-контроллер**, если нужен внешний доступ вместо
  `kubectl port-forward`.

---

## Образы контейнеров

Среда сочетает публичные образы и три кастомных.

### Публичные образы (Docker Hub, настройка не требуется)

| Компонент | Образ |
| --------- | ----- |
| PostgreSQL | `postgres:17-alpine` |
| Redis | `redis:7-alpine` |
| 389ds (LDAP) | `389ds/dirsrv:3.1` |
| nginx proxy | `nginx:1.27-alpine` |
| VictoriaMetrics | `victoriametrics/victoria-metrics:v1.108.1` |
| VMAlert | `victoriametrics/vmalert:v1.108.1` |
| AlertManager | `prom/alertmanager:v0.28.1` |
| Grafana | `grafana/grafana:11.6.0` |

### Кастомные образы

| Компонент | Источник по умолчанию (публично читаемый) | Fallback: сборка из исходников |
| --------- | ----------------------------------------- | ------------------------------ |
| **dephealth-ui** | `container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst/dephealth-ui` | `docker buildx build -t <registry>/dephealth-ui:latest --push .` (из репо `dephealth-ui/`) |
| **uniproxy** | `container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst/uniproxy` | сборка из репо `uniproxy/` — см. примечание ниже |
| **grpc-stub** | `container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst/dephealth-grpc-stub` | сборка из `topologymetrics/conformance/stubs/grpc-stub/` |

Override-файлы quickstart уже указывают на Yandex Container Registry, который
**публично читаем**, поэтому pull должен работать без аутентификации.

> **Если реестр недоступен** из кластера (например, изолированный контур),
> соберите три кастомных образа и запушьте их в реестр, доступный кластеру,
> затем:
>
> - Для **dephealth-ui**: отредактируйте
>   `deploy/quickstart/values-dephealth-ui.yaml` и укажите `image.name`.
> - Для **uniproxy** и **grpc-stub**: отредактируйте поле `global.pushRegistry`
>   в `deploy/quickstart/values-uniproxy.yaml` и `deploy/quickstart/values-infra.yaml`.
>
> Сборка **uniproxy** из исходников: его `Dockerfile` ссылается на приватное
> зеркало base-образов. Перед сборкой замените две строки `FROM` на публичные
> аналоги:
>
> ```dockerfile
> FROM golang:1.25.8-alpine AS builder
> ...
> FROM alpine:3.21
> ```

---

## Развёртывание

> Все команды предполагают, что вы выполняете их из **корня репозитория
> `dephealth-ui/`**. Override-файлы quickstart лежат в `deploy/quickstart/`.

### Шаг 1 — Инфраструктурные зависимости

Разверните PostgreSQL, Redis, gRPC-stub и LDAP. Это реальные backing-сервисы,
состояние которых будут проверять инстансы uniproxy.

```bash
helm upgrade --install dephealth-infra deploy/helm/dephealth-infra \
  -f deploy/quickstart/values-infra.yaml
```

Дождитесь готовности подов:

```bash
kubectl get pods -n dephealth-postgresql
kubectl get pods -n dephealth-redis
kubectl get pods -n dephealth-grpc-stub
kubectl get pods -n dephealth-389ds
```

### Шаг 2 — Стек мониторинга

Разверните VictoriaMetrics (хранилище метрик + скрейпер), VMAlert, AlertManager
и Grafana.

```bash
helm upgrade --install dephealth-monitoring deploy/helm/dephealth-monitoring \
  -f deploy/quickstart/values-monitoring.yaml \
  -n dephealth-monitoring --create-namespace
```

```bash
kubectl get pods -n dephealth-monitoring
```

**Как работает скрейпинг (прочтите один раз):** VictoriaMetrics
автообнаруживает поды uniproxy через Kubernetes pod discovery, оставляя только
поды, у которых **одновременно** есть:

1. аннотация `prometheus.io/scrape: "true"`, и
2. лейбл `app.kubernetes.io/part-of: dephealth`.

Helm-чарт uniproxy проставляет оба автоматически на каждом инстансе, поэтому
дополнительная настройка скрейпинга не требуется. Путь и порт скрейпинга берутся
из аннотаций пода (`/metrics` на порту `8080`).

### Шаг 3 — Инстансы uniproxy (тестовая топология)

Разверните двенадцать инстансов uniproxy в трёх неймспейсах. Они образуют граф
зависимостей, который будет визуализировать dephealth-ui.

```bash
# Неймспейс 1 — три инстанса (точка входа + цепочка)
helm upgrade --install uniproxy-ns1 deploy/helm/dephealth-uniproxy \
  -f deploy/quickstart/values-uniproxy.yaml \
  -f deploy/quickstart/instances/ns1.yaml \
  -n dephealth-uniproxy --create-namespace

# Неймспейс 2 — пять инстансов (сценарии аутентификации + инициатор proxy)
helm upgrade --install uniproxy-ns2 deploy/helm/dephealth-uniproxy \
  -f deploy/quickstart/values-uniproxy.yaml \
  -f deploy/quickstart/instances/ns2.yaml \
  -n dephealth-uniproxy-2 --create-namespace

# Неймспейс 3 — четыре инстанса за nginx reverse proxy
helm upgrade --install uniproxy-ns3 deploy/helm/dephealth-uniproxy \
  -f deploy/quickstart/values-uniproxy.yaml \
  -f deploy/quickstart/instances/ns3.yaml \
  -n dephealth-uniproxy-3 --create-namespace
```

```bash
kubectl get pods -n dephealth-uniproxy
kubectl get pods -n dephealth-uniproxy-2
kubectl get pods -n dephealth-uniproxy-3
```

Метрики начнут появляться в VictoriaMetrics через ~15–30 секунд (один интервал
скрейпинга плюс интервал проверки).

### Шаг 4 — dephealth-ui

Разверните приложение визуализации.

```bash
helm upgrade --install dephealth-ui deploy/helm/dephealth-ui \
  -f deploy/quickstart/values-dephealth-ui.yaml \
  -n dephealth-ui --create-namespace
```

```bash
kubectl get pods -n dephealth-ui
```

### Шаг 5 — Открытие UI

В quickstart внешние маршруты отключены для переносимости. Используйте
port-forwarding:

```bash
kubectl port-forward -n dephealth-ui svc/dephealth-ui 8080:8080
```

Затем откройте `http://localhost:8080` в браузере.

---

## Проверка

### 1. Метрики поступают в VictoriaMetrics

Пробросьте порт VictoriaMetrics и опросите его:

```bash
kubectl port-forward -n dephealth-monitoring svc/victoriametrics 8428:8428 &
```

```bash
# Должен вернуть по одной серии на каждую зависимость (непустой "data.result")
curl -s 'http://localhost:8428/api/v1/query?query=app_dependency_health' | jq '.data.result | length'
```

Если счётчик `0` — см. [Траблшутинг](#траблшутинг).

### 2. Эндпоинты uniproxy отвечают

Пробросьте сервис точки входа (NodePort 30080 привязан к `uniproxy-01`):

```bash
kubectl port-forward -n dephealth-uniproxy svc/uniproxy-01 8081:8080
curl http://localhost:8081/                        # статус JSON
curl "http://localhost:8081/?detail=true&depth=2"  # рекурсивный детальный статус
curl http://localhost:8081/metrics | grep app_dependency_health
```

### 3. dephealth-ui показывает топологию

Откройте `http://localhost:8080`. Должен отобразиться граф узлов (сервисов) и
рёбер (зависимостей). Здоровые зависимости — зелёные, проблемные — красные.

JSON, который потребляет UI:

```bash
curl -s http://localhost:8080/api/v1/topology | jq '.nodes | length'
```

---

## Тестовая топология

```text
┌─ Неймспейс: dephealth-uniproxy ─────────────────────────────────────┐
│                                                                     │
│  uniproxy-01 ──critical──► uniproxy-02 ──► redis                    │
│  (вход, NodePort 30080)    │           ──► grpc-stub                │
│       │                     │                                       │
│       └──critical──► uniproxy-03 ──critical──► postgresql           │
│                          │  └──► ldap (389ds)                       │
└──────────────────────────┼──────────────────────────────────────────┘
                           │ cross-namespace
                           ▼
┌─ Неймспейс: dephealth-uniproxy-2 (сценарии auth) ───────────────────┐
│                                                                     │
│  uniproxy-04 ──Bearer──► uniproxy-05 ◄──wrong token── uniproxy-06   │
│       │                                  │  │  │  │   │           │
│       └──► uniproxy-06 ──► uniproxy-07 ──► postgresql│   │           │
│                          ──Basic──► uniproxy-08 ──► postgresql      │
│                          │  │  │  (маршрутизация по Host)           │
└──────────────────────────┼──┼──┼──┼─────────────────────────────────┘
                           │  │  │
              ┌────────────┘  │  └──────────┐ cross-namespace (один proxy)
              ▼               ▼             ▼
┌─ Неймспейс: dephealth-uniproxy-3 (reverse-proxy сценарий) ──────────┐
│                                                                     │
│                  nginx-proxy (один host:port)                        │
│   ──Host: uniproxy-09──► uniproxy-09                                 │
│   ──Host: uniproxy-10──► uniproxy-10                                 │
│   ──Host: uniproxy-11──► uniproxy-11                                 │
│   ──Host: uniproxy-12──► uniproxy-12                                 │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

Что позволяет протестировать эта топология:

- **HTTP-цепочка** с рекурсией (`uniproxy-01 → 02 → 03 → 01`).
- **Cross-namespace**-связь (`uniproxy-02 → uniproxy-04`).
- **Разные типы зависимостей**: HTTP, gRPC, PostgreSQL, Redis, LDAP.
- **Критичные и некритичные** рёбра.
- **Аутентификация**: рабочий Bearer-токен, рабочий Basic-auth и
  **намеренно неверный токен** (`uniproxy-06 → uniproxy-05`), чтобы статус
  `auth_error` был виден в UI.
- **Серверная аутентификация** (`uniproxy-05`, `uniproxy-08`) с открытым
  `/metrics`.
- **Reverse proxy / маршрутизация по Host** (`uniproxy-05 → nginx-proxy →
  uniproxy-09..12`): четыре зависимости с общим `host:port`, различаются
  только заголовком `Host` (см. [Сценарий с proxy](#сценарий-с-proxy)).

### Сценарий с proxy

`uniproxy-05` (ns2) проверяет четыре сервиса, расположенные за одним nginx
reverse proxy (`nginx-proxy`, ns3). Поскольку у всех четыре общая TCP-точка,
в списке соединений используются `host`/`port` и поконнекционный `hostHeader`,
а не отдельные `url`:

```yaml
# соединения uniproxy-05 (фрагмент) — deploy/quickstart/instances/ns2.yaml
- name: uniproxy-09
  type: http
  host: "nginx-proxy.dephealth-uniproxy-3.svc"   # общий адрес для всех четырех
  port: "80"
  hostHeader: "uniproxy-09"                       # выбирает бэкенд на nginx
  critical: "yes"
  healthPath: "/"
# ... uniproxy-10, uniproxy-11, uniproxy-12 отличаются только name + hostHeader
```

- `host`/`port` — адрес proxy, TCP-точка назначения каждой проверки.
- `hostHeader` задаёт HTTP-заголовок `Host` исходящего запроса
  (`DEPHEALTH_<NAME>_HOST_HEADER` → `req.Host` в SDK). nginx по входящему
  `Host` выбирает один из четырёх бэкендов (`uniproxy-09`..`uniproxy-12`).
- **Не** кладите `Host` в `auth.headers`: Go `net/http` игнорирует
  `req.Header.Set("Host", ...)`; работает только `hostHeader` (устанавливающий
  `req.Host`), а SDK запрещает совмещать оба варианта.

Это воспроизводит реальную топологию, где общий ingress/proxy стоит перед
несколькими приложениями: в метриках четыре серии имеют одинаковые `host:port`
и различаются только меткой `dependency` — вид, показанный для `nginx-back-app1`
в `tmp/metrics.txt`.

---

## Формат метрик (справочно)

dephealth-ui опрашивает семейство метрик dephealth SDK. Все четыре
экспортируются uniproxy на `/metrics`:

| Метрика | Тип | Значение |
| ------- | --- | -------- |
| `app_dependency_health` | Gauge | `1` = здоров, `0` = нездоров |
| `app_dependency_latency_seconds` | Histogram | Задержка проверки здоровья |
| `app_dependency_status` | Gauge (enum) | Ровно одна серия = `1`: `ok`, `timeout`, `connection_error`, `dns_error`, `auth_error`, `tls_error`, `unhealthy`, `error` |
| `app_dependency_status_detail` | Gauge (info) | Всегда `1`; читаемая причина — в лейбле `detail` |

Общие лейблы: `name`, `group`, `namespace`, `dependency`, `type`, `host`,
`port`, `critical`. Полная спецификация — в [METRICS.md](./METRICS.ru.md).

---

## Опционально: Bare-metal хост

Чтобы протестировать способность dephealth-ui визуализировать топологию,
охватывающую Kubernetes **и** внешний хост, запустите дополнительный инстанс
uniproxy на VM/bare-metal хосте и зарегистрируйте его как внешнюю цель
скрейпинга.

1. На хосте (с установленным Docker) создайте `docker-compose.yaml`:

   ```yaml
   services:
     uniproxy-pr1:
       image: container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst/uniproxy:v0.7.0
       ports:
         - "8080:8080"
       environment:
         DEPHEALTH_NAME: uniproxy-pr1
         DEPHEALTH_GROUP: infra-host
         DEPHEALTH_DEPS: "postgres:postgres,redis:redis"
         DEPHEALTH_POSTGRES_URL: "postgres://dephealth:dephealth-test-pass@<PG_HOST>:5432/dephealth"
         DEPHEALTH_POSTGRES_CRITICAL: "yes"
         DEPHEALTH_REDIS_URL: "redis://<REDIS_HOST>:6379"
         DEPHEALTH_REDIS_CRITICAL: "no"
   ```

2. Запустите: `docker compose up -d`. Проверка: `curl http://<HOST_IP>:8080/metrics`.

3. Убедитесь, что порт `8080` хоста доступен из Kubernetes-кластера (его будет
   скрейпить VictoriaMetrics).

4. Зарегистрируйте цель в стеке мониторинга. Отредактируйте
   `deploy/quickstart/values-monitoring.yaml`:

   ```yaml
   victoriametrics:
     externalTargets:
       - jobName: uniproxy-pr1
         target: "<HOST_IP>:8080"
         labels:
           namespace: hostpr1
           service: uniproxy-pr1
   ```

5. Обновите monitoring и перезапустите VictoriaMetrics для перезагрузки конфига
   скрейпинга:

   ```bash
   helm upgrade --install dephealth-monitoring deploy/helm/dephealth-monitoring \
     -f deploy/quickstart/values-monitoring.yaml \
     -n dephealth-monitoring
   kubectl delete pod -n dephealth-monitoring -l app=victoriametrics
   ```

6. (Опционально) Свяжите кластерный uniproxy с этим хостом, раскомментировав
   подключение `uniproxy-pr1` в `deploy/quickstart/instances/ns1.yaml` и
   повторив команду обновления ns1.

---

## Опционально: Внешний доступ (Gateway / Ingress)

Вместо `kubectl port-forward` можно открыть dephealth-ui и Grafana внешне.

### Через Ingress

```yaml
# deploy/quickstart/values-dephealth-ui.yaml
ingress:
  enabled: true
  className: "nginx"          # ваш Ingress-контроллер
  hostname: dephealth.example.com
```

### Через Gateway API

```yaml
# deploy/quickstart/values-dephealth-ui.yaml
route:
  enabled: true
  hostname: dephealth.example.com
global:
  gateway:
    name: my-gateway
    namespace: gateway-system
```

Примените изменение командой `helm upgrade` из Шага 4, затем направьте DNS
(или `/etc/hosts`) для `dephealth.example.com` на IP вашего LoadBalancer/Gateway.

Чтобы в UI также появились ссылки на Grafana, включите route Grafana в
`values-monitoring.yaml` и укажите `config.grafana.baseUrl` в override-файлах
dephealth-ui — URL Grafana.

---

## Адаптация под ваше окружение

Override-файлы quickstart намеренно минимальны. Частые изменения:

| Потребность | Что менять |
| ----------- | ---------- |
| Конкретный StorageClass | `global.storageClass` в каждом values-файле quickstart |
| Приватный реестр образов | `global.pushRegistry` (infra, uniproxy) и `image.name` (dephealth-ui); зеркалируйте туда кастомные образы |
| Другие неймспейсы | Значения `global.namespace` чартов и FQDN `.svc` в `instances/ns1.yaml` / `ns2.yaml` должны совпадать |
| TLS для dephealth-ui | `tls.enabled`, либо Ingress TLS — см. [документацию чарта](../deploy/helm/dephealth-ui/README.md) |
| Доверие self-signed CA | Создайте ConfigMap с CA и укажите `customCA` в values dephealth-ui |

Полный справочник по хоумлабу (с конкретными IP, доменами и автоматизацией
`make env-deploy`) — см. [../deploy/README.ru.md](../deploy/README.ru.md).

---

## Траблшутинг

### dephealth-ui показывает пустую топологию

Пройдите по потоку данных по порядку:

1. **Поды uniproxy запущены и здоровы?**

   ```bash
   kubectl get pods -n dephealth-uniproxy -n dephealth-uniproxy-2
   ```

   Поды не в статусе `Running`/`Ready` не экспортируют метрики.

2. **Метрики попадают в VictoriaMetrics?**

   ```bash
   kubectl port-forward -n dephealth-monitoring svc/victoriametrics 8428:8428 &
   curl -s 'http://localhost:8428/api/v1/query?query=app_dependency_health' | jq '.data.result | length'
   ```

   Пустой результат означает, что скрейпинг не работает — проверьте следующий пункт.

3. **Поды отбираются скрейпером?** У пода должны быть **одновременно** аннотация
   `prometheus.io/scrape=true` **и** лейбл `app.kubernetes.io/part-of=dephealth`.
   Чарт проставляет их; проверьте:

   ```bash
   kubectl get pod <pod> -n dephealth-uniproxy -o jsonpath='{.metadata.annotations}'
   kubectl get pod <pod> -n dephealth-uniproxy -o jsonpath='{.metadata.labels}'
   ```

4. **dephealth-ui достучался до VictoriaMetrics?** Проверьте настроенный URL:

   ```bash
   kubectl get configmap -n dephealth-ui dephealth-ui -o yaml | grep -A2 prometheus
   ```

   Должно быть `http://victoriametrics.dephealth-monitoring.svc:8428`.

5. **Проверьте логи dephealth-ui на ошибки соединения.**

   ```bash
   kubectl logs -n dephealth-ui -l app.kubernetes.io/name=dephealth-ui
   ```

### Поды зависли в `ImagePullBackOff`

Кластер не может стянуть образ. Проверьте:

- Ссылка на образ в values quickstart корректна.
- Кластер имеет доступ к реестру (Docker Hub / Yandex CR) — проверьте egress
  узлов и настройки прокси.
- Для кастомных образов: либо Yandex CR доступен, либо они зеркалированы в
  реестр, доступный кластеру (см. [Образы контейнеров](#образы-контейнеров)).

### VictoriaMetrics не скрейпит внешние цели

После изменения `victoriametrics.externalTargets` VictoriaMetrics должен
перезагрузить конфиг скрейпинга:

```bash
kubectl delete pod -n dephealth-monitoring -l app=victoriametrics
```

### Ошибочное предположение о порте

uniproxy экспонирует **всё на единственном порту `:8080`** — включая `/`,
`/healthz`, `/readyz` и `/metrics`. Отдельного порта для метрик нет. Аннотации
скрейпинга в чарте уже это отражают.

---

## Удаление

Удаляйте компоненты в обратном порядке (опциональные шаги — только если вы их
выполняли):

```bash
# Приложение
helm uninstall dephealth-ui -n dephealth-ui

# Инстансы uniproxy
helm uninstall uniproxy-ns1 -n dephealth-uniproxy
helm uninstall uniproxy-ns2 -n dephealth-uniproxy-2

# Мониторинг
helm uninstall dephealth-monitoring -n dephealth-monitoring

# Инфраструктура
helm uninstall dephealth-infra

# Удаление неймспейсов (несуществующие игнорируются)
kubectl delete namespace dephealth-ui dephealth-uniproxy dephealth-uniproxy-2 \
  dephealth-monitoring dephealth-postgresql dephealth-redis \
  dephealth-grpc-stub dephealth-389ds --ignore-not-found
```

Остановите любой bare-metal uniproxy командой `docker compose down` на этом хосте.

---

## Сопутствующая документация

| Документ | Описание |
| -------- | -------- |
| [../deploy/README.ru.md](../deploy/README.ru.md) | Руководство по развёртыванию в хоумлабе с автоматизацией `make` |
| [METRICS.ru.md](./METRICS.ru.md) | Полная спецификация Prometheus-метрик |
| [API.ru.md](./API.ru.md) | Справочник REST API dephealth-ui |
| [application-design.ru.md](./application-design.ru.md) | Архитектура и проектные решения |
| [README uniproxy](https://github.com/BigKAA/uniproxy) | Конфигурация и сценарии использования uniproxy |
