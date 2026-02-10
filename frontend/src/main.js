import 'bootstrap-icons/font/bootstrap-icons.css';
import 'tom-select/dist/css/tom-select.default.css';
import './style.css';
import { initGraph, renderGraph, updateGraphTheme } from './graph.js';
import { fetchTopology, fetchConfig, fetchUserInfo, withRetry } from './api.js';
import { showToast } from './toast.js';
import {
  initFilters, updateFilters, applyFilters, resetFilters,
  hasActiveFilters, updateNamespaceOptions, setNamespaceValue,
} from './filter.js';
import { initToolbar } from './toolbar.js';
import { initTooltip } from './tooltip.js';

let cy = null;
let pollTimer = null;
let autoRefresh = true;
let pollInterval = 15000;
let selectedNamespace = '';
let appConfig = null; // Store full config including alerts severity levels

// Connection state
let isDisconnected = false;
let retryDelay = 5000;
let retryTimer = null;
let countdownTimer = null;

// Partial data tracking
let lastPartialErrors = [];

const $ = (sel) => document.querySelector(sel);

const RETRY_BASE = 5000;
const RETRY_MAX = 30000;

function updateStatus(data) {
  const now = new Date().toLocaleTimeString();
  const { nodeCount, edgeCount } = data.meta;
  let text = `Updated ${now} | ${nodeCount} nodes, ${edgeCount} edges`;

  if (data.alerts && data.alerts.length > 0) {
    const critical = data.alerts.filter((a) => a.severity === 'critical').length;
    const warning = data.alerts.filter((a) => a.severity === 'warning').length;
    const parts = [];
    if (critical > 0) parts.push(`${critical} critical`);
    if (warning > 0) parts.push(`${warning} warning`);
    text += ` | Alerts: ${parts.join(', ') || data.alerts.length}`;
  }

  const dot = $('#status-connection');

  if (data.meta.partial) {
    text += ' | Partial data';
    dot.classList.add('partial');
    dot.classList.remove('connected', 'disconnected');

    // Toast new errors only when the error list changes
    const currentErrors = data.meta.errors || [];
    const errorsKey = currentErrors.join('|');
    const lastKey = lastPartialErrors.join('|');
    if (errorsKey !== lastKey) {
      for (const err of currentErrors) {
        showToast(`Data source error: ${err}`, 'warning');
      }
      lastPartialErrors = currentErrors;
    }
  } else {
    dot.classList.add('connected');
    dot.classList.remove('disconnected', 'partial');
    lastPartialErrors = [];
  }

  if (hasActiveFilters()) {
    text += ' | Filtered';
  }

  $('#status-info').textContent = text;
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
    const data = await fetchTopology(selectedNamespace || undefined);
    renderGraph(cy, data, appConfig);
    updateStatus(data);
    checkEmptyState(data);
    updateNamespaceOptions(data);
    updateFilters(data);
    applyFilters(cy);

    if (isDisconnected) {
      isDisconnected = false;
      retryDelay = RETRY_BASE;
      hideBanner();
      showToast('Connection restored', 'success');
      startPolling();
    }
  } catch (err) {
    console.error('Failed to refresh topology:', err);
    setConnectionError();

    if (!isDisconnected) {
      isDisconnected = true;
      retryDelay = RETRY_BASE;
      stopPolling();
      showToast(`Connection lost: ${err.message}`, 'error');
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
    const url = new URL(window.location);
    url.searchParams.delete('namespace');
    history.replaceState(null, '', url);
    applyFilters(cy);
    refresh();
  });

  window.addEventListener('filters-changed', () => {
    applyFilters(cy);
  });

  window.addEventListener('namespace-changed', (e) => {
    selectedNamespace = e.detail;
    const url = new URL(window.location);
    if (selectedNamespace) {
      url.searchParams.set('namespace', selectedNamespace);
    } else {
      url.searchParams.delete('namespace');
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

  $('#btn-toolbar-fit').addEventListener('click', () => {
    if (cy) cy.fit(50);
  });
}

function setupGrafanaClickThrough() {
  cy.on('tap', 'node[grafanaUrl]', (evt) => {
    const url = evt.target.data('grafanaUrl');
    if (url) window.open(url, '_blank');
  });

  cy.on('tap', 'edge[grafanaUrl]', (evt) => {
    const url = evt.target.data('grafanaUrl');
    if (url) window.open(url, '_blank');
  });
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

async function init() {
  initTheme();

  try {
    const config = await withRetry(fetchConfig);
    appConfig = config; // Store globally for graph rendering

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
    initToolbar();
    initTooltip(cy);
    setupGrafanaClickThrough();

    // Read namespace from URL.
    const params = new URLSearchParams(window.location.search);
    selectedNamespace = params.get('namespace') || '';

    const data = await withRetry(() => fetchTopology(selectedNamespace || undefined));
    renderGraph(cy, data, appConfig);
    updateStatus(data);
    checkEmptyState(data);
    initFilters(data);
    updateNamespaceOptions(data);
    if (selectedNamespace) {
      setNamespaceValue(selectedNamespace);
    }
    applyFilters(cy);
    startPolling();
  } catch (err) {
    console.error('Initialization failed:', err);
    showError(err.message);
  }

  $('#btn-error-retry').addEventListener('click', () => {
    hideError();
    init();
  });
}

init();
