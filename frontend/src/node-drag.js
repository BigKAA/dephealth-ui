// Node drag module: group drag for selected nodes, Ctrl+Drag downstream

/**
 * Collect downstream nodes via BFS.
 * @param {cytoscape.NodeSingular} node - Starting node
 * @param {boolean} allLevels - If true, collect full downstream; otherwise 1 level only
 * @returns {cytoscape.Collection} Collection of downstream nodes (excluding the start node)
 */
function getDownstreamNodes(node, allLevels) {
  const cy = node.cy();
  let collected = cy.collection();

  if (!allLevels) {
    // 1-level downstream only
    collected = node.outgoers('node');
    return collected;
  }

  // Full BFS downstream
  const visited = new Set();
  const queue = [node];
  visited.add(node.id());

  while (queue.length > 0) {
    const current = queue.shift();
    const neighbors = current.outgoers('node');
    neighbors.forEach((n) => {
      if (!visited.has(n.id())) {
        visited.add(n.id());
        collected = collected.union(n);
        queue.push(n);
      }
    });
  }

  return collected;
}

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

  cy.on('free', 'node', () => {
    companions.clear();
    grabbedStartPos = null;
  });
}
