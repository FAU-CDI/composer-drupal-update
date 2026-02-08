// =============================================================================
// API Client
// =============================================================================
// Pure functions for talking to the backend. No DOM access.
// Used by app.js and testable independently.

/**
 * POST JSON to a URL and return the parsed response.
 * @param {string} url
 * @param {object} body
 * @returns {Promise<object>}
 */
export async function postJSON(url, body) {
  const resp = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await resp.json();
  if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
  return data;
}

/**
 * GET JSON from a URL and return the parsed response.
 * @param {string} url
 * @returns {Promise<object>}
 */
export async function getJSON(url) {
  const resp = await fetch(url);
  const data = await resp.json();
  if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
  return data;
}

/**
 * Call POST /api/parse with a composer.json object.
 * Returns the list of drupal packages.
 * @param {object} composerJSON
 * @returns {Promise<{packages: Array<{name: string, module: string, version: string}>}>}
 */
export async function parseComposer(composerJSON) {
  return postJSON("/api/parse", { composer_json: composerJSON });
}

/**
 * Call GET /api/releases?module=... to fetch releases for a module.
 * @param {string} moduleName
 * @returns {Promise<{module: string, releases: Array<{name: string, version: string, core_compatibility: string}>}>}
 */
export async function fetchReleases(moduleName) {
  return getJSON("/api/releases?module=" + encodeURIComponent(moduleName));
}

/**
 * Call POST /api/update to apply version changes to a composer.json.
 * @param {object} composerJSON
 * @param {Object<string, string>} versions - map of package name to new version
 * @returns {Promise<{composer_json: object}>}
 */
export async function updateComposer(composerJSON, versions) {
  return postJSON("/api/update", { composer_json: composerJSON, versions });
}

/**
 * Build a version map from packages and their selected values.
 * Only includes entries where the selected version differs from the current one.
 * @param {Array<{name: string, version: string, selectedVersion: string}>} packages
 * @returns {Object<string, string>}
 */
export function buildVersionMap(packages) {
  const versions = {};
  for (const pkg of packages) {
    if (pkg.selectedVersion && pkg.selectedVersion !== pkg.version) {
      versions[pkg.name] = pkg.selectedVersion;
    }
  }
  return versions;
}
