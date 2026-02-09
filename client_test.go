//spellchecker:words drupalupdate
package drupalupdate_test

//spellchecker:words http httptest testing github composer drupal update drupalupdate
import (
	"net/http"
	"net/http/httptest"
	"testing"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
)

// =============================================================================
// FetchReleases (routing)
// =============================================================================

func TestFetchReleases_RoutesDrupal(t *testing.T) {
	t.Parallel()
	drupalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gin/current" {
			if _, err := w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<project>
					<supported_branches>5.0.</supported_branches>
					<releases>
						<release><name>gin 5.0.3</name><version>5.0.3</version><status>published</status></release>
					</releases>
				</project>`)); err != nil {
				return
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer drupalServer.Close()

	packagistServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Packagist should not be called for drupal/ packages")
		http.NotFound(w, r)
	}))
	defer packagistServer.Close()

	client := drupalupdate.NewClient()
	client.DrupalBaseURL = drupalServer.URL
	client.PackagistBaseURL = packagistServer.URL

	releases, err := client.FetchReleases(t.Context(), "drupal/gin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 || releases[0].Version != "5.0.3" {
		t.Errorf("unexpected releases: %+v", releases)
	}
}

func TestFetchReleases_RoutesCore(t *testing.T) {
	t.Parallel()
	drupalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/drupal/current" {
			if _, err := w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<project>
					<supported_branches>10.4.,11.0.,11.1.</supported_branches>
					<releases>
						<release><name>drupal 11.1.0</name><version>11.1.0</version><status>published</status></release>
						<release><name>drupal 11.0.8</name><version>11.0.8</version><status>published</status></release>
						<release><name>drupal 10.4.3</name><version>10.4.3</version><status>published</status></release>
					</releases>
				</project>`)); err != nil {
				return
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer drupalServer.Close()

	packagistServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Packagist should not be called for core packages")
		http.NotFound(w, r)
	}))
	defer packagistServer.Close()

	client := drupalupdate.NewClient()
	client.DrupalBaseURL = drupalServer.URL
	client.PackagistBaseURL = packagistServer.URL

	// Any core package name should route to drupal.org project "drupal"
	releases, err := client.FetchReleases(t.Context(), "drupal/core-recommended")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 3 {
		t.Fatalf("expected 3 releases, got %d: %+v", len(releases), releases)
	}
	// Should be sorted newest first
	if releases[0].Version != "11.1.0" {
		t.Errorf("expected 11.1.0, got %s", releases[0].Version)
	}
	if releases[1].Version != "11.0.8" {
		t.Errorf("expected 11.0.8, got %s", releases[1].Version)
	}
	if releases[2].Version != "10.4.3" {
		t.Errorf("expected 10.4.3, got %s", releases[2].Version)
	}
}

func TestFetchReleases_RoutesPackagist(t *testing.T) {
	t.Parallel()
	drupalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Drupal should not be called for non-drupal packages")
		http.NotFound(w, r)
	}))
	defer drupalServer.Close()

	packagistServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/p2/drush/drush.json" {
			if _, err := w.Write([]byte(samplePackagistJSON)); err != nil {
				return
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer packagistServer.Close()

	client := drupalupdate.NewClient()
	client.DrupalBaseURL = drupalServer.URL
	client.PackagistBaseURL = packagistServer.URL

	releases, err := client.FetchReleases(t.Context(), "drush/drush")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 3 {
		t.Errorf("expected 3 releases, got %d", len(releases))
	}
}

// =============================================================================
// checkPackageName (validation)
// =============================================================================

func TestFetchReleases_InvalidPackageName(t *testing.T) {
	t.Parallel()
	client := drupalupdate.NewClient()

	testCases := []struct {
		name        string
		packageName string
		wantErr     bool
	}{
		{
			name:        "empty string",
			packageName: "",
			wantErr:     true,
		},
		{
			name:        "no slash",
			packageName: "drupal",
			wantErr:     true,
		},
		{
			name:        "multiple slashes",
			packageName: "drupal/gin/extra",
			wantErr:     true,
		},
		{
			name:        "uppercase letters",
			packageName: "Drupal/Gin",
			wantErr:     true,
		},
		{
			name:        "starts with hyphen",
			packageName: "-drupal/gin",
			wantErr:     true,
		},
		{
			name:        "starts with underscore",
			packageName: "_drupal/gin",
			wantErr:     true,
		},
		{
			name:        "starts with dot",
			packageName: ".drupal/gin",
			wantErr:     true,
		},
		{
			name:        "empty vendor",
			packageName: "/gin",
			wantErr:     true,
		},
		{
			name:        "empty package",
			packageName: "drupal/",
			wantErr:     true,
		},
		{
			name:        "spaces",
			packageName: "drupal/gin module",
			wantErr:     true,
		},
		{
			name:        "special characters",
			packageName: "drupal/gin@module",
			wantErr:     true,
		},
		{
			name:        "valid with underscore",
			packageName: "drupal/admin_toolbar",
			wantErr:     false,
		},
		{
			name:        "valid with hyphen",
			packageName: "drupal/core-recommended",
			wantErr:     false,
		},
		{
			name:        "valid with dot",
			packageName: "vendor.name/package.name",
			wantErr:     false,
		},
		{
			name:        "valid simple",
			packageName: "drush/drush",
			wantErr:     false,
		},
		{
			name:        "valid with numbers",
			packageName: "vendor123/package456",
			wantErr:     false,
		},
		{
			name:        "valid starts with number",
			packageName: "123vendor/456package",
			wantErr:     false,
		},
		{
			name:        "invalid with dots",
			packageName: "../example",
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := client.FetchReleases(t.Context(), tc.packageName)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for package name %q, got nil", tc.packageName)
				} else {
					errMsg := err.Error()
					if errMsg == "" {
						t.Errorf("expected non-empty error message for package name %q", tc.packageName)
					} else if len(errMsg) < 20 || errMsg[:20] != "invalid package name" {
						t.Errorf("expected validation error for package name %q, got: %v", tc.packageName, err)
					}
				}
			} else {
				// For valid names, we might get network errors, but not validation errors
				// Check that the error is not about invalid package name
				if err != nil {
					errMsg := err.Error()
					if len(errMsg) >= 20 && errMsg[:20] == "invalid package name" {
						t.Errorf("unexpected validation error for valid package name %q: %v", tc.packageName, err)
					}
				}
			}
		})
	}
}
