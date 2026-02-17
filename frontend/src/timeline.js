/**
 * Timeline module for history mode.
 * Manages time-travel UI: presets, custom range, custom slider, and Live button.
 */

import { t } from './i18n.js';
import { fetchTimelineEvents } from './api.js';
import { showToast } from './toast.js';

// --- Interaction states ---
const INTERACTION = { IDLE: 0, THUMB_DRAG: 1, RANGE_SELECT: 2, MARKER_HOVER: 3 };

// --- State ---
let historyMode = false;
let selectedTime = null; // Date
let rangeStart = null; // Date
let rangeEnd = null; // Date
let onTimeChangedCb = null;
let interactionState = INTERACTION.IDLE;
let savedThumbRatio = null; // saved position for marker hover restore

// DOM references
let panelEl = null;
let trackEl = null;
let thumbEl = null;
let trackFillEl = null;
let rangeOverlayEl = null;
let tooltipEl = null;
let markersEl = null;
let timeDisplayEl = null;

const PRESETS = [
  { label: '1h', hours: 1 },
  { label: '6h', hours: 6 },
  { label: '12h', hours: 12 },
  { label: '1d', hours: 24 },
  { label: '7d', hours: 168 },
  { label: '30d', hours: 720 },
  { label: '90d', hours: 2160 },
];

// --- Public API ---

export function isHistoryMode() {
  return historyMode;
}

export function getSelectedTime() {
  return selectedTime;
}

/**
 * Initialize the timeline module: build DOM inside #timeline-panel.
 * @param {Function} onTimeChanged - callback(Date|null), null means back to live
 */
export function initTimeline(onTimeChanged) {
  onTimeChangedCb = onTimeChanged;
  buildUI();
}

export function enterHistoryMode() {
  historyMode = true;
  document.body.classList.add('history-active');
  document.getElementById('header')?.classList.add('history-mode');
  document.getElementById('btn-history')?.classList.add('active');
  if (panelEl) panelEl.classList.remove('hidden');

  // Default: 1h preset if no range set yet
  if (!rangeStart) {
    applyPreset(1);
    const first = panelEl?.querySelector('.timeline-preset');
    if (first) first.classList.add('active');
  }
}

export function exitHistoryMode() {
  historyMode = false;
  selectedTime = null;
  rangeStart = null;
  rangeEnd = null;
  document.body.classList.remove('history-active');
  document.getElementById('header')?.classList.remove('history-mode');
  document.getElementById('btn-history')?.classList.remove('active');
  if (panelEl) panelEl.classList.add('hidden');

  // Clear active presets
  if (panelEl) {
    for (const b of panelEl.querySelectorAll('.timeline-preset')) {
      b.classList.remove('active');
    }
  }

  clearURLParams();
}

/**
 * Attempt to restore history mode from URL ?time= parameter.
 * @returns {boolean} true if history mode was activated from URL
 */
export function restoreFromURL() {
  try {
    const params = new URLSearchParams(window.location.search);
    const timeParam = params.get('time');
    if (!timeParam) return false;

    const time = new Date(timeParam);
    if (isNaN(time.getTime())) return false;

    const fromParam = params.get('from');
    const toParam = params.get('to');
    let start, end;

    if (fromParam && toParam) {
      start = new Date(fromParam);
      end = new Date(toParam);
      if (isNaN(start.getTime()) || isNaN(end.getTime())) {
        start = new Date(time.getTime() - 3600_000);
        end = new Date(time.getTime() + 3600_000);
      }
    } else {
      // Default: +/- 1h around the selected time
      start = new Date(time.getTime() - 3600_000);
      end = new Date(time.getTime() + 3600_000);
    }

    enterHistoryMode();
    rangeStart = start;
    rangeEnd = end;
    selectedTime = time;

    // Update inputs
    const startInput = document.getElementById('timeline-start');
    const endInput = document.getElementById('timeline-end');
    if (startInput) startInput.value = toLocalDateTimeString(start);
    if (endInput) endInput.value = toLocalDateTimeString(end);

    // Position thumb
    const totalMs = end.getTime() - start.getTime();
    const ratio = totalMs > 0
      ? (time.getTime() - start.getTime()) / totalMs
      : 1;
    setThumbPositionVisual(Math.max(0, Math.min(1, ratio)));

    updateTimeDisplay();
    loadMarkers();
    return true;
  } catch (err) {
    console.warn('Failed to restore history from URL:', err);
    return false;
  }
}

/**
 * Sync current timeline state to URL query parameters.
 * Preserves existing params like ?namespace=.
 */
function syncToURL() {
  const url = new URL(window.location);
  if (selectedTime) {
    url.searchParams.set('time', selectedTime.toISOString());
  }
  if (rangeStart) {
    url.searchParams.set('from', rangeStart.toISOString());
  }
  if (rangeEnd) {
    url.searchParams.set('to', rangeEnd.toISOString());
  }
  history.replaceState(null, '', url);
}

/**
 * Remove timeline-related params from URL.
 */
function clearURLParams() {
  const url = new URL(window.location);
  url.searchParams.delete('time');
  url.searchParams.delete('from');
  url.searchParams.delete('to');
  history.replaceState(null, '', url);
}

// --- Slider helpers ---

/**
 * Move thumb and track fill visually without updating selectedTime.
 * @param {number} ratio - 0..1 position on the track
 */
function setThumbPositionVisual(ratio) {
  ratio = Math.max(0, Math.min(1, ratio));
  if (thumbEl) thumbEl.style.left = `${ratio * 100}%`;
  if (trackFillEl) trackFillEl.style.width = `${ratio * 100}%`;
}

/**
 * Move thumb visually and update selectedTime from ratio.
 * @param {number} ratio - 0..1 position on the track
 */
function setThumbPosition(ratio) {
  ratio = Math.max(0, Math.min(1, ratio));
  setThumbPositionVisual(ratio);
  if (rangeStart && rangeEnd) {
    const ms = rangeStart.getTime() + ratio * (rangeEnd.getTime() - rangeStart.getTime());
    selectedTime = new Date(ms);
  }
}

/**
 * Get current thumb position as 0..1 ratio.
 */
function getThumbRatio() {
  if (!thumbEl) return 1;
  return (parseFloat(thumbEl.style.left) || 0) / 100;
}

/**
 * Convert a mouse clientX to a 0..1 ratio on the track.
 */
function getTrackRatio(clientX) {
  if (!trackEl) return 0;
  const rect = trackEl.getBoundingClientRect();
  if (rect.width === 0) return 0;
  return Math.max(0, Math.min(1, (clientX - rect.left) / rect.width));
}

// --- Tooltip helpers ---

/**
 * Show tooltip at the given track ratio with the specified text.
 * Clamped to container edges so it doesn't overflow.
 */
function showTooltip(text, ratio) {
  if (!tooltipEl || !trackEl) return;
  tooltipEl.textContent = text;
  tooltipEl.classList.remove('hidden');

  const containerWidth = trackEl.parentElement.offsetWidth;
  const tooltipWidth = tooltipEl.offsetWidth;
  const targetPx = ratio * containerWidth;

  let leftPx = targetPx - tooltipWidth / 2;
  leftPx = Math.max(0, Math.min(containerWidth - tooltipWidth, leftPx));
  tooltipEl.style.left = `${leftPx}px`;
}

function hideTooltip() {
  if (!tooltipEl) return;
  tooltipEl.classList.add('hidden');
}

// --- Slider interaction handlers ---

function onThumbMouseDown(e) {
  if (interactionState !== INTERACTION.IDLE) return;
  e.preventDefault();
  e.stopPropagation();
  interactionState = INTERACTION.THUMB_DRAG;
  thumbEl.classList.add('dragging');

  const onMouseMove = (ev) => {
    const ratio = getTrackRatio(ev.clientX);
    setThumbPosition(ratio);
    updateTimeDisplay();
    showTooltip(selectedTime.toLocaleString(), ratio);
  };

  const onMouseUp = () => {
    interactionState = INTERACTION.IDLE;
    thumbEl.classList.remove('dragging');
    hideTooltip();
    syncToURL();
    if (onTimeChangedCb) onTimeChangedCb(selectedTime);
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  };

  document.addEventListener('mousemove', onMouseMove);
  document.addEventListener('mouseup', onMouseUp);
}

function onContainerMouseDown(e) {
  if (interactionState !== INTERACTION.IDLE) return;
  if (e.target.closest('.timeline-marker')) return;
  e.preventDefault();

  const startRatio = getTrackRatio(e.clientX);
  let hasMoved = false;

  const onMouseMove = (ev) => {
    const currentRatio = getTrackRatio(ev.clientX);
    if (Math.abs(currentRatio - startRatio) > 0.01) {
      hasMoved = true;
      // Range selection will be implemented in Phase 3
    }
  };

  const onMouseUp = (ev) => {
    if (!hasMoved) {
      // Treat as click: jump thumb to position
      const ratio = getTrackRatio(ev.clientX);
      setThumbPosition(ratio);
      updateTimeDisplay();
      syncToURL();
      if (onTimeChangedCb) onTimeChangedCb(selectedTime);
    }
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  };

  document.addEventListener('mousemove', onMouseMove);
  document.addEventListener('mouseup', onMouseUp);
}

// --- Private ---

function buildUI() {
  panelEl = document.getElementById('timeline-panel');
  if (!panelEl) return;

  panelEl.innerHTML = `
    <div class="timeline-header">
      <div class="timeline-presets">
        ${PRESETS.map((p) => `<button class="timeline-preset" data-hours="${p.hours}">${p.label}</button>`).join('')}
      </div>
      <div class="timeline-custom-range">
        <input type="datetime-local" id="timeline-start" step="1">
        <span class="timeline-range-sep">&ndash;</span>
        <input type="datetime-local" id="timeline-end" step="1">
        <button id="timeline-apply" class="timeline-apply-btn" data-i18n="timeline.apply">Apply</button>
      </div>
      <div class="timeline-time-display" id="timeline-time-display"></div>
      <button id="timeline-live" class="timeline-live-btn" data-i18n="timeline.live">Live</button>
    </div>
    <div class="timeline-slider-container" id="timeline-slider-container">
      <div class="timeline-track" id="timeline-track">
        <div class="timeline-track-fill" id="timeline-track-fill"></div>
        <div class="timeline-range-overlay hidden" id="timeline-range-overlay"></div>
      </div>
      <div id="timeline-markers" class="timeline-markers"></div>
      <div class="timeline-thumb" id="timeline-thumb"></div>
      <div class="timeline-tooltip hidden" id="timeline-tooltip"></div>
    </div>
  `;

  trackEl = document.getElementById('timeline-track');
  thumbEl = document.getElementById('timeline-thumb');
  trackFillEl = document.getElementById('timeline-track-fill');
  rangeOverlayEl = document.getElementById('timeline-range-overlay');
  tooltipEl = document.getElementById('timeline-tooltip');
  markersEl = document.getElementById('timeline-markers');
  timeDisplayEl = document.getElementById('timeline-time-display');

  // Preset buttons
  for (const btn of panelEl.querySelectorAll('.timeline-preset')) {
    btn.addEventListener('click', () => {
      const hours = parseInt(btn.dataset.hours, 10);
      applyPreset(hours);
      for (const b of panelEl.querySelectorAll('.timeline-preset')) b.classList.remove('active');
      btn.classList.add('active');
    });
  }

  // Custom range Apply
  document.getElementById('timeline-apply').addEventListener('click', () => {
    const startInput = document.getElementById('timeline-start');
    const endInput = document.getElementById('timeline-end');
    if (!startInput.value || !endInput.value) return;
    const start = new Date(startInput.value);
    const end = new Date(endInput.value);
    if (isNaN(start.getTime()) || isNaN(end.getTime()) || start >= end) return;
    setRange(start, end);
    for (const b of panelEl.querySelectorAll('.timeline-preset')) b.classList.remove('active');
  });

  // Custom slider: thumb drag
  thumbEl.addEventListener('mousedown', onThumbMouseDown);

  // Custom slider: container click / range select
  const containerEl = document.getElementById('timeline-slider-container');
  containerEl.addEventListener('mousedown', onContainerMouseDown);

  // Live button
  document.getElementById('timeline-live').addEventListener('click', () => {
    exitHistoryMode();
    if (onTimeChangedCb) onTimeChangedCb(null);
  });
}

function applyPreset(hours) {
  const end = new Date();
  const start = new Date(end.getTime() - hours * 3600_000);
  setRange(start, end);
}

function setRange(start, end) {
  rangeStart = start;
  rangeEnd = end;

  // Update custom range inputs to reflect current range
  const startInput = document.getElementById('timeline-start');
  const endInput = document.getElementById('timeline-end');
  if (startInput) startInput.value = toLocalDateTimeString(start);
  if (endInput) endInput.value = toLocalDateTimeString(end);

  // Position thumb at the end (most recent)
  setThumbPosition(1);
  updateTimeDisplay();
  syncToURL();

  if (onTimeChangedCb) onTimeChangedCb(selectedTime);

  // Fetch and render event markers
  loadMarkers();
}

async function loadMarkers() {
  if (!rangeStart || !rangeEnd || !markersEl) return;
  try {
    const events = await fetchTimelineEvents(rangeStart.toISOString(), rangeEnd.toISOString());
    if (!events || events.length === 0) {
      markersEl.innerHTML = `<div class="timeline-no-data">${t('timeline.noData')}</div>`;
      return;
    }
    renderMarkers(events);
  } catch (err) {
    console.warn('Failed to load timeline events:', err);
    markersEl.innerHTML = '';
    showToast(t('timeline.eventsError'), 'warning');
  }
}

function renderMarkers(events) {
  if (!markersEl || !rangeStart || !rangeEnd) return;
  const totalMs = rangeEnd.getTime() - rangeStart.getTime();
  if (totalMs <= 0) { markersEl.innerHTML = ''; return; }

  markersEl.innerHTML = events.map((ev) => {
    const ts = new Date(ev.timestamp).getTime();
    const pct = ((ts - rangeStart.getTime()) / totalMs) * 100;
    if (pct < 0 || pct > 100) return '';
    const cls = ev.kind === 'degradation' ? 'marker-degradation'
      : ev.kind === 'recovery' ? 'marker-recovery'
        : 'marker-change';
    const title = `${ev.service}: ${ev.fromState} \u2192 ${ev.toState}`;
    return `<div class="timeline-marker ${cls}" style="left:${pct}%" title="${title}" data-ts="${ts}"></div>`;
  }).join('');

  for (const m of markersEl.querySelectorAll('.timeline-marker')) {
    // Hover: snap thumb visually and show tooltip
    m.addEventListener('mouseenter', () => {
      if (interactionState !== INTERACTION.IDLE) return;
      interactionState = INTERACTION.MARKER_HOVER;
      savedThumbRatio = getThumbRatio();

      const ts = parseInt(m.dataset.ts, 10);
      const ratio = (ts - rangeStart.getTime()) / totalMs;
      setThumbPositionVisual(ratio);

      const time = new Date(ts).toLocaleString();
      const info = m.getAttribute('title');
      showTooltip(info ? `${time}\n${info}` : time, ratio);
    });

    m.addEventListener('mouseleave', () => {
      if (interactionState !== INTERACTION.MARKER_HOVER) return;
      setThumbPositionVisual(savedThumbRatio);
      savedThumbRatio = null;
      hideTooltip();
      interactionState = INTERACTION.IDLE;
    });

    // Click: commit to marker position and load data
    m.addEventListener('click', () => {
      interactionState = INTERACTION.IDLE;
      savedThumbRatio = null;
      hideTooltip();

      const ts = parseInt(m.dataset.ts, 10);
      const ratio = (ts - rangeStart.getTime()) / totalMs;
      setThumbPosition(ratio);
      updateTimeDisplay();
      syncToURL();
      if (onTimeChangedCb) onTimeChangedCb(selectedTime);
    });
  }
}

function updateTimeDisplay() {
  if (!timeDisplayEl || !selectedTime) return;
  timeDisplayEl.textContent = selectedTime.toLocaleString();
}

function toLocalDateTimeString(date) {
  const pad = (n) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
