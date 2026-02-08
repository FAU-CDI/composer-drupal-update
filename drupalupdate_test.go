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
// IsSkippablePackage
// =============================================================================

func TestIsSkippablePackage(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"php", true},
		{"composer", true},
		{"ext-json", true},
		{"ext-mbstring", true},
		{"lib-openssl", true},
		{"drupal/core-recommended", true},
		{"drupal/core-composer-scaffold", true},
		{"drupal/core", true},
		{"drupal/admin_toolbar", false},
		{"drupal/gin", false},
		{"drush/drush", false},
		{"composer/installers", false},
		{"cweagans/composer-patches", false},
	}
	for _, tt := range tests {
		if got := drupalupdate.IsSkippablePackage(tt.name); got != tt.want {
			t.Errorf("IsSkippablePackage(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// =============================================================================
// DrupalPackages
// =============================================================================

func TestDrupalPackages(t *testing.T) {
	input := []byte(`{
    "require": {
        "drupal/admin_toolbar": "^3.6",
        "drupal/gin": "^5.0",
        "drupal/core-recommended": "^11",
        "drush/drush": "^13",
        "php": ">=8.2"
    }
}`)
	c, err := drupalupdate.ParseComposerJSON(input)
	if err != nil {
		t.Fatal(err)
	}

	pkgs := drupalupdate.DrupalPackages(c)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 drupal packages, got %d", len(pkgs))
	}
	// Sorted: admin_toolbar, gin
	if pkgs[0].Name != "drupal/admin_toolbar" || pkgs[0].Module != "admin_toolbar" {
		t.Errorf("unexpected first package: %+v", pkgs[0])
	}
	if pkgs[1].Name != "drupal/gin" || pkgs[1].Module != "gin" {
		t.Errorf("unexpected second package: %+v", pkgs[1])
	}
}

// =============================================================================
// ComposerPackages
// =============================================================================

func TestComposerPackages(t *testing.T) {
	input := []byte(`{
    "require": {
        "drupal/admin_toolbar": "^3.6",
        "drupal/core-recommended": "^11",
        "drush/drush": "^13",
        "composer/installers": "^2.0",
        "cweagans/composer-patches": "^1.7",
        "php": ">=8.2",
        "ext-json": "*"
    }
}`)
	c, err := drupalupdate.ParseComposerJSON(input)
	if err != nil {
		t.Fatal(err)
	}

	pkgs := drupalupdate.ComposerPackages(c)
	// Should include: composer/installers, cweagans/composer-patches, drush/drush
	// Should skip: drupal/*, php, ext-json
	if len(pkgs) != 3 {
		t.Fatalf("expected 3 composer packages, got %d: %+v", len(pkgs), pkgs)
	}

	// Sorted alphabetically
	if pkgs[0].Name != "composer/installers" {
		t.Errorf("expected composer/installers first, got %s", pkgs[0].Name)
	}
	if pkgs[1].Name != "cweagans/composer-patches" {
		t.Errorf("expected cweagans/composer-patches second, got %s", pkgs[1].Name)
	}
	if pkgs[2].Name != "drush/drush" {
		t.Errorf("expected drush/drush third, got %s", pkgs[2].Name)
	}

	// Module should be the full package name for composer packages
	if pkgs[2].Module != "drush/drush" {
		t.Errorf("expected module 'drush/drush', got %s", pkgs[2].Module)
	}
}

func TestComposerPackages_Empty(t *testing.T) {
	input := []byte(`{
    "require": {
        "drupal/gin": "^5.0",
        "php": ">=8.2"
    }
}`)
	c, err := drupalupdate.ParseComposerJSON(input)
	if err != nil {
		t.Fatal(err)
	}

	pkgs := drupalupdate.ComposerPackages(c)
	if len(pkgs) != 0 {
		t.Fatalf("expected 0 composer packages, got %d", len(pkgs))
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
// Packagist Version Filtering
// =============================================================================

func TestIsStableVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"13.0.1", true},
		{"v2.1.0", true},
		{"1.0.0", true},
		{"dev-main", false},
		{"dev-master", false},
		{"2.0.0-alpha1", false},
		{"3.0.0-beta2", false},
		{"4.0.0-RC1", false},
		{"4.0.0-rc1", false},
		{"1.0.x-dev", false},
	}
	for _, tt := range tests {
		v := drupalupdate.PackagistVersion{Version: tt.version, VersionNormalized: "1.0.0.0"}
		if got := drupalupdate.IsStableVersion(v); got != tt.want {
			t.Errorf("IsStableVersion(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestMajorVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"13.0.1.0", "13"},
		{"2.1.0.0", "2"},
		{"0.5.0.0", "0"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := drupalupdate.MajorVersion(tt.input); got != tt.want {
			t.Errorf("MajorVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLatestStablePerMajor(t *testing.T) {
	versions := []drupalupdate.PackagistVersion{
		{Version: "13.0.1", VersionNormalized: "13.0.1.0"},
		{Version: "13.0.0", VersionNormalized: "13.0.0.0"},
		{Version: "13.0.0-rc1", VersionNormalized: "13.0.0.0-RC1"},
		{Version: "12.5.6", VersionNormalized: "12.5.6.0"},
		{Version: "12.4.0", VersionNormalized: "12.4.0.0"},
		{Version: "dev-main", VersionNormalized: "9999999-dev"},
		{Version: "11.0.0", VersionNormalized: "11.0.0.0"},
	}

	got := drupalupdate.LatestStablePerMajor("drush/drush", versions)

	if len(got) != 3 {
		t.Fatalf("expected 3 releases, got %d: %+v", len(got), got)
	}
	if got[0].Version != "13.0.1" {
		t.Errorf("expected 13.0.1, got %s", got[0].Version)
	}
	if got[1].Version != "12.5.6" {
		t.Errorf("expected 12.5.6, got %s", got[1].Version)
	}
	if got[2].Version != "11.0.0" {
		t.Errorf("expected 11.0.0, got %s", got[2].Version)
	}
	if got[0].Name != "drush/drush 13.0.1" {
		t.Errorf("expected name 'drush/drush 13.0.1', got %s", got[0].Name)
	}
	// CoreCompatibility should be empty for Packagist releases
	if got[0].CoreCompatibility != "" {
		t.Errorf("expected empty core_compatibility, got %s", got[0].CoreCompatibility)
	}
}

func TestLatestStablePerMajor_StripsVPrefix(t *testing.T) {
	versions := []drupalupdate.PackagistVersion{
		{Version: "v2.1.0", VersionNormalized: "2.1.0.0"},
		{Version: "v1.5.0", VersionNormalized: "1.5.0.0"},
	}

	got := drupalupdate.LatestStablePerMajor("some/pkg", versions)

	if len(got) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(got))
	}
	if got[0].Version != "2.1.0" {
		t.Errorf("expected 2.1.0, got %s", got[0].Version)
	}
	if got[1].Version != "1.5.0" {
		t.Errorf("expected 1.5.0, got %s", got[1].Version)
	}
}

func TestLatestStablePerMajor_AllUnstable(t *testing.T) {
	versions := []drupalupdate.PackagistVersion{
		{Version: "dev-main", VersionNormalized: "9999999-dev"},
		{Version: "1.0.0-alpha1", VersionNormalized: "1.0.0.0-alpha1"},
	}

	got := drupalupdate.LatestStablePerMajor("pkg/x", versions)

	if len(got) != 0 {
		t.Fatalf("expected 0 releases, got %d", len(got))
	}
}

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

	client := drupalupdate.NewClientWithHTTP(server.URL, "", &http.Client{})

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

	client := drupalupdate.NewClientWithHTTP(server.URL, "", &http.Client{})

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

	client := drupalupdate.NewClientWithHTTP(server.URL, "", &http.Client{})

	_, err := client.FetchReleases("broken")
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/p2/drush/drush.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(samplePackagistJSON))
	}))
	defer server.Close()

	client := drupalupdate.NewClientWithHTTP("", server.URL, &http.Client{})

	releases, err := client.FetchPackagistReleases("drush/drush")
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := drupalupdate.NewClientWithHTTP("", server.URL, &http.Client{})

	_, err := client.FetchPackagistReleases("nonexistent/package")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchPackagistReleases_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := drupalupdate.NewClientWithHTTP("", server.URL, &http.Client{})

	_, err := client.FetchPackagistReleases("broken/pkg")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// =============================================================================
// FetchReleasesForPackage (routing)
// =============================================================================

func TestFetchReleasesForPackage_RoutesDrupal(t *testing.T) {
	drupalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gin/current" {
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
				<project>
					<supported_branches>5.0.</supported_branches>
					<releases>
						<release><name>gin 5.0.3</name><version>5.0.3</version><status>published</status></release>
					</releases>
				</project>`))
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

	client := drupalupdate.NewClientWithHTTP(drupalServer.URL, packagistServer.URL, &http.Client{})

	releases, err := client.FetchReleasesForPackage("drupal/gin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 || releases[0].Version != "5.0.3" {
		t.Errorf("unexpected releases: %+v", releases)
	}
}

func TestFetchReleasesForPackage_RoutesPackagist(t *testing.T) {
	drupalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Drupal should not be called for non-drupal packages")
		http.NotFound(w, r)
	}))
	defer drupalServer.Close()

	packagistServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/p2/drush/drush.json" {
			w.Write([]byte(samplePackagistJSON))
			return
		}
		http.NotFound(w, r)
	}))
	defer packagistServer.Close()

	client := drupalupdate.NewClientWithHTTP(drupalServer.URL, packagistServer.URL, &http.Client{})

	releases, err := client.FetchReleasesForPackage("drush/drush")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 3 {
		t.Errorf("expected 3 releases, got %d", len(releases))
	}
}
