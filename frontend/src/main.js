import './style.css';
import { initGraph, renderGraph, updateGraphTheme } from './graph.js';
import { fetchTopology, fetchConfig, fetchUserInfo, withRetry } from './api.js';

let cy = null;
let pollTimer = null;
let autoRefresh = true;
let pollInterval = 15000;

const $ = (sel) => document.querySelector(sel);

function updateStatus(nodeCount, edgeCount, alerts) {
  const now = new Date().toLocaleTimeString();
  let text = `Updated ${now} | ${nodeCount} nodes, ${edgeCount} edges`;
  if (alerts && alerts.length > 0) {
    const critical = alerts.filter((a) => a.severity === 'critical').length;
    const warning = alerts.filter((a) => a.severity === 'warning').length;
    const parts = [];
    if (critical > 0) parts.push(`${critical} critical`);
    if (warning > 0) parts.push(`${warning} warning`);
    text += ` | Alerts: ${parts.join(', ') || alerts.length}`;
  }
  $('#status-info').textContent = text;
  $('#status-connection').classList.add('connected');
  $('#status-connection').classList.remove('disconnected');
}

function setConnectionError() {
  $('#status-connection').classList.add('disconnected');
  $('#status-connection').classList.remove('connected');
}

function showError(message) {
  $('#error-message').textContent = message;
  $('#error-overlay').classList.remove('hidden');
}

function hideError() {
  $('#error-overlay').classList.add('hidden');
}

async function refresh() {
  try {
    const data = await fetchTopology();
    renderGraph(cy, data);
    updateStatus(data.meta.nodeCount, data.meta.edgeCount, data.alerts);
    hideError();
  } catch (err) {
    console.error('Failed to refresh topology:', err);
    setConnectionError();
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
    setupToolbar();
    setupGrafanaClickThrough();

    const data = await withRetry(fetchTopology);
    renderGraph(cy, data);
    updateStatus(data.meta.nodeCount, data.meta.edgeCount, data.alerts);
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
