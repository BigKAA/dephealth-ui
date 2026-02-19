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
 * Returns white for unknown namespace (e.g. non-Kubernetes environments).
 * @param {string} namespace
 * @returns {string} CSS color
 */
export function getNamespaceColor(namespace) {
  if (!namespace) return '#ffffff';
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

/**
 * Extract Kubernetes namespace from a dependency host string.
 * Supports K8s DNS patterns:
 *   - <service>.<namespace>.svc
 *   - <service>.<namespace>.svc.cluster.local
 * Returns null for non-K8s hosts (plain hostnames, IPs, external FQDNs).
 * @param {string} host - dependency host (e.g. "redis.dephealth-redis.svc")
 * @returns {string|null} namespace or null if not detectable
 */
export function extractNamespaceFromHost(host) {
  if (!host) return null;
  const parts = host.split('.');
  const svcIdx = parts.indexOf('svc');
  if (svcIdx >= 2) {
    return parts[svcIdx - 1];
  }
  return null;
}

/**
 * Return a contrasting text color (white or dark) for a given hex background.
 * Uses relative luminance (WCAG formula).
 * @param {string} hex - CSS hex color (e.g. "#2196f3")
 * @returns {string} '#fff' or '#1e1e1e'
 */
export function getContrastTextColor(hex) {
  if (!hex || hex.length < 7) return '#fff';
  const r = parseInt(hex.slice(1, 3), 16) / 255;
  const g = parseInt(hex.slice(3, 5), 16) / 255;
  const b = parseInt(hex.slice(5, 7), 16) / 255;
  // sRGB → linear
  const lr = r <= 0.03928 ? r / 12.92 : ((r + 0.055) / 1.055) ** 2.4;
  const lg = g <= 0.03928 ? g / 12.92 : ((g + 0.055) / 1.055) ** 2.4;
  const lb = b <= 0.03928 ? b / 12.92 : ((b + 0.055) / 1.055) ** 2.4;
  const luminance = 0.2126 * lr + 0.7152 * lg + 0.0722 * lb;
  return luminance > 0.179 ? '#1e1e1e' : '#fff';
}

// Cache for base64-encoded SVG stripe images (color -> data URI)
const stripeCache = {};

/**
 * Get a base64-encoded SVG data URI for a 1x1 colored pixel.
 * Used as background-image for namespace stripes on graph nodes.
 * @param {string} color - CSS hex color (e.g. "#2196f3")
 * @returns {string} data URI
 */
export function getStripeDataUri(color) {
  if (stripeCache[color]) return stripeCache[color];
  const svg = `<svg xmlns='http://www.w3.org/2000/svg' width='1' height='1'><rect width='1' height='1' fill='${color}'/></svg>`;
  const uri = 'data:image/svg+xml;base64,' + btoa(svg);
  stripeCache[color] = uri;
  return uri;
}
