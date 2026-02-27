# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.18.0] - 2026-02-27

### Added

- **Explicit entry point marking** — services can be marked as entry points via `isentry=yes` label in dephealth SDK metrics (env: `DEPHEALTH_ISENTRY=yes` in uniproxy)
- **`isEntry` API field** — topology API response includes `isEntry: true` on entry point nodes
- **`isentry` metric label** — new optional label in PromQL `group by` clauses for topology discovery
- **LDAP dependency type** — `ldap` connection type supported in test environment (389 Directory Server)
- **389 Directory Server** — added 389ds (LDAP) to test infrastructure Helm chart (`dephealth-infra`)

### Changed

- **BREAKING: `isRoot` → `isEntry` rename** — API field `isRoot` renamed to `isEntry` in topology response nodes
- **BREAKING: Dependency node ID format** — dependency nodes now use `{source}/{dependency}` format (e.g., `order-service/postgres-main`) instead of `host:port`
- **Dependency node labels** — dependency nodes display logical dependency name (e.g., `postgres-main`, `ldap`) instead of hostname; `host:port` shown as secondary line in UI
- **CSS class rename** — `.root-badge` → `.entry-badge`, `.sidebar-root-badge` → `.sidebar-entry-badge`

### Removed

- **Automatic entry point detection** — nodes are no longer auto-detected as entry points based on absence of incoming edges; explicit `isentry` label is required
- **Dependency node deduplication** — dependency nodes are no longer deduplicated by `host:port`; each `(source, dependency)` pair creates a separate node

### Documentation

- Entry points section added to application design docs (EN + RU)
- Dependency node identification section added to application design docs (EN + RU)
- `isEntry` field documented in API reference node table (EN + RU)
- Dependency node ID format (`source/dependency`) documented in API reference (EN + RU)
- `isentry` label added to metrics specification (EN + RU)
- PromQL topology discovery queries updated with `isentry` in `group by` clauses (EN + RU)

## [0.17.2] - 2026-02-21

### Security

- **Fix PromQL injection** — sanitize user-supplied `namespace` and `group` parameters in Prometheus queries to prevent query manipulation
- **Fix XSS vulnerabilities** — add `escapeHtml()` to all `innerHTML` insertions with API data in sidebar, tooltip, timeline, and main modules (4 frontend files)
- **Non-root Docker image** — add dedicated user (UID 10001) to runtime container stage
- **Kubernetes pod hardening** — add `securityContext` (runAsNonRoot, readOnlyRootFilesystem, drop ALL capabilities) and dedicated `ServiceAccount` with `automountServiceAccountToken: false`

### Fixed

- Fix 19 errcheck lint violations across 7 Go packages (unchecked error returns)
- Fix event listener stacking bug in `main.js` — retry button handler was re-registered on each `init()` call

### Changed

- Increase default memory limit from 64Mi to 128Mi in Helm chart (adequate for Graphviz rendering)
- Add `.dockerignore` to reduce Docker build context size

## [0.17.1] - 2026-02-21

### Added

- **Optional AlertManager** — AlertManager is now an optional data source; when `datasources.alertmanager.url` is empty, alert-related UI elements are gracefully disabled
- **`alerts.enabled` config field** — `GET /api/v1/config` returns `alerts.enabled` boolean indicating whether AlertManager is configured
- **Disabled alerts UI** — when AlertManager is not configured: alerts button is visually disabled with tooltip, alert badges hidden on nodes/edges, alert sections hidden in sidebars, alert counters hidden in status bar
- **Grafana dashboard availability checking** — at startup, validates dashboard existence via Grafana API and hides links to unavailable dashboards

### Documentation

- Optional AlertManager behavior documented in application design docs (EN + RU)
- `alerts.enabled` field documented in API reference config endpoint (EN + RU)

## [0.17.0] - 2026-02-20

### Added

- **Graph export** — multi-format topology export via `GET /api/v1/export/{format}` endpoint
- **Export formats** — JSON (structured data with metadata), CSV (ZIP with nodes.csv + edges.csv), DOT (Graphviz format with clusters and colors), PNG (Graphviz-rendered raster), SVG (Graphviz-rendered vector)
- **Export modal** — frontend dialog with format selection (PNG/SVG/JSON/CSV/DOT), scope selection (current view / full graph), and download button
- **Frontend export** — "current view" PNG/SVG via Cytoscape.js `cy.png()` and `cy.svg()` preserving exact visual layout
- **Backend export** — "full graph" rendering via `internal/export` package with Graphviz integration
- **Export parameters** — scope filtering (`?scope=current&namespace=X`), historical export (`?time=`), PNG scale control (`?scale=1-4`)
- **Graphviz integration** — server-side DOT→PNG/SVG rendering via `dot` CLI (10s timeout, DPI-based scaling)
- **Export keyboard shortcut** — `E` key opens export modal (previously exported PNG directly)
- **cytoscape-svg** — added dependency for frontend SVG export support

### Changed

- Dockerfile runtime stage now includes Graphviz package (~55–65 MB addition to image size)
- Export button tooltip changed from "Export as PNG" to generic "Export"

### Documentation

- Export endpoint (`/api/v1/export/{format}`) documented in API reference (EN + RU)
- Graph export architecture section added to application design docs (EN + RU)
- Backend responsibilities table updated with export entry (EN + RU)
- Architecture diagram updated with export endpoint (EN + RU)
- Docker image size updated in deployment section (EN + RU)

## [0.16.1] - 2026-02-19

### Added

- **Root node detection** — detect and highlight entry point nodes (services with no incoming edges) in topology graph
- **Root node badge** — visual badge on root nodes and corresponding legend entry
- **Group label support** — SDK v0.5.0 `group` label in PromQL queries with `optFilter()` combining namespace+group
- **Dependency namespace resolution** — `resolveDepNamespace()` extracts Kubernetes namespace from FQDN dependency hosts
- **Dimension toggle** — group/namespace visual grouping switch in frontend toolbar
- **Test environment** — bare metal uniproxy host (`uniproxy-pr1`), group label config for uniproxy test instances

### Fixed

- Improve text contrast on colored backgrounds — WCAG-compliant luminance threshold (0.179), dynamic text colors on node labels, sidebar badges and status pills
- Fix node stripe colors and labels for dimension toggle
- Update dimension dropdown text on namespace/group toggle

### Documentation

- Group label and dimension toggle documentation (EN + RU)
- Test environment documentation

## [0.16.0] - 2026-02-17

### Added

- **History mode** — time-travel through topology graph to view dependency state at any historical point
- **Historical queries** — all Prometheus queries accept optional `?time=` parameter; uses `ALERTS` metric instead of AlertManager for historical alerts
- **Timeline events endpoint** — `GET /api/v1/timeline/events?start=&end=` detects `app_dependency_status` transitions via `query_range` with auto-calculated step
- **Timeline panel UI** — bottom panel with time range presets (1h–90d), custom datetime inputs, range slider with event markers
- **Event markers** — colored markers on timeline slider (red=degradation, green=recovery, orange=change) with click-to-snap
- **URL synchronization** — `?time=`, `?from=`, `?to=` query parameters maintained via `history.replaceState()` for shareable historical links
- **Grafana history links** — all Grafana dashboard URLs include `&from=<ts-1h>&to=<ts+1h>` in history mode (sidebar, context menu)
- **History mode visual indicator** — distinct header background and timestamp display in status bar
- **Error handling** — graceful fallbacks for timeline events API failures (toast notification), empty results ("no status changes" message), invalid URL params

### Changed

- Historical requests bypass in-memory cache entirely (no Get, no Set)
- `TopologyMeta` extended with `time` and `isHistory` fields
- `QueryOptions` extended with `Time *time.Time` for historical point-in-time queries
- `PrometheusClient` interface extended with `QueryStatusRange()` and `QueryHistoricalAlerts()` methods
- Auto-refresh pauses in history mode and resumes on "Live" button click

### Documentation

- API docs updated with `?time=` parameter on topology and cascade-analysis endpoints (EN + RU)
- New `/api/v1/timeline/events` endpoint documented with step auto-calculation table (EN + RU)
- History Mode architecture section added to application-design docs (EN + RU)
- `meta.time` and `meta.isHistory` fields documented in API reference (EN + RU)

## [0.14.1] - 2026-02-12

### Added

- **Cascade warnings** — failure propagation visualization through critical dependencies with BFS algorithm
- **Root cause detection** — automatic tracing downstream through critical edges to find the actual unavailable dependency
- **Cascade badge** — `⚠ N` pill-shaped badge on upstream nodes showing number of root cause sources
- **Cascade tooltip** — hover tooltip displaying root cause services with their states
- **Virtual "warning" filter state** — frontend-only filter for nodes receiving cascade warnings
- **Degraded/down chain filter** — pass 1.5 reveals downstream non-ok dependencies when degraded or down state filter is active
- **`inCascadeChain` flag** — marks down and root-cause nodes for filter system support

### Changed

- **State model refined** — `calcServiceNodeState` now returns only `ok`, `degraded`, or `unknown` (never `down`); down state set by stale detection only
- **Filter system extended** — 5 state filter buttons (ok, degraded, down, unknown, warning) with cascade chain visibility
- **Badge design improved** — alert badge `! N` pill-shape and cascade badge `⚠ N` with white border, cascade offset +22px from left
- **Node height increased** — taller nodes to accommodate badges and namespace display

### Documentation

- Bilingual docs split into separate EN/RU files (`.ru.md` pattern) with language switch links
- State model documented in application-design (EN + RU)
- Cascade warnings algorithm and visual representation documented (EN + RU)
- Critical label significance for cascade propagation documented in METRICS (EN + RU)
- API docs updated with accurate state calculation rules and cascade note (EN + RU)
- New screenshots: cascade warnings main view, tooltip, state filters
- CHANGELOG updated with v0.14.0 section

## [0.13.0] - 2026-02-11

### Added

- **Edge sidebar** — clickable edges with dependency details, state, latency, alerts, connected nodes, and Grafana links
- **Namespace grouping** — compound parent nodes grouping services by namespace with fcose layout engine
- **Collapse/expand** — double-tap on namespace group to collapse into summary node showing worst-state and alert count
- **Collapse/expand all** — toolbar buttons for batch collapse/expand operations
- **Click-to-expand navigation** — clicking a service in collapsed namespace sidebar expands the group, centers the node, and opens its sidebar
- **Aggregated edges** — collapsed namespace edges merged with `×N` count display
- **Edge navigation labels** — edge labels showing dependency type on the graph

### Fixed

- Highlight cleanup on node/edge deselection not fully removing styles
- Expand bug when `collapsedStore` data was lost during data-only graph updates (`reapplyCollapsedState` guard)

### Changed

- Dual layout engine: dagre (flat mode) ↔ fcose (grouped namespace mode), toggled via toolbar
- Sidebar now supports 3 types: node detail, edge detail, collapsed namespace summary
- Namespace-colored collapsed nodes with WCAG-compliant contrast text

### Documentation

- Complete rewrite of `docs/API.md` fixing 9 discrepancies with actual Go implementation
- Restructured `docs/application-design.md` into full English + full Russian sections
- Added complete Russian documentation to Helm chart README
- Added tree-view topology screenshots (EN/RU)

## [0.12.0] - 2026-02-11

### Added

- **Stale node retention** — services that stop sending metrics remain visible on the graph with `state="unknown"` for a configurable duration instead of vanishing
- `topology.lookback` configuration parameter (env: `DEPHEALTH_TOPOLOGY_LOOKBACK`) with validation (>=1m or 0 to disable)
- `last_over_time()` PromQL query for lookback-based topology structure
- Stale detection logic: edges present in lookback but absent from instant health query are marked `Stale=true`
- Frontend: gray dashed borders for stale nodes, gray dashed edges, hidden latency
- Frontend: "Metrics disappeared" / "Метрики пропали" in tooltips and sidebar for stale elements
- Frontend: `unknown` state filter button, stats counter shows unknown count
- Draggable Legend and Namespaces panels
- 6 unit tests for stale detection (all-current, service-disappears, partial-stale, connected-graph, lookback-disabled, all-stale)
- Helm chart: `config.topology.lookback` value with documentation

### Changed

- `EDGE_STYLES` now includes `unknown` entry (dashed gray line)
- `NewGraphBuilder` signature extended with `lookback` parameter
- `Node` and `Edge` models include `Stale bool` field (`json:"stale,omitempty"`)
- Helm chart version bumped to 0.6.0

## [0.11.4] - 2026-02-10

### Added

- Grafana integration with 5 dashboard types (service list, services status, links status, service status, link status)
- Context-aware Grafana links in sidebar (pre-fills variables based on selected node)
- Internationalization (i18n) with English and Russian translations (~120 keys each)
- Namespace display on nodes with colored left stripe (deterministic 16-color palette, djb2 hash)
- Right-click context menu on nodes/edges (Open in Grafana, Copy URL, Show Details)
- Ingress support with TLS options (existing secret or cert-manager auto-provisioning)
- Comprehensive bilingual documentation (API.md, METRICS.md, application-design.md)
- Dynamic node sizing based on label text length
- Alert drawer toggle behavior fix
- SERVICE filter fix and alert badge artifact cleanup

### Changed

- Helm chart bumped to 0.5.1
- Uniproxy extracted to separate repository

## [0.11.0] - 2026-02-09

### Added

- Instances API endpoint (`GET /api/v1/instances/:service`) with sidebar integration
- Alert drawer with severity grouping, stats counter in status bar
- Fullscreen mode and keyboard shortcuts (R, F, +/-, /, L, E, Escape, ?)
- Search panel with recursive downstream highlighting (`successors()`)
- Layout toggle (top-bottom / left-right) and PNG export
- Legend panel and node detail sidebar with connected edges
- Bootstrap Icons and floating graph toolbar
- Alert severity configuration, models and badge computation
- Namespace filtering for topology API (`?namespace=...`)
- Client-side filters: type, state, service (Tom Select autocomplete)
- Connected graph: service-to-service edges by matching dependency labels

### Changed

- SDK dependency migrated from local replace to GitHub `v0.3.0`
- Uniproxy rewritten to use dephealth SDK v0.2.x with env var config

## [0.10.0] - 2026-02-07

### Added

- OIDC authentication with PKCE (S256) — supports Dex, Keycloak
- Dark theme with CSS custom properties
- Responsive layout with touch-friendly targets
- Error handling improvements (partial data, reconnection)
- Performance optimizations (smart diff, ETag/304, gzip)
- In-memory TTL cache with ETag computation
- Basic auth middleware
- Helm chart with Gateway API (HTTPRoute) support
- Dex OIDC provider in dephealth-infra chart

## [0.2.0] - 2026-02-04

### Added

- Frontend SPA with Cytoscape.js + dagre layout (Phase 2)
- AlertManager integration for topology alert enrichment (Phase 3)

## [0.1.0] - 2026-02-03

### Added

- Initial Go project structure with Prometheus client and topology graph builder
- `GET /api/v1/topology` endpoint returning nodes, edges, alerts
- Multi-stage Docker build (Go + Vite + Alpine)
- Test environment with Helm charts (infra, monitoring, services)

[0.18.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.17.2...v0.18.0
[0.17.2]: https://github.com/BigKAA/dephealth-ui/compare/v0.17.1...v0.17.2
[0.17.1]: https://github.com/BigKAA/dephealth-ui/compare/v0.17.0...v0.17.1
[0.17.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.16.1...v0.17.0
[0.16.1]: https://github.com/BigKAA/dephealth-ui/compare/v0.16.0...v0.16.1
[0.16.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.14.1...v0.16.0
[0.14.1]: https://github.com/BigKAA/dephealth-ui/compare/v0.13.0...v0.14.1
[0.13.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.11.4...v0.12.0
[0.11.4]: https://github.com/BigKAA/dephealth-ui/compare/v0.11.0...v0.11.4
[0.11.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.2.0...v0.10.0
[0.2.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/BigKAA/dephealth-ui/releases/tag/v0.1.0
