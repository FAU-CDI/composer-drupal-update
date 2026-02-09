//spellchecker:words drupalupdate
package drupalupdate

//spellchecker:words encoding json maps sort strings
import (
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
)

// ComposerJSON represents the structure of a composer.json file.
type ComposerJSON struct {
	Require map[string]string // required dependencies, set to nil to remove dependencies entirely.

	Raw map[string]json.RawMessage // original JSON, for round-trip on extra fields
}

// UnmarshalJSON implements json.Unmarshaler for ComposerJSON.
func (c *ComposerJSON) UnmarshalJSON(data []byte) error {
	// First unmarshal everything into the raw map.
	// and then extract the 'require' key if it exists.
	if err := json.Unmarshal(data, &c.Raw); err != nil {
		return fmt.Errorf("composerJSON must be a map: %w", err)
	}
	require, ok := c.Raw["require"]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(require, &c.Require); err != nil {
		return fmt.Errorf("failed to unmarshal require key: %w", err)
	}
	return nil
}

// MarshalJSON implements json.Marshaler for ComposerJSON.
// It preserves all original fields and only updates "require".
func (c ComposerJSON) MarshalJSON() ([]byte, error) {
	original := maps.Clone(c.Raw)

	// Re-marshal the require key unless it is nil!
	if c.Require != nil {
		var err error
		original["require"], err = json.Marshal(c.Require)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal require key: %w", err)
		}
	} else {
		delete(original, "require")
	}

	output, err := json.MarshalIndent(original, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return append(output, '\n'), nil
}

// =============================================================================
// Package Logic
// =============================================================================

// Package represents a composer package found in composer.json.
type Package struct {
	Name    string `json:"name"`    // composer package name, e.g. "drupal/gin" or "drush/drush"
	Module  string `json:"module"`  // identifier for fetching releases (drupal module name or full package name)
	Version string `json:"version"` // current version constraint, e.g. "^5.0"
}

// filterPackages iterates over c.Require and collects packages that match the filter.
// The filter function should return the module name and true if the package should be included.
func (c *ComposerJSON) filterPackages(filter func(name, version string) (module string, include bool)) []Package {
	var pkgs []Package
	for name, version := range c.Require {
		module, include := filter(name, version)
		if !include {
			continue
		}
		pkgs = append(pkgs, Package{Name: name, Module: module, Version: version})
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].Name < pkgs[j].Name
	})
	return pkgs
}

// DrupalPackages extracts all updatable drupal modules from a ComposerJSON.
// It skips non-drupal packages and core packages.
func (c *ComposerJSON) DrupalPackages() []Package {
	return c.filterPackages(func(name, version string) (string, bool) {
		module, ok := drupalModuleName(name)
		return module, ok && !isCorePackage(module)
	})
}

// CorePackages extracts all Drupal core packages from a ComposerJSON.
// These are packages like drupal/core, drupal/core-recommended, etc.
func (c *ComposerJSON) CorePackages() []Package {
	return c.filterPackages(func(name, version string) (string, bool) {
		module, ok := drupalModuleName(name)
		return module, ok && isCorePackage(module)
	})
}

// ComposerPackages extracts all non-Drupal, non-skippable packages from a ComposerJSON.
func (c *ComposerJSON) ComposerPackages() []Package {
	return c.filterPackages(func(name, version string) (string, bool) {
		if isSkippablePackage(name) {
			return "", false
		}
		if _, isDrupal := drupalModuleName(name); isDrupal {
			return "", false
		}
		return name, true
	})
}

// =============================================================================
// Package Classification
// =============================================================================

// drupalModuleName extracts the module name from a composer package name.
// Returns the module name and true if it's a drupal/* package, or ("", false) otherwise.
func drupalModuleName(pkg string) (name string, ok bool) {
	if !strings.HasPrefix(pkg, "drupal/") {
		return "", false
	}
	return pkg[len("drupal/"):], true
}

// isSkippablePackage returns true for packages that should not be offered
// for version updates. This includes invalid package names, PHP itself, PHP extensions, and Drupal
// core infrastructure packages.
func isSkippablePackage(pkg string) bool {
	if checkPackageName(pkg) != nil {
		return true
	}
	if pkg == "php" || pkg == "composer" || strings.HasPrefix(pkg, "ext-") || strings.HasPrefix(pkg, "lib-") {
		return true
	}
	if name, ok := drupalModuleName(pkg); ok && isCorePackage(name) {
		return true
	}
	return false
}

// isCorePackage returns true if the drupal module name is a drupal core package.
// name is expected to be a drupal-scoped module name.
func isCorePackage(name string) bool {
	return name == "core" || strings.HasPrefix(name, "core-")
}
