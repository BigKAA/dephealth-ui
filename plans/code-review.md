# Plan: Code Review & Improvement of dephealth-ui

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-02-21
- **Last updated**: 2026-02-21
- **Status**: Pending

---

## Version History

- **v1.0.0** (2026-02-21): Initial plan — 9 phases, full codebase audit

---

## Current Status

- **Active phase**: Phase 9 (Final)
- **Active item**: 9.4
- **Last updated**: 2026-02-21
- **Note**: Phases 1-8 complete. All tests pass, Docker builds, Helm validates. Ready for merge.

---

## Table of Contents

- [x] [Phase 1: Setup + Foundational Go Packages](#phase-1-setup--foundational-go-packages)
- [x] [Phase 2: Security-Critical Go Packages](#phase-2-security-critical-go-packages)
- [x] [Phase 3: Core Business Logic — topology](#phase-3-core-business-logic--topology)
- [x] [Phase 4: Analysis & Output Go Packages](#phase-4-analysis--output-go-packages)
- [x] [Phase 5: HTTP Layer, Integrations & Entry Point](#phase-5-http-layer-integrations--entry-point)
- [x] [Phase 6: Frontend Core Modules](#phase-6-frontend-core-modules)
- [x] [Phase 7: Frontend Secondary Modules](#phase-7-frontend-secondary-modules)
- [x] [Phase 8: Infrastructure — Helm, Dockerfile, Makefile](#phase-8-infrastructure--helm-dockerfile-makefile)
- [x] [Phase 9: Final Validation & Summary](#phase-9-final-validation--summary)

---

## Execution Protocol

**Every phase follows this protocol:**

```
1. /clear                          — clear context
2. Read plans/code-review.md       — find current phase
3. git checkout refactor/code-review
4. /sc:analyze <target files>      — analyze component(s)
5. /sc:improve <target files>      — apply fixes (accept best options)
6. make test                       — run Go tests (must pass)
7. make lint                       — run linters (must pass)
8. git add + commit                — conventional commit format
9. Update this plan                — mark items completed, advance status
```

**Branch:** `refactor/code-review` (created in Phase 1.1)
**Commit format:** `refactor(<scope>): <description>`
**Merge policy:** NO merge to `master` until Phase 9 is complete

---

## Phase 1: Setup + Foundational Go Packages

**Dependencies**: None
**Status**: Done

### Description

Create the working branch and review foundational Go packages that other packages depend on:
`config` (338 LOC), `logging` (127 LOC), `cache` (86 LOC). These are low-risk,
well-tested packages that establish baseline patterns for the review.

### Items

- [x] **1.1 Create branch and verify baseline**
  - **Dependencies**: None
  - **Description**: Create `refactor/code-review` branch from `master`. Run `make test` and `make lint` to verify clean baseline. Record any pre-existing issues.
  - **Commands**:
    ```bash
    git checkout -b refactor/code-review
    make test
    make lint
    ```
  - **Creates**: Branch `refactor/code-review`

- [x] **1.2 Review and improve `internal/config`**
  - **Dependencies**: 1.1
  - **Description**: Analyze and improve `internal/config/` package (config.go — 338 LOC, config_test.go — 650 LOC). Focus: validation logic, error handling, YAML parsing security, environment variable handling.
  - **Commands**:
    ```
    /sc:analyze internal/config/
    /sc:improve internal/config/
    make test && make lint
    ```
  - **Files**:
    - `internal/config/config.go` (338 lines)
    - `internal/config/config_test.go` (650 lines)

- [x] **1.3 Review and improve `internal/logging`**
  - **Dependencies**: 1.1
  - **Description**: Analyze and improve `internal/logging/` package (logging.go — 96 LOC, middleware.go — 31 LOC + tests). Focus: slog configuration, middleware correctness, potential panics.
  - **Commands**:
    ```
    /sc:analyze internal/logging/
    /sc:improve internal/logging/
    make test && make lint
    ```
  - **Files**:
    - `internal/logging/logging.go` (96 lines)
    - `internal/logging/middleware.go` (31 lines)
    - `internal/logging/logging_test.go` (169 lines)
    - `internal/logging/middleware_test.go` (69 lines)

- [x] **1.4 Review and improve `internal/cache`**
  - **Dependencies**: 1.1
  - **Description**: Analyze and improve `internal/cache/` package (cache.go — 86 LOC, cache_test.go — 172 LOC). Focus: concurrency safety, TTL handling, ETag computation, memory management.
  - **Commands**:
    ```
    /sc:analyze internal/cache/
    /sc:improve internal/cache/
    make test && make lint
    ```
  - **Files**:
    - `internal/cache/cache.go` (86 lines)
    - `internal/cache/cache_test.go` (172 lines)

### Completion Criteria Phase 1

- [x] Branch `refactor/code-review` created
- [x] All items completed (1.1–1.4)
- [x] `make test` passes
- [x] `make lint` passes
- [ ] Changes committed: `refactor(config): ...`, `refactor(logging): ...`, `refactor(cache): ...`

---

## Phase 2: Security-Critical Go Packages

**Dependencies**: Phase 1
**Status**: Done

### Description

Review security-sensitive packages: `auth` (543 LOC, 767 LOC tests) and `alerts` (126 LOC).
These handle authentication (Basic, OIDC/PKCE, sessions) and external API communication.
Security focus is paramount.

### Items

- [x] **2.1 Review and improve `internal/auth`**
  - **Dependencies**: None
  - **Description**: Analyze and improve the auth package — 5 source files (auth.go, basic.go, none.go, oidc.go, session.go) and 3 test files. Focus: authentication bypass risks, session management, OIDC token handling, secret exposure, timing attacks, CSRF.
  - **Commands**:
    ```
    /sc:analyze internal/auth/
    /sc:improve internal/auth/
    make test && make lint
    ```
  - **Files**:
    - `internal/auth/auth.go` (58 lines) — Authenticator interface
    - `internal/auth/basic.go` (64 lines) — Basic auth
    - `internal/auth/none.go` (16 lines) — No-auth passthrough
    - `internal/auth/oidc.go` (288 lines) — OIDC/PKCE implementation
    - `internal/auth/session.go` (117 lines) — Session management
    - `internal/auth/basic_test.go` (198 lines)
    - `internal/auth/oidc_test.go` (471 lines)
    - `internal/auth/session_test.go` (98 lines)

- [x] **2.2 Review and improve `internal/alerts`**
  - **Dependencies**: None
  - **Description**: Analyze and improve AlertManager client (alertmanager.go — 126 LOC, test — 135 LOC). Focus: API response parsing, error handling, credential handling, HTTP client configuration.
  - **Commands**:
    ```
    /sc:analyze internal/alerts/
    /sc:improve internal/alerts/
    make test && make lint
    ```
  - **Files**:
    - `internal/alerts/alertmanager.go` (126 lines)
    - `internal/alerts/alertmanager_test.go` (135 lines)

### Completion Criteria Phase 2

- [ ] All items completed (2.1–2.2)
- [ ] No security vulnerabilities in auth flow
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Changes committed: `refactor(auth): ...`, `refactor(alerts): ...`

---

## Phase 3: Core Business Logic — topology

**Dependencies**: Phase 2
**Status**: Done

### Description

Review the largest and most critical package: `topology` (1,315 LOC source, 2,423 LOC tests).
Contains data models, Prometheus client, and graph builder. This is the core of the application.

### Items

- [x] **3.1 Review and improve `internal/topology/models.go`**
  - **Dependencies**: None
  - **Description**: Analyze topology data models (126 LOC). Focus: struct design, JSON serialization, type safety, documentation.
  - **Commands**:
    ```
    /sc:analyze internal/topology/models.go
    /sc:improve internal/topology/models.go
    make test && make lint
    ```
  - **Files**:
    - `internal/topology/models.go` (126 lines)

- [x] **3.2 Review and improve `internal/topology/prometheus.go`**
  - **Dependencies**: 3.1
  - **Description**: Analyze Prometheus client (523 LOC, 674 LOC tests). Focus: PromQL injection, HTTP client config, response parsing, error handling, connection pooling, timeout handling.
  - **Commands**:
    ```
    /sc:analyze internal/topology/prometheus.go internal/topology/prometheus_test.go
    /sc:improve internal/topology/prometheus.go internal/topology/prometheus_test.go
    make test && make lint
    ```
  - **Files**:
    - `internal/topology/prometheus.go` (523 lines)
    - `internal/topology/prometheus_test.go` (674 lines)

- [x] **3.3 Review and improve `internal/topology/graph.go`**
  - **Dependencies**: 3.1
  - **Description**: Analyze graph builder (666 LOC, 1,749 LOC tests). Focus: algorithm correctness, edge deduplication, connected graph resolution, namespace filtering, performance with large graphs.
  - **Commands**:
    ```
    /sc:analyze internal/topology/graph.go internal/topology/graph_test.go
    /sc:improve internal/topology/graph.go internal/topology/graph_test.go
    make test && make lint
    ```
  - **Files**:
    - `internal/topology/graph.go` (666 lines)
    - `internal/topology/graph_test.go` (1,749 lines)

### Completion Criteria Phase 3

- [ ] All items completed (3.1–3.3)
- [ ] No PromQL injection risks
- [ ] Graph algorithm correctness preserved
- [ ] `make test` passes (2,423 lines of topology tests)
- [ ] `make lint` passes
- [ ] Changes committed: `refactor(topology): ...`

---

## Phase 4: Analysis & Output Go Packages

**Dependencies**: Phase 3
**Status**: Done

### Description

Review analysis and output packages: `cascade` (574 LOC), `timeline` (170 LOC),
`export` (425 LOC, 5 source files). These consume topology data and produce derived outputs.

### Items

- [x] **4.1 Review and improve `internal/cascade`**
  - **Dependencies**: None
  - **Description**: Analyze BFS cascade analysis (cascade.go — 574 LOC, test — 623 LOC). Focus: BFS correctness, cycle detection, root cause accuracy, performance on complex graphs.
  - **Commands**:
    ```
    /sc:analyze internal/cascade/
    /sc:improve internal/cascade/
    make test && make lint
    ```
  - **Files**:
    - `internal/cascade/cascade.go` (574 lines)
    - `internal/cascade/cascade_test.go` (623 lines)

- [x] **4.2 Review and improve `internal/timeline`**
  - **Dependencies**: None
  - **Description**: Analyze timeline events (events.go — 170 LOC, test — 272 LOC). Focus: event ordering, memory usage, data structure efficiency.
  - **Commands**:
    ```
    /sc:analyze internal/timeline/
    /sc:improve internal/timeline/
    make test && make lint
    ```
  - **Files**:
    - `internal/timeline/events.go` (170 lines)
    - `internal/timeline/events_test.go` (272 lines)

- [x] **4.3 Review and improve `internal/export`**
  - **Dependencies**: None
  - **Description**: Analyze multi-format export (5 source files — 425 LOC total, 4 test files — 657 LOC). Focus: output correctness, Graphviz command injection, file handling, CSV escaping.
  - **Commands**:
    ```
    /sc:analyze internal/export/
    /sc:improve internal/export/
    make test && make lint
    ```
  - **Files**:
    - `internal/export/model.go` (122 lines)
    - `internal/export/csv.go` (96 lines)
    - `internal/export/dot.go` (141 lines)
    - `internal/export/json.go` (8 lines)
    - `internal/export/render.go` (58 lines)
    - `internal/export/csv_test.go` (172 lines)
    - `internal/export/dot_test.go` (157 lines)
    - `internal/export/json_test.go` (196 lines)
    - `internal/export/render_test.go` (132 lines)

### Completion Criteria Phase 4

- [ ] All items completed (4.1–4.3)
- [ ] No command injection in export/render
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Changes committed: `refactor(cascade): ...`, `refactor(timeline): ...`, `refactor(export): ...`

---

## Phase 5: HTTP Layer, Integrations & Entry Point

**Dependencies**: Phase 4
**Status**: Done

### Description

Review the HTTP server (984 LOC — largest package), Grafana integration (83 LOC),
and application entry point (151 LOC). These compose everything together.

### Items

- [x] **5.1 Review and improve `internal/server`**
  - **Dependencies**: None
  - **Description**: Analyze chi router, middleware, handlers (server.go — 679 LOC, static.go — 72 LOC, gzip.go — 53 LOC, export.go — 180 LOC + tests — 897 LOC). Focus: middleware ordering, CORS config, request validation, response handling, ETag logic, gzip compression, export endpoint security.
  - **Commands**:
    ```
    /sc:analyze internal/server/
    /sc:improve internal/server/
    make test && make lint
    ```
  - **Files**:
    - `internal/server/server.go` (679 lines)
    - `internal/server/static.go` (72 lines)
    - `internal/server/gzip.go` (53 lines)
    - `internal/server/export.go` (180 lines)
    - `internal/server/server_test.go` (568 lines)
    - `internal/server/gzip_test.go` (76 lines)
    - `internal/server/export_test.go` (253 lines)

- [x] **5.2 Review and improve `internal/grafana`**
  - **Dependencies**: None
  - **Description**: Analyze Grafana API checker (checker.go — 83 LOC, test — 156 LOC). Focus: API authentication handling, HTTP client reuse, error handling.
  - **Commands**:
    ```
    /sc:analyze internal/grafana/
    /sc:improve internal/grafana/
    make test && make lint
    ```
  - **Files**:
    - `internal/grafana/checker.go` (83 lines)
    - `internal/grafana/checker_test.go` (156 lines)

- [x] **5.3 Review and improve `cmd/dephealth-ui/main.go`**
  - **Dependencies**: 5.1, 5.2
  - **Description**: Analyze entry point (151 LOC). Focus: initialization order, graceful shutdown, signal handling, flag parsing, error propagation.
  - **Commands**:
    ```
    /sc:analyze cmd/dephealth-ui/main.go
    /sc:improve cmd/dephealth-ui/main.go
    make test && make lint
    ```
  - **Files**:
    - `cmd/dephealth-ui/main.go` (151 lines)

### Completion Criteria Phase 5

- [ ] All items completed (5.1–5.3)
- [ ] HTTP handler chain is secure
- [ ] Graceful shutdown works correctly
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Changes committed: `refactor(server): ...`, `refactor(grafana): ...`, `refactor(main): ...`

---

## Phase 6: Frontend Core Modules

**Dependencies**: Phase 5
**Status**: Done

### Description

Review the 5 largest and most critical frontend JS modules:
`sidebar.js` (978 LOC), `main.js` (875 LOC), `filter.js` (669 LOC),
`graph.js` (666 LOC), `api.js` (140 LOC). Total: ~3,328 LOC.
No automated tests exist — lint-only verification.

### Items

- [x] **6.1 Review and improve `frontend/src/graph.js`**
  - **Dependencies**: None
  - **Description**: Analyze Cytoscape graph initialization and styling (666 LOC). Focus: XSS in node labels, memory leaks in event handlers, performance with large graphs, status color mapping correctness.
  - **Commands**:
    ```
    /sc:analyze frontend/src/graph.js
    /sc:improve frontend/src/graph.js
    make lint
    ```
  - **Files**:
    - `frontend/src/graph.js` (666 lines)

- [x] **6.2 Review and improve `frontend/src/main.js`**
  - **Dependencies**: None
  - **Description**: Analyze main initialization (875 LOC). Focus: initialization order, event listener cleanup, memory management, global state handling, auto-refresh logic.
  - **Commands**:
    ```
    /sc:analyze frontend/src/main.js
    /sc:improve frontend/src/main.js
    make lint
    ```
  - **Files**:
    - `frontend/src/main.js` (875 lines)

- [x] **6.3 Review and improve `frontend/src/sidebar.js`**
  - **Dependencies**: None
  - **Description**: Analyze sidebar panel (978 LOC — largest JS file). Focus: DOM manipulation safety, innerHTML usage (XSS), event handler management, data rendering correctness.
  - **Commands**:
    ```
    /sc:analyze frontend/src/sidebar.js
    /sc:improve frontend/src/sidebar.js
    make lint
    ```
  - **Files**:
    - `frontend/src/sidebar.js` (978 lines)

- [x] **6.4 Review and improve `frontend/src/filter.js` and `frontend/src/api.js`**
  - **Dependencies**: None
  - **Description**: Analyze multi-pass filtering (669 LOC) and API client (140 LOC). Focus: filter logic correctness, query parameter handling, fetch error handling, response validation, XSS in filter values.
  - **Commands**:
    ```
    /sc:analyze frontend/src/filter.js frontend/src/api.js
    /sc:improve frontend/src/filter.js frontend/src/api.js
    make lint
    ```
  - **Files**:
    - `frontend/src/filter.js` (669 lines)
    - `frontend/src/api.js` (140 lines)

### Completion Criteria Phase 6

- [ ] All items completed (6.1–6.4)
- [ ] No XSS vulnerabilities (innerHTML, DOM injection)
- [ ] No memory leaks in event handlers
- [ ] `make lint` passes (no Go tests affected by frontend changes)
- [ ] Changes committed: `refactor(frontend): improve core modules`

---

## Phase 7: Frontend Secondary Modules

**Dependencies**: Phase 6
**Status**: Done

### Description

Review remaining 14 frontend JS modules (~2,966 LOC) and locale files (522 LOC).
Smaller, more focused modules.

### Items

- [x] **7.1 Review and improve UI interaction modules**
  - **Dependencies**: None
  - **Description**: Analyze interactive UI modules: `timeline.js` (563 LOC), `grouping.js` (462 LOC), `contextmenu.js` (253 LOC), `search.js` (253 LOC). Focus: DOM safety, event handling, state management.
  - **Commands**:
    ```
    /sc:analyze frontend/src/timeline.js frontend/src/grouping.js frontend/src/contextmenu.js frontend/src/search.js
    /sc:improve frontend/src/timeline.js frontend/src/grouping.js frontend/src/contextmenu.js frontend/src/search.js
    make lint
    ```
  - **Files**:
    - `frontend/src/timeline.js` (563 lines)
    - `frontend/src/grouping.js` (462 lines)
    - `frontend/src/contextmenu.js` (253 lines)
    - `frontend/src/search.js` (253 lines)

- [x] **7.2 Review and improve feature modules**
  - **Dependencies**: None
  - **Description**: Analyze feature modules: `alerts.js` (255 LOC), `export.js` (250 LOC), `tooltip.js` (226 LOC), `cascade.js` (154 LOC). Focus: API interaction, DOM rendering, error handling.
  - **Commands**:
    ```
    /sc:analyze frontend/src/alerts.js frontend/src/export.js frontend/src/tooltip.js frontend/src/cascade.js
    /sc:improve frontend/src/alerts.js frontend/src/export.js frontend/src/tooltip.js frontend/src/cascade.js
    make lint
    ```
  - **Files**:
    - `frontend/src/alerts.js` (255 lines)
    - `frontend/src/export.js` (250 lines)
    - `frontend/src/tooltip.js` (226 lines)
    - `frontend/src/cascade.js` (154 lines)

- [x] **7.3 Review and improve utility modules and locales**
  - **Dependencies**: None
  - **Description**: Analyze utilities: `draggable.js` (187 LOC), `namespace.js` (114 LOC), `i18n.js` (97 LOC), `shortcuts.js` (74 LOC), `toast.js` (66 LOC), `toolbar.js` (12 LOC). Also review locale files `en.js` and `ru.js` (261 LOC each) for completeness and consistency.
  - **Commands**:
    ```
    /sc:analyze frontend/src/draggable.js frontend/src/namespace.js frontend/src/i18n.js frontend/src/shortcuts.js frontend/src/toast.js frontend/src/toolbar.js frontend/src/locales/en.js frontend/src/locales/ru.js
    /sc:improve frontend/src/draggable.js frontend/src/namespace.js frontend/src/i18n.js frontend/src/shortcuts.js frontend/src/toast.js frontend/src/toolbar.js frontend/src/locales/en.js frontend/src/locales/ru.js
    make lint
    ```
  - **Files**:
    - `frontend/src/draggable.js` (187 lines)
    - `frontend/src/namespace.js` (114 lines)
    - `frontend/src/i18n.js` (97 lines)
    - `frontend/src/shortcuts.js` (74 lines)
    - `frontend/src/toast.js` (66 lines)
    - `frontend/src/toolbar.js` (12 lines)
    - `frontend/src/locales/en.js` (261 lines)
    - `frontend/src/locales/ru.js` (261 lines)

### Completion Criteria Phase 7

- [ ] All items completed (7.1–7.3)
- [ ] Locale files are consistent (same keys in en.js and ru.js)
- [ ] `make lint` passes
- [ ] Changes committed: `refactor(frontend): improve secondary modules`

---

## Phase 8: Infrastructure — Helm, Dockerfile, Makefile

**Dependencies**: Phase 7
**Status**: Done

### Description

Review infrastructure files: Helm chart templates (1,073 LOC),
Dockerfile (multi-stage), and Makefile. Focus on security, best practices, and correctness.

### Items

- [x] **8.1 Review and improve Helm chart**
  - **Dependencies**: None
  - **Description**: Analyze `deploy/helm/dephealth-ui/` — Chart.yaml, values.yaml, all templates. Focus: security contexts, resource limits, RBAC, secret handling, template correctness, values validation.
  - **Commands**:
    ```
    /sc:analyze deploy/helm/dephealth-ui/
    /sc:improve deploy/helm/dephealth-ui/
    helm lint deploy/helm/dephealth-ui/
    ```
  - **Files**:
    - `deploy/helm/dephealth-ui/Chart.yaml` (11 lines)
    - `deploy/helm/dephealth-ui/values.yaml` (117 lines)
    - `deploy/helm/dephealth-ui/templates/_helpers.tpl` (36 lines)
    - `deploy/helm/dephealth-ui/templates/namespace.yml` (6 lines)
    - `deploy/helm/dephealth-ui/templates/service.yml` (16 lines)
    - `deploy/helm/dephealth-ui/templates/deployment.yml` (89 lines)
    - `deploy/helm/dephealth-ui/templates/configmap.yml` (10 lines)
    - `deploy/helm/dephealth-ui/templates/ingress.yml` (51 lines)
    - `deploy/helm/dephealth-ui/templates/httproute.yml` (40 lines)

- [x] **8.2 Review and improve Dockerfile**
  - **Dependencies**: None
  - **Description**: Analyze multi-stage Dockerfile. Focus: base image pinning, layer caching, security (non-root user), build arg handling, final image size, Graphviz installation safety.
  - **Commands**:
    ```
    /sc:analyze Dockerfile
    /sc:improve Dockerfile
    ```
  - **Files**:
    - `Dockerfile`

- [x] **8.3 Review and improve Makefile**
  - **Dependencies**: None
  - **Description**: Analyze Makefile targets. Focus: variable quoting, command correctness, target dependencies, DRY principle, security of registry/host commands.
  - **Commands**:
    ```
    /sc:analyze Makefile
    /sc:improve Makefile
    ```
  - **Files**:
    - `Makefile`

### Completion Criteria Phase 8

- [ ] All items completed (8.1–8.3)
- [ ] `helm lint deploy/helm/dephealth-ui/` passes
- [ ] Dockerfile follows security best practices
- [ ] `make lint` passes
- [ ] Changes committed: `refactor(helm): ...`, `refactor(docker): ...`, `refactor(make): ...`

---

## Phase 9: Final Validation & Summary

**Dependencies**: Phase 8
**Status**: Done (pending merge)

### Description

Final comprehensive validation: run all tests, all linters, build Docker image,
verify Helm chart. Generate summary report of all changes made across phases 1–8.

### Items

- [x] **9.1 Full test and lint pass**
  - **Dependencies**: None
  - **Description**: Run complete test and lint suite to confirm nothing is broken.
  - **Commands**:
    ```bash
    make test          # All Go tests with -race
    make lint          # golangci-lint + markdownlint
    helm lint deploy/helm/dephealth-ui/
    ```
  - **Creates**: Test results confirmation

- [x] **9.2 Docker image build verification**
  - **Dependencies**: 9.1
  - **Description**: Build Docker image to verify Dockerfile + all code changes compile and bundle correctly.
  - **Commands**:
    ```bash
    docker build -t dephealth-ui:review-test .
    ```
  - **Creates**: Docker image `dephealth-ui:review-test`

- [x] **9.3 Generate review summary report**
  - **Dependencies**: 9.1, 9.2
  - **Description**: Create a summary of all changes, improvements, and findings across all phases. List categories: security fixes, performance improvements, code quality, architecture changes.
  - **Creates**:
    - Summary in commit message or separate report

- [ ] **9.4 Prepare for merge**
  - **Dependencies**: 9.3
  - **Description**: Squash or organize commits if needed. Update CHANGELOG.md. Ask user to choose merge method (local merge vs GitHub PR). Do NOT merge automatically.
  - **Creates**:
    - Updated `CHANGELOG.md`
    - Ready-to-merge branch

### Completion Criteria Phase 9

- [ ] All Go tests pass (`make test`)
- [ ] All linters pass (`make lint`)
- [ ] Helm chart validates (`helm lint`)
- [ ] Docker image builds successfully
- [ ] Summary report generated
- [ ] CHANGELOG.md updated
- [ ] User decides merge method

---

## Codebase Statistics

| Category | Files | Source LOC | Test LOC |
|----------|-------|-----------|----------|
| Go (internal/) | 22 source, 20 test | 4,497 | 5,929 |
| Go (cmd/) | 1 | 151 | — |
| Frontend (JS) | 19 + 2 locales | 6,816 | — |
| Helm | 9 templates + configs | 1,073 | — |
| Infra | 2 (Dockerfile, Makefile) | ~200 | — |
| **Total** | **75 files** | **~12,737** | **~5,929** |

## Notes

- Frontend has **no automated tests** — only lint verification is possible for JS changes
- Go test coverage ratio is excellent (1.02x test-to-source)
- The `topology` package is the largest (1,315 LOC) and most critical — Phase 3 is dedicated to it
- Security-critical code (`auth`, `server`) gets focused review in Phases 2 and 5
- All optimizations should be accepted (choose best option if multiple alternatives exist)
- Each phase should use `/sc:analyze` followed by `/sc:improve` as the primary workflow

---

**Plan ready for execution. Start with Phase 1.**
