# Plan: Timeline Ticks & Labels

## Metadata

- **Plan version**: 1.0.0
- **Created**: 2026-03-07
- **Last updated**: 2026-03-07
- **Status**: In Progress

---

## Version History

- **v1.0.0** (2026-03-07): Initial plan

---

## Current Status

- **Active phase**: Phase 2
- **Active item**: 2.1
- **Last updated**: 2026-03-07
- **Note**: Phase 1 completed — chooseTicks, generateTicks, anti-overlap logic implemented

---

## Table of Contents

- [x] [Phase 1: Tick Calculation Engine](#phase-1-tick-calculation-engine)
- [ ] [Phase 2: DOM Rendering & Styles](#phase-2-dom-rendering--styles)
- [ ] [Phase 3: Build, Deploy & Test](#phase-3-build-deploy--test)

---

## Phase 1: Tick Calculation Engine

**Dependencies**: None
**Status**: Completed

### Description

Implement the core logic for computing tick positions and time labels based on the current
time range. This is pure logic with no DOM manipulation — easy to reason about and test in
isolation.

### Items

- [x] **1.1 Tick interval selection function**
  - **Dependencies**: None
  - **Description**: Create a function `chooseTicks(rangeMs)` that returns
    `{ majorStep, minorStep, format }` based on the total range duration.
    Interval table:

    | Range        | Major step | Minor step | Label format    |
    |-------------|-----------|-----------|-----------------|
    | <= 1h        | 10 min     | 2 min      | `HH:mm`         |
    | <= 6h        | 1h         | 15 min     | `HH:mm`         |
    | <= 12h       | 2h         | 30 min     | `HH:mm`         |
    | <= 1d        | 4h         | 1h         | `HH:mm`         |
    | <= 7d        | 1d         | 6h         | `dd.MM HH:mm`   |
    | <= 30d       | 7d         | 1d         | `dd.MM`         |
    | <= 90d       | 14d        | 7d         | `dd.MM`         |

  - **Modifies**: `frontend/src/timeline.js`
  - **Links**: N/A

- [x] **1.2 Tick generation function**
  - **Dependencies**: 1.1
  - **Description**: Create a function `generateTicks(rangeStart, rangeEnd, containerWidth)`
    that returns an array of tick objects:
    ```js
    { time: Date, ratio: number, type: 'major' | 'minor', label?: string }
    ```
    Logic:
    1. Call `chooseTicks()` to get step sizes and format.
    2. Snap `rangeStart` up to the nearest major boundary for the first major tick.
       Snap `rangeEnd` down to the nearest major boundary for the last major tick.
       These become the "mandatory" start/end labels (nearest "pretty" time).
    3. Fill in major ticks at `majorStep` intervals between them.
    4. Fill in minor ticks at `minorStep` intervals (skip positions where a major tick exists).
    5. For each major tick, compute `label` using the format string.
    6. Anti-overlap: estimate label width (~70px for `HH:mm`, ~110px for `dd.MM HH:mm`),
       walk left-to-right and suppress labels that would overlap the previous visible label.
       The first and last major ticks are never suppressed.
  - **Modifies**: `frontend/src/timeline.js`
  - **Links**: N/A

### Completion Criteria — Phase 1

- [x] All items completed (1.1, 1.2)
- [x] `chooseTicks()` covers all 7 range tiers
- [x] `generateTicks()` produces correct tick arrays for representative ranges (1h, 6h, 1d, 7d, 30d, 90d)
- [x] Anti-overlap logic prevents label collisions

---

## Phase 2: DOM Rendering & Styles

**Dependencies**: Phase 1
**Status**: Pending

### Description

Render ticks and labels into the timeline DOM, style them for both themes, and integrate
with existing slider interactions (range change, zoom, preset switches).

### Items

- [ ] **2.1 Tick container DOM structure**
  - **Dependencies**: None
  - **Description**: In `buildUI()`, add a new `div.timeline-ticks` container inside
    `timeline-slider-container`, positioned **below** the track and **behind** the markers layer.
    DOM order inside `timeline-slider-container`:
    ```
    div.timeline-ticks        ← NEW (ticks + labels, background layer)
    div.timeline-track        ← existing
    div.timeline-markers      ← existing (events, on top of ticks)
    div.timeline-thumb        ← existing
    div.timeline-tooltip      ← existing
    ```
    Increase `timeline-slider-container` height from 24px to ~48px.
    Adjust `timeline-track` vertical position to stay near the top (~6px from top).
    Adjust `timeline-thumb` top position accordingly.
  - **Modifies**:
    - `frontend/src/timeline.js` — `buildUI()` HTML template
    - `frontend/src/style.css` — container height, track/thumb positions

- [ ] **2.2 Render ticks function**
  - **Dependencies**: 1.2, 2.1
  - **Description**: Create `renderTicks()` that:
    1. Calls `generateTicks(rangeStart, rangeEnd, containerWidth)`.
    2. Builds tick DOM elements inside `div.timeline-ticks`:
       - Major tick: `div.timeline-tick.major` — vertical line (~10px tall) + label `span.timeline-tick-label`.
       - Minor tick: `div.timeline-tick.minor` — shorter vertical line (~5px tall), no label.
    3. Each tick positioned via `style="left: {ratio*100}%"`.
    Call `renderTicks()` from `setRange()` and on window resize (debounced).
  - **Modifies**: `frontend/src/timeline.js`
  - **Links**: N/A

- [ ] **2.3 CSS styles for ticks and labels**
  - **Dependencies**: 2.1
  - **Description**: Add styles for tick elements:
    - `.timeline-ticks` — absolute positioning, full width, below track.
    - `.timeline-tick` — absolute, width 1px, background `var(--border-color)`.
    - `.timeline-tick.major` — height 10px.
    - `.timeline-tick.minor` — height 5px.
    - `.timeline-tick-label` — `font-size: 10px`, `color: var(--text-muted)`,
      `white-space: nowrap`, positioned below the tick line, `transform: translateX(-50%)`.
    - Dark theme overrides:
      - `html[data-theme="dark"] .timeline-tick` — lighter border color for visibility.
      - `html[data-theme="dark"] .timeline-tick-label` — adjusted muted text color.
  - **Modifies**: `frontend/src/style.css`
  - **Links**: N/A

- [ ] **2.4 Integration with existing interactions**
  - **Dependencies**: 2.2
  - **Description**: Ensure ticks re-render when the range changes:
    - Preset button click → `setRange()` → `renderTicks()` (already covered in 2.2).
    - Custom range Apply → same path.
    - Drag-zoom (range select on track) → `setRange()` → `renderTicks()`.
    - Window resize → debounced `renderTicks()` call (add `ResizeObserver` on container).
    - `restoreFromURL()` → call `renderTicks()` after range is set.
  - **Modifies**: `frontend/src/timeline.js`
  - **Links**: N/A

### Completion Criteria — Phase 2

- [ ] All items completed (2.1–2.4)
- [ ] Ticks render correctly for all 7 preset ranges
- [ ] Labels do not overlap at any reasonable window width (>=768px)
- [ ] Ticks update on range change, drag-zoom, and window resize
- [ ] Both light and dark themes display correctly
- [ ] Existing slider interactions (click, drag, marker hover) work as before

---

## Phase 3: Build, Deploy & Test

**Dependencies**: Phase 2
**Status**: Pending

### Description

Build the Docker image, deploy to the test Kubernetes cluster, and verify the timeline
visually across different ranges and themes.

### Items

- [ ] **3.1 Build and push dev image**
  - **Dependencies**: None
  - **Description**: Build a development Docker image and push to Harbor.
    Use the next dev tag in the current version sequence.
    ```bash
    make docker-dev TAG=vX.Y.Z-N
    ```
  - **Creates**: Docker image in Harbor
  - **Links**: N/A

- [ ] **3.2 Deploy to test cluster**
  - **Dependencies**: 3.1
  - **Description**: Update the Helm values with the new image tag and deploy:
    ```bash
    make deploy
    ```
  - **Links**: N/A

- [ ] **3.3 Visual verification**
  - **Dependencies**: 3.2
  - **Description**: Verify in the browser:
    - [ ] All 7 presets show appropriate tick intervals and labels
    - [ ] Start/end labels snap to nearest "pretty" time
    - [ ] Labels don't overlap at various window widths
    - [ ] Drag-zoom recalculates ticks correctly
    - [ ] Light theme: ticks and labels visible, not too prominent
    - [ ] Dark theme: ticks and labels visible, proper contrast
    - [ ] Existing features unaffected: thumb drag, marker hover, tooltip, copy URL
  - **Links**: N/A

### Completion Criteria — Phase 3

- [ ] All items completed (3.1–3.3)
- [ ] Docker image built and pushed successfully
- [ ] Deployed to test cluster without errors
- [ ] All visual checks pass in both themes
- [ ] No regressions in existing timeline functionality

---

## Notes

- Anti-overlap algorithm: walk labels left-to-right, track the right edge of the last
  visible label (`lastRightPx`). Skip any label whose left edge < `lastRightPx + gap`.
  First and last major ticks are always shown — if a middle label overlaps with the last
  mandatory label, suppress the middle one.
- Label width estimation: use a conservative fixed width per format rather than measuring
  DOM, to avoid layout thrashing. Can refine later if needed.
- `ResizeObserver` is preferred over `window.resize` for container width tracking — more
  precise and handles sidebar toggle.
