# Plan: UI Improvements (Edge Sidebar, Bug Fixes)

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-02-11
- **Last updated**: 2026-02-11
- **Status**: Pending
- **Requirements**: [.tasks/ui-requirements.md](../.tasks/ui-requirements.md)

---

## Version History

- **v1.0.0** (2026-02-11): Initial plan

---

## Current Status

- **Active phase**: Phase 4
- **Active step**: 4.1
- **Last updated**: 2026-02-11
- **Note**: Phase 1 + Phase 2 + Phase 3 complete, ready for build

---

## Table of Contents

- [x] [Phase 1: Quick Fixes (CSS scroll + drawer/sidebar bug)](#phase-1-quick-fixes)
- [x] [Phase 2: Edge Detail Sidebar](#phase-2-edge-detail-sidebar)
- [x] [Phase 3: Clickable Edges in Sidebar List](#phase-3-clickable-edges-in-sidebar-list)
- [ ] [Phase 4: Build, Deploy & Verify](#phase-4-build-deploy--verify)

---

## Phase 1: Quick Fixes

**Dependencies**: None
**Status**: Pending

### Description

Two independent, low-risk fixes: horizontal scroll for the instances table and the alert-drawer/sidebar coexistence bug.

### Steps

- [ ] **1.1 Horizontal scroll for instances table**
  - **Dependencies**: None
  - **Description**: Add `overflow-x: auto` to `.sidebar-instances-table` in CSS so that the instances table scrolls horizontally when content overflows the sidebar width.
  - **Files to modify**:
    - `frontend/src/style.css` — `.sidebar-instances-table` rule (line ~1564)
  - **Change detail**:
    ```css
    .sidebar-instances-table {
      margin-top: 8px;
      overflow-x: auto;  /* ADD */
    }
    ```

- [ ] **1.2 Fix alert drawer closing node sidebar**
  - **Dependencies**: None
  - **Description**: The root cause is the "click outside" handler in `sidebar.js:56-64`. When a user clicks the alert toggle button (`#btn-alerts`) or anywhere in the alert drawer, the click event fires on an element that is neither inside `#node-sidebar` nor inside `#cy`, so the sidebar's click-outside handler closes it. Fix: add `#alert-drawer` and `#btn-alerts` to the exclusion check.
  - **Files to modify**:
    - `frontend/src/sidebar.js` — `document.addEventListener('click', ...)` handler (line ~56)
  - **Change detail**: Update the condition from:
    ```javascript
    if (
      !sidebar.classList.contains('hidden') &&
      !sidebar.contains(e.target) &&
      !e.target.closest('#cy')
    ) {
      closeSidebar();
    }
    ```
    to:
    ```javascript
    if (
      !sidebar.classList.contains('hidden') &&
      !sidebar.contains(e.target) &&
      !e.target.closest('#cy') &&
      !e.target.closest('#alert-drawer') &&
      !e.target.closest('#btn-alerts')
    ) {
      closeSidebar();
    }
    ```
  - **Rationale**: Clicks on the alert drawer or its toggle button should not be treated as "outside" the sidebar. The same pattern should extend to any future toolbar buttons.

- [ ] **1.3 Escape closes both panels**
  - **Dependencies**: 1.2
  - **Description**: Verify that the existing `closeAll` in `main.js` (triggered by Escape via `shortcuts.js`) already closes both panels simultaneously. This should already work — the `closeAll` function hides both `#node-sidebar` and `#alert-drawer`. If it works, mark as done. If not, ensure both are closed.
  - **Files to check**:
    - `frontend/src/main.js` — `closeAll` handler
    - `frontend/src/shortcuts.js` — Escape binding

### Completion Criteria Phase 1

- [ ] Steps 1.1, 1.2, 1.3 completed
- [ ] Instances table scrolls horizontally on narrow sidebar
- [ ] Opening alert drawer does NOT close node sidebar
- [ ] Both panels open simultaneously without layout overlap
- [ ] Escape closes both panels at once

---

## Phase 2: Edge Detail Sidebar

**Dependencies**: Phase 1
**Status**: Pending

### Description

Add edge detail view to the right sidebar. When a user clicks an edge (connection line), the sidebar shows detailed information: source/target, state, type, latency, alerts, Grafana links. Reuses the same `#node-sidebar` DOM element.

### Steps

- [ ] **2.1 Pass missing edge fields to Cytoscape**
  - **Dependencies**: None
  - **Description**: Currently `graph.js` does not copy `type`, `latencyRaw`, `health` from API data to Cytoscape edge elements. Add these fields so they're available for the sidebar.
  - **Files to modify**:
    - `frontend/src/graph.js` — `renderGraph()` edge `cy.add()` block (~line 397) and `smartDiffUpdate()` edge update block (~line 357)
  - **Change detail**: Add to edge data object:
    ```javascript
    type: edge.type || undefined,
    latencyRaw: edge.latencyRaw || 0,
    health: edge.health ?? -1,
    ```

- [ ] **2.2 Add i18n keys for edge sidebar**
  - **Dependencies**: None
  - **Description**: Add translation keys for edge sidebar sections and labels. Follow existing `sidebar.*` / `sidebar.edge.*` naming convention.
  - **Files to modify**:
    - `frontend/src/locales/en.js`
    - `frontend/src/locales/ru.js`
  - **New keys**:
    | Key | EN | RU |
    |-----|----|----|
    | `sidebar.edge.source` | Source | Источник |
    | `sidebar.edge.target` | Target | Назначение |
    | `sidebar.edge.type` | Type | Тип |
    | `sidebar.edge.latency` | Latency | Задержка |
    | `sidebar.edge.health` | Health | Здоровье |
    | `sidebar.edge.critical` | Critical | Критичная |
    | `sidebar.edge.criticalYes` | Yes | Да |
    | `sidebar.edge.criticalNo` | No | Нет |
    | `sidebar.edge.goToNode` | Go to node | Перейти к узлу |

- [ ] **2.3 Implement `openEdgeSidebar()` function**
  - **Dependencies**: 2.1, 2.2
  - **Description**: Create the main function to populate the sidebar with edge data. This function will reuse the same `#node-sidebar` container but render edge-specific content in each section.
  - **Files to modify**:
    - `frontend/src/sidebar.js` — add `openEdgeSidebar(edge, cy)` function, export it
  - **Implementation**:
    - **Title**: `sourceLabel → targetLabel` (get labels from `cy.getElementById(source/target).data('label')`)
    - **Details section** (`#sidebar-details`): State badge, type, latency, health (formatted), critical flag, stale hint
    - **Alerts section** (`#sidebar-alerts`): Filter alerts where `alert.service === source && alert.dependency === target` (or match by edge source/target)
    - **Instances section** (`#sidebar-instances`): Empty (edges have no instances) — clear the section
    - **Edges section** (`#sidebar-edges`): Replace with **Source/Target node links** — two clickable items:
      - "Source: {sourceLabel}" → click calls `openSidebar(sourceNode, cy)` + centers node
      - "Target: {targetLabel}" → click calls `openSidebar(targetNode, cy)` + centers node
    - **Actions section** (`#sidebar-actions`): "Open in Grafana" button if `grafanaUrl` exists
    - **Grafana section** (`#sidebar-grafana`): Context-aware dashboard links (linkStatus with edge variables)
  - **Toggle behavior**: Track `currentEdgeId` (like `currentNodeId`). If same edge clicked → close. If different element clicked → switch view.

- [ ] **2.4 Wire edge tap event**
  - **Dependencies**: 2.3
  - **Description**: Replace the current `cy.on('tap', 'edge[grafanaUrl]', ...)` handler (which opens Grafana directly) with a handler that opens the edge sidebar.
  - **Files to modify**:
    - `frontend/src/sidebar.js` — `initSidebar()` function
  - **Change detail**:
    - Remove: `cy.on('tap', 'edge[grafanaUrl]', ...)` (line ~47-50)
    - Add: `cy.on('tap', 'edge', ...)` — calls `openEdgeSidebar(edge, cy)` with toggle behavior
    - Update click-outside handler to also exclude edge sidebar clicks

- [ ] **2.5 Add CSS for edge sidebar elements**
  - **Dependencies**: 2.3
  - **Description**: Add CSS styles for edge-specific sidebar elements: node link items (clickable), edge detail rows.
  - **Files to modify**:
    - `frontend/src/style.css`
  - **New styles**:
    - `.sidebar-node-link` — clickable source/target node links with hover effect, cursor pointer
    - Reuse existing `.sidebar-detail-row`, `.sidebar-state-badge`, `.sidebar-button` classes

### Completion Criteria Phase 2

- [ ] Steps 2.1–2.5 completed
- [ ] Clicking an edge opens sidebar with all edge data
- [ ] Toggle: click same edge again → close sidebar
- [ ] Switching between node/edge sidebar works seamlessly
- [ ] Source/Target links in edge sidebar → open node sidebar + center node
- [ ] Stale edges show `state=unknown` with stale hint
- [ ] Grafana button/links appear only when data exists
- [ ] All labels translated EN + RU
- [ ] No regressions in node sidebar functionality

---

## Phase 3: Clickable Edges in Sidebar List

**Dependencies**: Phase 2
**Status**: Pending

### Description

In the node sidebar's "Connected Edges" section, make each edge item clickable. Clicking navigates to the edge: centers it on the graph and opens its edge sidebar.

### Steps

- [ ] **3.1 Make edge items clickable**
  - **Dependencies**: None
  - **Description**: Update `renderEdges()` in `sidebar.js` to store the edge ID in a data attribute and attach click handlers to each edge item.
  - **Files to modify**:
    - `frontend/src/sidebar.js` — `renderEdges()` function
  - **Change detail**:
    - Add `data-edge-id="${edgeId}"` to each `.sidebar-edge-item` div
    - After rendering HTML, attach click listeners:
      ```javascript
      section.querySelectorAll('.sidebar-edge-item[data-edge-id]').forEach(el => {
        el.addEventListener('click', () => {
          const edgeId = el.dataset.edgeId;
          const edge = cy.getElementById(edgeId);
          if (edge && edge.length) {
            // Center edge on graph with animation
            cy.animate({ center: { eles: edge }, duration: 300 });
            // Open edge sidebar
            openEdgeSidebar(edge, cy);
            // Flash highlight
            highlightEdge(edge);
          }
        });
      });
      ```

- [ ] **3.2 Add edge highlight animation**
  - **Dependencies**: 3.1
  - **Description**: Add a brief flash/highlight on the clicked edge for visual feedback. Use Cytoscape's animation API or a CSS class toggle.
  - **Files to modify**:
    - `frontend/src/sidebar.js` — add `highlightEdge(edge)` helper
    - `frontend/src/style.css` — optional keyframes for edge highlight
  - **Implementation**:
    ```javascript
    function highlightEdge(edge) {
      const origWidth = edge.style('width');
      edge.animate({
        style: { 'width': 6, 'line-color': '#2196f3' },
        duration: 200,
      }).animate({
        style: { 'width': origWidth, 'line-color': null },
        duration: 400,
      });
    }
    ```

- [ ] **3.3 CSS for clickable edge items**
  - **Dependencies**: None
  - **Description**: Add `cursor: pointer` and hover styles to `.sidebar-edge-item` elements.
  - **Files to modify**:
    - `frontend/src/style.css`
  - **Change detail**:
    ```css
    .sidebar-edge-item {
      cursor: pointer;
    }
    .sidebar-edge-item:hover {
      background: var(--bg-hover);
    }
    ```

### Completion Criteria Phase 3

- [ ] Steps 3.1–3.3 completed
- [ ] Edge items in sidebar show pointer cursor on hover
- [ ] Click → sidebar updates with edge details
- [ ] Click → graph pans/zooms to center the edge (animated)
- [ ] Edge flashes briefly in the graph for visual feedback
- [ ] No regressions in node sidebar or edge sidebar

---

## Phase 4: Build, Deploy & Verify

**Dependencies**: Phase 1, Phase 2, Phase 3
**Status**: Pending

### Description

Build Docker image, deploy to Kubernetes, and manually verify all changes in the live environment.

### Steps

- [ ] **4.1 Frontend lint check**
  - **Dependencies**: None
  - **Description**: Run linters on modified frontend files to catch any issues.
  - **Command**: `make lint` (or run eslint on frontend/src/)

- [ ] **4.2 Build Docker image**
  - **Dependencies**: 4.1
  - **Description**: Build multi-arch Docker image with dev tag.
  - **Command**: `make docker-build TAG=v0.13.0-1`
  - **Creates**: Docker image `harbor.kryukov.lan/library/dephealth-ui:v0.13.0-1`

- [ ] **4.3 Deploy to Kubernetes**
  - **Dependencies**: 4.2
  - **Description**: Deploy the new image to the test cluster via Helm.
  - **Command**: `make helm-deploy`

- [ ] **4.4 Manual verification**
  - **Dependencies**: 4.3
  - **Description**: Open `https://dephealth.kryukov.lan` and verify all acceptance criteria:
    1. Instances table scrolls horizontally
    2. Alert drawer and node sidebar coexist
    3. Clicking an edge opens edge sidebar with all data
    4. Toggle behavior for edges works
    5. Source/target links navigate to node sidebar
    6. Clicking edge in Connected Edges list → centers + opens edge sidebar
    7. Edge highlight animation visible
    8. Escape closes both panels
    9. No regressions in existing node sidebar / context menu

### Completion Criteria Phase 4

- [ ] All steps completed
- [ ] All acceptance criteria verified in live K8s environment
- [ ] No visual regressions
- [ ] Ready for commit

---

## Notes

- **Image version**: Use `v0.13.0-N` dev tags for this feature set. Final release as `v0.13.0`.
- **Root cause of drawer/sidebar bug**: Click-outside handler in `sidebar.js` treats alert drawer clicks as "outside" — fixed by adding exclusion checks.
- **Cytoscape edge fields gap**: `type`, `latencyRaw`, `health` not passed to Cytoscape elements — fixed in Phase 2.
- **Reuse principle**: Edge sidebar reuses the same `#node-sidebar` DOM element and as many existing CSS classes as possible.
