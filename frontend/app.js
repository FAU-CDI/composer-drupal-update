import { parseComposer, fetchReleases, updateComposer, buildVersionMap } from "./api.js";

// =============================================================================
// State
// =============================================================================

// The current composer.json text lives in the textarea.
// Packages and their available releases are stored here after parsing.
let packages = [];    // [{ name, module, version, releases }]
let editing = false;  // Whether the textarea is currently editable

// =============================================================================
// DOM Elements
// =============================================================================

const textarea      = document.getElementById("composer-textarea");
const statusEl      = document.getElementById("status");
const packagesBody  = document.getElementById("packages-body");
const dropZone      = document.getElementById("drop-zone");
const fileInput     = document.getElementById("file-input");
const btnEdit       = document.getElementById("btn-edit");
const btnDownload   = document.getElementById("btn-download");
const btnApply      = document.getElementById("btn-apply");

// =============================================================================
// Edit Toggle
// =============================================================================

/** Enter edit mode: make textarea editable, clear the table. */
function startEditing() {
  editing = true;
  textarea.readOnly = false;
  btnEdit.textContent = "Done Editing";
  dropZone.classList.add("disabled");
  packages = [];
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

/** Parse the composer.json from the textarea, fetch releases, and render the table. */
async function loadComposer() {
  const text = textarea.value.trim();
  if (!text) {
    setStatus("Textarea is empty.", true);
    return;
  }

  let composerJSON;
  try {
    composerJSON = JSON.parse(text);
  } catch (e) {
    setStatus("Invalid JSON in textarea.", true);
    return;
  }

  setStatus("Parsing composer.json...");

  // Step 1: Parse to get drupal packages
  let parsed;
  try {
    parsed = await parseComposer(composerJSON);
  } catch (e) {
    setStatus("Error parsing: " + e.message, true);
    return;
  }

  // Step 2: Fetch releases for each package
  packages = parsed.packages.map(pkg => ({
    name: pkg.name,
    module: pkg.module,
    version: pkg.version,
    releases: [],
  }));

  setStatus("Fetching releases...");
  renderTable();

  await Promise.all(packages.map(async (pkg) => {
    try {
      const data = await fetchReleases(pkg.module);
      pkg.releases = data.releases || [];
    } catch (e) {
      pkg.releases = [];
    }
  }));

  renderTable();
  btnApply.disabled = false;
  setStatus("Ready. Select versions and click Apply.");
}

/** Build a version map from the dropdowns and call the update API. */
async function applyVersions() {
  const text = textarea.value.trim();
  if (!text) return;

  let composerJSON;
  try {
    composerJSON = JSON.parse(text);
  } catch (e) {
    setStatus("Invalid JSON in textarea.", true);
    return;
  }

  // Collect selected versions from dropdowns
  const withSelections = packages.map(pkg => {
    const select = document.getElementById("select-" + pkg.module);
    return {
      name: pkg.name,
      version: pkg.version,
      selectedVersion: select ? select.value : pkg.version,
    };
  });

  const versions = buildVersionMap(withSelections);

  if (Object.keys(versions).length === 0) {
    setStatus("No version changes selected.");
    return;
  }

  setStatus("Applying updates...");

  try {
    const data = await updateComposer(composerJSON, versions);
    // Update textarea with the new composer.json (pretty-printed)
    textarea.value = JSON.stringify(data.composer_json, null, 4);
    setStatus("Updated " + Object.keys(versions).length + " package(s). Reloading table...");
    await loadComposer();
  } catch (e) {
    setStatus("Error applying updates: " + e.message, true);
  }
}

// =============================================================================
// Rendering
// =============================================================================

/** Render the packages table with dropdowns. */
function renderTable() {
  if (packages.length === 0) {
    packagesBody.innerHTML = '<tr><td colspan="3">No Drupal packages found.</td></tr>';
    return;
  }

  packagesBody.innerHTML = "";
  for (const pkg of packages) {
    const row = document.createElement("tr");

    // Package name (linked to drupal.org)
    const nameCell = document.createElement("td");
    const link = document.createElement("a");
    link.href = "https://www.drupal.org/project/" + pkg.module + "#project-releases";
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.textContent = pkg.name;
    nameCell.appendChild(link);
    row.appendChild(nameCell);

    // Current version
    const versionCell = document.createElement("td");
    versionCell.textContent = pkg.version;
    row.appendChild(versionCell);

    // Dropdown for available versions
    const selectCell = document.createElement("td");
    if (pkg.releases.length > 0) {
      const select = document.createElement("select");
      select.id = "select-" + pkg.module;

      // "Keep current" option
      const keepOption = document.createElement("option");
      keepOption.value = pkg.version;
      keepOption.textContent = pkg.version + " (current)";
      select.appendChild(keepOption);

      // One option per available release
      for (const release of pkg.releases) {
        const option = document.createElement("option");
        option.value = "^" + release.version;
        const core = release.core_compatibility || "unknown";
        option.textContent = "^" + release.version + "  (core: " + core + ")";
        select.appendChild(option);
      }

      selectCell.appendChild(select);
    } else {
      selectCell.textContent = "Loading...";
    }
    row.appendChild(selectCell);

    packagesBody.appendChild(row);
  }
}

function setStatus(msg, isError = false) {
  statusEl.textContent = msg;
  statusEl.classList.toggle("error", isError);
}

// =============================================================================
// File Handling
// =============================================================================

/** Read a File object into the textarea and trigger loading. */
function handleFile(file) {
  // If we were editing, leave edit mode
  if (editing) {
    editing = false;
    textarea.readOnly = true;
    btnEdit.textContent = "Edit";
  }

  const reader = new FileReader();
  reader.onload = function (e) {
    textarea.value = e.target.result;
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
  if (fileInput.files.length > 0) handleFile(fileInput.files[0]);
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
  if (!editing && e.dataTransfer.files.length > 0) handleFile(e.dataTransfer.files[0]);
});
