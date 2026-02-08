package drupalupdate_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
)

// newTestServer creates a Server backed by a mock drupal.org that serves sampleXML.
func newTestServer(t *testing.T) (*drupalupdate.Server, func()) {
	t.Helper()

	drupalMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin_toolbar/current" {
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(sampleXML))
			return
		}
		http.NotFound(w, r)
	}))

	client := drupalupdate.NewClientWithHTTP(drupalMock.URL, drupalMock.Client())
	server := drupalupdate.NewServer(client)

	return server, drupalMock.Close
}

// =============================================================================
// POST /api/parse
// =============================================================================

func TestServer_Parse(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	body := `{
		"composer_json": {
			"require": {
				"drupal/admin_toolbar": "^3.6",
				"drupal/gin": "^5.0",
				"drupal/core-recommended": "^11",
				"drush/drush": "^13"
			}
		}
	}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/parse", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp drupalupdate.ParseResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	// Should return admin_toolbar and gin (skip core-recommended and drush)
	if len(resp.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(resp.Packages))
	}

	// Sorted: admin_toolbar first, gin second
	if resp.Packages[0].Module != "admin_toolbar" {
		t.Errorf("expected admin_toolbar, got %s", resp.Packages[0].Module)
	}
	if resp.Packages[1].Module != "gin" {
		t.Errorf("expected gin, got %s", resp.Packages[1].Module)
	}
	if resp.Packages[0].Version != "^3.6" {
		t.Errorf("expected ^3.6, got %s", resp.Packages[0].Version)
	}
}

func TestServer_Parse_InvalidJSON(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/parse", bytes.NewBufferString("not json"))
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServer_Parse_InvalidComposerJSON(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	body := `{"composer_json": "this is a string, not an object"}`
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/parse", bytes.NewBufferString(body))
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// GET /api/releases
// =============================================================================

func TestServer_Releases(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/releases?module=admin_toolbar", nil)
	server.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp drupalupdate.ReleasesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Module != "admin_toolbar" {
		t.Errorf("expected module admin_toolbar, got %s", resp.Module)
	}
	if len(resp.Releases) != 2 {
		t.Fatalf("expected 2 releases (one per branch), got %d", len(resp.Releases))
	}
	if resp.Releases[0].Version != "3.0.5" {
		t.Errorf("expected 3.0.5, got %s", resp.Releases[0].Version)
	}
	if resp.Releases[1].Version != "4.0.2" {
		t.Errorf("expected 4.0.2, got %s", resp.Releases[1].Version)
	}
	if resp.Releases[1].CoreCompatibility != "^10.3 || ^11" {
		t.Errorf("expected '^10.3 || ^11', got %s", resp.Releases[1].CoreCompatibility)
	}
}

func TestServer_Releases_MissingModule(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/releases", nil)
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServer_Releases_NotFound(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/releases?module=nonexistent", nil)
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

// =============================================================================
// POST /api/update
// =============================================================================

func TestServer_Update(t *testing.T) {
	server, cleanup := newTestServer(t)
	defer cleanup()

	body := `{
		"composer_json": {
			"name": "my/project",
			"require": {
				"drupal/admin_toolbar": "^3.6",
				"drupal/gin": "^5.0"
			},
			"extra": {"key": "value"}
		},
		"versions": {
			"drupal/admin_toolbar": "^4.0"
		}
	}`

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/update", bytes.NewBufferString(body))
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
	server, cleanup := newTestServer(t)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/update", bytes.NewBufferString("broken"))
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestServer_Update_IgnoresUnknownPackages(t *testing.T) {
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
	r := httptest.NewRequest("POST", "/api/update", bytes.NewBufferString(body))
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
