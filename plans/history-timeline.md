# План разработки: History Timeline

## Метаданные

- **Версия плана**: 1.0.0
- **Дата создания**: 2026-02-17
- **Последнее обновление**: 2026-02-17
- **Статус**: Complete (E2E pending)
- **Дизайн-документ**: [.tasks/history-graf.md](../.tasks/history-graf.md)
- **Целевая версия**: v0.16.0

---

## История версий

- **v1.0.0** (2026-02-17): Начальная версия плана

---

## Текущий статус

- **Активная фаза**: Complete
- **Активный подпункт**: —
- **Последнее обновление**: 2026-02-17
- **Примечание**: All phases completed. Image v0.16.0-4 deployed. E2E testing pending.

---

## Оглавление

- [x] [Phase 1: Backend — Historical Queries](#phase-1-backend--historical-queries)
- [x] [Phase 2: Backend — Timeline Events Endpoint](#phase-2-backend--timeline-events-endpoint)
- [x] [Phase 3: Backend — Build & Test](#phase-3-backend--build--test)
- [x] [Phase 4: Frontend — Timeline Panel UI](#phase-4-frontend--timeline-panel-ui)
- [x] [Phase 5: Frontend — Event Markers & Polish](#phase-5-frontend--event-markers--polish)
- [x] [Phase 6: Full Build, Deploy & E2E Test](#phase-6-full-build-deploy--e2e-test)

---

## Phase 1: Backend — Historical Queries

**Dependencies**: None
**Status**: Done

### Описание

Add time-travel capability to all Prometheus queries. The core idea: extend `QueryOptions` with `Time *time.Time` and propagate it through `prometheusClient.query()` to the Prometheus `/api/v1/query?time=` parameter. Also add `QueryHistoricalAlerts()` that fetches firing alerts from the `ALERTS` metric at a given historical timestamp (instead of AlertManager API).

### Подпункты

- [ ] **1.1 Extend models and PrometheusClient interface**
  - **Dependencies**: None
  - **Description**: Add `Time *time.Time` field to `QueryOptions`. Add `Time *time.Time` and `IsHistory bool` fields to `TopologyMeta`. Add `QueryHistoricalAlerts(ctx, time.Time)` method to `PrometheusClient` interface. Add `HistoricalAlert` model struct.
  - **Modifies**:
    - `internal/topology/models.go`
    - `internal/topology/prometheus.go` (interface only)

- [ ] **1.2 Modify `query()` to accept time parameter**
  - **Dependencies**: 1.1
  - **Description**: Change `prometheusClient.query(ctx, promql)` signature to `query(ctx, promql, at *time.Time)`. When `at != nil`, add `time=<unix_ts>` to the Prometheus API request URL params. Update all callers within prometheus.go to pass the new parameter (initially `nil` — no behavioral change).
  - **Modifies**:
    - `internal/topology/prometheus.go`

- [ ] **1.3 Propagate `opts.Time` through all Query methods**
  - **Dependencies**: 1.2
  - **Description**: Update `QueryTopologyEdges`, `QueryHealthState`, `QueryAvgLatency`, `QueryDependencyStatus`, `QueryDependencyStatusDetail`, `QueryTopologyEdgesLookback` to pass `opts.Time` to the internal `query()` call. Each method already receives `opts QueryOptions`, so only the `query()` call line changes.
  - **Modifies**:
    - `internal/topology/prometheus.go`

- [ ] **1.4 Implement `QueryHistoricalAlerts()`**
  - **Dependencies**: 1.2
  - **Description**: Implement `QueryHistoricalAlerts(ctx, at time.Time)` on `prometheusClient`. Query: `ALERTS{alertstate="firing"}` with `time=at`. Parse labels: `alertname`, `namespace`, `name` (or `service`), `severity`. Return `[]HistoricalAlert`. Map `HistoricalAlert` → `alerts.Alert` for compatibility with `enrichWithAlerts()`.
  - **Modifies**:
    - `internal/topology/prometheus.go`

- [ ] **1.5 Modify `GraphBuilder.Build()` for history mode**
  - **Dependencies**: 1.3, 1.4
  - **Description**: In `Build()`, when `opts.Time != nil`: (a) pass `opts.Time` to all prom queries (already done via 1.3), (b) call `prom.QueryHistoricalAlerts(*opts.Time)` instead of `am.FetchAlerts()`, (c) convert historical alerts to `[]alerts.Alert` for `enrichWithAlerts()`, (d) set `meta.Time` and `meta.IsHistory` in the response. The `lookback` window should be applied relative to `opts.Time` when set.
  - **Modifies**:
    - `internal/topology/graph.go`

- [ ] **1.6 Parse `?time=` in HTTP handlers**
  - **Dependencies**: 1.5
  - **Description**: In `handleTopology()`: parse optional `?time=` query param (RFC3339 format), set `opts.Time`. Historical requests bypass cache entirely (no Get, no Set). In `handleCascadeAnalysis()`: same pattern — parse `?time=`, always build fresh when set. Validate time format and return 400 on invalid input.
  - **Modifies**:
    - `internal/server/server.go`

- [ ] **1.7 Unit tests for historical queries**
  - **Dependencies**: 1.6
  - **Description**: Add tests for: (a) `query()` correctly appends `time=` parameter, (b) `QueryHistoricalAlerts()` parses ALERTS response, (c) `Build()` uses historical alerts instead of AM, (d) `handleTopology(?time=)` bypasses cache and returns `isHistory=true` in meta, (e) `handleCascadeAnalysis(?time=)` works with historical data. Use existing test patterns from `prometheus_test.go`, `server_test.go`.
  - **Creates/Modifies**:
    - `internal/topology/prometheus_test.go`
    - `internal/topology/graph_test.go`
    - `internal/server/server_test.go`

### Критерии завершения Phase 1

- [ ] Все подпункты завершены (1.1–1.7)
- [ ] `go build ./...` compiles without errors
- [ ] `go test ./...` passes (existing + new tests)
- [ ] `GET /api/v1/topology?time=2026-01-01T00:00:00Z` returns historical data with `meta.isHistory=true`
- [ ] `GET /api/v1/cascade-analysis?time=...` returns historical cascade data
- [ ] Historical requests do not interact with the cache

---

## Phase 2: Backend — Timeline Events Endpoint

**Dependencies**: Phase 1
**Status**: Done

### Описание

Create a new `internal/timeline/` package and `/api/v1/timeline/events` endpoint. This endpoint queries `app_dependency_status` via Prometheus `query_range` API over a time window, detects state transitions, and returns a list of timestamped events for the frontend slider markers.

### Подпункты

- [ ] **2.1 Implement `queryRange()` in Prometheus client**
  - **Dependencies**: None
  - **Description**: Add `queryRange(ctx, promql, start, end, step)` private method to `prometheusClient`. Calls `/api/v1/query_range` with appropriate params. Parse the matrix response format (`"resultType":"matrix"`, values as `[[timestamp, value], ...]`). Add `promRangeResult` and `promMatrixData` response models.
  - **Modifies**:
    - `internal/topology/prometheus.go`

- [ ] **2.2 Export `QueryRange` on PrometheusClient interface**
  - **Dependencies**: 2.1
  - **Description**: Add `QueryStatusRange(ctx, start, end, step, namespace) ([]RangeResult, error)` to the `PrometheusClient` interface. This method queries `app_dependency_status == 1` over the range and returns per-edge time series data. The `RangeResult` model contains: `EdgeKey`, `Status` string, and `Values []TimeValue` (`{Timestamp time.Time, Value float64}`).
  - **Modifies**:
    - `internal/topology/prometheus.go`

- [ ] **2.3 Create `internal/timeline/` package**
  - **Dependencies**: 2.2
  - **Description**: Create new package with: `Event` struct (Timestamp, Service, Namespace, FromState, ToState, Kind), `EventsRequest` struct (Start, End time), `autoStep()` function (range→step mapping per design table), `QueryStatusTransitions()` function. The transition detection logic: iterate time series values, compare consecutive status values, emit an event when status changes. `Kind` classification: transition to worse state = "degradation", to better = "recovery", other = "change".
  - **Creates**:
    - `internal/timeline/events.go`
    - `internal/timeline/events_test.go`

- [ ] **2.4 Add `handleTimelineEvents` endpoint**
  - **Dependencies**: 2.3
  - **Description**: Add `GET /api/v1/timeline/events?start=<RFC3339>&end=<RFC3339>` handler in `server.go`. Parse and validate `start`/`end` params (required, must be valid RFC3339, start < end). Call `timeline.QueryStatusTransitions()`. Register route under auth middleware alongside existing API endpoints. Return JSON array of events.
  - **Modifies**:
    - `internal/server/server.go`

- [ ] **2.5 Unit tests for timeline events**
  - **Dependencies**: 2.4
  - **Description**: Test: (a) `autoStep()` returns correct step for each range bracket, (b) `QueryStatusTransitions()` correctly detects transitions in mock data, (c) edge case: no transitions returns empty array, (d) `handleTimelineEvents()` HTTP handler validation (missing params, bad format), (e) successful response format.
  - **Creates/Modifies**:
    - `internal/timeline/events_test.go`
    - `internal/server/server_test.go`

### Критерии завершения Phase 2

- [ ] Все подпункты завершены (2.1–2.5)
- [ ] `go build ./...` compiles without errors
- [ ] `go test ./...` passes
- [ ] `GET /api/v1/timeline/events?start=...&end=...` returns JSON array of events
- [ ] Step auto-calculation produces reasonable values for all range brackets

---

## Phase 3: Backend — Build & Test

**Dependencies**: Phase 1, Phase 2
**Status**: Done

### Описание

Build the Docker image with all backend changes and deploy to the test cluster. Verify historical queries and timeline events endpoint work against the live VictoriaMetrics instance with real data.

### Подпункты

- [ ] **3.1 Build Docker image**
  - **Dependencies**: None
  - **Description**: Run `make docker-build TAG=v0.16.0-1`. Verify multi-arch build succeeds (linux/amd64, linux/arm64). Push to Harbor registry.
  - **Creates**:
    - Docker image `harbor.kryukov.lan/library/dephealth-ui:v0.16.0-1`

- [ ] **3.2 Deploy to test cluster**
  - **Dependencies**: 3.1
  - **Description**: Update Helm values to use new image tag. Deploy with `helm upgrade`. Verify pod starts successfully and health probes pass.
  - **Modifies**:
    - Helm release `dephealth-ui`

- [ ] **3.3 Integration test: historical topology**
  - **Dependencies**: 3.2
  - **Description**: Manually test `GET /api/v1/topology?time=<5min_ago>` against the running instance. Verify: (a) response contains `meta.isHistory: true`, (b) nodes and edges reflect the state at that time, (c) historical alerts are populated from ALERTS metric. Compare with live response to confirm data differs appropriately.

- [ ] **3.4 Integration test: timeline events**
  - **Dependencies**: 3.2
  - **Description**: Test `GET /api/v1/timeline/events?start=<1h_ago>&end=<now>`. Verify: (a) response is a valid JSON array, (b) events have correct fields, (c) no server errors in logs. If no transitions occurred recently, artificially cause one by stopping/starting a test uniproxy pod and re-querying.

### Критерии завершения Phase 3

- [ ] Все подпункты завершены (3.1–3.4)
- [ ] Docker image built and pushed successfully
- [ ] Pod running in test cluster with healthy probes
- [ ] Historical topology API returns correct data
- [ ] Timeline events API returns correct data
- [ ] No errors in application logs

---

## Phase 4: Frontend — Timeline Panel UI

**Dependencies**: Phase 3
**Status**: Done

### Описание

Build the timeline panel UI: history mode toggle button in toolbar, bottom panel with time range presets + custom datetime inputs + slider, integration with `main.js` (stop polling in history mode, pass `?time=` to `fetchTopology`). No event markers yet — that's Phase 5.

### Подпункты

- [ ] **4.1 Add `time` parameter to `fetchTopology()` in api.js**
  - **Dependencies**: None
  - **Description**: Modify `fetchTopology(namespace)` → `fetchTopology(namespace, time)`. When `time` is provided, add `?time=<value>` to the URL. Disable ETag/If-None-Match for historical requests (they bypass server cache anyway). Add `fetchTimelineEvents(start, end)` function (will be used in Phase 5).
  - **Modifies**:
    - `frontend/src/api.js`

- [ ] **4.2 Create `timeline.js` module**
  - **Dependencies**: 4.1
  - **Description**: New module with: state management (`historyMode`, `selectedTime`, `rangeStart`, `rangeEnd`), exported functions (`initTimeline`, `isHistoryMode`, `getSelectedTime`, `enterHistoryMode`, `exitHistoryMode`), `buildUI()` — dynamically creates timeline panel DOM and injects before `<footer>`, preset buttons with click handlers, custom datetime-local inputs with "Apply" button, range slider (type=range, min=0, max=1000), "Live" button to exit history mode. Slider `change` event (fires on release) maps slider position to timestamp within [rangeStart, rangeEnd] and calls `onTimeChanged` callback.
  - **Creates**:
    - `frontend/src/timeline.js`

- [ ] **4.3 Add timeline HTML and toolbar button**
  - **Dependencies**: 4.2
  - **Description**: Add `#btn-history` button (icon: `bi-clock-history`) to header `.toolbar` in `index.html`, positioned before the theme button. The timeline panel itself is created dynamically by `timeline.js`, but add the placeholder `<div id="timeline-panel" class="timeline-panel hidden"></div>` before `<footer>` in `index.html`.
  - **Modifies**:
    - `frontend/index.html`

- [ ] **4.4 Add timeline CSS styles**
  - **Dependencies**: 4.3
  - **Description**: Add CSS for: `.timeline-panel` (fixed bottom, above status bar, 80px height), `.timeline-header` (flex row with presets + inputs + live button), `.timeline-presets button` (compact pill buttons), `.timeline-custom-range` (datetime inputs), `.timeline-slider-container` (relative wrapper for slider + markers), `.timeline-live-btn` (green "Live" button), `header.history-mode` (distinct background color), `body.history-active #cy` (reduced height to accommodate panel). Support both light and dark themes via CSS variables.
  - **Modifies**:
    - `frontend/src/style.css`

- [ ] **4.5 Integrate timeline with main.js**
  - **Dependencies**: 4.2, 4.3, 4.4
  - **Description**: In `main.js`: import timeline module, call `initTimeline()` during `init()` with `onTimeChanged` callback that stops/starts polling and calls `refresh()`. Modify `refresh()` to pass `getSelectedTime()?.toISOString()` as second argument to `fetchTopology()`. Add click handler for `#btn-history` that toggles timeline panel visibility and calls `enterHistoryMode()`/`exitHistoryMode()`. On `enterHistoryMode`: add `history-active` class to body. On `exitHistoryMode`: remove class, resume polling. Read `?time=` from URL params during init and enter history mode if present.
  - **Modifies**:
    - `frontend/src/main.js`

- [ ] **4.6 Localization keys**
  - **Dependencies**: 4.2
  - **Description**: Add translation keys for both locales: `timeline.title`, `timeline.live`, `timeline.apply`, `timeline.historyBanner` (shown when viewing historical data), `toolbar.history`, preset labels if needed. Approximately 10–15 new keys per locale.
  - **Modifies**:
    - `frontend/src/locales/en.js`
    - `frontend/src/locales/ru.js`

### Критерии завершения Phase 4

- [ ] Все подпункты завершены (4.1–4.6)
- [ ] History button appears in toolbar, opens timeline panel on click
- [ ] Preset buttons set the time range and slider boundaries
- [ ] Custom datetime inputs work with "Apply" button
- [ ] Slider release triggers graph redraw with historical data
- [ ] "Live" button returns to realtime mode with auto-refresh resumed
- [ ] Visual distinction between live and history mode (header styling)
- [ ] `#cy` container resizes correctly when timeline panel is visible
- [ ] Works in both light and dark themes
- [ ] All text is localized (en + ru)

---

## Phase 5: Frontend — Event Markers & Polish

**Dependencies**: Phase 4
**Status**: Done

### Описание

Add event markers on the slider, URL synchronization, Grafana link adjustments for history mode, and final UI polish.

### Подпункты

- [x] **5.1 Fetch and render event markers on slider** *(done in Phase 4)*
  - **Dependencies**: None
  - **Description**: When time range changes (preset click or custom apply), call `fetchTimelineEvents(start, end)`. Render markers as positioned `<div>` elements inside `.timeline-markers` container. Each marker has CSS class based on `kind` (degradation=red, recovery=green, change=orange). Marker `title` attribute shows `service: fromState → toState`. Click on marker snaps slider to that timestamp and triggers graph update.
  - **Modifies**:
    - `frontend/src/timeline.js`

- [x] **5.2 URL synchronization**
  - **Dependencies**: None
  - **Description**: When slider position changes: update URL with `?time=<ISO8601>` via `history.replaceState()`. Preserve existing `?namespace=` param. Optionally include `?from=` and `?to=` for the range. When entering live mode: remove `?time=`, `?from=`, `?to=` from URL. `syncFromURL()` called during init reads these params and restores state.
  - **Modifies**:
    - `frontend/src/timeline.js`
    - `frontend/src/main.js` (init section)

- [x] **5.3 History mode visual indicator** *(done in Phase 4)*
  - **Dependencies**: None
  - **Description**: When in history mode, show a prominent banner/badge with the current timestamp (e.g., in the header or above the timeline panel). Update the timestamp display as the slider moves. Format: locale-aware datetime string. The header should have a visually distinct background (CSS `header.history-mode`). Add `data-history-time` attribute for CSS `::after` content display.
  - **Modifies**:
    - `frontend/src/timeline.js`
    - `frontend/src/style.css`

- [x] **5.4 Grafana links with historical time range**
  - **Dependencies**: None
  - **Description**: In `sidebar.js`, when building Grafana dashboard URLs and `isHistoryMode()` is true: append `&from=<ts-1h>&to=<ts+1h>` (Unix milliseconds) to all Grafana URLs. This ensures "Open in Grafana" links navigate to the relevant historical time window. Import `isHistoryMode` and `getSelectedTime` from `timeline.js`.
  - **Modifies**:
    - `frontend/src/sidebar.js`

- [x] **5.5 Status bar update for history mode** *(done in Phase 4)*
  - **Dependencies**: None
  - **Description**: In `updateStatus()` function in `main.js`, when `data.meta.isHistory` is true, show a different status line: replace "Updated at HH:MM:SS" with "Viewing: <historical_timestamp>". Use a distinct icon or style for the status connection dot.
  - **Modifies**:
    - `frontend/src/main.js`

- [x] **5.6 Edge cases and error handling**
  - **Dependencies**: 5.1, 5.2
  - **Description**: Handle: (a) timeline events API failure (show toast, keep slider functional without markers), (b) topology API failure in history mode (show error, don't break timeline panel), (c) invalid URL `?time=` param (ignore, stay in live mode), (d) time range with no data from VM (show "no data" message in timeline), (e) slider at exact range boundaries. Add `timeline.noData` and `timeline.eventsError` localization keys.
  - **Modifies**:
    - `frontend/src/timeline.js`
    - `frontend/src/locales/en.js`
    - `frontend/src/locales/ru.js`

### Критерии завершения Phase 5

- [x] Все подпункты завершены (5.1–5.6)
- [x] Event markers render on slider at correct positions
- [x] Clicking a marker snaps slider and updates graph
- [x] URL reflects current `?time=` and can be shared
- [x] Opening shared URL enters history mode at correct time
- [x] Grafana links include historical time range
- [x] Status bar shows historical timestamp in history mode
- [x] Error scenarios handled gracefully (toast messages, no crashes)
- [x] All new text localized (en + ru)

---

## Phase 6: Full Build, Deploy & E2E Test

**Dependencies**: Phase 4, Phase 5
**Status**: Done

### Описание

Build the complete image with all backend + frontend changes, deploy to the test cluster, and perform end-to-end testing of the entire history timeline feature.

### Подпункты

- [x] **6.1 Build final Docker image**
  - **Dependencies**: None
  - **Description**: Run `make docker-build TAG=v0.16.0-2` (or appropriate dev tag). Verify build succeeds with frontend included.
  - **Creates**:
    - Docker image `harbor.kryukov.lan/library/dephealth-ui:v0.16.0-2`

- [x] **6.2 Deploy to test cluster**
  - **Dependencies**: 6.1
  - **Description**: Helm upgrade with new image tag. Verify pod starts, health probes pass, SPA loads in browser.

- [x] **6.3 E2E test: full timeline workflow**
  - **Dependencies**: 6.2
  - **Description**: Test the complete user flow: (a) click History button → timeline panel opens, (b) select "1d" preset → slider appears with markers, (c) drag slider → graph updates on release with historical data, (d) click event marker → slider snaps to event, (e) verify sidebar shows historical node details, (f) verify cascade analysis works for historical time, (g) copy URL → open in new tab → same historical view, (h) click "Live" → return to realtime with auto-refresh. Test in both light and dark themes. Test both English and Russian locales.

- [x] **6.4 Update documentation**
  - **Dependencies**: 6.3
  - **Description**: Update `docs/API.md` and `docs/API.ru.md` with new endpoints: `/api/v1/topology?time=`, `/api/v1/cascade-analysis?time=`, `/api/v1/timeline/events?start=&end=`. Update `docs/application-design.md` with history mode architecture. Update `CHANGELOG.md` if present.
  - **Modifies**:
    - `docs/API.md`
    - `docs/API.ru.md`
    - `docs/application-design.md`
    - `docs/application-design.ru.md`

### Критерии завершения Phase 6

- [x] Все подпункты завершены (6.1–6.4)
- [x] Docker image built and pushed successfully
- [x] Pod running in test cluster
- [ ] Complete E2E workflow works without errors
- [ ] Works in both themes (light + dark)
- [ ] Works in both locales (en + ru)
- [x] Documentation updated
- [ ] No console errors in browser
- [ ] No server errors in application logs
- [ ] Ready for release as v0.16.0

---

## Примечания

- **Версионирование**: Development images `v0.16.0-N`, release `v0.16.0`
- **ALERTS metric**: Prometheus/VM automatically creates `ALERTS{alertname, alertstate}` time series for all alerting rules. Labels available depend on alert rule configuration — `namespace`, `name`/`service`, and `severity` are typically present for dephealth alert rules
- **query_range performance**: For large time ranges (90d), the adaptive step prevents overloading VM. Monitor VM query latency during testing
- **Stale nodes in history**: The `lookback` window is applied relative to `opts.Time`, so `QueryTopologyEdgesLookback` uses `last_over_time(metric[lookback])` evaluated at the historical timestamp
- **Cache bypass**: Historical requests never use or populate the in-memory cache, ensuring live mode performance is unaffected
- **Keyboard navigation**: Deferred to a future iteration — arrow keys stepping through events on the timeline
