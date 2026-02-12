# Plan: Cascade Warnings

## Metadata

- **Version**: 1.0.0
- **Created**: 2026-02-12
- **Last updated**: 2026-02-12
- **Status**: Complete
- **Requirements**: [.tasks/ui-requirements.md](../.tasks/ui-requirements.md)

---

## History

- **v1.0.0** (2026-02-12): Initial plan

---

## Current Status

- **Active phase**: Complete
- **Active item**: N/A
- **Last updated**: 2026-02-12
- **Note**: All phases complete. State model refined during E2E: stale→"down", critical dep failure→"degraded"

---

## Table of Contents

- [x] [Phase 1: Backend — Critical-Aware State Logic](#phase-1-backend--critical-aware-state-logic)
- [x] [Phase 2: Frontend — Cascade Computation Engine](#phase-2-frontend--cascade-computation-engine)
- [x] [Phase 3: Frontend — Warning Badge Rendering](#phase-3-frontend--warning-badge-rendering)
- [x] [Phase 4: Frontend — Filter, Legend & Localization](#phase-4-frontend--filter-legend--localization)
- [x] [Phase 5: Build, Deploy & Test](#phase-5-build-deploy--test)

---

## Phase 1: Backend — Critical-Aware State Logic

**Dependencies**: None
**Status**: Pending

### Description

Change the node state computation in Go backend to account for the `critical` flag on edges.
Currently `calcNodeState()` in `internal/topology/graph.go:329` accepts `[]float64` (raw health
values) and returns state based on all-healthy / all-down / mixed logic. The new logic must
distinguish critical vs non-critical edges and apply different rules.

**New state rules (service nodes with outgoing edges):**

| Has critical edges? | Condition | Result |
|---|---|---|
| Yes | Any critical edge down (health=0) | **down** |
| Yes | All critical up, some non-critical down | **degraded** |
| Yes | All edges up | **ok** |
| No | Any condition | **ok** |

Stale/unknown handling remains unchanged. Alert overrides (lines 467-478) continue to work
as before — they modify edge health values which are then fed into the new calculation.

### Items

- [ ] **1.1 Refactor `calcNodeState` signature and logic**
  - **Dependencies**: None
  - **Description**:
    1. Create a new struct type `edgeHealthInfo` with fields `Health float64` and `Critical bool`
    2. Change `calcNodeState(healthValues []float64) string` → `calcNodeState(edges []edgeHealthInfo) string`
    3. Implement the new logic:
       - Partition edges into critical and non-critical groups
       - If critical group is empty → return "ok"
       - If any critical edge has health=0 → return "down"
       - If all critical edges healthy but any non-critical has health=0 → return "degraded"
       - Otherwise → return "ok"
    4. Handle stale edges (health=-1): exclude from calculation (existing behavior)
  - **Modifies**:
    - `internal/topology/graph.go` — `calcNodeState()` function and `edgeHealthInfo` type
  - **Links**:
    - [Current calcNodeState](../internal/topology/graph.go) (line 329)

- [ ] **1.2 Update callers of `calcNodeState`**
  - **Dependencies**: 1.1
  - **Description**:
    1. In `buildGraph()` (graph.go ~line 282): where service node state is computed from
       outgoing edges — construct `[]edgeHealthInfo` with both health and critical flag
    2. In `buildGraph()` (~line 294): where dependency node state is computed from incoming
       edges — same treatment (dependency nodes use incoming edge health)
    3. In alert override section (~line 509): where node state is recalculated after alert
       processing — pass critical flag alongside health values
    4. Verify that stale-node retention (lookback) logic still works correctly
  - **Modifies**:
    - `internal/topology/graph.go` — `buildGraph()` function, alert processing section
  - **Links**:
    - [buildGraph function](../internal/topology/graph.go) (line ~220)

- [ ] **1.3 Unit tests for new state logic**
  - **Dependencies**: 1.1
  - **Description**:
    Write table-driven tests for the new `calcNodeState` covering:
    - All critical edges up → ok
    - One critical edge down → down
    - All critical up, one non-critical down → degraded
    - No critical edges at all → ok (even if non-critical are down)
    - Empty edges → unknown
    - All edges stale → unknown
    - Mixed stale + critical down → down
    - Alert override scenarios
  - **Modifies**:
    - `internal/topology/graph_test.go`
  - **Links**: N/A

### Completion Criteria Phase 1

- [ ] All items completed (1.1, 1.2, 1.3)
- [ ] `go test ./internal/topology/...` passes
- [ ] `go vet ./...` clean
- [ ] No regressions in existing state computation for edges without critical flag changes

---

## Phase 2: Frontend — Cascade Computation Engine

**Dependencies**: Phase 1
**Status**: Pending

### Description

Create a frontend module that computes cascade warnings by traversing the Cytoscape graph
upward from Down nodes through critical edges only. This runs on every data refresh (15s poll
cycle) after `renderGraph()` updates node data but before `applyFilters()`.

**Algorithm (custom BFS, not `predecessors()`):**
```
for each node where state == "down":
  queue = [node]
  visited = Set()
  while queue not empty:
    current = queue.shift()
    for each incoming edge of current:
      if edge.data('critical') == true:
        sourceNode = edge.source()
        if sourceNode not in visited AND sourceNode.data('state') != "down":
          visited.add(sourceNode)
          sourceNode.cascadeSources.add(originalDownNode.id)
          queue.push(sourceNode)  // continue propagation
```

Cannot use `predecessors()` because it traverses ALL upstream paths regardless of edge
criticality. Custom BFS gives us control to stop at non-critical edges.

**Performance:** For 300 nodes, BFS is O(V+E) ≈ sub-millisecond. Runs inside `cy.batch()`.

### Items

- [ ] **2.1 Create `cascade.js` module**
  - **Dependencies**: None
  - **Description**:
    1. Create `frontend/src/cascade.js` with exported function `computeCascadeWarnings(cy)`
    2. Implement the custom BFS algorithm described above
    3. Store results as Cytoscape node data:
       - `node.data('cascadeCount', N)` — number of distinct root-cause Down services
       - `node.data('cascadeSources', ['svc-a', 'svc-b'])` — list of root-cause node IDs
    4. Clear previous cascade data before recomputation (reset all nodes to cascadeCount=0)
    5. Skip nodes that are themselves "down" (they don't need a warning — already red)
    6. Wrap mutations in `cy.batch()` for performance
    7. Export helper `hasCascadeWarning(node)` → returns boolean
  - **Creates**:
    - `frontend/src/cascade.js`
  - **Links**:
    - [Cytoscape traversal docs](https://js.cytoscape.org/#collection/traversing)
    - [Memory: cytoscape-patterns](../.serena/memories/cytoscape-patterns.md)

- [ ] **2.2 Integrate cascade computation into refresh cycle**
  - **Dependencies**: 2.1
  - **Description**:
    1. In `frontend/src/main.js`, import `computeCascadeWarnings` from `cascade.js`
    2. In `refresh()` function (~line 194), call `computeCascadeWarnings(cy)` after
       `renderGraph()` and before `applyFilters(cy)`:
       ```javascript
       const structureChanged = renderGraph(cy, data, appConfig);
       // ... existing code (reapplyCollapsedState, updateStatus, etc.)
       computeCascadeWarnings(cy);  // ← NEW
       applyFilters(cy);
       ```
    3. Also call it in the initial load path (after first renderGraph)
  - **Modifies**:
    - `frontend/src/main.js` — `refresh()` function, initial load
  - **Links**: N/A

### Completion Criteria Phase 2

- [ ] All items completed (2.1, 2.2)
- [ ] Cascade data (`cascadeCount`, `cascadeSources`) is set on nodes after each refresh
- [ ] Down nodes do NOT have cascade markers on themselves
- [ ] Non-critical edges do NOT propagate cascade
- [ ] No visible UI lag during cascade computation

---

## Phase 3: Frontend — Warning Badge Rendering

**Dependencies**: Phase 2
**Status**: Pending

### Description

Add a visual ⚠ badge with counter on nodes that have cascade warnings. Uses the existing
HTML overlay badge system (`alert-badge-container`). The cascade badge should be positioned
at **top-left** of the node to avoid overlap with the existing alert badge (top-right).

### Items

- [ ] **3.1 Add cascade badge rendering**
  - **Dependencies**: None
  - **Description**:
    1. In `frontend/src/graph.js`, extend `updateAlertBadges()` function (or create a
       parallel `updateCascadeBadges()` called from the same render/pan/zoom handler)
    2. For each node where `node.data('cascadeCount') > 0` AND `isElementVisible(node)`:
       - Create a badge element with class `cascade-badge`
       - Content: `⚠ N` (where N = cascadeCount)
       - Position: **top-left** corner of node (mirror of alert badge positioning):
         ```javascript
         badgeX = renderedX - renderedWidth/2 + 10;
         badgeY = renderedY - renderedHeight/2 + 10;
         ```
    3. Style: orange/amber background (#ff9800), white text, same 20px size as alert badges
    4. Respect `isElementVisible()` — hide badge if node is filtered/searched out
    5. Badge must update on render/pan/zoom events (attach to same handler as alert badges)
  - **Modifies**:
    - `frontend/src/graph.js` — badge rendering section
  - **Links**:
    - [Current alert badge code](../frontend/src/graph.js) (lines 256-336)

- [ ] **3.2 Add cascade badge CSS**
  - **Dependencies**: 3.1
  - **Description**:
    Add CSS for `.cascade-badge` in `frontend/src/style.css`:
    - Match alert badge sizing (20x20px circle)
    - Background: `#ff9800` (amber/orange — warning color)
    - Color: white, font-size: 10px, font-weight: bold
    - pointer-events: none, z-index: 10
    - Optional: add subtle pulse/glow animation to draw attention
  - **Modifies**:
    - `frontend/src/style.css`
  - **Links**: N/A

- [ ] **3.3 Add tooltip on cascade badge hover**
  - **Dependencies**: 3.1
  - **Description**:
    When hovering over a node with cascade warning, show the list of root-cause Down
    services in the tooltip. Extend `frontend/src/tooltip.js`:
    1. Check if node has `cascadeSources` data
    2. If yes, add a section to the tooltip:
       ```
       Cascade warning:
       ↳ service-a (Down)
       ↳ service-b (Down)
       ```
    3. Use localized labels from i18n
  - **Modifies**:
    - `frontend/src/tooltip.js`
  - **Links**: N/A

### Completion Criteria Phase 3

- [ ] All items completed (3.1, 3.2, 3.3)
- [ ] ⚠ badge visible on nodes with cascade warnings
- [ ] Badge shows correct count of root-cause Down services
- [ ] Badge does NOT overlap with alert badge (different corner)
- [ ] Badge hidden when node is filtered out
- [ ] Tooltip shows list of root-cause services on hover

---

## Phase 4: Frontend — Filter, Legend & Localization

**Dependencies**: Phase 3
**Status**: Pending

### Description

Add WARNING to the STATE filter system, update the legend, and add all necessary
localization keys.

### Items

- [ ] **4.1 Extend STATE filter with WARNING option**
  - **Dependencies**: None
  - **Description**:
    1. In `frontend/src/filter.js`, the STATES array is `['ok', 'degraded', 'down', 'unknown']`.
       Add `'warning'` to this list.
    2. In `applyFilters(cy)`, the state matching logic checks `node.data('state')`. The
       'warning' filter is different — it's NOT a backend state but a frontend-computed
       overlay. Add special handling:
       ```javascript
       // Node matches state filter if:
       // - Its backend state matches an active state filter, OR
       // - It has cascadeCount > 0 AND 'warning' is in active state filters
       ```
    3. A state chip for "warning" needs an appropriate color — use amber (#ff9800) to match
       the cascade badge color
    4. Filter logic: WARNING works with OR semantics alongside other states (consistent with
       existing behavior where selecting multiple states shows all matching)
  - **Modifies**:
    - `frontend/src/filter.js` — STATES array, `applyFilters()`, `renderStateChips()`
  - **Links**: N/A

- [ ] **4.2 Update legend**
  - **Dependencies**: None
  - **Description**:
    1. In `frontend/index.html`, add a legend entry for the cascade warning badge
    2. Visual: small ⚠ icon + text "Cascade warning" / "Каскадное предупреждение"
    3. Place it near existing alert badge legend entries
  - **Modifies**:
    - `frontend/index.html` — legend section
  - **Links**: N/A

- [ ] **4.3 Localization keys**
  - **Dependencies**: None
  - **Description**:
    Add translation keys to both locale files:

    **English** (`frontend/src/locales/en.js`):
    ```javascript
    'state.warning': 'Warning',
    'tooltip.cascadeWarning': 'Cascade warning:',
    'tooltip.cascadeSource': '↳ {service} (Down)',
    'legend.cascadeWarning': 'Cascade warning',
    ```

    **Russian** (`frontend/src/locales/ru.js`):
    ```javascript
    'state.warning': 'Внимание',
    'tooltip.cascadeWarning': 'Каскадное предупреждение:',
    'tooltip.cascadeSource': '↳ {service} (Недоступен)',
    'legend.cascadeWarning': 'Каскадное предупреждение',
    ```
  - **Modifies**:
    - `frontend/src/locales/en.js`
    - `frontend/src/locales/ru.js`
  - **Links**: N/A

### Completion Criteria Phase 4

- [ ] All items completed (4.1, 4.2, 4.3)
- [ ] WARNING chip appears in STATE filter panel
- [ ] Selecting WARNING shows only nodes with cascade warnings
- [ ] WARNING + DOWN together shows both Down nodes and warned nodes
- [ ] Legend shows cascade warning badge explanation
- [ ] All texts localized in EN and RU

---

## Phase 5: Build, Deploy & Test

**Dependencies**: Phase 1, Phase 2, Phase 3, Phase 4
**Status**: Pending

### Description

Build the Docker image, deploy to test cluster, and validate the cascade warning feature
end-to-end with the test microservices.

### Items

- [ ] **5.1 Run Go tests**
  - **Dependencies**: None
  - **Description**:
    1. Run `go test ./internal/topology/...` — verify new calcNodeState logic
    2. Run `go vet ./...` — check for issues
    3. Fix any test failures
  - **Creates**:
    - Test results
  - **Links**: N/A

- [ ] **5.2 Build Docker image**
  - **Dependencies**: 5.1
  - **Description**:
    1. Determine image tag (next development version, e.g. `v0.13.0-1`)
    2. Build multi-arch image: `make docker-build TAG=v0.13.0-1`
    3. Push to Harbor: `make docker-push TAG=v0.13.0-1`
  - **Creates**:
    - Docker image `harbor.kryukov.lan/library/dephealth-ui:v0.13.0-1`
  - **Links**: N/A

- [ ] **5.3 Deploy to test cluster**
  - **Dependencies**: 5.2
  - **Description**:
    1. Update Helm values with new image tag
    2. Deploy: `make deploy TAG=v0.13.0-1`
    3. Verify pods are running: `make env-status`
  - **Creates**:
    - Running deployment in test cluster
  - **Links**: N/A

- [ ] **5.4 End-to-end validation**
  - **Dependencies**: 5.3
  - **Description**:
    Manual and/or Playwright-based testing:
    1. **State logic**: Stop a critical dependency in test services → verify service shows
       as Down (not Degraded)
    2. **State logic (no critical)**: Stop a non-critical dependency → verify service stays OK
    3. **Cascade propagation**: Verify ⚠ badges appear on upstream services connected
       through critical edges
    4. **Counter**: Verify badge counter shows correct number of root-cause services
    5. **Non-critical path**: Verify cascade does NOT propagate through non-critical edges
    6. **Recovery**: Restart the stopped service → verify ⚠ badges disappear
    7. **Filter**: Toggle WARNING filter → verify correct filtering
    8. **Tooltip**: Hover over warned node → verify root-cause service list
    9. **Legend**: Verify cascade warning entry in legend
    10. **Localization**: Switch language → verify all new strings
  - **Creates**:
    - Test results / screenshots
  - **Links**: N/A

### Completion Criteria Phase 5

- [ ] All items completed (5.1, 5.2, 5.3, 5.4)
- [ ] Go tests pass
- [ ] Docker image built and pushed
- [ ] Feature works correctly in test cluster
- [ ] All 10 validation scenarios pass
- [ ] No regressions in existing functionality

---

## Notes

- **Image tag**: Current release is v0.13.0 (from CHANGELOG). Development builds for this
  feature should use `v0.13.0-N` pattern. Final release will be v0.14.0 (new feature = minor bump,
  pending user approval).
- **Cytoscape traversal**: Cannot use `predecessors()` for cascade — it follows ALL upstream
  paths. Custom BFS required for critical-edge-only traversal.
- **Badge collision**: Alert badges (top-right) vs cascade badges (top-left) — different
  corners to avoid overlap.
- **Open questions from requirements** (resolve during implementation):
  1. Filter OR logic for WARNING + other states → plan assumes OR (consistent with existing)
  2. Legend update → included in Phase 4
  3. Tooltip on warning badge → included in Phase 3

---

**Plan ready for implementation. Use `/sc:implement` to execute phase by phase.**
