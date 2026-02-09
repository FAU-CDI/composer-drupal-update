//spellchecker:words drupalupdate
package drupalupdate

//spellchecker:words reflect testing
import (
	"reflect"
	"testing"
)

// =============================================================================
// ParseSupportedDrupalBranches
// =============================================================================

func TestParseSupportedDrupalBranches(t *testing.T) {
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
		got := parseSupportedDrupalBranches(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseSupportedDrupalBranches(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// =============================================================================
// LatestPerDrupalBranch
// =============================================================================

func TestLatestPerDrupalBranch_WithBranches(t *testing.T) {
	t.Parallel()
	releases := []Release{
		{Version: "5.0.3", Status: "published", CoreCompatibility: "^10.3 || ^11"},
		{Version: "5.0.2", Status: "published", CoreCompatibility: "^10.3 || ^11"},
		{Version: "4.0.1", Status: "published", CoreCompatibility: "^10"},
		{Version: "4.0.0", Status: "published", CoreCompatibility: "^10"},
		{Version: "3.0.5", Status: "published", CoreCompatibility: "^9 || ^10"},
	}
	branches := []string{"3.0.", "4.0.", "5.0."}

	got := latestPerDrupalBranch(releases, branches)

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

func TestLatestPerDrupalBranch_SkipsUnpublished(t *testing.T) {
	t.Parallel()
	releases := []Release{
		{Version: "2.0.1", Status: "unpublished"},
		{Version: "2.0.0", Status: "published", CoreCompatibility: "^10"},
	}
	branches := []string{"2.0."}

	got := latestPerDrupalBranch(releases, branches)

	if len(got) != 1 {
		t.Fatalf("expected 1 release, got %d", len(got))
	}
	if got[0].Version != "2.0.0" {
		t.Errorf("expected 2.0.0, got %s", got[0].Version)
	}
}

func TestLatestPerDrupalBranch_NoBranches_FallsBackToRecent(t *testing.T) {
	t.Parallel()
	releases := []Release{
		{Version: "3.0.0", Status: "published"},
		{Version: "2.0.0", Status: "published"},
		{Version: "1.0.0", Status: "published"},
	}

	got := latestPerDrupalBranch(releases, nil)

	if len(got) != 3 {
		t.Fatalf("expected 3 releases, got %d", len(got))
	}
}

func TestLatestPerDrupalBranch_MissingBranch(t *testing.T) {
	t.Parallel()
	releases := []Release{
		{Version: "5.0.1", Status: "published"},
	}
	branches := []string{"4.0.", "5.0."}

	got := latestPerDrupalBranch(releases, branches)

	// Only branch 5.0. has a release
	if len(got) != 1 {
		t.Fatalf("expected 1 release, got %d", len(got))
	}
	if got[0].Version != "5.0.1" {
		t.Errorf("expected 5.0.1, got %s", got[0].Version)
	}
}
