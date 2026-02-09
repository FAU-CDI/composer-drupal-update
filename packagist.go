package drupalupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// FetchPackagistReleases fetches the latest stable release per major version
// from the Packagist p2 API.
func (c *Client) FetchPackagistReleases(ctx context.Context, pkg string) (releases []Release, err error) {
	return fetchResponse(ctx, c, fmt.Sprintf("%s/p2/%s.json", c.PackagistBaseURL, pkg), func(body io.Reader) ([]Release, error) {
		var result struct {
			Packages map[string][]packagistVersion `json:"packages"`
		}
		if err := json.NewDecoder(body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode JSON: %w", err)
		}

		versions := result.Packages[pkg]
		releases = latestStablePerPackagistMajor(pkg, versions)
		sortReleases(releases)
		return releases, nil
	})
}

// =============================================================================
// Packagist Version Filtering
// =============================================================================

// packagistVersion represents a single version entry from the Packagist API.
type packagistVersion struct {
	Version           string `json:"version"`
	VersionNormalized string `json:"version_normalized"`
}

// isStable returns true if the Packagist version is a stable release
// (not dev, alpha, beta, or RC).
func (v packagistVersion) isStable() bool {
	lower := strings.ToLower(v.Version)
	for _, tag := range []string{"dev", "alpha", "beta", "rc"} {
		if strings.Contains(lower, tag) {
			return false
		}
	}
	return true
}

// packagistMajorVersion extracts the major version number from a packagist version
// (e.g. "13.0.1.0" -> "13").
func (v packagistVersion) majorVersion() string {
	parts := strings.SplitN(v.VersionNormalized, ".", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// latestStablePerPackagistMajor filters Packagist versions to the latest stable release
// per major version. Versions are assumed to be ordered newest-first.
func latestStablePerPackagistMajor(pkg string, versions []packagistVersion) []Release {
	seen := make(map[string]bool)
	var result []Release
	for _, v := range versions {
		if !v.isStable() {
			continue
		}
		major := v.majorVersion()
		if seen[major] {
			continue
		}
		seen[major] = true
		version := strings.TrimPrefix(v.Version, "v")
		result = append(result, Release{
			Name:       pkg + " " + version,
			Version:    version,
			VersionPin: ParseVersion(version).VersionPin(),
		})
	}
	return result
}
