// =============================================================================
// API Client
// =============================================================================
// Pure functions for talking to the backend. No DOM access.
// Used by app.js and testable independently.

// =============================================================================
// Type Definitions
// =============================================================================

/**
 * @typedef {Object} Package
 * @property {string} name   - e.g. "drupal/gin" or "drush/drush"
 * @property {string} module - identifier for fetching releases
 * @property {string} version
 */

/**
 * @typedef {Object} Release
 * @property {string} name
 * @property {string} version
 * @property {string} [core_compatibility]
 */

/**
 * @typedef {Object} ParseResponse
 * @property {Package[]} drupal_packages
 * @property {Package[]} composer_packages
 */

/**
 * @typedef {Object} ReleasesResponse
 * @property {string} package
 * @property {Release[]} releases
 */

/**
 * @typedef {Object} UpdateResponse
 * @property {Record<string, any>} composer_json
 */

/**
 * @typedef {Object} VersionSelection
 * @property {string} name
 * @property {string} version
 * @property {string} [selectedVersion]
 */

// =============================================================================
// Generic HTTP Helpers
// =============================================================================

/**
 * POST JSON to a URL and return the parsed response.
 * @param {string} url
 * @param {Record<string, any>} body
 * @returns {Promise<any>}
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
 * @returns {Promise<any>}
 */
export async function getJSON(url) {
  const resp = await fetch(url);
  const data = await resp.json();
  if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
  return data;
}

// =============================================================================
// API Wrappers
// =============================================================================

/**
 * Call POST /api/parse with a composer.json object.
 * Returns Drupal and Composer packages separately.
 * @param {Record<string, any>} composerJSON
 * @returns {Promise<ParseResponse>}
 */
export async function parseComposer(composerJSON) {
  return postJSON("/api/parse", { composer_json: composerJSON });
}

/**
 * Call GET /api/releases?package=... to fetch releases for a package.
 * The server automatically routes to drupal.org or Packagist.
 * @param {string} packageName - full composer package name (e.g. "drupal/gin" or "drush/drush")
 * @returns {Promise<ReleasesResponse>}
 */
export async function fetchReleases(packageName) {
  return getJSON("/api/releases?package=" + encodeURIComponent(packageName));
}

/**
 * Call POST /api/update to apply version changes to a composer.json.
 * @param {Record<string, any>} composerJSON
 * @param {Record<string, string>} versions - map of package name to new version
 * @returns {Promise<UpdateResponse>}
 */
export async function updateComposer(composerJSON, versions) {
  return postJSON("/api/update", { composer_json: composerJSON, versions });
}

// =============================================================================
// Pure Helpers
// =============================================================================

/**
 * Build a version map from packages and their selected values.
 * Only includes entries where the selected version differs from the current one.
 * @param {VersionSelection[]} packages
 * @returns {Record<string, string>}
 */
export function buildVersionMap(packages) {
  /** @type {Record<string, string>} */
  const versions = {};
  for (const pkg of packages) {
    if (pkg.selectedVersion && pkg.selectedVersion !== pkg.version) {
      versions[pkg.name] = pkg.selectedVersion;
    }
  }
  return versions;
}

/**
 * Build a list of composer commands that apply the given requirements.
 * Returns one "composer require" per package, followed by "composer update --dry-run".
 * @param {Record<string, string>} versions - map of package name to version constraint
 * @returns {string[]}
 */
export function buildComposerCommands(versions) {
  /** @type {string[]} */
  const commands = [];
  for (const [pkg, version] of Object.entries(versions)) {
    commands.push(`composer require "${pkg}:${version}" --no-update`);
  }
  if (commands.length > 0) {
    commands.push("composer update --dry-run");
  }
  return commands;
}
