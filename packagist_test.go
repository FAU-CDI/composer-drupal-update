package drupalupdate_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
)

// =============================================================================
// FetchPackagistReleases (with mock HTTP server)
// =============================================================================

const samplePackagistJSON = `{
	"packages": {
		"drush/drush": [
			{"version": "13.0.1", "version_normalized": "13.0.1.0"},
			{"version": "13.0.0", "version_normalized": "13.0.0.0"},
			{"version": "13.0.0-rc1", "version_normalized": "13.0.0.0-RC1"},
			{"version": "12.5.6", "version_normalized": "12.5.6.0"},
			{"version": "12.4.0", "version_normalized": "12.4.0.0"},
			{"version": "dev-main", "version_normalized": "9999999-dev"},
			{"version": "11.0.0", "version_normalized": "11.0.0.0"}
		]
	}
}`

func TestFetchPackagistReleases(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/p2/drush/drush.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(samplePackagistJSON)); err != nil {
			return
		}
	}))
	defer server.Close()

	client := drupalupdate.NewClient()
	client.DrupalBaseURL = ""
	client.PackagistBaseURL = server.URL

	releases, err := client.FetchPackagistReleases(t.Context(), "drush/drush")
	if err != nil {
		t.Fatalf("FetchPackagistReleases returned error: %v", err)
	}

	// Should return latest stable per major: 13.0.1, 12.5.6, 11.0.0
	if len(releases) != 3 {
		t.Fatalf("expected 3 releases, got %d: %+v", len(releases), releases)
	}
	if releases[0].Version != "13.0.1" {
		t.Errorf("expected 13.0.1, got %s", releases[0].Version)
	}
	if releases[1].Version != "12.5.6" {
		t.Errorf("expected 12.5.6, got %s", releases[1].Version)
	}
	if releases[2].Version != "11.0.0" {
		t.Errorf("expected 11.0.0, got %s", releases[2].Version)
	}
}

func TestFetchPackagistReleases_NotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := drupalupdate.NewClient()
	client.DrupalBaseURL = ""
	client.PackagistBaseURL = server.URL

	_, err := client.FetchPackagistReleases(t.Context(), "nonexistent/package")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchPackagistReleases_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("not json")); err != nil {
			return
		}
	}))
	defer server.Close()

	client := drupalupdate.NewClient()
	client.DrupalBaseURL = ""
	client.PackagistBaseURL = server.URL

	_, err := client.FetchPackagistReleases(t.Context(), "broken/pkg")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
