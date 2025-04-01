package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	// Parse command line arguments
	projectPath := flag.String("project", "", "Path to the Go project (default: current directory)")
	outputPath := flag.String("output", "", "Path for the sync directory (default: ~/.gocontext/<module-name>)")
	includeDirs := flag.String("include-dirs", "", "Comma-separated list of directories to include source code from")
	excludeDirs := flag.String("exclude-dirs", "", "Comma-separated list of directories to exclude entirely")
	includePackages := flag.String("include-pkgs", "", "Comma-separated list of Go packages to include source code from")
	excludePackages := flag.String("exclude-pkgs", "", "Comma-separated list of Go packages to exclude")
	cleanFlag := flag.Bool("clean", false, "Remove existing sync directory before creating a new one")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Use current directory if project path not specified
	if *projectPath == "" {
		currentDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error getting current directory: %v\n", err)
			os.Exit(1)
		}
		*projectPath = currentDir

		if *verboseFlag {
			fmt.Printf("No project path specified, using current directory: %s\n", *projectPath)
		}
	}

	// Convert to absolute path
	absProjectPath, err := filepath.Abs(*projectPath)
	if err != nil {
		fmt.Printf("Error resolving project path: %v\n", err)
		os.Exit(1)
	}

	// Verify the directory is a Go project
	if !isGoProject(absProjectPath) {
		fmt.Printf("Error: %s does not appear to be a Go project\n", absProjectPath)
		fmt.Println("Make sure you're running this from a Go project directory or specify a valid project path with -project flag")
		os.Exit(1)
	}

	// Get module name for default output path
	moduleName, err := getModuleName(absProjectPath)
	if err != nil && *verboseFlag {
		fmt.Printf("Warning: Couldn't determine module name: %v\n", err)
	}

	// If no output path specified, use ~/.gocontext/<module-name>
	if *outputPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Error getting home directory: %v\n", err)
			os.Exit(1)
		}

		// Create a safe directory name from the module
		dirName := "default"
		if moduleName != "" {
			dirName = strings.Replace(moduleName, "/", "_", -1)
			dirName = strings.Replace(dirName, ".", "_", -1)
		} else {
			dirName = filepath.Base(absProjectPath)
		}

		*outputPath = filepath.Join(homeDir, ".gocontext", dirName)

		if *verboseFlag {
			fmt.Printf("No output path specified, using: %s\n", *outputPath)
		}
	}

	// Convert output path to absolute
	absOutputPath, err := filepath.Abs(*outputPath)
	if err != nil {
		fmt.Printf("Error resolving output path: %v\n", err)
		os.Exit(1)
	}

	// Parse filter lists
	includeDirsList := splitAndTrim(*includeDirs, ",")
	excludeDirsList := splitAndTrim(*excludeDirs, ",")
	includePkgsList := splitAndTrim(*includePackages, ",")
	excludePkgsList := splitAndTrim(*excludePackages, ",")

	// Check if the project is a git repository
	isGitRepo := isGitRepository(absProjectPath)
	if *verboseFlag && isGitRepo {
		fmt.Println("Git repository detected, will respect .gitignore patterns")
	}

	// Create sync directory
	if err := createSyncDirectory(absOutputPath, *cleanFlag); err != nil {
		fmt.Printf("Error creating sync directory: %v\n", err)
		os.Exit(1)
	}

	if *verboseFlag {
		fmt.Printf("Created sync directory at: %s\n", absOutputPath)
	}

	// Discover and filter Go packages
	allPackages, err := discoverPackages(absProjectPath)
	if err != nil {
		fmt.Printf("Error discovering packages: %v\n", err)
		os.Exit(1)
	}

	for _, dir := range excludeDirsList {
		excludePkgsList = append(excludePkgsList, path.Join(moduleName, dir))
	}

	packages := filterPackages(allPackages, includePkgsList, excludePkgsList)

	if *verboseFlag {
		fmt.Printf("Discovered %d packages, using %d after filtering\n", len(allPackages), len(packages))
	}

	// Extract documentation for each package
	for _, pkg := range packages {
		if err := extractDocumentation(pkg, absOutputPath, absProjectPath, *verboseFlag); err != nil && *verboseFlag {
			fmt.Printf("Warning: Error extracting documentation for %s: %v\n", pkg, err)
		}
	}

	// Find and symlink README.md files
	if err := findAndSymlinkReadmes(absProjectPath, absOutputPath, excludeDirsList, isGitRepo, *verboseFlag); err != nil {
		fmt.Printf("Error symlinking README files: %v\n", err)
		os.Exit(1)
	}

	// Process directories and packages for source files
	processedDirs := make(map[string]bool)

	// Convert relative directory paths to absolute
	for i, dir := range includeDirsList {
		if !filepath.IsAbs(dir) {
			includeDirsList[i] = filepath.Join(absProjectPath, dir)
		}
	}

	// Process included directories
	for _, dir := range includeDirsList {
		if _, processed := processedDirs[dir]; !processed {
			if err := symlinkDirectoryFiles(dir, absProjectPath, absOutputPath, isGitRepo, *verboseFlag); err != nil && *verboseFlag {
				fmt.Printf("Warning: Error symlinking files from directory %s: %v\n", dir, err)
			}
			processedDirs[dir] = true
		}
	}

	// Process included packages
	for _, pkg := range includePkgsList {
		pkgDir, err := getPackageDir(pkg, absProjectPath)
		if err != nil {
			if *verboseFlag {
				fmt.Printf("Warning: Error finding directory for package %s: %v\n", pkg, err)
			}
			continue
		}

		if _, processed := processedDirs[pkgDir]; !processed {
			if err := symlinkDirectoryFiles(pkgDir, absProjectPath, absOutputPath, isGitRepo, *verboseFlag); err != nil && *verboseFlag {
				fmt.Printf("Warning: Error symlinking files from package %s: %v\n", pkg, err)
			}
			processedDirs[pkgDir] = true
		}
	}

	fmt.Printf("Context synced successfully to: %s\n", absOutputPath)
}

// splitAndTrim splits a comma-separated string and trims each element
func splitAndTrim(s string, sep string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// isGoProject checks if a directory is a Go project
func isGoProject(path string) bool {
	// Try running 'go list' in the directory
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}}", ".")
	cmd.Dir = path
	cmd.Stderr = nil // Suppress stderr output

	// If the command succeeds, it's a Go project
	if output, err := cmd.Output(); err == nil && len(output) > 0 {
		return true
	}

	// If 'go list' fails, check for go.mod file as fallback
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		return true
	}

	return false
}

// getModuleName extracts the Go module name from go.mod
func getModuleName(projectPath string) (string, error) {
	goModPath := filepath.Join(projectPath, "go.mod")

	// Check if go.mod exists
	if _, err := os.Stat(goModPath); err != nil {
		return "", err
	}

	// Read go.mod file
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}

	// Parse to find the module name
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	return "", fmt.Errorf("module declaration not found in go.mod")
}

// isGitRepository checks if a directory is a git repository
func isGitRepository(path string) bool {
	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		return true
	}

	// Try running git command to be sure
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(out)) == "true"
}

// isIgnoredByGit checks if a file is ignored by git
func isIgnoredByGit(path string, projectPath string) (bool, error) {
	// Get relative path from project root
	relPath, err := filepath.Rel(projectPath, path)
	if err != nil {
		return false, err
	}

	// Use git check-ignore to see if the file is ignored
	cmd := exec.Command("git", "check-ignore", "-q", relPath)
	cmd.Dir = projectPath

	// If exit code is 0, the file is ignored
	// If exit code is 1, the file is not ignored
	// Any other exit code indicates an error
	err = cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				// Exit code 1 means the file is not ignored
				return false, nil
			}
		}
		return false, err
	}

	// Exit code 0 means the file is ignored
	return true, nil
}

// createSyncDirectory creates the output directory
func createSyncDirectory(path string, clean bool) error {
	if clean {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	return os.MkdirAll(path, 0755)
}

// discoverPackages finds all Go packages in the project
func discoverPackages(projectPath string) ([]string, error) {
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run 'go list ./...': %v", err)
	}

	return splitAndTrim(string(output), "\n"), nil
}

// filterPackages filters a list of packages based on inclusion/exclusion lists
func filterPackages(packages, includeList, excludeList []string) []string {
	// If no includes or excludes specified, return all packages
	if len(includeList) == 0 && len(excludeList) == 0 {
		return packages
	}

	var filtered []string

	for _, pkg := range packages {
		excluded := false
		for _, excl := range excludeList {
			if strings.HasPrefix(pkg, excl) {
				excluded = true
			}
		}
		if !excluded {
			filtered = append(filtered, pkg)
		}
	}

	return filtered
}

// getPackageDir gets the directory for a Go package
func getPackageDir(pkg string, projectPath string) (string, error) {
	// Run go list to get the package directory
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", pkg)
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// extractDocumentation runs go doc -all for a package and saves the output
func extractDocumentation(pkg, outputPath string, projectPath string, verbose bool) error {
	// Run go doc -all with the appropriate package path
	cmd := exec.Command("go", "doc", "-short", "-all", pkg)
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	if len(output) == 0 {
		return errors.New("doc is empty")
	}

	// Create filename with doc_ prefix - use the full package path for uniqueness
	docFile := filepath.Join(outputPath, "doc_"+strings.Replace(pkg, "/", "_", -1)+".txt")

	// Write output to file
	if err := os.WriteFile(docFile, output, 0644); err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Extracted documentation for %s\n", pkg)
	}

	return nil
}

// findAndSymlinkReadmes finds all README.md files and symlinks them
func findAndSymlinkReadmes(projectPath, syncPath string, excludeDirs []string, isGitRepo bool, verbose bool) error {
	// Walk through project directory
	count := 0
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the directory should be excluded based on explicit excludes
		if info.IsDir() {
			for _, excludeDir := range excludeDirs {
				excludePath := excludeDir
				if !filepath.IsAbs(excludePath) {
					excludePath = filepath.Join(projectPath, excludeDir)
				}
				if path == excludePath || strings.HasPrefix(path, excludePath+string(os.PathSeparator)) {
					if verbose {
						fmt.Printf("Skipping excluded directory: %s\n", path)
					}
					return filepath.SkipDir
				}
			}
		}

		// Check if the file/directory is ignored by git
		if isGitRepo {
			ignored, err := isIgnoredByGit(path, projectPath)
			if err != nil {
				// If there's an error checking git ignore status, just continue
				if verbose {
					fmt.Printf("Warning: Error checking git ignore status for %s: %v\n", path, err)
				}
			} else if ignored {
				if info.IsDir() {
					if verbose {
						fmt.Printf("Skipping git-ignored directory: %s\n", path)
					}
					return filepath.SkipDir
				}
				if verbose {
					fmt.Printf("Skipping git-ignored file: %s\n", path)
				}
				return nil
			}
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
			if err := os.Symlink(path, symlinkPath); err != nil {
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

// symlinkDirectoryFiles symlinks all .go files from a directory
func symlinkDirectoryFiles(dirPath, projectPath, syncPath string, isGitRepo bool, verbose bool) error {
	// Make sure the directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dirPath)
	}

	// Get relative path for directory naming
	relDir, err := filepath.Rel(projectPath, dirPath)
	if err != nil {
		// If we can't get a relative path, use the directory name
		relDir = filepath.Base(dirPath)
	}

	dirPrefix := "src_" + strings.Replace(relDir, string(os.PathSeparator), "_", -1) + "_"

	// File extensions to include
	extensions := map[string]bool{
		".go":    true,
		".proto": true,
		".tmpl":  true,
		".txt":   true,
	}

	// Walk through the directory and symlink files
	count := 0
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip subdirectories
		if info.IsDir() && path != dirPath {
			return nil
		}

		// Check if the file is ignored by git
		if isGitRepo && !info.IsDir() {
			ignored, err := isIgnoredByGit(path, projectPath)
			if err != nil {
				// If there's an error checking git ignore status, just continue
				if verbose {
					fmt.Printf("Warning: Error checking git ignore status for %s: %v\n", path, err)
				}
			} else if ignored {
				if verbose {
					fmt.Printf("Skipping git-ignored file: %s\n", path)
				}
				return nil
			}
		}

		// Check if it's a source file with an allowed extension
		if !info.IsDir() {
			ext := filepath.Ext(info.Name())
			if extensions[ext] {
				filename := info.Name()
				symlinkPath := filepath.Join(syncPath, dirPrefix+filename)

				// Create symlink
				if err := os.Symlink(path, symlinkPath); err != nil {
					return err
				}

				count++
				if verbose {
					fmt.Printf("Symlinked file: %s\n", path)
				}
			}
		}

		return nil
	})

	if verbose {
		fmt.Printf("Symlinked %d files from directory %s\n", count, dirPath)
	}

	return err
}
