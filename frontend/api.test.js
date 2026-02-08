import { describe, it, expect, vi, beforeEach } from "vitest";
import { postJSON, getJSON, parseComposer, fetchReleases, updateComposer, buildVersionMap, buildComposerCommands, buildDryRunCommand } from "./api.js";

// =============================================================================
// Mock fetch
// =============================================================================

function mockFetch(status, body) {
  return vi.fn(() =>
    Promise.resolve({
      ok: status >= 200 && status < 300,
      status,
      json: () => Promise.resolve(body),
    })
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
});

// =============================================================================
// postJSON
// =============================================================================

describe("postJSON", () => {
  it("sends a POST request and returns parsed JSON", async () => {
    global.fetch = mockFetch(200, { result: "ok" });

    const data = await postJSON("/test", { key: "value" });

    expect(data).toEqual({ result: "ok" });
    expect(global.fetch).toHaveBeenCalledWith("/test", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: "value" }),
    });
  });

  it("throws on non-ok response", async () => {
    global.fetch = mockFetch(400, { error: "bad request" });

    await expect(postJSON("/test", {})).rejects.toThrow("bad request");
  });
});

// =============================================================================
// getJSON
// =============================================================================

describe("getJSON", () => {
  it("sends a GET request and returns parsed JSON", async () => {
    global.fetch = mockFetch(200, { data: 42 });

    const data = await getJSON("/test");

    expect(data).toEqual({ data: 42 });
    expect(global.fetch).toHaveBeenCalledWith("/test");
  });

  it("throws on non-ok response", async () => {
    global.fetch = mockFetch(502, { error: "upstream failed" });

    await expect(getJSON("/test")).rejects.toThrow("upstream failed");
  });
});

// =============================================================================
// parseComposer
// =============================================================================

describe("parseComposer", () => {
  it("calls /api/parse with the composer_json", async () => {
    const mockResponse = {
      drupal_packages: [
        { name: "drupal/gin", module: "gin", version: "^5.0" },
      ],
      composer_packages: [
        { name: "drush/drush", module: "drush/drush", version: "^13" },
      ],
    };
    global.fetch = mockFetch(200, mockResponse);

    const data = await parseComposer({ require: { "drupal/gin": "^5.0", "drush/drush": "^13" } });

    expect(data.drupal_packages).toHaveLength(1);
    expect(data.drupal_packages[0].module).toBe("gin");
    expect(data.composer_packages).toHaveLength(1);
    expect(data.composer_packages[0].name).toBe("drush/drush");

    // Verify the request was sent to the right URL
    const [url] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/parse");
  });
});

// =============================================================================
// fetchReleases
// =============================================================================

describe("fetchReleases", () => {
  it("calls /api/releases with the full package name", async () => {
    const mockResponse = {
      package: "drupal/gin",
      releases: [
        { name: "gin 5.0.3", version: "5.0.3", core_compatibility: "^10 || ^11" },
      ],
    };
    global.fetch = mockFetch(200, mockResponse);

    const data = await fetchReleases("drupal/gin");

    expect(data.package).toBe("drupal/gin");
    expect(data.releases).toHaveLength(1);

    const [url] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/releases?package=drupal%2Fgin");
  });

  it("works for non-Drupal packages", async () => {
    global.fetch = mockFetch(200, {
      package: "drush/drush",
      releases: [{ name: "drush/drush 13.0.1", version: "13.0.1" }],
    });

    const data = await fetchReleases("drush/drush");

    expect(data.package).toBe("drush/drush");
    const [url] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/releases?package=drush%2Fdrush");
  });
});

// =============================================================================
// updateComposer
// =============================================================================

describe("updateComposer", () => {
  it("calls /api/update with composer_json and versions", async () => {
    const mockResponse = {
      composer_json: { require: { "drupal/gin": "^6.0", "drush/drush": "^13" } },
    };
    global.fetch = mockFetch(200, mockResponse);

    const data = await updateComposer(
      { require: { "drupal/gin": "^5.0", "drush/drush": "^12" } },
      { "drupal/gin": "^6.0", "drush/drush": "^13" },
    );

    expect(data.composer_json.require["drupal/gin"]).toBe("^6.0");
    expect(data.composer_json.require["drush/drush"]).toBe("^13");

    const [url, opts] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/update");
    const body = JSON.parse(opts.body);
    expect(body.versions).toEqual({ "drupal/gin": "^6.0", "drush/drush": "^13" });
  });
});

// =============================================================================
// buildVersionMap
// =============================================================================

describe("buildVersionMap", () => {
  it("returns only changed versions", () => {
    const packages = [
      { name: "drupal/gin", version: "^5.0", selectedVersion: "^6.0" },
      { name: "drupal/admin_toolbar", version: "^3.6", selectedVersion: "^3.6" },
      { name: "drush/drush", version: "^12", selectedVersion: "^13" },
    ];

    const result = buildVersionMap(packages);

    expect(result).toEqual({
      "drupal/gin": "^6.0",
      "drush/drush": "^13",
    });
  });

  it("returns empty object when nothing changed", () => {
    const packages = [
      { name: "drupal/gin", version: "^5.0", selectedVersion: "^5.0" },
    ];

    expect(buildVersionMap(packages)).toEqual({});
  });

  it("handles missing selectedVersion", () => {
    const packages = [
      { name: "drupal/gin", version: "^5.0", selectedVersion: undefined },
    ];

    expect(buildVersionMap(packages)).toEqual({});
  });

  it("returns empty object for empty input", () => {
    expect(buildVersionMap([])).toEqual({});
  });
});

// =============================================================================
// buildComposerCommands
// =============================================================================

describe("buildComposerCommands", () => {
  it("generates require commands followed by composer update", () => {
    const versions = {
      "drupal/gin": "^6.0",
      "drush/drush": "^13",
    };

    const commands = buildComposerCommands(versions);

    expect(commands).toHaveLength(3);
    expect(commands[0]).toBe('composer require "drupal/gin:^6.0" --no-update');
    expect(commands[1]).toBe('composer require "drush/drush:^13" --no-update');
    expect(commands[2]).toBe("composer update --with-all-dependencies");
  });

  it("returns a single require + update for one change", () => {
    const commands = buildComposerCommands({ "drupal/book": "^3.0" });

    expect(commands).toEqual([
      'composer require "drupal/book:^3.0" --no-update',
      "composer update --with-all-dependencies",
    ]);
  });

  it("returns empty array when no versions given", () => {
    expect(buildComposerCommands({})).toEqual([]);
  });
});

// =============================================================================
// buildDryRunCommand
// =============================================================================

describe("buildDryRunCommand", () => {
  it("generates a single require command with --dry-run", () => {
    const versions = {
      "drupal/gin": "^6.0",
      "drush/drush": "^13",
    };

    const cmd = buildDryRunCommand(versions);

    expect(cmd).toBe('composer require "drupal/gin:^6.0" "drush/drush:^13" --dry-run --with-all-dependencies');
  });

  it("works with a single package", () => {
    const cmd = buildDryRunCommand({ "drupal/book": "^3.0" });
    expect(cmd).toBe('composer require "drupal/book:^3.0" --dry-run --with-all-dependencies');
  });

  it("returns empty string when no versions given", () => {
    expect(buildDryRunCommand({})).toBe("");
  });
});
