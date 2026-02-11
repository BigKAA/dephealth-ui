# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.12.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.11.4...v0.12.0
[0.11.4]: https://github.com/BigKAA/dephealth-ui/compare/v0.11.0...v0.11.4
[0.11.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.2.0...v0.10.0
[0.2.0]: https://github.com/BigKAA/dephealth-ui/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/BigKAA/dephealth-ui/releases/tag/v0.1.0
