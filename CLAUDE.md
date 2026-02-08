# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**dephealth-ui** — a microservices health and topology visualization tool. Displays a node-graph diagram showing service states (OK, DEGRADED, DOWN), connection latency, and links to Grafana dashboards.

Data sources:
- Prometheus/VictoriaMetrics (via [topologymetrics](https://github.com/BigKAA/topologymetrics))
- AlertManager

Project status: early planning/research phase — no source code yet.

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
- Test domains: `test1.kryukov.lan`, `test2.kryukov.lan` → DNS 192.168.218.9, Gateway IP: 192.168.218.180
- Domain names used in development must be added to hosts file (ask user to do it manually)

### Container Registry (Harbor)
- `harbor.kryukov.lan/library` — local public images
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

## Project Structure (expected)

```
src/          — source code
plans/        — development and testing plans (phased, detailed)
docs/         — detailed project documentation
```

## Plans

- Plans must be detailed and broken into phases
- Each phase should fit within a single AI context window
- Mark completed phases in the plan file
