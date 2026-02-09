// Package drupalupdate provides functions for reading composer.json files
// and fetching release information from drupal.org and Packagist.
//
//spellchecker:words drupalupdate
package drupalupdate

//spellchecker:words context encoding json errors http slices strings
import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// =============================================================================
// Data Structures
// =============================================================================

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
	errHTTPStatus = errors.New("unexpected HTTP status")
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
// Release API Client
// =============================================================================

const (
	// DefaultBaseURL is the default base URL for the drupal.org release history API.
	DefaultBaseURL = "https://updates.drupal.org/release-history"

	// DefaultPackagistBaseURL is the default base URL for the Packagist p2 API.
	DefaultPackagistBaseURL = "https://repo.packagist.org"
)

// Client fetches release information from drupal.org and Packagist.
// Use NewClient() or NewClientWithHTTP() to initialize new instances.
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
func (c *Client) FetchReleasesForPackage(ctx context.Context, pkg string) ([]Release, error) {
	if name, ok := drupalModuleName(pkg); ok {
		if isCorePackage(name) {
			return c.FetchReleases(ctx, "drupal")
		}
		return c.FetchReleases(ctx, name)
	}
	return c.FetchPackagistReleases(ctx, pkg)
}

// =============================================================================
// Drupal.org Release API
// =============================================================================

// FetchReleases fetches the latest release per supported branch for a Drupal module.
func (c *Client) FetchReleases(ctx context.Context, name string) (result []Release, err error) {
	url := fmt.Sprintf("%s/%s/current", c.BaseURL, name)

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

	var history ReleaseHistory
	if err := xml.NewDecoder(resp.Body).Decode(&history); err != nil {
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
func (c *Client) FetchPackagistReleases(ctx context.Context, pkg string) (releases []Release, err error) {
	url := fmt.Sprintf("%s/p2/%s.json", c.PackagistBaseURL, pkg)

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

	var result PackagistResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	versions := result.Packages[pkg]
	releases = LatestStablePerMajor(pkg, versions)
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
// If no branches are given, it returns all published releases.
func latestPerBranch(releases []Release, supportedBranches []string) []Release {
	if len(supportedBranches) == 0 {
		var result []Release
		for _, r := range releases {
			if r.Status != "published" {
				continue
			}
			result = append(result, r)
		}
		return result
	}

	found := make(map[string]Release)
	for _, r := range releases {
		if r.Status != "published" {
			continue
		}
		for _, branch := range supportedBranches {
			if !strings.HasPrefix(r.Version, branch) {
				continue
			}
			if _, exists := found[branch]; !exists {
				found[branch] = r
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
