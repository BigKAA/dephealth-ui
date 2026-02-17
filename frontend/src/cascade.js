/**
 * Cascade warning computation module.
 *
 * Traverses the Cytoscape graph upward from Down nodes through critical edges
 * to mark upstream services with cascade warning indicators.
 */

/**
 * Find the real root cause(s) for a down service node by tracing downstream
 * through critical edges to the actual unavailable dependency.
 *
 * For example, if A(down) → B(unknown), the root cause is B, not A.
 *
 * @param {import('cytoscape').NodeSingular} downNode
 * @param {Set<string>} chainNodes - Collects all node IDs in the failure chain
 * @returns {string[]} Array of root-cause node IDs
 */
function findRealRootCauses(downNode, chainNodes) {
  const rootCauses = [];
  const visited = new Set([downNode.id()]);
  const queue = [downNode];

  chainNodes.add(downNode.id());

  while (queue.length > 0) {
    const current = queue.shift();
    const outEdges = current.outgoers('edge');

    outEdges.forEach((edge) => {
      if (!edge.data('critical')) return;

      const target = edge.target();
      const targetId = target.id();
      if (visited.has(targetId)) return;

      const targetState = target.data('state');
      if (targetState !== 'down' && targetState !== 'unknown') return;

      visited.add(targetId);
      chainNodes.add(targetId);

      // If target is a service that's also down, recurse deeper.
      if (target.data('type') === 'service' && targetState === 'down') {
        queue.push(target);
      } else {
        // Terminal root cause: unknown/stale node or non-service dependency.
        rootCauses.push(targetId);
      }
    });
  }

  // Fallback: if no downstream cause found, the down node itself is the cause.
  return rootCauses.length > 0 ? rootCauses : [downNode.id()];
}

/**
 * Compute cascade warnings for all nodes in the graph.
 *
 * Algorithm:
 * 1. For each Down node (service or dependency), trace downstream through
 *    critical edges to find the real root cause.
 * 2. BFS upstream from the Down node through critical edges to mark all
 *    upstream services with cascade warnings referencing the real root cause.
 * 3. Mark all nodes in the failure chain (down + root cause) with
 *    inCascadeChain flag for filter support.
 *
 * @param {import('cytoscape').Core} cy - Cytoscape instance
 */
export function computeCascadeWarnings(cy) {
  cy.batch(() => {
    // Clear previous cascade data on all nodes.
    cy.nodes().forEach((node) => {
      node.data('cascadeCount', 0);
      node.data('cascadeSources', []);
      node.data('inCascadeChain', false);
    });

    // Collect cascade sources per node: nodeId → Set of root-cause node IDs.
    const cascadeMap = new Map();
    const allChainNodes = new Set();

    // Find all Down nodes as cascade starting points.
    // Include all node types: when a service loses its own metrics it becomes
    // a dependency-type node, but should still trigger cascade upstream.
    const downNodes = cy.nodes().filter(
      (node) => node.data('state') === 'down' && !node.data('isGroup')
    );

    downNodes.forEach((downNode) => {
      // Find the real root causes by tracing downstream.
      const chainNodes = new Set();
      const rootCauses = findRealRootCauses(downNode, chainNodes);

      // Collect chain nodes globally.
      for (const id of chainNodes) allChainNodes.add(id);

      // BFS upstream from the down node through critical edges.
      const visited = new Set();
      const queue = [downNode];

      while (queue.length > 0) {
        const current = queue.shift();

        // Get all incoming edges to the current node.
        const incomingEdges = current.incomers('edge');

        incomingEdges.forEach((edge) => {
          // Only propagate through critical edges.
          if (!edge.data('critical')) return;

          const sourceNode = edge.source();
          const sourceId = sourceNode.id();

          // Skip if already visited in this BFS (cycle protection).
          if (visited.has(sourceId)) return;
          visited.add(sourceId);

          // Skip nodes that are themselves Down (they are their own root cause).
          if (sourceNode.data('state') === 'down') return;

          // Mark this node as affected by the cascade.
          if (!cascadeMap.has(sourceId)) {
            cascadeMap.set(sourceId, new Set());
          }
          for (const rc of rootCauses) {
            cascadeMap.get(sourceId).add(rc);
          }

          // Continue propagation upward from this node.
          queue.push(sourceNode);
        });
      }
    });

    // Apply cascade data to nodes.
    for (const [nodeId, sources] of cascadeMap) {
      const node = cy.getElementById(nodeId);
      if (node.length > 0) {
        const sourceArray = [...sources];
        node.data('cascadeCount', sourceArray.length);
        node.data('cascadeSources', sourceArray);
      }
    }

    // Mark cascade chain nodes (down + root cause nodes) for filter support.
    for (const nodeId of allChainNodes) {
      const node = cy.getElementById(nodeId);
      if (node.length > 0) {
        node.data('inCascadeChain', true);
      }
    }
  });
}

