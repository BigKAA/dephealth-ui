// Keyboard shortcuts module
// Provides keyboard navigation and actions for the application

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
  console.group('%cKeyboard Shortcuts', 'font-size: 14px; font-weight: bold; color: #2196f3');
  console.log('%cr%c - Refresh graph', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.log('%cf%c - Fit graph to screen', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.log('%c+/=%c - Zoom in', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.log('%c-%c - Zoom out', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.log('%c/%c - Open search', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.log('%cCtrl+K%c - Open search (alternative)', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.log('%cl%c - Toggle layout direction (TB/LR)', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.log('%ce%c - Export graph as PNG', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.log('%cEsc%c - Close all panels', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.log('%c?%c - Show this help', 'font-weight: bold; color: #4caf50', 'color: inherit');
  console.groupEnd();
}
