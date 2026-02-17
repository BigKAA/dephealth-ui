# Plan: Timeline Modifications

## Metadata

- **Version**: 1.0.0
- **Created**: 2026-02-17
- **Last updated**: 2026-02-17
- **Status**: Pending

---

## Version History

- **v1.0.0** (2026-02-17): Initial plan

---

## Current Status

- **Active phase**: Phase 4
- **Active subtask**: 4.1
- **Last updated**: 2026-02-17
- **Note**: Phase 3 complete — drag range selection with overlay and zoom-in

---

## Table of Contents

- [x] [Phase 1: Custom Slider](#phase-1-custom-slider)
- [x] [Phase 2: Tooltip and Marker Hover](#phase-2-tooltip-and-marker-hover)
- [x] [Phase 3: Range Selection Drag](#phase-3-range-selection-drag)
- [ ] [Phase 4: Copy URL Button and i18n](#phase-4-copy-url-button-and-i18n)
- [ ] [Phase 5: Build and Test](#phase-5-build-and-test)

---

## Phase 1: Custom Slider

**Dependencies**: None
**Status**: Pending

### Description

Replace native `<input type="range">` with a custom div-based slider. This is the foundation
for all other features (marker hover, range drag, tooltip). The custom slider must fully
replicate the current behavior: thumb drag to select a time point, visual positioning, and
integration with existing preset/custom range controls.

### Subtasks

- [ ] **1.1 Replace slider HTML and add base CSS**
  - **Dependencies**: None
  - **Description**: In `buildUI()`, replace the `<input type="range">` with the new HTML
    structure: `.timeline-track` (with `.timeline-track-fill` and `.timeline-range-overlay`
    inside), `.timeline-thumb`, and `.timeline-tooltip`. In `style.css`, remove native slider
    styles (`.timeline-slider`, `::-webkit-slider-thumb`, `::-moz-range-thumb`) and add new
    styles for custom track, thumb, fill, and container. The slider container height stays 24px.
    Track: 6px, centered vertically, full width, rounded, `var(--border-color)` background.
    Thumb: 16px circle, absolutely positioned, same styling as current thumb. Track fill:
    same height as track, accent color, width controlled via JS. Tooltip and overlay: hidden
    by default (styled in Phase 2 and 3).
  - **Modifies**:
    - `frontend/src/timeline.js` (lines 181-199: `buildUI()` HTML template)
    - `frontend/src/style.css` (lines 1960-1997: slider styles)

- [ ] **1.2 Implement custom thumb drag logic**
  - **Dependencies**: 1.1
  - **Description**: Add interaction state enum (`IDLE`, `THUMB_DRAG`, `RANGE_SELECT`,
    `MARKER_HOVER`). Add new module-level variables: `interactionState`, `trackEl`,
    `thumbEl`, `trackFillEl`, DOM refs set in `buildUI()`. Implement `mousedown` on thumb
    element to start `THUMB_DRAG` state. On `mousemove` (document): calculate X position
    relative to track via `trackEl.getBoundingClientRect()`, compute ratio (clamped 0..1),
    update thumb `left` and trackFill `width` as percentages, compute `selectedTime` from
    ratio, call `updateTimeDisplay()`. On `mouseup` (document): reset state to `IDLE`,
    call `syncToURL()` and `onTimeChangedCb(selectedTime)`, remove document listeners.
    Prevent default on thumb mousedown to avoid text selection.
  - **Modifies**:
    - `frontend/src/timeline.js`

- [ ] **1.3 Update existing functions for custom slider**
  - **Dependencies**: 1.2
  - **Description**: Update all functions that previously used `sliderEl.value` /
    `sliderEl.max` to work with the custom slider. Affected functions:
    - `setRange()`: instead of `sliderEl.value = sliderEl.max`, set thumb to 100% position
    - `restoreFromURL()`: calculate ratio from time position, set thumb and fill via CSS
    - `renderMarkers()` click handler: calculate ratio from marker timestamp, set thumb/fill
    - `updateTimeFromSlider()`: refactor to accept ratio parameter or compute from thumb
      position
    Remove the old `sliderEl` variable, replace with `thumbEl`/`trackEl` references.
    Add helper `setThumbPosition(ratio)` that updates thumb left, track fill width, and
    computes selectedTime from ratio. Add helper `getThumbRatio()` that returns current
    thumb position as 0..1 ratio.
  - **Modifies**:
    - `frontend/src/timeline.js`

- [ ] **1.4 Click-on-track to jump**
  - **Dependencies**: 1.3
  - **Description**: Add `mousedown` handler on `.timeline-track`. When clicked (not on
    thumb), if the mouse is released without significant movement (< 1% of track width),
    treat it as a click: compute ratio from click X, call `setThumbPosition(ratio)`,
    `syncToURL()`, `onTimeChangedCb(selectedTime)`. This preserves the native slider
    behavior where clicking on the track jumps the thumb to that position. This will later
    be extended in Phase 3 to handle range selection when there IS movement.
  - **Modifies**:
    - `frontend/src/timeline.js`

### Completion Criteria Phase 1

- [ ] All subtasks completed (1.1, 1.2, 1.3, 1.4)
- [ ] Custom slider visually matches the previous native slider
- [ ] Thumb drag selects time points and fires callback
- [ ] Click on track jumps thumb to position
- [ ] Preset buttons, custom range, Apply, Live button all work
- [ ] `restoreFromURL()` correctly positions custom thumb
- [ ] Marker click snaps thumb to marker position
- [ ] URL sync works (`?time=`, `?from=`, `?to=`)

---

## Phase 2: Tooltip and Marker Hover

**Dependencies**: Phase 1
**Status**: Pending

### Description

Add tooltip component and marker hover behavior (snap-to-marker). When hovering over a
marker, the thumb visually snaps to the marker position and a tooltip shows the marker's
time and event info. The tooltip is also used during thumb drag (shows current time).

### Subtasks

- [ ] **2.1 Implement tooltip component**
  - **Dependencies**: None
  - **Description**: Add CSS for `.timeline-tooltip`: absolutely positioned within
    `.timeline-slider-container`, `bottom: 28px`, dark semi-transparent background
    (`rgba(0,0,0,0.85)`), white text, `font-size: 11px`, `padding: 4px 8px`,
    `border-radius: 4px`, `pointer-events: none`, `z-index: 10`, hidden by default.
    Add helper functions `showTooltip(text, ratio)` — positions tooltip at the given
    ratio (clamped to not overflow container edges) and sets text, and
    `hideTooltip()` — hides the tooltip. Integrate tooltip into thumb drag: show tooltip
    with current time during `THUMB_DRAG`, hide on `mouseup`.
  - **Modifies**:
    - `frontend/src/timeline.js`
    - `frontend/src/style.css`

- [ ] **2.2 Implement marker hover snap**
  - **Dependencies**: 2.1
  - **Description**: Add module-level variable `savedThumbRatio` to save thumb position
    before hover. In `renderMarkers()`, add `mouseenter` and `mouseleave` handlers to
    each marker element (markers already have `pointer-events: auto`). On `mouseenter`
    (only when `interactionState === IDLE`): set state to `MARKER_HOVER`, save current
    thumb ratio to `savedThumbRatio`, calculate marker ratio from `data-ts`, move thumb
    visually to marker position (without changing `selectedTime`), show tooltip with
    marker time and event title. On `mouseleave`: restore thumb to `savedThumbRatio`,
    hide tooltip, set state back to `IDLE`. Marker click handler (existing) should also
    reset state and clear saved position.
  - **Modifies**:
    - `frontend/src/timeline.js`

### Completion Criteria Phase 2

- [ ] All subtasks completed (2.1, 2.2)
- [ ] Tooltip appears above thumb during drag with current time
- [ ] Hovering over a marker snaps thumb visually and shows tooltip
- [ ] Leaving marker restores thumb to previous position
- [ ] Clicking marker still works (snap + load data)
- [ ] Tooltip does not overflow container edges

---

## Phase 3: Range Selection Drag

**Dependencies**: Phase 1
**Status**: Pending

### Description

Implement drag-to-select-range on the slider track. When the user mousedowns on the track
(not on the thumb) and drags, a semi-transparent overlay highlights the selected region.
On mouseup, the timeline zooms into the selected range (replaces from/to, reloads markers).

### Subtasks

- [ ] **3.1 Add range selection overlay CSS**
  - **Dependencies**: None
  - **Description**: Add CSS for `.timeline-range-overlay`: absolutely positioned within
    `.timeline-track`, full height, `background: rgba(66, 133, 244, 0.25)`,
    `border: 1px solid rgba(66, 133, 244, 0.6)`, `border-radius: 2px`,
    `pointer-events: none`, hidden by default (`display: none`). Dark theme variant:
    slightly brighter blue. Add `.timeline-track` cursor styles: `cursor: crosshair`
    when hovering over the track (indicates range selection is possible).
  - **Modifies**:
    - `frontend/src/style.css`

- [ ] **3.2 Implement range selection drag logic**
  - **Dependencies**: 3.1, 1.4
  - **Description**: Extend the `mousedown` handler on `.timeline-track` (from 1.4).
    On mousedown (not on thumb): record `dragStartRatio`, show range overlay, set state
    to `RANGE_SELECT`. On `mousemove` (document): calculate `currentRatio`, update overlay
    `left` = `min(start, current) * 100%`, `width` = `|current - start| * 100%`. Show
    tooltip at cursor position with current time. On `mouseup`: if drag distance
    < 1% of track width, treat as click (existing behavior from 1.4). Otherwise: calculate
    `newStart` and `newEnd` from the ratios, hide overlay, call `setRange(newStart, newEnd)`
    which zooms in, reloads markers, and repositions slider. Reset state to `IDLE`.
    Change track cursor to `crosshair` during idle, `col-resize` during range select.
  - **Modifies**:
    - `frontend/src/timeline.js`

### Completion Criteria Phase 3

- [ ] All subtasks completed (3.1, 3.2)
- [ ] Dragging on track shows semi-transparent overlay
- [ ] Tooltip follows cursor during drag with current time
- [ ] On release, timeline zooms into selected range
- [ ] Small drags (< 1%) treated as clicks (jump to position)
- [ ] Overlay is hidden after selection
- [ ] Markers reload for the new range

---

## Phase 4: Copy URL Button and i18n

**Dependencies**: Phase 1
**Status**: Pending

### Description

Add a "Copy URL" button to the timeline header and localization keys for new UI elements.

### Subtasks

- [ ] **4.1 Add i18n keys**
  - **Dependencies**: None
  - **Description**: Add two new keys to both locale files:
    - `timeline.copyUrl`: EN "Copy URL" / RU "Копировать URL"
    - `timeline.urlCopied`: EN "URL copied to clipboard" / RU "URL скопирован в буфер обмена"
  - **Modifies**:
    - `frontend/src/locales/en.js`
    - `frontend/src/locales/ru.js`

- [ ] **4.2 Add Copy URL button**
  - **Dependencies**: 4.1
  - **Description**: In `buildUI()`, add a button between `.timeline-time-display` and
    `.timeline-live-btn`:
    ```html
    <button id="timeline-copy-url" class="timeline-copy-btn" title="${t('timeline.copyUrl')}">
      <i class="bi bi-clipboard"></i>
    </button>
    ```
    Add CSS for `.timeline-copy-btn`: compact button, same height as Live button, icon-only,
    subtle border, hover effect. On click: `navigator.clipboard.writeText(window.location.href)`,
    swap icon to `bi-check` for 1.5 seconds, call `showToast(t('timeline.urlCopied'), 'info')`.
    Handle clipboard API errors gracefully (fallback or toast warning).
  - **Modifies**:
    - `frontend/src/timeline.js`
    - `frontend/src/style.css`

### Completion Criteria Phase 4

- [ ] All subtasks completed (4.1, 4.2)
- [ ] Copy URL button visible in timeline header
- [ ] Click copies current URL (with time/from/to params) to clipboard
- [ ] Toast confirms successful copy
- [ ] Icon changes briefly to checkmark as visual feedback
- [ ] Button styled consistently with other timeline controls

---

## Phase 5: Build and Test

**Dependencies**: Phase 1, Phase 2, Phase 3, Phase 4
**Status**: Pending

### Description

Build Docker image, deploy to test cluster, and perform manual testing of all new features.

### Subtasks

- [ ] **5.1 Lint and verify code**
  - **Dependencies**: None
  - **Description**: Run ESLint on modified frontend files. Fix any linting errors.
    Verify no console errors in browser dev tools.
  - **Modifies**: N/A (lint fixes if needed)

- [ ] **5.2 Build Docker image**
  - **Dependencies**: 5.1
  - **Description**: Build a new development Docker image with incremented dev tag
    (check current tag in `deploy/helm/dephealth-ui/values.yaml` and bump `-N` suffix).
    Push to Harbor registry.
  - **Creates**:
    - Docker image `harbor.kryukov.lan/library/dephealth-ui:vX.Y.Z-N`

- [ ] **5.3 Deploy and test**
  - **Dependencies**: 5.2
  - **Description**: Deploy to test cluster via Helm. Manual testing checklist:
    1. Enter history mode, verify custom slider matches previous appearance
    2. Drag thumb — verify time selection and tooltip
    3. Click on track — verify jump to position
    4. Hover over markers — verify snap and tooltip
    5. Leave marker — verify thumb returns to previous position
    6. Click marker — verify snap + data load
    7. Drag on track to select range — verify overlay, zoom-in
    8. Small drag on track — verify treated as click
    9. Copy URL button — verify clipboard copy and toast
    10. Open copied URL in new tab — verify state restore
    11. Test in dark theme
    12. Test preset buttons, custom range, Apply, Live
    13. Verify Grafana links still work with history time range
  - **Modifies**:
    - `deploy/helm/dephealth-ui/values.yaml` (image tag)

### Completion Criteria Phase 5

- [ ] All subtasks completed (5.1, 5.2, 5.3)
- [ ] Docker image built and pushed
- [ ] Deployed to test cluster
- [ ] All 13 manual test cases pass
- [ ] No console errors
- [ ] No visual regressions in light and dark themes

---

## Notes

- Phases 2, 3, and 4 can be developed in parallel after Phase 1 is complete (no
  inter-dependencies between them), but Phase 5 requires all to be done.
- The custom slider is the critical path — all other features depend on it.
- The state machine (`IDLE`, `THUMB_DRAG`, `RANGE_SELECT`, `MARKER_HOVER`) is the central
  coordination mechanism. Each interaction mode is mutually exclusive.
- Minimum drag threshold of 1% track width prevents accidental range selections.

---
