# Code Review Plan — Requirements Specification

## Goal

Systematic code review of the entire dephealth-ui codebase using `/sc:analyze` (detect issues) + `/sc:improve` (apply fixes) approach. Auto-accept optimizations, run tests after each change.

## Scope

**Full codebase audit** covering all dimensions:
- Code quality, security, performance, architecture

### Components to Review

| # | Component | Type | Files | Tests | Priority |
|---|-----------|------|-------|-------|----------|
| 1 | `internal/config` | Go | config.go, validation | config_test.go | High |
| 2 | `internal/logging` | Go | logger, middleware | logging_test.go | Medium |
| 3 | `internal/cache` | Go | cache, ETag | cache_test.go | Medium |
| 4 | `internal/auth` | Go | authenticator, basic, oidc | auth_test.go | **Critical** |
| 5 | `internal/alerts` | Go | alertmanager client | alerts_test.go | High |
| 6 | `internal/topology` | Go | models, prometheus, graphbuilder | topology_test.go | **Critical** |
| 7 | `internal/cascade` | Go | BFS analysis | cascade_test.go | High |
| 8 | `internal/timeline` | Go | timeline data | timeline_test.go | Medium |
| 9 | `internal/export` | Go | JSON/CSV/DOT/PNG/SVG | export_test.go | Medium |
| 10 | `internal/server` | Go | chi router, middleware, handlers | server_test.go | **Critical** |
| 11 | `internal/grafana` | Go | API checker | grafana_test.go | Medium |
| 12 | `cmd/dephealth-ui` | Go | main.go entry point | — | High |
| 13 | `frontend/src/` core | JS | main, graph, api, filter, sidebar | — | **Critical** |
| 14 | `frontend/src/` secondary | JS | 14 remaining modules + locales | — | Medium |
| 15 | `deploy/helm/dephealth-ui` | Helm | Chart, values, templates | — | High |
| 16 | `Dockerfile` + `Makefile` | Infra | build/deploy config | — | Medium |

### Review Dimensions per Component

- **Quality**: code style, naming, error handling, duplication, readability
- **Security**: injection, auth bypass, secrets exposure, input validation (OWASP Top 10)
- **Performance**: memory leaks, goroutine leaks, N+1 queries, unnecessary allocations
- **Architecture**: coupling, cohesion, interface design, dependency direction

## Workflow Requirements

### Git Workflow
- Create branch `refactor/code-review` from `master`
- All changes committed to this branch only
- **No merge** to `master` until all phases complete
- Conventional Commits format: `refactor(<scope>): <description>`

### Per-Phase Execution
1. Start new conversation (clear context)
2. Read plan, identify current phase
3. Run `/sc:analyze` on target component(s)
4. Run `/sc:improve` to apply fixes
5. Accept all optimization suggestions (choose best option if multiple)
6. Run `make test` after each change
7. Run `make lint` to verify code style
8. Commit changes with descriptive message
9. Update plan status (mark completed items)

### Test Requirements
- Go tests: `make test` (must pass after each phase)
- Go linting: `make lint` (must pass after each phase)
- Frontend: no automated tests currently (lint only)
- Helm: `helm lint` for chart validation

## Proposed Plan Structure (9 Phases)

| Phase | Components | Estimated Scope |
|-------|-----------|-----------------|
| **1** | Setup branch + `config`, `logging`, `cache` | 3 foundational Go packages |
| **2** | `auth` + `alerts` | 2 security-sensitive Go packages |
| **3** | `topology` | Largest Go package (models, prometheus, graph) |
| **4** | `cascade` + `timeline` + `export` | 3 analysis/output Go packages |
| **5** | `server` + `grafana` + `cmd/` | HTTP layer + integrations + entry point |
| **6** | Frontend core | main.js, graph.js, api.js, filter.js, sidebar.js |
| **7** | Frontend secondary | 14 remaining JS modules + locales |
| **8** | Helm + Dockerfile + Makefile | Infrastructure as code |
| **9** | Final validation | Full test/lint, summary report, merge readiness |

## Acceptance Criteria

- [ ] All Go tests pass (`make test`)
- [ ] All linters pass (`make lint`)
- [ ] Helm chart validates (`helm lint`)
- [ ] No security vulnerabilities introduced
- [ ] Docker image builds successfully
- [ ] All changes are on `refactor/code-review` branch
- [ ] Each phase has a commit with clear message
- [ ] Plan file updated with completion status

## Open Questions

None — all clarified during brainstorm session.

## Next Step

Create the detailed execution plan using the project template at `.templates/DEVELOPMENT_PLAN_TEMPLATE.md` → save to `plans/code-review.md`.
