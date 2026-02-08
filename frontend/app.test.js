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
      <button id="btn-copy-json">Copy</button>
    </div>
    <div id="drop-zone" class="drop-zone">Drop or click to upload</div>
    <input type="file" id="file-input" accept=".json" hidden>
  </div>
  <div id="tab-packages" class="tab-panel">
    <div class="actions">
      <button id="btn-apply" disabled>Apply</button>
      <button id="btn-revert" disabled>Revert</button>
    </div>
    <table id="packages-table">
      <thead><tr><th>Package</th><th>Current</th><th>Drupal Core</th><th>Available</th></tr></thead>
      <tbody id="packages-body">
        <tr><td colspan="4">No composer.json loaded yet.</td></tr>
      </tbody>
    </table>
  </div>
  <div id="tab-commands" class="tab-panel">
    <p id="commands-empty">Load a composer.json to see commands.</p>
    <div id="commands-apply-section" hidden>
      <h3>Apply</h3>
      <pre id="commands-apply-output"></pre>
      <div class="commands-actions"><button id="btn-copy-apply">Copy</button></div>
    </div>
    <div id="commands-dryrun-section" hidden>
      <h3>Dry Run</h3>
      <pre id="commands-dryrun-output"></pre>
      <div class="commands-actions"><button id="btn-copy-dryrun">Copy</button></div>
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
let mockBuildDryRunCommand;

beforeEach(async () => {
  vi.resetModules();
  vi.restoreAllMocks();

  document.body.innerHTML = HTML;

  mockParseComposer = vi.fn();
  mockFetchReleases = vi.fn();
  mockUpdateComposer = vi.fn();
  mockBuildVersionMap = vi.fn().mockReturnValue({});
  mockBuildComposerCommands = vi.fn().mockReturnValue([]);
  mockBuildDryRunCommand = vi.fn().mockReturnValue("");

  vi.doMock("./api.js", () => ({
    parseComposer: mockParseComposer,
    fetchReleases: mockFetchReleases,
    updateComposer: mockUpdateComposer,
    buildVersionMap: mockBuildVersionMap,
    buildComposerCommands: mockBuildComposerCommands,
    buildDryRunCommand: mockBuildDryRunCommand,
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
  it("shows both command sections when textarea has valid composer.json with require", () => {
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update",
    ]);
    mockBuildDryRunCommand.mockReturnValue('composer require "drupal/gin:^5.0" --dry-run');

    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    // Enter + exit edit mode triggers renderTable -> updateCommands
    mockParseComposer.mockResolvedValue({ drupal_packages: [], composer_packages: [] });
    $("#btn-edit").click();
    $("#btn-edit").click();

    expect($("#commands-apply-section").hidden).toBe(false);
    expect($("#commands-dryrun-section").hidden).toBe(false);
    expect($("#commands-empty").hidden).toBe(true);
  });

  it("hides command sections when textarea is empty", () => {
    mockBuildComposerCommands.mockReturnValue([]);
    mockBuildDryRunCommand.mockReturnValue("");

    $("#btn-edit").click();
    $("#composer-textarea").value = "";
    $("#btn-edit").click();

    expect($("#commands-apply-section").hidden).toBe(true);
    expect($("#commands-dryrun-section").hidden).toBe(true);
    expect($("#commands-empty").hidden).toBe(false);
  });

  it("hides command sections when JSON has no require", () => {
    mockBuildComposerCommands.mockReturnValue([]);
    mockBuildDryRunCommand.mockReturnValue("");

    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ name: "test" });

    mockParseComposer.mockResolvedValue({ drupal_packages: [], composer_packages: [] });
    $("#btn-edit").click();
    $("#btn-edit").click();

    expect($("#commands-apply-section").hidden).toBe(true);
    expect($("#commands-dryrun-section").hidden).toBe(true);
  });
});

// =============================================================================
// Copy Button
// =============================================================================

describe("Copy buttons", () => {
  it("copies apply commands to clipboard", async () => {
    const output = $("#commands-apply-output");
    output.textContent = "composer require \"drupal/gin:^5.0\" --no-update\ncomposer update";

    $("#btn-copy-apply").click();
    await flushPromises();

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(output.textContent);
    expect($("#btn-copy-apply").textContent).toBe("Copied!");
  });

  it("copies dry-run command to clipboard", async () => {
    const output = $("#commands-dryrun-output");
    output.textContent = 'composer require "drupal/gin:^5.0" --dry-run';

    $("#btn-copy-dryrun").click();
    await flushPromises();

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(output.textContent);
    expect($("#btn-copy-dryrun").textContent).toBe("Copied!");
  });

  it("copies composer.json textarea content to clipboard", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = '{"require": {}}';

    $("#btn-copy-json").click();
    await flushPromises();

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('{"require": {}}');
    expect($("#btn-copy-json").textContent).toBe("Copied!");
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
      releases: [{ name: "gin 6.0.0", version: "6.0.0", version_pin: "^6.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update",
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
    select.value = "^6.0";
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
      releases: [{ name: "drush/drush 13.0.1", version: "13.0.1", version_pin: "^13.0" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drush/drush:^12" --no-update',
      "composer update",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    const select = $("#select-drush\\/drush");
    expect(select).toBeTruthy();

    select.value = "^13.0";
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
      releases: [{ name: "gin 6.0.0", version: "6.0.0", version_pin: "^6.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    const select = $("#select-drupal\\/gin");

    // Change to new version
    select.value = "^6.0";
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
      releases: [{ name: "gin 6.0.0", version: "6.0.0", version_pin: "^6.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Initially disabled (no changes)
    expect($("#btn-apply").disabled).toBe(true);

    // Change version -> enabled
    const select = $("#select-drupal\\/gin");
    select.value = "^6.0";
    select.dispatchEvent(new Event("change"));
    expect($("#btn-apply").disabled).toBe(false);

    // Change back -> disabled
    select.value = "^5.0";
    select.dispatchEvent(new Event("change"));
    expect($("#btn-apply").disabled).toBe(true);
  });
});

// =============================================================================
// Revert and Set All to Latest
// =============================================================================

describe("Revert and Set all to latest", () => {
  it("reverts all dropdowns to current version", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [
        { name: "gin 6.0.0", version: "6.0.0", version_pin: "^6.0", core_compatibility: "^10 || ^11" },
        { name: "gin 5.0.3", version: "5.0.3", version_pin: "^5.0", core_compatibility: "^10" },
      ],
    });
    mockBuildComposerCommands.mockReturnValue([]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Change to a different version
    const select = $("#select-drupal\\/gin");
    select.value = "^6.0";
    select.dispatchEvent(new Event("change"));
    expect($("#btn-apply").disabled).toBe(false);
    expect($("#btn-revert").disabled).toBe(false);

    // Click revert
    $("#btn-revert").click();

    expect(select.value).toBe("^5.0");
    expect($("#btn-apply").disabled).toBe(true);
    expect($("#btn-revert").disabled).toBe(true);
  });

  it("shows 'Set all to latest' button in Drupal Packages section header", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({
      require: { "drupal/core-recommended": "^11", "drupal/gin": "^5.0" },
    });

    mockParseComposer.mockResolvedValue({
      core_packages: [{ name: "drupal/core-recommended", module: "core-recommended", version: "^11" }],
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [
        { name: "gin 6.0.0", version: "6.0.0", version_pin: "^6.0", core_compatibility: "^10 || ^11" },
        { name: "gin 5.0.3", version: "5.0.3", version_pin: "^5.0", core_compatibility: "^10" },
      ],
    });
    mockBuildComposerCommands.mockReturnValue([]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Should have a "Set all to latest" button
    const btn = Array.from($$("#packages-body button")).find(
      (b) => b.textContent === "Set all to latest",
    );
    expect(btn).toBeTruthy();

    // Click it — the gin dropdown should switch to the latest (first non-current option)
    btn.click();

    const select = $("#select-drupal\\/gin");
    expect(select.value).toBe("^6.0");
    expect($("#btn-apply").disabled).toBe(false);
  });
});

// =============================================================================
// Core Packages
// =============================================================================

describe("Core packages", () => {
  it("renders a core row with package list and dropdown", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/core-recommended": "^11", "drupal/core-composer-scaffold": "^11" } });

    mockParseComposer.mockResolvedValue({
      core_packages: [
        { name: "drupal/core-composer-scaffold", module: "core-composer-scaffold", version: "^11" },
        { name: "drupal/core-recommended", module: "core-recommended", version: "^11" },
      ],
      drupal_packages: [],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [
        { name: "drupal 11.1.0", version: "11.1.0", version_pin: "^11.1" },
        { name: "drupal 10.4.3", version: "10.4.3", version_pin: "^10.4" },
      ],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/core-recommended:^11" --no-update',
      "composer update",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Should have a core link pointing to drupal.org
    const link = $$("#packages-body a")[0];
    expect(link).toBeTruthy();
    expect(link.textContent).toBe("Drupal Core");
    expect(link.href).toContain("drupal.org/project/drupal#project-releases");

    // Should list the package names
    const bodyHTML = $("#packages-body").innerHTML;
    expect(bodyHTML).toContain("drupal/core-recommended");
    expect(bodyHTML).toContain("drupal/core-composer-scaffold");

    // Should have a select with id "select-core"
    const select = $("#select-core");
    expect(select).toBeTruthy();
    expect(select.options).toHaveLength(3); // current + 2 releases
    expect(select.options[0].textContent).toContain("(current)");
  });

  it("marks dirty when core version is changed", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/core-recommended": "^11" } });

    mockParseComposer.mockResolvedValue({
      core_packages: [{ name: "drupal/core-recommended", module: "core-recommended", version: "^11" }],
      drupal_packages: [],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [{ name: "drupal 11.1.0", version: "11.1.0", version_pin: "^11.1" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/core-recommended:^11" --no-update',
      "composer update",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    expect($('[data-tab="tab-packages"]').textContent).toBe("Packages");
    expect($("#btn-apply").disabled).toBe(true);

    const select = $("#select-core");
    select.value = "^11.1";
    select.dispatchEvent(new Event("change"));

    expect($('[data-tab="tab-packages"]').textContent).toBe("Packages (*)");
    expect($("#btn-apply").disabled).toBe(false);
  });

  it("shows section headers when core and other packages coexist", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/core-recommended": "^11", "drupal/gin": "^5.0" } });

    mockParseComposer.mockResolvedValue({
      core_packages: [{ name: "drupal/core-recommended", module: "core-recommended", version: "^11" }],
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({ releases: [] });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/core-recommended:^11" --no-update',
      "composer update",
    ]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    const bodyHTML = $("#packages-body").innerHTML;
    expect(bodyHTML).toContain("Drupal Core");
    expect(bodyHTML).toContain("Drupal Packages");
  });

  it("clears core state when entering edit mode", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/core-recommended": "^11" } });

    mockParseComposer.mockResolvedValue({
      core_packages: [{ name: "drupal/core-recommended", module: "core-recommended", version: "^11" }],
      drupal_packages: [],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({ releases: [] });
    mockBuildComposerCommands.mockReturnValue([]);

    // Load first
    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Enter edit mode again
    $("#btn-edit").click();

    // Core select should be gone
    expect($("#select-core")).toBeNull();
    expect($("#packages-body").textContent).toContain("No packages found");
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
        { name: "gin 6.0.0", version: "6.0.0", version_pin: "^6.0", core_compatibility: "^10 || ^11" },
        { name: "gin 5.0.3", version: "5.0.3", version_pin: "^5.0", core_compatibility: "^10" },
      ],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update",
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

    // Core requirement column should show the core_compatibility of the matching release
    const cells = pkgRow.querySelectorAll("td");
    expect(cells[2].textContent).toBe("^10");

    // Dropdown should have current + 2 release options
    const select = pkgRow.querySelector("select");
    expect(select).toBeTruthy();
    expect(select.options).toHaveLength(3);
    expect(select.options[0].textContent).toContain("(current)");
  });

  it("updates Drupal Core column when changing dropdown selection", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drupal/gin": "^5.0" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [{ name: "drupal/gin", module: "gin", version: "^5.0" }],
      composer_packages: [],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [
        { name: "gin 6.0.0", version: "6.0.0", version_pin: "^6.0", core_compatibility: "^10 || ^11" },
        { name: "gin 5.0.3", version: "5.0.3", version_pin: "^5.0", core_compatibility: "^10" },
      ],
    });
    mockBuildComposerCommands.mockReturnValue([]);

    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    const pkgRow = Array.from($$("#packages-body tr")).find(r => r.querySelector("a"));
    const cells = pkgRow.querySelectorAll("td");
    const select = pkgRow.querySelector("select");

    // Initially shows core compat for current version
    expect(cells[2].textContent).toBe("^10");

    // Change to ^6.0 — core cell should update
    select.value = "^6.0";
    select.dispatchEvent(new Event("change"));
    expect(cells[2].textContent).toBe("^10 || ^11");

    // Change back to current — core cell should revert
    select.value = "^5.0";
    select.dispatchEvent(new Event("change"));
    expect(cells[2].textContent).toBe("^10");
  });

  it("parses and renders Composer packages", async () => {
    const textarea = $("#composer-textarea");
    textarea.value = JSON.stringify({ require: { "drush/drush": "^12" } });

    mockParseComposer.mockResolvedValue({
      drupal_packages: [],
      composer_packages: [{ name: "drush/drush", module: "drush/drush", version: "^12" }],
    });
    mockFetchReleases.mockResolvedValue({
      releases: [{ name: "drush/drush 13.0.1", version: "13.0.1", version_pin: "^13.0" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drush/drush:^12" --no-update',
      "composer update",
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
      "composer update",
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
      "composer update",
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
      releases: [{ name: "gin 6.0.0", version: "6.0.0", version_pin: "^6.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update",
    ]);
    mockBuildVersionMap.mockReturnValue({ "drupal/gin": "^6.0" });

    const updatedJSON = { require: { "drupal/gin": "^6.0" } };
    mockUpdateComposer.mockResolvedValue({ composer_json: updatedJSON });

    // Load first
    $("#btn-edit").click();
    $("#btn-edit").click();
    await flushPromises();
    await flushPromises();

    // Select a different version to enable Apply
    const select = $("#select-drupal\\/gin");
    select.value = "^6.0";
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
      releases: [{ name: "gin 6.0.0", version: "6.0.0", version_pin: "^6.0", core_compatibility: "^10 || ^11" }],
    });
    mockBuildComposerCommands.mockReturnValue([
      'composer require "drupal/gin:^5.0" --no-update',
      "composer update",
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
