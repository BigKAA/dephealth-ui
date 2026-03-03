// Node drag module: group drag for selected nodes, Ctrl+Drag downstream

import { getDownstreamNodes } from './graph-utils.js';
import { markManualPosition, markManualPositions } from './layout-store.js';

/**
 * Initialize node drag behavior on the Cytoscape instance.
 * - Drag on selected node: move entire selected group
 * - Ctrl+Drag: move node + 1-level downstream
 * - Ctrl+Shift+Drag: move node + full downstream subgraph
 * @param {cytoscape.Core} cy
 */
export function initNodeDrag(cy) {
  let companions = new Map(); // nodeId -> { x, y } start positions
  let grabbedStartPos = null;

  cy.on('grab', 'node', (evt) => {
    const node = evt.target;
    const oe = evt.originalEvent;
    companions.clear();
    grabbedStartPos = { ...node.position() };

    const ctrlKey = oe && (oe.ctrlKey || oe.metaKey);
    const shiftKey = oe && oe.shiftKey;

    // Determine companion nodes
    let companionNodes = cy.collection();

    // If the node is part of a multi-selection, drag the whole group
    const selected = cy.nodes(':selected');
    if (selected.length > 1 && node.selected()) {
      companionNodes = selected.filter((n) => n.id() !== node.id());
    }

    // Ctrl+Drag: add downstream nodes
    if (ctrlKey) {
      const downstream = getDownstreamNodes(node, shiftKey);
      companionNodes = companionNodes.union(downstream);
      // Remove the grabbed node itself from companions (it moves via Cytoscape natively)
      companionNodes = companionNodes.filter((n) => n.id() !== node.id());
    }

    if (companionNodes.length === 0) return;

    // Save start positions for all companions
    companionNodes.forEach((n) => {
      companions.set(n.id(), { ...n.position() });
    });
  });

  cy.on('drag', 'node', (evt) => {
    if (companions.size === 0) return;

    const node = evt.target;
    const currentPos = node.position();
    const dx = currentPos.x - grabbedStartPos.x;
    const dy = currentPos.y - grabbedStartPos.y;

    // Move all companions by the same delta
    cy.batch(() => {
      for (const [id, startPos] of companions) {
        const companion = cy.getElementById(id);
        if (companion.length) {
          companion.position({
            x: startPos.x + dx,
            y: startPos.y + dy,
          });
        }
      }
    });
  });

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
}
