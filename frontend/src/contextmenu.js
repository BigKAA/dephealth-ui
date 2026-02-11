// Context menu module for graph nodes and edges

import { t } from './i18n.js';
import { showToast } from './toast.js';
import { openSidebar } from './sidebar.js';
import { expandNamespace } from './grouping.js';

const $ = (sel) => document.querySelector(sel);

let menuEl = null;
let cyInstance = null;

/**
 * Initialize context menu on Cytoscape instance.
 * @param {cytoscape.Core} cy
 */
export function initContextMenu(cy) {
  cyInstance = cy;
  menuEl = $('#context-menu');
  if (!menuEl) return;

  // Suppress browser context menu on the graph container
  cy.container().addEventListener('contextmenu', (e) => {
    e.preventDefault();
  });

  // Right-click on service node
  cy.on('cxttap', 'node[type="service"]', (evt) => {
    const node = evt.target;
    const data = node.data();
    const pos = evt.renderedPosition || evt.cyRenderedPosition;
    const containerRect = cy.container().getBoundingClientRect();

    const items = [];

    if (data.grafanaUrl) {
      items.push({
        label: t('contextMenu.openInGrafana'),
        icon: 'bi-graph-up',
        action: () => window.open(data.grafanaUrl, '_blank'),
      });
      items.push({
        label: t('contextMenu.copyGrafanaUrl'),
        icon: 'bi-clipboard',
        action: () => copyToClipboard(data.grafanaUrl),
      });
    }

    items.push({
      label: t('contextMenu.showDetails'),
      icon: 'bi-info-circle',
      action: () => openSidebar(node, cy),
    });

    showMenu(pos.x + containerRect.left, pos.y + containerRect.top, items);
  });

  // Right-click on collapsed namespace node
  cy.on('cxttap', 'node[?isCollapsed]', (evt) => {
    const node = evt.target;
    const nsName = node.data('nsName') || node.data('label');
    const pos = evt.renderedPosition || evt.cyRenderedPosition;
    const containerRect = cy.container().getBoundingClientRect();

    const items = [
      {
        label: t('contextMenu.expandNamespace'),
        icon: 'bi-arrows-expand',
        action: () => expandNamespace(cy, nsName),
      },
      {
        label: t('contextMenu.copyNamespaceName'),
        icon: 'bi-clipboard',
        action: () => copyToClipboard(nsName, t('contextMenu.namespaceCopied')),
      },
    ];

    showMenu(pos.x + containerRect.left, pos.y + containerRect.top, items);
  });

  // Right-click on dependency node (skip group nodes)
  cy.on('cxttap', 'node[type!="service"]', (evt) => {
    const node = evt.target;
    if (node.data('isGroup')) return;
    const pos = evt.renderedPosition || evt.cyRenderedPosition;
    const containerRect = cy.container().getBoundingClientRect();

    const items = [{
      label: t('contextMenu.showDetails'),
      icon: 'bi-info-circle',
      action: () => openSidebar(node, cy),
    }];

    showMenu(pos.x + containerRect.left, pos.y + containerRect.top, items);
  });

  // Right-click on edge
  cy.on('cxttap', 'edge', (evt) => {
    const edge = evt.target;
    const data = edge.data();
    const pos = evt.renderedPosition || evt.cyRenderedPosition;
    const containerRect = cy.container().getBoundingClientRect();

    if (!data.grafanaUrl) return; // No menu if no Grafana URL

    const items = [
      {
        label: t('contextMenu.openInGrafana'),
        icon: 'bi-graph-up',
        action: () => window.open(data.grafanaUrl, '_blank'),
      },
      {
        label: t('contextMenu.copyGrafanaUrl'),
        icon: 'bi-clipboard',
        action: () => copyToClipboard(data.grafanaUrl),
      },
    ];

    showMenu(pos.x + containerRect.left, pos.y + containerRect.top, items);
  });

  // Close menu on various events
  document.addEventListener('click', hideMenu);
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') hideMenu();
  });
  document.addEventListener('scroll', hideMenu, true);
  cy.on('pan zoom tap', hideMenu);
}

/**
 * Show context menu at position.
 * @param {number} x - Screen X
 * @param {number} y - Screen Y
 * @param {Array<{label: string, icon: string, action: Function}>} items
 */
function showMenu(x, y, items) {
  if (!menuEl || items.length === 0) return;

  // Build menu HTML
  menuEl.innerHTML = items
    .map(
      (item, i) => `
    <div class="context-menu-item" data-index="${i}">
      <i class="bi ${item.icon}"></i>
      <span>${item.label}</span>
    </div>
  `
    )
    .join('');

  // Attach click handlers
  menuEl.querySelectorAll('.context-menu-item').forEach((el) => {
    el.addEventListener('click', (e) => {
      e.stopPropagation();
      const index = parseInt(el.dataset.index, 10);
      items[index].action();
      hideMenu();
    });
  });

  // Position menu
  menuEl.classList.remove('hidden');

  // Get container offset for positioning relative to #cy
  const containerRect = cyInstance.container().getBoundingClientRect();
  let left = x - containerRect.left;
  let top = y - containerRect.top;

  // Viewport boundary adjustment
  const menuRect = menuEl.getBoundingClientRect();
  if (left + menuRect.width > containerRect.width) {
    left = left - menuRect.width;
  }
  if (top + menuRect.height > containerRect.height) {
    top = top - menuRect.height;
  }

  menuEl.style.left = `${Math.max(0, left)}px`;
  menuEl.style.top = `${Math.max(0, top)}px`;
}

/**
 * Hide context menu.
 */
function hideMenu() {
  if (menuEl) {
    menuEl.classList.add('hidden');
  }
}

/**
 * Copy text to clipboard and show toast.
 * @param {string} text
 */
async function copyToClipboard(text, toastMessage) {
  const msg = toastMessage || t('contextMenu.urlCopied');
  try {
    await navigator.clipboard.writeText(text);
    showToast(msg, 'success');
  } catch {
    // Fallback for older browsers
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    document.body.removeChild(textarea);
    showToast(msg, 'success');
  }
}
