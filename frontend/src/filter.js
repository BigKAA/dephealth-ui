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

/**
 * Initialize filter panel from topology data.
 * Populates chips, restores saved selections from localStorage.
 * @param {object} data - Topology response {nodes, edges}
 */
export function initFilters(data) {
  restoreFromStorage();
  updateFilterValues(data);
  renderChips();
}

/**
 * Update known filter values from new topology data.
 * Preserves active selections that still exist in data.
 * @param {object} data - Topology response {nodes, edges}
 */
export function updateFilters(data) {
  updateFilterValues(data);
  renderChips();
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
        // Service nodes: match by job (id) and state.
        if (hasJobFilter && !activeFilters.job.has(id)) {
          visible = false;
        }
        if (hasStateFilter && !activeFilters.state.has(state)) {
          visible = false;
        }
      } else {
        // Dependency nodes: match by type and state.
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
 * Reset all filters and clear localStorage.
 */
export function resetFilters() {
  activeFilters.type.clear();
  activeFilters.state.clear();
  activeFilters.job.clear();
  localStorage.removeItem(STORAGE_KEY);
  renderChips();
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

function renderChips() {
  renderGroup('filter-type', 'type', knownValues.type);
  renderGroup('filter-state', 'state', knownValues.state);
  renderGroup('filter-job', 'job', knownValues.job);
}

function renderGroup(containerId, dimension, values) {
  const container = $(`#${containerId}`);
  if (!container) return;

  // Keep the label, remove old chips.
  const label = container.querySelector('.filter-label');
  container.innerHTML = '';
  if (label) container.appendChild(label);

  for (const value of values) {
    const chip = document.createElement('button');
    chip.className = 'filter-chip';
    chip.textContent = value;
    chip.dataset.dimension = dimension;
    chip.dataset.value = value;

    if (activeFilters[dimension].has(value)) {
      chip.classList.add('active');
    }

    chip.addEventListener('click', () => {
      toggleFilter(dimension, value);
    });

    container.appendChild(chip);
  }
}

function toggleFilter(dimension, value) {
  if (activeFilters[dimension].has(value)) {
    activeFilters[dimension].delete(value);
  } else {
    activeFilters[dimension].add(value);
  }

  // Update chip visual state.
  const chips = document.querySelectorAll(`.filter-chip[data-dimension="${dimension}"][data-value="${value}"]`);
  chips.forEach((chip) => chip.classList.toggle('active'));

  saveToStorage();

  // Dispatch custom event so main.js can re-apply filters.
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
