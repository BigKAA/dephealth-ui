/**
 * Node detail sidebar functionality.
 * Shows node info, related alerts, instances, and connected edges on node click.
 */

import { fetchInstances } from './api.js';
import { t } from './i18n.js';

let topologyDataCache = null;
let currentNodeId = null; // Track currently opened node for toggle behavior

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

  // Single tap on node: toggle sidebar
  cy.on('tap', 'node', (evt) => {
    const node = evt.target;
    const nodeId = node.data('id');
    const sidebar = $('#node-sidebar');

    // If clicking the same node while sidebar is open - close it
    if (currentNodeId === nodeId && !sidebar.classList.contains('hidden')) {
      closeSidebar();
    } else {
      openSidebar(node, cy);
    }
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
export function openSidebar(node, cy) {
  const sidebar = $('#node-sidebar');
  const data = node.data();

  // Track current node for toggle behavior
  currentNodeId = data.id;

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

  // Show sidebar
  sidebar.classList.remove('hidden');
}

/**
 * Close sidebar.
 */
function closeSidebar() {
  $('#node-sidebar').classList.add('hidden');
  currentNodeId = null; // Reset tracked node
}

/**
 * Render node details section.
 * @param {object} data - Node data
 */
function renderDetails(data) {
  const section = $('#sidebar-details');
  const stateBadgeClass = `sidebar-state-badge ${data.state || 'unknown'}`;

  const details = [
    { label: t('sidebar.state'), value: `<span class="${stateBadgeClass}">${data.state || 'unknown'}</span>` },
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
      label: otherNode.data('label') || otherNode.id(),
      latency: data.latency || '—',
    };

    if (isOutgoing) {
      outgoing.push(edgeInfo);
    } else {
      incoming.push(edgeInfo);
    }
  });

  // Build HTML with separated groups
  let html = `<div class="sidebar-section-title">${t('sidebar.connectedEdges', { count: connectedEdges.length })}</div>`;

  if (outgoing.length > 0) {
    html += `
      <div class="sidebar-edge-group">
        <div class="sidebar-edge-group-title">${t('sidebar.outgoingEdges', { count: outgoing.length })}</div>
        ${outgoing
          .map(
            (info) => `
          <div class="sidebar-edge-item">
            <span class="sidebar-edge-label">→ ${info.label}</span>
            <span class="sidebar-edge-latency">${info.latency}</span>
          </div>
        `
          )
          .join('')}
      </div>
    `;
  }

  if (incoming.length > 0) {
    html += `
      <div class="sidebar-edge-group">
        <div class="sidebar-edge-group-title">${t('sidebar.incomingEdges', { count: incoming.length })}</div>
        ${incoming
          .map(
            (info) => `
          <div class="sidebar-edge-item">
            <span class="sidebar-edge-label">← ${info.label}</span>
            <span class="sidebar-edge-latency">${info.latency}</span>
          </div>
        `
          )
          .join('')}
      </div>
    `;
  }

  section.innerHTML = html;
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
