# AGENTS.md

Инструкция для агентов ZCode, работающих с проектом **dephealth-ui**. Сначала прочитайте
этот файл; для полноты деталей смотрите `CLAUDE.md`, `GIT-WORKFLOW.md` и `docs/`.

> **Важно:** Общение с пользователем всегда вести **на русском языке**. Весь код,
> комментарии, документация и сообщения коммитов — **на английском**.

## Обзор проекта

dephealth-ui — инструмент визуализации состояния и топологии микросервисов. Бэкенд на Go
(chi) отдаёт SPA на Vite + Cytoscape.js, который рисует граф узлов со статусами сервисов
(OK/DEGRADED/DOWN), задержками соединений и ссылками на Grafana. Источники данных:
Prometheus/VictoriaMetrics (через topologymetrics) и AlertManager.

- Go-модуль: `github.com/BigKAA/dephealth-ui` (Go 1.25)
- Фронтенд: ванильный JS + Vite + Cytoscape.js + ELK + Tom Select

## Команды сборки / тестов / линтера (из Makefile и README)

- `make build` — только сборка Go: `go build -ldflags="-s -w" -o dephealth-ui ./cmd/dephealth-ui`
- `make frontend-build` — `npm --prefix frontend ci`, затем `npm --prefix frontend run build`
- `make test` — `go test ./... -v -race`
- `make lint` — `golangci-lint run ./...` + `markdownlint '**/*.md'` (игнорирует node_modules)
- Точечный Go-тест: `go test ./internal/topology/... -run TestName -v -race`
- Dev-сервер фронтенда: `npm --prefix frontend run dev` (HMR); прод-сборка: `npm --prefix frontend run build`
- Docker-образ для разработки → Yandex CR: `make docker-build TAG=vX.Y.Z-N`
- Docker-образ релиза → Yandex CR (мульти-арх amd64+arm64): `make docker-release TAG=vX.Y.Z`

## Архитектура и каталоги

```
cmd/dephealth-ui/    — точка входа; связывает конфиг и все внутренние пакеты
internal/
  config/            — загрузка и валидация YAML-конфига
  server/            — chi-роутер, встроенный статический SPA, gzip/cors middleware
  topology/          — клиент Prometheus + построение графа
  alerts/            — клиент AlertManager
  auth/              — basic / ldap / oidc аутентификация + сессии + rate limiting
  cache/             — кэширование ответов
  grafana/           — проверка доступности дашбордов при старте
  cascade/           — анализ каскадных сбоев
  timeline/          — лента событий
  export/            — экспорт SVG/PNG/JSON/CSV/DOT (с тестами)
  logging/           — структурированный логгер slog + HTTP middleware
frontend/src/        — модули SPA на ванильном JS (graph, sidebar, i18n, export, ...)
deploy/helm/         — чарт приложения + тестовые чарты infra/monitoring/uniproxy
docs/                — API, дизайн, метрики, grafana (EN + RU)
plans/               — поэтапные планы разработки/тестирования (используйте шаблон .templates/)
```

## Важный нюанс процесса сборки

Фронтенд встраивается в Go-бинарник через `//go:embed all:static` в
`internal/server/static.go`. Dockerfile (мультисборка) собирает `frontend/`, затем копирует
`dist/` в `internal/server/static/`.

**Локальный `make build` НЕ пересобирает фронтенд.** Чтобы применить изменения фронтенда
локально: выполните `make frontend-build` (результат в `frontend/dist`), затем скопируйте
его в `internal/server/static/` перед `make build`. В git отслеживается только
`internal/server/static/.gitkeep`; собранные ассеты генерируются и не хранятся в репозитории.

## API

REST под `/api/v1/*` на chi-роутере. Auth middleware (`s.auth.Middleware()`) защищает
группу роутов `/api/v1`. Эндпоинты без аутентификации: `/healthz`, `/readyz`, `/auth`,
`/api/v1/config`. Роуты группы: `topology`, `alerts`, `instances`, `cascade-analysis`,
`cascade-graph`, `timeline/events`, `export/{format}`.

При добавлении эндпоинтов регистрируйте их внутри группы роутов `/api/v1` в
`internal/server/server.go`, чтобы применялась аутентификация.

## Соглашения

- **Общение с пользователем — на русском языке.** Весь код, комментарии, документация и
  сообщения коммитов — на английском.
- Conventional Commits: `<type>(<scope>): <subject>` (типы: feat, fix, docs, style,
  refactor, test, chore). Ветвление от `master` с префиксами: `feature/`, `bugfix/`,
  `docs/`, `refactor/`, `test/`, `hotfix/`.
- Спрашивать пользователя перед коммитом. После коммита уточнять метод слияния (локальный
  merge или GitHub PR). Удалять ветки после слияния. Быстрые правки (опечатки) можно
  коммитить сразу в `master`.
- Разработка и тестирование ведутся **в Docker или Kubernetes** (homelab-кластер, образы
  в Yandex CR) — подробности по кластеру/реестрам см. в CLAUDE.md.
- Для схем в md файлах вместо txt использовать mermaid. За исключением иерархии
  файловых систем.
- Выполнять `make lint` (Go + Markdown) перед завершением работы.

## Документация для чтения перед изменением критичных областей

- `CLAUDE.md` — полный обзор проекта, окружение, реестры, чеклист релиза
- `GIT-WORKFLOW.md` — правила ветвления, тегирование/релизы
- `docs/application-design.md` — архитектура
- `docs/API.md` — контракт REST API
- `.templates/DEVELOPMENT_PLAN_TEMPLATE.md` — обязательный формат для новых планов разработки
