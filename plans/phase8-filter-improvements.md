# Phase 8: Filter Panel Improvements — Implementation Workflow

**Requirements:** `.tasks/requirements-filter-improvements.md`
**Branch:** `feature/filter-improvements`

## Overview

Replace chip-based Type and Service filters with Tom Select autocomplete multi-select inputs,
move Namespace selector into the filter panel, and reorganize the layout as a 4-column CSS grid.

## Files Affected

| File | Action | Description |
|------|--------|-------------|
| `frontend/package.json` | Modify | Add `tom-select` dependency |
| `frontend/index.html` | Modify | Restructure filter panel HTML, remove namespace from toolbar |
| `frontend/src/style.css` | Modify | CSS Grid layout, Tom Select theme overrides, responsive |
| `frontend/src/filter.js` | Rewrite | Tom Select for type/service/namespace, keep chips for state |
| `frontend/src/main.js` | Modify | Remove `setupNamespaceSelector()`, update init/reset flow |
| `frontend/vite.config.js` | Modify | Add tom-select to manualChunks (optional, for bundle optimization) |

---

## Phase 8.1: Dependencies & HTML Structure ✅

**Goal:** Install Tom Select, restructure HTML for 4-column grid layout.

### Step 1: Install Tom Select

```bash
cd frontend && npm install tom-select
```

### Step 2: Restructure `index.html`

**Remove** from `<header>` toolbar:
```html
<!-- DELETE this line -->
<select id="namespace-select" title="Filter by namespace">
  <option value="">All namespaces</option>
</select>
```

**Replace** `#filter-panel` contents with:
```html
<div id="filter-panel" class="hidden">
  <div class="filter-column" id="filter-namespace">
    <span class="filter-label">Namespace</span>
    <select id="namespace-select" placeholder="All namespaces"></select>
  </div>
  <div class="filter-column" id="filter-type">
    <span class="filter-label">Type</span>
    <select id="type-select" multiple placeholder="All types"></select>
  </div>
  <div class="filter-column" id="filter-state">
    <span class="filter-label">State</span>
    <div class="filter-chips" id="state-chips"></div>
  </div>
  <div class="filter-column" id="filter-job">
    <span class="filter-label">Service</span>
    <select id="job-select" multiple placeholder="All services"></select>
  </div>
  <div class="filter-column filter-actions">
    <span class="filter-label">&nbsp;</span>
    <button id="btn-reset-filters" title="Reset all filters">Reset</button>
  </div>
</div>
```

**Key changes:**
- `filter-group` → `filter-column` (semantic name for grid layout)
- Each column has a `filter-label` on top + control below (vertical stack)
- Namespace uses `<select>` (single, Tom Select will enhance it)
- Type and Service use `<select multiple>` (Tom Select multi-select tagging mode)
- State keeps a `div.filter-chips` container for chip buttons
- Labels are now header-style (no colon, above the control)

### Checkpoint 8.1
- [ ] `npm install` succeeds
- [ ] Page loads without errors (Tom Select not initialized yet)
- [ ] Namespace select removed from toolbar
- [ ] Filter panel HTML shows new structure

---

## Phase 8.2: CSS Grid Layout & Tom Select Theming ✅

**Goal:** Style the 4-column grid, override Tom Select CSS for project theme.

### Step 1: Import Tom Select CSS

In `main.js` add at the top:
```js
import 'tom-select/dist/css/tom-select.default.css';
```

### Step 2: Replace `#filter-panel` styles in `style.css`

**Replace** the old flex-based `#filter-panel` block (lines 192-267) with:

```css
/* Filter panel — 4-column grid */
#filter-panel {
  display: grid;
  grid-template-columns: 1fr 1fr auto 1fr auto;
  gap: 12px;
  padding: 10px 16px;
  background: var(--bg-primary);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  align-items: end;
}

#filter-panel.hidden {
  display: none;
}

.filter-column {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;  /* Allow shrinking in grid */
}

.filter-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* State chips inside filter-column */
.filter-chips {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  align-items: center;
  min-height: 34px;  /* Match Tom Select height */
}

/* Keep existing .filter-chip styles unchanged */

/* Reset button in its column */
.filter-actions {
  /* No margin-left:auto needed — grid handles alignment */
}

.filter-actions button {
  padding: 6px 16px;
  font-size: 12px;
  border: 1px solid var(--border-light);
  border-radius: 4px;
  background: var(--bg-primary);
  color: var(--text-secondary);
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s;
  height: 34px;  /* Match Tom Select height */
}
```

### Step 3: Tom Select theme overrides

```css
/* Tom Select — theme integration */
.ts-wrapper {
  --ts-pr-clear-button: 0;
  font-size: 12px;
}

.ts-wrapper .ts-control {
  background: var(--bg-secondary);
  border-color: var(--border-light);
  color: var(--text-primary);
  border-radius: 4px;
  min-height: 34px;
  padding: 2px 8px;
  transition: border-color 0.15s;
}

.ts-wrapper .ts-control:hover {
  border-color: var(--border-hover);
}

.ts-wrapper.focus .ts-control {
  border-color: var(--btn-active-border);
  box-shadow: 0 0 0 1px var(--btn-active-border);
}

.ts-wrapper .ts-control input {
  color: var(--text-primary);
}

.ts-wrapper .ts-control input::placeholder {
  color: var(--text-muted);
}

/* Dropdown */
.ts-dropdown {
  background: var(--bg-primary);
  border-color: var(--border-light);
  color: var(--text-primary);
  border-radius: 4px;
  box-shadow: 0 4px 12px var(--shadow);
  z-index: 100;
}

.ts-dropdown .active {
  background: var(--btn-active-bg);
  color: var(--btn-active-text);
}

.ts-dropdown .option:hover {
  background: var(--btn-hover-bg);
}

.ts-dropdown .no-results {
  color: var(--text-muted);
  padding: 8px;
  font-size: 12px;
}

/* Tags (multi-select items) */
.ts-wrapper.multi .ts-control > .item {
  background: var(--btn-active-bg);
  border: 1px solid var(--btn-active-border);
  color: var(--btn-active-text);
  border-radius: 10px;
  padding: 1px 8px;
  font-size: 11px;
  margin: 1px 2px;
}

.ts-wrapper.multi .ts-control > .item .remove {
  border: none;
  color: var(--btn-active-text);
}

/* Single-select (namespace) */
.ts-wrapper.single .ts-control::after {
  border-color: var(--text-muted) transparent transparent transparent;
}
```

### Step 4: Responsive overrides

```css
@media (max-width: 900px) {
  #filter-panel {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 600px) {
  #filter-panel {
    grid-template-columns: 1fr;
    padding: 6px 12px;
    gap: 8px;
  }
}
```

### Checkpoint 8.2
- [ ] Tom Select CSS loads (no 404)
- [ ] Grid layout renders 4 columns + reset
- [ ] Theme colors match project design tokens
- [ ] Dark theme correct
- [ ] Responsive breakpoints work (resize browser)

---

## Phase 8.3: filter.js — Tom Select Integration ✅

**Goal:** Rewrite filter.js to use Tom Select for Type, Service, Namespace. Keep chips for State.

### Step 1: Imports and Constants

```js
import TomSelect from 'tom-select';

const STORAGE_KEY = 'dephealth-filters';
const STATES = ['ok', 'degraded', 'down', 'unknown'];
const $ = (sel) => document.querySelector(sel);
```

### Step 2: Module State

```js
let activeFilters = {
  type: new Set(),
  state: new Set(),
  job: new Set(),
};

let knownValues = {
  type: [],
  state: STATES,
  job: [],
};

// Tom Select instances
let tsType = null;
let tsJob = null;
let tsNamespace = null;
```

### Step 3: Initialize Tom Select Instances

**Namespace (single-select with search):**
```js
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
```

**Type (multi-select tagging):**
```js
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
```

**Service (multi-select tagging):**
```js
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
```

### Step 4: Dynamic Option Updates

```js
function syncTomSelectOptions(instance, newValues, activeSet) {
  // Clear all existing options
  instance.clearOptions();

  // Add new options
  for (const val of newValues) {
    instance.addOption({ value: val, text: val });
  }

  // Prune active selections that no longer exist
  for (const val of activeSet) {
    if (!newValues.includes(val)) {
      activeSet.delete(val);
    }
  }

  // Restore active selections without triggering onChange
  instance.setValue([...activeSet], true);
}
```

### Step 5: Exported Functions

**`initFilters(data)`** — called once on startup:
1. `restoreFromStorage()`
2. `initNamespaceSelect()` + `initTypeSelect()` + `initJobSelect()`
3. `updateFilterValues(data)`
4. Render state chips
5. Sync Tom Select values from restored state

**`updateFilters(data)`** — called on each refresh:
1. `updateFilterValues(data)`
2. `syncTomSelectOptions(tsType, knownValues.type, activeFilters.type)`
3. `syncTomSelectOptions(tsJob, knownValues.job, activeFilters.job)`
4. Render state chips

**`updateNamespaceOptions(namespaces)`** — new export:
1. Update `tsNamespace` options from namespace list
2. Preserve current selection

**`resetFilters()`** — enhanced:
1. Clear all `activeFilters`
2. Clear all Tom Select instances: `tsType.clear()`, `tsJob.clear()`, `tsNamespace.setValue('', true)`
3. Clear localStorage
4. Re-render state chips

**Keep unchanged:** `applyFilters(cy)`, `getActiveFilters()`, `hasActiveFilters()`

### Step 6: State Chips (minimal change)

State chips rendering moves from `renderGroup()` to a dedicated `renderStateChips()` that targets `#state-chips` div.
Logic remains identical (toggle on click, update CSS class, save to storage, dispatch event).

### Checkpoint 8.3
- [ ] Tom Select instances render for Namespace, Type, Service
- [ ] State chips render correctly
- [ ] Selecting options in Tom Select updates activeFilters
- [ ] State chip toggle works
- [ ] localStorage save/restore works
- [ ] Removing a tag updates the filter correctly

---

## Phase 8.4: main.js — Integration ✅

**Goal:** Wire up the new filter.js exports, remove old namespace logic.

### Step 1: Remove `setupNamespaceSelector()` function (lines 233-258)

This function is fully replaced by filter.js namespace handling.

### Step 2: Update `init()` function

```js
async function init() {
  initTheme();

  // ... existing config/auth init ...

  cy = initGraph($('#cy'));
  setupToolbar();
  setupFilters();        // still sets up panel toggle + reset + filters-changed listener
  setupGrafanaClickThrough();

  const data = await withRetry(() => fetchTopology(selectedNamespace || undefined));
  renderGraph(cy, data);
  updateStatus(data);
  checkEmptyState(data);
  initFilters(data);     // Now handles namespace + type/service/state
  applyFilters(cy);
  startPolling();
}
```

Note: `setupNamespaceSelector()` call removed.

### Step 3: Update `setupFilters()` — reset handler

```js
$('#btn-reset-filters').addEventListener('click', () => {
  resetFilters();                        // Now clears Tom Select instances too
  selectedNamespace = '';
  const url = new URL(window.location);
  url.searchParams.delete('namespace');
  history.replaceState(null, '', url);
  applyFilters(cy);
  refresh();
});
```

### Step 4: Add namespace-changed listener

```js
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
```

### Step 5: Update `refresh()` — namespace options

```js
async function refresh() {
  try {
    const data = await fetchTopology(selectedNamespace || undefined);
    renderGraph(cy, data);
    updateStatus(data);
    checkEmptyState(data);
    updateNamespaceOptions(data);  // Now calls filter.js export
    updateFilters(data);
    applyFilters(cy);
    // ... rest unchanged
  }
}
```

`updateNamespaceOptions()` is now imported from `filter.js` instead of being defined locally.

### Step 6: Update imports in main.js

```js
import {
  initFilters, updateFilters, applyFilters, resetFilters,
  hasActiveFilters, updateNamespaceOptions
} from './filter.js';
```

### Step 7: Restore namespace from URL on init

In `init()`, before first `fetchTopology`:
```js
const params = new URLSearchParams(window.location.search);
selectedNamespace = params.get('namespace') || '';
```

And after `initFilters(data)`, set namespace in Tom Select:
```js
if (selectedNamespace) {
  setNamespaceValue(selectedNamespace);  // New export from filter.js
}
```

### Checkpoint 8.4
- [ ] Page loads without errors
- [ ] Namespace select works (changes trigger API refresh)
- [ ] URL `?namespace=` synced correctly
- [ ] Type/Service filters affect graph visibility
- [ ] State chips work as before
- [ ] Reset clears all filters including namespace
- [ ] Auto-refresh updates options dynamically
- [ ] Theme toggle doesn't break Tom Select

---

## Phase 8.5: Build, Deploy & Test ✅ (build done, deploy pending)

**Goal:** Verify production build and deploy to Kubernetes.

### Step 1: Vite Config (optional optimization)

Add tom-select to manual chunks in `vite.config.js`:
```js
manualChunks: {
  cytoscape: ['cytoscape', 'cytoscape-dagre', 'dagre'],
  'tom-select': ['tom-select'],
},
```

### Step 2: Build & Verify

```bash
cd frontend && npm run build
```
- Check `dist/` output — no build errors
- Check bundle sizes (tom-select chunk should be ~30KB)

### Step 3: Docker Build

```bash
make docker-build TAG=v0.7.0
```

### Step 4: Deploy to Kubernetes

Update `deploy/helm/dephealth-ui/values.yaml` with new tag, then:
```bash
helm upgrade dephealth-ui deploy/helm/dephealth-ui -n dephealth-ui
```

### Step 5: Manual Testing Checklist

- [ ] Filter panel toggle button works
- [ ] Namespace dropdown shows all namespaces, search works
- [ ] Selecting namespace refreshes graph
- [ ] Type autocomplete shows types, multi-select works
- [ ] Service autocomplete shows services, multi-select works
- [ ] State chips toggle correctly
- [ ] Reset clears all filters and namespace
- [ ] URL `?namespace=` preserved on page reload
- [ ] Filter selections persist in localStorage across page reload
- [ ] Dark theme: all Tom Select elements themed correctly
- [ ] Light theme: all Tom Select elements themed correctly
- [ ] Responsive: 2 columns on tablet-width
- [ ] Responsive: 1 column on mobile-width
- [ ] Graph filtering works correctly (service/dependency/state combinations)
- [ ] Orphan nodes hidden when filters active
- [ ] Auto-refresh updates filter options (add/remove test services)

### Checkpoint 8.5
- [ ] Production build succeeds
- [ ] Docker image built and pushed
- [ ] K8s deployment updated
- [ ] All manual tests pass

---

## Summary of Phases

| Phase | Description | Files Changed | Estimated Complexity |
|-------|-------------|---------------|---------------------|
| 8.1 | Dependencies & HTML | `package.json`, `index.html` | Low |
| 8.2 | CSS Grid & Tom Select theme | `style.css` | Medium |
| 8.3 | filter.js rewrite | `filter.js` | High |
| 8.4 | main.js integration | `main.js` | Medium |
| 8.5 | Build, deploy, test | `vite.config.js`, helm values | Low |

## Dependencies Between Phases

```
8.1 ──→ 8.2 ──→ 8.3 ──→ 8.4 ──→ 8.5
 │              ↑
 └──────────────┘ (HTML structure needed for JS init)
```

All phases are sequential — each builds on the previous.
