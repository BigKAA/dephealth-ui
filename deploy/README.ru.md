# Тестовая площадка

**Язык:** [English](./README.md) | Русский

---

## Обзор

Этот каталог содержит всё необходимое для развёртывания полной тестовой среды **dephealth-ui**: инфраструктурные сервисы, тестовые микросервисы, стек мониторинга и само приложение.

Тестовая среда создаёт реалистичную микросервисную топологию с несколькими namespace'ами, различными типами зависимостей (HTTP, gRPC, PostgreSQL, Redis), сценариями аутентификации и bare metal хостом — что позволяет полноценно тестировать возможности визуализации dephealth-ui.

> **Важно:** Все имена хостов, IP-адреса и URL реестра в файлах `values-homelab.yaml` относятся к домашней лаборатории автора. Вы **должны** адаптировать их под свою среду. См. [Адаптация под своё окружение](#адаптация-под-своё-окружение).

---

## Необходимое ПО

### Программное обеспечение

| Инструмент | Версия | Назначение |
| ------------ | -------- | ------------ |
| **Kubernetes** | 1.28+ | Кластер с доступом через `kubectl` |
| **Helm** | 3.0+ | Развёртывание чартов |
| **Docker** | 24+ | Сборка контейнеров, деплой на bare metal |
| **docker buildx** | 0.10+ | Мульти-архитектурные сборки (amd64/arm64) |
| **SSH** | любой | Деплой на bare metal хост |
| **make** | любой | Автоматизация сборки |

### Требования к Kubernetes кластеру

- **Gateway API** (предпочтительно) или Ingress-контроллер
- **StorageClass** для постоянных томов (например `nfs-client`, `local-path`)
- **MetalLB** или аналогичный LoadBalancer-провайдер (для bare metal кластеров)
- **cert-manager** с `ClusterIssuer` (для TLS-сертификатов)
- Сетевая связность от подов кластера к bare metal хостам (для внешнего скрейпинга)

### Реестр контейнеров

Тестовая среда загружает образы из приватного Harbor реестра. Вам нужен:

- Собственный реестр контейнеров, или
- Прямой доступ к Docker Hub (измените `values.yaml` для использования стандартных реестров)

---

## Структура каталога

```text
deploy/
├── docker/
│   └── uniproxy-pr1/             # Деплой на bare metal хост
│       └── docker-compose.yaml   # uniproxy + PostgreSQL + Redis
├── helm/
│   ├── dephealth-infra/          # Инфраструктурные сервисы
│   │   ├── values.yaml           # Значения по умолчанию
│   │   └── values-homelab.yaml   # Переопределения для домашней лаборатории
│   ├── dephealth-monitoring/     # Стек мониторинга
│   │   ├── dashboards/           # JSON дашборды Grafana
│   │   ├── values.yaml           # Значения по умолчанию
│   │   └── values-homelab.yaml   # Переопределения для домашней лаборатории
│   ├── dephealth-ui/             # Чарт приложения
│   │   ├── README.ru.md          # Документация Helm-чарта
│   │   ├── values.yaml           # Значения по умолчанию
│   │   └── values-homelab.yaml   # Переопределения для домашней лаборатории
│   └── dephealth-uniproxy/       # Тестовые прокси-экземпляры
│       ├── instances/
│       │   ├── ns1-homelab.yaml  # Топология NS1 (3 экземпляра)
│       │   └── ns2-homelab.yaml  # Топология NS2 (5 экземпляров, сценарии аутентификации)
│       ├── values.yaml           # Значения по умолчанию
│       └── values-homelab.yaml   # Переопределения для домашней лаборатории
└── k8s-dev/
    └── dephealth-ui.yaml         # Raw K8s манифест (для разработки)
```

---

## Helm-чарты

### dephealth-infra

Инфраструктурные сервисы, используемые тестовыми микросервисами.

| Сервис | Namespace | По умолчанию | Описание |
| -------- | ----------- | ------------- | ---------- |
| PostgreSQL | `dephealth-postgresql` | Включён | v17-alpine, креды: `dephealth/dephealth-test-pass` |
| Redis | `dephealth-redis` | Включён | v7-alpine, in-memory кэш |
| gRPC Stub | `dephealth-grpc-stub` | Включён | Простой gRPC health check responder |
| Dex (OIDC) | `dephealth-test` | Выключен | OIDC-провайдер для тестирования аутентификации |
| Kafka | — | Выключен | Зарезервировано |
| RabbitMQ | — | Выключен | Зарезервировано |

### dephealth-uniproxy

Экземпляры [uniproxy](https://github.com/BigKAA/uniproxy) — тестового прокси, собранного с [dephealth SDK](https://github.com/BigKAA/topologymetrics). Создают многосервисную топологию в двух namespace'ах.

**Namespace 1** (`dephealth-uniproxy`):

| Экземпляр | Реплики | Зависимости | Примечание |
| ----------- | --------- | ------------- | ------------ |
| uniproxy-01 | 2 | uniproxy-02 (critical), uniproxy-03 (critical) | Точка входа, NodePort 30080 |
| uniproxy-02 | 2 | redis, grpc-stub, uniproxy-04 (critical), uniproxy-pr1 (critical) | Кросс-namespace + bare metal |
| uniproxy-03 | 3 | postgresql (critical) | Зависимость от БД |

**Namespace 2** (`dephealth-uniproxy-2`) — тестовые сценарии аутентификации:

| Экземпляр | Реплики | Зависимости | Аутентификация |
| ----------- | --------- | ------------- | --------------- |
| uniproxy-04 | 2 | uniproxy-05 (Bearer), uniproxy-06 | Клиентская auth |
| uniproxy-05 | 1 | — | Сервер: Bearer-токен |
| uniproxy-06 | 2 | uniproxy-07, uniproxy-08 (Basic), uniproxy-05 (неверный токен) | Смешанная auth |
| uniproxy-07 | 1 | postgresql (critical) | Без auth |
| uniproxy-08 | 1 | postgresql (critical) | Сервер: Basic auth |

### dephealth-monitoring

Полный стек мониторинга для сбора метрик и визуализации.

| Компонент | Версия | Описание |
| ----------- | -------- | ---------- |
| VictoriaMetrics | v1.108.1 | Prometheus-совместимая TSDB, 7 дней хранения |
| VMAlert | v1.108.1 | Движок вычисления алертов |
| AlertManager | v0.28.1 | Маршрутизация и группировка алертов |
| Grafana | v11.6.0 | Дашборды (8 готовых), admin/dephealth |

**Сбор метрик:**

- Поды Kubernetes обнаруживаются автоматически через `prometheus.io/scrape=true` + `app.kubernetes.io/part-of=dephealth`
- Внешние таргеты (bare metal хосты) через `victoriametrics.externalTargets` в values

### dephealth-ui

Само приложение. См. [deploy/helm/dephealth-ui/README.ru.md](./helm/dephealth-ui/README.ru.md) для подробной документации чарта.

---

## Bare Metal хост

Каталог `deploy/docker/uniproxy-pr1/` содержит Docker Compose для запуска `uniproxy-pr1` на физическом хосте вне Kubernetes кластера. Это позволяет тестировать визуализацию смешанных K8s + bare metal топологий.

**Сервисы:**

- `uniproxy-pr1` — тестовый прокси с dephealth SDK (порт 8080)
- `postgresql` — локальный PostgreSQL 17 (критическая зависимость)
- `redis` — локальный Redis 7 (некритическая зависимость)

**Требования к хосту:**

- Docker с плагином Compose
- Сетевой доступ из K8s кластера (для скрейпинга Prometheus)
- Доверие к приватному CA-сертификату (при использовании приватного реестра)

---

## Тестовая топология

```text
                           ┌─ NS: dephealth-uniproxy ────────────────────────────────────┐
                           │                                                             │
                           │  uniproxy-01 ──critical──► uniproxy-02 ──► redis            │
                           │       │                         │          ──► grpc-stub    │
                           │       │                         │                           │
                           │       └──critical──► uniproxy-03 ──critical──► postgresql   │
                           │                         │                                   │
                           └─────────────────────────┼───────────────────────────────────┘
                                                     │
                              ┌───────────────────critical────────────────────────┐
                              │                      │                            │
                              ▼                      ▼                            │
┌─ NS: dephealth-uniproxy-2 ──────────────────────────────────────────────────┐   │
│                                                                             │   │
│  uniproxy-04 ──Bearer──► uniproxy-05 ◄──wrong token── uniproxy-06           │   │
│       │                                                     │    │          │   │
│       └──critical──► uniproxy-06 ──► uniproxy-07 ──► postgresql  │          │   │
│                                       ──Basic──► uniproxy-08 ──► postgresql │   │
│                                                                             │   │
└─────────────────────────────────────────────────────────────────────────────┘   │
                                                                                  │
┌─ Хост: 192.168.218.168 (NS: hostpr1) ───────────────────────────────────────┐   │
│                                                                             │   │
│  uniproxy-pr1 ──critical──► postgresql                               ◄──────┘   │
│       │                                                                     │
│       └──────────────────► redis                                            │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Быстрый старт

### Развернуть всё

```bash
make env-deploy    # Инфраструктура + uniproxy + bare metal хост + мониторинг
make helm-deploy   # Приложение dephealth-ui
```

### Проверить статус

```bash
make env-status    # Все namespace'ы + bare metal хост
```

### Удалить всё

```bash
make helm-undeploy  # Удалить dephealth-ui
make env-undeploy   # Удалить все тестовые компоненты
```

### Отдельные компоненты

```bash
make uniproxy-deploy    # Развернуть/обновить экземпляры uniproxy
make uniproxy-undeploy  # Удалить экземпляры uniproxy

make host-deploy        # Развернуть Docker Compose на bare metal
make host-undeploy      # Остановить Docker Compose на bare metal
make host-status        # Проверить контейнеры на bare metal
```

---

## Адаптация под своё окружение

Файлы `values-homelab.yaml` содержат настройки, специфичные для конкретной среды. Для использования собственной инфраструктуры создайте свои файлы переопределений или измените существующие.

### 1. Реестр контейнеров

**Релизный:** `container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst` (Yandex Container Registry)

**Для разработки:** `harbor.kryukov.lan/library` (образы), `harbor.kryukov.lan/docker` (Docker Hub прокси)

**Варианты:**

- Docker Hub напрямую: уберите переопределения `global.imageRegistry` из `values-homelab.yaml`
- Свой реестр: укажите `global.pushRegistry` с URL вашего реестра
- Для приватных реестров с самоподписанным CA: установите CA-сертификат на все узлы K8s и bare metal хосты

### 2. StorageClass

**Текущий:** `nfs-client`

Замените на StorageClass вашего кластера:

```yaml
global:
  storageClass: "local-path"  # или "standard", "gp3" и т.д.
```

### 3. DNS и имена хостов

**Текущие имена хостов** (должны резолвиться на IP вашего Gateway/LoadBalancer):

| Имя хоста | Сервис | Порт |
| ----------- | -------- | ------ |
| `dephealth.kryukov.lan` | dephealth-ui | HTTPS |
| `grafana.kryukov.lan` | Grafana | HTTP |
| `dex.kryukov.lan` | Dex OIDC (опционально) | HTTPS |

**Для адаптации:**

1. Выберите свой домен (например `dephealth.mylab.local`)
2. Обновите все файлы `values-homelab.yaml` с новыми именами хостов
3. Добавьте DNS-записи, указывающие на IP вашего Gateway/LoadBalancer
4. Или добавьте записи в `/etc/hosts` на машине разработчика

### 4. Gateway API

**Текущий:** Envoy Gateway (`eg` в namespace `envoy-gateway-system`)

Если вы используете другой Gateway-контроллер или Ingress:

- Измените `route.gateway.name` и `route.gateway.namespace` в values
- Или переключитесь на Ingress: `ingress.enabled=true` и `route.enabled=false`

### 5. Bare metal хост

**Текущий:** `192.168.218.168` (Rocky Linux, SSH от root)

Для использования своего хоста:

```bash
make host-deploy HOST_PR1_IP=10.0.0.50
```

Или установите постоянно в Makefile:

```makefile
HOST_PR1_IP ?= 10.0.0.50
```

**Требования к хосту:**

- Docker с плагином Compose
- SSH-доступ (рекомендуется авторизация по ключу)
- Порт 8080 доступен из K8s кластера
- Доступ к реестру контейнеров (с доверием к CA при необходимости)

### 6. TLS-сертификаты

**Текущий:** cert-manager с `ClusterIssuer: dev-ca-issuer` (самоподписанный CA)

Для вашей среды:

- Используйте cert-manager с Let's Encrypt для публичных кластеров
- Используйте свой CA и настройте `customCA` в values dephealth-ui
- Или отключите TLS для разработки (не рекомендуется)

### Пример: минимальная пользовательская конфигурация

Создайте `values-myenv.yaml` для каждого чарта:

```yaml
# dephealth-infra/values-myenv.yaml
global:
  storageClass: "local-path"

# dephealth-monitoring/values-myenv.yaml
global:
  storageClass: "local-path"
grafana:
  rootUrl: "http://grafana.mylab.local"
  route:
    enabled: true
    hostname: grafana.mylab.local
    gateway:
      name: my-gateway
      namespace: gateway-system

# dephealth-ui/values-myenv.yaml
image:
  registry: "my-registry.example.com"
  tag: "v0.16.0"
route:
  enabled: true
  hostname: dephealth.mylab.local
  gateway:
    name: my-gateway
    namespace: gateway-system
config:
  grafana:
    baseUrl: "http://grafana.mylab.local"
```

Затем деплой:

```bash
helm upgrade --install dephealth-infra deploy/helm/dephealth-infra -f deploy/helm/dephealth-infra/values-myenv.yaml
```

---

## Устранение неполадок

### Поды зависли в ImagePullBackOff

Кластер не может получить доступ к реестру контейнеров. Проверьте:

- URL реестра в `values-homelab.yaml` (или вашем переопределении)
- Сетевую связность от узлов кластера к реестру
- Доверие к CA-сертификату (для приватных реестров с самоподписанными сертификатами)

### VictoriaMetrics не скрейпит внешние таргеты

После обновления Helm-чарта мониторинга VictoriaMetrics требуется перезапуск пода для перезагрузки конфига:

```bash
kubectl delete pod victoriametrics-0 -n dephealth-monitoring
```

### Bare metal хост: ошибка TLS-сертификата

Docker на хосте не доверяет вашему приватному CA. Установите CA-сертификат:

```bash
# Rocky Linux / CentOS / RHEL
scp ca.crt root@<HOST_IP>:/etc/pki/ca-trust/source/anchors/
ssh root@<HOST_IP> 'update-ca-trust && systemctl restart docker'

# Ubuntu / Debian
scp ca.crt root@<HOST_IP>:/usr/local/share/ca-certificates/
ssh root@<HOST_IP> 'update-ca-certificates && systemctl restart docker'
```

### dephealth-ui не показывает топологию

1. Проверьте, есть ли метрики в VictoriaMetrics: `curl http://victoriametrics:8428/api/v1/query?query=app_dependency_health`
2. Проверьте, что dephealth-ui может обратиться к VictoriaMetrics: убедитесь в `config.datasources.prometheus.url`
3. Проверьте, что поды uniproxy запущены: `make env-status`

### Custom CA для dephealth-ui

Перед деплоем dephealth-ui создайте ConfigMap с вашим CA-сертификатом:

```bash
kubectl create configmap custom-ca \
  --from-file=ca.crt=/path/to/your/ca.crt \
  -n dephealth-ui
```

---

## Связанная документация

| Документ | Описание |
| ---------- | ---------- |
| [Руководство по Helm-чарту](./helm/dephealth-ui/README.ru.md) | Развёртывание dephealth-ui в Kubernetes |
| [Спецификация метрик](../docs/METRICS.ru.md) | Формат необходимых Prometheus-метрик |
| [Справочник API](../docs/API.ru.md) | REST API endpoint'ы |
| [Проектирование приложения](../docs/application-design.ru.md) | Архитектура и проектные решения |
