package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	// Parse command line arguments
	projectPath := flag.String("project", "", "Path to the Go project (default: current directory)")
	outputPath := flag.String("output", "", "Path for the sync directory (default: ~/.gocontext/<module-name>)")
	includeFlag := flag.String("include", "", "Comma-separated list of directories or packages to include source code from")
	excludeFlag := flag.String("exclude", "", "Comma-separated list of directories or packages to exclude")
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
	includeList := splitAndTrim(*includeFlag, ",")
	excludeList := splitAndTrim(*excludeFlag, ",")

	// Categorize includes and excludes based on whether they are packages or directories
	includeDirsList, includePkgsList := categorizeIncludesExcludes(includeList, moduleName)
	excludeDirsList, excludePkgsList := categorizeIncludesExcludes(excludeList, moduleName)

	if *verboseFlag {
		fmt.Printf("Include directories: %v\n", includeDirsList)
		fmt.Printf("Include packages: %v\n", includePkgsList)
		fmt.Printf("Exclude directories: %v\n", excludeDirsList)
		fmt.Printf("Exclude packages: %v\n", excludePkgsList)
	}

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

	// Directory exclusions are already handled by categorizeIncludesExcludes

	packages := filterPackages(allPackages, excludeDirsList, excludePkgsList, moduleName)

	if *verboseFlag {
		fmt.Printf("Discovered %d packages, using %d after filtering\n", len(allPackages), len(packages))
	}

	// Extract documentation for each package
	for _, pkg := range packages {
		if err := extractDocumentation(moduleName, pkg, absOutputPath, absProjectPath, isGitRepo, *verboseFlag); err != nil && *verboseFlag {
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

	// Process included directories
	for _, dir := range includeDirsList {
		includePkgsList = append(includePkgsList, path.Join(moduleName, dir))
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

	if err := generateDirectoryStructure(absProjectPath, absOutputPath, excludeDirsList, isGitRepo, *verboseFlag); err != nil {
		fmt.Printf("Error generating directory structure: %v\n", err)
		os.Exit(1)
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

// categorizeIncludesExcludes separates paths into directories and packages based on module name
func categorizeIncludesExcludes(items []string, moduleName string) (dirs []string, pkgs []string) {
	for _, item := range items {
		// If the item starts with the module name, it's a package
		if strings.HasPrefix(item, moduleName+"/") || item == moduleName {
			pkgs = append(pkgs, item)
		} else {
			// Otherwise it's a directory
			dirs = append(dirs, item)
		}
	}
	return dirs, pkgs
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
func filterPackages(packages, excludeDirs, excludePkgs []string, moduleName string) []string {
	// If no includes or excludes specified, return all packages
	if len(excludeDirs) == 0 && len(excludePkgs) == 0 {
		return packages
	}

	for _, excl := range excludeDirs {
		excludePkgs = append(excludePkgs, path.Join(moduleName, excl))
	}

	var filtered []string

	for _, pkg := range packages {
		excluded := false
		for _, excl := range excludePkgs {
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

var pkgCache map[string]string = make(map[string]string)

// getPackageDir gets the directory for a Go package
func getPackageDir(pkg string, projectPath string) (string, error) {
	if cachedPath, ok := pkgCache[pkg]; ok {
		return cachedPath, nil
	}
	// Run go list to get the package directory
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", pkg)
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	pkgPath := strings.TrimSpace(string(output))
	pkgCache[pkg] = pkgPath
	return pkgPath, nil
}

// hasDocFile checks if a package directory contains a doc.go file
func hasDocFile(pkg string, projectPath string) (bool, error) {
	// Get the package directory
	pkgDir, err := getPackageDir(pkg, projectPath)
	if err != nil {
		return false, err
	}

	// Check if doc.go exists in the package directory
	docFilePath := filepath.Join(pkgDir, "doc.go")
	_, err = os.Stat(docFilePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// needsDocUpdate checks if the documentation for a package needs to be updated
func needsDocUpdate(pkg, outputPath, projectPath string, isGitRepo bool) (bool, error) {
	// First, check if doc.go exists in the package directory
	hasDoc, err := hasDocFile(pkg, projectPath)
	if err != nil {
		return false, err
	}

	// Skip documentation generation if doc.go doesn't exist
	if !hasDoc {
		return false, nil
	}

	// Check if the documentation file already exists
	docFile := filepath.Join(outputPath, "doc_"+strings.Replace(pkg, "/", "_", -1)+".txt")
	docFileInfo, err := os.Stat(docFile)
	if os.IsNotExist(err) {
		// Doc file doesn't exist, so it needs to be created
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// If not a git repository, always update
	if !isGitRepo {
		return true, nil
	}

	// Get the package directory
	pkgDir, err := getPackageDir(pkg, projectPath)
	if err != nil {
		return false, err
	}

	// Check for uncommitted changes
	cmd := exec.Command("git", "status", "--porcelain", pkgDir)
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		// There are uncommitted changes
		return true, nil
	}

	// Get the last modified time of the package in git
	cmd = exec.Command("git", "log", "-1", "--format=%at", "--", pkgDir)
	cmd.Dir = projectPath
	output, err = cmd.Output()
	if err != nil || len(output) == 0 {
		// If there's an error or no output, fall back to always updating
		return true, nil
	}

	// Parse the timestamp
	timestamp, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return true, nil
	}

	// Convert the timestamp to time.Time
	lastModifiedTime := time.Unix(timestamp, 0)

	// Compare the timestamp of the doc file with the timestamp of the latest commit
	return docFileInfo.ModTime().Before(lastModifiedTime), nil
}

// extractDocumentation runs go doc -all for a package and saves the output if needed
func extractDocumentation(moduleName, pkg, outputPath string, projectPath string, isGitRepo bool, verbose bool) error {
	// Check if documentation needs to be updated
	needsUpdate, err := needsDocUpdate(pkg, outputPath, projectPath, isGitRepo)
	if err != nil {
		return err
	}

	if !needsUpdate {
		// Check if it's because doc.go doesn't exist
		hasDoc, err := hasDocFile(pkg, projectPath)
		if err == nil && !hasDoc && verbose {
			fmt.Printf("Skipping documentation for %s: no doc.go file found\n", pkg)
		} else if verbose {
			fmt.Printf("Documentation for %s is up-to-date, skipping\n", pkg)
		}
		return nil
	}

	// Run go doc -all with the appropriate package path
	cmd := exec.Command("go", "doc", "-short", "-all", "./"+pkg[len(moduleName)+1:])
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	if len(output) <= 1 {
		return errors.New("doc is empty")
	}

	// Create filename with doc_ prefix - use the relative package path for uniqueness
	docFile := filepath.Join(outputPath, "doc_"+strings.Replace(strings.TrimPrefix(pkg, moduleName+"/"), "/", "_", -1)+".txt")

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

			// Ignore existing symlinks
			if _, err := os.Lstat(symlinkPath); err == nil {
				if verbose {
					fmt.Printf("Ignoring already symlinked README: %s\n", relPath)
				}
				return nil
			}

			// Create symlink
			if err := os.Symlink(path, symlinkPath); err != nil {
				return err
			}

			if verbose {
				fmt.Printf("Symlinked README: %s\n", relPath)
			}
		}

		return nil
	})

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

				// Remove any existing symlink regardless of -clean flag
				if _, err := os.Lstat(symlinkPath); err == nil {
					if verbose {
						fmt.Printf("Ignoring already symlinked file: %s\n", path)
					}
				}

				// Create symlink
				if err := os.Symlink(path, symlinkPath); err != nil {
					return err
				}

				if verbose {
					fmt.Printf("Symlinked file: %s\n", path)
				}
			}
		}

		return nil
	})

	if verbose {
		fmt.Printf("Symlinked from directory %s\n", dirPath)
	}

	return err
}

// generateDirectoryStructure creates a text file with the project's directory structure using tree command
func generateDirectoryStructure(projectPath, outputPath string, excludeDirs []string, isGitRepo, verbose bool) error {
	structureFile := filepath.Join(outputPath, "directory_structure.txt")

	if verbose {
		fmt.Println("Generating directory structure...")
	}

	// Check if tree command is available
	treeCmd := exec.Command("tree", "--version")
	err := treeCmd.Run()
	if err != nil {
		return fmt.Errorf("tree command not found. Please install tree utility to use this feature: %v", err)
	}

	// Prepare exclude patterns for tree command
	excludePatterns := []string{}

	// Add exclude directory patterns
	for _, excludeDir := range excludeDirs {
		excludePatterns = append(excludePatterns, "-I", excludeDir)
	}

	// Add output directory to exclude patterns if it's within the project
	relOutputPath, err := filepath.Rel(projectPath, outputPath)
	if err == nil && !strings.HasPrefix(relOutputPath, "..") {
		excludePatterns = append(excludePatterns, "-I", filepath.Base(outputPath))
	}

	// Add gitignore patterns if in a git repo
	treeOptions := []string{"--dirsfirst", "--noreport", "-o", structureFile}
	if isGitRepo {
		treeOptions = append(treeOptions, "--gitignore")
	}

	// Create command with all options
	args := append(treeOptions, excludePatterns...)
	cmd := exec.Command("tree", args...)
	cmd.Dir = projectPath

	// Execute command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running tree command: %v", err)
	}

	if verbose {
		fmt.Println("Generated directory structure")
	}

	return nil
}
