// Export modal module
// Handles graph export in multiple formats via frontend (PNG/SVG current view)
// and backend API (all formats for full graph, data formats for current scope).

import { t } from './i18n.js';
import { showToast } from './toast.js';

const $ = (sel) => document.querySelector(sel);

let cyRef = null;
let getFiltersRef = null;
let selectedFormat = 'png';
let selectedScope = 'current';

// Hint keys per format+scope combination
const HINTS = {
  'current:png': 'export.hint.currentPng',
  'current:svg': 'export.hint.currentSvg',
  'current:json': 'export.hint.currentData',
  'current:csv': 'export.hint.currentData',
  'current:dot': 'export.hint.currentData',
  'full:png': 'export.hint.fullPng',
  'full:svg': 'export.hint.fullSvg',
  'full:json': 'export.hint.fullData',
  'full:csv': 'export.hint.fullData',
  'full:dot': 'export.hint.fullData',
};

/**
 * Initialize the export modal.
 * @param {Object} cy - Cytoscape instance
 * @param {Function} getFilters - Returns { namespace, group } for current filters
 */
export function initExportModal(cy, getFilters) {
  cyRef = cy;
  getFiltersRef = getFilters;

  // Format buttons
  const formatBtns = $('#export-formats').querySelectorAll('.export-format-btn');
  formatBtns.forEach((btn) => {
    btn.addEventListener('click', () => {
      formatBtns.forEach((b) => b.classList.remove('active'));
      btn.classList.add('active');
      selectedFormat = btn.dataset.format;
      updateHint();
    });
  });

  // Scope radios
  const scopeRadios = document.querySelectorAll('input[name="export-scope"]');
  scopeRadios.forEach((radio) => {
    radio.addEventListener('change', () => {
      selectedScope = radio.value;
      updateHint();
    });
  });

  // Download button
  $('#btn-export-download').addEventListener('click', () => {
    handleExport();
  });

  // Cancel / close
  $('#btn-export-cancel').addEventListener('click', closeExportModal);
  $('#btn-export-close').addEventListener('click', closeExportModal);

  // Click overlay backdrop to close
  $('#export-overlay').addEventListener('click', (e) => {
    if (e.target === $('#export-overlay')) {
      closeExportModal();
    }
  });

  // Keyboard: Esc to close, Enter to download
  $('#export-overlay').addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      closeExportModal();
    }
    if (e.key === 'Enter' && !$('#btn-export-download').disabled) {
      e.preventDefault();
      handleExport();
    }
  });
}

/**
 * Open the export modal dialog.
 */
export function openExportModal() {
  // Reset state
  selectedFormat = 'png';
  selectedScope = 'current';

  const formatBtns = $('#export-formats').querySelectorAll('.export-format-btn');
  formatBtns.forEach((b) => b.classList.remove('active'));
  formatBtns[0].classList.add('active');

  const scopeRadios = document.querySelectorAll('input[name="export-scope"]');
  scopeRadios[0].checked = true;

  setLoading(false);
  updateHint();

  $('#export-overlay').classList.remove('hidden');

  // Focus the download button for keyboard navigation
  requestAnimationFrame(() => {
    $('#btn-export-download').focus();
  });
}

/**
 * Close the export modal dialog.
 */
export function closeExportModal() {
  $('#export-overlay').classList.add('hidden');
}

/**
 * Update the hint text based on current format+scope selection.
 */
function updateHint() {
  const key = HINTS[`${selectedScope}:${selectedFormat}`] || 'export.hint.currentPng';
  $('#export-hint').textContent = t(key);
}

/**
 * Set loading state on the download button.
 * @param {boolean} loading
 */
function setLoading(loading) {
  const btn = $('#btn-export-download');
  const text = $('#export-download-text');
  const spinner = $('#export-spinner');

  btn.disabled = loading;
  text.textContent = loading ? t('export.downloading') : t('export.download');
  spinner.classList.toggle('hidden', !loading);
}

/**
 * Handle the export action — dispatches to frontend or backend.
 */
async function handleExport() {
  if (!cyRef) return;

  // Frontend export: current view + PNG or SVG
  if (selectedScope === 'current' && (selectedFormat === 'png' || selectedFormat === 'svg')) {
    exportFrontend(selectedFormat);
    return;
  }

  // Backend export: everything else
  await exportBackend(selectedFormat, selectedScope);
}

/**
 * Export using Cytoscape's built-in methods (frontend rendering).
 * @param {'png'|'svg'} format
 */
function exportFrontend(format) {
  try {
    const bg = document.documentElement.dataset.theme === 'dark' ? '#1e1e1e' : '#ffffff';
    let dataUrl;
    let mimeType;

    if (format === 'png') {
      dataUrl = cyRef.png({ full: true, scale: 2, bg });
      mimeType = 'image/png';
    } else {
      dataUrl = cyRef.svg({ full: true, bg });
      // cy.svg() returns SVG string, not data URL
      const blob = new Blob([dataUrl], { type: 'image/svg+xml' });
      dataUrl = URL.createObjectURL(blob);
      mimeType = 'image/svg+xml';
    }

    const a = document.createElement('a');
    a.href = dataUrl;
    a.download = `dephealth-topology-${Date.now()}.${format}`;
    a.click();

    // Clean up object URL for SVG
    if (mimeType === 'image/svg+xml') {
      setTimeout(() => URL.revokeObjectURL(dataUrl), 1000);
    }

    closeExportModal();
    showToast(t('toast.exported', { format: format.toUpperCase() }), 'success');
  } catch (err) {
    console.error('Frontend export failed:', err);
    showToast(t('toast.exportFailed', { error: err.message }), 'error');
  }
}

/**
 * Export via backend API.
 * @param {string} format - json, csv, dot, png, svg
 * @param {string} scope - current, full
 */
async function exportBackend(format, scope) {
  setLoading(true);

  try {
    const params = new URLSearchParams({ scope });

    if (scope === 'current' && getFiltersRef) {
      const filters = getFiltersRef();
      if (filters.namespace) params.set('namespace', filters.namespace);
      if (filters.group) params.set('group', filters.group);
    }

    const resp = await fetch(`/api/v1/export/${format}?${params}`);

    if (!resp.ok) {
      let errMsg = `HTTP ${resp.status}`;
      try {
        const errBody = await resp.json();
        if (errBody.error) errMsg = errBody.error;
      } catch { /* ignore parse error */ }
      throw new Error(errMsg);
    }

    // Get filename from Content-Disposition or generate one
    const disposition = resp.headers.get('Content-Disposition');
    let filename = `dephealth-topology-${Date.now()}.${format === 'csv' ? 'zip' : format}`;
    if (disposition) {
      const match = disposition.match(/filename="?([^"]+)"?/);
      if (match) filename = match[1];
    }

    const blob = await resp.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);

    closeExportModal();
    showToast(t('toast.exported', { format: format.toUpperCase() }), 'success');
  } catch (err) {
    console.error('Backend export failed:', err);
    showToast(t('toast.exportFailed', { error: err.message }), 'error');
  } finally {
    setLoading(false);
  }
}
