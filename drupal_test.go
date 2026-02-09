package drupalupdate_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
)

// =============================================================================
// FetchReleases (Drupal, with mock HTTP server)
// =============================================================================

// sampleXML is a minimal drupal.org release history response for testing.
const sampleXML = `<?xml version="1.0" encoding="utf-8"?>
<project xmlns:dc="http://purl.org/dc/elements/1.1/">
  <title>Admin Toolbar</title>
  <short_name>admin_toolbar</short_name>
  <supported_branches>3.0.,4.0.</supported_branches>
  <releases>
    <release>
      <name>admin_toolbar 4.0.2</name>
      <version>4.0.2</version>
      <status>published</status>
      <core_compatibility>^10.3 || ^11</core_compatibility>
    </release>
    <release>
      <name>admin_toolbar 4.0.1</name>
      <version>4.0.1</version>
      <status>published</status>
      <core_compatibility>^10.3 || ^11</core_compatibility>
    </release>
    <release>
      <name>admin_toolbar 3.0.5</name>
      <version>3.0.5</version>
      <status>published</status>
      <core_compatibility>^9 || ^10</core_compatibility>
    </release>
    <release>
      <name>admin_toolbar 3.0.4</name>
      <version>3.0.4</version>
      <status>published</status>
      <core_compatibility>^9 || ^10</core_compatibility>
    </release>
  </releases>
</project>`

func TestFetchDrupalReleases(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin_toolbar/current" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		if _, err := w.Write([]byte(sampleXML)); err != nil {
			return
		}
	}))
	defer server.Close()

	client := drupalupdate.NewClient()
	client.DrupalBaseURL = server.URL
	client.PackagistBaseURL = ""

	releases, err := client.FetchDrupalReleases(t.Context(), "admin_toolbar")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	// Should return latest per branch, sorted newest first: 4.0.2, 3.0.5
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}
	if releases[0].Version != "4.0.2" {
		t.Errorf("expected 4.0.2, got %s", releases[0].Version)
	}
	if releases[1].Version != "3.0.5" {
		t.Errorf("expected 3.0.5, got %s", releases[1].Version)
	}
	if releases[0].CoreCompatibility != "^10.3 || ^11" {
		t.Errorf("expected core compat '^10.3 || ^11', got %s", releases[0].CoreCompatibility)
	}
	if releases[0].VersionPin != "^4.0" {
		t.Errorf("expected version pin '^4.0', got %s", releases[0].VersionPin)
	}
	if releases[1].VersionPin != "^3.0" {
		t.Errorf("expected version pin '^3.0', got %s", releases[1].VersionPin)
	}
}

func TestFetchDrupalReleases_NotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := drupalupdate.NewClient()
	client.DrupalBaseURL = server.URL
	client.PackagistBaseURL = ""

	_, err := client.FetchDrupalReleases(t.Context(), "nonexistent_module")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchDrupalReleases_InvalidXML(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("this is not xml")); err != nil {
			return
		}
	}))
	defer server.Close()

	client := drupalupdate.NewClient()
	client.DrupalBaseURL = server.URL
	client.PackagistBaseURL = ""

	_, err := client.FetchDrupalReleases(t.Context(), "broken")
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}
