import cytoscape from 'cytoscape';
import dagre from 'cytoscape-dagre';

cytoscape.use(dagre);

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
      width: 140,
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
      width: 120,
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
 * Render alert badges on the canvas overlay.
 * Called on each Cytoscape render event.
 * @param {CanvasRenderingContext2D} ctx
 * @param {cytoscape.Core} cy
 */
function renderAlertBadges(ctx, cy) {
  const zoom = cy.zoom();
  const pan = cy.pan();

  // Render node badges
  cy.nodes('[alertCount > 0]').forEach((node) => {
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
    const badgeRadius = 10;

    // Draw badge circle
    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.arc(badgeX, badgeY, badgeRadius, 0, 2 * Math.PI);
    ctx.fill();

    // Draw alert count text
    ctx.fillStyle = '#fff';
    ctx.font = `bold ${Math.max(10, 10 * zoom)}px sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(alertCount.toString(), badgeX, badgeY);
  });

  // Render edge alert markers
  cy.edges('[alertCount > 0]').forEach((edge) => {
    const alertSeverity = edge.data('alertSeverity');
    if (!alertSeverity) return;

    const color = severityColorMap[alertSeverity] || '#999';
    const sourcePos = edge.source().renderedPosition();
    const targetPos = edge.target().renderedPosition();

    // Marker position: 20% along the edge from source
    const markerX = sourcePos.x + (targetPos.x - sourcePos.x) * 0.2;
    const markerY = sourcePos.y + (targetPos.y - sourcePos.y) * 0.2;
    const markerRadius = 6;

    // Draw marker circle
    ctx.fillStyle = color;
    ctx.strokeStyle = '#fff';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(markerX, markerY, markerRadius, 0, 2 * Math.PI);
    ctx.fill();
    ctx.stroke();
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

  // Render alert badges on canvas overlay
  cy.on('render', (evt) => {
    const canvas = evt.cy.container().querySelector('canvas');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    renderAlertBadges(ctx, evt.cy);
  });

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
    rankDir: 'TB',
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
