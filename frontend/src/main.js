import './style.css';
import { initGraph, renderGraph, updateGraphTheme } from './graph.js';
import { fetchTopology, fetchConfig, fetchUserInfo, withRetry } from './api.js';
import { showToast } from './toast.js';

let cy = null;
let pollTimer = null;
let autoRefresh = true;
let pollInterval = 15000;
let selectedNamespace = '';

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
    renderGraph(cy, data);
    updateStatus(data);
    checkEmptyState(data);
    updateNamespaceOptions(data);

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
  $('#btn-theme').textContent = theme === 'dark' ? 'Light' : 'Dark';
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

function updateNamespaceOptions(data) {
  const select = $('#namespace-select');
  const namespaces = new Set();
  if (data.nodes) {
    for (const node of data.nodes) {
      if (node.namespace) {
        namespaces.add(node.namespace);
      }
    }
  }

  const sorted = [...namespaces].sort();
  const current = select.value;

  // Rebuild options only if set changed.
  const existing = [...select.options].slice(1).map((o) => o.value);
  if (sorted.length === existing.length && sorted.every((v, i) => v === existing[i])) {
    return;
  }

  // Preserve selection.
  select.innerHTML = '<option value="">All namespaces</option>';
  for (const ns of sorted) {
    const opt = document.createElement('option');
    opt.value = ns;
    opt.textContent = ns;
    select.appendChild(opt);
  }
  select.value = current;
}

function setupNamespaceSelector() {
  const select = $('#namespace-select');

  // Read namespace from URL on init.
  const params = new URLSearchParams(window.location.search);
  const ns = params.get('namespace') || '';
  if (ns) {
    selectedNamespace = ns;
    select.value = ns;
  }

  select.addEventListener('change', () => {
    selectedNamespace = select.value;

    // Sync URL.
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
    if (config.cache && config.cache.ttl > 0) {
      pollInterval = config.cache.ttl * 1000;
    }

    if (config.auth && config.auth.type === 'oidc') {
      await initUserInfo();
    }

    cy = initGraph($('#cy'));
    setupNamespaceSelector();
    setupToolbar();
    setupGrafanaClickThrough();

    const data = await withRetry(() => fetchTopology(selectedNamespace || undefined));
    renderGraph(cy, data);
    updateStatus(data);
    checkEmptyState(data);
    updateNamespaceOptions(data);
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
