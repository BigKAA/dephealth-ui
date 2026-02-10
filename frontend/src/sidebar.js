/**
 * Node detail sidebar functionality.
 * Shows node info, related alerts, and connected edges on node click.
 */

let topologyDataCache = null;

const $ = (sel) => document.querySelector(sel);

/**
 * Initialize sidebar interactions.
 * @param {cytoscape.Core} cy - Cytoscape instance
 * @param {object} topologyData - Full topology data (nodes, edges, alerts)
 */
export function initSidebar(cy, topologyData) {
  topologyDataCache = topologyData;

  const sidebar = $('#node-sidebar');
  const closeBtn = $('#btn-sidebar-close');

  // Single tap on node: open sidebar
  cy.on('tap', 'node', (evt) => {
    const node = evt.target;
    openSidebar(node, cy);
  });

  // Double tap on node with Grafana URL: open Grafana in new tab
  cy.on('dbltap', 'node[grafanaUrl]', (evt) => {
    const url = evt.target.data('grafanaUrl');
    if (url) window.open(url, '_blank');
  });

  // Edge tap: open Grafana (if URL exists)
  cy.on('tap', 'edge[grafanaUrl]', (evt) => {
    const url = evt.target.data('grafanaUrl');
    if (url) window.open(url, '_blank');
  });

  // Close button
  closeBtn.addEventListener('click', closeSidebar);

  // Click outside sidebar to close
  document.addEventListener('click', (e) => {
    if (
      !sidebar.classList.contains('hidden') &&
      !sidebar.contains(e.target) &&
      !e.target.closest('#cy')
    ) {
      closeSidebar();
    }
  });
}

/**
 * Update topology data cache (called on each refresh).
 * @param {object} topologyData - Full topology data
 */
export function updateSidebarData(topologyData) {
  topologyDataCache = topologyData;
}

/**
 * Open sidebar with node details.
 * @param {cytoscape.NodeSingular} node - Cytoscape node
 * @param {cytoscape.Core} cy - Cytoscape instance
 */
function openSidebar(node, cy) {
  const sidebar = $('#node-sidebar');
  const data = node.data();

  // Title
  $('#sidebar-title').textContent = data.label || data.id;

  // Details section
  renderDetails(data);

  // Alerts section
  renderAlerts(data.id);

  // Connected edges section
  renderEdges(node, cy);

  // Actions section
  renderActions(data);

  // Show sidebar
  sidebar.classList.remove('hidden');
}

/**
 * Close sidebar.
 */
function closeSidebar() {
  $('#node-sidebar').classList.add('hidden');
}

/**
 * Render node details section.
 * @param {object} data - Node data
 */
function renderDetails(data) {
  const section = $('#sidebar-details');
  const stateBadgeClass = `sidebar-state-badge ${data.state || 'unknown'}`;

  const details = [
    { label: 'State', value: `<span class="${stateBadgeClass}">${data.state || 'unknown'}</span>` },
    data.namespace && { label: 'Namespace', value: data.namespace },
    data.type && { label: 'Type', value: data.type },
    data.host && { label: 'Host', value: data.host },
    data.port && { label: 'Port', value: data.port },
    data.alertCount > 0 && { label: 'Active Alerts', value: data.alertCount },
  ].filter(Boolean);

  section.innerHTML = details
    .map(
      (item) => `
    <div class="sidebar-detail-row">
      <span class="sidebar-detail-label">${item.label}:</span>
      <span class="sidebar-detail-value">${item.value}</span>
    </div>
  `
    )
    .join('');
}

/**
 * Render related alerts section.
 * @param {string} nodeId - Node ID
 */
function renderAlerts(nodeId) {
  const section = $('#sidebar-alerts');
  if (!topologyDataCache || !topologyDataCache.alerts) {
    section.innerHTML = '';
    return;
  }

  const nodeAlerts = topologyDataCache.alerts.filter(
    (alert) => alert.service === nodeId || alert.dependency === nodeId
  );

  if (nodeAlerts.length === 0) {
    section.innerHTML = '';
    return;
  }

  section.innerHTML = `
    <div class="sidebar-section-title">Active Alerts (${nodeAlerts.length})</div>
    ${nodeAlerts
      .map(
        (alert) => `
      <div class="sidebar-alert-item ${alert.severity || 'info'}">
        <div class="sidebar-alert-name">${alert.alertname || 'Unknown Alert'}</div>
        <div class="sidebar-alert-meta">
          ${alert.severity ? `<strong>${alert.severity.toUpperCase()}</strong>` : ''}
          ${alert.service !== nodeId && alert.service ? ` • Service: ${alert.service}` : ''}
          ${alert.dependency !== nodeId && alert.dependency ? ` • Dependency: ${alert.dependency}` : ''}
        </div>
      </div>
    `
      )
      .join('')}
  `;
}

/**
 * Render connected edges section.
 * @param {cytoscape.NodeSingular} node - Cytoscape node
 * @param {cytoscape.Core} cy - Cytoscape instance
 */
function renderEdges(node, cy) {
  const section = $('#sidebar-edges');
  const connectedEdges = node.connectedEdges();

  if (connectedEdges.length === 0) {
    section.innerHTML = '';
    return;
  }

  const edgeInfos = connectedEdges.map((edge) => {
    const source = edge.source();
    const target = edge.target();
    const data = edge.data();

    const isOutgoing = source.id() === node.id();
    const otherNode = isOutgoing ? target : source;
    const direction = isOutgoing ? '→' : '←';

    return {
      label: `${direction} ${otherNode.data('label') || otherNode.id()}`,
      latency: data.latency || '—',
    };
  });

  section.innerHTML = `
    <div class="sidebar-section-title">Connected Edges (${edgeInfos.length})</div>
    ${edgeInfos
      .map(
        (info) => `
      <div class="sidebar-edge-item">
        <span class="sidebar-edge-label">${info.label}</span>
        <span class="sidebar-edge-latency">${info.latency}</span>
      </div>
    `
      )
      .join('')}
  `;
}

/**
 * Render actions section.
 * @param {object} data - Node data
 */
function renderActions(data) {
  const section = $('#sidebar-actions');

  if (!data.grafanaUrl) {
    section.innerHTML = '';
    return;
  }

  section.innerHTML = `
    <button class="sidebar-button" id="sidebar-grafana-btn">
      <i class="bi bi-graph-up"></i>
      Open in Grafana
    </button>
  `;

  $('#sidebar-grafana-btn').addEventListener('click', () => {
    window.open(data.grafanaUrl, '_blank');
  });
}
