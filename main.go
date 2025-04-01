package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Parse command line arguments
	projectPath := flag.String("project", "", "Path to the Go project")
	outputPath := flag.String("output", "./claude-context", "Path for the sync directory")
	includePackages := flag.String("include", "", "Comma-separated packages to include source")
	cleanFlag := flag.Bool("clean", false, "Remove existing sync directory before creating a new one")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Check required flags
	if *projectPath == "" {
		fmt.Println("Error: -project flag is required")
		flag.Usage()
		os.Exit(1)
	}

	// Convert to absolute paths
	absProjectPath, err := filepath.Abs(*projectPath)
	if err != nil {
		fmt.Printf("Error resolving project path: %v\n", err)
		os.Exit(1)
	}

	absOutputPath, err := filepath.Abs(*outputPath)
	if err != nil {
		fmt.Printf("Error resolving output path: %v\n", err)
		os.Exit(1)
	}

	// Parse included packages
	var includePkgs []string
	if *includePackages != "" {
		includePkgs = strings.Split(*includePackages, ",")
	}

	// Create sync directory
	err = createSyncDirectory(absOutputPath, *cleanFlag)
	if err != nil {
		fmt.Printf("Error creating sync directory: %v\n", err)
		os.Exit(1)
	}

	if *verboseFlag {
		fmt.Printf("Created sync directory at: %s\n", absOutputPath)
	}

	// Discover Go packages
	packages, err := discoverPackages(absProjectPath)
	if err != nil {
		fmt.Printf("Error discovering packages: %v\n", err)
		os.Exit(1)
	}

	if *verboseFlag {
		fmt.Printf("Discovered %d packages\n", len(packages))
	}

	// Extract documentation for each package
	for _, pkg := range packages {
		err = extractDocumentation(pkg, absOutputPath, *verboseFlag)
		if err != nil && *verboseFlag {
			fmt.Printf("Warning: Error extracting documentation for %s: %v\n", pkg, err)
		}
	}

	// Find and symlink README.md files
	err = findAndSymlinkReadmes(absProjectPath, absOutputPath, *verboseFlag)
	if err != nil {
		fmt.Printf("Error symlinking README files: %v\n", err)
		os.Exit(1)
	}

	// Symlink package files for specified packages
	for _, pkg := range includePkgs {
		err = symlinkPackageFiles(pkg, absProjectPath, absOutputPath, *verboseFlag)
		if err != nil && *verboseFlag {
			fmt.Printf("Warning: Error symlinking files for %s: %v\n", pkg, err)
		}
	}

	fmt.Printf("Context synced successfully to: %s\n", absOutputPath)
}

// createSyncDirectory creates the output directory
func createSyncDirectory(path string, clean bool) error {
	if clean {
		err := os.RemoveAll(path)
		if err != nil {
			return err
		}
	}

	// Create directory if it doesn't exist
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	return nil
}

// discoverPackages finds all Go packages in the project
func discoverPackages(projectPath string) ([]string, error) {
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	packages := strings.Split(strings.TrimSpace(string(output)), "\n")
	return packages, nil
}

// extractDocumentation runs go doc -all for a package and saves the output
func extractDocumentation(pkg, outputPath string, verbose bool) error {
	// Run go doc -all
	cmd := exec.Command("go", "doc", "-all", pkg)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// Create filename with doc_ prefix
	docFile := filepath.Join(outputPath, "doc_"+strings.Replace(pkg, "/", "_", -1)+".txt")

	// Write output to file
	err = os.WriteFile(docFile, output, 0644)
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Extracted documentation for %s\n", pkg)
	}

	return nil
}

// findAndSymlinkReadmes finds all README.md files and symlinks them
func findAndSymlinkReadmes(projectPath, syncPath string, verbose bool) error {
	// Walk through project directory
	count := 0
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if it's a README.md file
		if !info.IsDir() && strings.ToLower(info.Name()) == "readme.md" {
			// Create a unique name for the symlink
			relPath, err := filepath.Rel(projectPath, path)
			if err != nil {
				return err
			}
			symlinkName := "readme_" + strings.Replace(relPath, "/", "_", -1)
			symlinkPath := filepath.Join(syncPath, symlinkName)

			// Create symlink
			err = os.Symlink(path, symlinkPath)
			if err != nil {
				return err
			}

			count++
			if verbose {
				fmt.Printf("Symlinked README: %s\n", relPath)
			}
		}

		return nil
	})

	if verbose {
		fmt.Printf("Symlinked %d README files\n", count)
	}

	return err
}

// symlinkPackageFiles symlinks all .go files for a specified package
func symlinkPackageFiles(pkg, projectPath, syncPath string, verbose bool) error {
	// Get package directory
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", pkg)
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	pkgDir := strings.TrimSpace(string(output))
	if pkgDir == "" {
		return fmt.Errorf("package directory not found for %s", pkg)
	}

	// Get package name for prefix
	pkgPrefix := "src_" + strings.Replace(pkg, "/", "_", -1) + "_"

	// Symlink all .go files
	count := 0
	err = filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process files in the package directory, not subdirectories
		if filepath.Dir(path) != pkgDir {
			return nil
		}

		// Check if it's a .go file
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			symlinkPath := filepath.Join(syncPath, pkgPrefix+info.Name())

			// Create symlink
			err = os.Symlink(path, symlinkPath)
			if err != nil {
				return err
			}

			count++
			if verbose {
				fmt.Printf("Symlinked file: %s\n", path)
			}
		}

		return nil
	})

	if verbose {
		fmt.Printf("Symlinked %d files for package %s\n", count, pkg)
	}

	return err
}
