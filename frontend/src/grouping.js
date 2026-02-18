// Namespace grouping module — state management, compound nodes, collapse/expand

import { extractNamespaceFromHost } from './namespace.js';

const LS_GROUPING = 'dephealth-grouping';
const LS_COLLAPSED = 'dephealth-collapsed-ns';
const LS_DIMENSION = 'dephealth-grouping-dim';
const NS_PREFIX = 'ns::';

// State priority for worst-state computation (higher = worse)
const STATE_PRIORITY = { ok: 0, unknown: 1, degraded: 2, down: 3 };

// In-memory store for collapsed namespace data (children + edges)
// Map<nsName, { children: Array<{data}>, edges: Array<{data}> }>
const collapsedStore = new Map();

// ─── State persistence ───────────────────────────────────────────────

/**
 * Check whether namespace grouping mode is enabled.
 * @returns {boolean}
 */
export function isGroupingEnabled() {
  return localStorage.getItem(LS_GROUPING) === 'true';
}

/**
 * Enable or disable namespace grouping mode.
 * @param {boolean} enabled
 */
export function setGroupingEnabled(enabled) {
  localStorage.setItem(LS_GROUPING, enabled ? 'true' : 'false');
  if (!enabled) {
    collapsedStore.clear();
  }
}

/**
 * Get the active grouping dimension.
 * @returns {'namespace'|'group'}
 */
export function getGroupingDimension() {
  return localStorage.getItem(LS_DIMENSION) === 'group' ? 'group' : 'namespace';
}

/**
 * Set the active grouping dimension.
 * Clears collapsed state when switching dimensions.
 * @param {'namespace'|'group'} dim
 */
export function setGroupingDimension(dim) {
  const prev = getGroupingDimension();
  localStorage.setItem(LS_DIMENSION, dim);
  if (prev !== dim) {
    collapsedStore.clear();
    localStorage.removeItem(LS_COLLAPSED);
  }
}

/**
 * Get the set of currently collapsed namespace names.
 * @returns {Set<string>}
 */
export function getCollapsedNamespaces() {
  try {
    const raw = localStorage.getItem(LS_COLLAPSED);
    if (!raw) return new Set();
    return new Set(JSON.parse(raw));
  } catch {
    return new Set();
  }
}

/**
 * Persist the set of collapsed namespace names.
 * @param {Set<string>} nsSet
 */
export function setCollapsedNamespaces(nsSet) {
  localStorage.setItem(LS_COLLAPSED, JSON.stringify([...nsSet]));
}

// ─── Compound node construction ──────────────────────────────────────

/**
 * Build compound parent elements for grouping.
 * Uses the active dimension (namespace or group) to determine grouping.
 * @param {{nodes: Array}} data - Topology data from API
 * @returns {{ parents: Array, parentMap: Map<string, string> }}
 */
export function buildCompoundElements(data) {
  const dim = getGroupingDimension();
  const groups = new Set();
  const parentMap = new Map(); // nodeId -> parentId

  for (const node of data.nodes) {
    let val;
    if (dim === 'group') {
      // Only service nodes have group; deps without group are ungrouped
      val = node.group || null;
    } else {
      // namespace dimension: existing behavior with FQDN fallback for deps
      val = node.namespace || (node.type !== 'service' ? extractNamespaceFromHost(node.label) : null);
    }
    if (val) {
      groups.add(val);
      parentMap.set(node.id, NS_PREFIX + val);
    }
  }

  const parents = [...groups].map((g) => ({
    group: 'nodes',
    data: {
      id: NS_PREFIX + g,
      label: g,
      nsName: g,
      isGroup: true,
    },
  }));

  return { parents, parentMap };
}

/**
 * Get the namespace prefix used for compound parent node IDs.
 * @returns {string}
 */
export function getNamespacePrefix() {
  return NS_PREFIX;
}

/**
 * Get stored children data for a collapsed namespace.
 * Used by filters to determine collapsed node visibility.
 * @param {string} nsName - Namespace name (without prefix)
 * @returns {Array<{data: object}>|null}
 */
export function getCollapsedChildren(nsName) {
  const stored = collapsedStore.get(nsName);
  return stored ? stored.children : null;
}

/**
 * Find original child node connected to an external node within a collapsed namespace.
 * Searches stored edges to determine which child was the actual endpoint.
 * @param {string} nsName - Namespace name (without prefix)
 * @param {string} externalNodeId - ID of the node outside the namespace
 * @returns {string|null} ID of the matching child, or sole child if only one exists
 */
export function findConnectedChild(nsName, externalNodeId) {
  const stored = collapsedStore.get(nsName);
  if (!stored) return null;

  const childIds = new Set(stored.children.map((c) => c.data.id));

  for (const edge of stored.edges) {
    if (edge.data.source === externalNodeId && childIds.has(edge.data.target)) {
      return edge.data.target;
    }
    if (edge.data.target === externalNodeId && childIds.has(edge.data.source)) {
      return edge.data.source;
    }
  }

  // Fallback: if only one child, return it
  if (stored.children.length === 1) return stored.children[0].data.id;
  return null;
}

// ─── Collapse / Expand ───────────────────────────────────────────────

/**
 * Compute worst state from a list of state strings.
 * @param {string[]} states
 * @returns {string}
 */
function worstState(states) {
  let worst = 'ok';
  for (const s of states) {
    if ((STATE_PRIORITY[s] ?? 0) > (STATE_PRIORITY[worst] ?? 0)) {
      worst = s;
    }
  }
  return worst;
}

/**
 * Collapse a namespace group into a single summary node.
 * Removes children, stores their data, replaces the compound parent with
 * a collapsed summary node, and creates aggregated redirect edges.
 *
 * @param {cytoscape.Core} cy
 * @param {string} nsName - Namespace name (without prefix)
 */
export function collapseNamespace(cy, nsName) {
  const parentId = NS_PREFIX + nsName;
  const parentNode = cy.getElementById(parentId);
  if (!parentNode.length) return;

  const children = parentNode.children();
  if (children.length === 0) return;

  // Collect child IDs for fast lookup
  const childIds = new Set();
  children.forEach((c) => childIds.add(c.id()));

  // Compute summary from children
  const states = [];
  let totalAlerts = 0;
  let worstAlertSeverity = null;
  children.forEach((c) => {
    states.push(c.data('state') || 'unknown');
    totalAlerts += c.data('alertCount') || 0;
    if (c.data('alertSeverity')) worstAlertSeverity = c.data('alertSeverity');
  });

  // Collect all edges involving children
  const storedEdges = [];
  const edgesToRemove = cy.collection();

  children.connectedEdges().forEach((edge) => {
    storedEdges.push({ data: { ...edge.data() } });
    edgesToRemove.merge(edge);
  });

  // Store children data for later restore
  const storedChildren = [];
  children.forEach((c) => {
    storedChildren.push({ data: { ...c.data() } });
  });

  collapsedStore.set(nsName, {
    children: storedChildren,
    edges: storedEdges,
  });

  // Remove edges first, then children
  cy.batch(() => {
    edgesToRemove.remove();
    children.remove();

    // Update parent node to become a collapsed summary
    parentNode.data('isCollapsed', true);
    parentNode.data('childCount', storedChildren.length);
    parentNode.data('state', worstState(states));
    parentNode.data('alertCount', totalAlerts);
    parentNode.data('alertSeverity', worstAlertSeverity || undefined);
    parentNode.data('label', `${nsName} (${storedChildren.length})`);

    // Build aggregated redirect edges
    const aggMap = new Map(); // "src->tgt" -> { count, worstState, states[] }

    for (const stored of storedEdges) {
      const src = stored.data.source;
      const tgt = stored.data.target;

      // Skip internal edges (both endpoints in this namespace)
      if (childIds.has(src) && childIds.has(tgt)) continue;

      // Redirect endpoints that were inside this namespace
      let effSrc = childIds.has(src) ? parentId : src;
      let effTgt = childIds.has(tgt) ? parentId : tgt;

      // Check if the other endpoint is in another collapsed namespace
      // (its node may have been replaced by another collapsed node)
      if (effSrc !== parentId && !cy.getElementById(effSrc).length) continue;
      if (effTgt !== parentId && !cy.getElementById(effTgt).length) continue;

      // Skip self-loops
      if (effSrc === effTgt) continue;

      const key = `${effSrc}->${effTgt}`;
      if (!aggMap.has(key)) {
        aggMap.set(key, { source: effSrc, target: effTgt, count: 0, states: [] });
      }
      const agg = aggMap.get(key);
      agg.count++;
      agg.states.push(stored.data.state || 'ok');
    }

    // Add aggregated edges
    for (const [key, agg] of aggMap) {
      cy.add({
        group: 'edges',
        data: {
          id: `agg::${key}`,
          source: agg.source,
          target: agg.target,
          state: worstState(agg.states),
          latency: agg.count > 1 ? `×${agg.count}` : '',
          isAggregated: true,
          aggCount: agg.count,
        },
      });
    }
  });

  // Persist collapsed state
  const collapsed = getCollapsedNamespaces();
  collapsed.add(nsName);
  setCollapsedNamespaces(collapsed);
}

/**
 * Expand a previously collapsed namespace back to its full group.
 *
 * @param {cytoscape.Core} cy
 * @param {string} nsName - Namespace name (without prefix)
 */
export function expandNamespace(cy, nsName) {
  const parentId = NS_PREFIX + nsName;
  const parentNode = cy.getElementById(parentId);
  if (!parentNode.length) return;

  const stored = collapsedStore.get(nsName);
  if (!stored) return;

  const otherCollapsed = getCollapsedNamespaces();
  otherCollapsed.delete(nsName);

  cy.batch(() => {
    // Remove aggregated edges connected to this collapsed node
    parentNode.connectedEdges().filter((e) => e.data('isAggregated')).remove();

    // Restore parent as expanded compound node
    parentNode.data('isCollapsed', false);
    parentNode.data('label', nsName);
    parentNode.removeData('childCount');
    parentNode.removeData('state');
    parentNode.removeData('alertCount');
    parentNode.removeData('alertSeverity');

    // Re-add children
    for (const child of stored.children) {
      child.data.parent = parentId;
      cy.add({ group: 'nodes', data: child.data });
    }

    // Re-add original edges, redirecting to other collapsed namespaces if needed
    for (const edge of stored.edges) {
      let src = edge.data.source;
      let tgt = edge.data.target;

      // Check if source/target is in another collapsed namespace
      if (!cy.getElementById(src).length) {
        const redirected = findCollapsedParent(src, otherCollapsed);
        if (redirected) src = redirected; else continue;
      }
      if (!cy.getElementById(tgt).length) {
        const redirected = findCollapsedParent(tgt, otherCollapsed);
        if (redirected) tgt = redirected; else continue;
      }

      // Skip self-loops and duplicates
      if (src === tgt) continue;

      const edgeId = src === edge.data.source && tgt === edge.data.target
        ? edge.data.id
        : `agg::${src}->${tgt}`;

      // Only add if this exact edge doesn't already exist
      if (!cy.getElementById(edgeId).length) {
        const newData = { ...edge.data, id: edgeId, source: src, target: tgt };
        // If redirected, mark as aggregated
        if (src !== edge.data.source || tgt !== edge.data.target) {
          newData.isAggregated = true;
        }
        cy.add({ group: 'edges', data: newData });
      }
    }
  });

  collapsedStore.delete(nsName);

  // Persist
  setCollapsedNamespaces(otherCollapsed);

  // Re-layout
  cy.layout({
    name: 'fcose',
    animate: true,
    animationDuration: 400,
    quality: 'default',
    nodeSeparation: 80,
    idealEdgeLength: 120,
    nodeRepulsion: 6000,
    tile: true,
  }).run();
}

/**
 * Find the collapsed parent node ID for a node that's inside a collapsed namespace.
 * Searches the collapsedStore for a child matching the given nodeId.
 *
 * @param {string} nodeId
 * @param {Set<string>} collapsedNs - Set of collapsed namespace names
 * @returns {string|null} Parent ID (ns::name) or null
 */
function findCollapsedParent(nodeId, collapsedNs) {
  for (const nsName of collapsedNs) {
    const stored = collapsedStore.get(nsName);
    if (!stored) continue;
    for (const child of stored.children) {
      if (child.data.id === nodeId) return NS_PREFIX + nsName;
    }
  }
  return null;
}

/**
 * Collapse all expanded namespace groups.
 * @param {cytoscape.Core} cy
 */
export function collapseAll(cy) {
  const parents = cy.nodes('[?isGroup]').filter((n) => !n.data('isCollapsed'));
  parents.forEach((p) => {
    const nsName = p.data('label');
    collapseNamespace(cy, nsName);
  });
}

/**
 * Expand all collapsed namespace groups.
 * @param {cytoscape.Core} cy
 */
export function expandAll(cy) {
  // Expand in reverse order to avoid edge conflicts
  const collapsed = [...getCollapsedNamespaces()];
  for (const nsName of collapsed) {
    expandNamespace(cy, nsName);
  }
}

/**
 * Check if any namespace group is currently expanded.
 * @param {cytoscape.Core} cy
 * @returns {boolean}
 */
export function hasExpandedGroups(cy) {
  return cy.nodes('[?isGroup]').filter((n) => !n.data('isCollapsed')).length > 0;
}

/**
 * Re-apply collapsed state after a full graph rebuild (e.g. auto-refresh).
 * Call after renderGraph() when grouping is enabled.
 *
 * @param {cytoscape.Core} cy
 */
export function reapplyCollapsedState(cy) {
  const collapsed = getCollapsedNamespaces();
  if (collapsed.size === 0) return;

  // Clear in-memory store since the graph was rebuilt
  collapsedStore.clear();

  for (const nsName of collapsed) {
    const parentId = NS_PREFIX + nsName;
    const parentNode = cy.getElementById(parentId);
    if (parentNode.length && parentNode.children().length > 0) {
      collapseNamespace(cy, nsName);
    }
  }
}
