# Design: Graph Layout Optimization — ELK + Position Persistence

## Metadata

- **Version**: 1.1.0
- **Created**: 2026-03-03
- **Last updated**: 2026-03-03
- **Status**: Draft
- **Requirements**: [plans/ui-graph-layout-optimization.md](ui-graph-layout-optimization.md)

---

## Table of Contents

- [1. Architecture Overview](#1-architecture-overview)
- [2. New Module: layout-store.js](#2-new-module-layout-storejs)
- [3. ELK Layout Configuration](#3-elk-layout-configuration)
- [4. Changes to Existing Modules](#4-changes-to-existing-modules)
- [5. Data Flow](#5-data-flow)
- [6. Position Persistence Schema](#6-position-persistence-schema)
- [7. Integration with Existing Features](#7-integration-with-existing-features)
- [8. UI Changes](#8-ui-changes)
- [9. Dependencies](#9-dependencies)
- [10. Implementation Phases](#10-implementation-phases)
- [11. Edge Cases and Risks](#11-edge-cases-and-risks)

---

## 1. Architecture Overview

### Current Layout Architecture

```
buildLayoutConfig()
    ├── isGroupingEnabled() === true  → fCoSE (force-directed, compound)
    └── isGroupingEnabled() === false → Dagre (hierarchical, flat only)
```

**Problem**: fCoSE is a force-directed algorithm — it does not produce layered/hierarchical output.
Dagre works well for flat graphs but is not used when grouping is enabled.

### Target Layout Architecture

```
renderGraph() — structure changed:
    ├── hasSavedPositions() === true
    │   ├─ Step 1: preset layout (restore saved coords)
    │   ├─ hasNewNodes()?
    │   │   ├─ YES → Step 2: ELK for new nodes subgraph only
    │   │   └─ NO  → done (all positioned)
    │   └─ saveAutoPositions()
    └── hasSavedPositions() === false
        ├─ ELK layered (full layout)
        │   ├── compound nodes → elk.hierarchyHandling: INCLUDE_CHILDREN
        │   └── flat graph     → standard layered
        └─ saveAutoPositions()
```

**Solution**: Single layout engine (ELK `layered`) for all modes.
Position persistence layer sits between Cytoscape and localStorage.

**Key principle**: ELK never runs on nodes with saved positions.
Instead, a two-step approach is used: (1) restore saved positions via `preset`,
(2) run ELK only for the subgraph of unpositioned nodes.

### Module Dependency Diagram

```
                    ┌──────────────┐
                    │   main.js    │
                    │  (init,      │
                    │   toolbar,   │
                    │   polling)   │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
    ┌─────────▼──┐  ┌──────▼──────┐  ┌──▼───────────┐
    │  graph.js  │  │ node-drag.js│  │ grouping.js  │
    │ (render,   │  │ (drag       │  │ (compound,   │
    │  layout,   │  │  events)    │  │  collapse)   │
    │  styles)   │  └──────┬──────┘  └──────┬───────┘
    └─────┬──────┘         │                │
          │          ┌─────▼────────────────▼──┐
          ├──────────► layout-store.js [NEW]    │
          │          │ (position save/restore,  │
          │          │  manual position flags,  │
          │          │  localStorage I/O)       │
          └──────────┴─────────────────────────┘
```

---

## 2. New Module: layout-store.js

A dedicated module for position persistence. Keeps layout-store concerns
separated from rendering (graph.js) and drag (node-drag.js).

### Public API

```javascript
// frontend/src/layout-store.js

const STORAGE_KEY = 'dephealth-node-positions';

/**
 * Get all saved positions from localStorage.
 * @returns {Object<string, {x: number, y: number, manual: boolean}>}
 */
export function getSavedPositions() { ... }

/**
 * Mark a specific node as manually positioned and save its position.
 * Called on drag end. Skips save if position unchanged (no-op drag).
 * @param {string} nodeId
 * @param {{x: number, y: number}} position
 */
export function markManualPosition(nodeId, position) { ... }

/**
 * Mark multiple nodes as manually positioned (group drag).
 * @param {Array<{id: string, x: number, y: number}>} nodes
 */
export function markManualPositions(nodes) { ... }

/**
 * Clear all saved positions (reset layout).
 */
export function clearSavedPositions() { ... }

/**
 * Clear only the manual flag on all saved positions.
 * Called on direction change (TB→LR) — manual positions become auto,
 * so ELK can recalculate everything for the new direction.
 */
export function clearManualFlags() { ... }

/**
 * Remove positions for nodes that no longer exist in topology.
 * @param {Set<string>} currentNodeIds - IDs of nodes in current data
 */
export function pruneStalePositions(currentNodeIds) { ... }

/**
 * Apply saved positions to Cytoscape nodes (preset mode).
 * Returns set of node IDs that have no saved position (need layout).
 * @param {cytoscape.Core} cy
 * @returns {Set<string>} nodeIds without saved positions
 */
export function applySavedPositions(cy) { ... }

/**
 * Check if any saved positions exist.
 * @returns {boolean}
 */
export function hasSavedPositions() { ... }

/**
 * Save all current positions as auto-positioned (after ELK layout).
 * Does NOT overwrite existing manual=true positions.
 * @param {cytoscape.Core} cy
 */
export function saveAutoPositions(cy) { ... }
```

### Internal Implementation Notes

- localStorage value: JSON string of position map
- Read on startup: parse once, keep in-memory cache
- Write: debounced (300ms) to avoid excessive I/O during drag
- Manual flag: preserved across auto-layout runs, cleared on direction change
- Parent/compound nodes: positions NOT saved (they auto-size around children)
- No-op drag guard: `markManualPosition` compares current vs saved coords,
  skips save if delta < 1px (prevents marking nodes as manual on click-release)

---

## 3. ELK Layout Configuration

### Dependencies

```json
{
  "dependencies": {
    "cytoscape-elk": "^2.2.0",
    "elkjs": "^0.9.3"
  }
}
```

Note: `elkjs` is a peer dependency of `cytoscape-elk`.
The default import uses the JS-compiled variant (`elk.bundled.js`, ~1.2MB unminified,
~200KB gzipped). A web-worker variant is also available for non-blocking computation
but is not needed for graphs <50 nodes.

### Plugin Registration (graph.js)

```javascript
import cytoscape from 'cytoscape';
import elk from 'cytoscape-elk';
// Remove: import dagre from 'cytoscape-dagre';
// Remove: import fcose from 'cytoscape-fcose';

cytoscape.use(elk);
// Keep: cytoscape.use(cytoscapeSvg);
```

Decision: **remove Dagre and fCoSE** entirely. ELK `layered` covers both flat
and compound graph layouts. This eliminates the dual-algorithm complexity.

### ELK Layout Options

#### Full Layout (no saved positions)

```javascript
function buildElkLayoutConfig({ animate = false, animationDuration = 500 } = {}) {
  const direction = layoutDirection === 'LR' ? 'RIGHT' : 'DOWN';

  return {
    name: 'elk',
    nodeDimensionsIncludeLabels: true,
    fit: true,
    padding: 50,
    animate,
    animationDuration,

    // Per-node ELK options: pin entry points to first layer
    nodeLayoutOptions: (node) => {
      if (node.isParent()) return {}; // skip compound parents
      if (node.data('isEntry') || node.indegree(false) === 0) {
        return { 'elk.layered.layering.layerConstraint': 'FIRST' };
      }
      return {};
    },

    elk: {
      algorithm: 'layered',
      'elk.direction': direction,

      // Hierarchy: layout parent and children together
      'elk.hierarchyHandling': 'INCLUDE_CHILDREN',

      // Layer assignment: prioritize longest path (entry → deps)
      'elk.layered.layering.strategy': 'LONGEST_PATH',

      // Crossing minimization for cleaner edges
      'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',

      // Node placement: optimize for balanced vertical distribution
      'elk.layered.nodePlacement.strategy': 'BRANDES_KOEPF',

      // Spacing
      'elk.layered.spacing.nodeNodeBetweenLayers': 100,
      'elk.layered.spacing.edgeNodeBetweenLayers': 30,
      'elk.spacing.nodeNode': 60,
      'elk.padding': '[left=20, top=30, right=20, bottom=20]',

      // Edge routing: smooth polylines (not strict right-angle)
      'elk.layered.edgeRouting': 'POLYLINE',
    },
  };
}
```

#### Key ELK Options Explained

| Option | Value | Why |
|--------|-------|-----|
| `algorithm` | `layered` | Hierarchical DAG layout (Sugiyama-style) |
| `elk.direction` | `DOWN` / `RIGHT` | Maps to TB / LR direction toggle |
| `elk.hierarchyHandling` | `INCLUDE_CHILDREN` | Lays out compound children within parent bounds |
| `elk.layered.layering.strategy` | `LONGEST_PATH` | Entry points (sources) naturally go to top layer |
| `elk.layered.crossingMinimization.strategy` | `LAYER_SWEEP` | Minimizes edge crossings between layers |
| `elk.layered.nodePlacement.strategy` | `BRANDES_KOEPF` | Balanced node placement within each layer |
| `elk.layered.edgeRouting` | `POLYLINE` | Smooth polyline edges (not strict right-angle) |
| `elk.spacing.nodeNode` | `60` | Horizontal spacing between siblings |
| `elk.layered.spacing.nodeNodeBetweenLayers` | `100` | Vertical spacing between layers |
| `nodeLayoutOptions` | per-node function | Pins `isEntry` and source nodes to FIRST layer |

#### Entry Point Handling

Two mechanisms work together to place entry points on the top layer:

1. **ELK `LONGEST_PATH` layering**: automatically places source nodes
   (no incoming edges) on the first layer for DAGs.
2. **`nodeLayoutOptions` with `layerConstraint: FIRST`**: explicitly pins
   nodes with `isEntry=true` or zero indegree to the first layer.
   This handles cases where `isEntry` nodes DO have incoming edges
   (e.g., health-check loops) but should still be at the top.

The `nodeLayoutOptions` callback filters out parent/compound nodes
(they have indegree 0 but should not be pinned).

---

## 4. Changes to Existing Modules

### 4.1 graph.js

#### Remove

- `import dagre from 'cytoscape-dagre'` and `cytoscape.use(dagre)`
- `import fcose from 'cytoscape-fcose'` and `cytoscape.use(fcose)`
- Old `buildLayoutConfig()` function (lines 553-573)

#### Add

- `import elk from 'cytoscape-elk'` and `cytoscape.use(elk)`
- `import { getSavedPositions, applySavedPositions, saveAutoPositions, pruneStalePositions, hasSavedPositions, clearSavedPositions, clearManualFlags } from './layout-store.js'`
- New `buildElkLayoutConfig()` function (see Section 3)
- New `runIncrementalElk(cy, unpositionedIds)` helper (see Section 4.1 renderGraph)
- New `export function getLayoutDirection()` getter (returns `layoutDirection`)

#### Modify: `renderGraph()` (lines 636-757)

Current flow (structure changed path):
```
1. cy.batch(() => { remove all, add parents, add nodes, add edges })
2. cy.layout(buildLayoutConfig()).run()
3. if first render: cy.fit(50)
```

New flow (structure changed path):
```
1. cy.batch(() => { remove all, add parents, add nodes, add edges })
2. pruneStalePositions(currentNodeIds)
3. if (hasSavedPositions()):
   a. unpositionedIds = applySavedPositions(cy)  // preset: set saved coords
   b. if (unpositionedIds.size > 0):
      - Run ELK on unpositioned subgraph only (see runIncrementalElk below)
      - saveAutoPositions(cy)
   c. else:
      - All nodes positioned from saved data, no ELK needed
4. else:
   a. Run full ELK layout
   b. saveAutoPositions(cy)
5. cy.fit(50)  // always fit after positioning (first render or not)
```

**Important**: ELK never recalculates already-positioned nodes. The two-step
approach (preset + incremental ELK) avoids the `node.lock()` issue where
cytoscape-elk ignores Cytoscape lock state.

**`runIncrementalElk(cy, unpositionedIds)`** — helper function:
```javascript
function runIncrementalElk(cy, unpositionedIds) {
  // Position unpositioned nodes near their graph neighbors as starting hint
  for (const id of unpositionedIds) {
    const node = cy.getElementById(id);
    const neighbors = node.neighborhood('node');
    if (neighbors.length > 0) {
      const avgX = neighbors.reduce((s, n) => s + n.position('x'), 0) / neighbors.length;
      const avgY = neighbors.reduce((s, n) => s + n.position('y'), 0) / neighbors.length;
      node.position({ x: avgX + 50, y: avgY + 50 });
    }
  }
  // Then run ELK only for unpositioned nodes:
  // Use cy.layout() with `fit: false` to avoid viewport jump
  const config = buildElkLayoutConfig({ animate: false });
  config.fit = false;
  // Use transform to only apply ELK positions for unpositioned nodes
  config.transform = (node, pos) => {
    return unpositionedIds.has(node.id()) ? pos : node.position();
  };
  cy.layout(config).run();
}
```

#### Modify: `relayout()` (lines 782-786)

Current:
```javascript
export function relayout(cy, direction = 'TB') {
  layoutDirection = direction;
  cy.layout(buildLayoutConfig({ animate: true, animationDuration: 500 })).run();
}
```

New — direction change clears manual flags, then full ELK:
```javascript
export function relayout(cy, direction = 'TB') {
  layoutDirection = direction;

  // Direction change invalidates manual positions (TB coords ≠ LR coords)
  clearManualFlags();

  const layout = cy.layout(buildElkLayoutConfig({
    animate: true,
    animationDuration: 500,
  }));

  layout.on('layoutstop', () => {
    saveAutoPositions(cy);
  });

  layout.run();
}
```

Note: `clearManualFlags()` does NOT delete positions — it only sets `manual: false`.
After ELK finishes, `saveAutoPositions()` overwrites all positions with the new layout.

#### New: `resetLayout()` export

```javascript
export function resetLayout(cy) {
  clearSavedPositions();
  const layout = cy.layout(buildElkLayoutConfig({ animate: true, animationDuration: 500 }));
  layout.on('layoutstop', () => saveAutoPositions(cy));
  layout.run();
}
```

### 4.2 node-drag.js

#### Current `free` Event (end of drag)

```javascript
cy.on('free', 'node', () => {
  companions.clear();
  grabbedStartPos = null;
});
```

#### New `free` Event — save manual positions

```javascript
import { markManualPosition, markManualPositions } from './layout-store.js';

cy.on('free', 'node', (evt) => {
  const node = evt.target;

  // Skip compound parent nodes — their position is derived from children
  if (node.isParent()) {
    companions.clear();
    grabbedStartPos = null;
    return;
  }

  // No-op drag guard: skip if node barely moved (click-release, not actual drag)
  const endPos = node.position();
  const movedEnough = grabbedStartPos &&
    (Math.abs(endPos.x - grabbedStartPos.x) > 1 ||
     Math.abs(endPos.y - grabbedStartPos.y) > 1);

  if (movedEnough) {
    // Save grabbed node position as manual
    markManualPosition(node.id(), endPos);

    // Save companion positions as manual
    if (companions.size > 0) {
      const batch = [];
      for (const [id] of companions) {
        const n = cy.getElementById(id);
        if (n.length && !n.isParent()) {
          batch.push({ id, ...n.position() });
        }
      }
      if (batch.length > 0) {
        markManualPositions(batch);
      }
    }
  }

  companions.clear();
  grabbedStartPos = null;
});
```

The 1px threshold prevents click-release (without actual movement) from
marking a node as manually positioned, which would lock it during future
layout recalculations.

### 4.3 main.js

#### Add Reset Layout Button Handler

In `setupGraphToolbar()` (after layout toggle setup, ~line 444):

```javascript
const btnResetLayout = $('#btn-reset-layout');
btnResetLayout.addEventListener('click', () => {
  resetLayout(cy);
});
```

#### Modify Layout Direction Toggle

The existing toggle calls `relayout(cy, layoutDirection)` which now clears
manual flags and runs full ELK internally. **No changes needed** to the toggle
handler — the updated `relayout()` in graph.js handles direction change logic.

#### Modify Grouping Toggle Visibility

Currently, the layout direction button is hidden when grouping is ON
(because Dagre was only used for flat graphs). With ELK supporting both modes,
the direction toggle should be visible in all modes:

```javascript
// Remove these lines:
// btnLayoutToggle.classList.toggle('hidden', next);

// The direction toggle is always visible now.
```

### 4.4 grouping.js

#### Modify: `expandNamespace()` relayout (lines 378-387)

Current — hardcoded fCoSE:
```javascript
cy.layout({
  name: 'fcose',
  animate: true,
  animationDuration: 400,
  quality: 'default',
  nodeSeparation: 80,
  idealEdgeLength: 120,
  nodeRepulsion: 6000,
  tile: true,
}).run();
```

New — use shared ELK layout:
```javascript
import { relayout, getLayoutDirection } from './graph.js';

// After batch restore of children:
relayout(cy, getLayoutDirection());
```

This reuses the same ELK layout config. Note: `getLayoutDirection()` is a new
getter export from graph.js (since `layoutDirection` is a module-local variable).

On expand, `relayout()` clears manual flags and runs full ELK — this is
acceptable because expand changes the graph structure significantly.
Alternatively, we could use the two-step preset+incremental approach if
preserving manual positions after expand is desired (future enhancement).

---

## 5. Data Flow

### 5.1 Startup Flow

```
init()
  │
  ├─ initGraph(container) → cy (preset layout, no positions)
  │
  ├─ fetchTopology() → data
  │
  ├─ renderGraph(cy, data)
  │   ├─ cy.batch() → add all elements
  │   ├─ pruneStalePositions(nodeIds)
  │   ├─ hasSavedPositions()?
  │   │   ├─ YES: applySavedPositions(cy) → set saved coords on nodes
  │   │   │       ├─ unpositionedIds.size > 0?
  │   │   │       │   └─ runIncrementalElk(cy, unpositionedIds)
  │   │   │       └─ saveAutoPositions(cy)
  │   │   └─ NO:  ELK full layout → saveAutoPositions(cy)
  │   └─ cy.fit(50)
  │
  └─ startPolling()
```

### 5.2 Polling Update Flow

```
refresh()
  │
  ├─ fetchTopology() → data
  │
  ├─ renderGraph(cy, data)
  │   ├─ computeSignature(data)
  │   ├─ structureChanged?
  │   │   ├─ NO:  update data only (state, latency) → return
  │   │   └─ YES: full rebuild (same as startup flow)
  │   │           ├─ cy.batch() → remove all, add new
  │   │           ├─ pruneStalePositions(nodeIds)
  │   │           ├─ applySavedPositions(cy)
  │   │           │   └─ runIncrementalElk() if new nodes
  │   │           └─ saveAutoPositions(cy)
  │   │
  │   └─ return structureChanged
  │
  └─ if structureChanged: reapplyCollapsedState(cy)
```

### 5.3 User Drag Flow

```
User drags node(s)
  │
  ├─ grab event → determine companions (selection, Ctrl+downstream)
  │
  ├─ drag event → move companions by delta (existing logic)
  │
  └─ free event → NEW:
      ├─ markManualPosition(nodeId, pos) for grabbed node
      ├─ markManualPositions([...]) for companions
      └─ debounced write to localStorage
```

### 5.4 Reset Layout Flow

```
User clicks "Reset Layout"
  │
  ├─ clearSavedPositions() → localStorage cleared
  │
  ├─ cy.layout(ELK full).run()
  │
  └─ on layoutstop: saveAutoPositions(cy)
```

---

## 6. Position Persistence Schema

### localStorage Key

```
dephealth-node-positions
```

### Value Format

```json
{
  "admin-module": { "x": 350.5, "y": 120.3, "manual": false },
  "query-module": { "x": 520.0, "y": 120.3, "manual": false },
  "ingester-module": { "x": 180.0, "y": 120.3, "manual": true },
  "se-edit-1": { "x": 100.0, "y": 380.0, "manual": true },
  "se-ro": { "x": 300.0, "y": 380.0, "manual": false }
}
```

### Rules

| Scenario | Action |
|----------|--------|
| Node dragged manually | Save with `manual: true` |
| Node positioned by ELK | Save with `manual: false` |
| Node already manual, data update (no structural change) | Keep position, no layout triggered |
| Node already manual, structural change | Restore from saved via preset (not ELK) |
| Node removed from topology | Remove from store (prune) |
| New node appears | No entry in store → ELK positions it (incremental) |
| Reset layout clicked | Clear entire store, full ELK recalculation |
| Grouping toggled | Positions preserved (same node IDs), full rebuild |
| Layout direction changed (TB↔LR) | `clearManualFlags()`, full ELK recalculation |
| Click-release without movement (<1px) | Not marked as manual (no-op guard) |

### Parent/Compound Nodes

Parent nodes (namespace groups like `ns::core-modules`) are **NOT saved**.
Their position and size are automatically derived from children positions by Cytoscape.
Saving parent positions would conflict with Cytoscape's compound sizing.

### Collapsed Namespace Nodes

When a namespace is collapsed, its children are removed and the parent becomes
a summary node. Positions of original children are **preserved in store**
even while collapsed. On expand, children are restored and their saved positions
are applied.

---

## 7. Integration with Existing Features

### 7.1 Focus Mode (focus.js)

**No changes needed.** Focus mode uses CSS classes (`dimmed`, `focused`, etc.)
which are independent of node positions. Focus mode does not trigger relayout.

### 7.2 Selection (selection.js)

**No changes needed.** Multi-select uses Cytoscape's `:selected` state
which is independent of layout.

### 7.3 Search (search.js)

**No changes needed.** Search uses bypass styles (`.style()`) for highlighting,
which does not affect positions.

### 7.4 Collapse/Expand (grouping.js)

**Changes needed** (see Section 4.4):
- `expandNamespace()`: Replace hardcoded fCoSE with `relayout()` call
- Collapsed children positions preserved in layout-store
- On expand: children restored with their saved positions

### 7.5 Export (export.js)

**No changes needed.** Export captures current visual state regardless of
how positions were determined.

### 7.6 Context Menu (contextmenu.js)

**No changes needed.** Context menu is position-independent.

### 7.7 Cascade Warnings (cascade.js)

**No changes needed.** Cascade computation is data-based, not position-based.

### 7.8 Direction Toggle (TB/LR)

**Behavior change**: Currently hidden when grouping is enabled. With ELK
supporting both modes, the direction toggle becomes **always visible**.
Switching direction clears manual flags and triggers full ELK relayout.

### 7.9 Tooltip (tooltip.js)

**No changes needed.** Tooltips are triggered by hover events and position
themselves relative to the node — independent of layout algorithm.

### 7.10 Timeline / History Mode (timeline.js)

**No changes needed for MVP.** Positions are saved globally (not per-timestamp).
When navigating history, topology may differ — saved positions for nodes that
exist in the historical snapshot will be applied, others will be calculated
by ELK. This is acceptable behavior.

**Future consideration**: per-timestamp position storage could be added
if users frequently switch between historical views with custom layouts.

---

## 8. UI Changes

### 8.1 New Toolbar Button: Reset Layout

**Position**: After `btn-layout-toggle`, before `btn-grouping` in the
"Structure group" section of the graph toolbar.

```html
<!-- In graph-toolbar, Structure group -->
<button id="btn-layout-toggle" ...>...</button>
<button id="btn-reset-layout"
        data-i18n-title="graphToolbar.resetLayout"
        title="Reset layout">
  <i class="bi bi-arrow-counterclockwise"></i>
</button>
<button id="btn-grouping" ...>...</button>
```

**Icon**: `bi-arrow-counterclockwise` (Bootstrap Icons) — universally
recognized as "reset/redo".

**Behavior**: Click → `resetLayout(cy)` → no confirmation.

### 8.2 i18n Keys

Add to both EN and RU translation files:

```javascript
// English
graphToolbar: {
  resetLayout: 'Reset layout',
  // ...existing keys
}

// Russian
graphToolbar: {
  resetLayout: 'Сбросить раскладку',
  // ...existing keys
}
```

### 8.3 Direction Toggle Always Visible

Remove the `hidden` class toggle for `btn-layout-toggle` when grouping
is toggled. The button stays visible in all modes.

---

## 9. Dependencies

### New npm Packages

| Package | Version | Size | Purpose |
|---------|---------|------|---------|
| `cytoscape-elk` | ^2.2.0 | ~10KB | Cytoscape.js adapter for ELK |
| `elkjs` | ^0.9.3 | ~200KB | ELK layout engine (WASM) |

### Removed npm Packages

| Package | Reason |
|---------|--------|
| `cytoscape-dagre` | Replaced by ELK |
| `cytoscape-fcose` | Replaced by ELK |
| `dagre` | Transitive dependency of cytoscape-dagre |

### Net Bundle Size Impact

- Removed: dagre (~30KB) + cytoscape-dagre (~5KB) + cytoscape-fcose (~50KB) = ~85KB
- Added: elkjs (~200KB gzipped, ~1.2MB raw) + cytoscape-elk (~10KB) = ~210KB gzipped
- **Net increase: ~125KB gzipped**

This is acceptable per NFR-3. The JS-compiled ELK variant is fast enough
for graphs <50 nodes without needing a web worker. Vite's tree-shaking
and compression will minimize the actual impact on load time.

---

## 10. Implementation Phases

### Phase 1: ELK Integration (Core Layout)

Replace Dagre/fCoSE with ELK. No position persistence yet.

- [x] Install `cytoscape-elk` and `elkjs`
- [x] Remove `cytoscape-dagre`, `cytoscape-fcose`, `dagre`
- [x] Register ELK plugin in graph.js
- [x] Implement `buildElkLayoutConfig()`
- [x] Update `renderGraph()` to use ELK
- [x] Update `relayout()` to use ELK
- [x] Update `expandNamespace()` to use ELK (instead of fCoSE)
- [x] Make direction toggle visible in all modes
- [x] Test: flat layout TB/LR, grouped layout TB/LR, collapse/expand

**Acceptance**: Graph renders hierarchically in both flat and grouped modes.
Entry points at top, dependencies flowing down.

### Phase 2: Position Persistence

Add `layout-store.js` module and integrate.

- Create `layout-store.js` with full API
- Integrate into `renderGraph()` (save auto positions after layout)
- Integrate into `node-drag.js` (mark manual positions on free)
- Integrate into `relayout()` (lock manual, save after layout)
- Implement position restore on startup
- Implement stale position pruning
- Test: drag → refresh → positions preserved; reload → positions restored

**Acceptance**: Manual positions survive polling updates and page reload.

### Phase 3: Reset Layout Button + Polish

Add UI button and final integration.

- Add reset button to toolbar HTML
- Add i18n keys (EN/RU)
- Wire `resetLayout()` handler
- Test: reset clears positions and runs fresh ELK
- Test: new nodes appear at logical positions
- Test: collapsed namespace expand respects saved positions
- Test: all existing features (focus, search, export, cascade) still work

**Acceptance**: All requirements from spec satisfied. No regressions.

### Phase 4: Build, Test, Release

- Build Docker image
- Deploy to test cluster
- Visual testing with real topology data
- Performance check (<500ms for 50 nodes)
- Update CHANGELOG.md
- Tag release

---

## 11. Edge Cases and Risks

### Edge Cases

| Case | Handling |
|------|----------|
| Empty graph (0 nodes) | Skip layout, show empty state |
| Single node | ELK positions it at center, no edges to route |
| Cyclic graph (A→B→C→A) | ELK `layered` breaks cycles automatically (greedy cycle breaking) |
| All nodes manually positioned, then topology changes | Preset restores all, incremental ELK for new nodes only |
| localStorage full / corrupted JSON | Catch parse errors, fall back to full ELK layout |
| Multiple browser tabs | Each tab has its own in-memory cache; last-write-wins in localStorage. Acceptable for MVP. |
| Collapsed namespace + position restore | Children positions preserved in store even while collapsed. On expand, children get their saved positions back. |

### Risks

| Risk | Mitigation |
|------|------------|
| ELK `layered` produces worse results than Dagre for flat graphs | Tune ELK spacing options; keep Dagre as npm dependency for quick rollback in Phase 1 |
| `cytoscape-elk` does not support `transform` callback for incremental layout | Verify during Phase 2; fallback: run ELK for all nodes, then overwrite positions for saved nodes |
| ELK layout >500ms for large compound graphs | Profile during Phase 4; consider web worker variant of elkjs if needed |
| `nodeLayoutOptions` not called for compound parent nodes | Verified: callback receives all nodes including parents; we filter with `isParent()` |

---

## References

- [ELK Layout Options Reference](http://www.eclipse.org/elk/reference.html)
- [cytoscape-elk GitHub](https://github.com/cytoscape/cytoscape.js-elk)
- [elkjs GitHub](https://github.com/kieler/elkjs)
- [ELK Layered Algorithm](https://www.eclipse.org/elk/reference/algorithms/org-eclipse-elk-layered.html)
- [Requirements Spec](ui-graph-layout-optimization.md)
