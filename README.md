# GoContext

A utility tool to prepare Golang project context for AI conversations.

## Overview

GoContext is a command-line tool that creates organized context from your Golang projects for more effective AI-assisted development. It extracts documentation, collects important files, and prepares them in a format that provides models with comprehensive understanding of your codebase.

## Features

- Creates a dedicated "sync directory" containing all context files
- Uses `go list ./...` to discover packages in your project
- Extracts concise package documentation using `go doc -short -all`
- Includes all README.md files from your project
- Flexible filtering - include/exclude both directories and packages
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

# Include source code from specific directories
gocontext -include-dirs="cmd,internal/auth,pkg/models"

# Include source code from specific packages
gocontext -include-pkgs="github.com/yourusername/project/cmd,github.com/yourusername/project/pkg/models"

# Mix of directory and package-based inclusion
gocontext -include-dirs="cmd" -include-pkgs="github.com/yourusername/project/pkg/models"

# Exclude packages from documentation
gocontext -exclude-pkgs="github.com/yourusername/project/internal/testdata"

# Exclude directories
gocontext -exclude-dirs="test,examples"

# Specify a custom output directory
gocontext -output="./my-context-dir"

# Enable verbose output
gocontext -verbose
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
Usage: goontext [options]

Options:
  -project string
        Path to the Go project (default: current directory)
  -output string
        Path where the sync directory will be created (default: ~/.gocontext/<module-name>)
  -include-dirs string
        Comma-separated list of directories to include source code from
  -exclude-dirs string
        Comma-separated list of directories to exclude entirely
  -include-pkgs string
        Comma-separated list of Go packages to include source code from
  -exclude-pkgs string
        Comma-separated list of Go packages to exclude
  -clean
        Remove existing sync directory before creating a new one
  -verbose
        Enable verbose logging
```

The tool uses several mechanisms to determine what files to include:

1. **Package discovery**: Uses `go list ./...` to find all packages in the project
2. **Package filtering**: Applies `-include-pkgs` and `-exclude-pkgs` to filter packages
3. **Directory filtering**: Automatically converts excluded directories to package exclusions
4. **Git integration**: Respects `.gitignore` patterns in Git repositories

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
   goontext -include-pkgs="github.com/me/myproject/models" -include-dirs="cmd,internal/auth"
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
