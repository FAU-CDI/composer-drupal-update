package drupalupdate

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// FetchDrupalReleases fetches the latest release per supported branch for a Drupal module.
func (c *Client) FetchDrupalReleases(ctx context.Context, name string) (result []Release, err error) {
	return fetchResponse(ctx, c, fmt.Sprintf("%s/%s/current", c.DrupalBaseURL, name), func(body io.Reader) ([]Release, error) {
		var history struct {
			XMLName           xml.Name  `xml:"project"`
			Title             string    `xml:"title"`
			SupportedBranches string    `xml:"supported_branches"`
			Releases          []Release `xml:"releases>release"`
		}
		if err := xml.NewDecoder(body).Decode(&history); err != nil {
			return nil, fmt.Errorf("decode XML: %w", err)
		}

		branches := parseSupportedDrupalBranches(history.SupportedBranches)
		result = latestPerDrupalBranch(history.Releases, branches)
		for i := range result {
			result[i].VersionPin = ParseVersion(result[i].Version).VersionPin()
		}
		sortReleases(result)
		return result, nil
	})
}

// parseSupportedDrupalBranches splits a comma-separated branches string.
// Example: "3.0.,4.0.,5.0." -> ["3.0.", "4.0.", "5.0."].
func parseSupportedDrupalBranches(branches string) []string {
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

// latestPerDrupalBranch returns the first (latest) published release for each supported branch.
// If no branches are given, it returns all published releases.
func latestPerDrupalBranch(releases []Release, supportedBranches []string) []Release {
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
