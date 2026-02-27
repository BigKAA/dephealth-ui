# Plan: UI Toolbar Optimization & Edge Type Labels

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-02-27
- **Last updated**: 2026-02-27
- **Status**: Pending

---

## Version History

- **v1.0.0** (2026-02-27): Initial version

---

## Current Status

- **Active phase**: Phase 1
- **Active item**: 1.1
- **Last updated**: 2026-02-27
- **Note**: Plan created, awaiting approval

---

## Table of Contents

- [ ] [Phase 1: Toolbar optimization](#phase-1-toolbar-optimization)
- [ ] [Phase 2: Edge type labels](#phase-2-edge-type-labels)
- [ ] [Phase 3: Build and test](#phase-3-build-and-test)

---

## Phase 1: Toolbar optimization

**Dependencies**: None
**Status**: Pending

### Description

Reorganize graph toolbar and header to reduce button count in the floating toolbar.
Key changes:
- Remove duplicate Fit button from graph toolbar (already in header)
- Move Search button to header (between Filter and Refresh)
- Consolidate 3 Legend buttons into a single dropdown
- Add visual separators between button groups

### Items

- [ ] **1.1 Remove duplicate Fit button from graph toolbar**
  - **Dependencies**: None
  - **Description**: Remove `btn-toolbar-fit` from graph toolbar HTML and its event listener in `main.js`. The `btn-fit` in header already provides this functionality. Remove related i18n key `graphToolbar.fit` if unused elsewhere.
  - **Modifies**:
    - `frontend/index.html` — remove button element (line 67)
    - `frontend/src/main.js` — remove `btn-toolbar-fit` click handler
  - **Links**: N/A

- [ ] **1.2 Move Search button to header**
  - **Dependencies**: 1.1
  - **Description**: Move `btn-search` from graph toolbar to header toolbar (between Filter and Refresh buttons). Relocate `search-panel` div from inside `#cy` container to below the header (after `#filter-panel`), and adjust CSS positioning so the search panel appears below the header instead of relative to graph toolbar. Update `search.js` if it references graph toolbar positioning. Add i18n key `toolbar.search` for the header button.
  - **Modifies**:
    - `frontend/index.html` — move button and search-panel elements
    - `frontend/src/style.css` — adjust `.search-panel` positioning (currently `position: absolute; top: 40px; right: 16px`)
    - `frontend/src/search.js` — verify no hard-coded positioning relative to graph toolbar
    - `frontend/src/locales/en.js` — add `toolbar.search` key
    - `frontend/src/locales/ru.js` — add `toolbar.search` key
  - **Links**: N/A

- [ ] **1.3 Consolidate Legend buttons into dropdown**
  - **Dependencies**: 1.1
  - **Description**: Replace the three separate legend buttons (`btn-legend`, `btn-ns-legend`, `btn-conn-legend`) with a single button that opens a small dropdown/popover menu. The dropdown shows three toggle items with checkbox indicators (checked = legend visible). Each item click toggles the corresponding legend visibility and updates localStorage — reusing existing toggle logic from `setupLegend()`, `setupNamespaceLegend()`, `setupConnectionLegend()`. Dropdown closes on click outside.
    - Button icon: `bi-info-circle` (same as current legend button)
    - Dropdown items:
      1. Graph Legend (`bi-info-circle`) — toggles `#graph-legend`
      2. Namespace Legend (`bi-palette`) — toggles `#namespace-legend`
      3. Connection Legend (`bi-ethernet`) — toggles `#connection-legend`
  - **Modifies**:
    - `frontend/index.html` — replace 3 buttons with 1 button + dropdown container
    - `frontend/src/style.css` — add `.toolbar-dropdown` styles (position, z-index, items)
    - `frontend/src/main.js` — refactor `setupLegend/setupNamespaceLegend/setupConnectionLegend` to work with dropdown items; add dropdown open/close logic
    - `frontend/src/locales/en.js` — add `graphToolbar.legends` key ("Legends")
    - `frontend/src/locales/ru.js` — add corresponding key
  - **Links**: N/A

- [ ] **1.4 Add visual separators between button groups in graph toolbar**
  - **Dependencies**: 1.1, 1.2, 1.3
  - **Description**: Add thin horizontal line separators between button groups in the graph toolbar. Groups:
    1. **Navigation**: Zoom in, Zoom out
    2. **Structure**: Layout toggle, Grouping, Dimension toggle, Collapse all
    3. **Display**: (will contain Edge labels in Phase 2)
    4. **Utilities**: Export, Fullscreen, Legends dropdown
    Use a `<div class="toolbar-separator">` element between groups. Style as a thin horizontal line (1px, var(--border-light), 80% width, centered).
  - **Modifies**:
    - `frontend/index.html` — add separator divs between button groups
    - `frontend/src/style.css` — add `.toolbar-separator` style
  - **Links**: N/A

### Completion criteria Phase 1

- [ ] All items completed (1.1, 1.2, 1.3, 1.4)
- [ ] `btn-fit` in header works correctly (no regression)
- [ ] Search from header opens search panel below header, keyboard shortcut still works
- [ ] Legend dropdown opens/closes correctly, each toggle works, localStorage persists
- [ ] Separators render correctly in both light and dark themes
- [ ] No broken references or unused code left behind

---

## Phase 2: Edge type labels

**Dependencies**: Phase 1
**Status**: Pending

### Description

Add toggleable edge type labels to the graph. When enabled, the connection type (http, grpc, tcp, postgres, etc.) is prepended to the existing edge label. Toggle is controlled by a button in the graph toolbar "Display" group.

### Items

- [ ] **2.1 Add edge type label toggle button**
  - **Dependencies**: None
  - **Description**: Add a new button `btn-edge-labels` in the graph toolbar, in the "Display" group (between Structure and Utilities separators). Icon: `bi-tag`. Toggle state stored in `localStorage` key `dephealth-edge-labels`. Default: off. Button gets `active` class when enabled. Export a getter function `isEdgeLabelsEnabled()` from main.js for use in graph.js.
  - **Modifies**:
    - `frontend/index.html` — add button in Display section
    - `frontend/src/main.js` — add click handler, localStorage read/write, export getter
    - `frontend/src/locales/en.js` — add `graphToolbar.edgeLabels: 'Toggle edge type labels'`
    - `frontend/src/locales/ru.js` — add corresponding key
  - **Links**: N/A

- [ ] **2.2 Modify edge label rendering to include type**
  - **Dependencies**: 2.1
  - **Description**: In `graph.js`, modify the edge style `label` function (line 283-286) to conditionally prepend `ele.data('type')` when edge labels are enabled. Format: `type STATUS LATENCY` (e.g. `http TMO 142ms`, `grpc 23ms`, `postgres CONN`). When type is absent or toggle is off, behavior is unchanged. After toggling, call `cy.style().update()` to refresh labels without layout recalculation.
  - **Modifies**:
    - `frontend/src/graph.js` — update edge label function, import `isEdgeLabelsEnabled`
    - `frontend/src/main.js` — call `cy.style().update()` on toggle
  - **Links**: N/A

- [ ] **2.3 Update keyboard shortcuts**
  - **Dependencies**: 2.1
  - **Description**: Optionally add keyboard shortcut for edge labels toggle (e.g. `T` key) if consistent with existing shortcuts. Add to shortcuts help modal.
  - **Modifies**:
    - `frontend/src/main.js` — add keydown handler
    - `frontend/src/locales/en.js` — add `shortcuts.edgeLabels` key
    - `frontend/src/locales/ru.js` — add corresponding key
  - **Links**: N/A

### Completion criteria Phase 2

- [ ] All items completed (2.1, 2.2, 2.3)
- [ ] Toggle button shows/hides type on edge labels
- [ ] State persists across page reloads via localStorage
- [ ] Edges without `type` data show label unchanged
- [ ] Label is readable in both light and dark themes
- [ ] No layout recalculation on toggle (only style update)

---

## Phase 3: Build and test

**Dependencies**: Phase 1, Phase 2
**Status**: Pending

### Description

Build the application, deploy to test environment, and verify all changes work correctly.

### Items

- [ ] **3.1 Lint and build frontend**
  - **Dependencies**: None
  - **Description**: Run linter (`npm run lint`) and build (`npm run build`) to verify no errors. Fix any linting issues.
  - **Creates**:
    - Build artifacts in `frontend/dist/`
  - **Links**: N/A

- [ ] **3.2 Build Docker image**
  - **Dependencies**: 3.1
  - **Description**: Build Docker image with dev tag using `make docker-dev`. Push to Harbor registry.
  - **Creates**:
    - Docker image
  - **Links**: N/A

- [ ] **3.3 Deploy and test in Kubernetes**
  - **Dependencies**: 3.2
  - **Description**: Deploy dev image to test cluster, verify:
    - Graph toolbar is compact with separators
    - Search works from header
    - Legend dropdown works
    - Edge labels toggle works
    - All existing functionality is preserved (no regressions)
  - **Links**: N/A

### Completion criteria Phase 3

- [ ] All items completed (3.1, 3.2, 3.3)
- [ ] Container successfully built
- [ ] All linting passes
- [ ] Manual testing in Kubernetes cluster passes
- [ ] No regressions in existing functionality

---

## Notes

- Edge `type` field already comes from the backend (via topologymetrics Prometheus labels). No backend changes needed.
- Possible types per `docs/METRICS.md`: `http`, `grpc`, `tcp`, `postgres`, `mysql`, `redis`, `mongodb`, `amqp`, `kafka`.
- The `btn-fit` duplication was unintentional — header Fit button was added first, toolbar Fit was added later for fullscreen convenience. In fullscreen mode, the header is still visible, so no functionality is lost.
- Search panel positioning will need CSS adjustment for header-relative position instead of graph-toolbar-relative.

---
