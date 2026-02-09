//spellchecker:words drupalupdate
package drupalupdate_test

//spellchecker:words encoding json testing github composer drupal update drupalupdate
import (
	"encoding/json"
	"testing"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
)

// =============================================================================
// ParseComposerJSON
// =============================================================================

func TestParseComposerJSON(t *testing.T) {
	t.Parallel()
	input := []byte(`{
    "name": "drupal/example",
    "require": {
        "drupal/admin_toolbar": "^3.6",
        "drupal/core-recommended": "^11",
        "drush/drush": "^13"
    },
    "extra": {"key": "value"}
}`)

	var c drupalupdate.ComposerJSON
	err := json.Unmarshal(input, &c)
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
	t.Parallel()
	var c drupalupdate.ComposerJSON
	err := json.Unmarshal([]byte(`not json`), &c)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// =============================================================================
// MarshalComposerJSON (round-trip)
// =============================================================================

func TestMarshalComposerJSON_PreservesExtraFields(t *testing.T) {
	t.Parallel()
	input := []byte(`{
    "name": "drupal/example",
    "require": {
        "drupal/admin_toolbar": "^3.6"
    },
    "extra": {"key": "value"}
}`)

	var c drupalupdate.ComposerJSON
	err := json.Unmarshal(input, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Change a version
	c.Require["drupal/admin_toolbar"] = "^4.0"

	output, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	// Re-parse to verify
	var c2 drupalupdate.ComposerJSON
	err = json.Unmarshal(output, &c2)
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
	t.Parallel()
	input := []byte(`{"require": {"z/z": "1", "a/a": "2", "m/m": "3"}}`)

	var c drupalupdate.ComposerJSON
	err := json.Unmarshal(input, &c)
	if err != nil {
		t.Fatal(err)
	}

	output, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	// Re-parse and check values are correct
	var c2 drupalupdate.ComposerJSON
	err = json.Unmarshal(output, &c2)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Require["a/a"] != "2" || c2.Require["m/m"] != "3" || c2.Require["z/z"] != "1" {
		t.Error("require values were corrupted")
	}
}

// =============================================================================
// DrupalPackages
// =============================================================================

func TestDrupalPackages(t *testing.T) {
	t.Parallel()
	input := []byte(`{
    "require": {
        "drupal/admin_toolbar": "^3.6",
        "drupal/gin": "^5.0",
        "drupal/core-recommended": "^11",
        "drush/drush": "^13",
        "php": ">=8.2"
    }
}`)
	var c drupalupdate.ComposerJSON
	err := json.Unmarshal(input, &c)
	if err != nil {
		t.Fatal(err)
	}

	pkgs := c.DrupalPackages()
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
	t.Parallel()
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
	var c drupalupdate.ComposerJSON
	err := json.Unmarshal(input, &c)
	if err != nil {
		t.Fatal(err)
	}

	pkgs := c.ComposerPackages()
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
	t.Parallel()
	input := []byte(`{
    "require": {
        "drupal/gin": "^5.0",
        "php": ">=8.2"
    }
}`)
	var c drupalupdate.ComposerJSON
	err := json.Unmarshal(input, &c)
	if err != nil {
		t.Fatal(err)
	}

	pkgs := c.ComposerPackages()
	if len(pkgs) != 0 {
		t.Fatalf("expected 0 composer packages, got %d", len(pkgs))
	}
}

// =============================================================================
// CorePackages
// =============================================================================

func TestCorePackages(t *testing.T) {
	t.Parallel()
	input := []byte(`{
    "require": {
        "drupal/core-recommended": "^11",
        "drupal/core-composer-scaffold": "^11",
        "drupal/core-project-message": "^11",
        "drupal/admin_toolbar": "^3.6",
        "drupal/gin": "^5.0",
        "drush/drush": "^13",
        "php": ">=8.2"
    }
}`)
	var c drupalupdate.ComposerJSON
	err := json.Unmarshal(input, &c)
	if err != nil {
		t.Fatal(err)
	}

	pkgs := c.CorePackages()
	if len(pkgs) != 3 {
		t.Fatalf("expected 3 core packages, got %d: %+v", len(pkgs), pkgs)
	}
	// Sorted alphabetically
	if pkgs[0].Name != "drupal/core-composer-scaffold" {
		t.Errorf("expected drupal/core-composer-scaffold first, got %s", pkgs[0].Name)
	}
	if pkgs[1].Name != "drupal/core-project-message" {
		t.Errorf("expected drupal/core-project-message second, got %s", pkgs[1].Name)
	}
	if pkgs[2].Name != "drupal/core-recommended" {
		t.Errorf("expected drupal/core-recommended third, got %s", pkgs[2].Name)
	}
	if pkgs[2].Version != "^11" {
		t.Errorf("expected version ^11, got %s", pkgs[2].Version)
	}
}

func TestCorePackages_Empty(t *testing.T) {
	t.Parallel()
	input := []byte(`{
    "require": {
        "drupal/gin": "^5.0",
        "drush/drush": "^13"
    }
}`)
	var c drupalupdate.ComposerJSON
	err := json.Unmarshal(input, &c)
	if err != nil {
		t.Fatal(err)
	}

	pkgs := c.CorePackages()
	if len(pkgs) != 0 {
		t.Fatalf("expected 0 core packages, got %d", len(pkgs))
	}
}
