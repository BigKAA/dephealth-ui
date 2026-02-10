import cytoscape from 'cytoscape';
import dagre from 'cytoscape-dagre';
import { isElementVisible } from './search.js';

cytoscape.use(dagre);

let layoutDirection = 'TB'; // Global layout direction: 'TB' or 'LR'

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
};

const cytoscapeStyles = [
  // Service nodes
  {
    selector: 'node[type="service"]',
    style: {
      shape: 'round-rectangle',
      width: (ele) => {
        const label = ele.data('label') || '';
        const fontSize = 12;
        const charWidth = fontSize * 0.6; // approximate width per character
        const padding = 40; // horizontal padding
        return Math.max(100, label.length * charWidth + padding);
      },
      height: 40,
      label: 'data(label)',
      'text-valign': 'center',
      'text-halign': 'center',
      'font-size': 12,
      color: '#fff',
      'text-outline-width': 0,
      'background-color': (ele) => STATE_COLORS[ele.data('state')] || STATE_COLORS.unknown,
      'border-width': 2,
      'border-color': (ele) => {
        const c = STATE_COLORS[ele.data('state')] || STATE_COLORS.unknown;
        return c;
      },
    },
  },
  // Dependency nodes
  {
    selector: 'node[type!="service"]',
    style: {
      shape: 'ellipse',
      width: (ele) => {
        const label = ele.data('label') || '';
        const fontSize = 11;
        const charWidth = fontSize * 0.6; // approximate width per character
        const padding = 50; // extra padding for ellipse shape
        return Math.max(100, label.length * charWidth + padding);
      },
      height: 40,
      label: 'data(label)',
      'text-valign': 'center',
      'text-halign': 'center',
      'font-size': 11,
      color: '#fff',
      'text-outline-width': 0,
      'background-color': (ele) => STATE_COLORS[ele.data('state')] || STATE_COLORS.unknown,
      'border-width': 2,
      'border-color': (ele) => STATE_COLORS[ele.data('state')] || STATE_COLORS.unknown,
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
      width: (ele) => (ele.data('critical') ? 3 : 1.5),
      'curve-style': 'bezier',
      'target-arrow-shape': 'triangle',
      'target-arrow-color': (ele) => (EDGE_STYLES[ele.data('state')] || EDGE_STYLES.ok).color,
      'line-color': (ele) => (EDGE_STYLES[ele.data('state')] || EDGE_STYLES.ok).color,
      'line-style': (ele) => (EDGE_STYLES[ele.data('state')] || EDGE_STYLES.ok).lineStyle,
      label: 'data(latency)',
      'font-size': 10,
      color: () => (isDarkTheme() ? '#aaa' : '#555'),
      'text-background-color': () => (isDarkTheme() ? '#2a2a2a' : '#f5f5f5'),
      'text-background-opacity': 0.8,
      'text-background-padding': '2px',
      'text-rotation': 'autorotate',
    },
  },
  // Edges with grafanaUrl get pointer cursor
  {
    selector: 'edge[grafanaUrl]',
    style: {
      cursor: 'pointer',
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
  const nodeIds = data.nodes.map((n) => n.id).sort();
  const edgeKeys = data.edges.map((e) => `${e.source}->${e.target}`).sort();
  return nodeIds.join(',') + '|' + edgeKeys.join(',');
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
      width: 20px;
      height: 20px;
      border-radius: 50%;
      background-color: ${color};
      color: white;
      font-size: 10px;
      font-weight: bold;
      display: flex;
      align-items: center;
      justify-content: center;
      transform: translate(-50%, -50%);
      pointer-events: none;
      z-index: 10;
    `;
    badge.textContent = alertCount;
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
    const marker = document.createElement('div');
    marker.className = 'alert-marker';
    marker.style.cssText = `
      position: absolute;
      left: ${markerX}px;
      top: ${markerY}px;
      width: 12px;
      height: 12px;
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
          ele.data('alertCount', alertCounts[node.id] || 0);
          ele.data('alertSeverity', node.alertSeverity || undefined);
        }
      }
      for (const edge of data.edges) {
        const id = `${edge.source}->${edge.target}`;
        const ele = cy.getElementById(id);
        if (ele.length) {
          ele.data('latency', edge.latency);
          ele.data('state', edge.state);
          ele.data('critical', edge.critical);
          ele.data('alertCount', edge.alertCount || 0);
          ele.data('alertSeverity', edge.alertSeverity || undefined);
        }
      }
    });
    cy.style().update();
    return;
  }

  // Structure changed — full rebuild
  cy.batch(() => {
    cy.elements().remove();

    for (const node of data.nodes) {
      cy.add({
        group: 'nodes',
        data: {
          id: node.id,
          label: node.label,
          state: node.state,
          type: node.type,
          alertCount: alertCounts[node.id] || 0,
          alertSeverity: node.alertSeverity || undefined,
          grafanaUrl: node.grafanaUrl || undefined,
        },
      });
    }

    for (const edge of data.edges) {
      cy.add({
        group: 'edges',
        data: {
          id: `${edge.source}->${edge.target}`,
          source: edge.source,
          target: edge.target,
          latency: edge.latency,
          state: edge.state,
          critical: edge.critical,
          alertCount: edge.alertCount || 0,
          alertSeverity: edge.alertSeverity || undefined,
          grafanaUrl: edge.grafanaUrl || undefined,
        },
      });
    }
  });

  cy.layout({
    name: 'dagre',
    rankDir: layoutDirection,
    nodeSep: 80,
    rankSep: 120,
    animate: false,
  }).run();

  if (isFirstRender) {
    cy.fit(50);
    isFirstRender = false;
  }
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
  cy.layout({
    name: 'dagre',
    rankDir: direction,
    nodeSep: 80,
    rankSep: 120,
    animate: true,
    animationDuration: 500,
  }).run();
}
