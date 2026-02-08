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
      width: 2,
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

/**
 * Initialize Cytoscape instance on the given container.
 * @param {HTMLElement} container
 * @returns {cytoscape.Core}
 */
export function initGraph(container) {
  isFirstRender = true;
  return cytoscape({
    container,
    style: cytoscapeStyles,
    layout: { name: 'preset' },
    minZoom: 0.3,
    maxZoom: 3,
    wheelSensitivity: 0.3,
  });
}

/**
 * Render topology data into the Cytoscape instance.
 * @param {cytoscape.Core} cy
 * @param {{nodes: Array, edges: Array, alerts: Array}} data
 */
export function renderGraph(cy, data) {
  // Count alerts per node (service = source).
  const alertCounts = {};
  if (data.alerts) {
    for (const a of data.alerts) {
      alertCounts[a.service] = (alertCounts[a.service] || 0) + 1;
    }
  }

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
