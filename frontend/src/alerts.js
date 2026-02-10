// Alert Drawer module
// Displays a slide-in panel with grouped alerts and interactive navigation

let alertsData = [];
let severityLevels = [];
let cyInstance = null;

/**
 * Initialize alert drawer with Cytoscape instance
 */
export function initAlertDrawer(cy) {
  cyInstance = cy;

  const btnOpen = document.getElementById('btn-alerts');
  const btnClose = document.getElementById('btn-drawer-close');
  const drawer = document.getElementById('alert-drawer');

  if (!btnOpen || !btnClose || !drawer) {
    console.warn('Alert drawer elements not found in DOM');
    return;
  }

  // Toggle drawer (open/close)
  btnOpen.addEventListener('click', () => {
    drawer.classList.toggle('hidden');
  });

  // Close drawer
  btnClose.addEventListener('click', () => {
    drawer.classList.add('hidden');
  });

  // Close on Escape key (handled by shortcuts.js)
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && !drawer.classList.contains('hidden')) {
      drawer.classList.add('hidden');
    }
  });
}

/**
 * Update alert drawer with new alerts data
 * @param {Array} alerts - Array of alert objects
 * @param {Array} levels - Severity levels config from backend
 */
export function updateAlertDrawer(alerts, levels) {
  alertsData = alerts || [];
  severityLevels = levels || [];

  updateBadge();
  renderAlertList();
}

/**
 * Update alert count badge on header button
 */
function updateBadge() {
  const badge = document.getElementById('alert-badge');
  if (!badge) return;

  const count = alertsData.length;
  if (count > 0) {
    badge.textContent = count > 99 ? '99+' : count;
    badge.classList.remove('hidden');
  } else {
    badge.classList.add('hidden');
  }
}

/**
 * Render alert list grouped by severity
 */
function renderAlertList() {
  const container = document.getElementById('alert-list');
  if (!container) return;

  container.innerHTML = '';

  if (alertsData.length === 0) {
    container.innerHTML = '<div class="alert-empty">No active alerts</div>';
    return;
  }

  // Group alerts by severity (order from config)
  const grouped = groupAlertsBySeverity(alertsData);

  // Render each severity group
  severityLevels.forEach((level) => {
    const alerts = grouped[level.value];
    if (!alerts || alerts.length === 0) return;

    const section = document.createElement('div');
    section.className = 'alert-section';

    const header = document.createElement('div');
    header.className = 'alert-section-header';
    header.style.borderLeftColor = level.color;
    header.innerHTML = `
      <span class="alert-severity-name">${capitalize(level.value)}</span>
      <span class="alert-severity-count">${alerts.length}</span>
    `;
    section.appendChild(header);

    alerts.forEach((alert) => {
      const item = createAlertItem(alert, level.color);
      section.appendChild(item);
    });

    container.appendChild(section);
  });
}

/**
 * Group alerts by severity
 */
function groupAlertsBySeverity(alerts) {
  const groups = {};
  alerts.forEach((alert) => {
    const severity = alert.severity || 'unknown';
    if (!groups[severity]) {
      groups[severity] = [];
    }
    groups[severity].push(alert);
  });
  return groups;
}

/**
 * Create alert item DOM element
 */
function createAlertItem(alert, color) {
  const item = document.createElement('div');
  item.className = 'alert-item';
  item.style.borderLeftColor = color;

  const name = document.createElement('div');
  name.className = 'alert-name';
  name.textContent = alert.alertname || 'Unknown alert';
  item.appendChild(name);

  const meta = document.createElement('div');
  meta.className = 'alert-meta';

  // Service → Dependency
  if (alert.service && alert.dependency) {
    const link = document.createElement('span');
    link.className = 'alert-link';
    link.textContent = `${alert.service} → ${alert.dependency}`;
    meta.appendChild(link);
  } else if (alert.service) {
    const service = document.createElement('span');
    service.textContent = `Service: ${alert.service}`;
    meta.appendChild(service);
  }

  // Time since
  if (alert.startsAt) {
    const time = document.createElement('span');
    time.className = 'alert-time';
    time.textContent = formatTimeSince(alert.startsAt);
    meta.appendChild(time);
  }

  item.appendChild(meta);

  // Click to navigate to node
  if (alert.service) {
    item.style.cursor = 'pointer';
    item.addEventListener('click', () => {
      navigateToNode(alert.service);
      document.getElementById('alert-drawer').classList.add('hidden');
    });
  }

  return item;
}

/**
 * Navigate to node in graph with animation
 */
function navigateToNode(nodeId) {
  if (!cyInstance) return;

  const node = cyInstance.getElementById(nodeId);
  if (node && node.length > 0) {
    cyInstance.animate({
      center: { eles: node },
      zoom: 1.5,
    }, {
      duration: 500,
      easing: 'ease-in-out',
    });

    // Highlight node briefly
    node.flashClass('highlight', 1000);
  }
}

/**
 * Format time since timestamp
 */
function formatTimeSince(timestamp) {
  const now = new Date();
  const start = new Date(timestamp);
  const diff = Math.floor((now - start) / 1000); // seconds

  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

/**
 * Capitalize first letter
 */
function capitalize(str) {
  return str.charAt(0).toUpperCase() + str.slice(1);
}
