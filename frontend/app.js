import { parseComposer, fetchReleases, updateComposer, buildVersionMap, buildComposerCommands, buildDryRunCommand } from "./api.js";

/** @typedef {import("./api.js").Release} Release */
/** @typedef {import("./api.js").VersionSelection} VersionSelection */

/**
 * @typedef {Object} PackageState
 * @property {string} name
 * @property {string} module
 * @property {string} version
 * @property {Release[]} releases
 */

/**
 * @typedef {Object} CoreState
 * @property {{name: string, version: string}[]} packages
 * @property {string} version
 * @property {Release[]} releases
 */

// =============================================================================
// State
// =============================================================================

/** @type {CoreState | null} */
let coreState = null;
/** @type {PackageState[]} */
let drupalPackages = [];
/** @type {PackageState[]} */
let composerPackages = [];
/** Whether the textarea is currently editable. */
let editing = false;

// =============================================================================
// DOM Elements
// =============================================================================

const textarea       = /** @type {HTMLTextAreaElement} */ (document.getElementById("composer-textarea"));
const statusEl       = /** @type {HTMLParagraphElement} */ (document.getElementById("status"));
const packagesBody   = /** @type {HTMLTableSectionElement} */ (document.getElementById("packages-body"));
const dropZone       = /** @type {HTMLDivElement} */ (document.getElementById("drop-zone"));
const fileInput      = /** @type {HTMLInputElement} */ (document.getElementById("file-input"));
const btnEdit        = /** @type {HTMLButtonElement} */ (document.getElementById("btn-edit"));
const btnDownload    = /** @type {HTMLButtonElement} */ (document.getElementById("btn-download"));
const btnApply       = /** @type {HTMLButtonElement} */ (document.getElementById("btn-apply"));
const btnRevert      = /** @type {HTMLButtonElement} */ (document.getElementById("btn-revert"));
const commandsEmpty       = /** @type {HTMLParagraphElement} */ (document.getElementById("commands-empty"));
const commandsApplySection  = /** @type {HTMLDivElement} */ (document.getElementById("commands-apply-section"));
const commandsApplyOutput   = /** @type {HTMLPreElement} */ (document.getElementById("commands-apply-output"));
const btnCopyApply          = /** @type {HTMLButtonElement} */ (document.getElementById("btn-copy-apply"));
const commandsDryrunSection = /** @type {HTMLDivElement} */ (document.getElementById("commands-dryrun-section"));
const commandsDryrunOutput  = /** @type {HTMLPreElement} */ (document.getElementById("commands-dryrun-output"));
const btnCopyDryrun         = /** @type {HTMLButtonElement} */ (document.getElementById("btn-copy-dryrun"));
const btnCopyJson           = /** @type {HTMLButtonElement} */ (document.getElementById("btn-copy-json"));
const tabPackages    = /** @type {HTMLButtonElement} */ (document.querySelector('[data-tab="tab-packages"]'));

// =============================================================================
// Tabs
// =============================================================================

const tabs   = /** @type {NodeListOf<HTMLButtonElement>} */ (document.querySelectorAll(".tab"));
const panels = /** @type {NodeListOf<HTMLElement>} */ (document.querySelectorAll(".tab-panel"));

/**
 * Switch to the tab with the given panel id.
 * @param {string | undefined} tabId
 */
function switchTab(tabId) {
  tabs.forEach(t => t.classList.toggle("active", t.dataset.tab === tabId));
  panels.forEach(p => p.classList.toggle("active", p.id === tabId));
}

tabs.forEach(tab => {
  tab.addEventListener("click", () => switchTab(tab.dataset.tab));
});

// =============================================================================
// Edit Toggle
// =============================================================================

/** Enter edit mode: make textarea editable, clear the table. */
function startEditing() {
  editing = true;
  textarea.readOnly = false;
  btnEdit.textContent = "Done Editing";
  dropZone.classList.add("disabled");
  coreState = null;
  drupalPackages = [];
  composerPackages = [];
  renderTable();
  btnApply.disabled = true;
  setStatus("Editing. Click 'Done Editing' when finished.");
}

/** Leave edit mode: lock textarea, parse its content. */
function stopEditing() {
  editing = false;
  textarea.readOnly = true;
  btnEdit.textContent = "Edit";
  dropZone.classList.remove("disabled");
  loadComposer();
}

/** Toggle between edit and read-only mode. */
function toggleEdit() {
  if (editing) {
    stopEditing();
  } else {
    startEditing();
  }
}

// =============================================================================
// Core Logic
// =============================================================================

/** @returns {PackageState[]} All non-core packages (both Drupal and Composer). */
function allPackages() {
  return [...drupalPackages, ...composerPackages];
}

/** Parse the composer.json from the textarea, fetch releases, and render the table. */
async function loadComposer() {
  const text = textarea.value.trim();
  if (!text) {
    setStatus("Textarea is empty.", true);
    return;
  }

  /** @type {Record<string, any>} */
  let composerJSON;
  try {
    composerJSON = JSON.parse(text);
  } catch (e) {
    setStatus("Invalid JSON in textarea.", true);
    return;
  }

  setStatus("Parsing composer.json...");

  let parsed;
  try {
    parsed = await parseComposer(composerJSON);
  } catch (e) {
    setStatus("Error parsing: " + /** @type {Error} */ (e).message, true);
    return;
  }

  // Core packages
  const corePkgs = parsed.core_packages || [];
  if (corePkgs.length > 0) {
    coreState = {
      packages: corePkgs.map(p => ({ name: p.name, version: p.version })),
      version: corePkgs[0].version,
      releases: [],
    };
  } else {
    coreState = null;
  }

  drupalPackages = (parsed.drupal_packages || []).map(pkg => ({
    name: pkg.name,
    module: pkg.module,
    version: pkg.version,
    releases: /** @type {Release[]} */ ([]),
  }));

  composerPackages = (parsed.composer_packages || []).map(pkg => ({
    name: pkg.name,
    module: pkg.module,
    version: pkg.version,
    releases: /** @type {Release[]} */ ([]),
  }));

  setStatus("Fetching releases...");
  renderTable();

  // Fetch releases for all packages in parallel
  /** @type {Promise<void>[]} */
  const fetches = [];

  // Core releases (fetch once)
  if (coreState) {
    fetches.push((async () => {
      try {
        const data = await fetchReleases(coreState.packages[0].name);
        coreState.releases = data.releases || [];
      } catch (e) {
        coreState.releases = [];
      }
    })());
  }

  // Regular packages
  for (const pkg of allPackages()) {
    fetches.push((async () => {
      try {
        const data = await fetchReleases(pkg.name);
        pkg.releases = data.releases || [];
      } catch (e) {
        pkg.releases = [];
      }
    })());
  }

  await Promise.all(fetches);

  renderTable();
  setStatus("Ready. Select versions and click Apply.");
}

/** Build a version map from the dropdowns and call the update API. */
async function applyVersions() {
  const text = textarea.value.trim();
  if (!text) return;

  /** @type {Record<string, any>} */
  let composerJSON;
  try {
    composerJSON = JSON.parse(text);
  } catch (e) {
    setStatus("Invalid JSON in textarea.", true);
    return;
  }

  /** @type {VersionSelection[]} */
  const withSelections = allPackages().map(pkg => {
    const select = /** @type {HTMLSelectElement | null} */ (document.getElementById("select-" + pkg.name));
    return {
      name: pkg.name,
      version: pkg.version,
      selectedVersion: select ? select.value : pkg.version,
    };
  });

  // Add core packages (all get the same selected version)
  if (coreState) {
    const coreSelect = /** @type {HTMLSelectElement | null} */ (document.getElementById("select-core"));
    const coreSelectedVersion = coreSelect ? coreSelect.value : coreState.version;
    for (const pkg of coreState.packages) {
      withSelections.push({
        name: pkg.name,
        version: pkg.version,
        selectedVersion: coreSelectedVersion,
      });
    }
  }

  const versions = buildVersionMap(withSelections);

  if (Object.keys(versions).length === 0) {
    setStatus("No version changes selected.");
    return;
  }

  setStatus("Applying updates...");

  try {
    const data = await updateComposer(composerJSON, versions);
    textarea.value = JSON.stringify(data.composer_json, null, 4);
    setStatus("Updated " + Object.keys(versions).length + " package(s). Reloading table...");
    await loadComposer();
  } catch (e) {
    setStatus("Error applying updates: " + /** @type {Error} */ (e).message, true);
  }
}

// =============================================================================
// Revert & Set All to Latest
// =============================================================================

/** Reset all version dropdowns to their current (original) value. */
function revertVersions() {
  if (coreState) {
    const coreSelect = /** @type {HTMLSelectElement | null} */ (document.getElementById("select-core"));
    if (coreSelect) {
      coreSelect.value = coreState.version;
      coreSelect.dispatchEvent(new Event("change"));
    }
  }
  for (const pkg of allPackages()) {
    const select = /** @type {HTMLSelectElement | null} */ (document.getElementById("select-" + pkg.name));
    if (select) {
      select.value = pkg.version;
      select.dispatchEvent(new Event("change"));
    }
  }
  updatePackagesTabDirty();
}

/**
 * Set all dropdowns in a package list to their latest (first non-current) option.
 * @param {PackageState[]} packages
 */
function setAllToLatest(packages) {
  for (const pkg of packages) {
    const select = /** @type {HTMLSelectElement | null} */ (document.getElementById("select-" + pkg.name));
    if (select && select.options.length > 1) {
      // First option is "(current)", second is the latest release
      select.value = select.options[1].value;
      select.dispatchEvent(new Event("change"));
    }
  }
  updatePackagesTabDirty();
}

// =============================================================================
// Packages Tab Dirty Indicator
// =============================================================================

/** Check if any version dropdown differs from the current version, update tab title. */
function updatePackagesTabDirty() {
  let dirty = false;

  // Check core dropdown
  if (coreState) {
    const select = /** @type {HTMLSelectElement | null} */ (document.getElementById("select-core"));
    if (select && select.value !== coreState.version) {
      dirty = true;
    }
  }

  // Check regular package dropdowns
  if (!dirty) {
    for (const pkg of allPackages()) {
      const select = /** @type {HTMLSelectElement | null} */ (document.getElementById("select-" + pkg.name));
      if (select && select.value !== pkg.version) {
        dirty = true;
        break;
      }
    }
  }

  tabPackages.textContent = dirty ? "Packages (*)" : "Packages";
  btnApply.disabled = !dirty;
  btnRevert.disabled = !dirty;
}

// =============================================================================
// Rendering
// =============================================================================

/**
 * Render a section header row in the packages table.
 * @param {string} title
 * @param {(() => void) | null} [onSetLatest] - If provided, adds a "Set all to latest" button.
 */
function renderSectionHeader(title, onSetLatest) {
  const row = document.createElement("tr");
  const cell = document.createElement("td");
  cell.colSpan = 4;
  cell.style.background = "#eee";

  const strong = document.createElement("strong");
  strong.textContent = title;
  cell.appendChild(strong);

  if (onSetLatest) {
    const btn = document.createElement("button");
    btn.textContent = "Set all to latest";
    btn.style.marginLeft = "1rem";
    btn.style.fontSize = "0.8rem";
    btn.style.padding = "0.15rem 0.5rem";
    btn.addEventListener("click", onSetLatest);
    cell.appendChild(btn);
  }

  row.appendChild(cell);
  packagesBody.appendChild(row);
}

/** Render the special Drupal Core row with a single dropdown for all core packages. */
function renderCoreRow() {
  if (!coreState) return;

  const row = document.createElement("tr");

  // Package name cell
  const nameCell = document.createElement("td");
  const link = document.createElement("a");
  link.href = "https://www.drupal.org/project/drupal#project-releases";
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  link.textContent = "Drupal Core";
  nameCell.appendChild(link);

  const packageList = document.createElement("div");
  packageList.style.fontSize = "0.85em";
  packageList.style.color = "#666";
  packageList.textContent = coreState.packages.map(p => p.name).join(", ");
  nameCell.appendChild(packageList);
  row.appendChild(nameCell);

  // Current version
  const versionCell = document.createElement("td");
  versionCell.className = "col-version";
  versionCell.textContent = coreState.version;
  row.appendChild(versionCell);

  // Drupal Core column (empty for the core row itself)
  const emptyCore = document.createElement("td");
  emptyCore.className = "col-core";
  row.appendChild(emptyCore);

  // Dropdown for core releases
  const selectCell = document.createElement("td");
  if (coreState.releases.length > 0) {
    const select = document.createElement("select");
    select.id = "select-core";

    const keepOption = document.createElement("option");
    keepOption.value = coreState.version;
    keepOption.textContent = coreState.version + " (current)";
    select.appendChild(keepOption);

    for (const release of coreState.releases) {
      const option = document.createElement("option");
      option.value = release.version_pin;
      let label = release.version_pin;
      if (release.core_compatibility) {
        label += "  (" + release.version + ", core: " + release.core_compatibility + ")";
      } else if (release.version_pin !== "^" + release.version) {
        label += "  (" + release.version + ")";
      }
      option.textContent = label;
      select.appendChild(option);
    }

    select.addEventListener("change", () => updatePackagesTabDirty());
    selectCell.appendChild(select);
  } else {
    selectCell.textContent = "Loading...";
  }
  row.appendChild(selectCell);

  packagesBody.appendChild(row);
}

/**
 * Render a single package row.
 * @param {PackageState} pkg
 * @param {boolean} isDrupal
 */
function renderPackageRow(pkg, isDrupal) {
  const row = document.createElement("tr");

  // Package name (linked)
  const nameCell = document.createElement("td");
  const link = document.createElement("a");
  if (isDrupal) {
    link.href = "https://www.drupal.org/project/" + pkg.module + "#project-releases";
  } else {
    link.href = "https://packagist.org/packages/" + pkg.name;
  }
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  link.textContent = pkg.name;
  nameCell.appendChild(link);
  row.appendChild(nameCell);

  // Current version
  const versionCell = document.createElement("td");
  versionCell.className = "col-version";
  versionCell.textContent = pkg.version;
  row.appendChild(versionCell);

  // Drupal Core column (Drupal packages only — shows core_compatibility)
  const coreCell = document.createElement("td");
  coreCell.className = "col-core";

  /** Update the core cell to reflect the given version constraint. */
  function updateCoreCell(/** @type {string} */ selectedValue) {
    if (!isDrupal) return;
    const match = pkg.releases.find((r) => r.version_pin === selectedValue);
    coreCell.textContent = (match && match.core_compatibility) ? match.core_compatibility : "";
  }
  updateCoreCell(pkg.version);
  row.appendChild(coreCell);

  // Dropdown for available versions
  const selectCell = document.createElement("td");
  if (pkg.releases.length > 0) {
    const select = document.createElement("select");
    select.id = "select-" + pkg.name;

    const keepOption = document.createElement("option");
    keepOption.value = pkg.version;
    keepOption.textContent = pkg.version + " (current)";
    select.appendChild(keepOption);

    for (const release of pkg.releases) {
      const option = document.createElement("option");
      option.value = release.version_pin;
      let label = release.version_pin;
      if (release.core_compatibility) {
        label += "  (" + release.version + ", core: " + release.core_compatibility + ")";
      } else if (release.version_pin !== "^" + release.version) {
        label += "  (" + release.version + ")";
      }
      option.textContent = label;
      select.appendChild(option);
    }

    // Update dirty indicator and core cell when user changes selection
    select.addEventListener("change", () => {
      updateCoreCell(select.value);
      updatePackagesTabDirty();
    });

    selectCell.appendChild(select);
  } else {
    selectCell.textContent = "Loading...";
  }
  row.appendChild(selectCell);

  packagesBody.appendChild(row);
}

/** Render the packages table with dropdowns, and update the commands block. */
function renderTable() {
  updateCommands();
  updatePackagesTabDirty();

  const hasCore = coreState !== null && coreState.packages.length > 0;
  const hasDrupal = drupalPackages.length > 0;
  const hasComposer = composerPackages.length > 0;

  if (!hasCore && !hasDrupal && !hasComposer) {
    packagesBody.innerHTML = '<tr><td colspan="4">No packages found.</td></tr>';
    return;
  }

  packagesBody.innerHTML = "";

  const typeCount = [hasCore, hasDrupal, hasComposer].filter(Boolean).length;
  const showHeaders = typeCount > 1;

  if (hasCore) {
    if (showHeaders) renderSectionHeader("Drupal Core");
    renderCoreRow();
  }

  if (hasDrupal) {
    if (showHeaders) renderSectionHeader("Drupal Packages", () => setAllToLatest(drupalPackages));
    for (const pkg of drupalPackages) {
      renderPackageRow(pkg, true);
    }
  }

  if (hasComposer) {
    if (showHeaders) renderSectionHeader("Composer Packages");
    for (const pkg of composerPackages) {
      renderPackageRow(pkg, false);
    }
  }
}

/** Update the composer commands blocks from the current textarea content. */
function updateCommands() {
  const text = textarea.value.trim();
  if (!text) {
    commandsApplySection.hidden = true;
    commandsDryrunSection.hidden = true;
    commandsEmpty.hidden = false;
    return;
  }

  let composerJSON;
  try {
    composerJSON = JSON.parse(text);
  } catch (e) {
    commandsApplySection.hidden = true;
    commandsDryrunSection.hidden = true;
    commandsEmpty.hidden = false;
    return;
  }

  /** @type {Record<string, string>} */
  const require = composerJSON.require || {};
  const commands = buildComposerCommands(require);
  const dryRun = buildDryRunCommand(require);

  if (commands.length === 0) {
    commandsApplySection.hidden = true;
    commandsDryrunSection.hidden = true;
    commandsEmpty.hidden = false;
    return;
  }

  commandsApplyOutput.textContent = commands.join("\n");
  commandsApplySection.hidden = false;

  commandsDryrunOutput.textContent = dryRun;
  commandsDryrunSection.hidden = false;

  commandsEmpty.hidden = true;
}

/**
 * Display a status message.
 * @param {string} msg
 * @param {boolean} [isError]
 */
function setStatus(msg, isError = false) {
  statusEl.textContent = msg;
  statusEl.classList.toggle("error", isError);
}

// =============================================================================
// Copy Buttons
// =============================================================================

/**
 * Wire a copy button to copy the text content of an element.
 * @param {HTMLButtonElement} btn
 * @param {() => string} getText
 */
function wireCopyButton(btn, getText) {
  btn.addEventListener("click", async () => {
    const text = getText();
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      btn.textContent = "Copied!";
      setTimeout(() => { btn.textContent = "Copy"; }, 1500);
    } catch (e) {
      setStatus("Failed to copy to clipboard.", true);
    }
  });
}

wireCopyButton(btnCopyApply, () => commandsApplyOutput.textContent || "");
wireCopyButton(btnCopyDryrun, () => commandsDryrunOutput.textContent || "");
wireCopyButton(btnCopyJson, () => textarea.value);

// =============================================================================
// File Handling
// =============================================================================

/**
 * Read a File object into the textarea and trigger loading.
 * @param {File} file
 */
function handleFile(file) {
  if (editing) {
    editing = false;
    textarea.readOnly = true;
    btnEdit.textContent = "Edit";
  }

  const reader = new FileReader();
  reader.onload = function () {
    textarea.value = /** @type {string} */ (reader.result);
    loadComposer();
  };
  reader.readAsText(file);
}

// =============================================================================
// Event Listeners
// =============================================================================

// Drop zone click opens the file picker (disabled while editing)
dropZone.addEventListener("click", () => {
  if (!editing) fileInput.click();
});
fileInput.addEventListener("change", () => {
  if (fileInput.files && fileInput.files.length > 0) handleFile(fileInput.files[0]);
});

// Edit toggle button
btnEdit.addEventListener("click", () => toggleEdit());

// Download button saves the textarea content as a file
btnDownload.addEventListener("click", () => {
  const text = textarea.value;
  if (!text.trim()) return;
  const blob = new Blob([text], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "composer.json";
  a.click();
  URL.revokeObjectURL(url);
});

// Apply button
btnApply.addEventListener("click", () => applyVersions());

// Revert button — reset all dropdowns to their current version
btnRevert.addEventListener("click", () => revertVersions());

// Drag and drop (disabled while editing)
dropZone.addEventListener("dragover", (e) => {
  e.preventDefault();
  if (!editing) dropZone.classList.add("dragover");
});
dropZone.addEventListener("dragleave", () => {
  dropZone.classList.remove("dragover");
});
dropZone.addEventListener("drop", (e) => {
  e.preventDefault();
  dropZone.classList.remove("dragover");
  if (!editing && e.dataTransfer && e.dataTransfer.files.length > 0) {
    handleFile(e.dataTransfer.files[0]);
  }
});
