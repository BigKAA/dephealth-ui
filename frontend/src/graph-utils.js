// Shared graph traversal utilities for focus mode and node drag

/**
 * Get 1-hop connected elements for a node (edges + neighbor nodes),
 * split by direction.
 * @param {cytoscape.NodeSingular} node
 * @returns {{ incomingEdges: cytoscape.Collection, outgoingEdges: cytoscape.Collection, sourceNodes: cytoscape.Collection, targetNodes: cytoscape.Collection }}
 */
export function getConnectedElements(node) {
  const incomingEdges = node.incomers('edge');
  const outgoingEdges = node.outgoers('edge');
  const sourceNodes = incomingEdges.sources();
  const targetNodes = outgoingEdges.targets();
  return { incomingEdges, outgoingEdges, sourceNodes, targetNodes };
}

/**
 * Get downstream nodes (follow outgoing edges).
 * Uses Cytoscape builtin successors() for full BFS — handles cycles correctly.
 * @param {cytoscape.NodeSingular} node - Starting node
 * @param {boolean} allLevels - false = 1-hop only, true = full traversal
 * @returns {cytoscape.Collection} Collection of downstream nodes (excluding start)
 */
export function getDownstreamNodes(node, allLevels) {
  if (!allLevels) {
    return node.outgoers('node');
  }
  return node.successors('node');
}

/**
 * Get upstream nodes (follow incoming edges).
 * Uses Cytoscape builtin predecessors() for full BFS — handles cycles correctly.
 * @param {cytoscape.NodeSingular} node - Starting node
 * @param {boolean} allLevels - false = 1-hop only, true = full traversal
 * @returns {cytoscape.Collection} Collection of upstream nodes (excluding start)
 */
export function getUpstreamNodes(node, allLevels) {
  if (!allLevels) {
    return node.incomers('node');
  }
  return node.predecessors('node');
}

// NOTE: cascade.js intentionally uses its own manual BFS because it
// needs to filter by edge.data('critical') at each step.
// Do not consolidate cascade traversal into this module.
