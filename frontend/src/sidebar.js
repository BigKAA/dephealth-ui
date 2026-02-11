/**
 * Sidebar functionality for node and edge details.
 * Shows info, related alerts, instances, and connected edges/nodes on click.
 */

import { fetchInstances } from './api.js';
import { t } from './i18n.js';

let topologyDataCache = null;
let currentNodeId = null; // Track currently opened node for toggle behavior
let currentEdgeId = null; // Track currently opened edge for toggle behavior
let grafanaConfig = null; // Grafana config from /api/v1/config
let highlightedElement = null; // Track currently highlighted element for cleanup
let highlightTimer = null; // Timer for highlight auto-clear

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

  // Single tap on node: toggle sidebar (skip namespace group nodes)
  cy.on('tap', 'node', (evt) => {
    const node = evt.target;
    if (node.data('isGroup')) return;
    const nodeId = node.data('id');
    const sidebar = $('#node-sidebar');

    // If clicking the same node while sidebar is open - close it
    if (currentNodeId === nodeId && !sidebar.classList.contains('hidden')) {
      closeSidebar();
    } else {
      openSidebar(node, cy);
    }
  });

  // Double tap on node with Grafana URL: open Grafana in new tab (skip group nodes)
  cy.on('dbltap', 'node[grafanaUrl]', (evt) => {
    if (evt.target.data('isGroup')) return;
    const url = evt.target.data('grafanaUrl');
    if (url) window.open(url, '_blank');
  });

  // Single tap on edge: toggle edge sidebar
  cy.on('tap', 'edge', (evt) => {
    const edge = evt.target;
    const edgeId = edge.data('id');
    const sidebar = $('#node-sidebar');

    // If clicking the same edge while sidebar is open - close it
    if (currentEdgeId === edgeId && !sidebar.classList.contains('hidden')) {
      closeSidebar();
    } else {
      openEdgeSidebar(edge, cy);
    }
  });

  // Close button
  closeBtn.addEventListener('click', closeSidebar);

  // Click outside sidebar to close (exclude alert drawer and toolbar buttons)
  document.addEventListener('click', (e) => {
    if (
      !sidebar.classList.contains('hidden') &&
      !sidebar.contains(e.target) &&
      !e.target.closest('#cy') &&
      !e.target.closest('#alert-drawer') &&
      !e.target.closest('#btn-alerts')
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
 * Set Grafana config for sidebar dashboard links.
 * @param {object} config - Config object from /api/v1/config
 */
export function setGrafanaConfig(config) {
  if (config && config.grafana) {
    grafanaConfig = config.grafana;
  }
}

/**
 * Open sidebar with node details.
 * @param {cytoscape.NodeSingular} node - Cytoscape node
 * @param {cytoscape.Core} cy - Cytoscape instance
 */
export function openSidebar(node, cy) {
  const sidebar = $('#node-sidebar');
  const data = node.data();

  // Track current node for toggle behavior
  currentNodeId = data.id;
  currentEdgeId = null;

  // Title
  $('#sidebar-title').textContent = data.label || data.id;

  // Details section
  renderDetails(data);

  // Alerts section
  renderAlerts(data.id);

  // Instances section (for service nodes only)
  if (data.type === 'service') {
    renderInstances(data.id, data.label || data.id);
  } else {
    $('#sidebar-instances').innerHTML = '';
  }

  // Connected edges section
  renderEdges(node, cy);

  // Actions section
  renderActions(data);

  // Grafana dashboards section
  renderGrafanaDashboards(data);

  // Show sidebar
  sidebar.classList.remove('hidden');
}

/**
 * Close sidebar.
 */
function closeSidebar() {
  $('#node-sidebar').classList.add('hidden');
  currentNodeId = null;
  currentEdgeId = null;
  clearHighlight();
}

/**
 * Render node details section.
 * @param {object} data - Node data
 */
function renderDetails(data) {
  const section = $('#sidebar-details');
  const stateBadgeClass = `sidebar-state-badge ${data.state || 'unknown'}`;

  const staleDetail = data.stale ? ` <span class="sidebar-stale-hint">${t('state.unknown.detail')}</span>` : '';
  const details = [
    { label: t('sidebar.state'), value: `<span class="${stateBadgeClass}">${data.state || 'unknown'}</span>${staleDetail}` },
    data.namespace && { label: t('sidebar.namespace'), value: data.namespace },
    data.type && { label: t('sidebar.type'), value: data.type },
    data.host && { label: t('sidebar.host'), value: data.host },
    data.port && { label: t('sidebar.port'), value: data.port },
    data.alertCount > 0 && { label: t('sidebar.activeAlerts'), value: data.alertCount },
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
    <div class="sidebar-section-title">${t('sidebar.activeAlertsCount', { count: nodeAlerts.length })}</div>
    ${nodeAlerts
      .map(
        (alert) => `
      <div class="sidebar-alert-item ${alert.severity || 'info'}">
        <div class="sidebar-alert-name">${alert.alertname || t('alerts.unknownAlert')}</div>
        <div class="sidebar-alert-meta">
          ${alert.severity ? `<strong>${alert.severity.toUpperCase()}</strong>` : ''}
          ${alert.service !== nodeId && alert.service ? ` &bull; ${t('alerts.service', { name: alert.service })}` : ''}
          ${alert.dependency !== nodeId && alert.dependency ? ` &bull; ${t('alerts.dependency', { name: alert.dependency })}` : ''}
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

  // Separate incoming and outgoing edges
  const outgoing = [];
  const incoming = [];

  connectedEdges.forEach((edge) => {
    const source = edge.source();
    const target = edge.target();
    const data = edge.data();

    const isOutgoing = source.id() === node.id();
    const otherNode = isOutgoing ? target : source;

    const edgeInfo = {
      edgeId: data.id,
      label: otherNode.data('label') || otherNode.id(),
      latency: data.stale ? '—' : (data.latency || '—'),
      stale: data.stale || false,
    };

    if (isOutgoing) {
      outgoing.push(edgeInfo);
    } else {
      incoming.push(edgeInfo);
    }
  });

  const renderItem = (info, arrow) => `
    <div class="sidebar-edge-item${info.stale ? ' stale' : ''}" data-edge-id="${info.edgeId}">
      <span class="sidebar-edge-label">${arrow} ${info.label}</span>
      <span class="sidebar-edge-latency">${info.latency}</span>
      <span class="sidebar-edge-action">${t('sidebar.edge.goToEdge')} →</span>
    </div>
  `;

  // Build HTML with separated groups
  let html = `<div class="sidebar-section-title">${t('sidebar.connectedEdges', { count: connectedEdges.length })}</div>`;

  if (outgoing.length > 0) {
    html += `
      <div class="sidebar-edge-group">
        <div class="sidebar-edge-group-title">${t('sidebar.outgoingEdges', { count: outgoing.length })}</div>
        ${outgoing.map((info) => renderItem(info, '→')).join('')}
      </div>
    `;
  }

  if (incoming.length > 0) {
    html += `
      <div class="sidebar-edge-group">
        <div class="sidebar-edge-group-title">${t('sidebar.incomingEdges', { count: incoming.length })}</div>
        ${incoming.map((info) => renderItem(info, '←')).join('')}
      </div>
    `;
  }

  section.innerHTML = html;

  // Attach click handlers: navigate to edge + open edge sidebar
  section.querySelectorAll('.sidebar-edge-item[data-edge-id]').forEach((el) => {
    el.addEventListener('click', (e) => {
      e.stopPropagation(); // Prevent click-outside handler from closing sidebar
      const edgeId = el.dataset.edgeId;
      const edge = cy.getElementById(edgeId);
      if (edge && edge.length) {
        cy.animate({ center: { eles: edge }, duration: 300 });
        highlightElement(edge);
        openEdgeSidebar(edge, cy);
      }
    });
  });
}

/**
 * Clear highlight from previously highlighted element.
 * Removes inline overlay styles and cancels the auto-clear timer.
 */
function clearHighlight() {
  if (highlightTimer) {
    clearTimeout(highlightTimer);
    highlightTimer = null;
  }
  if (highlightedElement) {
    highlightedElement.removeStyle('overlay-color overlay-opacity overlay-padding');
    highlightedElement = null;
  }
}

/**
 * Highlight an element (node or edge) with a visible overlay dot.
 * Sets overlay immediately, auto-clears after 1.5s.
 * @param {cytoscape.NodeSingular|cytoscape.EdgeSingular} ele
 */
function highlightElement(ele) {
  clearHighlight();
  highlightedElement = ele;

  ele.style({
    'overlay-color': '#2196f3',
    'overlay-opacity': 0.35,
    'overlay-padding': ele.isEdge() ? 8 : 14,
  });

  highlightTimer = setTimeout(() => clearHighlight(), 1500);
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
      ${t('sidebar.openGrafana')}
    </button>
  `;

  $('#sidebar-grafana-btn').addEventListener('click', () => {
    window.open(data.grafanaUrl, '_blank');
  });
}

/**
 * Render Grafana dashboards section with links to all dashboards.
 * Context-aware: pre-fills variables when a node is selected.
 * @param {object} data - Currently selected node data
 */
function renderGrafanaDashboards(data) {
  const section = $('#sidebar-grafana');
  if (!section) return;

  if (!grafanaConfig || !grafanaConfig.baseUrl) {
    section.innerHTML = '';
    return;
  }

  const base = grafanaConfig.baseUrl;
  const db = grafanaConfig.dashboards || {};

  // Build dashboard links with context-aware parameters
  const dashboards = [];

  if (db.serviceList) {
    dashboards.push({
      label: t('sidebar.grafana.serviceList'),
      url: `${base}/d/${db.serviceList}/`,
    });
  }
  if (db.servicesStatus) {
    dashboards.push({
      label: t('sidebar.grafana.servicesStatus'),
      url: `${base}/d/${db.servicesStatus}/`,
    });
  }
  if (db.linksStatus) {
    dashboards.push({
      label: t('sidebar.grafana.linksStatus'),
      url: `${base}/d/${db.linksStatus}/`,
    });
  }
  if (db.serviceStatus) {
    let url = `${base}/d/${db.serviceStatus}/`;
    // Context-aware: add service variable when a service node is selected
    if (data && data.type === 'service' && data.id) {
      url += `?var-service=${encodeURIComponent(data.id)}`;
    }
    dashboards.push({
      label: t('sidebar.grafana.serviceStatus'),
      url,
    });
  }
  if (db.linkStatus) {
    let url = `${base}/d/${db.linkStatus}/`;
    // Context-aware: add link variables when node has connection details
    if (data && data.host && data.port) {
      const params = new URLSearchParams();
      if (data.label) params.set('var-dependency', data.label);
      if (data.host) params.set('var-host', data.host);
      if (data.port) params.set('var-port', data.port);
      url += `?${params.toString()}`;
    }
    dashboards.push({
      label: t('sidebar.grafana.linkStatus'),
      url,
    });
  }

  if (dashboards.length === 0) {
    section.innerHTML = '';
    return;
  }

  section.innerHTML = `
    <div class="sidebar-section-title">${t('sidebar.grafanaDashboards')}</div>
    ${dashboards
      .map(
        (d) => `
      <a href="${d.url}" target="_blank" rel="noopener" class="sidebar-grafana-link">
        <i class="bi bi-graph-up"></i>
        <span>${d.label}</span>
        <i class="bi bi-box-arrow-up-right sidebar-grafana-external"></i>
      </a>
    `
      )
      .join('')}
  `;
}

/**
 * Open sidebar with edge details.
 * @param {cytoscape.EdgeSingular} edge - Cytoscape edge
 * @param {cytoscape.Core} cy - Cytoscape instance
 */
export function openEdgeSidebar(edge, cy) {
  const sidebar = $('#node-sidebar');
  const data = edge.data();

  // Track current edge for toggle behavior
  currentEdgeId = data.id;
  currentNodeId = null;

  // Get source and target node labels
  const sourceNode = cy.getElementById(data.source);
  const targetNode = cy.getElementById(data.target);
  const sourceLabel = sourceNode.data('label') || data.source;
  const targetLabel = targetNode.data('label') || data.target;

  // Title: source → target
  $('#sidebar-title').textContent = `${sourceLabel} → ${targetLabel}`;

  // Details section
  renderEdgeDetails(data);

  // Alerts section (match by source + target)
  renderEdgeAlerts(data.source, data.target);

  // Instances section: empty for edges
  $('#sidebar-instances').innerHTML = '';

  // Connected nodes section (replaces edges section for node sidebar)
  renderConnectedNodes(sourceNode, targetNode, cy);

  // Actions section
  renderActions(data);

  // Grafana dashboards section (context-aware for edges)
  renderEdgeGrafanaDashboards(data, sourceLabel, targetLabel);

  // Show sidebar
  sidebar.classList.remove('hidden');
}

/**
 * Render edge details section.
 * @param {object} data - Edge data
 */
function renderEdgeDetails(data) {
  const section = $('#sidebar-details');
  const stateBadgeClass = `sidebar-state-badge ${data.state || 'unknown'}`;

  const staleDetail = data.stale ? ` <span class="sidebar-stale-hint">${t('state.unknown.detail')}</span>` : '';
  const details = [
    { label: t('sidebar.state'), value: `<span class="${stateBadgeClass}">${data.state || 'unknown'}</span>${staleDetail}` },
    data.type && { label: t('sidebar.edge.type'), value: data.type },
    { label: t('sidebar.edge.latency'), value: data.stale ? '—' : (data.latency || '—') },
    { label: t('sidebar.edge.critical'), value: data.critical ? t('sidebar.edge.criticalYes') : t('sidebar.edge.criticalNo') },
    data.alertCount > 0 && { label: t('sidebar.activeAlerts'), value: data.alertCount },
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
 * Render alerts related to a specific edge (matched by source + target).
 * @param {string} source - Source node ID
 * @param {string} target - Target node ID
 */
function renderEdgeAlerts(source, target) {
  const section = $('#sidebar-alerts');
  if (!topologyDataCache || !topologyDataCache.alerts) {
    section.innerHTML = '';
    return;
  }

  const edgeAlerts = topologyDataCache.alerts.filter(
    (alert) => alert.service === source && alert.dependency === target
  );

  if (edgeAlerts.length === 0) {
    section.innerHTML = '';
    return;
  }

  section.innerHTML = `
    <div class="sidebar-section-title">${t('sidebar.activeAlertsCount', { count: edgeAlerts.length })}</div>
    ${edgeAlerts
      .map(
        (alert) => `
      <div class="sidebar-alert-item ${alert.severity || 'info'}">
        <div class="sidebar-alert-name">${alert.alertname || t('alerts.unknownAlert')}</div>
        <div class="sidebar-alert-meta">
          ${alert.severity ? `<strong>${alert.severity.toUpperCase()}</strong>` : ''}
        </div>
      </div>
    `
      )
      .join('')}
  `;
}

/**
 * Render connected nodes section for edge sidebar.
 * Shows source and target nodes as clickable links.
 * @param {cytoscape.NodeSingular} sourceNode
 * @param {cytoscape.NodeSingular} targetNode
 * @param {cytoscape.Core} cy
 */
function renderConnectedNodes(sourceNode, targetNode, cy) {
  const section = $('#sidebar-edges');

  const sourceLabel = sourceNode.data('label') || sourceNode.id();
  const targetLabel = targetNode.data('label') || targetNode.id();
  const sourceState = sourceNode.data('state') || 'unknown';
  const targetState = targetNode.data('state') || 'unknown';

  section.innerHTML = `
    <div class="sidebar-section-title">${t('sidebar.edge.connectedNodes')}</div>
    <div class="sidebar-node-link" data-node-id="${sourceNode.id()}">
      <span class="sidebar-state-dot ${sourceState}"></span>
      <span class="sidebar-node-link-label">
        <span class="sidebar-node-link-role">${t('sidebar.edge.source')}:</span>
        ${sourceLabel}
      </span>
      <span class="sidebar-node-link-action">${t('sidebar.edge.goToNode')} →</span>
    </div>
    <div class="sidebar-node-link" data-node-id="${targetNode.id()}">
      <span class="sidebar-state-dot ${targetState}"></span>
      <span class="sidebar-node-link-label">
        <span class="sidebar-node-link-role">${t('sidebar.edge.target')}:</span>
        ${targetLabel}
      </span>
      <span class="sidebar-node-link-action">${t('sidebar.edge.goToNode')} →</span>
    </div>
  `;

  // Attach click handlers for node navigation
  section.querySelectorAll('.sidebar-node-link[data-node-id]').forEach((el) => {
    el.addEventListener('click', (e) => {
      e.stopPropagation(); // Prevent click-outside handler from closing sidebar
      const nodeId = el.dataset.nodeId;
      const node = cy.getElementById(nodeId);
      if (node && node.length) {
        // Center node on graph with animation
        cy.animate({ center: { eles: node }, duration: 300 });
        highlightElement(node);
        // Open node sidebar
        openSidebar(node, cy);
      }
    });
  });
}

/**
 * Render Grafana dashboards section for edge sidebar.
 * Context-aware: pre-fills link variables for the edge.
 * @param {object} data - Edge data
 * @param {string} sourceLabel - Source node label
 * @param {string} targetLabel - Target node label
 */
function renderEdgeGrafanaDashboards(data, sourceLabel, targetLabel) {
  const section = $('#sidebar-grafana');
  if (!section) return;

  if (!grafanaConfig || !grafanaConfig.baseUrl) {
    section.innerHTML = '';
    return;
  }

  const base = grafanaConfig.baseUrl;
  const db = grafanaConfig.dashboards || {};

  const dashboards = [];

  if (db.linksStatus) {
    dashboards.push({
      label: t('sidebar.grafana.linksStatus'),
      url: `${base}/d/${db.linksStatus}/`,
    });
  }
  if (db.linkStatus) {
    let url = `${base}/d/${db.linkStatus}/`;
    // Pre-fill edge variables: source service + target dependency
    const params = new URLSearchParams();
    if (data.source) params.set('var-service', data.source);
    if (targetLabel) params.set('var-dependency', targetLabel);
    if (params.toString()) url += `?${params.toString()}`;
    dashboards.push({
      label: t('sidebar.grafana.linkStatus'),
      url,
    });
  }
  if (db.serviceStatus) {
    // Link to source service status
    let url = `${base}/d/${db.serviceStatus}/`;
    if (data.source) {
      url += `?var-service=${encodeURIComponent(data.source)}`;
    }
    dashboards.push({
      label: t('sidebar.grafana.serviceStatus'),
      url,
    });
  }

  if (dashboards.length === 0) {
    section.innerHTML = '';
    return;
  }

  section.innerHTML = `
    <div class="sidebar-section-title">${t('sidebar.grafanaDashboards')}</div>
    ${dashboards
      .map(
        (d) => `
      <a href="${d.url}" target="_blank" rel="noopener" class="sidebar-grafana-link">
        <i class="bi bi-graph-up"></i>
        <span>${d.label}</span>
        <i class="bi bi-box-arrow-up-right sidebar-grafana-external"></i>
      </a>
    `
      )
      .join('')}
  `;
}

/**
 * Render instances section (pods/containers).
 * @param {string} serviceId - Service ID
 * @param {string} serviceName - Service display name
 */
async function renderInstances(serviceId, serviceName) {
  const section = $('#sidebar-instances');

  // Show loading state
  section.innerHTML = `
    <div class="sidebar-section-title">${t('sidebar.instances')}</div>
    <div class="sidebar-instances-loading">${t('sidebar.loadingInstances')}</div>
  `;

  try {
    const instances = await fetchInstances(serviceId);

    if (!instances || instances.length === 0) {
      section.innerHTML = `
        <div class="sidebar-section-title">${t('sidebar.instances')}</div>
        <div class="sidebar-instances-empty">${t('sidebar.noInstances')}</div>
      `;
      return;
    }

    // Render instances table
    const tableHTML = `
      <div class="sidebar-section-title">${t('sidebar.instancesCount', { count: instances.length })}</div>
      <div class="sidebar-instances-table">
        <table>
          <thead>
            <tr>
              <th>${t('sidebar.instanceCol')}</th>
              <th>${t('sidebar.podCol')}</th>
            </tr>
          </thead>
          <tbody>
            ${instances.map(inst => `
              <tr>
                <td class="instance-cell" title="${inst.instance}">${inst.instance || '—'}</td>
                <td class="pod-cell" title="${inst.pod || ''}">${inst.pod || '—'}</td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    `;

    section.innerHTML = tableHTML;
  } catch (err) {
    console.error('Failed to fetch instances:', err);
    section.innerHTML = `
      <div class="sidebar-section-title">${t('sidebar.instances')}</div>
      <div class="sidebar-instances-error">${t('sidebar.failedInstances')}</div>
    `;
  }
}
