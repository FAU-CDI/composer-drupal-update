//spellchecker:words drupalupdate
package drupalupdate_test

//spellchecker:words testing github composer drupal update drupalupdate
import (
	"testing"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
)

func TestParseVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input         string
		wantPrefix    string
		wantMajor     int
		wantMinor     int
		wantPatch     int
		wantStability string
	}{
		{"5.0.3", "", 5, 0, 3, ""},
		{"8.x-3.16", "8.x", 3, 16, -1, ""},
		{"11.1.0", "", 11, 1, 0, ""},
		{"8.x-1.0-rc17", "8.x", 1, 0, -1, "RC"},
		{"3.0.0-rc21", "", 3, 0, 0, "RC"},
		{"2.0.0-beta3", "", 2, 0, 0, "beta"},
		{"1.0.0-alpha1", "", 1, 0, 0, "alpha"},
		{"12.x-1.0.3-beta5", "12.x", 1, 0, 3, "beta"},
		{"42", "", 42, -1, -1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			v := drupalupdate.ParseVersion(tt.input)
			if v.Prefix != tt.wantPrefix {
				t.Errorf("ParseVersion(%q).Prefix = %q, want %q", tt.input, v.Prefix, tt.wantPrefix)
			}
			if v.Major != tt.wantMajor {
				t.Errorf("ParseVersion(%q).Major = %d, want %d", tt.input, v.Major, tt.wantMajor)
			}
			if v.Minor != tt.wantMinor {
				t.Errorf("ParseVersion(%q).Minor = %d, want %d", tt.input, v.Minor, tt.wantMinor)
			}
			if v.Patch != tt.wantPatch {
				t.Errorf("ParseVersion(%q).Patch = %d, want %d", tt.input, v.Patch, tt.wantPatch)
			}
			if v.Stability != tt.wantStability {
				t.Errorf("ParseVersion(%q).Stability = %q, want %q", tt.input, v.Stability, tt.wantStability)
			}
		})
	}
}

func TestVersion_VersionPin(t *testing.T) {
	t.Parallel()
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
		{"8.x-1.0-rc17", "^1.0@RC"},
		{"3.0.0-rc21", "^3.0@RC"},
		{"2.1.0-RC3", "^2.1@RC"},
		{"2.0.0-beta3", "^2.0@beta"},
		{"8.x-4.0-beta1", "^4.0@beta"},
		{"1.0.0-alpha1", "^1.0@alpha"},
		{"8.x-2.0-alpha5", "^2.0@alpha"},
		{"42", "^42"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			v := drupalupdate.ParseVersion(tt.input)
			if got := v.VersionPin(); got != tt.want {
				t.Errorf("ParseVersion(%q).VersionPin() = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersion_Compare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a    string
		b    string
		want int // >0 if a > b, <0 if a < b, 0 if equal
	}{
		{"11.1.0", "10.4.3", 1},
		{"10.4.3", "11.1.0", -1},
		{"4.0.2", "3.0.5", 1},
		{"3.0.5", "4.0.2", -1},
		{"11.1.0", "11.0.8", 1},
		{"11.0.8", "11.1.0", -1},
		{"8.x-1.5", "3.0.5", -1},
		{"3.0.5", "3.0.5", 0},
		{"42", "11.1.0", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			t.Parallel()
			va := drupalupdate.ParseVersion(tt.a)
			vb := drupalupdate.ParseVersion(tt.b)
			got := va.Compare(vb)
			// Compare returns >0, <0, or 0; we only check the sign
			if (got > 0) != (tt.want > 0) || (got < 0) != (tt.want < 0) || (got == 0) != (tt.want == 0) {
				t.Errorf("Compare(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
