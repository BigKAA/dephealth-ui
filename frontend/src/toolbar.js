import { makeDraggable } from './draggable.js';

/**
 * Initialize drag & drop for the floating graph toolbar.
 * Saves position to localStorage and restores it on init.
 */
export function initToolbar() {
  const toolbar = document.getElementById('graph-toolbar');
  if (!toolbar) return;

  makeDraggable(toolbar, 'dephealth-toolbar-pos', { dragHandle: '.toolbar-grip' });
}
