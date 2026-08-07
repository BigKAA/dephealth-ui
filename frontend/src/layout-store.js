// Position persistence for graph layout.
// Keeps an in-memory cache backed by localStorage with debounced writes.

const STORAGE_KEY = 'dephealth-node-positions';
const DEBOUNCE_MS = 300;

/** @type {Object<string, {x: number, y: number, manual: boolean}>|null} */
let cache = null;
let writeTimer = null;

/**
 * Read positions from localStorage into cache (once).
 */
function ensureCache() {
  if (cache !== null) return;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    cache = raw ? JSON.parse(raw) : {};
  } catch {
    cache = {};
  }
}

/**
 * Schedule a debounced write of the cache to localStorage.
 */
function scheduleSave() {
  if (writeTimer) clearTimeout(writeTimer);
  writeTimer = setTimeout(() => {
    writeTimer = null;
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(cache));
    } catch {
      // localStorage full or unavailable — silently ignore
    }
  }, DEBOUNCE_MS);
}

/**
 * Flush pending debounced write immediately.
 */
function flushSave() {
  if (writeTimer) {
    clearTimeout(writeTimer);
    writeTimer = null;
  }
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(cache));
  } catch {
    // ignore
  }
}

/**
 * Get all saved positions.
 * @returns {Object<string, {x: number, y: number, manual: boolean}>}
 */
export function getSavedPositions() {
  ensureCache();
  return cache;
}

/**
 * Check if any saved positions exist.
 * @returns {boolean}
 */
export function hasSavedPositions() {
  ensureCache();
  return Object.keys(cache).length > 0;
}

/**
 * Mark a node as manually positioned and save its coords.
 * Skips save if position unchanged (no-op drag guard handled by caller).
 * @param {string} nodeId
 * @param {{x: number, y: number}} position
 */
export function markManualPosition(nodeId, position) {
  ensureCache();
  cache[nodeId] = { x: position.x, y: position.y, manual: true };
  scheduleSave();
}

/**
 * Mark multiple nodes as manually positioned (group drag).
 * @param {Array<{id: string, x: number, y: number}>} nodes
 */
export function markManualPositions(nodes) {
  ensureCache();
  for (const n of nodes) {
    cache[n.id] = { x: n.x, y: n.y, manual: true };
  }
  scheduleSave();
}

/**
 * Clear all saved positions (reset layout).
 */
export function clearSavedPositions() {
  cache = {};
  if (writeTimer) {
    clearTimeout(writeTimer);
    writeTimer = null;
  }
  localStorage.removeItem(STORAGE_KEY);
}

/**
 * Clear only the manual flag on all saved positions.
 * Called on direction change (TB->LR) so ELK can recalculate everything.
 */
export function clearManualFlags() {
  ensureCache();
  for (const id of Object.keys(cache)) {
    cache[id].manual = false;
  }
  // No need to persist — positions will be overwritten by saveAutoPositions after ELK
}

/**
 * Remove positions for nodes that no longer exist in topology.
 * @param {Set<string>} currentNodeIds - IDs of nodes in current data
 */
export function pruneStalePositions(currentNodeIds) {
  ensureCache();
  let changed = false;
  for (const id of Object.keys(cache)) {
    if (!currentNodeIds.has(id)) {
      delete cache[id];
      changed = true;
    }
  }
  if (changed) scheduleSave();
}

/**
 * Apply saved positions to Cytoscape nodes (preset mode).
 * Returns set of node IDs that have no saved position (need layout).
 * Parent/compound nodes are skipped (their position is derived from children).
 * @param {cytoscape.Core} cy
 * @returns {Set<string>} nodeIds without saved positions
 */
export function applySavedPositions(cy) {
  ensureCache();
  const unpositioned = new Set();

  cy.nodes().forEach((node) => {
    if (node.isParent()) return; // compound parents auto-size

    const id = node.id();
    const saved = cache[id];
    if (saved) {
      node.position({ x: saved.x, y: saved.y });
    } else {
      unpositioned.add(id);
    }
  });

  return unpositioned;
}

/**
 * Save all current node positions as auto-positioned (after ELK layout).
 * Does NOT overwrite existing manual=true positions.
 * Parent/compound nodes are skipped.
 * @param {cytoscape.Core} cy
 */
export function saveAutoPositions(cy) {
  ensureCache();
  cy.nodes().forEach((node) => {
    if (node.isParent()) return;

    const id = node.id();
    const existing = cache[id];
    // Preserve manual positions — only save if not manual
    if (existing && existing.manual) return;

    const pos = node.position();
    cache[id] = { x: pos.x, y: pos.y, manual: false };
  });
  flushSave();
}
