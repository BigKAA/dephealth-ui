/**
 * Node search functionality with highlight and navigation.
 */

import { t } from './i18n.js';

const $ = (sel) => document.querySelector(sel);

let cy = null;
let searchActive = false;
let matchedNodes = [];
let currentMatchIndex = -1;
let visibleElements = null; // Set of visible nodes and edges during search

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

  // Collect all downstream nodes and edges using Cytoscape graph traversal
  const visibleNodes = new Set();
  const visibleEdges = new Set();
  
  // For each matched node, get all descendants (downstream)
  matchedNodes.forEach((node) => {
    // Add the matched node itself
    visibleNodes.add(node);
    
    // Get all descendants using Cytoscape's breadth-first search
    // This traverses all outgoing edges recursively
    const descendants = node.successors();
    
    descendants.forEach((element) => {
      if (element.isNode()) {
        visibleNodes.add(element);
      } else if (element.isEdge()) {
        visibleEdges.add(element);
      }
    });
  });

  // Store visible elements globally for badge filtering
  visibleElements = new Set([...visibleNodes, ...visibleEdges]);
  
  // Update opacity for nodes
  cy.nodes().forEach((node) => {
    if (visibleNodes.has(node)) {
      node.style('opacity', 1);
    } else {
      node.style('opacity', 0.15);
    }
  });

  // Update opacity for edges (only visible edges from downstream collection)
  cy.edges().forEach((edge) => {
    if (visibleEdges.has(edge)) {
      edge.style('opacity', 1);
    } else {
      edge.style('opacity', 0.15);
    }
  });

  currentMatchIndex = matchedNodes.length > 0 ? 0 : -1;
  updateCount();

  // Trigger badge update (badges will be filtered by opacity in graph.js)
  cy.trigger('render');
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
    countEl.textContent = searchActive ? t('search.noMatches') : '';
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
  visibleElements = null; // Clear visible elements filter
  const input = $('#search-input');
  if (input) input.value = '';
  updateCount();
  
  // Trigger badge update to restore all badges
  cy.trigger('render');
}

/**
 * Check if an element (node or edge) is visible in current search.
 * Returns true if search is inactive (all visible) or if element is in visible set.
 * @param {cytoscape.NodeSingular|cytoscape.EdgeSingular} element
 * @returns {boolean}
 */
export function isElementVisible(element) {
  // Check if search filter is active
  if (searchActive && visibleElements) {
    if (!visibleElements.has(element)) {
      return false;
    }
  }
  
  // Check Cytoscape visibility (for SERVICE and other filters)
  return element.visible();
}
