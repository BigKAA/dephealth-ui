# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**dephealth-ui** — a microservices health and topology visualization tool. Displays a node-graph diagram showing service states (OK, DEGRADED, DOWN), connection latency, and links to Grafana dashboards.

Data sources:
- Prometheus/VictoriaMetrics (via [topologymetrics](https://github.com/BigKAA/topologymetrics))
- AlertManager

Project status: Phase 0 complete — project structure initialized, test environment deployed.

## Communication & Language

- Communicate with the user in **Russian**
- All code comments, documentation, and commit messages in **English**
- Lint all programming language files and markdown files with appropriate linters

## Development Environment

All development, debugging, and testing must use **Docker containers or Kubernetes**.

Available tools: `kubectl`, `helm`, `docker`

### Kubernetes Test Cluster
- Gateway API installed (no Ingress controller)
- MetalLB enabled (LoadBalancer services get auto-assigned IPs, no auto DNS)
- cert-manager with `ClusterIssuer: dev-ca-issuer`
- Test domains: `test1.kryukov.lan`, `test2.kryukov.lan`, `test3.kryukov.lan`, `test4.kryukov.lan` → DNS 192.168.218.9, Gateway IP: 192.168.218.180
- Domain names used in development must be added to hosts file (ask user to do it manually)

### Container Registries

**Release registry (Yandex Container Registry):**
- `container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst` — release images
- Authentication via `yc` credential helper (configured in `~/.docker/config.json`)
- Use for all documentation examples and Helm chart defaults

**Development registry (Harbor):**
- `harbor.kryukov.lan/library` — dev/test images (homelab only)
- `harbor.kryukov.lan/docker` — Docker Hub proxy
- `harbor.kryukov.lan/mcr` — Microsoft Container Registry proxy
- Admin: `admin` / `password`

## Git Workflow

Follow **GitHub Flow + Semantic Versioning** (see [GIT-WORKFLOW.md](GIT-WORKFLOW.md)):

- Main branch: `master` (always deployable)
- Branch prefixes: `feature/`, `bugfix/`, `docs/`, `refactor/`, `test/`, `hotfix/`
- Commit format: **Conventional Commits** — `<type>(<scope>): <subject>`
- Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`
- After finishing work, ask user before committing
- After commit, ask user to choose merge method (local merge vs GitHub PR)
- Delete branches after merge
- Releases via git tags `vX.Y.Z` on `master`
- Quick fixes (typos, small fixes) can be committed directly to `master`

### Release checklist

Before creating a release tag, **always** perform these steps:

1. Update `CHANGELOG.md` — add a new section following [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format
2. Update project documentation in `docs/` if any user-facing behavior changed
3. Bump `appVersion` in `deploy/helm/dephealth-ui/Chart.yaml`
4. Commit all release preparation changes before tagging
5. Build and push release image: `make docker-release TAG=vX.Y.Z`
6. Create GitHub release with `gh release create` — include a **Docker Image** section with pull command:
   `docker pull container-registry.cloud.yandex.net/crpklna5l8v5m7c0ipst/dephealth-ui:vX.Y.Z`
   and supported platforms: `linux/amd64`, `linux/arm64`

### Image tagging convention

- **Development:** `vX.Y.Z-N` (e.g. `v0.11.4-1`, `v0.11.4-2`) — increment `-N` suffix for each build, push to Harbor
- **Release:** `vX.Y.Z` (e.g. `v0.11.5`) — drop the suffix, bump patch version, push to Yandex CR
- **Minor version** (second digit) — only bump with explicit user approval

## Project Structure

```
cmd/dephealth-ui/       — application entry point
internal/               — Go packages (config, server, topology, alerts, auth, cache)
frontend/               — Vite + Cytoscape.js SPA (Phase 2)
deploy/helm/
  dephealth-ui/         — application Helm chart (Phase 4)
  dephealth-infra/      — test infrastructure (PostgreSQL, Redis, stubs)
  dephealth-services/   — test microservices (go, python, java, csharp)
  dephealth-monitoring/ — monitoring stack (VictoriaMetrics, AlertManager, Grafana)
plans/                  — development and testing plans (phased, detailed)
docs/                   — detailed project documentation
```

## Test Environment

Deploy/manage test environment with local Helm charts:

```bash
make env-deploy    # deploy infra + services + monitoring
make env-undeploy  # remove all test components
make env-status    # check pod status
```

## Plans

- **All development plans must use the template from [`.templates/DEVELOPMENT_PLAN_TEMPLATE.md`](.templates/DEVELOPMENT_PLAN_TEMPLATE.md)**
- Plans must be detailed and broken into phases
- Each phase should fit within a single AI context window
- Mark completed phases in the plan file
- When all items in a plan are marked as completed, move the plan file to `plans/archive/`
