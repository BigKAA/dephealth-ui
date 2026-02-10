# Plan: UI v2 — Interface Improvements

Source: `.tasks/interface.md`
Status: **Not Started**

---

## Table of Contents

- [x] [Phase 11: Backend — Alerts severity config + models](#phase-11-backend--alerts-severity-config--models)
- [x] [Phase 12: Frontend — Bootstrap Icons + Floating Toolbar](#phase-12-frontend--bootstrap-icons--floating-toolbar)
- [x] [Phase 13: Frontend — Alert Badges + Tooltip](#phase-13-frontend--alert-badges--tooltip)
- [x] [Phase 14: Frontend — Legend + Node Detail Sidebar](#phase-14-frontend--legend--node-detail-sidebar)
- [ ] [Phase 15: Frontend — Search + Layout Toggle + Export PNG](#phase-15-frontend--search--layout-toggle--export-png)
- [ ] [Phase 16: Frontend — Alert Drawer + Stats + Fullscreen + Hotkeys](#phase-16-frontend--alert-drawer--stats--fullscreen--hotkeys)
- [ ] [Phase 17: Backend+Frontend — Pods API + Pod Display](#phase-17-backendfrontend--pods-api--pod-display)
- [ ] [Phase 18: Build + Deploy + Verify](#phase-18-build--deploy--verify)

---

## Overview

Implementation plan for 12 new UI features (interface.md §1–§12).
Each phase fits in one AI context window. Phases are ordered by dependencies.

Numbering continues from project history (Phase 10 = last completed).

---

## Dependencies between features

```
Phase 11 (backend: alerts config)
    └──► Phase 13 (alert badges + tooltip)

Phase 12 (floating toolbar)
    ├──► Phase 15 (search + layout toggle + export)
    ├──► Phase 16.5 (fullscreen button)
    └──► Phase 17 (pods button)

Phase 14 (sidebar)
    └──► Phase 16.6 (Esc keyboard shortcut — close sidebar)

Phase 16 (alert drawer, stats, fullscreen, hotkeys)
    └── depends on: all features exist to wire shortcuts
```

Features without dependencies (can be implemented in any order):
- Tooltip (§7)
- Legend (§8)
- Stats summary (§10)

---

## Phase 11: Backend — Alerts severity config + models

**Goal:** Add configurable severity levels, compute `alertSeverity` for nodes and edges,
expose severity config to frontend via `/api/v1/config`.

**Source:** interface.md §2.1, §2.2, §2.4

### 11.1. Config: AlertsConfig struct

**File:** `internal/config/config.go`

Add new types:

```go
type SeverityLevel struct {
    Value string `yaml:"value" json:"value"`
    Color string `yaml:"color" json:"color"`
}

type AlertsConfig struct {
    SeverityLabel  string          `yaml:"severityLabel"`
    SeverityLevels []SeverityLevel `yaml:"severityLevels"`
}
```

Add `Alerts AlertsConfig` field to `Config` struct.

### 11.2. Defaults

**File:** `internal/config/config.go` — `defaultConfig()`

```go
Alerts: AlertsConfig{
    SeverityLabel: "severity",
    SeverityLevels: []SeverityLevel{
        {Value: "critical", Color: "#f44336"},
        {Value: "warning",  Color: "#ff9800"},
        {Value: "info",     Color: "#2196f3"},
    },
},
```

### 11.3. Environment variable overrides

**File:** `internal/config/config.go` — `applyEnvOverrides()`

- `DEPHEALTH_ALERTS_SEVERITYLABEL` → `cfg.Alerts.SeverityLabel`
- `DEPHEALTH_ALERTS_SEVERITYLEVELS` → JSON decode into `cfg.Alerts.SeverityLevels`

### 11.4. Validation

**File:** `internal/config/config.go` — `Validate()`

- `SeverityLevels` must not be empty.
- Each level must have non-empty `Value` and `Color`.
- `Color` must be a valid hex color (regexp `^#[0-9a-fA-F]{6}$`).

### 11.5. Config tests

**File:** `internal/config/config_test.go`

- Test default severity levels.
- Test YAML override.
- Test env var override (both `SEVERITYLABEL` and JSON `SEVERITYLEVELS`).
- Test validation errors (empty levels, invalid color).

### 11.6. Models: alertSeverity fields

**File:** `internal/topology/models.go`

Add to `Node`:
```go
AlertSeverity string `json:"alertSeverity,omitempty"`
```

Add to `Edge`:
```go
AlertCount    int    `json:"alertCount,omitempty"`
AlertSeverity string `json:"alertSeverity,omitempty"`
```

### 11.7. GraphBuilder: severity computation

**File:** `internal/topology/graph.go`

- Pass `AlertsConfig` (or at least the ordered list of severity values) to `GraphBuilder`.
  Add field to `GraphBuilder` struct and `NewGraphBuilder()` constructor.
- In `enrichWithAlerts()`:
  - Track per-node and per-edge alert severity values.
  - For each node/edge, determine worst severity using the configured priority order
    (index 0 = most critical).
  - Set `AlertSeverity` on Node and Edge structs.
- Note: `alertCount` for nodes is currently computed only in frontend (`main.js`).
  Move this computation to backend so `Node.AlertCount` is populated server-side
  (was previously set to 0 in `renderGraph`). This ensures consistent data in API.

### 11.8. Config handler: expose alerts section

**File:** `internal/server/routes.go` (or wherever `/api/v1/config` handler is)

Include alerts config in the response:
```json
{
  "grafana": { ... },
  "cache": { ... },
  "auth": { ... },
  "alerts": {
    "severityLevels": [...]
  }
}
```

### 11.9. Update wiring in main.go

**File:** `cmd/dephealth-ui/main.go`

Pass `cfg.Alerts` to `NewGraphBuilder()`.

### 11.10. Tests

**Files:** `internal/topology/graph_test.go`, `internal/server/routes_test.go`

- Test that `enrichWithAlerts()` correctly sets `AlertSeverity` on nodes and edges.
- Test worst-severity logic (critical > warning > info).
- Test config endpoint includes alerts section.

**Checklist:**
- [x] 11.1 AlertsConfig struct
- [x] 11.2 Defaults
- [x] 11.3 Env var overrides
- [x] 11.4 Validation
- [x] 11.5 Config tests
- [x] 11.6 Models: alertSeverity fields
- [x] 11.7 GraphBuilder: severity computation
- [x] 11.8 Config handler: alerts section
- [x] 11.9 Wiring in main.go
- [x] 11.10 Tests (graph + routes)

---

## Phase 12: Frontend — Bootstrap Icons + Floating Toolbar

**Goal:** Replace text button labels with Bootstrap Icons.
Create a draggable floating toolbar over the graph area with Zoom In/Out/Fit buttons.

**Source:** interface.md §1

### 12.1. Install bootstrap-icons

```bash
cd frontend && npm install bootstrap-icons
```

Import in `main.js`:
```js
import 'bootstrap-icons/font/bootstrap-icons.css';
```

### 12.2. Replace header toolbar text with icons

**File:** `frontend/index.html`, `frontend/src/main.js`

Replace button text:
- "Filter" → `<i class="bi bi-funnel"></i>`
- "Refresh" → `<i class="bi bi-arrow-clockwise"></i>`
- "Fit" → `<i class="bi bi-arrows-fullscreen"></i>`
- "Auto" → `<i class="bi bi-play-circle"></i>` / `<i class="bi bi-pause-circle"></i>`
- "Dark"/"Light" → `<i class="bi bi-moon"></i>` / `<i class="bi bi-sun"></i>`
- "Logout" → `<i class="bi bi-box-arrow-right"></i>`

Keep `title` attributes for accessibility.

### 12.3. Floating toolbar HTML

**File:** `frontend/index.html`

Add inside `#cy` container:
```html
<div id="graph-toolbar" class="graph-toolbar">
  <button id="btn-zoom-in" title="Zoom in"><i class="bi bi-zoom-in"></i></button>
  <button id="btn-zoom-out" title="Zoom out"><i class="bi bi-zoom-out"></i></button>
  <button id="btn-toolbar-fit" title="Fit to screen"><i class="bi bi-arrows-fullscreen"></i></button>
</div>
```

### 12.4. Floating toolbar CSS

**File:** `frontend/src/style.css`

```css
.graph-toolbar {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 50;
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 6px;
  box-shadow: 0 2px 8px var(--shadow);
  cursor: grab;
}

.graph-toolbar button { /* icon buttons */ }
```

### 12.5. Drag & drop

**File:** `frontend/src/toolbar.js` (new file)

Implement mousedown/mousemove/mouseup handlers on `#graph-toolbar` for repositioning.
Save position to `localStorage` key `dephealth-toolbar-pos`.
Restore on init.

### 12.6. Wire buttons

**File:** `frontend/src/main.js`

```js
$('#btn-zoom-in').addEventListener('click', () => cy.zoom(cy.zoom() * 1.2));
$('#btn-zoom-out').addEventListener('click', () => cy.zoom(cy.zoom() / 1.2));
$('#btn-toolbar-fit').addEventListener('click', () => cy.fit(50));
```

### 12.7. Test manually in browser

- Verify icons render correctly in both themes.
- Verify toolbar drag works and position persists.
- Verify zoom buttons work.

**Checklist:**
- [x] 12.1 Install bootstrap-icons
- [x] 12.2 Replace header text with icons
- [x] 12.3 Floating toolbar HTML
- [x] 12.4 Floating toolbar CSS
- [x] 12.5 Drag & drop (toolbar.js)
- [x] 12.6 Wire zoom/fit buttons
- [x] 12.7 Manual browser test

---

## Phase 13: Frontend — Alert Badges + Tooltip

**Goal:** Render colored alert severity badges on nodes/edges.
Show tooltip on hover.

**Source:** interface.md §2.3, §7

### 13.1. Fetch severity config

**File:** `frontend/src/main.js` or `frontend/src/api.js`

On init, after `fetchConfig()`, store `config.alerts.severityLevels` in a module-level variable.
Build a lookup: `severityColorMap = { critical: '#f44336', warning: '#ff9800', ... }`.

### 13.2. Pass alertSeverity to Cytoscape

**File:** `frontend/src/graph.js` — `renderGraph()`

In the node-add loop, pass:
```js
alertSeverity: node.alertSeverity || undefined
```

In the edge-add loop, pass:
```js
alertCount: edge.alertCount || 0,
alertSeverity: edge.alertSeverity || undefined
```

### 13.3. Dynamic CSS styles for badges

**File:** `frontend/src/graph.js`

Generate Cytoscape style entries for each severity level.
For each level, add a selector `node[alertCount > 0][alertSeverity = "<value>"]` with:

```js
{
  'text-margin-x': 60,        // shift to right edge of node
  'text-margin-y': -15,       // shift above node
  'text-background-color': level.color,
  'text-background-opacity': 1,
  'text-background-shape': 'round-rectangle',
  'text-background-padding': '3px',
  // ... use secondary label approach or overlay
}
```

Note: Cytoscape single-label limitation — may need `source-label`/`target-label` hack
on a self-loop, or use the canvas overlay approach from `cy.on('render')`.
Investigate the best Cytoscape approach and document the chosen solution.

### 13.4. Edge alert markers

For edges with `alertCount > 0`, add `source-label` showing a dot/icon:
```js
'source-label': '⬤',
'source-text-offset': 20,
'color': level.color,
```

### 13.5. Tooltip — HTML element

**File:** `frontend/index.html`

Add:
```html
<div id="graph-tooltip" class="graph-tooltip hidden"></div>
```

### 13.6. Tooltip — CSS

**File:** `frontend/src/style.css`

```css
.graph-tooltip {
  position: absolute;
  z-index: 60;
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 12px;
  box-shadow: 0 2px 8px var(--shadow);
  pointer-events: none;
  max-width: 280px;
}
```

### 13.7. Tooltip — JS logic

**File:** `frontend/src/tooltip.js` (new file)

Export `initTooltip(cy)`:
- `cy.on('mouseover', 'node', (evt) => { ... })` — show tooltip with node info.
- `cy.on('mouseover', 'edge', (evt) => { ... })` — show tooltip with edge info.
- `cy.on('mouseout', () => { ... })` — hide tooltip.
- Position tooltip near cursor using `evt.renderedPosition`.

Wire in `main.js` after `initGraph()`.

**Checklist:**
- [x] 13.1 Fetch severity config + build color map
- [x] 13.2 Pass alertSeverity to Cytoscape data
- [x] 13.3 Dynamic CSS badge styles (nodes)
- [x] 13.4 Edge alert markers
- [x] 13.5 Tooltip HTML
- [x] 13.6 Tooltip CSS
- [x] 13.7 Tooltip JS logic (tooltip.js)

---

## Phase 14: Frontend — Legend + Node Detail Sidebar

**Goal:** Add color legend panel. Add slide-in sidebar for node details on click.

**Source:** interface.md §8, §3

### 14.1. Legend — HTML

**File:** `frontend/index.html`

Add inside `#cy`:
```html
<div id="graph-legend" class="graph-legend">
  <div class="legend-title">Legend <button id="btn-legend-close">×</button></div>
  <div class="legend-item"><span class="legend-dot" style="background:#4caf50"></span> OK</div>
  <div class="legend-item"><span class="legend-dot" style="background:#ff9800"></span> Degraded</div>
  <div class="legend-item"><span class="legend-dot" style="background:#f44336"></span> Down</div>
  <div class="legend-item"><span class="legend-dot" style="background:#9e9e9e"></span> Unknown</div>
  <div class="legend-item"><span class="legend-line thick"></span> Critical</div>
  <div class="legend-item"><span class="legend-line dashed"></span> Degraded edge</div>
  <div class="legend-item"><span class="legend-line dotted"></span> Down edge</div>
</div>
```

### 14.2. Legend — CSS

**File:** `frontend/src/style.css`

Absolute position in bottom-left of `#cy`, semi-transparent background, compact layout.

### 14.3. Legend toggle

Add button to floating toolbar: `<i class="bi bi-info-circle"></i>`.
Toggle legend visibility. Save state in `localStorage`.

### 14.4. Node Detail Sidebar — HTML

**File:** `frontend/index.html`

```html
<div id="node-sidebar" class="node-sidebar hidden">
  <div class="sidebar-header">
    <h3 id="sidebar-title"></h3>
    <button id="btn-sidebar-close">×</button>
  </div>
  <div class="sidebar-body">
    <div id="sidebar-details"></div>
    <div id="sidebar-alerts"></div>
    <div id="sidebar-edges"></div>
    <div id="sidebar-actions"></div>
  </div>
</div>
```

### 14.5. Sidebar — CSS

**File:** `frontend/src/style.css`

- `position: fixed; right: 0; top: 48px; bottom: 28px; width: 320px;`
- Slide-in animation: `transform: translateX(100%)` → `translateX(0)`.
- Dark/light theme support via CSS variables.

### 14.6. Sidebar — JS logic

**File:** `frontend/src/sidebar.js` (new file)

Export `initSidebar(cy, topologyData)`:

- `cy.on('tap', 'node', (evt) => { ... })` — open sidebar with node info.
  - Replace current Grafana click-through (which was on single tap).
- `cy.on('dbltap', 'node', (evt) => { ... })` — open Grafana URL.
- Sidebar content:
  - Node: name, state (colored badge), namespace, type, host:port.
  - Related alerts from `topologyData.alerts.filter(a => a.service === nodeId)`.
  - Connected edges with latency.
  - Button "Open in Grafana" (if grafanaUrl exists).
- Click outside sidebar or `×` button → close.

Wire in `main.js`.

### 14.7. Update Grafana click-through

**File:** `frontend/src/main.js`

Remove `setupGrafanaClickThrough()` — replaced by sidebar + dblclick logic in sidebar.js.
Keep edge tap → Grafana (or optionally show edge detail in sidebar).

**Checklist:**
- [x] 14.1 Legend HTML
- [x] 14.2 Legend CSS
- [x] 14.3 Legend toggle button in toolbar
- [x] 14.4 Sidebar HTML
- [x] 14.5 Sidebar CSS
- [x] 14.6 Sidebar JS logic (sidebar.js)
- [x] 14.7 Update Grafana click-through

---

## Phase 15: Frontend — Search + Layout Toggle + Export PNG

**Goal:** Add node search with highlight. Add layout direction switch.
Add PNG export.

**Source:** interface.md §4, §5, §6

### 15.1. Search — UI

Add search button to floating toolbar: `<i class="bi bi-search"></i>`.
On click → show search input overlay near toolbar:
```html
<div id="search-panel" class="search-panel hidden">
  <input id="search-input" type="text" placeholder="Search nodes..." />
  <span id="search-count"></span>
</div>
```

### 15.2. Search — CSS

**File:** `frontend/src/style.css`

Position near floating toolbar. Input field with matching theme styles.

### 15.3. Search — JS logic

**File:** `frontend/src/search.js` (new file)

Export `initSearch(cy)`:
- On input: filter nodes by substring match (case-insensitive) on `label` and `id`.
- Matching nodes: full opacity. Non-matching: opacity 0.15.
- Show match count: "3 / 42 nodes".
- Enter: center on current match (`cy.animate({ center: { eles: node }, zoom: 1.5 })`).
- Repeated Enter: cycle through matches.
- Esc: close search, restore opacity.

### 15.4. Layout toggle — button

Add to floating toolbar: `<i class="bi bi-distribute-vertical"></i>` (TB)
/ `<i class="bi bi-distribute-horizontal"></i>` (LR).

### 15.5. Layout toggle — JS logic

**File:** `frontend/src/graph.js`

Export `relayout(cy, direction)`:
```js
export function relayout(cy, direction = 'TB') {
  cy.layout({ name: 'dagre', rankDir: direction, nodeSep: 80, rankSep: 120, animate: true }).run();
}
```

In `main.js`:
- Track current direction in variable (default `TB`).
- On button click: toggle, call `relayout()`, save to `localStorage`.
- On page load: restore from `localStorage`.

### 15.6. Export PNG — button

Add to floating toolbar: `<i class="bi bi-download"></i>`.

### 15.7. Export PNG — JS logic

**File:** `frontend/src/main.js` (inline, small)

```js
$('#btn-export').addEventListener('click', () => {
  const bg = document.documentElement.dataset.theme === 'dark' ? '#1e1e1e' : '#ffffff';
  const dataUrl = cy.png({ full: true, scale: 2, bg });
  const a = document.createElement('a');
  a.href = dataUrl;
  a.download = `dephealth-topology-${Date.now()}.png`;
  a.click();
});
```

**Checklist:**
- [ ] 15.1 Search UI (button + input overlay)
- [ ] 15.2 Search CSS
- [ ] 15.3 Search JS logic (search.js)
- [ ] 15.4 Layout toggle button
- [ ] 15.5 Layout toggle JS logic
- [ ] 15.6 Export PNG button
- [ ] 15.7 Export PNG JS logic

---

## Phase 16: Frontend — Alert Drawer + Stats + Fullscreen + Hotkeys

**Goal:** Add alert list panel. Add health stats to status bar.
Add fullscreen mode. Add keyboard shortcuts.

**Source:** interface.md §9, §10, §11, §12

### 16.1. Alert Drawer — button in header

**File:** `frontend/index.html`

Add button to header toolbar (with badge):
```html
<button id="btn-alerts" title="Active alerts">
  <i class="bi bi-bell"></i>
  <span id="alert-badge" class="alert-badge hidden">0</span>
</button>
```

### 16.2. Alert Drawer — HTML panel

```html
<div id="alert-drawer" class="alert-drawer hidden">
  <div class="drawer-header">
    <h3>Active Alerts</h3>
    <button id="btn-drawer-close">×</button>
  </div>
  <div id="alert-list" class="alert-list"></div>
</div>
```

### 16.3. Alert Drawer — CSS

Slide-in from left, dark/light theme, severity color indicators.

### 16.4. Alert Drawer — JS logic

**File:** `frontend/src/alerts.js` (new file)

Export `initAlertDrawer(cy)` and `updateAlertDrawer(alerts, severityLevels)`:
- Group alerts by severity (order from config).
- Render each alert: colored left border, alertname, service → dependency, since.
- Click on alert → `cy.animate({ center: { eles: cy.getElementById(alert.service) } })`.
- Update badge count on header button.

### 16.5. Stats summary

**File:** `frontend/src/main.js` — `updateStatus()`

Add health stats computation:
```js
const counts = { ok: 0, degraded: 0, down: 0, unknown: 0 };
data.nodes.forEach(n => counts[n.state] = (counts[n.state] || 0) + 1);
```

Render as colored segments in status bar:
```html
<span id="status-stats">
  <span class="stat-ok">12 OK</span> |
  <span class="stat-degraded">3 Degraded</span> |
  <span class="stat-down">1 Down</span>
</span>
```

### 16.6. Fullscreen — button

Add to floating toolbar: `<i class="bi bi-fullscreen"></i>`.

### 16.7. Fullscreen — JS logic

```js
$('#btn-fullscreen').addEventListener('click', () => {
  if (document.fullscreenElement) {
    document.exitFullscreen();
  } else {
    document.documentElement.requestFullscreen();
  }
});
document.addEventListener('fullscreenchange', () => {
  document.body.classList.toggle('fullscreen', !!document.fullscreenElement);
});
```

CSS: `.fullscreen #header, .fullscreen #filter-panel, .fullscreen #status-bar { display: none; }`

### 16.8. Keyboard shortcuts

**File:** `frontend/src/shortcuts.js` (new file)

Export `initShortcuts(actions)`:

```js
const SHORTCUTS = {
  'r': () => actions.refresh(),
  'f': () => actions.fit(),
  '+': () => actions.zoomIn(),
  '=': () => actions.zoomIn(),
  '-': () => actions.zoomOut(),
  '/': () => actions.openSearch(),
  'l': () => actions.toggleLayout(),
  'e': () => actions.exportPNG(),
  'Escape': () => actions.closeAll(),
};

document.addEventListener('keydown', (e) => {
  if (['INPUT', 'SELECT', 'TEXTAREA'].includes(e.target.tagName)) return;
  const fn = SHORTCUTS[e.key];
  if (fn) { e.preventDefault(); fn(); }
});
```

Wire in `main.js` — pass action callbacks.

`Ctrl+K` / `Meta+K` → `openSearch()` (handled separately for modifier keys).

**Checklist:**
- [ ] 16.1 Alert Drawer button + badge
- [ ] 16.2 Alert Drawer HTML
- [ ] 16.3 Alert Drawer CSS
- [ ] 16.4 Alert Drawer JS logic (alerts.js)
- [ ] 16.5 Stats summary
- [ ] 16.6 Fullscreen button
- [ ] 16.7 Fullscreen JS logic
- [ ] 16.8 Keyboard shortcuts (shortcuts.js)

---

## Phase 17: Backend+Frontend — Pods API + Pod Display

**Goal:** Show pods belonging to a service node when "Show pods" button is clicked.

**Source:** interface.md §1

> **Design decision required:** How to get pod data.
> Options:
> a) **Kubernetes API** — query pods by label selector matching service name.
>    Requires K8s service account with pod list permissions.
> b) **Prometheus metric labels** — `instance` or `pod` labels from scrape targets.
>    Requires no extra permissions but depends on metric labeling.
> c) **dephealth SDK** — if SDK reports pod name as a label on metrics.
>
> **Recommendation:** Option (b) — use Prometheus `instance` label from scrape targets
> or `pod` label if available. Avoids K8s API dependency. Discuss with user before implementing.

### 17.1. Backend — pod data source

Design depends on decision above. Likely:

**File:** `internal/topology/prometheus.go`

New method `QueryPods(ctx, serviceName)` — query Prometheus for:
```promql
group by (pod, instance) (app_dependency_health{name="<serviceName>"})
```

Returns list of `{pod, instance}` for the service.

### 17.2. Backend — API endpoint or extend topology response

Option A: New endpoint `GET /api/v1/pods?service=<name>` — on-demand.
Option B: Include pods in `Node.Pods []PodInfo` in topology response.

Recommend Option A (on-demand) to keep main topology response lightweight.

### 17.3. Frontend — "Show pods" button

In floating toolbar: `<i class="bi bi-grid-3x3-gap"></i>`.
On click → toggle pods mode. When pods mode is active, clicking a service node
fetches pods and expands the node to show a table.

### 17.4. Frontend — Pod table rendering

Use Cytoscape compound nodes or an HTML overlay for pod table.
Compound node approach: add child nodes (one per pod) to the service node,
re-run layout.

Alternative: HTML overlay table positioned over the node.

### 17.5. Testing

- Test API endpoint returns correct pod list.
- Test frontend shows pods for selected node.

**Checklist:**
- [ ] 17.0 Design decision: pod data source
- [ ] 17.1 Backend: pod data query
- [ ] 17.2 Backend: API endpoint
- [ ] 17.3 Frontend: "Show pods" button
- [ ] 17.4 Frontend: pod table rendering
- [ ] 17.5 Testing

---

## Phase 18: Build + Deploy + Verify

**Goal:** Build new Docker image, update Helm chart, deploy, verify in browser.

### 18.1. Frontend build check

```bash
cd frontend && npm run build
```

Verify `dist/` output.

### 18.2. Go build check

```bash
go build ./cmd/dephealth-ui/
go test ./...
```

### 18.3. Docker build

```bash
make docker-build TAG=v1.0.0
```

Multi-arch (amd64 + arm64).

### 18.4. Helm chart updates

**File:** `deploy/helm/dephealth-ui/`

- Update `values.yaml` with new alerts config section (if not already templated).
- Update ConfigMap template if needed.
- Bump chart version.

### 18.5. Deploy

```bash
helm upgrade dephealth-ui deploy/helm/dephealth-ui/ \
  -n dephealth-ui -f deploy/helm/dephealth-ui/values.yaml
```

### 18.6. Browser verification

Open `https://dephealth.kryukov.lan` and verify:
- [ ] Floating toolbar visible, draggable
- [ ] Bootstrap Icons render in header and toolbar
- [ ] Zoom in/out/fit buttons work
- [ ] Alert badges show on nodes with correct colors
- [ ] Tooltip appears on hover (nodes and edges)
- [ ] Legend panel toggles
- [ ] Node sidebar opens on click
- [ ] Double-click opens Grafana
- [ ] Search works and highlights nodes
- [ ] Layout toggle TB ↔ LR works
- [ ] PNG export downloads file
- [ ] Alert drawer opens with grouped alerts
- [ ] Stats summary in status bar
- [ ] Fullscreen mode works
- [ ] Keyboard shortcuts work
- [ ] Dark/light themes apply to all new components
- [ ] Responsive layout on narrow screens

**Checklist:**
- [ ] 18.1 Frontend build
- [ ] 18.2 Go build + tests
- [ ] 18.3 Docker build
- [ ] 18.4 Helm chart updates
- [ ] 18.5 Deploy to K8s
- [ ] 18.6 Browser verification

---

## Summary

| Phase | Description | Complexity | New files |
|:-----:|-------------|:----------:|-----------|
| 11 | Backend: alerts severity config + models | Medium | — |
| 12 | Frontend: Bootstrap Icons + floating toolbar | Medium | `toolbar.js` |
| 13 | Frontend: alert badges + tooltip | Medium | `tooltip.js` |
| 14 | Frontend: legend + node detail sidebar | Medium | `sidebar.js` |
| 15 | Frontend: search + layout toggle + export | Medium | `search.js` |
| 16 | Frontend: alert drawer + stats + fullscreen + hotkeys | Medium | `alerts.js`, `shortcuts.js` |
| 17 | Backend+Frontend: pods API + pod display | High | TBD |
| 18 | Build + deploy + verify | Low | — |

**New frontend files:** `toolbar.js`, `tooltip.js`, `sidebar.js`, `search.js`, `alerts.js`, `shortcuts.js`

**Backend changes:** Phase 11 (config + models + graph builder) + Phase 17 (pods API)
