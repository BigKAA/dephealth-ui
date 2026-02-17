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

// ETag tracking for topology endpoint
let lastETag = null;
let lastTopologyData = null;

/**
 * Fetch topology data from the backend API.
 * Supports ETag/If-None-Match for efficient polling (disabled for namespace-filtered
 * and historical requests).
 * @param {string} [namespace] - optional namespace filter
 * @param {string} [time] - optional ISO8601 timestamp for historical queries
 * @returns {Promise<{nodes: Array, edges: Array, alerts: Array, meta: Object}>}
 */
export async function fetchTopology(namespace, time) {
  let url = '/api/v1/topology';
  const params = new URLSearchParams();
  if (namespace) params.set('namespace', namespace);
  if (time) params.set('time', time);
  const qs = params.toString();
  if (qs) url += `?${qs}`;

  const headers = {};
  // ETag only for unfiltered live requests.
  if (!namespace && !time && lastETag) {
    headers['If-None-Match'] = lastETag;
  }

  const resp = await authenticatedFetch(url, { headers });

  if (resp.status === 304 && lastTopologyData) {
    return lastTopologyData;
  }

  if (!resp.ok) {
    throw new Error(`Topology API error: ${resp.status} ${resp.statusText}`);
  }

  const data = await resp.json();
  // Only track ETag for unfiltered live requests.
  if (!namespace && !time) {
    const etag = resp.headers.get('ETag');
    if (etag) {
      lastETag = etag;
    }
    lastTopologyData = data;
  }
  return data;
}

/**
 * Fetch timeline events (status transitions) for a given time range.
 * @param {string} start - ISO8601 start timestamp
 * @param {string} end - ISO8601 end timestamp
 * @returns {Promise<Array<{timestamp: string, service: string, fromState: string, toState: string, kind: string}>>}
 */
export async function fetchTimelineEvents(start, end) {
  const url = `/api/v1/timeline/events?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`;
  const resp = await authenticatedFetch(url);
  if (!resp.ok) {
    throw new Error(`Timeline events API error: ${resp.status} ${resp.statusText}`);
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
 * Fetch instances (pods/containers) for a given service.
 * @param {string} serviceName - service name to query instances for
 * @returns {Promise<Array<{instance: string, pod?: string, job?: string}>>}
 */
export async function fetchInstances(serviceName) {
  const url = `/api/v1/instances?service=${encodeURIComponent(serviceName)}`;
  const resp = await authenticatedFetch(url);
  if (!resp.ok) {
    throw new Error(`Instances API error: ${resp.status} ${resp.statusText}`);
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
