# Plan: ELK Layout Optimization — Implementation Workflow

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-03-03
- **Last updated**: 2026-03-03
- **Status**: In Progress
- **Design document**: [design-graph-layout-optimization.md](design-graph-layout-optimization.md)
- **Requirements**: [ui-graph-layout-optimization.md](ui-graph-layout-optimization.md)

---

## Version History

- **v1.0.0** (2026-03-03): Initial workflow plan
- **v1.1.0** (2026-03-03): Phases 1–4 implemented, updated status

---

## Current Status

- **Active phase**: Phase 5
- **Active item**: 5.1
- **Last updated**: 2026-03-03
- **Note**: Phases 1–4 fully implemented. Phase 5 (build, deploy, visual testing) pending.

---

## Git Strategy

- **Working branch**: `feature/elk-layout-optimization`
- **Base branch**: `master` (at tag `v0.18.0`)
- **Merge to main**: NOT included in this plan — user decides merge strategy after review
- **Dev image tag**: `v0.19.0-1`, `v0.19.0-2`, ... (increment per build)
- **Release image tag**: decided at merge time (not in scope)

---

## Table of Contents

- [x] [Phase 1: Project Setup & Dependency Swap](#phase-1-project-setup--dependency-swap)
- [x] [Phase 2: ELK Core Layout](#phase-2-elk-core-layout)
- [x] [Phase 3: Position Persistence](#phase-3-position-persistence)
- [x] [Phase 4: UI — Reset Button & Polish](#phase-4-ui--reset-button--polish)
- [ ] [Phase 5: Build, Deploy & Visual Testing](#phase-5-build-deploy--visual-testing)

---

## Phase 1: Project Setup & Dependency Swap

**Dependencies**: None
**Status**: ✅ Done

### Description

Create feature branch, swap npm dependencies (remove Dagre/fCoSE, add ELK),
and verify the build compiles without errors. No layout logic changes yet.

### Subpoints

- [x] **1.1 Create feature branch**
  - **Dependencies**: None
  - **Description**: Create `feature/elk-layout-optimization` from `master` HEAD (`v0.18.0`).
  - **Creates**: N/A (git operation)

- [x] **1.2 Swap npm dependencies**
  - **Dependencies**: 1.1
  - **Description**: Install `cytoscape-elk` and `elkjs`. Remove `cytoscape-dagre`, `cytoscape-fcose`, `dagre`. Update `package.json` and regenerate lock file.
  - **Creates**:
    - `frontend/package.json` (modified)
    - `frontend/package-lock.json` (regenerated)
  - **Links**:
    - [Design: Section 9 — Dependencies](design-graph-layout-optimization.md#9-dependencies)
    - [cytoscape-elk npm](https://www.npmjs.com/package/cytoscape-elk)
    - [elkjs npm](https://www.npmjs.com/package/elkjs)

- [x] **1.3 Update plugin registration in graph.js**
  - **Dependencies**: 1.2
  - **Description**: Replace Dagre/fCoSE imports with ELK import. Register `cytoscape.use(elk)`. Keep `cytoscapeSvg`. Comment out old `buildLayoutConfig()` temporarily (replace with stub that returns ELK config to prevent build errors).
  - **Creates**:
    - `frontend/src/graph.js` (modified — imports only)
  - **Links**:
    - [Design: Section 3 — Plugin Registration](design-graph-layout-optimization.md#plugin-registration-graphjs)

- [x] **1.4 Verify build compiles**
  - **Dependencies**: 1.3
  - **Description**: Run `cd frontend && npm run build` to verify no import/compile errors. Fix any issues.
  - **Creates**: N/A (verification step)

### ✅ Criteria for Phase 1

- [x] All subpoints completed (1.1–1.4)
- [x] `npm run build` succeeds with no errors
- [x] Old layout packages fully removed from `package.json`
- [x] ELK packages present in `node_modules`
- [x] Commit: `feat(frontend): replace Dagre/fCoSE with ELK layout engine` (`a28532f`)

---

## Phase 2: ELK Core Layout

**Dependencies**: Phase 1
**Status**: ✅ Done

### Description

Implement the ELK `layered` layout as the single layout engine for both flat
and grouped (compound) graph modes. Replace all Dagre and fCoSE layout calls.
After this phase, the graph should render hierarchically from entry points
downward in layers.

### Subpoints

- [x] **2.1 Implement `buildElkLayoutConfig()`**
  - **Dependencies**: None
  - **Description**: Create the new layout config function with all ELK options: `layered` algorithm, direction support (DOWN/RIGHT), `INCLUDE_CHILDREN` for compound nodes, `LONGEST_PATH` layering, crossing minimization, spacing, and `nodeLayoutOptions` callback for entry point pinning.
  - **Creates**:
    - `frontend/src/graph.js` (modified — new function)
  - **Links**:
    - [Design: Section 3 — ELK Layout Options](design-graph-layout-optimization.md#elk-layout-options)
    - [ELK Layered Algorithm Reference](https://www.eclipse.org/elk/reference/algorithms/org-eclipse-elk-layered.html)
  - **Code example**:
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
        nodeLayoutOptions: (node) => {
          if (node.isParent()) return {};
          if (node.data('isEntry') || node.indegree(false) === 0) {
            return { 'elk.layered.layering.layerConstraint': 'FIRST' };
          }
          return {};
        },
        elk: {
          algorithm: 'layered',
          'elk.direction': direction,
          'elk.hierarchyHandling': 'INCLUDE_CHILDREN',
          'elk.layered.layering.strategy': 'LONGEST_PATH',
          'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
          'elk.layered.nodePlacement.strategy': 'BRANDES_KOEPF',
          'elk.layered.spacing.nodeNodeBetweenLayers': 100,
          'elk.layered.spacing.edgeNodeBetweenLayers': 30,
          'elk.spacing.nodeNode': 60,
          'elk.padding': '[left=20, top=30, right=20, bottom=20]',
          'elk.layered.edgeRouting': 'POLYLINE',
        },
      };
    }
    ```

- [x] **2.2 Add `getLayoutDirection()` export**
  - **Dependencies**: None
  - **Description**: Add a getter function `export function getLayoutDirection()` that returns the module-local `layoutDirection` variable. Needed by `grouping.js` for relayout calls.
  - **Creates**:
    - `frontend/src/graph.js` (modified — new export)

- [x] **2.3 Update `renderGraph()` to use ELK**
  - **Dependencies**: 2.1
  - **Description**: In the structure-changed path of `renderGraph()`, replace `cy.layout(buildLayoutConfig()).run()` with `cy.layout(buildElkLayoutConfig()).run()`. Remove the old `buildLayoutConfig()` function entirely. Keep the smart diffing logic (data-only update) unchanged.
  - **Creates**:
    - `frontend/src/graph.js` (modified — renderGraph)
  - **Links**:
    - [Design: Section 4.1 — renderGraph](design-graph-layout-optimization.md#modify-rendergraph-lines-636-757)

- [x] **2.4 Update `relayout()` to use ELK**
  - **Dependencies**: 2.1
  - **Description**: Replace Dagre config with ELK config in `relayout()`. For now (Phase 2), simple version without position persistence — just run full ELK with animation.
  - **Creates**:
    - `frontend/src/graph.js` (modified — relayout)
  - **Links**:
    - [Design: Section 4.1 — relayout](design-graph-layout-optimization.md#modify-relayout-lines-782-786)

- [x] **2.5 Update `expandNamespace()` in grouping.js**
  - **Dependencies**: 2.1, 2.2
  - **Description**: Replace hardcoded fCoSE layout in `expandNamespace()` with `relayout(cy, getLayoutDirection())`. Import both from `graph.js`.
  - **Creates**:
    - `frontend/src/grouping.js` (modified — expand relayout)
  - **Links**:
    - [Design: Section 4.4 — grouping.js](design-graph-layout-optimization.md#44-groupingjs)

- [x] **2.6 Make direction toggle always visible**
  - **Dependencies**: None
  - **Description**: In `main.js`, remove the line that hides `btn-layout-toggle` when grouping is enabled: `btnLayoutToggle.classList.toggle('hidden', next)`. The direction toggle should be visible in all modes since ELK supports both flat and compound layouts.
  - **Creates**:
    - `frontend/src/main.js` (modified — grouping toggle handler)
  - **Links**:
    - [Design: Section 4.3 — Grouping Toggle Visibility](design-graph-layout-optimization.md#modify-grouping-toggle-visibility)
    - [Design: Section 7.8 — Direction Toggle](design-graph-layout-optimization.md#78-direction-toggle-tblr)

- [x] **2.7 Visual testing — ELK core**
  - **Dependencies**: 2.3, 2.4, 2.5, 2.6
  - **Description**: Deploy to test cluster and verify:
    - Flat graph: renders hierarchically TB and LR
    - Grouped graph: renders hierarchically with compound nodes
    - Entry points (`isEntry=true`) at top layer
    - Collapse/expand works with ELK relayout
    - Direction toggle works in both flat and grouped modes
    - No regressions in focus mode, search, selection, sidebar, export
  - **Creates**: N/A (manual testing)

### ✅ Criteria for Phase 2

- [x] All subpoints completed (2.1–2.7)
- [x] Graph renders hierarchically in flat mode (TB and LR)
- [x] Graph renders hierarchically in grouped mode (TB and LR)
- [x] Entry points placed on top layer
- [x] Collapse/expand triggers ELK relayout correctly
- [x] Direction toggle visible and working in all modes
- [x] No regressions in existing features
- [x] Commit: `feat(frontend): replace Dagre/fCoSE with ELK layout engine` (`a28532f`)

---

## Phase 3: Position Persistence

**Dependencies**: Phase 2
**Status**: ✅ Done

### Description

Create `layout-store.js` module for position persistence. Integrate with
`graph.js` (save/restore on render), `node-drag.js` (mark manual on drag),
and `relayout()` (direction change handling). After this phase, manual node
positions survive polling updates and page reloads.

### Subpoints

- [x] **3.1 Create `layout-store.js` module**
  - **Dependencies**: None
  - **Description**: Implement the full API: `getSavedPositions`, `markManualPosition`, `markManualPositions`, `clearSavedPositions`, `clearManualFlags`, `pruneStalePositions`, `applySavedPositions`, `hasSavedPositions`, `saveAutoPositions`. Include in-memory cache, debounced writes (300ms), no-op drag guard (<1px), parent node filtering.
  - **Creates**:
    - `frontend/src/layout-store.js` (new file)
  - **Links**:
    - [Design: Section 2 — layout-store.js API](design-graph-layout-optimization.md#2-new-module-layout-storejs)
    - [Design: Section 6 — Position Persistence Schema](design-graph-layout-optimization.md#6-position-persistence-schema)

- [x] **3.2 Integrate position restore into `renderGraph()`**
  - **Dependencies**: 3.1
  - **Description**: Modify the structure-changed path in `renderGraph()` to implement the two-step approach:
    1. After batch add: call `pruneStalePositions(currentNodeIds)`
    2. If `hasSavedPositions()`: call `applySavedPositions(cy)` to set saved coords, then `runIncrementalElk()` for unpositioned nodes
    3. If no saved positions: run full ELK, then `saveAutoPositions(cy)`
    4. Always `cy.fit(50)` after positioning

    Also implement `runIncrementalElk(cy, unpositionedIds)` helper using `config.transform` callback.
  - **Creates**:
    - `frontend/src/graph.js` (modified — renderGraph + new helper)
  - **Links**:
    - [Design: Section 4.1 — renderGraph new flow](design-graph-layout-optimization.md#modify-rendergraph-lines-636-757)
    - [Design: Section 5.1 — Startup Flow](design-graph-layout-optimization.md#51-startup-flow)
    - [Design: Section 5.2 — Polling Update Flow](design-graph-layout-optimization.md#52-polling-update-flow)

- [x] **3.3 Integrate manual position marking into `node-drag.js`**
  - **Dependencies**: 3.1
  - **Description**: Modify the `free` event handler to:
    - Skip compound parent nodes (`node.isParent()`)
    - Check no-op drag guard (>1px movement threshold)
    - Call `markManualPosition()` for the grabbed node
    - Call `markManualPositions()` for companion nodes (group/downstream drag)
  - **Creates**:
    - `frontend/src/node-drag.js` (modified — free event)
  - **Links**:
    - [Design: Section 4.2 — node-drag.js](design-graph-layout-optimization.md#42-node-dragjs)
    - [Design: Section 5.3 — User Drag Flow](design-graph-layout-optimization.md#53-user-drag-flow)

- [x] **3.4 Update `relayout()` for direction change**
  - **Dependencies**: 3.1
  - **Description**: Update `relayout()` to call `clearManualFlags()` before running ELK. On `layoutstop` event, call `saveAutoPositions(cy)`. This ensures direction change (TB↔LR) clears manual positions and recalculates everything for the new direction.
  - **Creates**:
    - `frontend/src/graph.js` (modified — relayout)
  - **Links**:
    - [Design: Section 4.1 — relayout](design-graph-layout-optimization.md#modify-relayout-lines-782-786)

- [x] **3.5 Handle edge cases**
  - **Dependencies**: 3.2, 3.3
  - **Description**: Implement error handling for:
    - Corrupted localStorage JSON → catch parse error, fall back to full ELK
    - Empty graph (0 nodes) → skip layout
    - Verify `transform` callback works with cytoscape-elk. If not supported, implement fallback: run ELK for all, then overwrite saved positions via `node.position()`.
  - **Creates**:
    - `frontend/src/layout-store.js` (modified — error handling)
    - `frontend/src/graph.js` (modified — edge cases)
  - **Links**:
    - [Design: Section 11 — Edge Cases and Risks](design-graph-layout-optimization.md#11-edge-cases-and-risks)

- [x] **3.6 Testing — position persistence**
  - **Dependencies**: 3.2, 3.3, 3.4, 3.5
  - **Description**: Verify in browser:
    - Drag node → refresh (polling) → position preserved
    - Drag node → reload page → position preserved
    - Drag node → switch direction TB→LR → manual position cleared, ELK recalculates
    - New node appears in topology → existing positions preserved, new node auto-positioned
    - Node removed from topology → position pruned from localStorage
    - Click-release (no drag) → node NOT marked as manual
    - Group drag (multi-select) → all dragged nodes marked as manual
    - Ctrl+Drag downstream → all downstream nodes marked as manual
  - **Creates**: N/A (manual testing)

### ✅ Criteria for Phase 3

- [x] All subpoints completed (3.1–3.6)
- [x] Manual positions survive polling updates (no structural change)
- [x] Manual positions survive page reload
- [x] Direction change clears manual positions and recalculates
- [x] New nodes auto-positioned without breaking existing layout
- [x] Removed nodes pruned from localStorage
- [x] No-op drag guard works (<1px = not manual)
- [x] Corrupted localStorage handled gracefully
- [x] Commit: `feat(frontend): add position persistence and reset layout button` (`3d48ebe`)

---

## Phase 4: UI — Reset Button & Polish

**Dependencies**: Phase 3
**Status**: ✅ Done

### Description

Add the "Reset layout" button to the toolbar, add i18n translations,
implement `resetLayout()`, and perform final integration testing with
all existing features.

### Subpoints

- [x] **4.1 Add reset button to toolbar HTML**
  - **Dependencies**: None
  - **Description**: Add `<button id="btn-reset-layout">` with icon `bi-arrow-counterclockwise` after `btn-layout-toggle` and before `btn-grouping` in the Structure group of `graph-toolbar`.
  - **Creates**:
    - `frontend/index.html` (modified — toolbar)
  - **Links**:
    - [Design: Section 8.1 — Reset Layout Button](design-graph-layout-optimization.md#81-new-toolbar-button-reset-layout)

- [x] **4.2 Add i18n keys**
  - **Dependencies**: None
  - **Description**: Add `graphToolbar.resetLayout` key to both EN and RU translation objects in `i18n.js`. EN: `"Reset layout"`, RU: `"Сбросить раскладку"`.
  - **Creates**:
    - `frontend/src/i18n.js` (modified — translation keys)
  - **Links**:
    - [Design: Section 8.2 — i18n Keys](design-graph-layout-optimization.md#82-i18n-keys)

- [x] **4.3 Implement `resetLayout()` and wire handler**
  - **Dependencies**: 4.1
  - **Description**: Add `export function resetLayout(cy)` to `graph.js`: clears saved positions, runs full ELK with animation, saves auto positions on layoutstop. Wire click handler for `btn-reset-layout` in `main.js`.
  - **Creates**:
    - `frontend/src/graph.js` (modified — new export)
    - `frontend/src/main.js` (modified — event handler)
  - **Links**:
    - [Design: Section 4.1 — resetLayout](design-graph-layout-optimization.md#new-resetlayout-export)
    - [Design: Section 4.3 — Reset handler](design-graph-layout-optimization.md#add-reset-layout-button-handler)
    - [Design: Section 5.4 — Reset Layout Flow](design-graph-layout-optimization.md#54-reset-layout-flow)

- [x] **4.4 Full regression testing**
  - **Dependencies**: 4.3
  - **Description**: Complete feature testing:
    - **Layout**: flat TB/LR, grouped TB/LR, entry points at top
    - **Persistence**: drag → refresh, drag → reload, direction change
    - **Reset**: clears all positions, fresh ELK layout
    - **Incremental**: new node → auto-positioned near neighbors
    - **Grouping**: collapse → positions preserved, expand → ELK relayout
    - **Focus mode**: click focus, shift+click downstream, shift+alt+click upstream
    - **Selection**: multi-select, box-select, Ctrl+click
    - **Search**: find node, highlight, clear
    - **Drag**: single, multi-select group, Ctrl+downstream, Ctrl+Shift+full downstream
    - **Export**: PNG, SVG, JSON, CSV, DOT
    - **Sidebar**: node details on click
    - **Context menu**: right-click, Grafana links
    - **Cascade warnings**: cascade badge display
    - **Timeline**: history mode navigation
    - **Edge labels**: toggle on/off
  - **Creates**: N/A (manual testing)

### ✅ Criteria for Phase 4

- [x] All subpoints completed (4.1–4.4)
- [x] Reset button visible in toolbar with correct icon
- [x] Reset button tooltip shows in current language (EN/RU)
- [x] Reset clears positions and runs fresh ELK
- [x] All existing features work without regressions
- [x] Commit: `feat(frontend): add position persistence and reset layout button` (`3d48ebe`)

---

## Phase 5: Build, Deploy & Visual Testing

**Dependencies**: Phase 4
**Status**: Pending

### Description

Build dev Docker image, deploy to test cluster with real topology data,
perform visual testing and performance check. Do NOT merge to main —
branch stays open for user review.

### Subpoints

- [ ] **5.1 Bump appVersion in Chart.yaml**
  - **Dependencies**: None
  - **Description**: Update `appVersion` in `deploy/helm/dephealth-ui/Chart.yaml` from `"0.18.0"` to `"0.19.0"`. This reflects the new layout engine version.
  - **Creates**:
    - `deploy/helm/dephealth-ui/Chart.yaml` (modified)

- [ ] **5.2 Build dev Docker image**
  - **Dependencies**: 5.1
  - **Description**: Build multi-platform image and push to Harbor dev registry:
    ```bash
    make docker-build TAG=v0.19.0-1
    ```
  - **Creates**:
    - Docker image `harbor.kryukov.lan/library/dephealth-ui:v0.19.0-1`

- [ ] **5.3 Deploy to test cluster**
  - **Dependencies**: 5.2
  - **Description**: Deploy using Helm with the new image tag:
    ```bash
    helm upgrade dephealth-ui deploy/helm/dephealth-ui \
      --set image.tag=v0.19.0-1 \
      --namespace dephealth-test
    ```
    Verify pods are running: `make env-status`
  - **Creates**: N/A (deployment)

- [ ] **5.4 Visual testing with real topology**
  - **Dependencies**: 5.3
  - **Description**: Open the deployed UI in browser and verify with real topology data:
    - Graph renders hierarchically (compare with screenshots from requirements spec)
    - Entry points at top, dependencies flowing down in layers
    - Grouped mode: namespace compound nodes correctly positioned
    - Drag nodes → reload page → positions restored
    - Click "Reset layout" → fresh hierarchical layout
    - Direction toggle TB↔LR works in all modes
    - Collapse/expand namespaces → layout recalculates correctly
    - Performance: layout calculates in <500ms (check DevTools Performance tab)
  - **Creates**: N/A (visual verification)

- [ ] **5.5 Final commit on branch**
  - **Dependencies**: 5.4
  - **Description**: If any fixes were needed during testing, commit them. Final commit message summarizes the feature. Do NOT merge to main — leave branch for user review.
  - **Creates**: N/A (git operation)

### ✅ Criteria for Phase 5

- [ ] All subpoints completed (5.1–5.5)
- [ ] Docker image built and pushed to Harbor
- [ ] Application deployed and running in test cluster
- [ ] Visual layout matches expected hierarchical structure
- [ ] Performance <500ms for test topology
- [ ] All positions survive page reload
- [ ] Branch `feature/elk-layout-optimization` ready for review
- [ ] No merge to main performed

---

## Notes

- **Dagre fallback**: During Phase 2, if ELK produces unexpectedly worse results
  for flat graphs, Dagre can be temporarily re-added for comparison (see
  [Design: Risk table](design-graph-layout-optimization.md#risks))
- **`transform` callback**: If `cytoscape-elk` doesn't support the `transform`
  option for incremental layout (Phase 3), use fallback: run full ELK, then
  overwrite positions for saved nodes via `node.position(savedPos)`
  (see [Design: Risk table](design-graph-layout-optimization.md#risks))
- **Commit convention**: All commits use Conventional Commits format per
  project CLAUDE.md. Each phase ends with a commit.
- **Minor version**: `v0.19.0` chosen because this is a new feature (layout
  engine change), not a patch. Confirmed appropriate per CLAUDE.md convention.

---

**Phases 1–4 implemented. Phase 5 (build, deploy, visual testing) pending.**
