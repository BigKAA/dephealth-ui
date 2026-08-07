# Документация dephealth-ui

**Язык:** [English](./README.md) | Русский

---

## Обзор

Этот каталог содержит полную документацию по **dephealth-ui** — инструменту визуализации топологии и здоровья микросервисов в реальном времени.

## Документы

| Документ | Описание | Аудитория |
|----------|----------|-----------|
| **[METRICS.ru.md](./METRICS.ru.md)** | **⭐ НАЧНИТЕ ОТСЮДА** — Спецификация формата метрик, обязательные метки, PromQL-запросы, руководство по интеграции | Разработчики, DevOps |
| **[API.ru.md](./API.ru.md)** | Справочник REST API со всеми endpoint'ами и форматами ответов | Frontend-разработчики, потребители API |
| **[application-design.ru.md](./application-design.ru.md)** | Полный обзор архитектуры, стек технологий, проектные решения | Архитекторы, senior-разработчики |
| **[grafana-dashboards.ru.md](./grafana-dashboards.ru.md)** | Интеграция с дашбордами Grafana, переменные, аутентификация, проверка доступности | DevOps, Операторы |
| **[graph-interactions.ru.md](./graph-interactions.ru.md)** | Взаимодействие с графом: выделение, перетаскивание, управление камерой | Пользователи, Frontend-разработчики |
| **[Развёртывание](../deploy/helm/dephealth-ui/README.ru.md)** | Руководство по развёртыванию в Kubernetes через Helm | DevOps, SRE |
| **[Руководство по тестовой среде](./test-environment.ru.md)** | **⭐ Переносимое** пошаговое руководство по развёртыванию полного тестового стека в любом Kubernetes-кластере | DevOps, Новые пользователи |
| **[Тестовая площадка](../deploy/README.ru.md)** | Настройка тестовой среды для хоумлаба, топология, адаптация под своё окружение | DevOps, Контрибьюторы |

## Быстрый старт

1. **Для пользователей:** Изучите [формат необходимых метрик](./METRICS.ru.md) и как инструментировать ваши сервисы
2. **Для разработчиков:** Прочитайте [API-документацию](./API.ru.md) для интеграции с dephealth-ui
3. **Для операторов:** Следуйте [руководству по развёртыванию Helm](../deploy/helm/dephealth-ui/README.ru.md)
4. **Для архитекторов:** Ознакомьтесь с [проектной документацией](./application-design.ru.md) для обзора системы

## Ключевые концепции

**Необходимые метрики:**
- `app_dependency_health` — Gauge (0/1), указывающий состояние здоровья зависимости
- `app_dependency_latency_seconds` — Histogram, измеряющий latency health check'ов
- `app_dependency_status` — Gauge (enum-паттерн), активная категория статуса (SDK v0.4.0+)
- `app_dependency_status_detail` — Gauge (info-паттерн), детальное описание статуса (SDK v0.4.0+)

**Метки SDK (обязательные):**
- `name` — Имя сервиса
- `group` — Логическая группа сервиса (обязательна с SDK v0.5.0; dephealth-ui работает без неё)
- `dependency` — Логическое имя зависимости
- `type` — Тип подключения (`http`, `grpc`, `tcp`, `postgres`, `mysql`, `redis`, `amqp`, `kafka`, `ldap`)
- `host` — Hostname целевого endpoint'а
- `port` — Порт целевого endpoint'а
- `critical` — Флаг критичности (`yes`/`no`)

**Дополнительные метки (не из SDK):**
- `namespace` — Kubernetes namespace (добавляется Prometheus; рекомендуется использовать вне K8s)
- `isentry` — Метка точки входа (рекомендуется для dephealth-ui)

**Поток интеграции:**
```
Ваш сервис (с dephealth SDK)
  ↓ экспортирует метрики
Prometheus/VictoriaMetrics
  ↓ собирает
dephealth-ui backend
  ↓ отдаёт JSON
dephealth-ui frontend (браузер)
  ↓ рендерит
Интерактивный граф топологии
```

---

## Вклад

Нашли ошибку или хотите улучшить документацию?

1. Отредактируйте соответствующий `.md` файл
2. Следуйте формату Conventional Commits
3. Создайте pull request

---

## Поддержка

- **Issues:** [GitHub Issues](https://github.com/BigKAA/dephealth-ui/issues)
- **dephealth SDK:** [topologymetrics](https://github.com/BigKAA/topologymetrics)

---

## Лицензия

См. [LICENSE](../LICENSE) в корне проекта.
