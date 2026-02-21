# План разработки: Optional AlertManager Support

## Метаданные

- **Версия плана**: 1.4.0
- **Дата создания**: 2026-02-21
- **Последнее обновление**: 2026-02-21
- **Статус**: Done

---

## История версий

- **v1.0.0** (2026-02-21): Начальная версия плана
- **v1.1.0** (2026-02-21): Phase 1 completed
- **v1.2.0** (2026-02-21): Phase 2 completed
- **v1.3.0** (2026-02-21): Phase 3.1 completed (Go tests pass)
- **v1.4.0** (2026-02-21): Phase 3 completed (Docker build, deploy, E2E pass)
- **v1.5.0** (2026-02-21): Phase 4 completed (Documentation updated)

---

## Текущий статус

- **Активная фаза**: All done
- **Активный подпункт**: —
- **Последнее обновление**: 2026-02-21
- **Примечание**: All phases complete. Documentation updated (application-design, API, CHANGELOG)

---

## Оглавление

- [x] [Phase 1: Backend — API признак доступности AlertManager](#phase-1-backend--api-признак-доступности-alertmanager)
- [x] [Phase 2: Frontend — disabled состояние и скрытие alert-элементов](#phase-2-frontend--disabled-состояние-и-скрытие-alert-элементов)
- [x] [Phase 3: Сборка и тестирование](#phase-3-сборка-и-тестирование)
- [x] [Phase 4: Документация](#phase-4-документация)

---

## Phase 1: Backend — API признак доступности AlertManager

**Dependencies**: None
**Status**: Done

### Описание

Добавить в API `/api/v1/config` признак `alertmanager.enabled`, чтобы фронтенд мог
отличить ситуацию «AlertManager не настроен» от «нет активных алертов».
Бэкенд уже корректно обрабатывает пустой URL (возвращает nil, nil из FetchAlerts),
нужно только пробросить этот признак в config API.

### Подпункты

- [x] **1.1 Добавить поле `Enabled` в configAlerts и заполнить в handleConfig**
  - **Dependencies**: None
  - **Description**: В структуру `configAlerts` (server.go:296) добавить поле
    `Enabled bool \`json:"enabled"\``. В `handleConfig` (server.go:324) вычислять
    `Enabled: s.cfg.Datasources.Alertmanager.URL != ""`.
    Это расширяет JSON-ответ `/api/v1/config` полем `alerts.enabled`, не ломая
    обратную совместимость.
  - **Changes**:
    - `internal/server/server.go` — struct `configAlerts` (line 296): add `Enabled bool`
    - `internal/server/server.go` — method `handleConfig` (line 345): set `Enabled`
  - **Links**: N/A

- [x] **1.2 Добавить тест для handleConfig с/без AlertManager URL**
  - **Dependencies**: 1.1
  - **Description**: В тестах сервера проверить, что:
    - при пустом `Alertmanager.URL` → `alerts.enabled = false`
    - при заполненном `Alertmanager.URL` → `alerts.enabled = true`
  - **Changes**:
    - `internal/server/server_test.go` — new/updated test case
  - **Links**: N/A

### Критерии завершения Phase 1

- [x] Все подпункты завершены (1.1, 1.2)
- [x] `GET /api/v1/config` возвращает `alerts.enabled: true/false`
- [x] Тесты проходят: `go test ./internal/server/...`
- [x] Обратная совместимость API сохранена

---

## Phase 2: Frontend — disabled состояние и скрытие alert-элементов

**Dependencies**: Phase 1
**Status**: Done

### Описание

При `alertmanager.enabled === false` в конфигурации:
- Кнопка alerts визуально неактивна с tooltip «Подключите AlertManager»
- Alert drawer не открывается
- Бейджи алертов на нодах и рёбрах графа не отрисовываются
- Секция алертов в сайдбаре не отрисовывается
- Счётчики алертов в статус-баре не показываются

### Подпункты

- [x] **2.1 Локализация — новые ключи**
  - **Dependencies**: None
  - **Description**: Добавить ключ `alerts.unavailable` в оба файла локализации:
    - `en.js`: `'alerts.unavailable': 'Connect AlertManager'`
    - `ru.js`: `'alerts.unavailable': 'Подключите AlertManager'`
  - **Changes**:
    - `frontend/src/locales/en.js` — add key after line 169
    - `frontend/src/locales/ru.js` — add key after line 169
  - **Links**: N/A

- [x] **2.2 CSS — стили disabled-кнопки**
  - **Dependencies**: None
  - **Description**: Добавить CSS-правило для `#btn-alerts.disabled`:
    ```css
    #btn-alerts.disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }
    ```
    Стиль уже есть для других disabled-элементов — проверить и следовать паттерну.
  - **Changes**:
    - `frontend/src/style.css` — add rule
  - **Links**: N/A

- [x] **2.3 alerts.js — поддержка disabled состояния**
  - **Dependencies**: 2.1, 2.2
  - **Description**: Модифицировать `initAlertDrawer(cy)` — добавить экспортируемую
    функцию `setAlertManagerAvailable(enabled)`:
    - Если `enabled === false`:
      - Добавить класс `disabled` на `#btn-alerts`
      - Установить `title` = `t('alerts.unavailable')`
      - Клик не должен открывать drawer (проверка в listener)
    - Если `enabled === true`:
      - Убрать класс `disabled`
      - Восстановить оригинальный title
      - Клик работает как обычно
    - Модифицировать `updateAlertDrawer()` — если disabled, сбрасывать данные
      и скрывать бейдж
  - **Changes**:
    - `frontend/src/alerts.js` — add `setAlertManagerAvailable()`, modify `initAlertDrawer()`
  - **Links**: N/A

- [x] **2.4 main.js — инициализация и передача состояния**
  - **Dependencies**: 2.3
  - **Description**: В функции `init()` после загрузки config:
    - Проверить `appConfig.alerts.enabled`
    - Вызвать `setAlertManagerAvailable(appConfig.alerts.enabled)`
    - Сохранить состояние в переменную модуля для использования в `updateStatus()`

    В функции `updateStatus()`:
    - Если AlertManager disabled — пропускать блок с alert counts в статус-баре
    - Если AlertManager disabled — не вызывать `updateAlertDrawer()`
  - **Changes**:
    - `frontend/src/main.js` — modify `init()`, `updateStatus()`
  - **Links**: N/A

- [x] **2.5 graph.js — скрытие alert-бейджей на графе**
  - **Dependencies**: 2.4
  - **Description**: Модифицировать функцию рендеринга alert-бейджей:
    - Добавить экспортируемую функцию `setAlertBadgesEnabled(enabled)` или
      принимать флаг через `appConfig` (уже передаётся в `renderGraph`)
    - Если AlertManager disabled:
      - Не создавать `alert-badge` элементы на нодах (line ~348)
      - Не создавать edge alert markers (line ~404)
      - Не применять стиль `node[alertCount > 0]` с толстой рамкой (line ~288)
      - Не заполнять `alertCount`/`alertSeverity` в data нод/рёбер

    `appConfig` уже передаётся в `renderGraph(cy, data, appConfig)` — использовать
    `appConfig.alerts.enabled` для условного рендеринга.
  - **Changes**:
    - `frontend/src/graph.js` — modify badge rendering, node/edge data mapping
  - **Links**: N/A

- [x] **2.6 sidebar.js — скрытие секции алертов**
  - **Dependencies**: 2.4
  - **Description**: Модифицировать функции `renderAlerts()` и `renderEdgeAlerts()`:
    - Добавить экспортируемый setter `setAlertManagerEnabled(enabled)`
      (или использовать глобальный config, следуя паттерну `setGrafanaConfig`)
    - Если disabled — `renderAlerts()` и `renderEdgeAlerts()` выходят без рендеринга
    - Скрывать строку `activeAlerts` в renderNodeDetails/renderEdgeDetails
  - **Changes**:
    - `frontend/src/sidebar.js` — modify `renderAlerts()`, `renderEdgeAlerts()`,
      add config setter
  - **Links**: N/A

### Критерии завершения Phase 2

- [x] Все подпункты завершены (2.1–2.6)
- [x] При `alertmanager.url: ""` кнопка alerts визуально disabled
- [x] Tooltip на disabled кнопке показывает локализованное сообщение
- [x] Клик по disabled кнопке не открывает drawer
- [x] На графе нет alert-бейджей (ни на нодах, ни на рёбрах)
- [x] В сайдбаре нет секции алертов
- [x] В статус-баре нет счётчиков алертов
- [x] При `alertmanager.url` заполненном — всё работает как раньше

---

## Phase 3: Сборка и тестирование

**Dependencies**: Phase 1, Phase 2
**Status**: Done

### Описание

Сборка Docker-образа, деплой в тестовый кластер, E2E проверка обоих сценариев
(с AlertManager и без).

### Подпункты

- [x] **3.1 Go тесты**
  - **Dependencies**: None
  - **Description**: Запустить полный набор Go-тестов:
    `go test ./...`
    Убедиться, что все тесты проходят, включая новые из Phase 1.
  - **Links**: N/A

- [x] **3.2 Сборка Docker-образа**
  - **Dependencies**: 3.1
  - **Description**: Собрать dev-образ:
    `make docker-build TAG=v0.17.0-2`
    Push в Harbor registry.
  - **Links**: N/A

- [x] **3.3 Деплой и E2E проверка**
  - **Dependencies**: 3.2
  - **Description**: Обновить Helm release в тестовом кластере.
    Проверить два сценария:

    **Сценарий A — AlertManager отключён (текущий конфиг):**
    - `alertmanager.url` пустой → `alerts.enabled = false`
    - Кнопка alerts disabled + tooltip
    - Нет alert-бейджей, нет секции в сайдбаре, нет счётчиков

    **Сценарий B — AlertManager включён:**
    - Прописать `alertmanager.url` → `alerts.enabled = true`
    - Кнопка alerts активна, drawer открывается
    - При наличии алертов — бейджи, секция, счётчики работают
  - **Links**: N/A

### Критерии завершения Phase 3

- [x] Все подпункты завершены (3.1–3.3)
- [x] Go-тесты проходят
- [x] Docker-образ собран и запушен (`harbor.kryukov.lan/library/dephealth-ui:v0.17.0-2`)
- [x] E2E проверка обоих сценариев пройдена
- [x] Нет регрессий в существующей функциональности

---

## Phase 4: Документация

**Dependencies**: Phase 3
**Status**: Done

### Описание

Обновить документацию проекта, отразив опциональность AlertManager.

### Подпункты

- [x] **4.1 Обновить docs/application-design.md**
  - **Dependencies**: None
  - **Description**: Добавить описание поведения UI без AlertManager:
    - Disabled-состояние кнопки alerts
    - Скрытие alert-связанных элементов
    - Описание поля `alerts.enabled` в config API
  - **Changes**:
    - `docs/application-design.md`
    - `docs/application-design.ru.md`
  - **Links**: N/A

- [x] **4.2 Обновить docs/API.md**
  - **Dependencies**: None
  - **Description**: Добавить поле `alerts.enabled` в описание ответа
    `GET /api/v1/config`.
  - **Changes**:
    - `docs/API.md`
    - `docs/API.ru.md`
  - **Links**: N/A

- [x] **4.3 Обновить CHANGELOG.md**
  - **Dependencies**: 4.1, 4.2
  - **Description**: Добавить запись об изменении в CHANGELOG.
  - **Changes**:
    - `CHANGELOG.md`
  - **Links**: N/A

### Критерии завершения Phase 4

- [x] Все подпункты завершены (4.1–4.3)
- [x] Документация описывает оба сценария (с/без AlertManager)
- [x] API-документация содержит поле `alerts.enabled`
- [x] CHANGELOG обновлён

---

## Примечания

- **Обратная совместимость**: API расширяется (новое поле `enabled`), существующие клиенты не ломаются
- **History mode**: Исторические алерты берутся из Prometheus через `ALERTS` метрику, не из AlertManager — поведение не затрагивается
- **Cascade analysis**: Не зависит от AlertManager, работает через Prometheus — без изменений
- **Partial data**: Кейс «AlertManager настроен, но недоступен» (partial=true, warning toast) сохраняется без изменений — это отдельный сценарий от «не настроен»
- **Dev tag**: следующий образ `v0.17.0-2`

---

**План готов к реализации.**
