# GoClaudeContext

A simple utility tool to prepare Golang project context for Claude AI conversations.

## Overview

GoClaudeContext is a straightforward command-line tool that creates organized context from your Golang projects for more effective AI-assisted development with Claude. It extracts documentation, collects important files, and prepares them in a format that provides Claude with comprehensive understanding of your codebase.

## Features

- Creates a dedicated "sync directory" containing all context files
- Automatically extracts package documentation using `go doc -all`
- Includes all README.md files from your project
- Selectively includes full source code for specified packages
- Uses symlinks to maintain references to original files

## Installation

```bash
go install github.com/ruteri/gocontext@latest
```

## Quick Start

```bash
# Basic usage
gocontext -project /path/to/your/golang/project -output ./claude-context

# Include source code for specific packages
gocontext -project /path/to/your/golang/project -output ./claude-context -include="github.com/yourusername/project/pkg1,github.com/yourusername/project/pkg2"

# Enable verbose output
gocontext -project /path/to/your/golang/project -verbose
```

## How It Works

1. **Initialization**: Creates a "sync directory" at your specified location
2. **Package Discovery**: Scans your project to identify all Go packages
3. **Documentation Extraction**: Runs `go doc -all` on each package and stores the output
4. **README Collection**: Symlinks all README.md files found in the project
5. **Source Code Collection**: For specified packages, symlinks all .go files

## Usage Options

```
Usage: gocontext [options]

Options:
  -project string
        Path to the Go project (required)
  -output string
        Path where the sync directory will be created (default "./claude-context")
  -include string
        Comma-separated list of packages to include source code
  -clean
        Remove existing sync directory before creating a new one
  -verbose
        Enable verbose logging
```

## Directory Structure

After running the tool, your sync directory will have a flat structure with prefixed filenames:

```
claude-context/
├── doc_github.com_yourusername_project_pkg1.txt
├── doc_github.com_yourusername_project_pkg2.txt
├── readme_README.md
├── readme_cmd_tool_README.md
├── src_github.com_yourusername_project_pkg1_file1.go
├── src_github.com_yourusername_project_pkg1_file2.go
└── ... (all files with appropriate prefixes)
```

## Example Workflow

1. Generate context for your project:
   ```bash
   gocontext -project ./myproject -output ./context -include="github.com/me/myproject/models"
   ```

2. Upload the contents of the context directory to Claude:
   - Select files from the context directory when prompted by Claude
   - Or zip the directory and upload it as a single file

3. Ask Claude questions about your project with full context:
   ```
   I've provided my Go project context. Can you help me understand how the authentication flow works?
   ```

## License

This project is licensed under the MIT License.
