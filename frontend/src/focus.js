// Focus mode module: highlight a node and its connections, dim everything else.
//
// Event interaction contract:
// - Plain click on node -> focus mode (this module)
// - Ctrl/Meta+Click on node -> multi-select (selection.js)
// - Plain click on background -> clear focus (this module) AND clear selection (selection.js)
// Both background tap handlers fire independently (Cytoscape has no stopPropagation).
// Focus and multi-select are mutually exclusive:
//   applyFocus() calls clearSelection(), cy.on('select') calls clearFocus().

import { getConnectedElements, getDownstreamNodes, getUpstreamNodes } from './graph-utils.js';
import { clearSelection } from './selection.js';

const FOCUS_CLASSES = ['focused', 'focus-neighbor', 'focus-edge-in',
                       'focus-edge-out', 'focus-traversal', 'dimmed'];
const ALL_CLASSES_STR = FOCUS_CLASSES.join(' ');

let focusActive = false;

/**
 * Whether focus mode is currently active.
 * @returns {boolean}
 */
export function isFocusActive() {
  return focusActive;
}

/**
 * Clear all focus mode classes from the graph.
 * @param {cytoscape.Core} cy
 */
export function clearFocus(cy) {
  if (!focusActive) return;
  cy.batch(() => {
    cy.elements().removeClass(ALL_CLASSES_STR);
  });
  focusActive = false;
}

/**
 * Apply 1-hop focus to a node: highlight it, its neighbors, and connecting edges.
 * Incoming edges are colored blue, outgoing edges purple.
 * @param {cytoscape.Core} cy
 * @param {cytoscape.NodeSingular} node
 */
function applyFocus(cy, node) {
  const { incomingEdges, outgoingEdges, sourceNodes, targetNodes } =
    getConnectedElements(node);

  // Clear selection — focus and multi-select are mutually exclusive
  clearSelection(cy);

  cy.batch(() => {
    // 1. Clear ALL old focus classes + dim everything in one step.
    //    This prevents stale .focused/.focus-neighbor/.focus-edge-*
    //    classes from persisting on previously focused elements.
    cy.elements().removeClass(ALL_CLASSES_STR).addClass('dimmed');

    // 2. Highlight focused node
    node.removeClass('dimmed').addClass('focused');

    // 3. Highlight neighbors
    sourceNodes.removeClass('dimmed').addClass('focus-neighbor');
    targetNodes.removeClass('dimmed').addClass('focus-neighbor');

    // 4. Color edges by direction
    incomingEdges.removeClass('dimmed').addClass('focus-edge-in');
    outgoingEdges.removeClass('dimmed').addClass('focus-edge-out');

    // 5. Undim parent nodes of visible children (namespace group boundaries)
    const visibleSet = node.union(sourceNodes).union(targetNodes);
    visibleSet.parents().removeClass('dimmed');
  });

  focusActive = true;
}

/**
 * Apply downstream focus: highlight the node and its full downstream chain.
 * Uses successors() for BFS + edgesWith() for all internal edges (including back-edges).
 * Edges keep their state colors via .focus-traversal class.
 * @param {cytoscape.Core} cy
 * @param {cytoscape.NodeSingular} node
 */
function applyDownstreamFocus(cy, node) {
  const downstream = getDownstreamNodes(node, true);
  const allFocused = downstream.union(node);
  // edgesWith(same collection) returns edges where BOTH endpoints are in allFocused.
  // Catches ALL internal edges including back-edges in cycles.
  const focusedEdges = allFocused.edgesWith(allFocused);

  clearSelection(cy);

  cy.batch(() => {
    cy.elements().removeClass(ALL_CLASSES_STR).addClass('dimmed');
    node.removeClass('dimmed').addClass('focused');
    downstream.removeClass('dimmed').addClass('focus-neighbor');
    focusedEdges.removeClass('dimmed').addClass('focus-traversal');
    allFocused.parents().removeClass('dimmed');
  });

  focusActive = true;
}

/**
 * Apply upstream focus: highlight the node and its full upstream chain.
 * Symmetric to applyDownstreamFocus — uses predecessors() for BFS.
 * @param {cytoscape.Core} cy
 * @param {cytoscape.NodeSingular} node
 */
function applyUpstreamFocus(cy, node) {
  const upstream = getUpstreamNodes(node, true);
  const allFocused = upstream.union(node);
  const focusedEdges = allFocused.edgesWith(allFocused);

  clearSelection(cy);

  cy.batch(() => {
    cy.elements().removeClass(ALL_CLASSES_STR).addClass('dimmed');
    node.removeClass('dimmed').addClass('focused');
    upstream.removeClass('dimmed').addClass('focus-neighbor');
    focusedEdges.removeClass('dimmed').addClass('focus-traversal');
    allFocused.parents().removeClass('dimmed');
  });

  focusActive = true;
}

/**
 * Initialize focus mode on the Cytoscape instance.
 * @param {cytoscape.Core} cy
 */
export function initFocusMode(cy) {
  // Click on node: route by modifier keys
  cy.on('tap', 'node', (evt) => {
    const oe = evt.originalEvent;
    if (!oe) return;
    // Ctrl/Meta → multi-select (handled by selection.js)
    if (oe.ctrlKey || oe.metaKey) return;

    const node = evt.target;
    // Skip parent/group nodes — Phase 4 handles collapsed namespace focus
    if (node.isParent()) return;

    if (oe.shiftKey && oe.altKey) {
      applyUpstreamFocus(cy, node);
    } else if (oe.shiftKey) {
      applyDownstreamFocus(cy, node);
    } else {
      applyFocus(cy, node);
    }
  });

  // Click background (no modifiers) -> clear focus
  cy.on('tap', (evt) => {
    if (evt.target !== cy) return;
    const oe = evt.originalEvent;
    if (oe && (oe.ctrlKey || oe.metaKey)) return;
    clearFocus(cy);
  });

  // Auto-clear focus when multi-select is activated (Ctrl+Click or box-select).
  // Listens for Cytoscape's built-in 'select' event — no need to modify selection.js.
  cy.on('select', 'node', () => {
    clearFocus(cy);
  });
}
