// Package drupalupdate provides functions for reading composer.json files
// and fetching release information from drupal.org and Packagist.
//
//spellchecker:words drupalupdate
package drupalupdate

//spellchecker:words context encoding json errors http slices strings regexp
import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
)

// =============================================================================
// Data Structures
// =============================================================================

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

// =============================================================================
// Release API Client
// =============================================================================

const (
	// DefaultDrupalBaseURL is the default base URL for the drupal.org release history API.
	DefaultDrupalBaseURL = "https://updates.drupal.org/release-history"

	// DefaultPackagistBaseURL is the default base URL for the Packagist p2 API.
	DefaultPackagistBaseURL = "https://repo.packagist.org"
)

// Client fetches release information from drupal.org and Packagist.
// Use [NewClient] to initialize new instances.
type Client struct {
	HTTPClient *http.Client

	DrupalBaseURL    string // base URL for drupal updates release history API
	PackagistBaseURL string // base URL for packagist p2 API
}

// NewClient creates a Client that talks to the real drupal.org and Packagist APIs.
func NewClient() *Client {
	return &Client{
		DrupalBaseURL:    DefaultDrupalBaseURL,
		PackagistBaseURL: DefaultPackagistBaseURL,
		HTTPClient:       http.DefaultClient,
	}
}

// FetchReleases fetches releases for any composer package.
// For drupal core packages (drupal/core, drupal/core-recommended, etc.),
// it queries the "drupal" project on drupal.org.
// For other drupal/* packages, it queries drupal.org by module name.
// For all other packages, it queries Packagist.
func (c *Client) FetchReleases(ctx context.Context, pkg string) ([]Release, error) {
	if err := checkPackageName(pkg); err != nil {
		return nil, fmt.Errorf("invalid package name: %w", err)
	}
	if name, ok := drupalModuleName(pkg); ok {
		if isCorePackage(name) {
			return c.FetchDrupalReleases(ctx, "drupal")
		}
		return c.FetchDrupalReleases(ctx, name)
	}
	return c.FetchPackagistReleases(ctx, pkg)
}

// fetchResponse fetches a response from a URL and parses it using a parser function.
func fetchResponse[T any](ctx context.Context, client *Client, url string, parser func(io.Reader) (T, error)) (t T, e error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return t, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return t, fmt.Errorf("request: %w", err)
	}
	defer func() {
		err := resp.Body.Close()
		if err == nil {
			return
		}
		e = errors.Join(e, err)
	}()

	if resp.StatusCode != http.StatusOK {
		return t, fmt.Errorf("%w: %d", errHTTPStatus, resp.StatusCode)
	}

	return parser(resp.Body)
}

// sortReleases sorts releases by version in descending order (newest first).
func sortReleases(releases []Release) {
	// Parse all the versions once
	sortElems := make([]releaseAndVersion, len(releases))
	for i, release := range releases {
		sortElems[i] = releaseAndVersion{
			Version: ParseVersion(release.Version),
			Release: release,
		}
	}

	// sort by the versions.
	slices.SortFunc(sortElems, func(a, b releaseAndVersion) int {
		return b.Version.Compare(a.Version)
	})

	// and finally put the releases in the right order.
	for i := range sortElems {
		releases[i] = sortElems[i].Release
	}
}

type releaseAndVersion struct {
	Release Release
	Version Version
}

var (
	packageNameRegex = regexp.MustCompile(`^[a-z0-9]([_.-]?[a-z0-9]+)*/[a-z0-9]([_.-]?[a-z0-9]+)*$`)

	errPackageNameEmpty  = errors.New("package name cannot be empty")
	errPackageNameFormat = errors.New("package name must match format vendor/package (e.g., drupal/gin or drush/drush)")
)

// checkPackageName checks if a package name is valid.
// Valid package names follow the format vendor/package where both parts
// contain only lowercase letters, numbers, hyphens, underscores, and dots.
func checkPackageName(pkg string) error {
	if pkg == "" {
		return errPackageNameEmpty
	}
	if !packageNameRegex.MatchString(pkg) {
		return errPackageNameFormat
	}
	return nil
}
