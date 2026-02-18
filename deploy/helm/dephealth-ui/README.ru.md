# dephealth-ui Helm Chart

Helm chart для развёртывания приложения dephealth-ui в Kubernetes.

**Язык:** [English](./README.md) | Русский

## Установка

```bash
helm install dephealth-ui ./deploy/helm/dephealth-ui \
  -f ./deploy/helm/dephealth-ui/values.yaml \
  -n dephealth-ui --create-namespace
```

## Конфигурация

### Gateway API (HTTPRoute)

Для использования Gateway API:

```yaml
route:
  enabled: true
  hostname: dephealth.example.com

tls:
  enabled: true
  issuerName: letsencrypt-prod
  issuerKind: ClusterIssuer

global:
  gateway:
    name: gateway
    namespace: gateway-system
```

Будут созданы:
- Ресурс `HTTPRoute`, указывающий на сконфигурированный Gateway
- Ресурс `Certificate` (при `tls.enabled: true`), управляемый cert-manager

### Ingress (Традиционный)

Для использования стандартного Kubernetes Ingress:

```yaml
route:
  enabled: false  # Отключить Gateway API

ingress:
  enabled: true
  className: nginx  # Класс вашего Ingress-контроллера
  hostname: dephealth.example.com
  annotations:
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
```

#### Вариант 1: Использование существующего TLS-секрета

Если TLS-сертификат уже есть в Kubernetes Secret:

```yaml
ingress:
  enabled: true
  className: nginx
  hostname: dephealth.example.com
  tls:
    enabled: true
    secretName: my-existing-tls-secret  # Заранее созданный Secret
    certManager:
      enabled: false
```

Secret должен содержать ключи `tls.crt` и `tls.key`:

```bash
kubectl create secret tls my-existing-tls-secret \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  -n dephealth-ui
```

#### Вариант 2: Автоматический сертификат через cert-manager

Для автоматического выпуска сертификатов:

```yaml
ingress:
  enabled: true
  className: nginx
  hostname: dephealth.example.com
  tls:
    enabled: true
    secretName: ""  # Оставить пустым
    certManager:
      enabled: true
      issuerName: letsencrypt-prod
      issuerKind: ClusterIssuer
```

Будет создан ресурс `Certificate`, cert-manager автоматически выпустит TLS-сертификат.

### OIDC-аутентификация

```yaml
config:
  auth:
    type: oidc
    oidc:
      issuer: "https://dex.example.com"
      clientId: "dephealth-ui"
      clientSecret: "base64-encoded-secret"
      redirectUrl: "https://dephealth.example.com/auth/callback"

customCA:
  enabled: true
  configMapName: custom-ca
  key: ca.crt
```

### Удержание stale-нод (lookback)

Когда сервис перестаёт отправлять метрики, он обычно исчезает с графа. Включите окно lookback, чтобы сохранять исчезнувшие сервисы в состоянии `unknown` на настраиваемый период:

```yaml
config:
  topology:
    lookback: "1h"  # Хранить stale-ноды 1 час (по умолчанию: "0" = отключено)
```

Рекомендуемые значения: `1h` (большинство окружений), `6h` (редкие деплои), `0` (отключено).

### Источники данных

```yaml
config:
  datasources:
    prometheus:
      url: "http://victoriametrics.monitoring.svc:8428"
    alertmanager:
      url: "http://alertmanager.monitoring.svc:9093"
```

### Интеграция с Grafana

Настройте базовый URL Grafana и UID дашбордов для прямых ссылок из UI:

```yaml
config:
  grafana:
    baseUrl: "https://grafana.example.com"
    dashboards:
      cascadeOverview: "dephealth-cascade-overview"  # Обзор каскадных сбоев
      rootCause: "dephealth-root-cause"              # Анализ первопричин
      serviceStatus: "dephealth-service-status"      # Статус одного сервиса
      linkStatus: "dephealth-link-status"            # Статус одной зависимости
      serviceList: "dephealth-service-list"          # Список всех сервисов
      servicesStatus: "dephealth-services-status"    # Обзор всех сервисов
      linksStatus: "dephealth-links-status"          # Обзор всех связей
```

Если `grafana.baseUrl` пустой, ссылки на Grafana скрыты в UI.

### Grafana-дашборды

Helm chart `dephealth-monitoring` включает 7 готовых Grafana-дашбордов, разворачиваемых через ConfigMaps:

| Дашборд | UID | Описание | Источник данных |
|---------|-----|----------|-----------------|
| Cascade Overview | `dephealth-cascade-overview` | Обзор каскадных сбоев с затронутыми сервисами и первопричинами | Infinity (API) |
| Root Cause Analyzer | `dephealth-root-cause` | Детальный анализ первопричин с графом зависимостей | Infinity (API) + Prometheus |
| Service Status | `dephealth-service-status` | Состояние одного сервиса и timeline зависимостей | Prometheus |
| Link Status | `dephealth-link-status` | Состояние одной зависимости, latency | Prometheus |
| Service List | `dephealth-service-list` | Таблица всех сервисов с состоянием и кол-вом зависимостей | Prometheus |
| Services Status | `dephealth-services-status` | Обзор всех сервисов с timeline состояний | Prometheus |
| Links Status | `dephealth-links-status` | Обзор всех связей с timeline здоровья | Prometheus |

> **Важно: требования к плагинам и API**
>
> Дашборды **Cascade Overview** и **Root Cause Analyzer** требуют:
>
> 1. **Плагин Grafana Infinity datasource** ([yesoreyeram-infinity-datasource](https://grafana.com/grafana/plugins/yesoreyeram-infinity-datasource/)) — должен быть установлен в Grafana. Устанавливается через переменную окружения `GF_INSTALL_PLUGINS` или Grafana CLI.
> 2. **API dephealth-ui** — должен быть доступен из Grafana по сети (например, `http://dephealth-ui.dephealth-ui.svc:8080`). Эти дашборды обращаются к endpoint'ам `/api/v1/cascade-analysis` и `/api/v1/cascade-graph` для получения данных о каскадных сбоях.
>
> Дашборд **Root Cause Analyzer** также включает панель **Node Graph**, визуализирующую цепочки каскадных зависимостей. Для работы этой панели Infinity datasource должен быть создан с UID `infinity` (или обновите переменную `${DS_INFINITY}` в JSON дашборда).
>
> Без плагина Infinity или сетевого доступа к API dephealth-ui эти два дашборда будут показывать пустые панели. Остальные 5 дашбордов используют только Prometheus и работают независимо.

## Примеры

См. `values-homelab.yaml` — полный пример для домашней лаборатории с Gateway API.

См. `values-ingress-example.yaml` — примеры конфигурации Ingress.

## Параметры

| Параметр | Описание | По умолчанию |
|----------|----------|--------------|
| `global.pushRegistry` | Container registry для образов | `""` |
| `global.namespace` | Целевой namespace | `dephealth-ui` |
| `image.name` | Имя образа | `dephealth-ui` |
| `image.tag` | Тег образа | `latest` |
| `route.enabled` | Включить Gateway API HTTPRoute | `false` |
| `route.hostname` | Hostname для HTTPRoute | `dephealth.example.com` |
| `ingress.enabled` | Включить Ingress | `false` |
| `ingress.className` | Класс Ingress-контроллера | `""` |
| `ingress.hostname` | Hostname для Ingress | `dephealth.example.com` |
| `ingress.annotations` | Аннотации Ingress | `{}` |
| `ingress.tls.enabled` | Включить TLS для Ingress | `false` |
| `ingress.tls.secretName` | Имя существующего TLS-секрета | `""` |
| `ingress.tls.certManager.enabled` | Генерировать сертификат через cert-manager | `false` |
| `ingress.tls.certManager.issuerName` | Имя Issuer cert-manager | `""` |
| `ingress.tls.certManager.issuerKind` | Тип Issuer cert-manager | `ClusterIssuer` |
| `config.auth.type` | Тип аутентификации: `none`, `basic`, `oidc` | `none` |
| `config.datasources.prometheus.url` | URL Prometheus/VictoriaMetrics | `http://victoriametrics:8428` |
| `config.datasources.alertmanager.url` | URL AlertManager | `""` |
| `config.cache.ttl` | TTL кэша | `15s` |
| `config.topology.lookback` | Окно удержания stale-нод (`0` = отключено) | `"0"` |
| `config.grafana.baseUrl` | Базовый URL Grafana (пустой = ссылки скрыты) | `""` |
| `config.grafana.dashboards.*` | UID дашбордов Grafana | см. values.yaml |
| `customCA.enabled` | Монтировать custom CA-сертификат | `false` |
| `customCA.configMapName` | Имя ConfigMap с CA-сертификатом | `""` |
| `customCA.key` | Ключ в ConfigMap с сертификатом | `ca.crt` |

## Требования

- Kubernetes 1.21+
- Helm 3.0+
- Для Gateway API: установлены CRD Gateway API
- Для TLS: cert-manager (опционально, для автоматических сертификатов)
- Для OIDC: провайдер OIDC (например, Dex, Keycloak)
- Для группировки по group: topologymetrics SDK v0.5.0+ (добавляет метку `group` к метрикам)
