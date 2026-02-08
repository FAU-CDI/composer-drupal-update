package drupalupdate

import (
	"reflect"
	"testing"
)

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
		got := parseSupportedBranches(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseSupportedBranches(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// =============================================================================
// VersionPin
// =============================================================================

func TestVersionPin(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"5.0.3", "^5.0"},
		{"4.0.2", "^4.0"},
		{"3.0.5", "^3.0"},
		{"11.1.0", "^11.1"},
		{"10.4.3", "^10.4"},
		{"13.0.1", "^13.0"},
		{"2.1.0", "^2.1"},
		{"3.16", "^3.16"},
		{"8.x-3.16", "^3.16"},
		{"8.x-1.5", "^1.5"},
		{"8.x-2.0", "^2.0"},
		{"1.0.0", "^1.0"},
		{"0.5.0", "^0.5"},
		// RC versions
		{"8.x-1.0-rc17", "^1.0@RC"},
		{"3.0.0-rc21", "^3.0@RC"},
		{"2.1.0-RC3", "^2.1@RC"},
		// Beta versions
		{"2.0.0-beta3", "^2.0@beta"},
		{"8.x-4.0-beta1", "^4.0@beta"},
		// Alpha versions
		{"1.0.0-alpha1", "^1.0@alpha"},
		{"8.x-2.0-alpha5", "^2.0@alpha"},
		// Edge case: single part
		{"42", "^42"},
	}
	for _, tt := range tests {
		if got := versionPin(tt.input); got != tt.want {
			t.Errorf("VersionPin(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSortReleasesDescending(t *testing.T) {
	releases := []Release{
		{Version: "3.0.5"},
		{Version: "11.1.0"},
		{Version: "4.0.2"},
		{Version: "10.4.3"},
		{Version: "8.x-1.5"},
		{Version: "11.0.8"},
	}
	sortReleases(releases)
	expected := []string{"11.1.0", "11.0.8", "10.4.3", "4.0.2", "3.0.5", "8.x-1.5"}
	for i, want := range expected {
		if releases[i].Version != want {
			t.Errorf("index %d: expected %s, got %s", i, want, releases[i].Version)
		}
	}
}

// =============================================================================
// LatestPerBranch
// =============================================================================

func TestLatestPerBranch_WithBranches(t *testing.T) {
	releases := []Release{
		{Version: "5.0.3", Status: "published", CoreCompatibility: "^10.3 || ^11"},
		{Version: "5.0.2", Status: "published", CoreCompatibility: "^10.3 || ^11"},
		{Version: "4.0.1", Status: "published", CoreCompatibility: "^10"},
		{Version: "4.0.0", Status: "published", CoreCompatibility: "^10"},
		{Version: "3.0.5", Status: "published", CoreCompatibility: "^9 || ^10"},
	}
	branches := []string{"3.0.", "4.0.", "5.0."}

	got := latestPerBranch(releases, branches)

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
	releases := []Release{
		{Version: "2.0.1", Status: "unpublished"},
		{Version: "2.0.0", Status: "published", CoreCompatibility: "^10"},
	}
	branches := []string{"2.0."}

	got := latestPerBranch(releases, branches)

	if len(got) != 1 {
		t.Fatalf("expected 1 release, got %d", len(got))
	}
	if got[0].Version != "2.0.0" {
		t.Errorf("expected 2.0.0, got %s", got[0].Version)
	}
}

func TestLatestPerBranch_NoBranches_FallsBackToRecent(t *testing.T) {
	releases := []Release{
		{Version: "3.0.0", Status: "published"},
		{Version: "2.0.0", Status: "published"},
		{Version: "1.0.0", Status: "published"},
	}

	got := latestPerBranch(releases, nil)

	if len(got) != 3 {
		t.Fatalf("expected 3 releases, got %d", len(got))
	}
}

func TestLatestPerBranch_MissingBranch(t *testing.T) {
	releases := []Release{
		{Version: "5.0.1", Status: "published"},
	}
	branches := []string{"4.0.", "5.0."}

	got := latestPerBranch(releases, branches)

	// Only branch 5.0. has a release
	if len(got) != 1 {
		t.Fatalf("expected 1 release, got %d", len(got))
	}
	if got[0].Version != "5.0.1" {
		t.Errorf("expected 5.0.1, got %s", got[0].Version)
	}
}
