// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";

// =============================================================================
// DOM Template (mirrors index.html structure)
// =============================================================================

const HTML = `
  <h1>Composer Drupal Update</h1>
  <div class="tabs">
    <button class="tab active" data-tab="tab-json" data-text="composer.json">composer.json</button>
    <button class="tab" data-tab="tab-packages" data-text="Packages (*)">Packages</button>
    <button class="tab" data-tab="tab-commands" data-text="Commands">Commands</button>
    <button class="tab" data-tab="tab-help" data-text="Help">Help</button>
  </div>
  <div id="tab-json" class="tab-panel active">
    <textarea id="composer-textarea" rows="20" readonly></textarea>
    <div class="actions">
      <button id="btn-edit">Edit</button>
      <button id="btn-download">Download</button>
    </div>
    <div id="drop-zone" class="drop-zone">Drop or click to upload</div>
    <input type="file" id="file-input" accept=".json" hidden>
  </div>
  <div id="tab-packages" class="tab-panel">
    <div class="actions">
      <button id="btn-apply" disabled>Apply</button>
    </div>
    <table id="packages-table">
      <thead><tr><th>Package</th><th>Current</th><th>Available</th></tr></thead>
      <tbody id="packages-body">
        <tr><td colspan="3">No composer.json loaded yet.</td></tr>
      </tbody>
    </table>
  </div>
  <div id="tab-commands" class="tab-panel">
    <p id="commands-empty">Load a composer.json to see commands.</p>
    <pre id="commands-output" hidden></pre>
    <div class="commands-actions">
      <button id="btn-copy" hidden>Copy</button>
    </div>
  </div>
  <div id="tab-help" class="tab-panel"><p>Help content</p></div>
  <footer>
    <p id="status"></p>
  </footer>
`;

// =============================================================================
// Helpers
// =============================================================================

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => document.querySelectorAll(sel);

function flushPromises() {
  return new Promise(resolve => setTimeout(resolve, 0));
}

// =============================================================================
// Mock API module
// =============================================================================

let mockParseComposer;
let mockFetchReleases;
let mockUpdateComposer;
let mockBuildVersionMap;
let mockBuildComposerCommands;

beforeEach(async () => {
  vi.resetModules();
  vi.restoreAllMocks();

  document.body.innerHTML = HTML;

  mockParseComposer = vi.fn();
  mockFetchReleases = vi.fn();
  mockUpdateComposer = vi.fn();
  mockBuildVersionMap = vi.fn().mockReturnValue({});
  mockBuildComposerCommands = vi.fn().mockReturnValue([]);

  vi.doMock("./api.js", () => ({
    parseComposer: mockParseComposer,
    fetchReleases: mockFetchReleases,
    updateComposer: mockUpdateComposer,
    buildVersionMap: mockBuildVersionMap,
    buildComposerCommands: mockBuildComposerCommands,
  }));

  // Mock clipboard API
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn(() => Promise.resolve()) },
    writable: true,
    configurable: true,
  });

  // Mock URL methods (jsdom doesn't provide them)
  if (!URL.createObjectURL) {
    URL.createObjectURL = vi.fn(() => "blob:mock");
  }
  if (!URL.revokeObjectURL) {
    URL.revokeObjectURL = vi.fn();
  }

  await import("./app.js");
});

// =============================================================================
// Tab Switching
// =============================================================================

describe("Tab switching", () => {
  it("shows the JSON tab by default", () => {
    expect($("#tab-json").classList.contains("active")).toBe(true);
    expect($("#tab-packages").classList.contains("active")).toBe(false);
    expect($("#tab-commands").classList.contains("active")).toBe(false);
  });

  it("shows only the active tab button as active", () => {
    const activeButtons = $$(".tab.active");
    expect(activeButtons).toHaveLength(1);
    expect(activeButtons[0].dataset.tab).toBe("tab-json");
  });

  it("switches to Packages tab on click", () => {
    $('[data-tab="tab-packages"]').click();

    expect($("#tab-json").classList.contains("active")).toBe(false);
    expect($("#tab-packages").classList.contains("active")).toBe(true);
    expect($("#tab-commands").classList.contains("active")).toBe(false);

    // Tab buttons
    expect($('[data-tab="tab-json"]').classList.contains("active")).toBe(false);
    expect($('[data-tab="tab-packages"]').classList.contains("active")).toBe(true);
  });

  it("switches to Commands tab on click", () => {
    $('[data-tab="tab-commands"]').click();

    expect($("#tab-commands").classList.contains("active")).toBe(true);
    expect($("#tab-json").classList.contains("active")).toBe(false);
    expect($("#tab-packages").classList.contains("active")).toBe(false);
  });

  it("switches back to JSON tab from another tab", () => {
    $('[data-tab="tab-packages"]').click();
    $('[data-tab="tab-json"]').click();

    expect($("#tab-json").classList.contains("active")).toBe(true);
    expect($("#tab-packages").classList.contains("active")).toBe(false);
  });

  it("only one panel is active at a time", () => {
    $('[data-tab="tab-commands"]').click();

    const activePanels = $$(".tab-panel.active");
    expect(activePanels).toHaveLength(1);
    expect(activePanels[0].id).toBe("tab-commands");
  });
});

// =============================================================================
// Edit Toggle
// =============================================================================

describe("Edit toggle", () => {
  it("starts in readonly mode", () => {
    expect($("#composer-textarea").readOnly).toBe(true);
    expect($("#btn-edit").textContent).toBe("Edit");
  });

  it("enters edit mode on first click", () => {
    $("#btn-edit").click();

    expect($("#composer-textarea").readOnly).toBe(false);
    expect($("#btn-edit").textContent).toBe("Done Editing");
  });

  it("disables drop zone while editing", () => {
    $("#btn-edit").click();

    expect($("#drop-zone").classList.contains("disabled")).toBe(true);
  });

  it("disables apply button when entering edit mode", () => {
    $("#btn-apply").disabled = false;
    $("#btn-edit").click();

    expect($("#btn-apply").disabled).toBe(true);
  });

  it("clears the packages table when entering edit mode", () => {
    $("#btn-edit").click();

    expect($("#packages-body").textContent).toContain("No packages found");
  });

  it("exits edit mode and restores readonly on second click", () => {
    const textarea = $("#composer-textarea");
    textarea.value = '{"require":{}}';
    mockParseComposer.mockResolvedValue({ drupal_packages: [], composer_packages: [] });

    // Enter
    $("#btn-edit").click();
    expect(textarea.readOnly).toBe(false);

    // Exit
    $("#btn-edit").click();
    expect(textarea.readOnly).toBe(true);
    expect($("#btn-edit").textContent).toBe("Edit");
    expect($("#drop-zone").classList.contains("disabled")).toBe(false);
  });
});

// =============================================================================
// Status Messages
// =============================================================================

describe("Status messages", () => {
  it("shows editing status on enter", () => {
    $("#btn-edit").click();

    expect($("#status").textContent).toContain("Editing");
    expect($("#status").classList.contains("error")).toBe(false);
  });

  it("shows error for empty textarea on stop editing", () => {
    $("#btn-edit").click();
    $("#composer-textarea").value = "";
    $("#btn-edit").click();

    expect($("#status").textContent).toContain("empty");
    expect($("#status").classList.contains("error")).toBe(true);
  });

  it("shows error for invalid JSON on stop editing", () => {
    $("#btn-edit").click();
    $("#composer-textarea").value = "not json at all";
    $("#btn-edit").click();

    expect($("#status").textContent).toContain("Invalid JSON");
    expect($("#status").classList.contains("error")).toBe(true);
  });
});

// =============================================================================
// Commands Visibility
// =============================================================================

describe("Commands visibility", () => {
  it("shows commands when textarea has valid composer.json with require", () => {
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update --dry-run",
    ]);

    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    // Enter + exit edit mode triggers renderTable -> updateCommands
    mockParseComposer.mockResolvedValue({ drupal_packages: [], composer_packages: [] });
    $("#btn-edit").click();
    $("#btn-edit").click();

    expect($("#commands-output").hidden).toBe(false);
    expect($("#commands-empty").hidden).toBe(true);
    expect($("#btn-copy").hidden).toBe(false);
  });

  it("hides commands when textarea is empty", () => {
    mockBuildComposerCommands.mockReturnValue([]);

    $("#btn-edit").click();
    $("#composer-textarea").value = "";
    $("#btn-edit").click();

    expect($("#commands-output").hidden).toBe(true);
    expect($("#commands-empty").hidden).toBe(false);
    expect($("#btn-copy").hidden).toBe(true);
  });

  it("hides commands when JSON has no require", () => {
    mockBuildComposerCommands.mockReturnValue([]);

    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ name: "test" });

    mockParseComposer.mockResolvedValue({ drupal_packages: [], composer_packages: [] });
    $("#btn-edit").click();
    $("#btn-edit").click();

    expect($("#commands-output").hidden).toBe(true);
    expect($("#btn-copy").hidden).toBe(true);
  });
});

// =============================================================================
// Copy Button
// =============================================================================

describe("Copy button", () => {
  it("starts hidden", () => {
    expect($("#btn-copy").hidden).toBe(true);
  });

  it("copies commands to clipboard on click", async () => {
    // Set up commands output manually
    const output = $("#commands-output");
    output.textContent = "composer require \"drupal/gin:^5.0\" --no-update\ncomposer update --dry-run";
    output.hidden = false;
    $("#btn-copy").hidden = false;

    $("#btn-copy").click();
    await flushPromises();

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(output.textContent);
  });

  it("shows 'Copied!' feedback after copying", async () => {
    const output = $("#commands-output");
    output.textContent = "some command";
    output.hidden = false;
    $("#btn-copy").hidden = false;

    $("#btn-copy").click();
    await flushPromises();

    expect($("#btn-copy").textContent).toBe("Copied!");
  });
});

// =============================================================================
// Packages Tab Dirty Indicator
// =============================================================================

describe("Packages tab dirty indicator", () => {
  it("shows 'Packages' without asterisk by default", () => {
    expect($('[data-tab="tab-packages"]').textContent).toBe("Packages");
  });

  it("shows (*) when a Drupal version is changed in a dropdown", async () => {
    // Set up textarea and trigger a full load
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [{ name: "gin 6.0.0", version: "6.0.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update --dry-run",
    ]);

    // Trigger loadComposer via edit toggle
    $("#btn-edit").click();
    $("#btn-edit").click();

    // Wait for all async operations (parseComposer + fetchReleases)
    await flushPromises();
    await flushPromises();

    // There should be a select for drupal/gin
    const select = $("#select-drupal\\/gin");
    expect(select).toBeTruthy();

    // Tab should show "Packages" (no changes yet)
    expect($('[data-tab="tab-packages"]').textContent).toBe("Packages");

    // Change dropdown to a different version
    select.value = "^6.0.0";
    select.dispatchEvent(new Event("change"));

    expect($('[data-tab="tab-packages"]').textContent).toBe("Packages (*)");
  });

  it("shows (*) when a Composer version is changed", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drush/drush": "^12" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [],
      composer_packages: [{ name: "drush/drush", module: "drush/drush", version: "^12" }],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [{ name: "drush/drush 13.0.1", version: "13.0.1" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drush/drush:^12" --no-update',
      "composer update --dry-run",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    const select = $("#select-drush\\/drush");
    expect(select).toBeTruthy();

    select.value = "^13.0.1";
    select.dispatchEvent(new Event("change"));

    expect($('[data-tab="tab-packages"]').textContent).toBe("Packages (*)");
  });

  it("removes (*) when version is changed back to current", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [{ name: "gin 6.0.0", version: "6.0.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update --dry-run",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    const select = $("#select-drupal\\/gin");

    // Change to new version
    select.value = "^6.0.0";
    select.dispatchEvent(new Event("change"));
    expect($('[data-tab="tab-packages"]').textContent).toBe("Packages (*)");

    // Change back to current
    select.value = "^5.0";
    select.dispatchEvent(new Event("change"));
    expect($('[data-tab="tab-packages"]').textContent).toBe("Packages");
  });

  it("enables Apply button when a version is changed", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [{ name: "gin 6.0.0", version: "6.0.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update --dry-run",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Initially disabled (no changes)
    expect($("#btn-apply").disabled).toBe(true);

    // Change version -> enabled
    const select = $("#select-drupal\\/gin");
    select.value = "^6.0.0";
    select.dispatchEvent(new Event("change"));
    expect($("#btn-apply").disabled).toBe(false);

    // Change back -> disabled
    select.value = "^5.0";
    select.dispatchEvent(new Event("change"));
    expect($("#btn-apply").disabled).toBe(true);
  });
});

// =============================================================================
// Full Load Flow
// =============================================================================

describe("loadComposer flow", () => {
  it("parses and renders Drupal packages with releases", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [
        { name: "gin 6.0.0", version: "6.0.0", core_compatibility: "^10 || ^11" },
        { name: "gin 5.1.0", version: "5.1.0", core_compatibility: "^10" },
      ],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update --dry-run",
    ]);

    // Trigger load via edit toggle
    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Should have rendered the table with package rows
    const rows = $$("#packages-body tr");
    expect(rows.length).toBeGreaterThanOrEqual(1);

    // Find the package row (skip section headers)
    const pkgRow = Array.from(rows).find(r => r.querySelector("a"));
    expect(pkgRow).toBeTruthy();

    // Package name should be a link to drupal.org
    const link = pkgRow.querySelector("a");
    expect(link.textContent).toBe("drupal/gin");
    expect(link.href).toContain("drupal.org/project/gin#project-releases");
    expect(link.target).toBe("_blank");

    // Dropdown should have current + 2 release options
    const select = pkgRow.querySelector("select");
    expect(select).toBeTruthy();
    expect(select.options).toHaveLength(3);
    expect(select.options[0].textContent).toContain("(current)");
  });

  it("parses and renders Composer packages", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drush/drush": "^12" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [],
      composer_packages: [{ name: "drush/drush", module: "drush/drush", version: "^12" }],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [{ name: "drush/drush 13.0.1", version: "13.0.1" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drush/drush:^12" --no-update',
      "composer update --dry-run",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Find the package link - should point to Packagist
    const link = $$("#packages-body a")[0];
    expect(link).toBeTruthy();
    expect(link.textContent).toBe("drush/drush");
    expect(link.href).toContain("packagist.org/packages/drush/drush");
  });

  it("renders both Drupal and Composer packages with section headers", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0", "drush/drush": "^12" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [{ name: "drush/drush", module: "drush/drush", version: "^12" }],
    });
    mockFetchReleases.mockResolvedValue({ releases: [] });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update --dry-run",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Should have section headers
    const bodyHTML = $("#packages-body").innerHTML;
    expect(bodyHTML).toContain("Drupal Packages");
    expect(bodyHTML).toContain("Composer Packages");
  });

  it("calls parseComposer with the parsed JSON", async () => {
    const composerJSON = { require: { "drupal/gin": "^5.0" } };
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify(composerJSON);

    mockParseComposer.mockResolvedValue({ drupal_packages: [], composer_packages: [] });

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();

    expect(mockParseComposer).toHaveBeenCalledWith(composerJSON);
  });

  it("calls fetchReleases with full package name", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0", "drush/drush": "^12" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [{ name: "drush/drush", module: "drush/drush", version: "^12" }],
    });
    mockFetchReleases.mockResolvedValue({ releases: [] });
    mockBuildComposerCommands.mockReturnValue([]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Should be called with full package names, not module names
    const calls = mockFetchReleases.mock.calls.map(c => c[0]);
    expect(calls).toContain("drupal/gin");
    expect(calls).toContain("drush/drush");
  });

  it("handles parseComposer errors gracefully", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: {} });

    mockParseComposer.mockRejectedValue(new Error("server error"));

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();

    expect($("#status").textContent).toContain("Error parsing");
    expect($("#status").classList.contains("error")).toBe(true);
  });

  it("handles fetchReleases errors gracefully", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockRejectedValue(new Error("network error"));
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update --dry-run",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Should still render the row, but with "Loading..." for releases
    const rows = $$("#packages-body tr");
    expect(rows.length).toBeGreaterThanOrEqual(1);
  });
});

// =============================================================================
// Apply Versions
// =============================================================================

describe("Apply versions", () => {
  it("calls updateComposer and reloads on apply", async () => {
    const textarea = $("#composer-textarea");
    const composerJSON = { require: { "drupal/gin": "^5.0" } };
    textarea.value = JSON.stringify(composerJSON);

    // Set up initial load
    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [{ name: "gin 6.0.0", version: "6.0.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update --dry-run",
    ]);
    mockBuildVersionMap.mockReturnValue({ "drupal/gin": "^6.0.0" });

    const updatedJSON = { require: { "drupal/gin": "^6.0.0" } };
    mockUpdateComposer.mockResolvedValue({ composer_json: updatedJSON });

    // Load first
    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Select a different version to enable Apply
    const select = $("#select-drupal\\/gin");
    select.value = "^6.0.0";
    select.dispatchEvent(new Event("change"));
    expect($("#btn-apply").disabled).toBe(false);

    // Click apply
    $("#btn-apply").click();
    await flushPromises();
    await flushPromises();

    expect(mockUpdateComposer).toHaveBeenCalled();
  });

  it("keeps Apply disabled when no changes are selected", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [{ name: "gin 6.0.0", version: "6.0.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update --dry-run",
    ]);

    // Load
    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Apply should be disabled since no version was changed
    expect($("#btn-apply").disabled).toBe(true);
  });
});

// =============================================================================
// Download Button
// =============================================================================

describe("Download button", () => {
  it("does nothing when textarea is empty", () => {
    const createElement = vi.spyOn(document, "createElement");
    $("#composer-textarea").value = "";
    $("#btn-download").click();

    // Should not have created an anchor element for download
    const anchorCalls = createElement.mock.calls.filter(([tag]) => tag === "a");
    expect(anchorCalls).toHaveLength(0);
  });

  it("creates a download link when textarea has content", () => {
    URL.createObjectURL = vi.fn(() => "blob:mock-url");
    URL.revokeObjectURL = vi.fn();

    const clickSpy = vi.fn();
    const origCreateElement = document.createElement.bind(document);
    vi.spyOn(document, "createElement").mockImplementation((tag) => {
      const el = origCreateElement(tag);
      if (tag === "a") el.click = clickSpy;
      return el;
    });

    $("#composer-textarea").value = '{"require":{}}';
    $("#btn-download").click();

    expect(URL.createObjectURL).toHaveBeenCalled();
    expect(clickSpy).toHaveBeenCalled();
    expect(URL.revokeObjectURL).toHaveBeenCalled();
  });
});

// =============================================================================
// Drop Zone
// =============================================================================

describe("Drop zone", () => {
  it("is not disabled by default", () => {
    expect($("#drop-zone").classList.contains("disabled")).toBe(false);
  });

  it("is disabled while editing", () => {
    $("#btn-edit").click();
    expect($("#drop-zone").classList.contains("disabled")).toBe(true);
  });

  it("adds dragover class on dragover event (when not editing)", () => {
    const event = new Event("dragover", { bubbles: true });
    event.preventDefault = vi.fn();
    $("#drop-zone").dispatchEvent(event);

    expect($("#drop-zone").classList.contains("dragover")).toBe(true);
  });

  it("does not add dragover class when editing", () => {
    $("#btn-edit").click();

    const event = new Event("dragover", { bubbles: true });
    event.preventDefault = vi.fn();
    $("#drop-zone").dispatchEvent(event);

    expect($("#drop-zone").classList.contains("dragover")).toBe(false);
  });

  it("removes dragover class on dragleave", () => {
    const dragover = new Event("dragover", { bubbles: true });
    dragover.preventDefault = vi.fn();
    $("#drop-zone").dispatchEvent(dragover);
    expect($("#drop-zone").classList.contains("dragover")).toBe(true);

    $("#drop-zone").dispatchEvent(new Event("dragleave"));
    expect($("#drop-zone").classList.contains("dragover")).toBe(false);
  });
});
