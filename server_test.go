package drupalupdate_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
)

// newTestServer creates a Server backed by mock drupal.org and Packagist servers.
func newTestServer(t *testing.T) (*drupalupdate.Server, func()) {
	t.Helper()

	drupalMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin_toolbar/current" {
			w.Header().Set("Content-Type", "application/xml")
			if _, err := w.Write([]byte(sampleXML)); err != nil {
				return
			}
			return
		}
		if r.URL.Path == "/drupal/current" {
			w.Header().Set("Content-Type", "application/xml")
			if _, err := w.Write([]byte(sampleCoreXML)); err != nil {
				return
			}
			return
		}
		http.NotFound(w, r)
	}))

	packagistMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/p2/drush/drush.json" {
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(samplePackagistJSON)); err != nil {
				return
			}
			return
		}
		http.NotFound(w, r)
	}))

	client := drupalupdate.NewClientWithHTTP(drupalMock.URL, packagistMock.URL, &http.Client{})
	server := drupalupdate.NewServer(client)

	return server, func() {
		drupalMock.Close()
		packagistMock.Close()
	}
}

// sampleCoreXML is a minimal drupal.org release history response for Drupal core.
const sampleCoreXML = `<?xml version="1.0" encoding="utf-8"?>
<project xmlns:dc="http://purl.org/dc/elements/1.1/">
  <title>Drupal core</title>
  <short_name>drupal</short_name>
  <supported_branches>10.4.,11.0.,11.1.</supported_branches>
  <releases>
    <release>
      <name>drupal 11.1.0</name>
      <version>11.1.0</version>
      <status>published</status>
    </release>
    <release>
      <name>drupal 11.0.8</name>
      <version>11.0.8</version>
      <status>published</status>
    </release>
    <release>
      <name>drupal 10.4.3</name>
      <version>10.4.3</version>
      <status>published</status>
    </release>
  </releases>
</project>`

// =============================================================================
// POST /api/parse
// =============================================================================

func TestServer_Parse(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	body := `{
		"composer_json": {
			"require": {
				"drupal/admin_toolbar": "^3.6",
				"drupal/gin": "^5.0",
				"drupal/core-recommended": "^11",
				"drupal/core-composer-scaffold": "^11",
				"drush/drush": "^13",
				"php": ">=8.2"
			}
		}
	}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/parse", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp drupalupdate.ParseResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	// Should return core-recommended and core-composer-scaffold as core packages
	if len(resp.CorePackages) != 2 {
		t.Fatalf("expected 2 core packages, got %d", len(resp.CorePackages))
	}
	if resp.CorePackages[0].Name != "drupal/core-composer-scaffold" {
		t.Errorf("expected drupal/core-composer-scaffold, got %s", resp.CorePackages[0].Name)
	}
	if resp.CorePackages[1].Name != "drupal/core-recommended" {
		t.Errorf("expected drupal/core-recommended, got %s", resp.CorePackages[1].Name)
	}

	// Should return admin_toolbar and gin as Drupal packages
	if len(resp.DrupalPackages) != 2 {
		t.Fatalf("expected 2 drupal packages, got %d", len(resp.DrupalPackages))
	}
	if resp.DrupalPackages[0].Module != "admin_toolbar" {
		t.Errorf("expected admin_toolbar, got %s", resp.DrupalPackages[0].Module)
	}
	if resp.DrupalPackages[1].Module != "gin" {
		t.Errorf("expected gin, got %s", resp.DrupalPackages[1].Module)
	}

	// Should return drush as a Composer package (skip php and core packages)
	if len(resp.ComposerPackages) != 1 {
		t.Fatalf("expected 1 composer package, got %d", len(resp.ComposerPackages))
	}
	if resp.ComposerPackages[0].Name != "drush/drush" {
		t.Errorf("expected drush/drush, got %s", resp.ComposerPackages[0].Name)
	}
}

func TestServer_Parse_InvalidJSON(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/parse", bytes.NewBufferString("not json"))
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServer_Parse_InvalidComposerJSON(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	body := `{"composer_json": "this is a string, not an object"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/parse", bytes.NewBufferString(body))
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// GET /api/releases
// =============================================================================

func TestServer_Releases_Drupal(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/releases?package=drupal/admin_toolbar", nil)
	server.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp drupalupdate.ReleasesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Package != "drupal/admin_toolbar" {
		t.Errorf("expected package drupal/admin_toolbar, got %s", resp.Package)
	}
	if len(resp.Releases) != 2 {
		t.Fatalf("expected 2 releases (one per branch), got %d", len(resp.Releases))
	}
	// Sorted newest first
	if resp.Releases[0].Version != "4.0.2" {
		t.Errorf("expected 4.0.2, got %s", resp.Releases[0].Version)
	}
	if resp.Releases[1].Version != "3.0.5" {
		t.Errorf("expected 3.0.5, got %s", resp.Releases[1].Version)
	}
	if resp.Releases[0].VersionPin != "^4.0" {
		t.Errorf("expected version pin '^4.0', got %s", resp.Releases[0].VersionPin)
	}
	if resp.Releases[1].VersionPin != "^3.0" {
		t.Errorf("expected version pin '^3.0', got %s", resp.Releases[1].VersionPin)
	}
	if resp.Releases[0].CoreCompatibility != "^10.3 || ^11" {
		t.Errorf("expected '^10.3 || ^11', got %s", resp.Releases[0].CoreCompatibility)
	}
}

func TestServer_Releases_Packagist(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/releases?package=drush/drush", nil)
	server.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp drupalupdate.ReleasesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Package != "drush/drush" {
		t.Errorf("expected package drush/drush, got %s", resp.Package)
	}
	if len(resp.Releases) != 3 {
		t.Fatalf("expected 3 releases, got %d", len(resp.Releases))
	}
	if resp.Releases[0].Version != "13.0.1" {
		t.Errorf("expected 13.0.1, got %s", resp.Releases[0].Version)
	}
	if resp.Releases[0].VersionPin != "^13.0" {
		t.Errorf("expected version pin '^13.0', got %s", resp.Releases[0].VersionPin)
	}
}

func TestServer_Releases_Core(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/releases?package=drupal/core-recommended", nil)
	server.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp drupalupdate.ReleasesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Package != "drupal/core-recommended" {
		t.Errorf("expected package drupal/core-recommended, got %s", resp.Package)
	}
	if len(resp.Releases) != 3 {
		t.Fatalf("expected 3 releases (one per branch), got %d", len(resp.Releases))
	}
	// Sorted newest first
	if resp.Releases[0].Version != "11.1.0" {
		t.Errorf("expected 11.1.0, got %s", resp.Releases[0].Version)
	}
	if resp.Releases[0].VersionPin != "^11.1" {
		t.Errorf("expected version pin '^11.1', got %s", resp.Releases[0].VersionPin)
	}
	if resp.Releases[2].Version != "10.4.3" {
		t.Errorf("expected 10.4.3, got %s", resp.Releases[2].Version)
	}
	if resp.Releases[2].VersionPin != "^10.4" {
		t.Errorf("expected version pin '^10.4', got %s", resp.Releases[2].VersionPin)
	}
}

func TestServer_Releases_MissingPackage(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/releases", nil)
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServer_Releases_NotFound(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/releases?package=drupal/nonexistent", nil)
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

// =============================================================================
// POST /api/update
// =============================================================================

func TestServer_Update(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	body := `{
		"composer_json": {
			"name": "my/project",
			"require": {
				"drupal/admin_toolbar": "^3.6",
				"drupal/gin": "^5.0",
				"drush/drush": "^12"
			},
			"extra": {"key": "value"}
		},
		"versions": {
			"drupal/admin_toolbar": "^4.0",
			"drush/drush": "^13"
		}
	}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/update", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp drupalupdate.UpdateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	// Parse the returned composer.json to verify changes
	result, err := drupalupdate.ParseComposerJSON(resp.ComposerJSON)
	if err != nil {
		t.Fatal(err)
	}

	if result.Require["drupal/admin_toolbar"] != "^4.0" {
		t.Errorf("expected ^4.0, got %s", result.Require["drupal/admin_toolbar"])
	}
	if result.Require["drush/drush"] != "^13" {
		t.Errorf("expected ^13, got %s", result.Require["drush/drush"])
	}
	// gin should remain unchanged
	if result.Require["drupal/gin"] != "^5.0" {
		t.Errorf("expected ^5.0, got %s", result.Require["drupal/gin"])
	}

	// Verify extra fields are preserved
	var raw map[string]any
	if err := json.Unmarshal(resp.ComposerJSON, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["extra"]; !ok {
		t.Error("extra field was lost")
	}
	if _, ok := raw["name"]; !ok {
		t.Error("name field was lost")
	}
}

func TestServer_Update_InvalidJSON(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/update", bytes.NewBufferString("broken"))
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServer_Update_IgnoresUnknownPackages(t *testing.T) {
	t.Parallel()
	server, cleanup := newTestServer(t)
	defer cleanup()

	body := `{
		"composer_json": {
			"require": {
				"drupal/gin": "^5.0"
			}
		},
		"versions": {
			"drupal/nonexistent": "^1.0"
		}
	}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/update", bytes.NewBufferString(body))
	server.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp drupalupdate.UpdateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	result, err := drupalupdate.ParseComposerJSON(resp.ComposerJSON)
	if err != nil {
		t.Fatal(err)
	}

	// gin unchanged, nonexistent not added
	if result.Require["drupal/gin"] != "^5.0" {
		t.Errorf("expected ^5.0, got %s", result.Require["drupal/gin"])
	}
	if _, exists := result.Require["drupal/nonexistent"]; exists {
		t.Error("nonexistent package should not be added")
	}
}
