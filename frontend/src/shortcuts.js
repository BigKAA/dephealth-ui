// Keyboard shortcuts module
// Provides keyboard navigation and actions for the application

import { t } from './i18n.js';

/**
 * Initialize keyboard shortcuts with action callbacks
 * @param {Object} actions - Object with action callback functions
 */
export function initShortcuts(actions) {
  const SHORTCUTS = {
    'r': () => actions.refresh && actions.refresh(),
    'f': () => actions.fit && actions.fit(),
    '+': () => actions.zoomIn && actions.zoomIn(),
    '=': () => actions.zoomIn && actions.zoomIn(), // Alternative for US keyboards
    '-': () => actions.zoomOut && actions.zoomOut(),
    '/': () => actions.openSearch && actions.openSearch(),
    'l': () => actions.toggleLayout && actions.toggleLayout(),
    'e': () => actions.exportPNG && actions.exportPNG(),
    'Escape': () => actions.closeAll && actions.closeAll(),
  };

  // Handle standard shortcuts
  document.addEventListener('keydown', (e) => {
    // Don't trigger shortcuts if typing in input/textarea/select
    if (['INPUT', 'SELECT', 'TEXTAREA'].includes(e.target.tagName)) {
      return;
    }

    // Check for standard shortcuts
    const fn = SHORTCUTS[e.key];
    if (fn) {
      e.preventDefault();
      fn();
      return;
    }

    // Handle Ctrl+K / Meta+K (Cmd+K on Mac) for search
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();
      if (actions.openSearch) {
        actions.openSearch();
      }
    }
  });

  // Log keyboard shortcuts info on '?' key
  document.addEventListener('keydown', (e) => {
    if (e.key === '?' && !['INPUT', 'SELECT', 'TEXTAREA'].includes(e.target.tagName)) {
      e.preventDefault();
      showShortcutsHelp();
    }
  });
}

/**
 * Show keyboard shortcuts help in console
 */
function showShortcutsHelp() {
  const s = 'font-weight: bold; color: #4caf50';
  const n = 'color: inherit';
  console.group(`%c${t('shortcuts.title')}`, 'font-size: 14px; font-weight: bold; color: #2196f3');
  console.log(`%cr%c - ${t('shortcuts.refresh')}`, s, n);
  console.log(`%cf%c - ${t('shortcuts.fit')}`, s, n);
  console.log(`%c+/=%c - ${t('shortcuts.zoomIn')}`, s, n);
  console.log(`%c-%c - ${t('shortcuts.zoomOut')}`, s, n);
  console.log(`%c/%c - ${t('shortcuts.search')}`, s, n);
  console.log(`%cCtrl+K%c - ${t('shortcuts.searchAlt')}`, s, n);
  console.log(`%cl%c - ${t('shortcuts.layout')}`, s, n);
  console.log(`%ce%c - ${t('shortcuts.export')}`, s, n);
  console.log(`%cEsc%c - ${t('shortcuts.closeAll')}`, s, n);
  console.log(`%c?%c - ${t('shortcuts.help')}`, s, n);
  console.groupEnd();
}
