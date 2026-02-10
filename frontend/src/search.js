/**
 * Node search functionality with highlight and navigation.
 */

const $ = (sel) => document.querySelector(sel);

let cy = null;
let searchActive = false;
let matchedNodes = [];
let currentMatchIndex = -1;

/**
 * Initialize search panel and interactions.
 * @param {cytoscape.Core} cyInstance - Cytoscape instance
 */
export function initSearch(cyInstance) {
  cy = cyInstance;

  const panel = $('#search-panel');
  const input = $('#search-input');
  const btnToggle = $('#btn-search');
  const btnClose = $('#btn-search-close');

  // Toggle search panel
  btnToggle.addEventListener('click', () => {
    const isHidden = panel.classList.toggle('hidden');
    if (!isHidden) {
      input.focus();
    } else {
      closeSearch();
    }
  });

  // Close button
  btnClose.addEventListener('click', () => {
    panel.classList.add('hidden');
    closeSearch();
  });

  // Input event: filter and highlight
  input.addEventListener('input', () => {
    performSearch(input.value.trim());
  });

  // Enter: navigate to next match
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      navigateToNext();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      panel.classList.add('hidden');
      closeSearch();
    }
  });
}

/**
 * Perform search and update highlights.
 * @param {string} query - Search query
 */
function performSearch(query) {
  if (!cy) return;

  if (!query) {
    // Empty query: restore all nodes
    cy.nodes().style('opacity', 1);
    cy.edges().style('opacity', 1);
    matchedNodes = [];
    currentMatchIndex = -1;
    updateCount();
    searchActive = false;
    return;
  }

  searchActive = true;
  const lowerQuery = query.toLowerCase();

  // Find matching nodes (case-insensitive match on label or id)
  matchedNodes = cy.nodes().filter((node) => {
    const label = (node.data('label') || '').toLowerCase();
    const id = (node.data('id') || '').toLowerCase();
    return label.includes(lowerQuery) || id.includes(lowerQuery);
  });

  // Update opacity
  cy.nodes().forEach((node) => {
    if (matchedNodes.includes(node)) {
      node.style('opacity', 1);
    } else {
      node.style('opacity', 0.15);
    }
  });

  // Fade edges based on node visibility
  cy.edges().forEach((edge) => {
    const source = edge.source();
    const target = edge.target();
    const bothVisible =
      matchedNodes.includes(source) && matchedNodes.includes(target);
    edge.style('opacity', bothVisible ? 1 : 0.15);
  });

  currentMatchIndex = matchedNodes.length > 0 ? 0 : -1;
  updateCount();
}

/**
 * Navigate to next match on Enter.
 */
function navigateToNext() {
  if (matchedNodes.length === 0) return;

  currentMatchIndex = (currentMatchIndex + 1) % matchedNodes.length;
  const node = matchedNodes[currentMatchIndex];

  cy.animate({
    center: { eles: node },
    zoom: 1.5,
    duration: 300,
  });

  updateCount();
}

/**
 * Update match count display.
 */
function updateCount() {
  const countEl = $('#search-count');
  if (matchedNodes.length === 0) {
    countEl.textContent = searchActive ? 'No matches' : '';
  } else {
    const total = cy.nodes().length;
    const current = currentMatchIndex >= 0 ? currentMatchIndex + 1 : 1;
    countEl.textContent = `${current} / ${matchedNodes.length} of ${total}`;
  }
}

/**
 * Close search and restore all nodes.
 */
function closeSearch() {
  if (!cy) return;
  cy.nodes().style('opacity', 1);
  cy.edges().style('opacity', 1);
  matchedNodes = [];
  currentMatchIndex = -1;
  searchActive = false;
  const input = $('#search-input');
  if (input) input.value = '';
  updateCount();
}
