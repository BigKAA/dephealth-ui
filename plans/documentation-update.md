# Plan: Documentation Update — Cascade Warnings, State Model & RU Split

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-02-12
- **Last updated**: 2026-02-12
- **Status**: Complete

---

## Version History

- **v1.0.0** (2026-02-12): Initial plan

---

## Current Status

- **Active phase**: All complete
- **Active step**: Done
- **Last updated**: 2026-02-12
- **Note**: All 4 phases completed successfully

---

## Table of Contents

- [x] [Phase 1: Split bilingual documents into separate EN/RU files](#phase-1-split-bilingual-documents-into-separate-enru-files)
- [x] [Phase 2: Update content — cascade warnings & state model](#phase-2-update-content--cascade-warnings--state-model)
- [x] [Phase 3: Screenshots](#phase-3-screenshots)
- [x] [Phase 4: CHANGELOG, cross-references & final review](#phase-4-changelog-cross-references--final-review)

---

## Phase 1: Split bilingual documents into separate EN/RU files

**Dependencies**: None
**Status**: Pending

### Description

Currently all core docs use a single bilingual file pattern: `## English` → content → `## Русский` → translated content. This doubles the file size and makes navigation harder. We will split each bilingual doc into a base EN file and a parallel `.ru.md` file with Russian translation. The `docs/README.md` index will be updated to link to both language versions.

**Files to split (6 files → 12 files):**

| Current file | EN file | RU file | Lines (approx) |
|---|---|---|---|
| `docs/application-design.md` | `docs/application-design.md` | `docs/application-design.ru.md` | ~870 → ~440 + ~430 |
| `docs/API.md` | `docs/API.md` | `docs/API.ru.md` | ~825 → ~410 + ~415 |
| `docs/METRICS.md` | `docs/METRICS.md` | `docs/METRICS.ru.md` | ~618 → ~310 + ~308 |
| `docs/README.md` | `docs/README.md` | `docs/README.ru.md` | ~130 → ~65 + ~65 |
| `README.md` (root) | `README.md` | `README.ru.md` | ~535 → ~400 + ~135 |
| `deploy/helm/.../README.md` | keep as-is | `deploy/helm/.../README.ru.md` | ~410 → ~210 + ~200 |

### Steps

- [ ] **1.1 Split `docs/application-design.md`**
  - **Dependencies**: None
  - **Description**: Extract `## Русский` section into `docs/application-design.ru.md`. Keep EN content in original file. Remove bilingual header/nav, add language switch link at the top of each file: `**Language:** English | [Русский](./application-design.ru.md)` and vice versa.
  - **Creates**:
    - `docs/application-design.ru.md`
  - **Modifies**:
    - `docs/application-design.md`

- [ ] **1.2 Split `docs/API.md`**
  - **Dependencies**: None
  - **Description**: Same pattern as 1.1 — extract Russian section into `docs/API.ru.md`.
  - **Creates**:
    - `docs/API.ru.md`
  - **Modifies**:
    - `docs/API.md`

- [ ] **1.3 Split `docs/METRICS.md`**
  - **Dependencies**: None
  - **Description**: Same pattern — extract Russian section into `docs/METRICS.ru.md`.
  - **Creates**:
    - `docs/METRICS.ru.md`
  - **Modifies**:
    - `docs/METRICS.md`

- [ ] **1.4 Split `docs/README.md` (docs index)**
  - **Dependencies**: None
  - **Description**: Rewrite as EN-only index pointing to both EN and RU versions of each document. Create `docs/README.ru.md` as Russian index.
  - **Creates**:
    - `docs/README.ru.md`
  - **Modifies**:
    - `docs/README.md`

- [ ] **1.5 Split `README.md` (root)**
  - **Dependencies**: None
  - **Description**: Keep root README in English. Extract Russian section into `README.ru.md` at the root. Add language switch link at the top.
  - **Creates**:
    - `README.ru.md`
  - **Modifies**:
    - `README.md`

- [ ] **1.6 Split `deploy/helm/dephealth-ui/README.md`**
  - **Dependencies**: None
  - **Description**: Extract Russian section into `deploy/helm/dephealth-ui/README.ru.md`. Add language switch links.
  - **Creates**:
    - `deploy/helm/dephealth-ui/README.ru.md`
  - **Modifies**:
    - `deploy/helm/dephealth-ui/README.md`

- [ ] **1.7 Update cross-references**
  - **Dependencies**: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6
  - **Description**: Scan all docs for internal links and ensure:
    - EN docs link to EN docs
    - RU docs link to RU docs
    - Where a link target is EN-only (e.g. GIT-WORKFLOW.md), keep original link
    - Update `docs/README.md` and `docs/README.ru.md` tables
  - **Modifies**:
    - All newly created and modified files

### Completion Criteria Phase 1

- [ ] All 6 bilingual files split into 12 files (6 EN + 6 RU)
- [ ] Each file has a language switch link at the top
- [ ] All internal cross-references are correct for each language
- [ ] No content lost during split (diff check)
- [ ] Markdown renders correctly (headers, tables, code blocks)

---

## Phase 2: Update content — cascade warnings & state model

**Dependencies**: Phase 1
**Status**: Pending

### Description

Add documentation for the new cascade warnings feature and updated state model (v0.14.0). Update both EN and RU versions of affected documents.

### Steps

- [ ] **2.1 Update `docs/application-design.md` + `.ru.md` — State model section**
  - **Dependencies**: None
  - **Description**: Add/update "State Model" section describing the 4 states and their calculation:
    - **ok** — all dependencies healthy
    - **degraded** — some dependency health=0, service alive
    - **down** — stale/truly unavailable (all edges stale)
    - **unknown** — no edges/data
    - `calcServiceNodeState` never returns "down" — only ok/degraded/unknown
    - State determination rules for service nodes vs dependency nodes
  - **Modifies**:
    - `docs/application-design.md` (EN)
    - `docs/application-design.ru.md` (RU)

- [ ] **2.2 Update `docs/application-design.md` + `.ru.md` — Cascade warnings section**
  - **Dependencies**: None
  - **Description**: Add new "Cascade Warnings" section to Frontend Behavior:
    - Overview: failure propagation visualization
    - Algorithm: 2-phase (findRealRootCauses + BFS upstream)
    - Critical edges requirement (only critical=true edges propagate)
    - Node data properties: `cascadeCount`, `cascadeSources`, `inCascadeChain`
    - Execution flow in refresh loop
    - Visual representation: badge `⚠ N`, tooltip sources
    - Virtual "warning" state in filter system
    - Filter pass 1.5 (degraded/down chain visibility)
    - Interaction with namespace grouping
  - **Modifies**:
    - `docs/application-design.md` (EN)
    - `docs/application-design.ru.md` (RU)

- [ ] **2.3 Update `docs/METRICS.md` + `.ru.md` — Critical label significance**
  - **Dependencies**: None
  - **Description**: Enhance the `critical` label documentation:
    - Explain `critical=yes` role in cascade warnings propagation
    - Note that only critical edges trigger cascade warnings upstream
    - Add example: if A→B(critical) and B goes down → A gets cascade warning
    - Update state calculation rules section with cascade context
  - **Modifies**:
    - `docs/METRICS.md` (EN)
    - `docs/METRICS.ru.md` (RU)

- [ ] **2.4 Update `docs/API.md` + `.ru.md` — Topology response fields**
  - **Dependencies**: None
  - **Description**: Document new/updated fields in topology API response:
    - Node: `state` enum now includes context about cascade computation
    - Node: `stale` boolean field documentation
    - Edge: `critical` boolean significance for cascade warnings
    - Note: cascade computation is frontend-only, not in API response
  - **Modifies**:
    - `docs/API.md` (EN)
    - `docs/API.ru.md` (RU)

- [ ] **2.5 Update `README.md` + `.ru.md` — Features list**
  - **Dependencies**: None
  - **Description**: Add cascade warnings and state model to the Features section:
    - Cascade failure propagation visualization
    - 4-state model (ok, degraded, down, unknown)
    - Smart filtering with cascade chain visibility
  - **Modifies**:
    - `README.md` (EN)
    - `README.ru.md` (RU)

### Completion Criteria Phase 2

- [ ] State model documented in application-design (EN + RU)
- [ ] Cascade warnings documented in application-design (EN + RU)
- [ ] Critical label significance documented in METRICS (EN + RU)
- [ ] API response fields updated (EN + RU)
- [ ] Features list updated in README (EN + RU)
- [ ] All content is technically accurate (matches code implementation)

---

## Phase 3: Screenshots

**Dependencies**: Phase 2
**Status**: Pending

### Description

Capture screenshots of the cascade warnings feature in action. Use the deployed test environment (`https://dephealth.kryukov.lan`) with Playwright browser. Screenshots should clearly show cascade badges, tooltip with root causes, and filter behavior.

### Steps

- [ ] **3.1 Capture main view with cascade warnings**
  - **Dependencies**: None
  - **Description**: Open `https://dephealth.kryukov.lan`, wait for topology to load. If cascade badges are visible (some service is down), capture the main topology view showing `⚠ N` badges on affected nodes. Save as `docs/images/cascade-warnings-main.png`.
    - If no service is currently down, we may need to scale down a uniproxy instance first (ask user).
  - **Creates**:
    - `docs/images/cascade-warnings-main.png`

- [ ] **3.2 Capture tooltip with cascade sources**
  - **Dependencies**: 3.1
  - **Description**: Hover over a node with cascade warning to show tooltip with "Cascade warning: ↳ service-name (down)" section. Capture tooltip screenshot. Save as `docs/images/cascade-warning-tooltip.png`.
  - **Creates**:
    - `docs/images/cascade-warning-tooltip.png`

- [ ] **3.3 Capture state filter with warning state**
  - **Dependencies**: 3.1
  - **Description**: Open filter panel showing the "Warning" filter button among state filters (ok, degraded, down, unknown, warning). Capture filter toolbar area. Save as `docs/images/state-filters.png`.
  - **Creates**:
    - `docs/images/state-filters.png`

- [ ] **3.4 Update existing main view screenshot**
  - **Dependencies**: None
  - **Description**: Re-capture `docs/images/dephealth-main-view.png` with current UI (includes namespace grouping, badges, new state colors). This replaces the outdated screenshot.
  - **Modifies**:
    - `docs/images/dephealth-main-view.png`

- [ ] **3.5 Add screenshots to documentation**
  - **Dependencies**: 3.1, 3.2, 3.3, 3.4
  - **Description**: Insert screenshot references into `docs/application-design.md` and `.ru.md` in the cascade warnings and state model sections. Use relative paths: `![Cascade warnings](./images/cascade-warnings-main.png)`.
  - **Modifies**:
    - `docs/application-design.md` (EN)
    - `docs/application-design.ru.md` (RU)

### Completion Criteria Phase 3

- [ ] At least 3 new screenshots captured (cascade main, tooltip, filters)
- [ ] Main view screenshot updated
- [ ] All screenshots are reasonable size (< 300KB each)
- [ ] Screenshots referenced in documentation text

---

## Phase 4: CHANGELOG, cross-references & final review

**Dependencies**: Phase 1, Phase 2, Phase 3
**Status**: Pending

### Description

Update CHANGELOG.md with v0.14.0 section, verify all cross-references, and do a final consistency review.

### Steps

- [ ] **4.1 Add v0.14.0 section to CHANGELOG.md**
  - **Dependencies**: None
  - **Description**: Add `## [0.14.0] - 2026-02-XX` section (date TBD) with:
    - **Added**: Cascade warnings (BFS, root cause detection, badges, tooltip), 4-state model, virtual "warning" state filter, degraded/down chain filter visibility, cascade badge `⚠ N`
    - **Changed**: State model refined (calcServiceNodeState never returns "down"), filter system extended with pass 1.5
    - **Documentation**: Bilingual docs split into separate EN/RU files, cascade warnings documented, state model documented, new screenshots
  - **Modifies**:
    - `CHANGELOG.md`

- [ ] **4.2 Final cross-reference audit**
  - **Dependencies**: 4.1
  - **Description**: Grep all `.md` files for broken links (`](./`, `](../`), verify each link target exists. Fix any broken references.
  - **Modifies**:
    - Any files with broken links

- [ ] **4.3 Review consistency**
  - **Dependencies**: 4.2
  - **Description**: Quick review of EN ↔ RU content parity: ensure both languages cover the same sections and features. Check that technical terms are consistent.

### Completion Criteria Phase 4

- [ ] CHANGELOG.md has v0.14.0 section
- [ ] Zero broken internal links across all docs
- [ ] EN and RU documents cover the same sections
- [ ] All documentation changes are ready for commit

---

## Notes

- **Naming convention**: Russian files use `.ru.md` suffix (e.g. `API.ru.md`, `application-design.ru.md`)
- **Language switch pattern**: Each file starts with `**Language:** [English](./file.md) | [Русский](./file.ru.md)`
- **Screenshots**: Require access to `https://dephealth.kryukov.lan` — if cascade warnings are not visible (no down services), user may need to scale down a test service
- **Scope**: Only documentation changes, no code modifications
- **Cascade logic is frontend-only**: Backend sends raw state + edge data; cascade computation happens in `cascade.js`
