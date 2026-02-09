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
// Packagist Version Filtering
// =============================================================================

func TestIsStableVersion(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	// VersionPin should drop patch
	if got[0].VersionPin != "^13.0" {
		t.Errorf("expected version pin '^13.0', got %s", got[0].VersionPin)
	}
	if got[1].VersionPin != "^12.5" {
		t.Errorf("expected version pin '^12.5', got %s", got[1].VersionPin)
	}
}

func TestLatestStablePerMajor_StripsVPrefix(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

	client := drupalupdate.NewClientWithHTTP(server.URL, "", &http.Client{})

	releases, err := client.FetchReleases(t.Context(), "admin_toolbar")
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

func TestFetchReleases_NotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := drupalupdate.NewClientWithHTTP(server.URL, "", &http.Client{})

	_, err := client.FetchReleases(t.Context(), "nonexistent_module")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchReleases_InvalidXML(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("this is not xml")); err != nil {
			return
		}
	}))
	defer server.Close()

	client := drupalupdate.NewClientWithHTTP(server.URL, "", &http.Client{})

	_, err := client.FetchReleases(t.Context(), "broken")
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

	client := drupalupdate.NewClientWithHTTP("", server.URL, &http.Client{})

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

	client := drupalupdate.NewClientWithHTTP("", server.URL, &http.Client{})

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

	client := drupalupdate.NewClientWithHTTP("", server.URL, &http.Client{})

	_, err := client.FetchPackagistReleases(t.Context(), "broken/pkg")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// =============================================================================
// FetchReleasesForPackage (routing)
// =============================================================================

func TestFetchReleasesForPackage_RoutesDrupal(t *testing.T) {
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

	client := drupalupdate.NewClientWithHTTP(drupalServer.URL, packagistServer.URL, &http.Client{})

	releases, err := client.FetchReleasesForPackage(t.Context(), "drupal/gin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 || releases[0].Version != "5.0.3" {
		t.Errorf("unexpected releases: %+v", releases)
	}
}

func TestFetchReleasesForPackage_RoutesCore(t *testing.T) {
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

	client := drupalupdate.NewClientWithHTTP(drupalServer.URL, packagistServer.URL, &http.Client{})

	// Any core package name should route to drupal.org project "drupal"
	releases, err := client.FetchReleasesForPackage(t.Context(), "drupal/core-recommended")
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

func TestFetchReleasesForPackage_RoutesPackagist(t *testing.T) {
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

	client := drupalupdate.NewClientWithHTTP(drupalServer.URL, packagistServer.URL, &http.Client{})

	releases, err := client.FetchReleasesForPackage(t.Context(), "drush/drush")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 3 {
		t.Errorf("expected 3 releases, got %d", len(releases))
	}
}
