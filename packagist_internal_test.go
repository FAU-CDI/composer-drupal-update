package drupalupdate

import (
	"testing"
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
		v := packagistVersion{Version: tt.version, VersionNormalized: "1.0.0.0"}
		if got := v.isStable(); got != tt.want {
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
		v := packagistVersion{Version: "unused", VersionNormalized: tt.input}
		if got := v.majorVersion(); got != tt.want {
			t.Errorf("MajorVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLatestStablePerMajor(t *testing.T) {
	t.Parallel()
	versions := []packagistVersion{
		{Version: "13.0.1", VersionNormalized: "13.0.1.0"},
		{Version: "13.0.0", VersionNormalized: "13.0.0.0"},
		{Version: "13.0.0-rc1", VersionNormalized: "13.0.0.0-RC1"},
		{Version: "12.5.6", VersionNormalized: "12.5.6.0"},
		{Version: "12.4.0", VersionNormalized: "12.4.0.0"},
		{Version: "dev-main", VersionNormalized: "9999999-dev"},
		{Version: "11.0.0", VersionNormalized: "11.0.0.0"},
	}

	got := latestStablePerPackagistMajor("drush/drush", versions)

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
	versions := []packagistVersion{
		{Version: "v2.1.0", VersionNormalized: "2.1.0.0"},
		{Version: "v1.5.0", VersionNormalized: "1.5.0.0"},
	}

	got := latestStablePerPackagistMajor("some/pkg", versions)

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
	versions := []packagistVersion{
		{Version: "dev-main", VersionNormalized: "9999999-dev"},
		{Version: "1.0.0-alpha1", VersionNormalized: "1.0.0.0-alpha1"},
	}

	got := latestStablePerPackagistMajor("pkg/x", versions)

	if len(got) != 0 {
		t.Fatalf("expected 0 releases, got %d", len(got))
	}
}
