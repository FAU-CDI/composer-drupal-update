// Package drupalupdate provides functions for reading composer.json files
// and fetching release information from drupal.org and Packagist.
package drupalupdate

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// =============================================================================
// Data Structures
// =============================================================================

// ComposerJSON represents the structure of a composer.json file.
type ComposerJSON struct {
	Require map[string]string `json:"require,omitempty"`

	// Raw stores the original JSON bytes for round-trip preservation.
	Raw json.RawMessage `json:"-"`
}

// ReleaseHistory is the XML response from drupal.org's release-history API.
type ReleaseHistory struct {
	XMLName           xml.Name  `xml:"project"`
	Title             string    `xml:"title"`
	SupportedBranches string    `xml:"supported_branches"`
	Releases          []Release `xml:"releases>release"`
}

// Release represents a single release from drupal.org or Packagist.
type Release struct {
	Name              string `json:"name"                         xml:"name"`
	Version           string `json:"version"                      xml:"version"`
	VersionPin        string `json:"version_pin"`
	Status            string `json:"-"                            xml:"status"`
	CoreCompatibility string `json:"core_compatibility,omitempty" xml:"core_compatibility"`
}

// errHTTPStatus is returned when an HTTP request returns a non-OK status.
var (
	errHTTPStatus  = errors.New("unexpected HTTP status")
	errInvalidPath = errors.New("invalid path")
)

// PackagistResponse represents the response from the Packagist p2 API.
type PackagistResponse struct {
	Packages map[string][]PackagistVersion `json:"packages"`
}

// PackagistVersion represents a single version entry from the Packagist API.
type PackagistVersion struct {
	Version           string `json:"version"`
	VersionNormalized string `json:"version_normalized"`
}

// =============================================================================
// Composer JSON I/O
// =============================================================================

// ParseComposerJSON parses a composer.json from raw bytes.
func ParseComposerJSON(data []byte) (*ComposerJSON, error) {
	var c ComposerJSON
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse composer.json: %w", err)
	}
	c.Raw = data
	return &c, nil
}

// ReadComposerJSON reads and parses a composer.json file from disk.
func ReadComposerJSON(path string) (*ComposerJSON, error) {
	path = filepath.Clean(path)
	if path == "" || path == "." || strings.Contains(path, "..") {
		return nil, fmt.Errorf("%w: %s", errInvalidPath, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseComposerJSON(data)
}

// MarshalComposerJSON serializes the composer.json back to bytes,
// preserving all original fields and only updating "require".
func MarshalComposerJSON(c *ComposerJSON) ([]byte, error) {
	var original map[string]any
	if err := json.Unmarshal(c.Raw, &original); err != nil {
		return nil, fmt.Errorf("unmarshal raw: %w", err)
	}

	original["require"] = sortedRequire(c.Require)

	output, err := json.MarshalIndent(original, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return append(output, '\n'), nil
}

// WriteComposerJSON writes the composer.json back to disk.
func WriteComposerJSON(path string, c *ComposerJSON) error {
	data, err := MarshalComposerJSON(c)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// sortedRequire returns require entries in sorted key order.
func sortedRequire(require map[string]string) map[string]string {
	keys := make([]string, 0, len(require))
	for k := range require {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sorted := make(map[string]string, len(require))
	for _, k := range keys {
		sorted[k] = require[k]
	}
	return sorted
}

// =============================================================================
// Package Classification
// =============================================================================

// IsCorePackage returns true if the drupal module name is a core package
// that should be skipped (e.g. "core-recommended", "core-composer-scaffold").
func IsCorePackage(moduleName string) bool {
	switch moduleName {
	case "core", "core-recommended", "core-composer-scaffold", "core-project-message", "core-dev":
		return true
	}
	return false
}

// DrupalModuleName extracts the module name from a composer package name.
// Returns the module name and true if it's a drupal/* package, or ("", false) otherwise.
func DrupalModuleName(packageName string) (string, bool) {
	if !strings.HasPrefix(packageName, "drupal/") {
		return "", false
	}
	return strings.TrimPrefix(packageName, "drupal/"), true
}

// IsSkippablePackage returns true for packages that should not be offered
// for version updates. This includes PHP itself, PHP extensions, and Drupal
// core infrastructure packages.
func IsSkippablePackage(name string) bool {
	if name == "php" || name == "composer" || strings.HasPrefix(name, "ext-") || strings.HasPrefix(name, "lib-") {
		return true
	}
	if moduleName, ok := DrupalModuleName(name); ok && IsCorePackage(moduleName) {
		return true
	}
	return false
}

// Package represents a composer package found in composer.json.
type Package struct {
	Name    string `json:"name"`    // composer package name, e.g. "drupal/gin" or "drush/drush"
	Module  string `json:"module"`  // identifier for fetching releases (drupal module name or full package name)
	Version string `json:"version"` // current version constraint, e.g. "^5.0"
}

// DrupalPackages extracts all updatable drupal modules from a ComposerJSON.
// It skips non-drupal packages and core packages.
func DrupalPackages(c *ComposerJSON) []Package {
	var pkgs []Package
	for name, version := range c.Require {
		module, ok := DrupalModuleName(name)
		if !ok || IsCorePackage(module) {
			continue
		}
		pkgs = append(pkgs, Package{Name: name, Module: module, Version: version})
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Name < pkgs[j].Name
	})
	return pkgs
}

// CorePackages extracts all Drupal core packages from a ComposerJSON.
// These are packages like drupal/core, drupal/core-recommended, etc.
func CorePackages(c *ComposerJSON) []Package {
	var pkgs []Package
	for name, version := range c.Require {
		module, ok := DrupalModuleName(name)
		if !ok || !IsCorePackage(module) {
			continue
		}
		pkgs = append(pkgs, Package{Name: name, Module: module, Version: version})
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Name < pkgs[j].Name
	})
	return pkgs
}

// ComposerPackages extracts all non-Drupal, non-skippable packages from a ComposerJSON.
func ComposerPackages(c *ComposerJSON) []Package {
	var pkgs []Package
	for name, version := range c.Require {
		if IsSkippablePackage(name) {
			continue
		}
		if _, isDrupal := DrupalModuleName(name); isDrupal {
			continue
		}
		pkgs = append(pkgs, Package{Name: name, Module: name, Version: version})
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Name < pkgs[j].Name
	})
	return pkgs
}

// =============================================================================
// Release API Client
// =============================================================================

// DefaultBaseURL is the default base URL for the drupal.org release history API.
const DefaultBaseURL = "https://updates.drupal.org/release-history"

// DefaultPackagistBaseURL is the default base URL for the Packagist p2 API.
const DefaultPackagistBaseURL = "https://repo.packagist.org"

// Client fetches release information from drupal.org and Packagist.
// Use NewClient() for production or NewClientWithHTTP() for testing.
type Client struct {
	BaseURL          string
	PackagistBaseURL string
	HTTPClient       *http.Client
}

// NewClient creates a Client that talks to the real drupal.org and Packagist APIs.
func NewClient() *Client {
	return &Client{
		BaseURL:          DefaultBaseURL,
		PackagistBaseURL: DefaultPackagistBaseURL,
		HTTPClient:       http.DefaultClient,
	}
}

// NewClientWithHTTP creates a Client with custom base URLs and HTTP client.
// This is useful for testing with httptest.Server.
func NewClientWithHTTP(drupalBaseURL, packagistBaseURL string, httpClient *http.Client) *Client {
	return &Client{
		BaseURL:          drupalBaseURL,
		PackagistBaseURL: packagistBaseURL,
		HTTPClient:       httpClient,
	}
}

// FetchReleasesForPackage fetches releases for any composer package.
// For drupal core packages (drupal/core, drupal/core-recommended, etc.),
// it queries the "drupal" project on drupal.org.
// For other drupal/* packages, it queries drupal.org by module name.
// For all other packages, it queries Packagist.
func (c *Client) FetchReleasesForPackage(ctx context.Context, packageName string) ([]Release, error) {
	if moduleName, ok := DrupalModuleName(packageName); ok {
		if IsCorePackage(moduleName) {
			return c.FetchReleases(ctx, "drupal")
		}
		return c.FetchReleases(ctx, moduleName)
	}
	return c.FetchPackagistReleases(ctx, packageName)
}

// =============================================================================
// Drupal.org Release API
// =============================================================================

// FetchReleases fetches the latest release per supported branch for a Drupal module.
func (c *Client) FetchReleases(ctx context.Context, moduleName string) (result []Release, err error) {
	url := fmt.Sprintf("%s/%s/current", c.BaseURL, moduleName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", errHTTPStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var history ReleaseHistory
	if err := xml.Unmarshal(body, &history); err != nil {
		return nil, fmt.Errorf("decode XML: %w", err)
	}

	branches := parseSupportedBranches(history.SupportedBranches)
	result = latestPerBranch(history.Releases, branches)
	for i := range result {
		result[i].VersionPin = ParseVersion(result[i].Version).VersionPin()
	}
	sortReleases(result)
	return result, nil
}

// =============================================================================
// Packagist Release API
// =============================================================================

// FetchPackagistReleases fetches the latest stable release per major version
// from the Packagist p2 API.
func (c *Client) FetchPackagistReleases(ctx context.Context, packageName string) (releases []Release, err error) {
	url := fmt.Sprintf("%s/p2/%s.json", c.PackagistBaseURL, packageName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", errHTTPStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result PackagistResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	versions := result.Packages[packageName]
	releases = LatestStablePerMajor(packageName, versions)
	sortReleases(releases)
	return releases, nil
}

// =============================================================================
// Packagist Version Filtering
// =============================================================================

// IsStableVersion returns true if the Packagist version is a stable release
// (not dev, alpha, beta, or RC).
func IsStableVersion(v PackagistVersion) bool {
	lower := strings.ToLower(v.Version)
	for _, tag := range []string{"dev", "alpha", "beta", "rc"} {
		if strings.Contains(lower, tag) {
			return false
		}
	}
	return true
}

// MajorVersion extracts the major version number from a normalized version string
// (e.g. "13.0.1.0" -> "13").
func MajorVersion(versionNormalized string) string {
	parts := strings.SplitN(versionNormalized, ".", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// LatestStablePerMajor filters Packagist versions to the latest stable release
// per major version. Versions are assumed to be ordered newest-first.
func LatestStablePerMajor(packageName string, versions []PackagistVersion) []Release {
	seen := make(map[string]bool)
	var result []Release
	for _, v := range versions {
		if !IsStableVersion(v) {
			continue
		}
		major := MajorVersion(v.VersionNormalized)
		if seen[major] {
			continue
		}
		seen[major] = true
		version := strings.TrimPrefix(v.Version, "v")
		result = append(result, Release{
			Name:       packageName + " " + version,
			Version:    version,
			VersionPin: ParseVersion(version).VersionPin(),
		})
	}
	return result
}

// =============================================================================
// Drupal Branch Filtering
// =============================================================================

// parseSupportedBranches splits a comma-separated branches string.
// Example: "3.0.,4.0.,5.0." -> ["3.0.", "4.0.", "5.0."].
func parseSupportedBranches(branches string) []string {
	if branches == "" {
		return nil
	}
	parts := strings.Split(branches, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// sortReleases sorts releases by version in descending order (newest first).
func sortReleases(releases []Release) {
	slices.SortFunc(releases, func(a, b Release) int {
		return ParseVersion(b.Version).Compare(ParseVersion(a.Version))
	})
}

// latestPerBranch returns the first (latest) published release for each supported branch.
// If no branches are given, it returns up to 10 published releases.
func latestPerBranch(releases []Release, supportedBranches []string) []Release {
	if len(supportedBranches) == 0 {
		var result []Release
		for _, r := range releases {
			if r.Status == "published" {
				result = append(result, r)
			}
		}
		return result
	}

	found := make(map[string]Release)
	for _, r := range releases {
		if r.Status != "published" {
			continue
		}
		for _, branch := range supportedBranches {
			if strings.HasPrefix(r.Version, branch) {
				if _, exists := found[branch]; !exists {
					found[branch] = r
				}
				break
			}
		}
	}

	var result []Release
	for _, branch := range supportedBranches {
		if r, ok := found[branch]; ok {
			result = append(result, r)
		}
	}
	return result
}
