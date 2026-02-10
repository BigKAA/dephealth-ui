// Namespace color utilities for graph visualization

// 16 visually distinct colors that work in both light and dark themes
const NAMESPACE_PALETTE = [
  '#2196f3', // blue
  '#e91e63', // pink
  '#009688', // teal
  '#ff5722', // deep orange
  '#673ab7', // deep purple
  '#00bcd4', // cyan
  '#8bc34a', // light green
  '#ff9800', // orange
  '#3f51b5', // indigo
  '#cddc39', // lime
  '#795548', // brown
  '#607d8b', // blue grey
  '#9c27b0', // purple
  '#03a9f4', // light blue
  '#ffc107', // amber
  '#4caf50', // green
];

/**
 * Simple string hash function (djb2).
 * @param {string} str
 * @returns {number}
 */
function hashString(str) {
  let hash = 5381;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) + hash + str.charCodeAt(i)) >>> 0;
  }
  return hash;
}

/**
 * Get a deterministic color for a namespace.
 * Same namespace always returns the same color.
 * @param {string} namespace
 * @returns {string} CSS color
 */
export function getNamespaceColor(namespace) {
  if (!namespace) return '#9e9e9e';
  return NAMESPACE_PALETTE[hashString(namespace) % NAMESPACE_PALETTE.length];
}

/**
 * Build a namespace → color map for an array of namespaces.
 * @param {string[]} namespaces
 * @returns {Object.<string, string>}
 */
export function getNamespaceColorMap(namespaces) {
  const map = {};
  for (const ns of namespaces) {
    map[ns] = getNamespaceColor(ns);
  }
  return map;
}
