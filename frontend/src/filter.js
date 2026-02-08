import TomSelect from 'tom-select';

const STORAGE_KEY = 'dephealth-filters';
const STATES = ['ok', 'degraded', 'down', 'unknown'];

const $ = (sel) => document.querySelector(sel);

// Active filter selections per dimension (empty Set = all visible).
let activeFilters = {
  type: new Set(),
  state: new Set(),
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
let tsNamespace = null;

// --- Tom Select initialization ---

function initNamespaceSelect() {
  const el = $('#namespace-select');
  tsNamespace = new TomSelect(el, {
    create: false,
    sortField: { field: 'text', direction: 'asc' },
    placeholder: 'All namespaces',
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
    placeholder: 'All types',
    onChange(values) {
      activeFilters.type = new Set(values);
      saveToStorage();
      window.dispatchEvent(new CustomEvent('filters-changed'));
    },
  });
}

function initJobSelect() {
  const el = $('#job-select');
  tsJob = new TomSelect(el, {
    create: false,
    plugins: ['remove_button'],
    placeholder: 'All services',
    onChange(values) {
      activeFilters.job = new Set(values);
      saveToStorage();
      window.dispatchEvent(new CustomEvent('filters-changed'));
    },
  });
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
  initJobSelect();
  updateFilterValues(data);
  syncTomSelectOptions(tsType, knownValues.type, activeFilters.type);
  syncTomSelectOptions(tsJob, knownValues.job, activeFilters.job);
  renderStateChips();
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
  tsNamespace.addOption({ value: '', text: 'All namespaces' });
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
  const hasJobFilter = activeFilters.job.size > 0;
  const hasAnyFilter = hasTypeFilter || hasStateFilter || hasJobFilter;

  if (!hasAnyFilter) {
    cy.elements().show();
    return;
  }

  cy.batch(() => {
    // First pass: determine node visibility.
    cy.nodes().forEach((node) => {
      const type = node.data('type');
      const state = node.data('state');
      const id = node.data('id');
      let visible = true;

      if (type === 'service') {
        if (hasJobFilter && !activeFilters.job.has(id)) {
          visible = false;
        }
        if (hasStateFilter && !activeFilters.state.has(state)) {
          visible = false;
        }
      } else {
        if (hasTypeFilter && !activeFilters.type.has(type)) {
          visible = false;
        }
        if (hasStateFilter && !activeFilters.state.has(state)) {
          visible = false;
        }
      }

      if (visible) {
        node.show();
      } else {
        node.hide();
      }
    });

    // Second pass: edges visible only if both endpoints are visible.
    cy.edges().forEach((edge) => {
      if (edge.source().visible() && edge.target().visible()) {
        edge.show();
      } else {
        edge.hide();
      }
    });

    // Third pass: hide orphan nodes (visible nodes with all edges hidden).
    cy.nodes().forEach((node) => {
      if (!node.visible()) return;
      const connectedEdges = node.connectedEdges();
      if (connectedEdges.length > 0 && connectedEdges.every((e) => !e.visible())) {
        node.hide();
      }
    });
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
    job: [...activeFilters.job],
  };
}

/**
 * Reset all filters (including Tom Select instances) and clear localStorage.
 */
export function resetFilters() {
  activeFilters.type.clear();
  activeFilters.state.clear();
  activeFilters.job.clear();
  localStorage.removeItem(STORAGE_KEY);

  if (tsType) tsType.clear(true);
  if (tsJob) tsJob.clear(true);
  if (tsNamespace) tsNamespace.setValue('', true);

  renderStateChips();
}

/**
 * Check if any filter is active.
 * @returns {boolean}
 */
export function hasActiveFilters() {
  return activeFilters.type.size > 0 || activeFilters.state.size > 0 || activeFilters.job.size > 0;
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
    if (data.job) activeFilters.job = new Set(data.job);
  } catch {
    // Corrupted data — ignore.
  }
}
