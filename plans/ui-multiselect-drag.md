# План разработки: Multi-select и Drag с downstream

## Метаданные

- **Версия плана**: 1.1.0
- **Дата создания**: 2026-02-27
- **Последнее обновление**: 2026-02-27
- **Статус**: In Progress

---

## История версий

- **v1.0.0** (2026-02-27): Начальная версия плана
- **v1.1.0** (2026-02-27): Phases 1-4 implemented, awaiting build & test

---

## Текущий статус

- **Активная фаза**: Phase 5
- **Активный подпункт**: 5.1
- **Последнее обновление**: 2026-02-27
- **Примечание**: Phases 1-4 implemented, frontend builds successfully

---

## Оглавление

- [x] [Phase 1: Стили и инфраструктура](#phase-1-стили-и-инфраструктура)
- [x] [Phase 2: Множественное выделение (selection.js)](#phase-2-множественное-выделение)
- [x] [Phase 3: Перетаскивание с downstream (node-drag.js)](#phase-3-перетаскивание-с-downstream)
- [x] [Phase 4: Интеграция и центрирование камеры](#phase-4-интеграция-и-центрирование-камеры)
- [ ] [Phase 5: Сборка и тестирование](#phase-5-сборка-и-тестирование)

---

## Спецификация требований

### Множественное выделение узлов

- Ctrl+Click (Cmd+Click на Mac) = добавить/убрать узел из выделения (toggle)
- Ctrl+Drag на пустом месте = box-select (прямоугольная область)
- Click на пустом месте (без Ctrl) = сброс выделения
- Визуальная подсветка выделенных узлов (обводка, без мини-toolbar)
- Collapsed namespace выделяется как единый узел

### Перетаскивание

- Обычный drag на узле = перемещение одного узла (уже работает)
- Ctrl+Drag на узле = тянуть с 1 уровнем downstream-зависимостей
- Ctrl+Shift+Drag = тянуть со всем поддграфом downstream (BFS)
- Shared-зависимости обрабатываются как обычные (тянутся если попали в downstream)
- Если есть множественное выделение — drag на выделенном узле перемещает всю группу
- Позиции после drag временные — сбрасываются при обновлении данных / relayout / F5

### Центрирование камеры

- Двойной клик на пустом месте = плавная анимация камеры к точке клика

### Матрица событий

| Действие | Target | Ctrl/Cmd | Shift | Результат |
|----------|--------|----------|-------|-----------|
| Click | Нода | - | - | Sidebar toggle |
| Click | Нода | + | - | Select toggle |
| Click | Фон | - | - | Clear selection + close sidebar |
| Drag | Нода | - | - | Move single node |
| Drag | Нода (selected group) | - | - | Move group |
| Drag | Нода | + | - | Move + 1 lvl downstream |
| Drag | Нода | + | + | Move + full downstream |
| Drag | Фон | - | - | Pan |
| Drag | Фон | + | - | Box-select |
| Dbltap | Фон | - | - | Center camera |
| Dbltap | Нода[grafanaUrl] | - | - | Open Grafana |
| Dbltap | Collapsed ns | - | - | Expand namespace |
| Escape | - | - | - | Close all + clear selection |

---

## Архитектура

```
┌──────────────────────────────────────────────────────┐
│                     main.js                          │
│  init() → initSelection(cy) → initNodeDrag(cy)      │
│  dbltap background → animate center to point         │
│  Escape → clearSelection(cy)                         │
└──────────┬─────────────────────────┬─────────────────┘
           │                         │
  ┌────────▼────────────┐   ┌───────▼──────────────┐
  │   selection.js NEW  │   │  node-drag.js NEW    │
  │                     │   │                      │
  │ Ctrl+Click: toggle  │   │ grab: detect mode    │
  │ Ctrl+Drag bg: box   │   │ drag: move companions│
  │ Click bg: clear     │   │ free: cleanup        │
  │                     │   │                      │
  │ Uses :selected      │◄──┤ reads :selected      │
  │ Cytoscape state     │   │ for group drag       │
  └────────┬────────────┘   └───────┬──────────────┘
           │                         │
  ┌────────▼─────────────────────────▼─────────────────┐
  │               graph.js (styles MOD)                │
  │  node:selected { border: #2196f3, overlay }        │
  └────────────────────────────────────────────────────┘
           │
  ┌────────▼────────────┐
  │  sidebar.js MOD     │
  │  tap: skip if Ctrl  │
  └─────────────────────┘
```

### Изменяемые файлы

| Файл | Тип | Строк |
|------|-----|-------|
| `frontend/src/selection.js` | NEW | ~120 |
| `frontend/src/node-drag.js` | NEW | ~100 |
| `frontend/src/graph.js` | MOD | +15 |
| `frontend/src/main.js` | MOD | +15 |
| `frontend/src/sidebar.js` | MOD | +2 |
| `frontend/src/shortcuts.js` | MOD | +1 |
| `frontend/src/style.css` | MOD | +15 |
| `docs/graph-interactions.md` | NEW | ~80 |

---

## Phase 1: Стили и инфраструктура

**Dependencies**: None
**Status**: Done

### Описание

Add visual styles for `:selected` and box-select overlay. Prepare infrastructure for new modules.

### Подпункты

- [x] **1.1 Add `:selected` and `:grabbed` styles to graph.js**
  - **Dependencies**: None
  - **Description**: Add Cytoscape stylesheet entries for `node:selected` (blue border + overlay) to `cytoscapeStyles` array in `graph.js`. Style should work for both service and dependency nodes, including collapsed namespace nodes.
  - **Modifies**:
    - `frontend/src/graph.js` — add styles after existing node/edge selectors
  - **Links**:
    - [Cytoscape.js Selectors](https://js.cytoscape.org/#selectors/state)

- [x] **1.2 Add box-select CSS to style.css**
  - **Dependencies**: None
  - **Description**: Add `.box-select-rect` CSS class for the selection rectangle overlay (absolute positioned div with semi-transparent blue border and background, `pointer-events: none`, `z-index: 10`).
  - **Modifies**:
    - `frontend/src/style.css`
  - **Links**: N/A

### Критерии завершения Phase 1

- [ ] Все подпункты завершены (1.1, 1.2)
- [ ] `:selected` style visible when manually calling `node.select()` in console
- [ ] Box-select CSS class renders correctly when applied to a test div

---

## Phase 2: Множественное выделение

**Dependencies**: Phase 1
**Status**: Done

### Описание

Create `selection.js` module implementing Ctrl+Click toggle selection, Ctrl+Drag box-select on background, and click-to-clear. Modify `sidebar.js` to skip sidebar on Ctrl+Click.

### Подпункты

- [x] **2.1 Create selection.js — Ctrl+Click toggle**
  - **Dependencies**: None
  - **Description**: Create `frontend/src/selection.js` with `initSelection(cy)` and `clearSelection(cy)` exports. Implement Ctrl+Click (Cmd+Click on Mac) on nodes using `cy.on('tap', 'node')` with `evt.originalEvent.ctrlKey || evt.originalEvent.metaKey` check. Toggle node selected state via `node.select()`/`node.unselect()`. Implement click-on-background to clear all selection via `cy.on('tap')` where `evt.target === cy`.
  - **Creates**:
    - `frontend/src/selection.js`
  - **Links**:
    - [Cytoscape.js Events](https://js.cytoscape.org/#events)

- [x] **2.2 Add box-select to selection.js**
  - **Dependencies**: 2.1
  - **Description**: Add box-select functionality: listen to `pointerdown` on `cy.container()` with ctrlKey/metaKey. When triggered on background (not on a node): disable panning, create `.box-select-rect` overlay div, track `pointermove` to resize, on `pointerup` compute nodes within rendered bounds using `node.renderedPosition()`, select them, remove overlay, re-enable panning. Use `setPointerCapture` for reliable tracking.
  - **Modifies**:
    - `frontend/src/selection.js`
  - **Links**: N/A

- [x] **2.3 Modify sidebar.js — skip on Ctrl+Click**
  - **Dependencies**: 2.1
  - **Description**: In `sidebar.js` tap handler on node (line ~137), add early return if `evt.originalEvent.ctrlKey || evt.originalEvent.metaKey` — this prevents sidebar toggle when user is selecting nodes.
  - **Modifies**:
    - `frontend/src/sidebar.js`
  - **Links**: N/A

### Критерии завершения Phase 2

- [ ] Все подпункты завершены (2.1, 2.2, 2.3)
- [ ] Ctrl+Click on 3 different nodes → all 3 highlighted with blue border
- [ ] Repeat Ctrl+Click on one of them → deselected, 2 remain
- [ ] Click on background → all deselected
- [ ] Ctrl+Drag on background → blue rectangle appears, nodes inside get selected
- [ ] Ctrl+Click does NOT open sidebar
- [ ] Normal click on node still opens sidebar

---

## Phase 3: Перетаскивание с downstream

**Dependencies**: Phase 2
**Status**: Done

### Описание

Create `node-drag.js` module implementing group drag for selected nodes, Ctrl+Drag with 1-level downstream, and Ctrl+Shift+Drag with full downstream subgraph.

### Подпункты

- [x] **3.1 Create node-drag.js — group drag**
  - **Dependencies**: None
  - **Description**: Create `frontend/src/node-drag.js` with `initNodeDrag(cy)` export. On `cy.on('grab', 'node')`: if there are `:selected` nodes and grabbed node is in selection, save `startPositions` Map for all selected nodes. On `cy.on('drag', 'node')`: compute delta from grabbed node movement, apply same delta to all companion nodes. On `cy.on('free', 'node')`: clear drag state. This enables dragging a group of multi-selected nodes together while preserving relative positions.
  - **Creates**:
    - `frontend/src/node-drag.js`
  - **Links**: N/A

- [x] **3.2 Add downstream drag (1-level and full)**
  - **Dependencies**: 3.1
  - **Description**: Extend `node-drag.js` grab handler: if `ctrlKey && shiftKey` — collect full downstream via BFS (`node.outgoers('node')` recursively). If only `ctrlKey` — collect 1-level downstream (`node.outgoers('node')`). Add collected downstream nodes to companions Map. Implement `getDownstreamNodes(node, allLevels)` helper using BFS with `cy.collection()` for full traversal. Shared dependencies are included if they appear in downstream — no special handling.
  - **Modifies**:
    - `frontend/src/node-drag.js`
  - **Links**: N/A

### Критерии завершения Phase 3

- [ ] Все подпункты завершены (3.1, 3.2)
- [ ] Select 3 nodes, drag one of them → all 3 move together preserving relative positions
- [ ] Ctrl+Drag on a service with 2 dependencies → all 3 nodes move
- [ ] Ctrl+Shift+Drag on a service → entire downstream subtree moves
- [ ] Drag on non-selected node → only that node moves (no group)
- [ ] Works correctly with both dagre and fcose layouts

---

## Phase 4: Интеграция и центрирование камеры

**Dependencies**: Phase 2, Phase 3
**Status**: Done

### Описание

Wire up new modules in `main.js`, add dbltap-to-center on background, update `shortcuts.js` to clear selection on Escape.

### Подпункты

- [x] **4.1 Wire up modules in main.js**
  - **Dependencies**: None
  - **Description**: Import `initSelection`, `clearSelection` from `selection.js` and `initNodeDrag` from `node-drag.js`. Call `initSelection(cy)` and `initNodeDrag(cy)` in `init()` after `initContextMenu(cy)`. Add dbltap-on-background handler: `cy.on('dbltap', evt => { if (evt.target !== cy) return; })` — compute target pan to center camera on clicked model position using `cy.zoom()`, container dimensions, and `cy.animate({ pan, duration: 300 })`.
  - **Modifies**:
    - `frontend/src/main.js`
  - **Links**: N/A

- [x] **4.2 Update shortcuts.js — Escape clears selection**
  - **Dependencies**: None
  - **Description**: In the Escape handler in `shortcuts.js`, add call to `clearSelection(cy)` to deselect all nodes when user presses Escape. Import `clearSelection` from `selection.js`. This integrates with existing `closeAll()` behavior.
  - **Modifies**:
    - `frontend/src/shortcuts.js`
  - **Links**: N/A

- [ ] **4.3 Create graph interactions documentation**
  - **Dependencies**: 4.1, 4.2
  - **Description**: Create `docs/graph-interactions.md` documenting all mouse and keyboard interactions with the graph. Include the full event matrix table (action × target × modifiers → result), modifier key mapping for Win/Linux/Mac, description of selection modes (Ctrl+Click, box-select), drag modes (single, group, 1-level downstream, full downstream), and camera controls (pan, zoom, dbltap-center). Document edge cases: collapsed namespaces, compound nodes, shared dependencies. This serves as both user-facing and developer reference.
  - **Creates**:
    - `docs/graph-interactions.md`
  - **Links**: N/A

### Критерии завершения Phase 4

- [ ] Все подпункты завершены (4.1, 4.2, 4.3)
- [ ] Double-click on background → camera smoothly centers on that point
- [ ] Escape key clears both sidebar and selection
- [ ] All features work together without conflicts
- [ ] Double-click on node with Grafana URL still opens Grafana
- [ ] Double-click on collapsed namespace still expands it

---

## Phase 5: Сборка и тестирование

**Dependencies**: Phase 4
**Status**: Pending

### Описание

Build the application, deploy to test environment, and verify all interactions work correctly.

### Подпункты

- [ ] **5.1 Build and verify**
  - **Dependencies**: None
  - **Description**: Run `make docker-build` to build dev container image. Deploy to test Kubernetes cluster. Verify in browser that all existing features still work (sidebar, tooltips, context menu, search, grouping, collapse/expand). Then test all new features per acceptance criteria.
  - **Creates**:
    - Docker image (dev tag)
  - **Links**: N/A

- [ ] **5.2 Cross-browser testing**
  - **Dependencies**: 5.1
  - **Description**: Test in Chrome and Firefox. Verify Ctrl (Win/Linux) and Cmd (Mac) modifiers work. Verify box-select overlay renders correctly. Verify drag performance with large graphs (50+ nodes).
  - **Creates**:
    - Test results
  - **Links**: N/A

### Критерии завершения Phase 5

- [ ] Все подпункты завершены (5.1, 5.2)
- [ ] Контейнер успешно собран
- [ ] All existing features work without regression
- [ ] All new features pass acceptance criteria
- [ ] No console errors or warnings

---

## Примечания

- Cytoscape built-in `:selected` state is used for visual highlighting — no custom state management needed
- `boxSelectionEnabled` is `true` by default in Cytoscape (Shift+drag), but we implement custom box-select with Ctrl/Cmd modifier instead
- Cytoscape `tap` event only fires when no drag occurred — this naturally separates Ctrl+Click (select) from Ctrl+Drag (downstream drag)
- Modifier key detection pattern: `evt.originalEvent.ctrlKey || evt.originalEvent.metaKey` (already used in `shortcuts.js`)
- Compound parent nodes: Cytoscape automatically moves children when parent is dragged — no special handling needed

---
