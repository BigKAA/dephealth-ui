/**
 * Shared utility functions.
 */

/**
 * Escape a string for safe insertion into HTML.
 * Prevents XSS when rendering API data via innerHTML.
 * @param {*} str - Value to escape (non-strings converted via String())
 * @returns {string}
 */
export function escapeHtml(str) {
  if (typeof str !== 'string') return String(str);
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}
