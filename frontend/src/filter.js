import TomSelect from 'tom-select';
import { t } from './i18n.js';
import { isGroupingEnabled, getCollapsedChildren } from './grouping.js';

const STORAGE_KEY = 'dephealth-filters';
const STATES = ['ok', 'degraded', 'down', 'warning'];
const STATUS_VALUES = ['ok', 'timeout', 'connection_error', 'dns_error', 'auth_error', 'tls_error', 'unhealthy', 'error'];

const $ = (sel) => document.querySelector(sel);

// Active filter selections per dimension (empty Set = all visible).
let activeFilters = {
  type: new Set(),
  state: new Set(),
  status: new Set(),
  job: new Set(),
};

// Known values per dimension (updated from data).
let knownValues = {
  type: [],
  state: STATES,
  job: [],
};

// Tom Select instances.
let tsType = null;
let tsJob = null;
let tsStatus = null;
let tsNamespace = null;

// --- Tom Select initialization ---

function initNamespaceSelect() {
  const el = $('#namespace-select');
  tsNamespace = new TomSelect(el, {
    create: false,
    sortField: { field: 'text', direction: 'asc' },
    placeholder: t('filter.allNamespaces'),
    allowEmptyOption: true,
    onChange(value) {
      window.dispatchEvent(new CustomEvent('namespace-changed', { detail: value }));
    },
  });
}

function initTypeSelect() {
  const el = $('#type-select');
  tsType = new TomSelect(el, {
    create: false,
    plugins: ['remove_button'],
    placeholder: t('filter.allTypes'),
    onChange(values) {
      activeFilters.type = new Set(values);
      saveToStorage();
      window.dispatchEvent(new CustomEvent('filters-changed'));
    },
  });
}

function initStatusSelect() {
  const el = $('#status-select');
  tsStatus = new TomSelect(el, {
    create: false,
    plugins: ['remove_button'],
    placeholder: t('filter.allStatuses'),
    onChange(values) {
      activeFilters.status = new Set(values);
      saveToStorage();
      window.dispatchEvent(new CustomEvent('filters-changed'));
    },
  });
  // Populate with all known status values
  for (const val of STATUS_VALUES) {
    tsStatus.addOption({ value: val, text: val });
  }
}

function initJobSelect() {
  const el = $('#job-select');
  tsJob = new TomSelect(el, {
    create: false,
    plugins: ['remove_button'],
    placeholder: t('filter.allServices'),
    onChange(values) {
      activeFilters.job = new Set(values);
      saveToStorage();
      window.dispatchEvent(new CustomEvent('filters-changed'));
    },
  });
}

/**
 * Update Tom Select placeholders on language change.
 */
function updateTomSelectPlaceholders() {
  const updates = [
    { instance: tsNamespace, key: 'filter.allNamespaces' },
    { instance: tsType, key: 'filter.allTypes' },
    { instance: tsStatus, key: 'filter.allStatuses' },
    { instance: tsJob, key: 'filter.allServices' },
  ];
  for (const { instance, key } of updates) {
    if (!instance) continue;
    const translated = t(key);
    instance.settings.placeholder = translated;
    const input = instance.control_input;
    if (input) input.setAttribute('placeholder', translated);
  }
}

// --- Dynamic option sync ---

function syncTomSelectOptions(instance, newValues, activeSet) {
  if (!instance) return;

  instance.clearOptions();
  for (const val of newValues) {
    instance.addOption({ value: val, text: val });
  }

  // Prune active selections that no longer exist in data.
  for (const val of activeSet) {
    if (!newValues.includes(val)) {
      activeSet.delete(val);
    }
  }

  // Restore active selections without triggering onChange.
  instance.setValue([...activeSet], true);
}

// --- Exported functions ---

/**
 * Initialize filter panel from topology data.
 * Creates Tom Select instances, populates state chips, restores saved state.
 * @param {object} data - Topology response {nodes, edges}
 */
export function initFilters(data) {
  restoreFromStorage();
  initNamespaceSelect();
  initTypeSelect();
  initStatusSelect();
  initJobSelect();
  updateFilterValues(data);
  syncTomSelectOptions(tsType, knownValues.type, activeFilters.type);
  syncTomSelectOptions(tsJob, knownValues.job, activeFilters.job);
  // Restore status filter selection
  if (tsStatus && activeFilters.status.size > 0) {
    tsStatus.setValue([...activeFilters.status], true);
  }
  renderStateChips();

  // Update placeholders on language change
  window.addEventListener('language-changed', () => {
    updateTomSelectPlaceholders();
  });
}

/**
 * Update known filter values and sync Tom Select options from new topology data.
 * @param {object} data - Topology response {nodes, edges}
 */
export function updateFilters(data) {
  updateFilterValues(data);
  syncTomSelectOptions(tsType, knownValues.type, activeFilters.type);
  syncTomSelectOptions(tsJob, knownValues.job, activeFilters.job);
  renderStateChips();
}

/**
 * Update namespace dropdown options from topology data.
 * Preserves the current selection.
 * @param {object} data - Topology response {nodes, edges}
 */
export function updateNamespaceOptions(data) {
  if (!tsNamespace) return;

  const namespaces = new Set();
  if (data.nodes) {
    for (const node of data.nodes) {
      if (node.namespace) {
        namespaces.add(node.namespace);
      }
    }
  }

  const sorted = [...namespaces].sort();
  const current = tsNamespace.getValue();

  // Rebuild only if the set changed.
  const existing = Object.keys(tsNamespace.options).filter((k) => k !== '');
  if (sorted.length === existing.length && sorted.every((v, i) => v === existing[i])) {
    return;
  }

  tsNamespace.clearOptions();
  tsNamespace.addOption({ value: '', text: t('filter.allNamespaces') });
  for (const ns of sorted) {
    tsNamespace.addOption({ value: ns, text: ns });
  }
  tsNamespace.setValue(current, true);
}

/**
 * Set the namespace value in Tom Select (e.g. from URL param on init).
 * @param {string} value
 */
export function setNamespaceValue(value) {
  if (!tsNamespace) return;
  tsNamespace.setValue(value, true);
}

/**
 * Apply current filters to Cytoscape graph.
 * Uses show/hide — no element removal.
 * @param {import('cytoscape').Core} cy
 */
export function applyFilters(cy) {
  if (!cy) return;

  const hasTypeFilter = activeFilters.type.size > 0;
  const hasStateFilter = activeFilters.state.size > 0;
  const hasStatusFilter = activeFilters.status.size > 0;
  const hasJobFilter = activeFilters.job.size > 0;
  const hasAnyFilter = hasTypeFilter || hasStateFilter || hasStatusFilter || hasJobFilter;

  // 'warning' is a virtual state (frontend-computed cascade overlay, not a backend state).
  // A node matches 'warning' if it has cascadeCount > 0 and is not itself 'down'.
  const hasWarningFilter = hasStateFilter && activeFilters.state.has('warning');
  // State filters without 'warning' for direct backend state matching.
  const backendStateFilters = new Set(activeFilters.state);
  backendStateFilters.delete('warning');

  if (!hasAnyFilter) {
    cy.elements().show();
    return;
  }

  // Check if a node matches the active state filter (OR logic).
  // Matches if the backend state is in the filter set, OR if 'warning' is
  // selected and the node has cascade warnings.
  function matchesStateFilter(node) {
    if (!hasStateFilter) return true;
    const state = node.data ? node.data('state') : node.state;
    const cascadeCount = node.data ? node.data('cascadeCount') : 0;
    if (backendStateFilters.has(state)) return true;
    if (hasWarningFilter && cascadeCount > 0 && state !== 'down') return true;
    if (hasWarningFilter && (node.data ? node.data('inCascadeChain') : false)) return true;
    return false;
  }

  // Same check for collapsed children stored data objects.
  function matchesStateFilterData(dataObj) {
    if (!hasStateFilter) return true;
    if (backendStateFilters.has(dataObj.state)) return true;
    if (hasWarningFilter && (dataObj.cascadeCount || 0) > 0 && dataObj.state !== 'down') return true;
    if (hasWarningFilter && dataObj.inCascadeChain) return true;
    return false;
  }

  const groupingActive = isGroupingEnabled();

  cy.batch(() => {
    // If SERVICE filter is active, collect all downstream nodes
    let downstreamNodes = new Set();
    if (hasJobFilter) {
      activeFilters.job.forEach((serviceId) => {
        const node = cy.getElementById(serviceId);
        if (node && node.length > 0) {
          downstreamNodes.add(node);
          // Get all descendants using Cytoscape traversal
          const descendants = node.successors();
          descendants.forEach((element) => {
            if (element.isNode()) {
              downstreamNodes.add(element);
            }
          });
        }
      });
    }

    // First pass: determine node visibility (skip group nodes).
    cy.nodes().forEach((node) => {
      if (groupingActive && node.data('isGroup')) return;

      const type = node.data('type');
      const state = node.data('state');
      const id = node.data('id');
      let visible = true;

      // If SERVICE filter is active and node is in downstream set, always show it
      if (hasJobFilter && downstreamNodes.has(node)) {
        // Check state filter only
        if (hasStateFilter && !matchesStateFilter(node)) {
          visible = false;
        }
      } else if (type === 'service') {
        // Service node not in downstream - hide it
        if (hasJobFilter) {
          visible = false;
        }
        if (hasStateFilter && !matchesStateFilter(node)) {
          visible = false;
        }
      } else {
        // Dependency node
        if (hasTypeFilter && !activeFilters.type.has(type)) {
          visible = false;
        }
        if (hasStateFilter && !matchesStateFilter(node)) {
          visible = false;
        }
      }

      if (visible) {
        node.show();
      } else {
        node.hide();
      }
    });

    // Pass 1.5: if 'degraded' or 'down' is selected, also reveal the downstream
    // chain of problematic dependencies so the user sees WHY the node is degraded/down.
    if (hasStateFilter && (backendStateFilters.has('degraded') || backendStateFilters.has('down'))) {
      cy.nodes(':visible').forEach((node) => {
        if (node.data('isGroup')) return;
        const st = node.data('state');
        if (st !== 'degraded' && st !== 'down') return;
        // Follow outgoing edges to non-ok targets
        node.outgoers('edge').forEach((edge) => {
          const target = edge.target();
          const tState = target.data('state');
          if (tState && tState !== 'ok') {
            target.show();
          }
        });
      });
    }

    // Second pass: edges visible only if both endpoints are visible.
    cy.edges().forEach((edge) => {
      if (edge.source().visible() && edge.target().visible()) {
        edge.show();
      } else {
        edge.hide();
      }
    });

    // Pass 2.5: status filter — hide edges whose status doesn't match.
    if (hasStatusFilter) {
      cy.edges().forEach((edge) => {
        if (!edge.visible()) return;
        const edgeStatus = edge.data('status') || 'ok';
        if (!activeFilters.status.has(edgeStatus)) {
          edge.hide();
        }
      });

      // Re-evaluate node visibility: hide nodes with no visible edges
      // (unless they match state filter explicitly).
      cy.nodes().forEach((node) => {
        if (!node.visible()) return;
        if (groupingActive && node.data('isGroup')) return;
        if (hasStateFilter && matchesStateFilter(node)) return;
        const connectedEdges = node.connectedEdges();
        if (connectedEdges.length > 0 && connectedEdges.every((e) => !e.visible())) {
          node.hide();
        }
      });
    }

    // Third pass: hide orphan nodes (skip group nodes).
    // Skip nodes that explicitly match the state filter — the user wants to see them
    // even if their neighbors are filtered out.
    cy.nodes().forEach((node) => {
      if (!node.visible()) return;
      if (groupingActive && node.data('isGroup')) return;
      if (hasStateFilter && matchesStateFilter(node)) return;
      const connectedEdges = node.connectedEdges();
      if (connectedEdges.length > 0 && connectedEdges.every((e) => !e.visible())) {
        node.hide();
      }
    });

    // Fourth pass: namespace group visibility.
    // Expanded groups: hide if all children are hidden.
    // Collapsed groups: hide if no stored child would pass filters.
    if (groupingActive) {
      cy.nodes('[?isGroup]').forEach((groupNode) => {
        if (groupNode.data('isCollapsed')) {
          const nsName = groupNode.data('nsName');
          const children = getCollapsedChildren(nsName);
          if (!children) { groupNode.show(); return; }

          const anyMatch = children.some((child) => {
            const type = child.data.type;
            const id = child.data.id;

            if (type === 'service') {
              if (hasJobFilter && !activeFilters.job.has(id)) return false;
              if (hasStateFilter && !matchesStateFilterData(child.data)) return false;
            } else {
              if (hasTypeFilter && !activeFilters.type.has(type)) return false;
              if (hasStateFilter && !matchesStateFilterData(child.data)) return false;
            }
            return true;
          });

          if (anyMatch) {
            groupNode.show();
          } else {
            groupNode.hide();
            groupNode.connectedEdges().hide();
          }
        } else {
          // Expanded group: hide if all children are hidden
          const children = groupNode.children();
          if (children.length > 0 && children.every((c) => !c.visible())) {
            groupNode.hide();
          } else {
            groupNode.show();
          }
        }
      });

      // Re-check edges after group visibility changes
      cy.edges().forEach((edge) => {
        if (!edge.visible()) return;
        if (!edge.source().visible() || !edge.target().visible()) {
          edge.hide();
        }
      });
    }
  });
}

/**
 * Get a copy of current active filters (for external use).
 * @returns {{type: string[], state: string[], job: string[]}}
 */
export function getActiveFilters() {
  return {
    type: [...activeFilters.type],
    state: [...activeFilters.state],
    status: [...activeFilters.status],
    job: [...activeFilters.job],
  };
}

/**
 * Reset all filters (including Tom Select instances) and clear localStorage.
 */
export function resetFilters() {
  activeFilters.type.clear();
  activeFilters.state.clear();
  activeFilters.status.clear();
  activeFilters.job.clear();
  localStorage.removeItem(STORAGE_KEY);

  if (tsType) tsType.clear(true);
  if (tsStatus) tsStatus.clear(true);
  if (tsJob) tsJob.clear(true);
  if (tsNamespace) tsNamespace.setValue('', true);

  renderStateChips();
}

/**
 * Check if any filter is active.
 * @returns {boolean}
 */
export function hasActiveFilters() {
  return activeFilters.type.size > 0 || activeFilters.state.size > 0 || activeFilters.status.size > 0 || activeFilters.job.size > 0;
}

// --- Internal helpers ---

function updateFilterValues(data) {
  const types = new Set();
  const jobs = new Set();

  if (data.nodes) {
    for (const node of data.nodes) {
      if (node.type === 'service') {
        jobs.add(node.id);
      } else if (node.type) {
        types.add(node.type);
      }
    }
  }

  knownValues.type = [...types].sort();
  knownValues.job = [...jobs].sort();

  // Prune active selections that no longer exist in data.
  pruneSet(activeFilters.type, types);
  pruneSet(activeFilters.job, jobs);
}

function pruneSet(active, known) {
  for (const val of active) {
    if (!known.has(val)) {
      active.delete(val);
    }
  }
}

function renderStateChips() {
  const container = $('#state-chips');
  if (!container) return;

  container.innerHTML = '';

  for (const value of knownValues.state) {
    const chip = document.createElement('button');
    chip.className = 'filter-chip';
    chip.textContent = value;
    chip.dataset.value = value;

    if (activeFilters.state.has(value)) {
      chip.classList.add('active');
    }

    chip.addEventListener('click', () => {
      toggleStateFilter(value);
    });

    container.appendChild(chip);
  }
}

function toggleStateFilter(value) {
  if (activeFilters.state.has(value)) {
    activeFilters.state.delete(value);
  } else {
    activeFilters.state.add(value);
  }

  const chips = document.querySelectorAll(`.filter-chip[data-value="${value}"]`);
  chips.forEach((chip) => chip.classList.toggle('active'));

  saveToStorage();
  window.dispatchEvent(new CustomEvent('filters-changed'));
}

function saveToStorage() {
  const data = {
    type: [...activeFilters.type],
    state: [...activeFilters.state],
    status: [...activeFilters.status],
    job: [...activeFilters.job],
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
}

function restoreFromStorage() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return;
    const data = JSON.parse(raw);
    if (data.type) activeFilters.type = new Set(data.type);
    if (data.state) activeFilters.state = new Set(data.state);
    if (data.status) activeFilters.status = new Set(data.status);
    if (data.job) activeFilters.job = new Set(data.job);
  } catch {
    // Corrupted data — ignore.
  }
}
