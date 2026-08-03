# План разработки: Чтение лейбла `path` и корректная дедупликация в dephealth-ui

## 📋 Метаданные

- **Версия плана**: 1.0.0
- **Дата создания**: 2026-07-30
- **Последнее обновление**: 2026-07-31
- **Статус**: Partially superseded
- **Репозиторий**: `dephealth-ui`
- **Связанные планы**: `topologymetrics/plans/add-path-label.md` (предусловие), `uniproxy/plans/add-path-label.md` (источник метрик)

> **Обновление 2026-07-31.** Базовая дедупликация по `(name, dependency)` уже реализована (ребро и
> dependency-узел идентифицируются парой «сервис → зависимость», ID узла `{source}/{dependency}`).
> Таким образом, основной баг (схлопывание разных зависимостей за одним `host:port`) уже исправлен
> без ожидания лейбла `path`. Описанный ниже вариант с `path` остаётся как **будущее уточнение**:
> при появлении лейбла `path` (SDK v0.9.0) он наслаивается поверх `(name, dependency)` как
> дополнительный discriminator, а не заменяет текущую модель. Разделы плана ниже сохранены как
> roadmap для фазы `path` и местами устарели относительно текущего кода.

---

## 📚 История версий

- **v1.0.0** (2026-07-30): Начальная версия плана

---

## 📍 Текущий статус

- **Активная фаза**: Phase 1
- **Активный подпункт**: 1.1
- **Последнее обновление**: 2026-07-30
- **Примечание**: Выполняется **после** релиза SDK v0.9.0 и обновления uniproxy. dephealth-ui делается обратно-совместимым: работает и со старыми метриками (без `path`, fallback), и с новыми (полная корректность).

---

## 📑 Оглавление

- [ ] [Phase 1: Бэкенд — модели, запросы, граф](#phase-1-бэкенд--модели-запросы-граф)
- [ ] [Phase 2: Слой экспорта (опционально)](#phase-2-слой-экспорта-опционально)
- [ ] [Phase 3: Фронтенд](#phase-3-фронтенд)
- [ ] [Phase 4: Тесты](#phase-4-тесты)
- [ ] [Phase 5: Сборка и валидация](#phase-5-сборка-и-валидация)

---

## Контекст и причина

dephealth-ui сейчас дедуплицирует узлы-зависимости и рёбра по `host:port`, игнорируя путь и имя зависимости. Из-за этого `nginx-back-app1`, у которого 4 разные зависимости идут через один `host:port` (smart.prod-dc-cloud2.passport.local:80), схлопывается в 1 узел/ребро вместо 4. Метрики теперь (SDK v0.9.0) несут лейбл `path` — dephealth-ui должен его читать и использовать для идентификации.

### Принятые решения (project-wide)
1. **Дедуп**: ключ `{host, port, path}`. Если `path` пуст (старые метрики без лейбла) — **fallback на `dependency`-имя**, чтобы разные зависимости на одном `host:port` не склеивались даже для старых данных.
2. dephealth-ui **обратно-совместим**: работает и со старыми метриками (без `path`), и с новыми.

### Ключевая идея — единая функция идентичности
Чтобы node-ID и lookup-ключи health/latency/status были консистентны и для новых, и для старых метрик, ввести один хелпер, используемый **везде** (resolveTarget + все EdgeKey-конструкции):

```go
// pathIdentity returns the endpoint identity suffix for node IDs and edge keys.
// Uses the URL path label when present (SDK >= 0.9.0). For legacy metrics without
// the path label, falls back to the dependency name so distinct dependencies on
// the same host:port are not collapsed.
func pathIdentity(path, dependency string) string {
    if path != "" {
        return path
    }
    return "/@dep/" + dependency
}
```

### Подтверждение предусловия
- [ ] SDK v0.9.0 выпущен, uniproxy обновлён и эмитит `path` (см. связанные планы).

---

## Phase 1: Бэкенд — модели, запросы, граф

**Dependencies**: None
**Status**: Pending
**Path**: `internal/topology/`

### Подпункты

- [ ] **1.1 Модели**
  - **Dependencies**: None
  - **Description**: Добавить поле `Path` в структуры.
  - **Creates**: изменения в `internal/topology/models.go`:
    - `EdgeKey` (≈ 94-99): добавить `Path string`
    - `TopologyEdge` (≈ 101-112): добавить `Path string`
    - `Node` (≈ 6-21): добавить `Path string \`json:"path,omitempty"\``
  - **Links**: `internal/topology/models.go`

- [ ] **1.2 PromQL и декодирование**
  - **Dependencies**: None
  - **Description**: Добавить `path` в group-by и читать лейбл из ответа Prometheus.
  - **Creates**: изменения в `internal/topology/prometheus.go`:
    - `queryTopologyEdges` (≈ строка 83) — добавить `path` в `group by (...)`
    - `queryTopologyEdgesLookback` (≈ строка 89) — добавить `path` в `group by (...)`
    - `QueryTopologyEdges` (≈ 329-351) — `TopologyEdge{...}` + `Path: r.Metric["path"]`
    - `QueryTopologyEdgesLookback` (≈ 353-376) — то же
    - `QueryStatusRange` (≈ 275-279) — `EdgeKey{...}` + `Path: pathIdentity(r.Metric["path"], r.Metric["dependency"])`
    - `parseEdgeStringValues` (≈ 463-467) — то же
    - `parseEdgeValues` (≈ 510-514) — то же
  - **Links**: `internal/topology/prometheus.go`
  - **Code examples**
    ```go
    Path: pathIdentity(r.Metric["path"], r.Metric["dependency"]),
    ```

- [ ] **1.3 Граф и хелпер идентичности**
  - **Dependencies**: 1.1, 1.2
  - **Description**: Перестать схлопывать зависимости с разными путями; единый `pathIdentity`.
  - **Creates**: изменения в `internal/topology/graph.go`:
    - ввести хелпер `pathIdentity(path, dependency string) string` (см. контекст)
    - `resolveTarget` (≈ 199-208): `return e.Host + ":" + e.Port + pathIdentity(e.Path, e.Dependency)`
    - `nodeInfo` (≈ 177-186): добавить поле `path string`
    - регистрация dep-узла (≈ 236-243): `path: e.Path`
    - `EdgeKey` literal (≈ строка 211): `Path: pathIdentity(e.Path, e.Dependency)`
    - `Node{...}` (≈ 400-411): `Path: info.path`
    - (опц.) `linkGrafanaURL` (≈ 536-543): добавить `&var-path=%s`, если дашборд scoped по пути; и `path` в вызовах (≈ 282, 309)
  - **Links**: `internal/topology/graph.go`
  - **Code examples**
    ```go
    func (b *GraphBuilder) resolveTarget(e TopologyEdge) string {
        if b.serviceNames[e.Dependency] {
            return e.Dependency
        }
        return e.Host + ":" + e.Port + pathIdentity(e.Path, e.Dependency)
    }
    ```
  - **Примечание**: `resolveTarget` вызывается и во втором проходе namespace-inheritance (≈ 359-366) — корректность сохраняется, т.к. работает через `TopologyEdge`.

### ✅ Критерии завершения Phase 1

- [ ] Поле `Path` есть в `EdgeKey`/`TopologyEdge`/`Node`
- [ ] `path` в group-by обеих topology-запросов
- [ ] Единый `pathIdentity` используется в resolveTarget и всех EdgeKey-конструкциях
- [ ] `make build` успешен, `go vet ./...` чист

---

## Phase 2: Слой экспорта (опционально)

**Dependencies**: Phase 1
**Status**: Pending
**Path**: `internal/export/`

### Подпункты

- [ ] **2.1 Экспортные модели и CSV**
  - **Dependencies**: None
  - **Description**: При желании показывать `path` в SVG/PNG/JSON/CSV/DOT.
  - **Creates**: изменения в:
    - `internal/export/model.go` — `ExportNode`/`ExportEdge`: добавить `Path`; заполнить при сборке (≈ 60-68, 74-86)
    - `internal/export/csv.go` (≈ 72-90) — колонка `path` в заголовке и строках
  - **Links**: `internal/export/model.go`, `internal/export/csv.go`

### ✅ Критерии завершения Phase 2

- [ ] Экспорт содержит `path` (если фаза выполнялась)

---

## Phase 3: Фронтенд

**Dependencies**: Phase 1
**Status**: Pending
**Path**: `frontend/src/`

### Подпункты

- [ ] **3.1 Граф: данные узла, подпись, размер**
  - **Dependencies**: None
  - **Description**: Передать `path` в Cytoscape, показать его в подписи dep-узла, учесть в расчёте размера узла.
  - **Creates**: изменения в `frontend/src/graph.js`:
    - `renderGraph` nodeData (≈ 761-775): добавить `path: node.path || undefined`
    - `nodeLabel` (≈ 78-89): для dep-узла вторая строка `${host}:${port}${path ? ' ' + path : ''}`
    - `makeNodeStyle` width/height (≈ 114-132): учесть `path` в `secondLine`, чтобы размер узла вмещал подпись
  - **Links**: `frontend/src/graph.js`

- [ ] **3.2 Sidebar / tooltip / Grafana-ссылка**
  - **Dependencies**: None
  - **Description**: Показать `path` в деталях; при необходимости добавить `var-path` в Grafana-ссылки.
  - **Creates**: изменения в:
    - `frontend/src/sidebar.js` (≈ 318-319): строка деталей — показать `path` для dep-узла
    - `frontend/src/tooltip.js` (опц.): показать `path`
    - `frontend/src/sidebar.js` (≈ 577-582) (опц.): Grafana-ссылка — `var-path`
  - **Links**: `frontend/src/sidebar.js`, `frontend/src/tooltip.js`

### ✅ Критерии завершения Phase 3

- [ ] dep-узел показывает `host:port path` в подписи и деталях
- [ ] Размер узла корректно вмещает расширенную подпись

---

## Phase 4: Тесты

**Dependencies**: Phase 1
**Status**: Pending
**Path**: `internal/topology/`

### Подпункты

- [ ] **4.1 Тесты prometheus.go**
  - **Dependencies**: None
  - **Description**: Обновить точные строки запросов и EdgeKey-литералы.
  - **Creates**: изменения в `internal/topology/prometheus_test.go`:
    - точные строки запросов с `group by` (≈ 190, 196, 230, 237, 663, 670) — добавить `path`
    - `EdgeKey{...}` литералы (≈ 126, 131, 151, 312, 317, 340, 357, 360, 363) — добавить `Path`

- [ ] **4.2 Тесты graph.go: новая семантика дедупликации**
  - **Dependencies**: None
  - **Description**: Зафиксировать новое поведение.
  - **Creates**: изменения в `internal/topology/graph_test.go`:
    - `TestDepNodeDedup_SameEndpointMerges` (≈ 243-286) — **ключевой тест**: добавить кейс «same host:port, разные `path` → НЕ мерджатся»; оставить кейс «одинаковый `path` → мердж»
    - счётчики узлов/рёбер и `EdgeKey`-литералы: `TestGraphBuilder_Build` (≈ 106), `TestDepNodeDedup_DifferentGroupsMerge` (≈ 288), `TestEdgeDedup_*` (≈ 1773, 1839, литералы ≈ 1779, 1842)
    - **новый тест** `TestDepNode_SameHost_DifferentPath_NotCollapsed` — fixtures как в реальном кейсе `nginx-back-app1` (4 зависимости, один host:port, разные пути)

### ✅ Критерии завершения Phase 4

- [ ] `go test ./internal/topology/... -v -race` проходит
- [ ] Новый тест `TestDepNode_SameHost_DifferentPath_NotCollapsed` зелёный
- [ ] Старые тесты дедупликации обновлены под новую семантику

---

## Phase 5: Сборка и валидация

**Dependencies**: Phase 1, Phase 2, Phase 3, Phase 4
**Status**: Pending

### Подпункты

- [ ] **5.1 Сборка фронтенда и бинаря**
  - **Dependencies**: None
  - **Description**: `make frontend-build` → скопировать `dist/` в `internal/server/static/` → `make build`.
  - **Creates**: собранный бинарь
  - **Links**: `Makefile`, `AGENTS.md` (нюанс сборки)

- [ ] **5.2 Линт и тесты**
  - **Dependencies**: None
  - **Description**: `make lint` (Go + Markdown), `make test`.
  - **Creates**: Test results

- [ ] **5.3 Smoke по реальным метрикам**
  - **Dependencies**: None
  - **Description**: Подать метрики из `tmp/metrics.txt` (с добавленным лейблом `path` для каждой из 4 зависимостей) и убедиться, что `nginx-back-app1` разворачивается в **4 отдельных ребра/узла**. API-проверка:
    ```bash
    curl -s '<host>/api/v1/topology' -u user:pass \
      | jq '.edges[] | select(.source=="nginx-back-app1")'
    ```
    Ожидание: 4 ребра. Проверить отображение во фронтенде.

### ✅ Критерии завершения Phase 5

- [ ] `make frontend-build` + `make build` успешны
- [ ] `make lint` и `make test` зелёные
- [ ] Smoke: `nginx-back-app1` → 4 ребра (а не 1)
- [ ] Во фронтенде виден `path` на dep-узлах

---

## 📝 Примечания

- **Обратная совместимость**: `group by (path)` на метриках без лейбла `path` (старые uniproxy) даёт `path=""` → `pathIdentity` возвращает `/@dep/<dependency>`, разные зависимости разделяются. Новые данные с `path` → разделяются по пути. Миграция прозрачная.
- Сценарий «connected graph» (зависимость сама является сервисом) не затрагивается — `resolveTarget` по-прежнему возвращает имя сервиса; `path` добавляется только к dep-узлам.
- C#-позиционный массив (см. план SDK) — это риск только на стороне SDK; в dephealth-ui все EdgeKey/TopologyEdge строятся по именам полей, сдвига нет.

---

**🎯 План готов к использованию. Удачной разработки!**
