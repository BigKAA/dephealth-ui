// Multi-select module: Ctrl+Click toggle, Ctrl+Drag box-select, click-to-clear

/**
 * Initialize multi-selection behavior on the Cytoscape instance.
 * - Ctrl/Cmd + Click on node: toggle selection
 * - Ctrl/Cmd + Drag on background: box-select
 * - Click on background (no Ctrl): clear selection
 * @param {cytoscape.Core} cy
 */
export function initSelection(cy) {
  // Ctrl+Click on node: toggle select/unselect
  cy.on('tap', 'node', (evt) => {
    const oe = evt.originalEvent;
    if (!oe || !(oe.ctrlKey || oe.metaKey)) return;

    const node = evt.target;
    if (node.selected()) {
      node.unselect();
    } else {
      node.select();
    }
  });

  // Click on background (no Ctrl): clear all selection
  cy.on('tap', (evt) => {
    if (evt.target !== cy) return;
    const oe = evt.originalEvent;
    if (oe && (oe.ctrlKey || oe.metaKey)) return; // let box-select handle Ctrl+bg
    clearSelection(cy);
  });

  // Box-select: Ctrl+Drag on background only
  initBoxSelect(cy);
}

/**
 * Clear all selected nodes.
 * @param {cytoscape.Core} cy
 */
export function clearSelection(cy) {
  cy.nodes(':selected').unselect();
}

/**
 * Check if a rendered position falls within any node's bounding box.
 * @param {cytoscape.Core} cy
 * @param {number} rx - rendered X coordinate (relative to cy container)
 * @param {number} ry - rendered Y coordinate (relative to cy container)
 * @returns {boolean}
 */
function isOnNode(cy, rx, ry) {
  // Convert rendered position to model position
  const pan = cy.pan();
  const zoom = cy.zoom();
  const mx = (rx - pan.x) / zoom;
  const my = (ry - pan.y) / zoom;

  // Check against all non-parent nodes
  return cy.nodes().some((node) => {
    if (node.isParent()) return false;
    const bb = node.boundingBox();
    return mx >= bb.x1 && mx <= bb.x2 && my >= bb.y1 && my <= bb.y2;
  });
}

/**
 * Initialize box-select (Ctrl/Cmd + drag on background).
 * Uses a deferred start: pointerdown sets pending state, actual box-select
 * begins only after a minimum drag distance and only if the pointer started
 * on the background (not on a node).
 * @param {cytoscape.Core} cy
 */
function initBoxSelect(cy) {
  const container = cy.container();
  let pending = false; // waiting for drag threshold
  let active = false; // box-select rectangle is visible
  let rect = null;
  let startX = 0;
  let startY = 0;
  let pointerId = null;

  container.addEventListener('pointerdown', (e) => {
    if (!(e.ctrlKey || e.metaKey)) return;
    if (e.button !== 0) return;

    const bounds = container.getBoundingClientRect();
    const rx = e.clientX - bounds.left;
    const ry = e.clientY - bounds.top;

    // Only start box-select on background, not on a node
    if (isOnNode(cy, rx, ry)) return;

    pending = true;
    active = false;
    startX = rx;
    startY = ry;
    pointerId = e.pointerId;

    e.target.setPointerCapture(e.pointerId);
    e.preventDefault();
    e.stopPropagation();
  });

  container.addEventListener('pointermove', (e) => {
    if (!pending && !active) return;
    if (e.pointerId !== pointerId) return;

    const bounds = container.getBoundingClientRect();
    const curX = e.clientX - bounds.left;
    const curY = e.clientY - bounds.top;
    const dx = Math.abs(curX - startX);
    const dy = Math.abs(curY - startY);

    // Start box-select only after minimum drag distance
    if (pending && !active && (dx > 5 || dy > 5)) {
      active = true;
      pending = false;

      // Disable panning during box-select
      cy.panningEnabled(false);
      cy.boxSelectionEnabled(false);

      // Create overlay rectangle
      rect = document.createElement('div');
      rect.className = 'box-select-rect';
      container.appendChild(rect);
    }

    if (!active || !rect) return;

    const x = Math.min(startX, curX);
    const y = Math.min(startY, curY);

    rect.style.left = `${x}px`;
    rect.style.top = `${y}px`;
    rect.style.width = `${dx}px`;
    rect.style.height = `${dy}px`;
  });

  container.addEventListener('pointerup', (e) => {
    if (e.pointerId !== pointerId) return;

    if (active && rect) {
      const bounds = container.getBoundingClientRect();
      const curX = e.clientX - bounds.left;
      const curY = e.clientY - bounds.top;

      const x1 = Math.min(startX, curX);
      const y1 = Math.min(startY, curY);
      const x2 = Math.max(startX, curX);
      const y2 = Math.max(startY, curY);

      // Select nodes within the rendered bounding box
      cy.nodes().forEach((node) => {
        if (node.isParent()) return;
        const pos = node.renderedPosition();
        if (pos.x >= x1 && pos.x <= x2 && pos.y >= y1 && pos.y <= y2) {
          node.select();
        }
      });

      // Cleanup rectangle
      rect.remove();
      rect = null;
    }

    // Re-enable panning
    cy.panningEnabled(true);
    cy.boxSelectionEnabled(false);

    pending = false;
    active = false;
    pointerId = null;
  });

  // Cancel on pointer leave or escape
  container.addEventListener('pointercancel', () => {
    if (rect) rect.remove();
    rect = null;
    pending = false;
    active = false;
    pointerId = null;
    cy.panningEnabled(true);
  });
}
