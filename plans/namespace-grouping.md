# Plan: Namespace Grouping

## Metadata

- **Version**: 1.0.0
- **Created**: 2026-02-11
- **Updated**: 2026-02-11
- **Status**: Pending
- **Requirements**: [.tasks/ui.md](../.tasks/ui.md)

---

## Version History

- **v1.0.0** (2026-02-11): Initial plan

---

## Current Status

- **Active Phase**: Phase 1
- **Active Task**: 1.1
- **Updated**: 2026-02-11
- **Note**: Plan created, awaiting approval

---

## Table of Contents

- [ ] [Phase 1: Foundation — Compound Nodes + fcose Layout](#phase-1-foundation--compound-nodes--fcose-layout)
- [ ] [Phase 2: Collapse / Expand](#phase-2-collapse--expand)
- [ ] [Phase 3: Persistence & Refresh Integration](#phase-3-persistence--refresh-integration)
- [ ] [Phase 4: Polish, Integration & Deploy](#phase-4-polish-integration--deploy)

---

## Phase 1: Foundation — Compound Nodes + fcose Layout

**Dependencies**: None
**Status**: Pending

### Description

Set up the technical foundation: install fcose, create the grouping module, add compound
parent nodes, implement Cytoscape compound styles, add the toolbar toggle button, and
wire dual layout switching (dagre ↔ fcose).

After this phase, pressing the toggle button should switch between the current flat dagre
graph and a grouped fcose graph with dashed namespace borders and labels.

### Tasks

- [ ] **1.1 Install cytoscape-fcose and register the layout**
  - **Dependencies**: None
  - **Description**: Add `cytoscape-fcose` to `package.json`, import and register it in
    `graph.js` alongside the existing dagre registration. Verify Vite resolves the module.
  - **Modifies**:
    - `frontend/package.json` — add `cytoscape-fcose` dependency
    - `frontend/src/graph.js` — import + `cytoscape.use(fcose)` registration

- [ ] **1.2 Create `grouping.js` module — state management and data transform**
  - **Dependencies**: 1.1
  - **Description**: Create the core grouping module with:
    - `isGroupingEnabled()` / `setGroupingEnabled(bool)` — read/write grouping state
      (backed by localStorage key `dephealth-grouping`).
    - `getCollapsedNamespaces()` / `setCollapsedNamespaces(set)` — read/write collapsed
      set (backed by localStorage key `dephealth-collapsed-ns`).
    - `buildCompoundElements(data)` — given topology data, produce an array of Cytoscape
      parent-node elements `{ data: { id: 'ns::<namespace>', label: namespace, isGroup: true } }`
      for each unique namespace found in nodes. Nodes **with** a namespace get
      `data.parent = 'ns::<namespace>'`. Nodes **without** a namespace stay parentless.
    - Export all functions.
  - **Creates**:
    - `frontend/src/grouping.js`

- [ ] **1.3 Add compound node styles to Cytoscape stylesheet**
  - **Dependencies**: 1.2
  - **Description**: In `graph.js`, add style selectors for compound (parent) nodes:
    - Selector `:parent` (or `node[?isGroup]`):
      - `shape: 'round-rectangle'`, `corner-radius: 6`
      - `border-width: 2`, `border-style: 'dashed'`, `border-color` from `getNamespaceColor(label)`
      - `background-opacity: 0.04`
      - `padding: 20px`
      - `label: data(label)`, `text-valign: 'top'`, `text-halign: 'center'`
      - `font-size: 13`, `font-weight: 'bold'`
      - `color` matches `border-color`
      - `compound-sizing-wrt-labels: 'include'`
    - Handle dark/light theme: adjust `background-opacity` and `color` per theme.
  - **Modifies**:
    - `frontend/src/graph.js` — add compound style entries to the stylesheet array

- [ ] **1.4 Add toolbar toggle button (HTML + wiring)**
  - **Dependencies**: 1.2
  - **Description**:
    - In `index.html`, add a grouping toggle button to `#graph-toolbar` (after the
      layout-direction button). Icon: `bi-bounding-box`. Tooltip: i18n key
      `toolbar.groupByNamespace` / `toolbar.ungroupNamespace`.
    - In `main.js` → `setupGraphToolbar()`, attach a click handler that toggles
      `setGroupingEnabled()` and calls `refresh()`.
    - Add i18n keys to `en.js` and `ru.js`.
    - When grouping is ON, visually indicate active state (e.g. `active` CSS class like
      the existing auto-refresh button).
  - **Modifies**:
    - `frontend/index.html`
    - `frontend/src/main.js`
    - `frontend/src/locales/en.js`
    - `frontend/src/locales/ru.js`
    - `frontend/src/style.css` (if needed for active state)

- [ ] **1.5 Wire dual layout in `renderGraph()`**
  - **Dependencies**: 1.3, 1.4
  - **Description**: Modify `renderGraph()` in `graph.js`:
    - If `isGroupingEnabled()`:
      1. Call `buildCompoundElements(data)` to get parent nodes.
      2. Add parent nodes **before** child nodes (dagre ordering requirement doesn't
         apply to fcose, but order-first is good practice).
      3. Set `data.parent` on each child node.
      4. Run **fcose** layout with options: `{ name: 'fcose', animate: false, quality: 'default', nodeSeparation: 80, idealEdgeLength: 120 }`.
    - If grouping is OFF:
      1. Remove any parent nodes, clear `data.parent` from children.
      2. Run dagre layout as before.
    - Hide/show the TB/LR toggle button based on grouping state (fcose is non-directional).
    - The existing `computeSignature()` should include grouping state so structure-change
      detection triggers a full re-render when toggling.
  - **Modifies**:
    - `frontend/src/graph.js` — `renderGraph()`, signature computation

### Completion Criteria — Phase 1

- [ ] All tasks completed (1.1–1.5)
- [ ] Toggle button switches between flat dagre and grouped fcose
- [ ] Namespace groups display dashed colored border + label
- [ ] Nodes without namespace remain ungrouped
- [ ] No regressions in existing graph functionality (sidebar, search, filters, context menu)
- [ ] Works in both dark and light themes

---

## Phase 2: Collapse / Expand

**Dependencies**: Phase 1
**Status**: Pending

### Description

Implement collapse and expand of individual namespace groups and bulk collapse/expand.
When a namespace is collapsed, its children are removed and replaced by a single summary
node. Edges are redirected to the summary node. This phase does NOT handle persistence
across refreshes (that's Phase 3).

### Tasks

- [ ] **2.1 Collapse a single namespace — core logic**
  - **Dependencies**: None
  - **Description**: In `grouping.js`, implement:
    - `collapseNamespace(cy, nsId)`:
      1. Find the parent node `ns::<name>`.
      2. Collect all child nodes and their data (state, alertCount, alertSeverity).
      3. Compute summary: `worstState`, `totalServices`, `totalAlerts`.
      4. Remove children from Cytoscape (but store them in a Map
         `collapsedData: Map<nsId, { nodes, edges }>`).
      5. Replace the parent node with a **collapsed summary node**:
         - `id: 'ns::<name>'` (reuse same ID for edge continuity)
         - `label: '<name> (N)'` where N = total services
         - `state: worstState` (determines node color)
         - `isCollapsed: true`, `isGroup: true`
         - Style: larger size (e.g. 120×60), round-rectangle, color by worst state,
           bold label.
      6. Update `collapsedNamespaces` set.
    - Important: edges whose source or target was a child must be redirected. Edges
      internal to the namespace (both endpoints inside) are removed entirely.
  - **Modifies**:
    - `frontend/src/grouping.js`

- [ ] **2.2 Edge redirection on collapse**
  - **Dependencies**: 2.1
  - **Description**: When collapsing namespace N:
    - **Cross-namespace edges** (one endpoint inside N, one outside):
      - Redirect the inside endpoint to the collapsed node `ns::<N>`.
      - Deduplicate: if multiple edges connect the same external node to N, create a
        single aggregated edge with label `×K` and worst-state color.
    - **Both endpoints collapsed** (both namespaces collapsed):
      - Create a single aggregated edge between the two collapsed nodes.
    - Store original edge data in `collapsedData` for restoration.
    - Edge ID convention for aggregated edges: `agg::<source>::<target>`.
  - **Modifies**:
    - `frontend/src/grouping.js`

- [ ] **2.3 Expand a single namespace**
  - **Dependencies**: 2.1, 2.2
  - **Description**: In `grouping.js`, implement:
    - `expandNamespace(cy, nsId)`:
      1. Retrieve stored `collapsedData[nsId]`.
      2. Remove aggregated edges connected to `ns::<name>`.
      3. Restore the parent node as a compound (non-collapsed) node.
      4. Re-add child nodes with `parent: 'ns::<name>'`.
      5. Re-add original edges (both internal and cross-namespace).
      6. Re-check other collapsed namespaces: if an edge target was in another
         collapsed namespace, redirect that end to the other collapsed node.
      7. Update `collapsedNamespaces` set.
      8. Run fcose layout on restored elements only (or full relayout).
  - **Modifies**:
    - `frontend/src/grouping.js`

- [ ] **2.4 Double-click handlers**
  - **Dependencies**: 2.3
  - **Description**: In `grouping.js` or `main.js`, register event handlers:
    - `cy.on('dbltap', 'node[?isGroup][!isCollapsed]', ...)` → `collapseNamespace()`
    - `cy.on('dbltap', 'node[?isCollapsed]', ...)` → `expandNamespace()`
    - These handlers should only be active when grouping is enabled.
    - **Conflict resolution**: existing `dbltap` on nodes with `grafanaUrl` opens Grafana
      (in `sidebar.js`). Parent/collapsed nodes won't have `grafanaUrl`, so no conflict.
      But verify the selector specificity.
  - **Modifies**:
    - `frontend/src/grouping.js` (or `frontend/src/main.js`)
    - `frontend/src/sidebar.js` — ensure dbltap on group nodes doesn't trigger Grafana

- [ ] **2.5 Collapsed node styling**
  - **Dependencies**: 2.1
  - **Description**: Add Cytoscape style entries for collapsed nodes:
    - Selector `node[?isCollapsed]`:
      - Size: `width: 140`, `height: 55`
      - `shape: 'round-rectangle'`
      - `background-color`: map `state` → color (ok=green, degraded=yellow, down=red, unknown=gray)
      - `border-width: 3`, `border-color`: darker shade of state color
      - `label`: namespace name + service count
      - `font-size: 14`, `font-weight: bold`, `text-wrap: wrap`
      - `text-valign: center`, `text-halign: center`
      - `color: white` (for contrast on colored background)
    - Ensure alert badge overlay (`updateAlertBadges`) works with collapsed nodes
      (show aggregate alert count).
  - **Modifies**:
    - `frontend/src/graph.js` — add collapsed-node style entries

- [ ] **2.6 Collapse All / Expand All button**
  - **Dependencies**: 2.4
  - **Description**:
    - In `index.html`, add a button next to the grouping toggle. Icon: `bi-arrows-collapse`
      / `bi-arrows-expand`. Tooltip: i18n keys.
    - Button only visible when grouping is ON.
    - Logic: if any namespace is expanded → collapse all; if all collapsed → expand all.
    - In `main.js` → `setupGraphToolbar()`, wire the click handler.
    - Add i18n keys.
  - **Modifies**:
    - `frontend/index.html`
    - `frontend/src/main.js`
    - `frontend/src/locales/en.js`
    - `frontend/src/locales/ru.js`

### Completion Criteria — Phase 2

- [ ] All tasks completed (2.1–2.6)
- [ ] Double-click on namespace group collapses it into a summary node
- [ ] Collapsed node shows namespace name, service count, worst state color
- [ ] Edges correctly redirect to collapsed nodes (no dangling edges)
- [ ] Aggregated edges show `×K` label
- [ ] Double-click on collapsed node expands it back
- [ ] Collapse All / Expand All button works
- [ ] Internal edges (within same namespace) are hidden when collapsed

---

## Phase 3: Persistence & Refresh Integration

**Dependencies**: Phase 2
**Status**: Pending

### Description

Make grouping and collapse state survive auto-refresh cycles and page reloads.
Integrate with the existing smart-diff rendering pipeline so that topology updates
don't reset the user's grouping choices.

### Tasks

- [ ] **3.1 Persist grouping state to localStorage**
  - **Dependencies**: None
  - **Description**:
    - `setGroupingEnabled()` already writes to localStorage (from 1.2).
    - `collapseNamespace()` / `expandNamespace()` must call
      `setCollapsedNamespaces()` to persist the collapsed set.
    - On page load, read both values and apply during `init()`.
    - If grouping was ON at last visit, `refresh()` should render in grouped mode
      from the start.
  - **Modifies**:
    - `frontend/src/grouping.js` — ensure write-through on every collapse/expand
    - `frontend/src/main.js` — read state in `init()`

- [ ] **3.2 Preserve collapse state across auto-refresh**
  - **Dependencies**: 3.1
  - **Description**: The `refresh()` → `renderGraph()` cycle currently does a smart diff
    (structure change → full rebuild, data-only → batch update). When grouping is ON:
    - **Structure change** (new/removed nodes): full rebuild, then re-apply collapsed
      state from `collapsedNamespaces` set. New namespaces default to expanded.
    - **Data-only change**: update node data attributes in-place. For collapsed nodes,
      recompute `worstState` and `totalAlerts` from the raw data (not from Cytoscape
      elements, since children are removed). Store latest raw data per namespace.
    - Modify `computeSignature()` to include grouping state.
    - After re-render, restore zoom/pan position.
  - **Modifies**:
    - `frontend/src/graph.js` — `renderGraph()`, `computeSignature()`
    - `frontend/src/grouping.js` — add `reapplyCollapsedState(cy, data)`

- [ ] **3.3 TB/LR toggle interaction**
  - **Dependencies**: None
  - **Description**:
    - When grouping is ON, hide or disable the layout-direction (TB/LR) toggle button
      (fcose is non-directional).
    - When grouping is turned OFF, restore the TB/LR toggle and re-layout with dagre
      in the previously saved direction.
    - Store the last dagre direction so it's restored correctly.
  - **Modifies**:
    - `frontend/src/main.js` — `setupGraphToolbar()` toggle visibility logic

- [ ] **3.4 Filter interaction**
  - **Dependencies**: None
  - **Description**: When a namespace filter is active (`selectedNamespace` is set):
    - If grouping is ON, only one namespace is shown — the compound parent is still
      rendered but collapsing it makes little sense (single group).
    - Approach: keep grouping functional but don't force-disable it. The user sees a
      single namespace group which they can still collapse if they want.
    - Type/state/job filters: `applyFilters()` hides individual nodes. If all children
      of a group are hidden, hide the parent too. Add logic in `applyFilters()` to check
      parent visibility.
  - **Modifies**:
    - `frontend/src/filter.js` — `applyFilters()` parent visibility check

### Completion Criteria — Phase 3

- [ ] All tasks completed (3.1–3.4)
- [ ] Grouping ON/OFF state survives page reload
- [ ] Collapsed namespaces survive auto-refresh (30s cycle)
- [ ] New namespaces appearing during refresh are expanded by default
- [ ] Data-only updates correctly refresh collapsed node summaries (worst state, alert count)
- [ ] TB/LR toggle hidden when grouping is ON, restored when OFF
- [ ] Filters properly interact with compound nodes

---

## Phase 4: Polish, Integration & Deploy

**Dependencies**: Phase 3
**Status**: Pending

### Description

Integrate grouping with sidebar, search, context menu. Ensure dark/light theme
support. Build Docker image and deploy to the test Kubernetes cluster for
end-to-end validation.

### Tasks

- [ ] **4.1 Sidebar for collapsed namespace node**
  - **Dependencies**: None
  - **Description**: When user clicks (single tap) a collapsed namespace node, open
    the sidebar with a namespace summary:
    - Section: namespace name, service count, worst state.
    - List of contained services (names + states), derived from `collapsedData`.
    - Aggregate alert count.
    - Action: "Expand" button that calls `expandNamespace()`.
    - In `sidebar.js`, detect `node.data('isCollapsed')` and render a different template.
  - **Modifies**:
    - `frontend/src/sidebar.js` — add `renderCollapsedSidebar(node)` branch

- [ ] **4.2 Search interaction with collapsed namespaces**
  - **Dependencies**: None
  - **Description**: In `search.js`, when a search matches a node that's inside a
    collapsed namespace:
    - Auto-expand that namespace to reveal the node.
    - Highlight the found node as usual.
    - Call `expandNamespace()` before navigating to the matched node.
    - After search is cleared, re-collapse previously expanded namespaces.
  - **Modifies**:
    - `frontend/src/search.js` — `performSearch()`, `navigateToNext()`
    - `frontend/src/grouping.js` — export `isNodeInCollapsedNs()`, `expandNamespace()`

- [ ] **4.3 Context menu for collapsed namespace node**
  - **Dependencies**: None
  - **Description**: In `contextmenu.js`, add a right-click handler for collapsed nodes:
    - "Expand" — expand the namespace.
    - "Copy namespace name" — copy to clipboard.
    - Selector: `cy.on('cxttap', 'node[?isCollapsed]', ...)`.
  - **Modifies**:
    - `frontend/src/contextmenu.js`
    - `frontend/src/locales/en.js`, `frontend/src/locales/ru.js` — i18n keys

- [ ] **4.4 Dark/light theme support**
  - **Dependencies**: None
  - **Description**: Verify and adjust compound node styles for both themes:
    - Light theme: dashed border in namespace color, subtle background tint.
    - Dark theme: slightly brighter border, adjusted background-opacity.
    - Collapsed node label color should be readable against the state-colored background.
    - Use `updateGraphTheme()` to re-evaluate styles on theme change.
  - **Modifies**:
    - `frontend/src/graph.js` — theme-dependent style functions

- [ ] **4.5 Highlight interaction**
  - **Dependencies**: None
  - **Description**: In `sidebar.js`, the `highlightElement()` function adds a blue
    overlay to connected elements. When grouping is ON and a highlighted element's
    connection leads to a collapsed namespace:
    - Highlight the collapsed node itself (not the hidden children).
    - No auto-expand (would be too disruptive).
  - **Modifies**:
    - `frontend/src/sidebar.js` — adjust `highlightElement()` for collapsed targets

- [ ] **4.6 Build Docker image and deploy to K8s**
  - **Dependencies**: 4.1, 4.2, 4.3, 4.4, 4.5
  - **Description**:
    - Build multi-arch Docker image: `make docker-build TAG=v0.13.0-8`
    - Push to Harbor: `harbor.kryukov.lan/library/dephealth-ui:v0.13.0-8`
    - Update Helm values: `image.tag: v0.13.0-8`
    - Deploy: `helm upgrade dephealth-ui deploy/helm/dephealth-ui -n dephealth-ui`
    - Verify in browser: `https://dephealth.kryukov.lan`
    - Test with real topology data (10+ namespaces if available via uniproxy).
  - **Modifies**:
    - `deploy/helm/dephealth-ui/values.yaml` — image tag

### Completion Criteria — Phase 4

- [ ] All tasks completed (4.1–4.6)
- [ ] Sidebar shows namespace summary for collapsed nodes
- [ ] Search auto-expands collapsed namespaces to find nodes
- [ ] Context menu on collapsed nodes works (Expand, Copy name)
- [ ] Graph looks correct in both dark and light themes
- [ ] Highlight doesn't break on collapsed namespaces
- [ ] Docker image built and deployed to test cluster
- [ ] End-to-end verification with real data
- [ ] No regressions in existing features

---

## Architecture Summary

```
                     ┌──────────────┐
                     │  index.html  │  Toolbar buttons:
                     │              │  [Group] [Collapse All]
                     └──────┬───────┘
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
        ┌──────────┐  ┌──────────┐  ┌──────────────┐
        │ main.js  │  │ graph.js │  │ grouping.js  │  NEW
        │          │  │          │  │              │
        │ refresh()│──│ render() │──│ buildCompound│
        │ toolbar  │  │ styles   │  │ collapse()   │
        │ init()   │  │ layout   │  │ expand()     │
        └──────────┘  │ dagre/   │  │ localStorage │
                      │ fcose    │  └──────────────┘
                      └──────────┘
                            │
              ┌─────────────┼──────────────┐
              ▼             ▼              ▼
        ┌──────────┐  ┌──────────┐  ┌────────────┐
        │sidebar.js│  │ search.js│  │contextmenu │
        │          │  │          │  │            │
        │ collapsed│  │ auto-    │  │ Expand     │
        │ summary  │  │ expand   │  │ Copy name  │
        └──────────┘  └──────────┘  └────────────┘
```

## File Change Matrix

| File | Phase 1 | Phase 2 | Phase 3 | Phase 4 |
|------|---------|---------|---------|---------|
| `frontend/package.json` | 1.1 | | | |
| `frontend/src/graph.js` | 1.1, 1.3, 1.5 | 2.5 | 3.2 | 4.4 |
| `frontend/src/grouping.js` | 1.2 (new) | 2.1–2.4, 2.6 | 3.1, 3.2 | 4.2 |
| `frontend/src/main.js` | 1.4 | 2.6 | 3.1, 3.3 | |
| `frontend/index.html` | 1.4 | 2.6 | | |
| `frontend/src/locales/en.js` | 1.4 | 2.6 | | 4.3 |
| `frontend/src/locales/ru.js` | 1.4 | 2.6 | | 4.3 |
| `frontend/src/sidebar.js` | | 2.4 | | 4.1, 4.5 |
| `frontend/src/search.js` | | | | 4.2 |
| `frontend/src/contextmenu.js` | | | | 4.3 |
| `frontend/src/filter.js` | | | 3.4 | |
| `frontend/src/style.css` | 1.4 | | | |
| `deploy/helm/*/values.yaml` | | | | 4.6 |

## Notes

- **cytoscape-fcose**: actively maintained fork of CoSE Bilkent, MIT license, supports
  compound nodes, ~15KB gzipped. GitHub: `iVis-at-Bilkent/cytoscape.js-fcose`.
- **No deprecated dependencies**: collapse/expand implemented manually, not via the
  unmaintained `cytoscape-expand-collapse` extension.
- **Edge case — orphan nodes**: nodes without namespace (external deps) are rendered as
  standalone nodes outside any group. They participate in edges normally.
- **Aggregate edges**: when two collapsed namespaces have cross-edges, a single aggregate
  edge with `×K` label is shown. State color = worst of aggregated edges.
- **Performance target**: fcose with ~100 nodes + ~15 parent nodes should layout in < 2s.
  The `quality: 'default'` option balances speed and aesthetics.
