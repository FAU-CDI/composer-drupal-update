//spellchecker:words main
package main

//spellchecker:words bufio context encoding json errors path filepath strings github composer drupal update drupalupdate
import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: composer-drupal-update <path-to-composer.json>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	composer, err := readComposerJSON(filePath)
	if err != nil {
		fmt.Printf("Error reading composer.json: %v\n", err)
		os.Exit(1)
	}

	client := drupalupdate.NewClient()
	reader := bufio.NewReader(os.Stdin)
	changed := false
	ctx := context.Background()

	// Process Drupal Core
	corePkgs := composer.CorePackages()
	if len(corePkgs) > 0 {
		fmt.Println("\n=== Drupal Core ===")
		fmt.Print("  Packages: ")
		for i, pkg := range corePkgs {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(pkg.Name)
		}
		fmt.Println()

		releases, err := client.FetchReleases(ctx, "drupal")
		switch {
		case err != nil:
			fmt.Printf("  Could not fetch core releases: %v\n", err)
		case len(releases) > 0:
			newVersion := selectVersion(reader, "Drupal Core", corePkgs[0].Version, releases)
			if newVersion != "" && newVersion != corePkgs[0].Version {
				for _, pkg := range corePkgs {
					composer.Require[pkg.Name] = newVersion
				}
				changed = true
			}
		default:
			fmt.Println("  No releases found")
		}
	}

	// Process Drupal packages
	drupalPkgs := composer.DrupalPackages()
	if len(drupalPkgs) > 0 {
		fmt.Println("\n=== Drupal Packages ===")
		for _, pkg := range drupalPkgs {
			releases, err := client.FetchReleases(ctx, pkg.Module)
			if err != nil {
				fmt.Printf("  [%s] Could not fetch releases: %v\n", pkg.Name, err)
				continue
			}
			if len(releases) == 0 {
				fmt.Printf("  [%s] No releases found\n", pkg.Name)
				continue
			}

			newVersion := selectVersion(reader, pkg.Name, pkg.Version, releases)
			if newVersion != "" && newVersion != pkg.Version {
				composer.Require[pkg.Name] = newVersion
				changed = true
			}
		}
	}

	// Process Composer (non-Drupal) packages
	composerPkgs := composer.ComposerPackages()
	if len(composerPkgs) > 0 {
		fmt.Println("\n=== Composer Packages ===")
		for _, pkg := range composerPkgs {
			releases, err := client.FetchPackagistReleases(ctx, pkg.Name)
			if err != nil {
				fmt.Printf("  [%s] Could not fetch releases: %v\n", pkg.Name, err)
				continue
			}
			if len(releases) == 0 {
				fmt.Printf("  [%s] No releases found\n", pkg.Name)
				continue
			}

			newVersion := selectVersion(reader, pkg.Name, pkg.Version, releases)
			if newVersion != "" && newVersion != pkg.Version {
				composer.Require[pkg.Name] = newVersion
				changed = true
			}
		}
	}

	if changed {
		if err := writeComposerJSON(filePath, composer); err != nil {
			fmt.Printf("Error writing composer.json: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\ncomposer.json updated successfully!")
	} else {
		fmt.Println("\nNo changes made.")
	}
}

func selectVersion(reader *bufio.Reader, packageName, currentVersion string, releases []drupalupdate.Release) string {
	fmt.Printf("\n%s (current: %s)\n", packageName, currentVersion)
	fmt.Println(strings.Repeat("-", 60))

	for i, r := range releases {
		coreCompat := r.CoreCompatibility
		if coreCompat != "" {
			fmt.Printf("  [%d] %-12s (%s, core: %s)\n", i+1, r.VersionPin, r.Version, coreCompat)
		} else {
			fmt.Printf("  [%d] %-12s (%s)\n", i+1, r.VersionPin, r.Version)
		}
	}
	fmt.Println("  [s] Skip (keep current version)")
	fmt.Println()

	for {
		fmt.Print("Select version: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "s" || input == "S" || input == "" {
			fmt.Println("  -> Keeping current version")
			return ""
		}

		var choice int
		if _, err := fmt.Sscanf(input, "%d", &choice); err == nil {
			if choice >= 1 && choice <= len(releases) {
				newVersion := releases[choice-1].VersionPin
				fmt.Printf("  -> Updated to %s\n", newVersion)
				return newVersion
			}
		}

		fmt.Println("  Invalid choice. Try again.")
	}
}

var errInvalidPath = errors.New("invalid path")

// readComposerJSON reads a composer.json file from the given path.
func readComposerJSON(path string) (c *drupalupdate.ComposerJSON, e error) {
	path = filepath.Clean(path)
	if path == "" || path == "." || strings.Contains(path, "..") {
		return nil, fmt.Errorf("%w: %s", errInvalidPath, path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			e = errors.Join(e, err)
		}
	}()

	var composer drupalupdate.ComposerJSON
	err = json.NewDecoder(file).Decode(&composer)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return &composer, nil
}

// writeComposerJSON writes a composer.json file to the given path.
func writeComposerJSON(path string, composer *drupalupdate.ComposerJSON) (e error) {
	path = filepath.Clean(path)
	if path == "" || path == "." || strings.Contains(path, "..") {
		return fmt.Errorf("%w: %s", errInvalidPath, path)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			e = errors.Join(e, err)
		}
	}()

	err = json.NewEncoder(file).Encode(composer)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
