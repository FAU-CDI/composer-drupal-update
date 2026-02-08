// Package drupalupdate provides functions for reading composer.json files
// and fetching Drupal module release information from drupal.org.
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

// Release represents a single release from drupal.org.
type Release struct {
	Name              string `xml:"name" json:"name"`
	Version           string `xml:"version" json:"version"`
	Status            string `xml:"status" json:"-"`
	CoreCompatibility string `xml:"core_compatibility" json:"core_compatibility"`
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
// Drupal Package Helpers
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

// Package represents a drupal module found in composer.json.
type Package struct {
	Name    string `json:"name"`    // composer package name, e.g. "drupal/gin"
	Module  string `json:"module"`  // drupal module name, e.g. "gin"
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

// =============================================================================
// Drupal.org Release API
// =============================================================================

// DefaultBaseURL is the default base URL for the drupal.org release history API.
const DefaultBaseURL = "https://updates.drupal.org/release-history"

// Client fetches release information from drupal.org.
// Use NewClient() for production or NewClientWithHTTP() for testing.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a Client that talks to the real drupal.org API.
func NewClient() *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		HTTPClient: http.DefaultClient,
	}
}

// NewClientWithHTTP creates a Client with a custom HTTP client and base URL.
// This is useful for testing with httptest.Server.
func NewClientWithHTTP(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: httpClient,
	}
}

// FetchReleases fetches the latest release per supported branch for a module.
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
// Branch Filtering
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
