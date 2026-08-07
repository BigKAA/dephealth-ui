# План разработки: Focus Mode — выделение связанных элементов графа

## 📋 Метаданные

- **Версия плана**: 1.4.0
- **Дата создания**: 2026-03-02
- **Последнее обновление**: 2026-03-02
- **Статус**: Done

---

## 📚 История версий

- **v1.4.0** (2026-03-02): Code review Phase 4 — collapsed focus works without code (verification only), filter/search interaction works via style priority (no graph-utils changes), clear focus on structure change only (not every poll), clearFocus in dbltap handler for collapse/expand
- **v1.3.0** (2026-03-02): Code review Phase 3 — remove redundant edgesWith filter, add upstream code example, document successors vs edgesWith edge collection rationale
- **v1.2.0** (2026-03-02): Code review Phase 2+3 — focus-switch class cleanup bug, .focused opacity, parent node handling, selection via 'select' event (no circular dep), sidebar.js shiftKey guard, isFocusActive() export
- **v1.1.0** (2026-03-02): Code review Phase 1 — successors/predecessors builtins, dimmed opacity 0.2, text-opacity fix, style insertion order, focus.js skeleton with interaction contract
- **v1.0.0** (2026-03-02): Начальная версия плана

---

## 📍 Текущий статус

- **Активная фаза**: Phase 5
- **Активный подпункт**: 5.1
- **Последнее обновление**: 2026-03-02
- **Примечание**: Plan created from brainstorm session

---

## 📑 Оглавление

- [x] [Phase 1: Core infrastructure — module, styles, graph traversal](#phase-1-core-infrastructure)
- [x] [Phase 2: Basic focus mode — click, 1-hop highlight, direction colors](#phase-2-basic-focus-mode)
- [x] [Phase 3: Downstream and upstream focus — Shift+Click, Shift+Alt+Click](#phase-3-downstream-and-upstream-focus)
- [x] [Phase 4: Edge cases and integration — collapsed namespaces, filters, multi-select](#phase-4-edge-cases-and-integration)
- [x] [Phase 5: Build, test, docs](#phase-5-build-test-docs)

---

## Phase 1: Core infrastructure

**Dependencies**: None
**Status**: Done

### Описание

Extract graph traversal utilities from `node-drag.js` into a shared module. Create the `focus.js` module skeleton with Cytoscape class-based styles for focus/dim states. This phase lays the groundwork without changing any user-facing behavior.

### Подпункты

- [x] **1.1 Extract graph traversal into shared utility**
  - **Dependencies**: None
  - **Description**: Move `getDownstreamNodes()` from `node-drag.js` to a new `graph-utils.js`. Simplify full-BFS case using Cytoscape builtins `successors('node')` / `predecessors('node')` instead of manual BFS (project convention — see `cytoscape-patterns` memory). Add `getUpstreamNodes()` using `predecessors('node')`. Add `getConnectedElements(node)` returning `{ incomingEdges, outgoingEdges, sourceNodes, targetNodes }`. Keep `node-drag.js` importing from the new module — no behavior change.
  - **Creates**:
    - `frontend/src/graph-utils.js`
  - **Modifies**:
    - `frontend/src/node-drag.js` — replace local function with import
  - **Links**:
    - [Cytoscape traversal API](https://js.cytoscape.org/#collection/traversing)
  - **Code examples**:

    ```javascript
    // graph-utils.js

    /**
     * Get 1-hop connected elements for a node (edges + neighbor nodes),
     * split by direction.
     */
    export function getConnectedElements(node) {
      const incomingEdges = node.incomers('edge');
      const outgoingEdges = node.outgoers('edge');
      const sourceNodes = incomingEdges.sources();
      const targetNodes = outgoingEdges.targets();
      return { incomingEdges, outgoingEdges, sourceNodes, targetNodes };
    }

    /**
     * Get downstream nodes (follow outgoing edges).
     * Uses Cytoscape builtin successors() for full BFS — handles cycles correctly.
     * @param {cytoscape.NodeSingular} node - Starting node
     * @param {boolean} allLevels - false = 1-hop only, true = full traversal
     * @returns {cytoscape.Collection} Collection of downstream nodes (excluding start)
     */
    export function getDownstreamNodes(node, allLevels) {
      if (!allLevels) {
        return node.outgoers('node');
      }
      return node.successors('node');
    }

    /**
     * Get upstream nodes (follow incoming edges).
     * Uses Cytoscape builtin predecessors() for full BFS — handles cycles correctly.
     * @param {cytoscape.NodeSingular} node - Starting node
     * @param {boolean} allLevels - false = 1-hop only, true = full traversal
     * @returns {cytoscape.Collection} Collection of upstream nodes (excluding start)
     */
    export function getUpstreamNodes(node, allLevels) {
      if (!allLevels) {
        return node.incomers('node');
      }
      return node.predecessors('node');
    }

    // NOTE: cascade.js intentionally uses its own manual BFS because it
    // needs to filter by edge.data('critical') at each step.
    // Do not consolidate cascade traversal into this module.
    ```

- [x] **1.2 Add Cytoscape focus/dim style selectors**
  - **Dependencies**: None (parallel with 1.1)
  - **Description**: Add new style rules to `cytoscapeStyles` array in `graph.js` **after the `node:selected` rule** (last rule in the array, ~line 332). This order is critical: focus classes must override `:selected` styles, and later rules win in Cytoscape. Use Cytoscape classes (not data attributes) for performance — `cy.batch()` + `addClass/removeClass` is the fastest approach. Define 5 classes: `.focused` (the clicked node), `.focus-neighbor` (connected nodes at full opacity), `.focus-edge-in` (incoming edges — blue), `.focus-edge-out` (outgoing edges — purple), `.dimmed` (everything else — low opacity + desaturation). **Important**: dimmed opacity uses 0.2 (not 0.15) to visually distinguish from search-hidden elements which use 0.15. All edge focus classes must explicitly set `text-opacity: 1` to override `edge.dimmed` which sets `text-opacity: 0`.
  - **Modifies**:
    - `frontend/src/graph.js` — append to `cytoscapeStyles` array after `node:selected` rule
  - **Links**:
    - [Cytoscape style](https://js.cytoscape.org/#style)
  - **Code examples**:

    ```javascript
    // Append to cytoscapeStyles array in graph.js AFTER the `node:selected` rule

    // --- Focus mode styles ---
    // Dimmed elements (everything not in focus).
    // Uses opacity 0.2 (not 0.15) to visually distinguish from search-hidden (0.15).
    {
      selector: '.dimmed',
      style: {
        'opacity': 0.2,
        'text-opacity': 0.2,
      },
    },
    // Dimmed edges: also hide labels
    {
      selector: 'edge.dimmed',
      style: {
        'opacity': 0.12,
        'text-opacity': 0,
      },
    },
    // The focused (clicked) node — prominent highlight.
    // opacity/text-opacity: 1 required to fully override .dimmed if both classes present.
    {
      selector: '.focused',
      style: {
        'opacity': 1,
        'text-opacity': 1,
        'border-width': 5,
        'border-color': '#2196f3',
        'overlay-color': '#2196f3',
        'overlay-opacity': 0.12,
        'overlay-padding': 6,
        'z-index': 10,
      },
    },
    // Neighbor nodes visible at full opacity (undoes any dimming)
    {
      selector: '.focus-neighbor',
      style: {
        'opacity': 1,
        'text-opacity': 1,
        'z-index': 9,
      },
    },
    // Incoming edges (who calls me) — blue.
    // text-opacity: 1 is required to override edge.dimmed text-opacity: 0.
    {
      selector: '.focus-edge-in',
      style: {
        'line-color': '#42a5f5',
        'target-arrow-color': '#42a5f5',
        'opacity': 1,
        'text-opacity': 1,
        'width': 2.5,
        'z-index': 10,
      },
    },
    // Outgoing edges (who I depend on) — purple.
    // text-opacity: 1 is required to override edge.dimmed text-opacity: 0.
    {
      selector: '.focus-edge-out',
      style: {
        'line-color': '#ab47bc',
        'target-arrow-color': '#ab47bc',
        'opacity': 1,
        'text-opacity': 1,
        'width': 2.5,
        'z-index': 10,
      },
    },
    // Downstream/upstream traversal edges — keep state colors, just ensure visibility.
    // text-opacity: 1 is required to override edge.dimmed text-opacity: 0.
    {
      selector: '.focus-traversal',
      style: {
        'opacity': 1,
        'text-opacity': 1,
        'width': 2.5,
        'z-index': 10,
      },
    },
    ```

- [x] **1.3 Create focus.js module skeleton**
  - **Dependencies**: 1.1, 1.2
  - **Description**: Create `frontend/src/focus.js` with `initFocusMode(cy)` and `clearFocus(cy)` exports. Wire it into `main.js` initialization after `initNodeDrag(cy)`. In this phase the module only exports the init function with empty event handlers and the FOCUS_CLASSES constant — actual logic in Phase 2. **Important design note**: both `focus.js` and `selection.js` register `cy.on('tap')` background handlers. In Cytoscape all handlers fire (no stopPropagation). This is by design — Phase 2 (item 2.2) resolves the interaction. Document this contract in the skeleton's JSDoc.
  - **Creates**:
    - `frontend/src/focus.js`
  - **Modifies**:
    - `frontend/src/main.js` — add `import` and `initFocusMode(cy)` call
  - **Code examples**:

    ```javascript
    // focus.js — Phase 1 skeleton

    /**
     * Focus mode module: highlight a node and its connections, dim everything else.
     *
     * Event interaction contract:
     * - Plain click on node → focus mode (this module)
     * - Ctrl/Meta+Click on node → multi-select (selection.js)
     * - Plain click on background → clear focus (this module) AND clear selection (selection.js)
     * Both background tap handlers fire independently (Cytoscape has no stopPropagation).
     * Phase 2 adds explicit cross-module clearing to avoid conflicting states.
     */

    const FOCUS_CLASSES = ['focused', 'focus-neighbor', 'focus-edge-in',
                           'focus-edge-out', 'focus-traversal', 'dimmed'];

    let focusActive = false;

    /**
     * Clear all focus mode classes from the graph.
     * @param {cytoscape.Core} cy
     */
    export function clearFocus(cy) {
      if (!focusActive) return;
      cy.batch(() => {
        cy.elements().removeClass(FOCUS_CLASSES.join(' '));
      });
      focusActive = false;
    }

    /**
     * Initialize focus mode on the Cytoscape instance.
     * Phase 1: skeleton only, no event handlers yet.
     * @param {cytoscape.Core} cy
     */
    export function initFocusMode(cy) {
      // Event handlers added in Phase 2
    }
    ```

### ✅ Критерии завершения Phase 1

- [x] All subtasks completed (1.1, 1.2, 1.3)
- [x] `node-drag.js` still works correctly (Ctrl+Drag downstream unchanged)
- [x] No visual changes to the graph (new styles have no effect without classes applied)
- [x] `focus.js` is initialized without errors

---

## Phase 2: Basic focus mode — click, 1-hop highlight, direction colors

**Dependencies**: Phase 1
**Status**: Done

### Описание

Implement the core focus interaction: click on a node to highlight it and its direct connections (1-hop), dim everything else. Incoming edges are blue, outgoing edges are purple. Click on another node switches focus. Click on background clears focus. Edge labels are hidden on dimmed edges.

### Подпункты

- [x] **2.1 Implement focus activation and clearing**
  - **Dependencies**: None
  - **Description**: In `focus.js`, implement click handler on nodes (without modifier keys) that: (1) **clears all old focus classes** and applies `.dimmed` to all elements in one step, (2) removes `.dimmed` and applies `.focused` to the clicked node, (3) applies direction-specific classes to connected edges and neighbor nodes, (4) undims parent nodes of focused/neighbor children to preserve namespace group boundaries. Use `cy.batch()` for performance. Implement background click to clear all focus classes. Export `clearFocus(cy)` and `isFocusActive()` for use by other modules. **Critical**: the clear+dim step must be combined (`removeClass(allClasses).addClass('dimmed')`) to prevent stale focus classes on previously focused elements when switching focus between nodes. Skip parent/group nodes in tap handler (`node.isParent()` guard) — collapsed namespace focus is Phase 4.
  - **Modifies**:
    - `frontend/src/focus.js`
  - **Links**:
    - [Cytoscape batch](https://js.cytoscape.org/#cy.batch)
    - [Cytoscape classes](https://js.cytoscape.org/#ele.addClass)
  - **Code examples**:

    ```javascript
    // focus.js — core logic

    import { getConnectedElements } from './graph-utils.js';
    import { clearSelection } from './selection.js';

    const FOCUS_CLASSES = ['focused', 'focus-neighbor', 'focus-edge-in',
                           'focus-edge-out', 'focus-traversal', 'dimmed'];
    const ALL_CLASSES_STR = FOCUS_CLASSES.join(' ');

    let focusActive = false;

    /**
     * Whether focus mode is currently active.
     * @returns {boolean}
     */
    export function isFocusActive() {
      return focusActive;
    }

    export function clearFocus(cy) {
      if (!focusActive) return;
      cy.batch(() => {
        cy.elements().removeClass(ALL_CLASSES_STR);
      });
      focusActive = false;
    }

    function applyFocus(cy, node) {
      const { incomingEdges, outgoingEdges, sourceNodes, targetNodes } =
        getConnectedElements(node);

      // Clear selection — focus and multi-select are mutually exclusive
      clearSelection(cy);

      cy.batch(() => {
        // 1. Clear ALL old focus classes + dim everything in one step.
        //    This prevents stale .focused/.focus-neighbor/.focus-edge-*
        //    classes from persisting on previously focused elements.
        cy.elements().removeClass(ALL_CLASSES_STR).addClass('dimmed');

        // 2. Highlight focused node
        node.removeClass('dimmed').addClass('focused');

        // 3. Highlight neighbors
        sourceNodes.removeClass('dimmed').addClass('focus-neighbor');
        targetNodes.removeClass('dimmed').addClass('focus-neighbor');

        // 4. Color edges by direction
        incomingEdges.removeClass('dimmed').addClass('focus-edge-in');
        outgoingEdges.removeClass('dimmed').addClass('focus-edge-out');

        // 5. Undim parent nodes of visible children (namespace group boundaries)
        const visibleSet = node.union(sourceNodes).union(targetNodes);
        visibleSet.parents().removeClass('dimmed');
      });

      focusActive = true;
    }

    export function initFocusMode(cy) {
      // Click on node (no modifiers) → activate/switch focus
      cy.on('tap', 'node', (evt) => {
        const oe = evt.originalEvent;
        if (!oe) return;
        if (oe.ctrlKey || oe.metaKey || oe.shiftKey || oe.altKey) return;

        const node = evt.target;
        // Skip parent/group nodes — Phase 4 handles collapsed namespace focus
        if (node.isParent()) return;
        applyFocus(cy, node);
      });

      // Click background (no modifiers) → clear focus
      cy.on('tap', (evt) => {
        if (evt.target !== cy) return;
        const oe = evt.originalEvent;
        if (oe && (oe.ctrlKey || oe.metaKey)) return;
        clearFocus(cy);
      });

      // Auto-clear focus when multi-select is activated (Ctrl+Click or box-select).
      // Listens for Cytoscape's built-in 'select' event — no need to modify selection.js.
      cy.on('select', 'node', () => {
        clearFocus(cy);
      });
    }
    ```

- [x] **2.2 Handle interaction with selection module**
  - **Dependencies**: 2.1
  - **Description**: Ensure focus and multi-select modes are mutually exclusive. This is already handled in 2.1 via two mechanisms: (a) `applyFocus` calls `clearSelection(cy)` before applying focus, (b) `cy.on('select', 'node')` in `initFocusMode` auto-clears focus when any node is selected. **No changes to `selection.js` needed** — focus.js has a one-way dependency on selection.js (imports `clearSelection`), avoiding circular imports. Verify: Ctrl+Click clears focus → selection activates. Plain click clears selection → focus activates. Box-select (Ctrl+Drag) also triggers 'select' event → focus clears.
  - **Modifies**:
    - `frontend/src/focus.js` — already done in 2.1 (verify only)
  - **Note**: `sidebar.js` also registers `cy.on('tap', 'node')` (line 137) with the same Ctrl/Meta guard. Focus and sidebar fire concurrently on plain click — this is by design (sidebar shows node details while focus highlights connections). Phase 3 will need to add a `shiftKey` guard to sidebar.js's tap handler to prevent sidebar toggle on Shift+Click downstream focus.

- [x] **2.3 Handle edge labels in focus mode**
  - **Dependencies**: 2.1
  - **Description**: The `.dimmed` style on edges sets `text-opacity: 0` (from Phase 1 styles). The `.focus-edge-in`, `.focus-edge-out`, and `.focus-traversal` classes already set `text-opacity: 1` explicitly (added in Phase 1 v1.1.0). Verify that when edge labels are globally enabled, focused edges show labels while dimmed edges hide them. No additional code expected — just visual verification in the test environment.
  - **Modifies**:
    - `frontend/src/graph.js` — adjust styles if needed

### ✅ Критерии завершения Phase 2

- [x] All subtasks completed (2.1, 2.2, 2.3)
- [x] Click on node → graph enters focus mode with direction-colored edges
- [x] Click on another node → focus switches immediately (**no stale highlight on previous node**)
- [x] Click on background → focus clears, all elements back to normal
- [x] Ctrl+Click still works for multi-select (focus auto-clears via 'select' event)
- [x] Click on expanded namespace group (parent node) → no focus triggered
- [x] With namespace grouping: focused node's namespace boundary remains visible (not dimmed)
- [x] Edge labels visible only on focused edges when labels enabled
- [x] Sidebar opens concurrently with focus (by design) — verify both work together
- [x] Performance acceptable on test environment graphs

---

## Phase 3: Downstream and upstream focus

**Dependencies**: Phase 2
**Status**: Done

### Описание

Add Shift+Click for downstream traversal (full BFS following outgoing edges) and Shift+Alt+Click for upstream traversal (full BFS following incoming edges). Downstream/upstream edges keep their state colors (ok/degraded/down) instead of direction coloring.

### Подпункты

- [x] **3.1 Implement downstream focus (Shift+Click)**
  - **Dependencies**: None
  - **Description**: In `focus.js`, add Shift+Click handler. Use `getDownstreamNodes(node, true)` from `graph-utils.js` for full traversal via `successors('node')`. Collect all edges between downstream nodes using `edgesWith()`. **Important**: use `successors('node')` (nodes only) + `edgesWith` (all internal edges) — do NOT use plain `successors()` because it only returns BFS-tree edges and misses backward/cross edges in cycles. **Must clear old focus classes before applying** (same pattern as `applyFocus` from Phase 2). Undim parent nodes of visible children. Apply `.focused` to clicked node, `.focus-neighbor` to downstream nodes, `.focus-traversal` to edges between them (preserves state colors via class design). **No conflict** with Ctrl+Shift+Drag (downstream drag in `node-drag.js`) — Cytoscape does not fire 'tap' on drag, and Ctrl modifier causes early return.
  - **Modifies**:
    - `frontend/src/focus.js`
  - **Code examples**:

    ```javascript
    function applyDownstreamFocus(cy, node) {
      const downstream = getDownstreamNodes(node, true);
      const allFocused = downstream.union(node);
      // edgesWith(same collection) returns edges where BOTH endpoints are in allFocused.
      // No additional .filter() needed — edgesWith already guarantees this.
      // Catches ALL internal edges including back-edges in cycles (unlike successors()
      // which only returns BFS-tree edges).
      const focusedEdges = allFocused.edgesWith(allFocused);

      clearSelection(cy);

      cy.batch(() => {
        // Clear old focus classes + dim everything (prevents stale classes)
        cy.elements().removeClass(ALL_CLASSES_STR).addClass('dimmed');
        node.removeClass('dimmed').addClass('focused');
        downstream.removeClass('dimmed').addClass('focus-neighbor');
        focusedEdges.removeClass('dimmed').addClass('focus-traversal');
        // Undim parent nodes of visible children
        allFocused.parents().removeClass('dimmed');
      });

      focusActive = true;
    }
    ```

- [x] **3.2 Implement upstream focus (Shift+Alt+Click)**
  - **Dependencies**: 3.1
  - **Description**: Symmetric to 3.1 — use `getUpstreamNodes(node, true)` via `predecessors('node')`. Same edge collection pattern with `edgesWith()`. Same class cleanup, parent undim, and selection clearing.
  - **Modifies**:
    - `frontend/src/focus.js`
  - **Code examples**:

    ```javascript
    function applyUpstreamFocus(cy, node) {
      const upstream = getUpstreamNodes(node, true);
      const allFocused = upstream.union(node);
      const focusedEdges = allFocused.edgesWith(allFocused);

      clearSelection(cy);

      cy.batch(() => {
        cy.elements().removeClass(ALL_CLASSES_STR).addClass('dimmed');
        node.removeClass('dimmed').addClass('focused');
        upstream.removeClass('dimmed').addClass('focus-neighbor');
        focusedEdges.removeClass('dimmed').addClass('focus-traversal');
        allFocused.parents().removeClass('dimmed');
      });

      focusActive = true;
    }
    ```

- [x] **3.3 Update tap handler to route modifiers**
  - **Dependencies**: 3.1, 3.2
  - **Description**: Refactor the node tap handler in `focus.js` to check modifier keys and route to the correct function: no modifiers → `applyFocus`, Shift only → `applyDownstreamFocus`, Shift+Alt → `applyUpstreamFocus`. Ctrl/Meta → skip (handled by selection.js). Keep `isParent()` guard before routing. **Also update `sidebar.js`**: add `shiftKey` guard to node tap handler (line 137) to prevent sidebar toggle on Shift+Click / Shift+Alt+Click — these are now focus-mode interactions, not sidebar interactions.
  - **Modifies**:
    - `frontend/src/focus.js`
    - `frontend/src/sidebar.js` — add `if (oe.shiftKey) return;` guard
  - **Code examples**:

    ```javascript
    // focus.js — updated tap handler
    cy.on('tap', 'node', (evt) => {
      const oe = evt.originalEvent;
      if (!oe) return;
      // Ctrl/Meta → multi-select (handled by selection.js)
      if (oe.ctrlKey || oe.metaKey) return;

      const node = evt.target;
      // Skip parent/group nodes — Phase 4 handles collapsed namespace focus
      if (node.isParent()) return;

      if (oe.shiftKey && oe.altKey) {
        applyUpstreamFocus(cy, node);
      } else if (oe.shiftKey) {
        applyDownstreamFocus(cy, node);
      } else {
        applyFocus(cy, node);
      }
    });
    ```

    ```javascript
    // sidebar.js — add shiftKey guard (line ~140)
    cy.on('tap', 'node', (evt) => {
      const oe = evt.originalEvent;
      if (oe && (oe.ctrlKey || oe.metaKey)) return;
      if (oe && oe.shiftKey) return; // Shift+Click = focus traversal, not sidebar
      // ... existing sidebar logic ...
    });
    ```

### ✅ Критерии завершения Phase 3

- [x] All subtasks completed (3.1, 3.2, 3.3)
- [x] Shift+Click → full downstream chain highlighted with state-colored edges
- [x] Shift+Alt+Click → full upstream chain highlighted
- [x] Shift+Click does NOT open sidebar (sidebar.js guard)
- [x] Background click clears any focus mode variant
- [x] BFS correctly handles circular dependencies (no infinite loops — `successors()`/`predecessors()` handle cycles)
- [x] Clicking another node while in downstream mode switches correctly

---

## Phase 4: Edge cases and integration

**Dependencies**: Phase 3
**Status**: Done

### Описание

Handle collapsed namespaces, filter/search interaction, and ensure correct behavior when graph data is updated (polling).

### Подпункты

- [x] **4.1 Collapsed namespace focus (verification only)**
  - **Dependencies**: None
  - **Description**: Verify that clicking a collapsed namespace node activates focus correctly — **no code changes expected**. This works because: (1) after collapse, children are removed from the graph → `isParent()` returns false → the Phase 2 tap handler processes the click instead of skipping it; (2) the collapsed node retains aggregated edges created by `grouping.js` → `getConnectedElements()` returns those aggregated edges and their external endpoints; (3) downstream/upstream traversal (`successors`/`predecessors`) follows aggregated edges for namespace-level focus view. Test scenarios: click collapsed namespace → 1-hop external connections highlighted; Shift+Click → downstream from collapsed namespace highlighted.
  - **Modifies**: None (verification task)
  - **Why it works without changes**: `collapseNamespace()` in `grouping.js` removes children with `children.remove()` (line 239) and marks parent with `isCollapsed: true` (line 242). Without children, Cytoscape `isParent()` returns false, so the tap handler's `isParent()` guard passes.

- [x] **4.2 Filter and search interaction (verification + minor optimization)**
  - **Dependencies**: None (parallel with 4.1)
  - **Description**: Verify that focus mode works correctly with active filters — **no changes to `graph-utils.js` needed**. This works because: (1) **Filter `.hide()/.show()`**: Cytoscape hidden elements (`display: none`) are invisible regardless of focus classes — `.dimmed` or `.focus-neighbor` on a hidden element has no visual effect; (2) **Search opacity bypass**: `search.js` uses Cytoscape bypass styles (`node.style('opacity', 0.15)`) which have HIGHER priority than stylesheet classes (`.dimmed` opacity 0.2, `.focus-neighbor` opacity 1). Search-dimmed elements stay at 0.15 even with focus classes applied — correct behavior (search filter takes priority over focus). **Optional optimization**: in `applyFocus`/`applyDownstreamFocus`/`applyUpstreamFocus`, use `cy.elements(':visible')` as base collection for dimming instead of `cy.elements()` to skip hidden elements (minor performance gain, not a correctness fix). **Edge case**: clicking a search-dimmed node (opacity 0.15) activates focus — border/overlay are visible but node stays faint. Acceptable since these nodes are barely clickable.
  - **Modifies**:
    - `frontend/src/focus.js` — optional: `cy.elements(':visible')` optimization

- [x] **4.3 Clear focus on structure change only**
  - **Dependencies**: 2.1
  - **Description**: Clear focus mode only when graph structure changes (nodes/edges added or removed) — **NOT on every 15s poll**. Clearing focus every poll cycle is bad UX: the user explicitly activated focus and expects it to persist while exploring the topology. When structure doesn't change, `renderGraph` only updates data attributes (state, latency) → focus classes are unaffected. When structure changes, `renderGraph` removes and re-adds all elements → old focus classes are gone, but `focusActive` flag remains stale. Call `clearFocus(cy)` after `renderGraph` only when `structureChanged` is true — this resets the `focusActive` flag (the `removeClass` call is a harmless no-op on fresh elements).
  - **Modifies**:
    - `frontend/src/main.js` — add conditional `clearFocus(cy)` call after renderGraph
  - **Code examples**:

    ```javascript
    // In refresh() in main.js — clear focus ONLY on structure change
    const structureChanged = renderGraph(cy, data, appConfig);
    if (structureChanged) {
      clearFocus(cy); // Resets focusActive flag; removeClass is no-op on new elements
    }
    ```

- [x] **4.4 Integration with context menu, tooltips, and collapse/expand**
  - **Dependencies**: 2.1
  - **Description**: **Context menu**: uses `cxttap` event (not `tap`) → no conflict, verified by code. **Tooltips**: hover events are independent of focus classes → tooltips appear on both focused and dimmed elements (opacity > 0 still receives pointer events). **Collapse/expand (dbltap)**: REQUIRES a fix. When double-clicking a collapsed node, the first `tap` activates focus (`.dimmed` applied to everything), then `dbltap` fires and expands the namespace — newly restored children have no classes while the rest of the graph is dimmed. Visual inconsistency. Fix: add `clearFocus(cy)` at the start of the `dbltap` handler in `setupGroupingHandlers()` (main.js), NOT in `focus.js`. Grouping state change clears focus — this is a grouping concern, not a focus concern.
  - **Modifies**:
    - `frontend/src/main.js` — add `clearFocus(cy)` in `setupGroupingHandlers()` dbltap handler
  - **Code examples**:

    ```javascript
    // In setupGroupingHandlers() in main.js
    cy.on('dbltap', 'node[?isGroup]', (evt) => {
      if (!isGroupingEnabled()) return;
      clearFocus(cy); // Clear focus before collapse/expand to avoid visual inconsistency
      const node = evt.target;
      if (node.data('isCollapsed')) {
        const nsName = node.id().replace(NS_PREFIX, '');
        expandNamespace(cy, nsName);
      } else {
        const nsName = node.data('label');
        collapseNamespace(cy, nsName);
      }
    });
    ```

### ✅ Критерии завершения Phase 4

- [x] All subtasks completed (4.1, 4.2, 4.3, 4.4)
- [x] Clicking collapsed namespace highlights its aggregated external connections
- [x] Shift+Click on collapsed namespace shows downstream namespace-level view
- [x] Focus persists across data polls (only clears on structure change)
- [x] Filter-hidden nodes stay invisible with focus active (`.hide()` overrides classes)
- [x] Search-dimmed nodes stay faint with focus active (bypass overrides classes)
- [x] Double-click collapse/expand clears focus before restructuring
- [x] Context menu opens normally during focus mode
- [x] Tooltips appear on both focused and dimmed elements
- [x] No regressions in existing functionality

---

## Phase 5: Build, test, docs

**Dependencies**: Phase 4
**Status**: Done

### Описание

Build the updated frontend, test in the Kubernetes environment, update documentation.

### Подпункты

- [x] **5.1 Build and deploy to test environment**
  - **Dependencies**: None
  - **Description**: Build the dev image with `make docker-dev`, deploy to the test Kubernetes cluster. Verify focus mode on a real topology with multiple services.
  - **Creates**:
    - Docker dev image

- [x] **5.2 Manual testing checklist**
  - **Dependencies**: 5.1
  - **Description**: Verify all user stories:
    - US-1: Click service → 1-hop focus, blue incoming / purple outgoing
    - US-2: Click another service → focus switches
    - US-3: Shift+Click → full downstream highlighted
    - US-4: Shift+Alt+Click → full upstream highlighted
    - US-5: Click background → focus clears
    - US-6: Click collapsed namespace → external connections highlighted
    - Verify Ctrl+Click multi-select still works
    - Verify Ctrl+Drag group drag still works
    - Verify dark/light theme switching in focus mode
    - Verify with edge labels on/off
    - Verify with active filters
    - Verify performance with full test topology

- [x] **5.3 Update documentation**
  - **Dependencies**: 5.2
  - **Description**: Update `docs/graph-interactions.md` with focus mode section. Add keyboard shortcuts table entry. Update CHANGELOG.md with the new feature.
  - **Modifies**:
    - `docs/graph-interactions.md`
    - `CHANGELOG.md`

### ✅ Критерии завершения Phase 5

- [x] All subtasks completed (5.1, 5.2, 5.3)
- [x] Dev image builds successfully
- [x] All manual test cases pass
- [x] Documentation updated
- [x] No regressions

---

## 📝 Примечания

- **Performance**: Using Cytoscape classes with `cy.batch()` is the most performant approach — avoids per-element style recalculation. Classes are applied/removed in O(n) time where n is the number of elements.
- **Graph traversal**: Use `successors('node')` / `predecessors('node')` for full BFS traversal (project convention from `cytoscape-patterns` memory). Manual BFS only when edge-level filtering is needed (e.g., `cascade.js` filters by `edge.data('critical')`).
- **State colors in downstream**: Downstream/upstream edges use `.focus-traversal` class which only sets opacity, text-opacity, and width, preserving the state-based `line-color` from existing styles.
- **Direction colors in 1-hop**: `.focus-edge-in` (blue #42a5f5) and `.focus-edge-out` (purple #ab47bc) override `line-color` — this is intentional for 1-hop mode only.
- **Dimmed opacity**: `.dimmed` uses opacity 0.2, search-hidden elements use 0.15. Different values prevent visual confusion when both modes are active simultaneously.
- **Edge label visibility**: All edge focus classes (`.focus-edge-in`, `.focus-edge-out`, `.focus-traversal`) explicitly set `text-opacity: 1` to override `edge.dimmed`'s `text-opacity: 0`. Without explicit `text-opacity`, class specificity would not guarantee label restoration.
- **Existing keyboard shortcuts**: Ctrl+Click = multi-select, Ctrl+Drag = group drag, Ctrl+Shift+Drag = full downstream drag. Focus mode uses: Click = 1-hop focus, Shift+Click = downstream focus, Shift+Alt+Click = upstream focus. No conflicts.
- **Cytoscape class precedence**: Later rules in the styles array take precedence. Focus styles must be placed after `:selected` styles in the array to ensure correct layering.
- **Event handler coexistence**: Both `focus.js` and `selection.js` register `cy.on('tap')` background handlers. All Cytoscape handlers fire (no stopPropagation). This is by design — Phase 2 uses `cy.on('select')` for auto-clearing focus without modifying selection.js.
- **Focus-switch safety**: `applyFocus` / `applyDownstreamFocus` / `applyUpstreamFocus` must always clear ALL old focus classes before applying new ones: `cy.elements().removeClass(ALL_CLASSES_STR).addClass('dimmed')`. Without this, stale `.focused` / `.focus-neighbor` / `.focus-edge-*` classes persist on previously focused elements, causing visual artifacts (dimmed opacity + focus border).
- **Parent node handling**: Compound parent nodes (namespace groups) are skipped by the tap handler (`isParent()` guard). When focus is applied, parent nodes of focused/neighbor children are undimmed to preserve namespace boundary visibility.
- **Sidebar coexistence**: `sidebar.js` registers `cy.on('tap', 'node')` with same Ctrl/Meta guard. Plain click triggers both focus and sidebar — by design. Shift+Click (Phase 3) requires a `shiftKey` guard in sidebar.js to prevent sidebar toggle.
- **Dependency direction**: `focus.js → selection.js` (imports `clearSelection`). `selection.js` has zero knowledge of `focus.js` (no circular dependency). Focus auto-clears on node selection via `cy.on('select')` event.
- **Edge collection for traversal focus**: Use `successors('node')` / `predecessors('node')` for node collection, then `allFocused.edgesWith(allFocused)` for edge collection. Do NOT use plain `successors()` / `predecessors()` (without 'node' filter) for edge collection — BFS builtins return only tree edges traversed during BFS, missing backward/cross edges in cycles. `edgesWith(same)` returns ALL edges where both endpoints are in the set — correct for visualization. No additional `.filter()` needed after `edgesWith` with identical collections.
- **Modifier key isolation**: Shift+Click (downstream focus) does NOT conflict with Ctrl+Shift+Drag (downstream drag in `node-drag.js`). Cytoscape fires 'tap' for clicks and 'grab'/'drag'/'free' for drags — never both. Ctrl+Shift+Click → focus handler returns early on ctrlKey check.
- **Collapsed namespace internals**: After `collapseNamespace()`, children are removed → parent's `isParent()` returns false → tap handler processes it (no `isParent()` guard block). Aggregated edges provide the connectivity for `getConnectedElements` / `successors` / `predecessors`.
- **Filter/search style priority**: Cytoscape bypass styles (set via `node.style()`) have HIGHER priority than stylesheet classes. Search module uses bypass (`opacity: 0.15`), focus module uses classes (`.dimmed` opacity 0.2). Bypass always wins → no conflict. Similarly, `.hide()/.show()` uses `display: none` → completely overrides opacity classes.
- **Focus persistence across polls**: Focus classes persist when `renderGraph` updates only data attributes (no structure change). Only cleared on structure change (elements removed + re-added). Clearing on every 15s poll is bad UX — user explicitly activated focus.
- **Double-click race**: First tap of double-click activates focus, dbltap immediately clears it. This is a 2-frame visual flash — imperceptible to users. The dbltap handler in `setupGroupingHandlers` calls `clearFocus` before collapse/expand.

---

**🎯 Plan ready for implementation.**
