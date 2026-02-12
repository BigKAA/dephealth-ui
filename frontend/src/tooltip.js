import { t } from './i18n.js';

/**
 * Initialize tooltip functionality for the graph.
 * Shows tooltips on hover for nodes and edges.
 * @param {cytoscape.Core} cy - Cytoscape instance
 */
export function initTooltip(cy) {
  const tooltip = document.getElementById('graph-tooltip');
  if (!tooltip) {
    console.warn('Tooltip element not found');
    return;
  }

  let hideTimeout = null;

  /**
   * Show tooltip with content at specified position.
   * @param {string} html - HTML content for tooltip
   * @param {number} x - X position
   * @param {number} y - Y position
   */
  function showTooltip(html, x, y) {
    if (hideTimeout) {
      clearTimeout(hideTimeout);
      hideTimeout = null;
    }

    tooltip.innerHTML = html;
    tooltip.classList.remove('hidden');

    // Position tooltip near cursor, adjust for viewport bounds
    const rect = tooltip.getBoundingClientRect();
    const containerRect = cy.container().getBoundingClientRect();

    let left = x + 12;
    let top = y + 12;

    // Adjust if tooltip would overflow right edge
    if (left + rect.width > containerRect.right) {
      left = x - rect.width - 12;
    }

    // Adjust if tooltip would overflow bottom edge
    if (top + rect.height > containerRect.bottom) {
      top = y - rect.height - 12;
    }

    tooltip.style.left = `${left - containerRect.left}px`;
    tooltip.style.top = `${top - containerRect.top}px`;
  }

  /**
   * Hide tooltip with a small delay.
   */
  function hideTooltip() {
    hideTimeout = setTimeout(() => {
      tooltip.classList.add('hidden');
    }, 100);
  }

  /**
   * Format state value with capitalized first letter.
   * @param {string} state
   * @returns {string}
   */
  function formatState(state) {
    if (!state) return 'unknown';
    return state.charAt(0).toUpperCase() + state.slice(1);
  }

  // Node hover
  cy.on('mouseover', 'node', (evt) => {
    const node = evt.target;
    const data = node.data();
    const renderedPos = evt.renderedPosition || evt.cyRenderedPosition;

    let html = `<div class="tooltip-title">${data.label || data.id}</div>`;

    // State
    html += `<div class="tooltip-row">
      <span class="tooltip-label">${t('tooltip.state')}</span>
      <span class="tooltip-value">${formatState(data.state)}${data.stale ? ` (${t('state.unknown.detail')})` : ''}</span>
    </div>`;

    // Type
    if (data.type) {
      html += `<div class="tooltip-row">
        <span class="tooltip-label">${t('tooltip.type')}</span>
        <span class="tooltip-value">${data.type}</span>
      </div>`;
    }

    // Namespace (if available in node data)
    if (data.namespace) {
      html += `<div class="tooltip-row">
        <span class="tooltip-label">${t('tooltip.namespace')}</span>
        <span class="tooltip-value">${data.namespace}</span>
      </div>`;
    }

    // Alert count
    if (data.alertCount && data.alertCount > 0) {
      html += `<div class="tooltip-row">
        <span class="tooltip-label">${t('tooltip.alerts')}</span>
        <span class="tooltip-value">${data.alertCount}</span>
      </div>`;
    }

    // Cascade warning sources
    const cascadeSources = data.cascadeSources;
    if (cascadeSources && cascadeSources.length > 0 && data.state !== 'down') {
      html += `<div class="tooltip-row">
        <span class="tooltip-label">${t('tooltip.cascadeWarning')}</span>
      </div>`;
      for (const src of cascadeSources) {
        const srcNode = cy.getElementById(src);
        const srcLabel = srcNode.length > 0 ? (srcNode.data('label') || srcNode.data('name') || src) : src;
        const srcState = srcNode.length > 0 ? formatState(srcNode.data('state')) : '';
        const display = srcState ? `${srcLabel} (${srcState})` : srcLabel;
        html += `<div class="tooltip-row">
          <span class="tooltip-value">${t('tooltip.cascadeSource', { service: display })}</span>
        </div>`;
      }
    }

    showTooltip(html, renderedPos.x, renderedPos.y);
  });

  // Edge hover
  cy.on('mouseover', 'edge', (evt) => {
    const edge = evt.target;
    const data = edge.data();
    const renderedPos = evt.renderedPosition || evt.cyRenderedPosition;

    const sourceLabel = edge.source().data('label') || edge.source().id();
    const targetLabel = edge.target().data('label') || edge.target().id();

    let html = `<div class="tooltip-title">${sourceLabel} → ${targetLabel}</div>`;

    // Latency (hide for stale edges)
    if (data.latency && !data.stale) {
      html += `<div class="tooltip-row">
        <span class="tooltip-label">${t('tooltip.latency')}</span>
        <span class="tooltip-value">${data.latency}</span>
      </div>`;
    }

    // State
    html += `<div class="tooltip-row">
      <span class="tooltip-label">${t('tooltip.state')}</span>
      <span class="tooltip-value">${formatState(data.state)}${data.stale ? ` (${t('state.unknown.detail')})` : ''}</span>
    </div>`;

    // Critical flag
    if (data.critical) {
      html += `<div class="tooltip-row">
        <span class="tooltip-label">${t('tooltip.critical')}</span>
        <span class="tooltip-value">${t('tooltip.yes')}</span>
      </div>`;
    }

    // Alert count
    if (data.alertCount && data.alertCount > 0) {
      html += `<div class="tooltip-row">
        <span class="tooltip-label">${t('tooltip.alerts')}</span>
        <span class="tooltip-value">${data.alertCount}</span>
      </div>`;
    }

    showTooltip(html, renderedPos.x, renderedPos.y);
  });

  // Hide tooltip on mouseout
  cy.on('mouseout', 'node,edge', () => {
    hideTooltip();
  });

  // Also hide tooltip when panning/zooming
  cy.on('pan zoom', () => {
    tooltip.classList.add('hidden');
  });
}
