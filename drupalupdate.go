// Package drupalupdate provides functions for reading composer.json files
// and fetching release information from drupal.org and Packagist.
package drupalupdate

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
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
	Name              string `xml:"name" json:"name"`
	Version           string `xml:"version" json:"version"`
	Status            string `xml:"status" json:"-"`
	CoreCompatibility string `xml:"core_compatibility" json:"core_compatibility,omitempty"`
}

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
		return nil, err
	}
	c.Raw = data
	return &c, nil
}

// ReadComposerJSON reads and parses a composer.json file from disk.
func ReadComposerJSON(path string) (*ComposerJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseComposerJSON(data)
}

// MarshalComposerJSON serializes the composer.json back to bytes,
// preserving all original fields and only updating "require".
func MarshalComposerJSON(c *ComposerJSON) ([]byte, error) {
	var original map[string]any
	if err := json.Unmarshal(c.Raw, &original); err != nil {
		return nil, err
	}

	original["require"] = sortedRequire(c.Require)

	output, err := json.MarshalIndent(original, "", "    ")
	if err != nil {
		return nil, err
	}
	return append(output, '\n'), nil
}

// WriteComposerJSON writes the composer.json back to disk.
func WriteComposerJSON(path string, c *ComposerJSON) error {
	data, err := MarshalComposerJSON(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
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
// For drupal/* packages (excluding core), it queries drupal.org.
// For all other packages, it queries Packagist.
func (c *Client) FetchReleasesForPackage(packageName string) ([]Release, error) {
	if moduleName, ok := DrupalModuleName(packageName); ok && !IsCorePackage(moduleName) {
		return c.FetchReleases(moduleName)
	}
	return c.FetchPackagistReleases(packageName)
}

// =============================================================================
// Drupal.org Release API
// =============================================================================

// FetchReleases fetches the latest release per supported branch for a Drupal module.
func (c *Client) FetchReleases(moduleName string) ([]Release, error) {
	url := fmt.Sprintf("%s/%s/current", c.BaseURL, moduleName)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var history ReleaseHistory
	if err := xml.Unmarshal(body, &history); err != nil {
		return nil, err
	}

	branches := ParseSupportedBranches(history.SupportedBranches)
	return LatestPerBranch(history.Releases, branches), nil
}

// =============================================================================
// Packagist Release API
// =============================================================================

// FetchPackagistReleases fetches the latest stable release per major version
// from the Packagist p2 API.
func (c *Client) FetchPackagistReleases(packageName string) ([]Release, error) {
	url := fmt.Sprintf("%s/p2/%s.json", c.PackagistBaseURL, packageName)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result PackagistResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	versions := result.Packages[packageName]
	return LatestStablePerMajor(packageName, versions), nil
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
			Name:    packageName + " " + version,
			Version: version,
		})
	}
	return result
}

// =============================================================================
// Drupal Branch Filtering
// =============================================================================

// ParseSupportedBranches splits a comma-separated branches string.
// Example: "3.0.,4.0.,5.0." -> ["3.0.", "4.0.", "5.0."]
func ParseSupportedBranches(branches string) []string {
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

// LatestPerBranch returns the first (latest) published release for each supported branch.
// If no branches are given, it returns up to 10 published releases.
func LatestPerBranch(releases []Release, supportedBranches []string) []Release {
	if len(supportedBranches) == 0 {
		var result []Release
		for _, r := range releases {
			if r.Status == "published" {
				result = append(result, r)
				if len(result) >= 10 {
					break
				}
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
