# Graph Interactions

**Language:** English | [Русский](./graph-interactions.ru.md)

---

## Overview

The topology graph supports mouse and keyboard interactions for navigating, selecting, and rearranging nodes. All interactions work with both **dagre** and **fcose** layouts.

## Modifier Keys

| Modifier | Windows / Linux | macOS |
|----------|----------------|-------|
| Ctrl | `Ctrl` | `Cmd (⌘)` |
| Shift | `Shift` | `Shift` |

Throughout this document, "Ctrl" refers to `Ctrl` on Windows/Linux and `Cmd (⌘)` on macOS.

## Event Matrix

| Action | Target | Ctrl | Shift | Alt | Result |
|--------|--------|------|-------|-----|--------|
| Click | Node | — | — | — | Focus mode (1-hop) + sidebar |
| Click | Node | + | — | — | Toggle node selection |
| Click | Node | — | + | — | Focus downstream (full chain) |
| Click | Node | — | + | + | Focus upstream (full chain) |
| Click | Background | — | — | — | Clear focus, clear selection |
| Drag | Node | — | — | — | Move single node |
| Drag | Selected node (group) | — | — | — | Move entire selected group |
| Drag | Node | + | — | — | Move node + 1-level downstream |
| Drag | Node | + | + | — | Move node + full downstream subgraph |
| Drag | Background | — | — | — | Pan camera |
| Drag | Background | + | — | — | Box-select (rectangular area) |
| Double-click | Background | — | — | — | Center camera on click point |
| Double-click | Node with Grafana URL | — | — | — | Open Grafana dashboard |
| Double-click | Collapsed namespace | — | — | — | Expand namespace |
| Escape | — | — | — | — | Close sidebar, clear selection |

## Focus Mode

Focus mode highlights a node and its connections while dimming everything else. This helps visually trace dependencies in complex topologies.

### Click (1-Hop Focus)

Click on any node (without modifier keys) to activate focus mode:

- The clicked node gets a **blue highlight border**
- **Incoming edges** are colored **blue** (who calls this service)
- **Outgoing edges** are colored **purple** (what this service depends on)
- **Neighbor nodes** (sources and targets) remain at full opacity
- All other elements are **dimmed** (low opacity)

Click another node to switch focus. Focus and sidebar open simultaneously — the sidebar shows node details while focus highlights connections.

### Shift+Click (Downstream Focus)

Hold **Shift** and click on a node to highlight its **entire downstream chain** — all services that transitively depend on it, following outgoing edges via BFS traversal.

- Edges between downstream nodes keep their **state colors** (green/orange/red) instead of direction coloring
- Correctly handles circular dependencies (no infinite loops)

### Shift+Alt+Click (Upstream Focus)

Hold **Shift+Alt** and click on a node to highlight its **entire upstream chain** — all services it transitively depends on, following incoming edges via BFS traversal.

### Clear Focus

- **Click on background** (without Ctrl): clears focus mode and returns all elements to normal
- Focus is automatically cleared when **multi-select** is activated (Ctrl+Click or box-select)
- Focus is automatically cleared on **graph structure changes** (nodes/edges added or removed)
- Focus persists across data polls when only data attributes change (state, latency)

### Collapsed Namespaces

Collapsed namespace nodes work with focus mode — clicking a collapsed namespace highlights its aggregated external connections. Shift+Click shows the downstream namespace-level view.

## Selection

### Ctrl+Click (Toggle Selection)

Hold **Ctrl** and click on a node to add it to the selection or remove it if already selected. The selected state is indicated by a blue border and overlay.

Multiple nodes can be selected simultaneously. Clicking a node without Ctrl opens the sidebar instead.

### Box-Select (Ctrl+Drag on Background)

Hold **Ctrl** and drag on the empty background to draw a selection rectangle. All non-parent nodes whose center falls within the rectangle will be selected.

- A minimum drag distance of 5px is required before the rectangle appears
- The selection rectangle is a semi-transparent blue overlay
- Releasing the mouse button completes the selection
- Box-select only activates when starting on the background, not on a node

### Clear Selection

- **Click on background** (without Ctrl): clears all selected nodes and closes the sidebar
- **Escape key**: clears selection and closes all panels (sidebar, search, context menu)

## Drag Modes

### Single Node Drag

Click and drag any node to move it. This is the default Cytoscape behavior.

### Group Drag

When multiple nodes are selected (via Ctrl+Click or box-select), dragging any **selected** node moves the entire group while preserving their relative positions.

Dragging a node that is **not** in the selection moves only that node.

### Ctrl+Drag: 1-Level Downstream

Hold **Ctrl** and drag a node to move it together with its **direct downstream dependencies** (1 level of outgoing edges).

This can be combined with group selection — if the dragged node is part of a selected group, both the group and the downstream nodes will move.

### Ctrl+Shift+Drag: Full Downstream Subgraph

Hold **Ctrl+Shift** and drag a node to move it together with its **entire downstream subgraph** (all transitive outgoing dependencies via BFS traversal).

Shared dependencies (nodes referenced by multiple parents) are included if they appear in the downstream traversal.

## Camera Controls

### Pan

Click and drag on the empty background to pan the camera.

### Zoom

Use the mouse scroll wheel to zoom in/out.

### Double-Click to Center

Double-click on the empty background to smoothly animate the camera so that the clicked point becomes the center of the viewport. Animation duration: 300ms.

Double-click on a **node** triggers node-specific actions (open Grafana or expand namespace) instead of centering.

## Edge Cases

### Collapsed Namespaces

Collapsed namespace nodes (compound parents) behave like regular nodes for selection — they can be Ctrl+Clicked to select/deselect. When a collapsed namespace is dragged, Cytoscape automatically moves all child nodes within it.

### Compound Nodes

Parent nodes (namespaces) are excluded from box-select hit testing. Only leaf (non-parent) nodes are selected via box-select or used for the `isOnNode` background detection.

### Temporary Positions

All position changes from drag operations are **temporary** — they are reset when:
- Data is refreshed from the server
- Layout is recalculated (layout button or layout change)
- Page is reloaded (F5)

### Keyboard Shortcuts

See also the keyboard shortcuts panel (accessible via the `?` button in the toolbar) for a complete list of shortcuts.
