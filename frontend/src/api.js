/**
 * Fetch topology data from the backend API.
 * @returns {Promise<{nodes: Array, edges: Array, alerts: Array, meta: Object}>}
 */
export async function fetchTopology() {
  const resp = await fetch('/api/v1/topology');
  if (!resp.ok) {
    throw new Error(`Topology API error: ${resp.status} ${resp.statusText}`);
  }
  return resp.json();
}

/**
 * Fetch frontend configuration from the backend API.
 * @returns {Promise<{grafana: Object, cache: {ttl: number}}>}
 */
export async function fetchConfig() {
  const resp = await fetch('/api/v1/config');
  if (!resp.ok) {
    throw new Error(`Config API error: ${resp.status} ${resp.statusText}`);
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
