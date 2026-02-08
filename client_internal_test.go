package drupalupdate

import (
	"reflect"
	"testing"
)

// =============================================================================
// ParseSupportedBranches
// =============================================================================

func TestParseSupportedBranches(t *testing.T) {
	t.Parallel()
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
// sortReleases (uses Version.Compare; Version tests in version_test.go)
// =============================================================================

func TestSortReleasesDescending(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
