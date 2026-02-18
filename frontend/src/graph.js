import cytoscape from 'cytoscape';
import dagre from 'cytoscape-dagre';
import fcose from 'cytoscape-fcose';
import { isElementVisible } from './search.js';
import { getNamespaceColor, getContrastTextColor, getStripeDataUri, extractNamespaceFromHost } from './namespace.js';
import { isGroupingEnabled, buildCompoundElements, getGroupingDimension } from './grouping.js';

cytoscape.use(dagre);
cytoscape.use(fcose);

let layoutDirection = 'TB'; // Global layout direction: 'TB' or 'LR'

/**
 * Returns the dimension value and prefix for a node based on the active grouping dimension.
 * Service nodes: use group or namespace depending on dimension.
 * Dependency nodes: always use namespace (they don't have group).
 * @returns {{ value: string, prefix: string }}
 */
function nodeDimension(ele) {
  const dim = getGroupingDimension();
  if (dim === 'group') {
    // In group mode, only show group value; dependencies without group get no stripe
    return { value: ele.data('group') || '', prefix: 'gr' };
  }
  return { value: ele.data('namespace') || '', prefix: 'ns' };
}

function isDarkTheme() {
  return document.documentElement.dataset.theme === 'dark';
}

const STATE_COLORS = {
  ok: '#4caf50',
  degraded: '#ff9800',
  down: '#f44336',
  unknown: '#9e9e9e',
};

const EDGE_STYLES = {
  ok: { lineStyle: 'solid', color: '#4caf50' },
  degraded: { lineStyle: 'dashed', color: '#ff9800' },
  down: { lineStyle: 'dotted', color: '#f44336' },
  unknown: { lineStyle: 'dashed', color: '#9e9e9e' },
};

// SDK v0.4.1: dependency status colors (used for edge coloring when status is available).
export const STATUS_COLORS = {
  ok: '#4caf50',
  timeout: '#ff9800',
  connection_error: '#f44336',
  error: '#f44336',
  dns_error: '#9c27b0',
  auth_error: '#ffeb3b',
  tls_error: '#b71c1c',
  unhealthy: '#ff5722',
};

// SDK v0.4.1: status abbreviations for edge labels.
export const STATUS_ABBREVIATIONS = {
  timeout: 'TMO',
  connection_error: 'CONN',
  dns_error: 'DNS',
  auth_error: 'AUTH',
  tls_error: 'TLS',
  unhealthy: 'UNH',
  error: 'ERR',
};

// SDK v0.4.1: full status labels for sidebar and tooltip display.
export const STATUS_LABELS = {
  ok: 'OK',
  timeout: 'Timeout',
  connection_error: 'Connection Error',
  dns_error: 'DNS Error',
  auth_error: 'Auth Error',
  tls_error: 'TLS Error',
  unhealthy: 'Unhealthy',
  error: 'Error',
};

const cytoscapeStyles = [
  // Service nodes
  {
    selector: 'node[type="service"]',
    style: {
      shape: 'round-rectangle',
      width: (ele) => {
        const label = ele.data('label') || '';
        const { value, prefix } = nodeDimension(ele);
        const secondLine = value ? `${prefix}: ${value}` : '';
        const maxLen = Math.max(label.length, secondLine.length);
        const fontSize = 12;
        const charWidth = fontSize * 0.6;
        const padding = 48; // extra for left stripe
        return Math.max(110, maxLen * charWidth + padding);
      },
      height: (ele) => (nodeDimension(ele).value ? 58 : 40),
      label: (ele) => {
        const label = ele.data('label') || '';
        const { value, prefix } = nodeDimension(ele);
        return value ? `${label}\n${prefix}: ${value}` : label;
      },
      'text-valign': 'center',
      'text-halign': 'center',
      'text-wrap': 'wrap',
      'text-max-width': 200,
      'font-size': 12,
      color: '#fff',
      'text-outline-width': 0,
      'background-color': (ele) => STATE_COLORS[ele.data('state')] || STATE_COLORS.unknown,
      'border-width': 2,
      'border-color': (ele) => STATE_COLORS[ele.data('state')] || STATE_COLORS.unknown,
      // Left dimension stripe via base64-encoded SVG
      'background-image': (ele) => {
        const { value } = nodeDimension(ele);
        if (!value) return 'none';
        return getStripeDataUri(getNamespaceColor(value));
      },
      'background-image-opacity': 1,
      'background-width': '12px',
      'background-height': '100%',
      'background-position-x': '0%',
      'background-position-y': '50%',
      'background-clip': 'node',
      'background-image-containment': 'over',
    },
  },
  // Dependency nodes
  {
    selector: 'node[type!="service"]',
    style: {
      shape: 'ellipse',
      width: (ele) => {
        const label = ele.data('label') || '';
        const { value, prefix } = nodeDimension(ele);
        const secondLine = value ? `${prefix}: ${value}` : '';
        const maxLen = Math.max(label.length, secondLine.length);
        const fontSize = 11;
        const charWidth = fontSize * 0.6;
        const padding = 50;
        return Math.max(100, maxLen * charWidth + padding);
      },
      height: (ele) => (nodeDimension(ele).value ? 56 : 40),
      label: (ele) => {
        const label = ele.data('label') || '';
        const { value, prefix } = nodeDimension(ele);
        return value ? `${label}\n${prefix}: ${value}` : label;
      },
      'text-valign': 'center',
      'text-halign': 'center',
      'text-wrap': 'wrap',
      'text-max-width': 200,
      'font-size': 11,
      color: '#fff',
      'text-outline-width': 0,
      'background-color': (ele) => STATE_COLORS[ele.data('state')] || STATE_COLORS.unknown,
      'border-width': 2,
      'border-color': (ele) => STATE_COLORS[ele.data('state')] || STATE_COLORS.unknown,
      // Left dimension stripe via base64-encoded SVG
      'background-image': (ele) => {
        const { value } = nodeDimension(ele);
        if (!value) return 'none';
        return getStripeDataUri(getNamespaceColor(value));
      },
      'background-image-opacity': 1,
      'background-width': '12px',
      'background-height': '100%',
      'background-position-x': '0%',
      'background-position-y': '50%',
      'background-clip': 'node',
      'background-image-containment': 'over',
    },
  },
  // Compound (parent) nodes — namespace groups
  {
    selector: ':parent',
    style: {
      shape: 'round-rectangle',
      'corner-radius': 6,
      'border-width': 2,
      'border-style': 'dashed',
      'border-color': (ele) => getNamespaceColor(ele.data('label')),
      'background-color': (ele) => getNamespaceColor(ele.data('label')),
      'background-opacity': () => (isDarkTheme() ? 0.08 : 0.04),
      padding: '20px',
      label: 'data(label)',
      'text-valign': 'top',
      'text-halign': 'center',
      'font-size': 13,
      'font-weight': 'bold',
      color: (ele) => getNamespaceColor(ele.data('label')),
      'text-margin-y': -4,
      'compound-sizing-wrt-labels': 'include',
      'min-width': 120,
      'min-height': 60,
    },
  },
  // Collapsed namespace summary node — namespace color fill, state color border
  {
    selector: 'node[?isCollapsed]',
    style: {
      shape: 'round-rectangle',
      'corner-radius': 8,
      width: (ele) => {
        const label = ele.data('label') || '';
        return Math.max(140, label.length * 8 + 40);
      },
      height: 55,
      'background-color': (ele) => getNamespaceColor(ele.data('nsName')),
      'background-opacity': 1,
      'border-width': 4,
      'border-style': 'solid',
      'border-color': (ele) => STATE_COLORS[ele.data('state')] || STATE_COLORS.unknown,
      label: 'data(label)',
      'text-valign': 'center',
      'text-halign': 'center',
      'font-size': 14,
      'font-weight': 'bold',
      color: (ele) => getContrastTextColor(getNamespaceColor(ele.data('nsName'))),
      'text-wrap': 'wrap',
      'text-max-width': 180,
      'text-outline-width': 2,
      'text-outline-color': (ele) => {
        const bg = getNamespaceColor(ele.data('nsName'));
        // Subtle outline matching background for better readability
        return getContrastTextColor(bg) === '#fff' ? 'rgba(0,0,0,0.3)' : 'rgba(255,255,255,0.3)';
      },
      cursor: 'pointer',
      padding: '0px',
      'background-image': 'none',
    },
  },
  // Aggregated edges (from collapsed namespaces)
  {
    selector: 'edge[?isAggregated]',
    style: {
      width: 3,
      'line-style': 'dashed',
      'target-arrow-shape': 'triangle',
    },
  },
  // Nodes with grafanaUrl get pointer cursor
  {
    selector: 'node[grafanaUrl]',
    style: {
      cursor: 'pointer',
    },
  },
  // Edges
  {
    selector: 'edge',
    style: {
      width: (ele) => (ele.data('critical') ? 4 : 1.5),
      'curve-style': 'bezier',
      'target-arrow-shape': 'triangle',
      'target-arrow-color': (ele) => {
        if (ele.data('stale')) return EDGE_STYLES.unknown.color;
        const status = ele.data('status');
        if (status && STATUS_COLORS[status]) return STATUS_COLORS[status];
        return (EDGE_STYLES[ele.data('state')] || EDGE_STYLES.ok).color;
      },
      'line-color': (ele) => {
        if (ele.data('stale')) return EDGE_STYLES.unknown.color;
        const status = ele.data('status');
        if (status && STATUS_COLORS[status]) return STATUS_COLORS[status];
        return (EDGE_STYLES[ele.data('state')] || EDGE_STYLES.ok).color;
      },
      'line-style': (ele) => (EDGE_STYLES[ele.data('state')] || EDGE_STYLES.ok).lineStyle,
      label: (ele) => {
        const status = ele.data('status');
        const latency = ele.data('latency') || '';
        const abbr = status ? STATUS_ABBREVIATIONS[status] : null;
        if (abbr && latency) return `${abbr} ${latency}`;
        if (abbr) return abbr;
        return latency;
      },
      'font-size': 12,
      color: () => (isDarkTheme() ? '#aaa' : '#555'),
      'text-background-color': () => (isDarkTheme() ? '#2a2a2a' : '#f5f5f5'),
      'text-background-opacity': 0.8,
      'text-background-padding': '3px',
      'text-rotation': 'autorotate',
      cursor: 'pointer',
    },
  },
  // Stale nodes get dashed border
  {
    selector: 'node[?stale]',
    style: {
      'border-style': 'dashed',
    },
  },
  // Nodes with active alerts get a thicker border
  {
    selector: 'node[alertCount > 0]',
    style: {
      'border-width': 4,
      'border-style': 'double',
    },
  },
];

let isFirstRender = true;
let lastStructureSignature = '';
let severityColorMap = {}; // Map severity value -> color (e.g., {critical: '#f44336', ...})
let severityLevels = []; // Ordered array of severity levels from config

/**
 * Compute a structural signature from node IDs and edge keys.
 * Changes in state/latency don't affect the signature.
 */
function computeSignature(data) {
  const nodeIds = data.nodes.map((n) => `${n.id}:${n.type}`).sort();
  const edgeKeys = data.edges.map((e) => `${e.source}->${e.target}`).sort();
  const groupFlag = isGroupingEnabled() ? 'G' : 'F';
  const dim = getGroupingDimension();
  return groupFlag + dim + '|' + nodeIds.join(',') + '|' + edgeKeys.join(',');
}

/**
 * Update alert badge HTML overlays.
 * Renders badges as positioned div elements over the graph.
 * @param {cytoscape.Core} cy
 * @param {HTMLElement} container - parent container for badges
 */
function updateAlertBadges(cy, container) {
  // Clear existing badges
  container.innerHTML = '';

  const zoom = cy.zoom();
  const badgeScale = Math.max(0.5, Math.min(zoom, 1.5));

  // Render node badges (only for visible nodes)
  cy.nodes('[alertCount > 0]').forEach((node) => {
    // Skip if node is hidden by search filter
    if (!isElementVisible(node)) return;
    const alertCount = node.data('alertCount');
    const alertSeverity = node.data('alertSeverity');
    if (!alertCount || !alertSeverity) return;

    const color = severityColorMap[alertSeverity] || '#999';
    const pos = node.renderedPosition();
    const width = node.renderedWidth();
    const height = node.renderedHeight();

    // Badge position: top-right corner of node
    const badgeX = pos.x + width / 2 - 10;
    const badgeY = pos.y - height / 2 + 10;

    // Create badge element
    const badge = document.createElement('div');
    badge.className = 'alert-badge';
    badge.style.cssText = `
      position: absolute;
      left: ${badgeX}px;
      top: ${badgeY}px;
      background-color: ${color};
      transform: translate(-50%, -50%) scale(${badgeScale});
      pointer-events: none;
      z-index: 10;
    `;
    badge.textContent = `! ${alertCount}`;
    container.appendChild(badge);
  });

  // Render cascade warning badges (top-left corner, distinct from alert badges)
  cy.nodes('[cascadeCount > 0]').forEach((node) => {
    if (!isElementVisible(node)) return;
    // Skip Down nodes — they are the root cause, no warning badge needed.
    if (node.data('state') === 'down') return;

    const cascadeCount = node.data('cascadeCount');
    const pos = node.renderedPosition();
    const width = node.renderedWidth();
    const height = node.renderedHeight();

    // Badge position: top-left corner of node, offset right to clear namespace stripe
    const badgeX = pos.x - width / 2 + 22;
    const badgeY = pos.y - height / 2 + 10;

    const badge = document.createElement('div');
    badge.className = 'cascade-badge';
    badge.style.cssText = `
      position: absolute;
      left: ${badgeX}px;
      top: ${badgeY}px;
      transform: translate(-50%, -50%) scale(${badgeScale});
      pointer-events: none;
      z-index: 10;
    `;
    badge.textContent = `⚠ ${cascadeCount}`;
    container.appendChild(badge);
  });

  // Render edge alert markers (only for visible edges)
  cy.edges('[alertCount > 0]').forEach((edge) => {
    // Skip if edge is hidden by search filter
    if (!isElementVisible(edge)) return;
    
    const alertSeverity = edge.data('alertSeverity');
    if (!alertSeverity) return;

    const color = severityColorMap[alertSeverity] || '#999';
    const sourcePos = edge.source().renderedPosition();
    const targetPos = edge.target().renderedPosition();

    // Marker position: 20% along the edge from source
    const markerX = sourcePos.x + (targetPos.x - sourcePos.x) * 0.2;
    const markerY = sourcePos.y + (targetPos.y - sourcePos.y) * 0.2;

    // Create marker element
    const markerSize = 12 * badgeScale;
    const marker = document.createElement('div');
    marker.className = 'alert-marker';
    marker.style.cssText = `
      position: absolute;
      left: ${markerX}px;
      top: ${markerY}px;
      width: ${markerSize}px;
      height: ${markerSize}px;
      border-radius: 50%;
      background-color: ${color};
      border: 2px solid white;
      transform: translate(-50%, -50%);
      pointer-events: none;
      z-index: 10;
    `;
    container.appendChild(marker);
  });
}

/**
 * Initialize Cytoscape instance on the given container.
 * @param {HTMLElement} container
 * @param {Object} config - Application config (including alerts.severityLevels)
 * @returns {cytoscape.Core}
 */
export function initGraph(container, config) {
  isFirstRender = true;
  lastStructureSignature = '';

  // Build severity color map from config
  if (config?.alerts?.severityLevels) {
    severityLevels = config.alerts.severityLevels;
    severityColorMap = {};
    for (const level of severityLevels) {
      severityColorMap[level.value] = level.color;
    }
  }

  const cy = cytoscape({
    container,
    style: cytoscapeStyles,
    layout: { name: 'preset' },
    minZoom: 0.3,
    maxZoom: 3,
    wheelSensitivity: 0.3,
  });

  // Create HTML overlay container for alert badges
  let badgeContainer = container.querySelector('.alert-badge-container');
  if (!badgeContainer) {
    badgeContainer = document.createElement('div');
    badgeContainer.className = 'alert-badge-container';
    badgeContainer.style.cssText = `
      position: absolute;
      top: 0;
      left: 0;
      width: 100%;
      height: 100%;
      pointer-events: none;
      z-index: 5;
    `;
    container.appendChild(badgeContainer);
  }

  // Update badges on render, pan, zoom
  const updateBadges = () => updateAlertBadges(cy, badgeContainer);
  cy.on('render pan zoom', updateBadges);

  return cy;
}

/**
 * Render topology data into the Cytoscape instance.
 * Uses smart diffing: if only data attributes changed (state, latency),
 * updates in-place without re-running dagre layout.
 * @param {cytoscape.Core} cy
 * @param {{nodes: Array, edges: Array, alerts: Array}} data
 * @param {Object} config - Application config (for alerts severity)
 */
export function renderGraph(cy, data, config) {
  const signature = computeSignature(data);
  const structureChanged = signature !== lastStructureSignature;
  lastStructureSignature = signature;

  // Count alerts per node (service = source).
  const alertCounts = {};
  if (data.alerts) {
    for (const a of data.alerts) {
      alertCounts[a.service] = (alertCounts[a.service] || 0) + 1;
    }
  }

  if (!structureChanged && !isFirstRender) {
    // Structure unchanged — update data attributes only, skip layout
    cy.batch(() => {
      for (const node of data.nodes) {
        const ele = cy.getElementById(node.id);
        if (ele.length) {
          ele.data('state', node.state);
          ele.data('stale', node.stale || false);
          ele.data('alertCount', alertCounts[node.id] || 0);
          ele.data('alertSeverity', node.alertSeverity || undefined);
        }
      }
      for (const edge of data.edges) {
        const id = `${edge.source}->${edge.target}`;
        const ele = cy.getElementById(id);
        if (ele.length) {
          ele.data('type', edge.type || undefined);
          ele.data('latency', edge.latency);
          ele.data('latencyRaw', edge.latencyRaw || 0);
          ele.data('health', edge.health ?? -1);
          ele.data('state', edge.state);
          ele.data('stale', edge.stale || false);
          ele.data('critical', edge.critical);
          ele.data('status', edge.status || undefined);
          ele.data('detail', edge.detail || undefined);
          ele.data('alertCount', edge.alertCount || 0);
          ele.data('alertSeverity', edge.alertSeverity || undefined);
        }
      }
    });
    cy.style().update();
    return false; // no structure change
  }

  // Structure changed — full rebuild
  const grouping = isGroupingEnabled();
  let parentMap;

  cy.batch(() => {
    cy.elements().remove();

    // Add compound parent nodes first when grouping is enabled
    if (grouping) {
      const compound = buildCompoundElements(data);
      parentMap = compound.parentMap;
      for (const parent of compound.parents) {
        cy.add(parent);
      }
    }

    for (const node of data.nodes) {
      // For dependency nodes without namespace, try to extract from host label
      const ns = node.namespace || (node.type !== 'service' ? extractNamespaceFromHost(node.label) : null);
      const nodeData = {
        id: node.id,
        label: node.label,
        state: node.state,
        stale: node.stale || false,
        type: node.type,
        namespace: ns || undefined,
        group: node.group || undefined,
        alertCount: alertCounts[node.id] || 0,
        alertSeverity: node.alertSeverity || undefined,
        grafanaUrl: node.grafanaUrl || undefined,
      };
      // Assign parent when grouping is enabled and node has a namespace
      if (grouping && parentMap && parentMap.has(node.id)) {
        nodeData.parent = parentMap.get(node.id);
      }
      cy.add({ group: 'nodes', data: nodeData });
    }

    for (const edge of data.edges) {
      cy.add({
        group: 'edges',
        data: {
          id: `${edge.source}->${edge.target}`,
          source: edge.source,
          target: edge.target,
          type: edge.type || undefined,
          latency: edge.latency,
          latencyRaw: edge.latencyRaw || 0,
          health: edge.health ?? -1,
          state: edge.state,
          stale: edge.stale || false,
          critical: edge.critical,
          status: edge.status || undefined,
          detail: edge.detail || undefined,
          alertCount: edge.alertCount || 0,
          alertSeverity: edge.alertSeverity || undefined,
          grafanaUrl: edge.grafanaUrl || undefined,
        },
      });
    }
  });

  if (grouping) {
    cy.layout({
      name: 'fcose',
      animate: false,
      quality: 'default',
      nodeSeparation: 80,
      idealEdgeLength: 120,
      nodeRepulsion: 6000,
      tile: true,
    }).run();
  } else {
    cy.layout({
      name: 'dagre',
      rankDir: layoutDirection,
      nodeSep: 80,
      rankSep: 120,
      animate: false,
    }).run();
  }

  if (isFirstRender) {
    cy.fit(50);
    isFirstRender = false;
  }

  return true; // structure changed
}

/**
 * Force Cytoscape to re-evaluate theme-dependent style functions (edge labels).
 * @param {cytoscape.Core} cy
 */
export function updateGraphTheme(cy) {
  if (cy) {
    cy.style().update();
  }
}

/**
 * Set the layout direction for future renders.
 * @param {string} direction - Layout direction: 'TB' or 'LR'
 */
export function setLayoutDirection(direction) {
  layoutDirection = direction;
}

/**
 * Re-run layout with specified direction.
 * @param {cytoscape.Core} cy - Cytoscape instance
 * @param {string} direction - Layout direction: 'TB' (top-bottom) or 'LR' (left-right)
 */
export function relayout(cy, direction = 'TB') {
  if (!cy) return;
  layoutDirection = direction; // Update global direction
  if (isGroupingEnabled()) {
    cy.layout({
      name: 'fcose',
      animate: true,
      animationDuration: 500,
      quality: 'default',
      nodeSeparation: 80,
      idealEdgeLength: 120,
      nodeRepulsion: 6000,
      tile: true,
    }).run();
  } else {
    cy.layout({
      name: 'dagre',
      rankDir: direction,
      nodeSep: 80,
      rankSep: 120,
      animate: true,
      animationDuration: 500,
    }).run();
  }
}
