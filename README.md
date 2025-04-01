# GoContext

A utility tool to prepare Golang project context for AI conversations.

## Overview

GoContext is a command-line tool that creates organized context from your Golang projects for more effective AI-assisted development. It extracts documentation, collects important files, and prepares them in a format that provides models with comprehensive understanding of your codebase.

## Features

- Creates a dedicated "sync directory" containing all context files
- Uses `go list ./...` to discover packages in your project
- Extracts concise package documentation using `go doc -short -all`
- Intelligently skips documentation generation when files haven't changed
- Includes all README.md files from your project
- Smart inclusion/exclusion with automatic detection of directories vs. packages
- Respects Git's `.gitignore` patterns when running in a Git repository
- Uses symlinks to maintain references to original files
- Uses a flat structure with prefixed filenames for easy upload
- By default, stores context in `~/.gocontext/<module-name>` for easy reuse

## Installation

```bash
go install github.com/ruteri/gocontext@latest
```

## Quick Start

```bash
# Basic usage - run from your Go project directory
cd /path/to/your/golang/project
gocontext

# Include source code from specific directories or packages
gocontext -include="cmd,internal/auth,github.com/yourusername/project/pkg/models"

# Exclude directories or packages
gocontext -exclude="test,examples,github.com/yourusername/project/internal/testdata"

# Specify a custom output directory
gocontext -output="./my-context-dir"

# Enable verbose output
gocontext -verbose

# Clean existing sync directory before creating a new one
gocontext -clean
```

## Directory Structure

After running the tool, your sync directory will have a flat structure with prefixed filenames:

```
~/.gocontext/github_com_yourusername_project/
├── doc_github.com_yourusername_project_cmd_app.txt
├── doc_github.com_yourusername_project_pkg_models.txt
├── readme_README.md
├── readme_cmd_app_README.md
├── src_cmd_app_main.go
├── src_cmd_app_config.go
├── src_pkg_models_user.go
└── ... (all files with appropriate prefixes)
```

## Usage Options

```
Usage: gocontext [options]

Options:
  -project string
        Path to the Go project (default: current directory)
  -output string
        Path where the sync directory will be created (default: ~/.gocontext/<module-name>)
  -include string
        Comma-separated list of directories or packages to include source code from
  -exclude string
        Comma-separated list of directories or packages to exclude
  -clean
        Remove existing sync directory before creating a new one
  -verbose
        Enable verbose logging
```

The tool uses several mechanisms to determine what files to include:

1. **Package discovery**: Uses `go list ./...` to find all packages in the project
2. **Smart filtering**: Automatically detects if an item is a package or directory based on its format
3. **Path-based exclusion**: Excludes any packages that match the excluded paths
4. **Git integration**: Respects `.gitignore` patterns in Git repositories

## Intelligent Documentation Generation

The tool intelligently determines when documentation needs to be regenerated:

- Always generates documentation if it doesn't exist yet
- In Git repositories, checks for uncommitted changes
- Compares the documentation file timestamp with the latest Git commit timestamp
- Only runs `go doc` when necessary, saving time for large projects

## File Types

The tool automatically includes files with the following extensions:
- `.go` - Go source files
- `.proto` - Protocol Buffer definitions
- `.tmpl` - Template files
- `.txt` - Text files

## Example Workflow

1. Generate context for your project:
   ```bash
   # From your project directory
   cd ~/projects/mygoproject
   
   # Include specific packages and directories
   gocontext -include="cmd,internal/auth,github.com/me/myproject/models"
   ```

2. Upload the contents of the context directory:
   - Select files from the ~/.gocontext/myproject directory when prompted
   - Or zip the directory and upload it as a single file

3. Ask questions about your project with full context:
   ```
   I've provided my Go project context. Can you help me understand how the authentication flow works?
   ```

## License

This project is licensed under the MIT License.
