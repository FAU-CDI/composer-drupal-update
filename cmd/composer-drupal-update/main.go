package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	drupalupdate "github.com/FAU-CDI/composer-drupal-update"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: composer-drupal-update <path-to-composer.json>")
		os.Exit(1)
	}

	filePath := os.Args[1]

	composer, err := drupalupdate.ReadComposerJSON(filePath)
	if err != nil {
		fmt.Printf("Error reading composer.json: %v\n", err)
		os.Exit(1)
	}

	client := drupalupdate.NewClient()
	reader := bufio.NewReader(os.Stdin)

	changed := false
	for packageName, currentVersion := range composer.Require {
		moduleName, ok := drupalupdate.DrupalModuleName(packageName)
		if !ok || drupalupdate.IsCorePackage(moduleName) {
			continue
		}

		releases, err := client.FetchReleases(moduleName)
		if err != nil {
			fmt.Printf("  [%s] Could not fetch releases: %v\n", packageName, err)
			continue
		}
		if len(releases) == 0 {
			fmt.Printf("  [%s] No releases found\n", packageName)
			continue
		}

		newVersion := selectVersion(reader, packageName, currentVersion, releases)
		if newVersion != "" && newVersion != currentVersion {
			composer.Require[packageName] = newVersion
			changed = true
		}
	}

	if changed {
		if err := drupalupdate.WriteComposerJSON(filePath, composer); err != nil {
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
		if coreCompat == "" {
			coreCompat = "unknown"
		}
		fmt.Printf("  [%d] %-20s (core: %s)\n", i+1, r.Version, coreCompat)
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
				newVersion := "^" + releases[choice-1].Version
				fmt.Printf("  -> Updated to %s\n", newVersion)
				return newVersion
			}
		}

		fmt.Println("  Invalid choice. Try again.")
	}
}
