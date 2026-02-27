import 'bootstrap-icons/font/bootstrap-icons.css';
import 'tom-select/dist/css/tom-select.default.css';
import './style.css';
import { initGraph, renderGraph, updateGraphTheme, relayout, setLayoutDirection } from './graph.js';
import { fetchTopology, fetchConfig, fetchUserInfo, withRetry } from './api.js';
import { showToast } from './toast.js';
import {
  initFilters, updateFilters, applyFilters, resetFilters,
  hasActiveFilters, updateNamespaceOptions, setNamespaceValue,
} from './filter.js';
import { initToolbar } from './toolbar.js';
import { initTooltip } from './tooltip.js';
import { initSidebar, updateSidebarData, setGrafanaConfig } from './sidebar.js';
import { initSearch } from './search.js';
import { initAlertDrawer, updateAlertDrawer, setAlertManagerAvailable } from './alerts.js';
import { initShortcuts } from './shortcuts.js';
import { initI18n, t, setLanguage, getLanguage, updateI18nDom } from './i18n.js';
import { getNamespaceColor, extractNamespaceFromHost } from './namespace.js';
import { initContextMenu, setContextMenuGrafanaConfig } from './contextmenu.js';
import { makeDraggable, clampElement } from './draggable.js';
import { computeCascadeWarnings } from './cascade.js';
import { initExportModal, openExportModal } from './export.js';
import {
  initTimeline, isHistoryMode, getSelectedTime,
  enterHistoryMode, exitHistoryMode, restoreFromURL,
} from './timeline.js';
import {
  isGroupingEnabled, setGroupingEnabled,
  getGroupingDimension, setGroupingDimension,
  collapseNamespace, expandNamespace, collapseAll, expandAll,
  hasExpandedGroups, reapplyCollapsedState, getCollapsedNamespaces,
  getNamespacePrefix,
} from './grouping.js';

let cy = null;
let pollTimer = null;
let autoRefresh = true;
let pollInterval = 15000;
let selectedNamespace = '';
let selectedGroup = '';
let appConfig = null; // Store full config including alerts severity levels
let layoutDirection = 'TB'; // Current layout direction: 'TB' or 'LR'
let dataHasGroups = false; // Whether current data has any nodes with group field
let edgeLabelsEnabled = localStorage.getItem('dephealth-edge-labels') === 'true';

// Connection state
let isDisconnected = false;
let retryDelay = 5000;
let retryTimer = null;
let countdownTimer = null;

// Partial data tracking
let lastPartialErrors = [];

/**
 * Returns whether edge type labels are enabled.
 * @returns {boolean}
 */
export function isEdgeLabelsEnabled() {
  return edgeLabelsEnabled;
}

const $ = (sel) => document.querySelector(sel);

const RETRY_BASE = 5000;
const RETRY_MAX = 30000;

/**
 * Escape a string for safe insertion into HTML.
 * @param {*} str
 * @returns {string}
 */
function escapeHtml(str) {
  if (typeof str !== 'string') return String(str);
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

function updateStatus(data) {
  const { nodeCount, edgeCount } = data.meta;
  let text;
  if (data.meta.isHistory && data.meta.time) {
    const histDate = new Date(data.meta.time).toLocaleString();
    text = t('status.viewing', { time: histDate, nodes: nodeCount, edges: edgeCount });
  } else {
    const now = new Date().toLocaleTimeString();
    text = t('status.updated', { time: now, nodes: nodeCount, edges: edgeCount });
  }

  const alertsEnabled = appConfig && appConfig.alerts && appConfig.alerts.enabled;
  if (alertsEnabled && data.alerts && data.alerts.length > 0) {
    const critical = data.alerts.filter((a) => a.severity === 'critical').length;
    const warning = data.alerts.filter((a) => a.severity === 'warning').length;
    const parts = [];
    if (critical > 0) parts.push(t('status.critical', { count: critical }));
    if (warning > 0) parts.push(t('status.warning', { count: warning }));
    text += ' | ' + t('status.alerts', { details: parts.join(', ') || data.alerts.length });
  }

  const dot = $('#status-connection');

  if (data.meta.partial) {
    text += ' | ' + t('status.partialData');
    dot.classList.add('partial');
    dot.classList.remove('connected', 'disconnected');

    // Toast new errors only when the error list changes
    const currentErrors = data.meta.errors || [];
    const errorsKey = currentErrors.join('|');
    const lastKey = lastPartialErrors.join('|');
    if (errorsKey !== lastKey) {
      for (const err of currentErrors) {
        showToast(t('toast.dataSourceError', { error: err }), 'warning');
      }
      lastPartialErrors = currentErrors;
    }
  } else {
    dot.classList.add('connected');
    dot.classList.remove('disconnected', 'partial');
    lastPartialErrors = [];
  }

  if (hasActiveFilters()) {
    text += ' | ' + t('status.filtered');
  }

  $('#status-info').textContent = text;

  // Update health stats
  updateHealthStats(data);

  // Update alert drawer (skip when AlertManager is not configured)
  if (alertsEnabled && appConfig.alerts.severityLevels) {
    updateAlertDrawer(data.alerts || [], appConfig.alerts.severityLevels);
  }
}

function updateHealthStats(data) {
  const statsContainer = $('#status-stats');
  if (!statsContainer) return;

  // Count nodes by state
  const counts = { ok: 0, degraded: 0, down: 0, unknown: 0 };
  if (data.nodes) {
    data.nodes.forEach((n) => {
      const state = n.state || 'unknown';
      counts[state] = (counts[state] || 0) + 1;
    });
  }

  // Build stats HTML
  const parts = [];
  if (counts.ok > 0) {
    parts.push(`<span class="stat-ok">${counts.ok} ${t('state.ok')}</span>`);
  }
  if (counts.degraded > 0) {
    parts.push(`<span class="stat-degraded">${counts.degraded} ${t('state.degraded')}</span>`);
  }
  if (counts.down > 0) {
    parts.push(`<span class="stat-down">${counts.down} ${t('state.down')}</span>`);
  }
  if (counts.unknown > 0) {
    parts.push(`<span class="stat-unknown">${counts.unknown} ${t('state.unknown')}</span>`);
  }

  statsContainer.innerHTML = parts.length > 0 ? ' | ' + parts.join(' | ') : '';
}

function setConnectionError() {
  $('#status-connection').classList.add('disconnected');
  $('#status-connection').classList.remove('connected', 'partial');
}

function showError(message) {
  $('#error-message').textContent = message;
  $('#error-overlay').classList.remove('hidden');
}

function hideError() {
  $('#error-overlay').classList.add('hidden');
}

function showBanner() {
  $('#connection-banner').classList.remove('hidden');
}

function hideBanner() {
  $('#connection-banner').classList.add('hidden');
  clearRetryTimers();
}

function clearRetryTimers() {
  if (retryTimer) {
    clearTimeout(retryTimer);
    retryTimer = null;
  }
  if (countdownTimer) {
    clearInterval(countdownTimer);
    countdownTimer = null;
  }
}

function startRetryCountdown(delay) {
  let remaining = Math.ceil(delay / 1000);
  $('#banner-countdown').textContent = remaining;
  showBanner();

  countdownTimer = setInterval(() => {
    remaining--;
    if (remaining > 0) {
      $('#banner-countdown').textContent = remaining;
    }
  }, 1000);

  retryTimer = setTimeout(() => {
    clearRetryTimers();
    refresh();
  }, delay);
}

function checkEmptyState(data) {
  const empty = $('#empty-state');
  if (!data.nodes || data.nodes.length === 0) {
    empty.classList.remove('hidden');
  } else {
    empty.classList.add('hidden');
  }
}

async function refresh() {
  try {
    const histTime = getSelectedTime();
    const dim = getGroupingDimension();
    const nsParam = dim === 'namespace' ? (selectedNamespace || undefined) : undefined;
    const grpParam = dim === 'group' ? (selectedGroup || undefined) : undefined;
    const data = await fetchTopology(
      nsParam,
      histTime ? histTime.toISOString() : undefined,
      grpParam,
    );
    const structureChanged = renderGraph(cy, data, appConfig);
    if (structureChanged && isGroupingEnabled() && getCollapsedNamespaces().size > 0) {
      reapplyCollapsedState(cy);
    }
    computeCascadeWarnings(cy);
    updateStatus(data);
    checkEmptyState(data);
    updateDimensionToggleVisibility(data);
    updateNamespaceOptions(data);
    updateDimensionFilter(data);
    updateNamespaceLegend(data);
    updateFilters(data);
    applyFilters(cy);
    updateSidebarData(data);

    if (isDisconnected) {
      isDisconnected = false;
      retryDelay = RETRY_BASE;
      hideBanner();
      showToast(t('toast.connectionRestored'), 'success');
      startPolling();
    }
  } catch (err) {
    console.error('Failed to refresh topology:', err);
    setConnectionError();

    if (!isDisconnected) {
      isDisconnected = true;
      retryDelay = RETRY_BASE;
      stopPolling();
      showToast(t('toast.connectionLost', { error: err.message }), 'error');
    }

    startRetryCountdown(retryDelay);
    retryDelay = Math.min(retryDelay * 2, RETRY_MAX);
  }
}

function startPolling() {
  stopPolling();
  if (autoRefresh) {
    pollTimer = setInterval(refresh, pollInterval);
  }
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  const icon = $('#btn-theme i');
  if (icon) {
    icon.className = theme === 'dark' ? 'bi bi-sun' : 'bi bi-moon';
  }
  updateGraphTheme(cy);
}

function initTheme() {
  const stored = localStorage.getItem('theme');
  if (stored) {
    applyTheme(stored);
  } else if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
    applyTheme('dark');
  } else {
    applyTheme('light');
  }

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    if (!localStorage.getItem('theme')) {
      applyTheme(e.matches ? 'dark' : 'light');
    }
  });
}

function setupFilters() {
  const panel = $('#filter-panel');
  const btn = $('#btn-filter');

  // Restore panel visibility.
  const panelVisible = localStorage.getItem('dephealth-filter-panel') !== 'closed';
  if (panelVisible) {
    panel.classList.remove('hidden');
    btn.classList.add('active');
  }

  btn.addEventListener('click', () => {
    const isHidden = panel.classList.toggle('hidden');
    btn.classList.toggle('active', !isHidden);
    localStorage.setItem('dephealth-filter-panel', isHidden ? 'closed' : 'open');
  });

  $('#btn-reset-filters').addEventListener('click', () => {
    resetFilters();
    selectedNamespace = '';
    selectedGroup = '';
    const url = new URL(window.location);
    url.searchParams.delete('namespace');
    url.searchParams.delete('group');
    history.replaceState(null, '', url);
    applyFilters(cy);
    refresh();
  });

  window.addEventListener('filters-changed', () => {
    applyFilters(cy);
  });

  window.addEventListener('namespace-changed', (e) => {
    const dim = getGroupingDimension();
    const url = new URL(window.location);
    if (dim === 'group') {
      selectedGroup = e.detail;
      selectedNamespace = '';
      url.searchParams.delete('namespace');
      if (selectedGroup) {
        url.searchParams.set('group', selectedGroup);
      } else {
        url.searchParams.delete('group');
      }
    } else {
      selectedNamespace = e.detail;
      selectedGroup = '';
      url.searchParams.delete('group');
      if (selectedNamespace) {
        url.searchParams.set('namespace', selectedNamespace);
      } else {
        url.searchParams.delete('namespace');
      }
    }
    history.replaceState(null, '', url);
    refresh();
  });
}

function setupToolbar() {
  $('#btn-refresh').addEventListener('click', () => {
    refresh();
  });

  $('#btn-fit').addEventListener('click', () => {
    if (cy) cy.fit(50);
  });

  $('#btn-auto-refresh').addEventListener('click', () => {
    autoRefresh = !autoRefresh;
    $('#btn-auto-refresh').classList.toggle('active', autoRefresh);
    const icon = $('#btn-auto-refresh i');
    if (icon) {
      icon.className = autoRefresh ? 'bi bi-play-circle' : 'bi bi-pause-circle';
    }
    if (autoRefresh) {
      startPolling();
    } else {
      stopPolling();
    }
  });

  $('#btn-theme').addEventListener('click', () => {
    const current = document.documentElement.dataset.theme;
    const next = current === 'dark' ? 'light' : 'dark';
    localStorage.setItem('theme', next);
    applyTheme(next);
  });

  $('#btn-retry-now').addEventListener('click', () => {
    clearRetryTimers();
    refresh();
  });
}

function setupGraphToolbar() {
  $('#btn-zoom-in').addEventListener('click', () => {
    if (cy) cy.zoom({ level: cy.zoom() * 1.2, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
  });

  $('#btn-zoom-out').addEventListener('click', () => {
    if (cy) cy.zoom({ level: cy.zoom() / 1.2, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
  });

  // Layout toggle button
  const btnLayoutToggle = $('#btn-layout-toggle');
  btnLayoutToggle.addEventListener('click', () => {
    layoutDirection = layoutDirection === 'TB' ? 'LR' : 'TB';
    const icon = btnLayoutToggle.querySelector('i');
    if (icon) {
      icon.className = layoutDirection === 'TB' ? 'bi bi-distribute-vertical' : 'bi bi-distribute-horizontal';
    }
    localStorage.setItem('dephealth-layout-direction', layoutDirection);
    relayout(cy, layoutDirection);
  });

  // Namespace grouping toggle button
  const btnGrouping = $('#btn-grouping');
  const btnCollapseAll = $('#btn-collapse-all');
  const btnDimToggle = $('#btn-dimension-toggle');
  if (isGroupingEnabled()) {
    btnGrouping.classList.add('active');
    btnLayoutToggle.classList.add('hidden');
    btnCollapseAll.classList.remove('hidden');
    // Dimension toggle visibility depends on data — updated after first fetch
  }
  btnGrouping.addEventListener('click', () => {
    const next = !isGroupingEnabled();
    setGroupingEnabled(next);
    btnGrouping.classList.toggle('active', next);
    btnLayoutToggle.classList.toggle('hidden', next);
    btnCollapseAll.classList.toggle('hidden', !next);
    btnDimToggle.classList.toggle('hidden', !next || !dataHasGroups);
    refresh();
  });

  // Dimension toggle button (NS ↔ GRP)
  updateDimensionLabel();
  btnDimToggle.addEventListener('click', () => {
    const current = getGroupingDimension();
    const next = current === 'namespace' ? 'group' : 'namespace';
    setGroupingDimension(next);
    updateDimensionLabel();
    // Clear dimension filter when switching
    selectedNamespace = '';
    selectedGroup = '';
    const url = new URL(window.location);
    url.searchParams.delete('namespace');
    url.searchParams.delete('group');
    history.replaceState(null, '', url);
    updateDimensionFilter(null);
    refresh();
  });

  // Collapse All / Expand All button
  btnCollapseAll.addEventListener('click', () => {
    if (!cy || !isGroupingEnabled()) return;
    if (hasExpandedGroups(cy)) {
      collapseAll(cy);
      const icon = btnCollapseAll.querySelector('i');
      if (icon) icon.className = 'bi bi-arrows-expand';
    } else {
      expandAll(cy);
      const icon = btnCollapseAll.querySelector('i');
      if (icon) icon.className = 'bi bi-arrows-collapse';
    }
  });

  // Export button — opens export modal
  $('#btn-export').addEventListener('click', () => {
    if (!cy) return;
    openExportModal();
  });

  // Fullscreen button
  $('#btn-fullscreen').addEventListener('click', () => {
    if (document.fullscreenElement) {
      document.exitFullscreen();
    } else {
      document.documentElement.requestFullscreen();
    }
  });

  // Edge labels toggle button
  const btnEdgeLabels = $('#btn-edge-labels');
  if (edgeLabelsEnabled) btnEdgeLabels.classList.add('active');
  btnEdgeLabels.addEventListener('click', () => {
    edgeLabelsEnabled = !edgeLabelsEnabled;
    btnEdgeLabels.classList.toggle('active', edgeLabelsEnabled);
    localStorage.setItem('dephealth-edge-labels', edgeLabelsEnabled);
    if (cy) cy.style().update();
  });

  // Update fullscreen icon
  document.addEventListener('fullscreenchange', () => {
    const isFullscreen = !!document.fullscreenElement;
    document.body.classList.toggle('fullscreen', isFullscreen);
    const icon = $('#btn-fullscreen i');
    if (icon) {
      icon.className = isFullscreen ? 'bi bi-fullscreen-exit' : 'bi bi-fullscreen';
    }
  });
}

/**
 * Update the dimension toggle button label to reflect current dimension.
 */
function updateDimensionLabel() {
  const label = $('#dimension-label');
  if (!label) return;
  const dim = getGroupingDimension();
  label.textContent = dim === 'group' ? t('graphToolbar.dimGroup') : t('graphToolbar.dimNamespace');
}

/**
 * Update dimension toggle visibility based on whether data has group values.
 * @param {{nodes: Array}} data - Topology data
 */
function updateDimensionToggleVisibility(data) {
  dataHasGroups = data.nodes ? data.nodes.some((n) => !!n.group) : false;
  const btn = $('#btn-dimension-toggle');
  if (!btn) return;
  btn.classList.toggle('hidden', !isGroupingEnabled() || !dataHasGroups);

  // Auto-select namespace if no group data available
  if (!dataHasGroups && getGroupingDimension() === 'group') {
    setGroupingDimension('namespace');
    updateDimensionLabel();
  }
}

/**
 * Notify the filter dropdown to update for current dimension.
 * @param {{nodes: Array}|null} data - Topology data (null to just reset value)
 */
function updateDimensionFilter(data) {
  window.dispatchEvent(new CustomEvent('dimension-changed', {
    detail: { dimension: getGroupingDimension(), data },
  }));
}

// Legend panels configuration: id → { panelSel, storageKey, defaultVisible }
const LEGEND_PANELS = {
  'graph-legend': { storageKey: 'dephealth-legend', defaultVisible: true, posKey: 'dephealth-legend-pos' },
  'namespace-legend': { storageKey: 'dephealth-ns-legend', defaultVisible: true, posKey: 'dephealth-ns-legend-pos' },
  'connection-legend': { storageKey: 'dephealth-conn-legend', defaultVisible: false, posKey: 'dephealth-conn-legend-pos' },
};

function setupLegends() {
  const dropdown = $('#legends-dropdown');
  const btnLegends = $('#btn-legends');

  // Setup each legend panel: restore visibility, close button, draggable
  for (const [panelId, cfg] of Object.entries(LEGEND_PANELS)) {
    const panel = $(`#${panelId}`);
    const closeBtn = panel.querySelector('.legend-close');

    // Restore visibility
    const stored = localStorage.getItem(cfg.storageKey);
    const visible = stored !== null ? stored !== 'hidden' : cfg.defaultVisible;
    if (visible) {
      panel.classList.remove('hidden');
    }

    // Close button
    if (closeBtn) {
      closeBtn.addEventListener('click', () => {
        panel.classList.add('hidden');
        localStorage.setItem(cfg.storageKey, 'hidden');
        updateDropdownChecks();
      });
    }

    makeDraggable(panel, cfg.posKey, { dragHandle: '.legend-header', exclude: 'button' });
  }

  // Toggle dropdown on button click
  btnLegends.addEventListener('click', (e) => {
    e.stopPropagation();
    const isHidden = dropdown.classList.toggle('hidden');
    if (!isHidden) updateDropdownChecks();
  });

  // Handle dropdown item clicks
  dropdown.addEventListener('click', (e) => {
    const item = e.target.closest('.toolbar-dropdown-item');
    if (!item) return;
    e.stopPropagation();

    const targetId = item.dataset.target;
    const cfg = LEGEND_PANELS[targetId];
    if (!cfg) return;

    const panel = $(`#${targetId}`);
    const isHidden = panel.classList.toggle('hidden');
    localStorage.setItem(cfg.storageKey, isHidden ? 'hidden' : 'visible');
    if (!isHidden) clampElement(panel);
    updateDropdownChecks();
  });

  // Close dropdown on click outside
  document.addEventListener('click', (e) => {
    if (!dropdown.classList.contains('hidden') && !dropdown.contains(e.target) && e.target !== btnLegends) {
      dropdown.classList.add('hidden');
    }
  });
}

function updateDropdownChecks() {
  const dropdown = $('#legends-dropdown');
  if (!dropdown) return;
  for (const item of dropdown.querySelectorAll('.toolbar-dropdown-item')) {
    const targetId = item.dataset.target;
    const panel = $(`#${targetId}`);
    const check = item.querySelector('.dropdown-check');
    if (panel && check) {
      check.classList.toggle('hidden', panel.classList.contains('hidden'));
    }
  }
}

function updateNamespaceLegend(data) {
  const container = $('#ns-legend-items');
  if (!container) return;

  const dim = getGroupingDimension();
  const values = new Set();
  if (data.nodes) {
    for (const node of data.nodes) {
      let val;
      if (dim === 'group') {
        val = node.group || null;
      } else {
        val = node.namespace || (node.type !== 'service' ? extractNamespaceFromHost(node.host) : null);
      }
      if (val) values.add(val);
    }
  }

  // Update legend title
  const titleEl = $('#namespace-legend .legend-title');
  if (titleEl) {
    titleEl.textContent = dim === 'group' ? t('namespaceLegend.groupTitle') : t('namespaceLegend.title');
  }

  const sorted = [...values].sort();
  container.innerHTML = sorted
    .map(
      (v) => `
    <div class="ns-legend-item">
      <span class="ns-legend-swatch" style="background: ${getNamespaceColor(v)};"></span>
      <span class="ns-legend-name" title="${escapeHtml(v)}">${escapeHtml(v)}</span>
    </div>
  `
    )
    .join('');
}

async function initUserInfo() {
  const user = await fetchUserInfo();
  if (user && (user.name || user.email)) {
    $('#user-name').textContent = user.name || user.email;
    $('#user-info').classList.remove('hidden');

    $('#btn-logout').addEventListener('click', () => {
      window.location.href = '/auth/logout';
    });
  }
}

function setupLanguage() {
  initI18n();

  // Update lang label to current language
  const langLabel = $('#lang-label');
  if (langLabel) {
    langLabel.textContent = getLanguage().toUpperCase();
  }

  // Language toggle button
  $('#btn-lang').addEventListener('click', () => {
    const next = getLanguage() === 'en' ? 'ru' : 'en';
    setLanguage(next);
    if (langLabel) {
      langLabel.textContent = next.toUpperCase();
    }
  });

  // Update DOM on language change
  window.addEventListener('language-changed', () => {
    updateI18nDom();
  });
}

function setupGroupingHandlers() {
  if (!cy) return;

  const NS_PREFIX = getNamespacePrefix();

  // Double-tap on expanded namespace group → collapse
  cy.on('dbltap', 'node[?isGroup]', (evt) => {
    if (!isGroupingEnabled()) return;
    const node = evt.target;
    if (node.data('isCollapsed')) {
      // Expand collapsed node
      const nsName = node.id().replace(NS_PREFIX, '');
      expandNamespace(cy, nsName);
    } else {
      // Collapse expanded group
      const nsName = node.data('label');
      collapseNamespace(cy, nsName);
    }
  });
}

async function init() {
  setupLanguage();
  initTheme();

  // Restore layout direction from localStorage
  const savedDirection = localStorage.getItem('dephealth-layout-direction');
  if (savedDirection === 'LR' || savedDirection === 'TB') {
    layoutDirection = savedDirection;
    setLayoutDirection(layoutDirection); // Set in graph.js
  }

  try {
    const config = await withRetry(fetchConfig);
    appConfig = config; // Store globally for graph rendering
    setGrafanaConfig(config);
    setContextMenuGrafanaConfig(config);

    // Set AlertManager availability for UI elements
    const amEnabled = !!(config.alerts && config.alerts.enabled);
    setAlertManagerAvailable(amEnabled);

    if (config.cache && config.cache.ttl > 0) {
      pollInterval = config.cache.ttl * 1000;
    }

    if (config.auth && config.auth.type === 'oidc') {
      await initUserInfo();
    }

    cy = initGraph($('#cy'), appConfig);
    setupToolbar();
    setupFilters();
    setupGraphToolbar();
    setupLegends();
    initExportModal(cy, () => ({ namespace: selectedNamespace, group: selectedGroup }));

    // Prevent clicks on floating panels inside #cy from reaching the Cytoscape canvas
    for (const sel of ['#graph-legend', '#namespace-legend', '#connection-legend', '#context-menu', '#graph-toolbar']) {
      const el = $(sel);
      if (el) el.addEventListener('pointerdown', (e) => e.stopPropagation());
    }

    initToolbar();
    initTimeline((time) => {
      // time is Date or null (back to live)
      if (time) {
        stopPolling();
        refresh();
      } else {
        refresh();
        startPolling();
      }
    });

    // History mode toggle button
    $('#btn-history').addEventListener('click', () => {
      if (isHistoryMode()) {
        exitHistoryMode();
        refresh();
        startPolling();
      } else {
        stopPolling();
        enterHistoryMode();
      }
    });

    initTooltip(cy);
    initSearch(cy);
    initAlertDrawer(cy);
    initShortcuts({
      refresh: () => refresh(),
      fit: () => cy && cy.fit(50),
      zoomIn: () => cy && cy.zoom({ level: cy.zoom() * 1.2, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } }),
      zoomOut: () => cy && cy.zoom({ level: cy.zoom() / 1.2, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } }),
      openSearch: () => {
        const searchPanel = $('#search-panel');
        if (searchPanel && searchPanel.classList.contains('hidden')) {
          $('#btn-search').click();
        }
      },
      toggleLayout: () => $('#btn-layout-toggle').click(),
      openExport: () => $('#btn-export').click(),
      toggleEdgeLabels: () => $('#btn-edge-labels').click(),
      closeAll: () => {
        // Close all panels
        const searchPanel = $('#search-panel');
        const sidebar = $('#node-sidebar');
        const drawer = $('#alert-drawer');
        if (searchPanel && !searchPanel.classList.contains('hidden')) {
          $('#btn-search-close').click();
        }
        if (sidebar && !sidebar.classList.contains('hidden')) {
          sidebar.classList.add('hidden');
        }
        if (drawer && !drawer.classList.contains('hidden')) {
          drawer.classList.add('hidden');
        }
      },
    });

    // Update layout toggle icon based on saved direction
    const btnLayoutToggle = $('#btn-layout-toggle');
    const icon = btnLayoutToggle.querySelector('i');
    if (icon) {
      icon.className = layoutDirection === 'TB' ? 'bi bi-distribute-vertical' : 'bi bi-distribute-horizontal';
    }

    // Read namespace/group from URL.
    const params = new URLSearchParams(window.location.search);
    selectedNamespace = params.get('namespace') || '';
    selectedGroup = params.get('group') || '';
    // If URL has group param, auto-switch dimension to group
    if (selectedGroup && getGroupingDimension() !== 'group') {
      setGroupingDimension('group');
      updateDimensionLabel();
    }

    // Restore history mode from URL (?time=...) if present
    const restoredHistory = restoreFromURL();
    if (restoredHistory) {
      stopPolling();
    }

    const histTime = getSelectedTime();
    const initDim = getGroupingDimension();
    const initNs = initDim === 'namespace' ? (selectedNamespace || undefined) : undefined;
    const initGrp = initDim === 'group' ? (selectedGroup || undefined) : undefined;
    const data = await withRetry(() => fetchTopology(
      initNs,
      histTime ? histTime.toISOString() : undefined,
      initGrp,
    ));
    const structureChanged = renderGraph(cy, data, appConfig);
    if (structureChanged && isGroupingEnabled() && getCollapsedNamespaces().size > 0) {
      reapplyCollapsedState(cy);
    }
    computeCascadeWarnings(cy);
    updateStatus(data);
    checkEmptyState(data);
    updateDimensionToggleVisibility(data);
    initFilters(data);
    updateNamespaceOptions(data);
    updateDimensionFilter(data);
    updateNamespaceLegend(data);
    if (selectedNamespace) {
      setNamespaceValue(selectedNamespace);
    }
    if (selectedGroup) {
      setNamespaceValue(selectedGroup);
    }
    applyFilters(cy);
    initSidebar(cy, data);
    setupGroupingHandlers();
    initContextMenu(cy);
    updateSidebarData(data);
    if (!isHistoryMode()) {
      startPolling();
    }
  } catch (err) {
    console.error('Initialization failed:', err);
    showError(err.message);
  }

}

// Register retry handler once (outside init to prevent listener stacking on retries).
$('#btn-error-retry').addEventListener('click', () => {
  hideError();
  init();
});

init();
