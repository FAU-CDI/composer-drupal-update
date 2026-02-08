import { describe, it, expect, vi, beforeEach } from "vitest";
import { postJSON, getJSON, parseComposer, fetchReleases, updateComposer, buildVersionMap, buildComposerCommands } from "./api.js";

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
      packages: [
        { name: "drupal/gin", module: "gin", version: "^5.0" },
      ],
    };
    global.fetch = mockFetch(200, mockResponse);

    const data = await parseComposer({ require: { "drupal/gin": "^5.0" } });

    expect(data.packages).toHaveLength(1);
    expect(data.packages[0].module).toBe("gin");

    // Verify the request was sent to the right URL
    const [url] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/parse");
  });
});

// =============================================================================
// fetchReleases
// =============================================================================

describe("fetchReleases", () => {
  it("calls /api/releases with the module name", async () => {
    const mockResponse = {
      module: "gin",
      releases: [
        { name: "gin 5.0.3", version: "5.0.3", core_compatibility: "^10 || ^11" },
      ],
    };
    global.fetch = mockFetch(200, mockResponse);

    const data = await fetchReleases("gin");

    expect(data.module).toBe("gin");
    expect(data.releases).toHaveLength(1);

    const [url] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/releases?module=gin");
  });

  it("encodes special characters in the module name", async () => {
    global.fetch = mockFetch(200, { module: "foo_bar", releases: [] });

    await fetchReleases("foo_bar");

    const [url] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/releases?module=foo_bar");
  });
});

// =============================================================================
// updateComposer
// =============================================================================

describe("updateComposer", () => {
  it("calls /api/update with composer_json and versions", async () => {
    const mockResponse = {
      composer_json: { require: { "drupal/gin": "^6.0" } },
    };
    global.fetch = mockFetch(200, mockResponse);

    const data = await updateComposer(
      { require: { "drupal/gin": "^5.0" } },
      { "drupal/gin": "^6.0" },
    );

    expect(data.composer_json.require["drupal/gin"]).toBe("^6.0");

    const [url, opts] = global.fetch.mock.calls[0];
    expect(url).toBe("/api/update");
    const body = JSON.parse(opts.body);
    expect(body.versions).toEqual({ "drupal/gin": "^6.0" });
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
      { name: "drupal/book", version: "^2.0", selectedVersion: "^3.0" },
    ];

    const result = buildVersionMap(packages);

    expect(result).toEqual({
      "drupal/gin": "^6.0",
      "drupal/book": "^3.0",
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
  it("generates require commands followed by update --dry-run", () => {
    const versions = {
      "drupal/gin": "^6.0",
      "drupal/admin_toolbar": "^4.0",
    };

    const commands = buildComposerCommands(versions);

    expect(commands).toHaveLength(3);
    expect(commands[0]).toBe('composer require "drupal/gin:^6.0" --no-update');
    expect(commands[1]).toBe('composer require "drupal/admin_toolbar:^4.0" --no-update');
    expect(commands[2]).toBe("composer update --dry-run");
  });

  it("returns a single require + update for one change", () => {
    const commands = buildComposerCommands({ "drupal/book": "^3.0" });

    expect(commands).toEqual([
      'composer require "drupal/book:^3.0" --no-update',
      "composer update --dry-run",
    ]);
  });

  it("returns empty array when no versions given", () => {
    expect(buildComposerCommands({})).toEqual([]);
  });
});
