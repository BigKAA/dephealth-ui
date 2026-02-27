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

| Action | Target | Ctrl | Shift | Result |
|--------|--------|------|-------|--------|
| Click | Node | — | — | Toggle sidebar details |
| Click | Node | + | — | Toggle node selection |
| Click | Background | — | — | Clear selection, close sidebar |
| Drag | Node | — | — | Move single node |
| Drag | Selected node (group) | — | — | Move entire selected group |
| Drag | Node | + | — | Move node + 1-level downstream |
| Drag | Node | + | + | Move node + full downstream subgraph |
| Drag | Background | — | — | Pan camera |
| Drag | Background | + | — | Box-select (rectangular area) |
| Double-click | Background | — | — | Center camera on click point |
| Double-click | Node with Grafana URL | — | — | Open Grafana dashboard |
| Double-click | Collapsed namespace | — | — | Expand namespace |
| Escape | — | — | — | Close sidebar, clear selection |

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
