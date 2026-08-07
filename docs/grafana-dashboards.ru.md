# Интеграция с дашбордами Grafana

**Язык:** [English](./grafana-dashboards.md) | Русский

## Обзор

dephealth-ui предоставляет прямые ссылки на дашборды Grafana из графа топологии. При клике на узел сервиса, ребро зависимости или использовании каскадного анализа UI генерирует URL, открывающие соответствующий дашборд Grafana с предзаполненными переменными.

При старте приложение проверяет доступность дашбордов через Grafana API и скрывает ссылки на ненайденные дашборды.

## Требования к переменным дашбордов

Каждый тип дашборда ожидает определённые template-переменные Grafana:

| Дашборд | Ключ конфига | Переменные | Описание |
|---------|-------------|------------|----------|
| Service Status | `serviceStatus` | `var-service` | Состояние одного сервиса |
| Link Status | `linkStatus` | `var-dependency`, `var-host`, `var-port` | Состояние одной зависимости |
| Cascade Overview | `cascadeOverview` | `var-namespace` | Обзор каскадных сбоев |
| Root Cause | `rootCause` | `var-service`, `var-namespace` | Анализ первопричин |
| Connection Diagnostics | `connectionDiagnostics` | `var-namespace`, `var-service` | Диагностика подключений |
| Service List | `serviceList` | *(нет)* | Таблица всех сервисов |
| Services Status | `servicesStatus` | *(нет)* | Обзор всех сервисов |
| Links Status | `linksStatus` | *(нет)* | Обзор всех связей |

## Примеры генерации URL

Backend генерирует URL для Grafana по шаблону `{baseUrl}/d/{uid}?{variables}`:

```
# Service Status
https://grafana.example.com/d/dephealth-service-status?var-service=my-service

# Link Status
https://grafana.example.com/d/dephealth-link-status?var-dependency=redis&var-host=redis.svc&var-port=6379

# Cascade Overview (генерируется на фронтенде)
https://grafana.example.com/d/dephealth-cascade-overview?var-namespace=production

# Root Cause (генерируется на фронтенде)
https://grafana.example.com/d/dephealth-root-cause?var-service=my-service&var-namespace=production
```

URL сервисов и связей генерируются бэкендом (`internal/topology/graph.go`). URL каскадного анализа, первопричин и диагностики генерируются фронтендом, используя UID из endpoint `/api/v1/config`.

## Настройка аутентификации

Приложение поддерживает три метода аутентификации для Grafana API со следующим приоритетом:

### 1. Service Account Token (рекомендуется)

```yaml
grafana:
  baseUrl: "https://grafana.example.com"
  token: "glsa_xxxxxxxxxxxxxxxxxxxx"
```

Или через переменную окружения:
```bash
DEPHEALTH_GRAFANA_TOKEN=glsa_xxxxxxxxxxxxxxxxxxxx
```

### 2. Basic-аутентификация

```yaml
grafana:
  baseUrl: "https://grafana.example.com"
  username: "api-user"
  password: "api-password"
```

Или через переменные окружения:
```bash
DEPHEALTH_GRAFANA_USERNAME=api-user
DEPHEALTH_GRAFANA_PASSWORD=api-password
```

### 3. Без аутентификации

Если ни токен, ни имя пользователя не настроены, запросы отправляются без аутентификации. Работает, когда Grafana разрешает анонимный доступ к API.

### Приоритет

При наличии нескольких настроенных методов: **token** > **basic auth** > **без аутентификации**.

### Kubernetes Secret (Helm)

В Kubernetes-развёртываниях учётные данные обычно хранятся в Secret и передаются через переменные окружения:

```bash
kubectl create secret generic grafana-creds \
  --from-literal=token="glsa_xxxxxxxxxxxxxxxxxxxx" \
  -n dephealth-ui
```

```yaml
# values.yaml
grafanaSecret:
  enabled: true
  secretName: grafana-creds
```

Подробнее см. [документацию Helm-чарта](../deploy/helm/dephealth-ui/README.ru.md).

## Проверка доступности

При старте приложения выполняются следующие проверки:

1. Если `grafana.baseUrl` пуст — проверки не выполняются, ссылки на дашборды отсутствуют
2. `GET {baseUrl}/api/health` — проверка доступности Grafana
   - Если недоступна: **все** UID дашбордов очищаются, в лог пишется предупреждение
3. Для каждого настроенного (непустого) UID дашборда: `GET {baseUrl}/api/dashboards/uid/{uid}`
   - Если дашборд не найден (404/403/5xx): UID очищается, в лог пишется предупреждение
   - Если найден (200): в лог пишется информационное сообщение

Очищенные UID приводят к скрытию ссылок по всему UI — как URL, генерируемых бэкендом (узлы сервисов/связей), так и URL, генерируемых фронтендом (кнопки каскадного анализа/диагностики в сайдбаре).

## Минимальная версия Grafana

- **Grafana 7.0+** — Dashboard UID API (`/api/dashboards/uid/{uid}`)
- **Grafana 9.0+** — Service Account Token (рекомендуемый метод аутентификации)

## Рекомендации по безопасности

1. **Используйте Service Account Token** (Grafana 9+) вместо учётных данных пользователя
2. Назначьте роль **Viewer** — чекер только читает метаданные дашбордов
3. Храните учётные данные в **Kubernetes Secret**, а не в конфигурационных файлах
4. Используйте переменные окружения (`DEPHEALTH_GRAFANA_TOKEN`) для CI/CD

## Готовые дашборды

Helm-чарт `dephealth-monitoring` включает 8 готовых дашбордов, разворачиваемых через ConfigMaps. Подробнее см. [документацию Helm-чарта](../deploy/helm/dephealth-ui/README.ru.md#grafana-дашборды).
