//spellchecker:words drupalupdate
package drupalupdate

//spellchecker:words testing
import (
	"testing"
)

// =============================================================================
// DrupalModuleName
// =============================================================================

func TestDrupalModuleName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		wantName string
		wantOK   bool
	}{
		{"drupal/admin_toolbar", "admin_toolbar", true},
		{"drupal/core-recommended", "core-recommended", true},
		{"drush/drush", "", false},
		{"composer/installers", "", false},
	}
	for _, tt := range tests {
		name, ok := drupalModuleName(tt.input)
		if name != tt.wantName || ok != tt.wantOK {
			t.Errorf("drupalModuleName(%q) = (%q, %v), want (%q, %v)",
				tt.input, name, ok, tt.wantName, tt.wantOK)
		}
	}
}

func TestIsCorePackage(t *testing.T) {
	t.Parallel()
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
		{"core-something-else", true},
	}
	for _, tt := range tests {
		if got := isCorePackage(tt.name); got != tt.want {
			t.Errorf("isCorePackage(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsSkippablePackage(t *testing.T) {
	t.Parallel()
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
		if got := isSkippablePackage(tt.name); got != tt.want {
			t.Errorf("isSkippablePackage(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
