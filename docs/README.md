# dephealth-ui Documentation | Документация dephealth-ui

**Language:** [English](#english) | [Русский](#русский)

---

## English

### Overview

This directory contains comprehensive documentation for **dephealth-ui** — a real-time microservices topology and health visualization tool.

### Documents

| Document | Description | Audience |
|----------|-------------|----------|
| **[METRICS.md](./METRICS.md)** | **⭐ START HERE** — Metrics format specification, required labels, PromQL queries, integration guide | Developers, DevOps |
| **[API.md](./API.md)** | REST API reference with all endpoints and response formats | Frontend developers, API consumers |
| **[application-design.md](./application-design.md)** | Complete architecture overview, tech stack, design decisions | Architects, senior developers |
| **[DEPLOYMENT.md](../deploy/helm/dephealth-ui/README.md)** | Kubernetes deployment guide using Helm | DevOps, SRE |

### Quick Start

1. **For Users:** Learn about [required metrics format](./METRICS.md) and how to instrument your services
2. **For Developers:** Read [API documentation](./API.md) to integrate with dephealth-ui
3. **For Operators:** Follow [Helm deployment guide](../deploy/helm/dephealth-ui/README.md)
4. **For Architects:** Review [application design](./application-design.md) for system overview

### Key Concepts

**Metrics Required:**
- `app_dependency_health` — Gauge (0/1) indicating dependency health status
- `app_dependency_latency_seconds` — Histogram measuring health check latency

**Mandatory Labels:**
- `name` — Service name
- `namespace` — Kubernetes namespace
- `dependency` — Logical dependency name
- `type` — Connection type (http, grpc, postgres, redis, etc.)
- `host` — Target endpoint hostname
- `port` — Target endpoint port
- `critical` — Criticality flag (yes/no)

**Integration Flow:**
```
Your Service (with dephealth SDK)
  ↓ emits metrics
Prometheus/VictoriaMetrics
  ↓ scraped by
dephealth-ui backend
  ↓ serves JSON
dephealth-ui frontend (browser)
  ↓ renders
Interactive topology graph
```

---

## Русский

### Обзор

Этот каталог содержит полную документацию по **dephealth-ui** — инструменту визуализации топологии и здоровья микросервисов в реальном времени.

### Документы

| Документ | Описание | Аудитория |
|----------|----------|-----------|
| **[METRICS.md](./METRICS.md)** | **⭐ НАЧНИТЕ ОТСЮДА** — Спецификация формата метрик, обязательные метки, PromQL-запросы, руководство по интеграции | Разработчики, DevOps |
| **[API.md](./API.md)** | Справочник REST API со всеми endpoint'ами и форматами ответов | Frontend-разработчики, потребители API |
| **[application-design.md](./application-design.md)** | Полный обзор архитектуры, стек технологий, проектные решения | Архитекторы, senior-разработчики |
| **[DEPLOYMENT.md](../deploy/helm/dephealth-ui/README.md)** | Руководство по развёртыванию в Kubernetes через Helm | DevOps, SRE |

### Быстрый старт

1. **Для пользователей:** Изучите [формат необходимых метрик](./METRICS.md) и как инструментировать ваши сервисы
2. **Для разработчиков:** Прочитайте [API-документацию](./API.md) для интеграции с dephealth-ui
3. **Для операторов:** Следуйте [руководству по развёртыванию Helm](../deploy/helm/dephealth-ui/README.md)
4. **Для архитекторов:** Ознакомьтесь с [проектной документацией](./application-design.md) для обзора системы

### Ключевые концепции

**Необходимые метрики:**
- `app_dependency_health` — Gauge (0/1), указывающий состояние здоровья зависимости
- `app_dependency_latency_seconds` — Histogram, измеряющий latency health check'ов

**Обязательные метки:**
- `name` — Имя сервиса
- `namespace` — Kubernetes namespace
- `dependency` — Логическое имя зависимости
- `type` — Тип подключения (http, grpc, postgres, redis и т.д.)
- `host` — Hostname целевого endpoint'а
- `port` — Порт целевого endpoint'а
- `critical` — Флаг критичности (yes/no)

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

## Contributing | Вклад

Found an error or want to improve documentation?

1. Edit the relevant `.md` file
2. Follow Conventional Commits format
3. Submit a pull request

---

## Support | Поддержка

- **Issues:** [GitHub Issues](https://github.com/BigKAA/dephealth-ui/issues)
- **dephealth SDK:** [topologymetrics](https://github.com/BigKAA/topologymetrics)

---

## License | Лицензия

See [LICENSE](../LICENSE) in the project root.
