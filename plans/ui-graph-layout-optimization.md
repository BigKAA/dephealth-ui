# Plan: Graph Layout Optimization — ELK + Position Persistence

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-03-03
- **Last updated**: 2026-03-03
- **Status**: Pending

---

## Version History

- **v1.0.0** (2026-03-03): Initial requirements specification (brainstorm result)

---

## Current Status

- **Active phase**: Not started
- **Active item**: N/A
- **Last updated**: 2026-03-03
- **Note**: Requirements specification complete, ready for design/implementation planning

---

## Problem Statement

The graph renders chaotically on startup — especially in grouped mode (fCoSE), where nodes overlap and connections are hard to follow. Users have to manually rearrange nodes every time. Manual positions are lost on data refresh and page reload.

**Current behavior** (fCoSE with groups): nodes placed by force-directed algorithm, no layering, no flow direction, random-looking result each time.

**Desired behavior**: hierarchical layered layout — entry points at top, dependencies flowing downward in layers, compound nodes (namespace groups) properly contained.

---

## Requirements

### Functional Requirements

**FR-1: Hierarchical layout with ELK**
- Replace fCoSE with ELK (Eclipse Layout Kernel) for all modes (flat and grouped)
- Flow direction: top-to-bottom (TB), with support for LR toggle (existing direction switch)
- Native compound node support (namespace groups render correctly)
- Entry points (`isEntry=true` from data OR source nodes with no incoming edges) placed on top layer
- Nodes arranged in layers by dependency depth
- New dependency: `cytoscape-elk` + `elkjs`

**FR-2: Manual position persistence**
- When a node is manually dragged — mark it as `manuallyPositioned`
- On layout recalculation: manually positioned nodes stay locked, others are recalculated
- Save positions to `localStorage` (key: `dephealth-node-positions`)
- Format: `{ [nodeId]: { x: number, y: number, manual: boolean } }`
- Save triggers: on node drag end, debounced

**FR-3: Incremental layout on topology changes**
- New nodes: auto-positioned by ELK near their graph neighbors
- Removed nodes: cleaned from saved positions
- Existing nodes with saved positions: restored from localStorage (manual ones locked)

**FR-4: Reset layout button**
- Clears all saved positions from localStorage
- Recalculates full ELK layout from scratch
- Placed in toolbar (near existing direction toggle)
- No confirmation dialog required

**FR-5: Position restore on startup**
- On app load: check localStorage for saved positions
- If found: use `preset` layout with saved coordinates for known nodes
- For nodes without saved positions: run ELK to position them
- Zoom/pan NOT saved (use cy.fit() as before)

### Non-Functional Requirements

- **NFR-1**: Layout calculation <500ms for graphs up to 50 nodes
- **NFR-2**: No regression in existing features (focus mode, grouping, collapse/expand, search, drag, export)
- **NFR-3**: Additional dependency `elkjs` (~200KB) is acceptable

### User Stories

**US-1**: As a user, I want to see a readable hierarchical graph immediately after loading, so I don't need to manually arrange nodes.
- AC: Entry points (isEntry) and source nodes are on the top layer
- AC: Dependencies are arranged in layers below, following edge direction
- AC: Compound nodes (namespace groups) are correctly displayed

**US-2**: As a user, I want to manually move nodes and have their positions preserved across data updates and page reloads.
- AC: After moving a node — its position is saved
- AC: On data update (polling) — manual positions are not reset
- AC: After page reload — nodes are at their saved positions

**US-3**: As a user, I want to reset layout to automatic if my manual arrangement becomes inconvenient.
- AC: "Reset layout" button clears all manual positions
- AC: Graph is recalculated by ELK from scratch

**US-4**: As a user, I want new nodes to automatically appear in a logical position without breaking my arrangement.
- AC: Existing nodes stay at their positions
- AC: New nodes are placed by algorithm near their graph neighbors

---

## Technical Context

### Current Layout Implementation
- **File**: `frontend/src/graph.js` — `buildLayoutConfig()` (line ~553)
- Flat graph: Dagre (TB/LR), grouped: fCoSE
- Layout runs on structural change only (smart diffing in `renderGraph()`)

### Data Model
- `Node.IsEntry` (bool) — already exists in backend (`internal/topology/models.go:17`)
- Frontend receives `isEntry` field in topology API response
- Entry badge "⬇" already rendered for isEntry nodes

### Existing Position-Related Code
- Layout direction saved: `dephealth-layout-direction` in localStorage
- Collapsed namespaces saved: `dephealth-collapsed-ns` in localStorage
- Grouping state saved: `dephealth-grouping` in localStorage
- Node drag: `frontend/src/node-drag.js` — handles multi-select and downstream drag
- Panel positions: `frontend/src/draggable.js` — saves panel positions to localStorage

### Key Files to Modify
- `frontend/package.json` — add `cytoscape-elk`, `elkjs`
- `frontend/src/graph.js` — replace layout config, add position save/restore
- `frontend/src/node-drag.js` — add manual position marking on drag end
- `frontend/src/main.js` — integrate reset button, position restore on startup
- `frontend/index.html` — add reset button to toolbar

---

## Notes

- ELK `layered` algorithm is the primary choice (specifically designed for hierarchical/DAG graphs)
- ELK supports `elk.layered.crossingMinimization` for clean edge routing
- For compound nodes, ELK uses `elk.hierarchyHandling: "INCLUDE_CHILDREN"`
- Dagre can be kept as a fallback or removed entirely (ELK covers all use cases)
- Consider debouncing position saves (e.g., 300ms after drag end) to avoid excessive writes

---

**Requirements specification complete. Next step: create detailed implementation plan with phases.**
