package drupalupdate_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FAU-CDI/composer-drupal-update"
)

// =============================================================================
// ParseComposerJSON
// =============================================================================

func TestParseComposerJSON(t *testing.T) {
	input := []byte(`{
    "name": "drupal/example",
    "require": {
        "drupal/admin_toolbar": "^3.6",
        "drupal/core-recommended": "^11",
        "drush/drush": "^13"
    },
    "extra": {"key": "value"}
}`)

	c, err := drupalupdate.ParseComposerJSON(input)
	if err != nil {
		t.Fatalf("ParseComposerJSON returned error: %v", err)
	}

	if len(c.Require) != 3 {
		t.Fatalf("expected 3 require entries, got %d", len(c.Require))
	}
	if c.Require["drupal/admin_toolbar"] != "^3.6" {
		t.Errorf("expected ^3.6, got %s", c.Require["drupal/admin_toolbar"])
	}
	if c.Raw == nil {
		t.Error("expected Raw to be preserved")
	}
}

func TestParseComposerJSON_Invalid(t *testing.T) {
	_, err := drupalupdate.ParseComposerJSON([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// =============================================================================
// MarshalComposerJSON (round-trip)
// =============================================================================

func TestMarshalComposerJSON_PreservesExtraFields(t *testing.T) {
	input := []byte(`{
    "name": "drupal/example",
    "require": {
        "drupal/admin_toolbar": "^3.6"
    },
    "extra": {"key": "value"}
}`)

	c, err := drupalupdate.ParseComposerJSON(input)
	if err != nil {
		t.Fatal(err)
	}

	// Change a version
	c.Require["drupal/admin_toolbar"] = "^4.0"

	output, err := drupalupdate.MarshalComposerJSON(c)
	if err != nil {
		t.Fatal(err)
	}

	// Re-parse to verify
	c2, err := drupalupdate.ParseComposerJSON(output)
	if err != nil {
		t.Fatal(err)
	}

	if c2.Require["drupal/admin_toolbar"] != "^4.0" {
		t.Errorf("expected ^4.0, got %s", c2.Require["drupal/admin_toolbar"])
	}

	// Check that "extra" field survived the round-trip
	var raw map[string]any
	if err := json.Unmarshal(output, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["extra"]; !ok {
		t.Error("extra field was lost during round-trip")
	}
	if _, ok := raw["name"]; !ok {
		t.Error("name field was lost during round-trip")
	}
}

func TestMarshalComposerJSON_SortsRequire(t *testing.T) {
	input := []byte(`{"require": {"z/z": "1", "a/a": "2", "m/m": "3"}}`)

	c, err := drupalupdate.ParseComposerJSON(input)
	if err != nil {
		t.Fatal(err)
	}

	output, err := drupalupdate.MarshalComposerJSON(c)
	if err != nil {
		t.Fatal(err)
	}

	// Re-parse and check values are correct
	c2, err := drupalupdate.ParseComposerJSON(output)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Require["a/a"] != "2" || c2.Require["m/m"] != "3" || c2.Require["z/z"] != "1" {
		t.Error("require values were corrupted")
	}
}

// =============================================================================
// ReadComposerJSON / WriteComposerJSON (file I/O)
// =============================================================================

func TestReadWriteComposerJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "composer.json")

	original := []byte(`{
    "name": "test",
    "require": {
        "drupal/foo": "^1.0"
    }
}`)
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	c, err := drupalupdate.ReadComposerJSON(path)
	if err != nil {
		t.Fatal(err)
	}

	c.Require["drupal/foo"] = "^2.0"

	if err := drupalupdate.WriteComposerJSON(path, c); err != nil {
		t.Fatal(err)
	}

	c2, err := drupalupdate.ReadComposerJSON(path)
	if err != nil {
		t.Fatal(err)
	}

	if c2.Require["drupal/foo"] != "^2.0" {
		t.Errorf("expected ^2.0 after write, got %s", c2.Require["drupal/foo"])
	}
}

func TestReadComposerJSON_FileNotFound(t *testing.T) {
	_, err := drupalupdate.ReadComposerJSON("/nonexistent/composer.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// =============================================================================
// IsCorePackage
// =============================================================================

func TestIsCorePackage(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"core", true},
		{"core-recommended", true},
		{"core-composer-scaffold", true},
		{"core-project-message", true},
		{"core-dev", true},
		{"admin_toolbar", false},
		{"gin", false},
		{"core-something-else", false},
	}
	for _, tt := range tests {
		if got := drupalupdate.IsCorePackage(tt.name); got != tt.want {
			t.Errorf("IsCorePackage(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// =============================================================================
// DrupalModuleName
// =============================================================================

func TestDrupalModuleName(t *testing.T) {
	tests := []struct {
		input        string
		wantName     string
		wantIsDrupal bool
	}{
		{"drupal/admin_toolbar", "admin_toolbar", true},
		{"drupal/core-recommended", "core-recommended", true},
		{"drush/drush", "", false},
		{"composer/installers", "", false},
	}
	for _, tt := range tests {
		name, ok := drupalupdate.DrupalModuleName(tt.input)
		if name != tt.wantName || ok != tt.wantIsDrupal {
			t.Errorf("DrupalModuleName(%q) = (%q, %v), want (%q, %v)",
				tt.input, name, ok, tt.wantName, tt.wantIsDrupal)
		}
	}
}

// =============================================================================
// ParseSupportedBranches
// =============================================================================

func TestParseSupportedBranches(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"3.0.", []string{"3.0."}},
		{"3.0.,4.0.,5.0.", []string{"3.0.", "4.0.", "5.0."}},
		{"8.x-1.,8.x-2.", []string{"8.x-1.", "8.x-2."}},
		{" 3.0. , 4.0. ", []string{"3.0.", "4.0."}},
	}
	for _, tt := range tests {
		got := drupalupdate.ParseSupportedBranches(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseSupportedBranches(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// =============================================================================
// LatestPerBranch
// =============================================================================

func TestLatestPerBranch_WithBranches(t *testing.T) {
	releases := []drupalupdate.Release{
		{Version: "5.0.3", Status: "published", CoreCompatibility: "^10.3 || ^11"},
		{Version: "5.0.2", Status: "published", CoreCompatibility: "^10.3 || ^11"},
		{Version: "4.0.1", Status: "published", CoreCompatibility: "^10"},
		{Version: "4.0.0", Status: "published", CoreCompatibility: "^10"},
		{Version: "3.0.5", Status: "published", CoreCompatibility: "^9 || ^10"},
	}
	branches := []string{"3.0.", "4.0.", "5.0."}

	got := drupalupdate.LatestPerBranch(releases, branches)

	// Should return one per branch, in branch order
	if len(got) != 3 {
		t.Fatalf("expected 3 releases, got %d", len(got))
	}
	if got[0].Version != "3.0.5" {
		t.Errorf("branch 3.0.: got %s, want 3.0.5", got[0].Version)
	}
	if got[1].Version != "4.0.1" {
		t.Errorf("branch 4.0.: got %s, want 4.0.1", got[1].Version)
	}
	if got[2].Version != "5.0.3" {
		t.Errorf("branch 5.0.: got %s, want 5.0.3", got[2].Version)
	}
}

func TestLatestPerBranch_SkipsUnpublished(t *testing.T) {
	releases := []drupalupdate.Release{
		{Version: "2.0.1", Status: "unpublished"},
		{Version: "2.0.0", Status: "published", CoreCompatibility: "^10"},
	}
	branches := []string{"2.0."}

	got := drupalupdate.LatestPerBranch(releases, branches)

	if len(got) != 1 {
		t.Fatalf("expected 1 release, got %d", len(got))
	}
	if got[0].Version != "2.0.0" {
		t.Errorf("expected 2.0.0, got %s", got[0].Version)
	}
}

func TestLatestPerBranch_NoBranches_FallsBackToRecent(t *testing.T) {
	releases := []drupalupdate.Release{
		{Version: "3.0.0", Status: "published"},
		{Version: "2.0.0", Status: "published"},
		{Version: "1.0.0", Status: "published"},
	}

	got := drupalupdate.LatestPerBranch(releases, nil)

	if len(got) != 3 {
		t.Fatalf("expected 3 releases, got %d", len(got))
	}
}

func TestLatestPerBranch_NoBranches_LimitsTo10(t *testing.T) {
	var releases []drupalupdate.Release
	for i := 20; i >= 1; i-- {
		releases = append(releases, drupalupdate.Release{
			Version: fmt.Sprintf("1.0.%d", i),
			Status:  "published",
		})
	}

	got := drupalupdate.LatestPerBranch(releases, nil)

	if len(got) != 10 {
		t.Fatalf("expected 10 releases, got %d", len(got))
	}
}

func TestLatestPerBranch_MissingBranch(t *testing.T) {
	releases := []drupalupdate.Release{
		{Version: "5.0.1", Status: "published"},
	}
	branches := []string{"4.0.", "5.0."}

	got := drupalupdate.LatestPerBranch(releases, branches)

	// Only branch 5.0. has a release
	if len(got) != 1 {
		t.Fatalf("expected 1 release, got %d", len(got))
	}
	if got[0].Version != "5.0.1" {
		t.Errorf("expected 5.0.1, got %s", got[0].Version)
	}
}

// =============================================================================
// FetchReleases (with mock HTTP server)
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

func TestFetchReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin_toolbar/current" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(sampleXML))
	}))
	defer server.Close()

	client := drupalupdate.NewClientWithHTTP(server.URL, server.Client())

	releases, err := client.FetchReleases("admin_toolbar")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	// Should return latest per branch: 3.0.5 and 4.0.2
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}
	if releases[0].Version != "3.0.5" {
		t.Errorf("expected 3.0.5, got %s", releases[0].Version)
	}
	if releases[1].Version != "4.0.2" {
		t.Errorf("expected 4.0.2, got %s", releases[1].Version)
	}
	if releases[1].CoreCompatibility != "^10.3 || ^11" {
		t.Errorf("expected core compat '^10.3 || ^11', got %s", releases[1].CoreCompatibility)
	}
}

func TestFetchReleases_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := drupalupdate.NewClientWithHTTP(server.URL, server.Client())

	_, err := client.FetchReleases("nonexistent_module")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchReleases_InvalidXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this is not xml"))
	}))
	defer server.Close()

	client := drupalupdate.NewClientWithHTTP(server.URL, server.Client())

	_, err := client.FetchReleases("broken")
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}
