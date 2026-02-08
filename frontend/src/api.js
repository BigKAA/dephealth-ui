/**
 * Wrapper around fetch that handles 401 responses by redirecting to OIDC login.
 * @param {string} url
 * @param {RequestInit} [opts]
 * @returns {Promise<Response>}
 */
async function authenticatedFetch(url, opts) {
  const resp = await fetch(url, opts);
  if (resp.status === 401) {
    window.location.href = '/auth/login';
    return new Promise(() => {}); // never resolves — page is redirecting
  }
  return resp;
}

/**
 * Fetch topology data from the backend API.
 * @returns {Promise<{nodes: Array, edges: Array, alerts: Array, meta: Object}>}
 */
export async function fetchTopology() {
  const resp = await authenticatedFetch('/api/v1/topology');
  if (!resp.ok) {
    throw new Error(`Topology API error: ${resp.status} ${resp.statusText}`);
  }
  return resp.json();
}

/**
 * Fetch frontend configuration from the backend API.
 * @returns {Promise<{grafana: Object, cache: {ttl: number}, auth: {type: string}}>}
 */
export async function fetchConfig() {
  const resp = await authenticatedFetch('/api/v1/config');
  if (!resp.ok) {
    throw new Error(`Config API error: ${resp.status} ${resp.statusText}`);
  }
  return resp.json();
}

/**
 * Fetch current user info from the OIDC session.
 * @returns {Promise<{sub: string, name: string, email: string}|null>}
 */
export async function fetchUserInfo() {
  const resp = await fetch('/auth/userinfo');
  if (!resp.ok) {
    return null;
  }
  return resp.json();
}

/**
 * Retry a function with exponential backoff.
 * @param {Function} fn - async function to retry
 * @param {number} maxRetries - max number of retries (default 3)
 * @returns {Promise<*>} result of fn
 */
export async function withRetry(fn, maxRetries = 3) {
  let lastError;
  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      return await fn();
    } catch (err) {
      lastError = err;
      if (attempt < maxRetries) {
        const delay = Math.min(1000 * 2 ** attempt, 5000);
        await new Promise((r) => setTimeout(r, delay));
      }
    }
  }
  throw lastError;
}
