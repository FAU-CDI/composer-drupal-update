package drupalupdate

import "testing"

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
