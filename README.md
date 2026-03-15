# SDLC

## Overview

**SDLC** is a lightweight, unified CLI tool designed to simplify your software development lifecycle. It abstracts away the complexity of different build tools and languages, providing a consistent interface for running, testing, and building your projects.

## Purpose

The purpose of this project is to provide developers with a single entry point to manage multi-language, multi-module repositories without needing to remember the specific commands for each technology. It aims to increase productivity, reduce context switching, and make onboarding new team members easier.

**SDLC** is a lightweight, unified CLI tool designed to simplify your software development lifecycle. It abstracts away the complexity of different build tools and languages, providing a consistent interface for running, testing, and building your projects.

Whether you're working on a Go backend, a Node.js frontend, or a multi-module monorepo, `sdlc` figures out what to do so you don't have to remember every specific command.

```text
   _____ ____  __    ______
  / ___// __ \/ /   / ____/
  \__ \/ / / / /   / /     
 ___/ / /_/ / /___/ /___   
/____/_____/_____/\____/   
```

## Features

- 🔍 **Auto-detection**: Automatically identifies project types by scanning for build files (`go.mod`, `package.json`, `pom.xml`, etc.).
- 🔧 **Unified Interface**: Use `sdlc run`, `sdlc test`, `sdlc build`, `sdlc install`, and `sdlc clean` for everything.
- 📦 **Multi-Module Support**: Seamlessly detects and manages multiple projects within a single repository (monorepos).
- ⚡ **Live Reload (Watch Mode)**: Automatically restarts your application or re-runs tests when files change (`--watch`).
- 🎨 **Color-Coded Output**: distinct colors for different modules in multi-module setups for easy log reading.
- ⚙️ **Flexible Configuration**:
    - Define custom project types via `.sdlc.json`.
    - Set environment variables and extra flags per module via `.sdlc.conf`.

## Installation

You can build and install `sdlc` from source using Go (1.20+):

```bash
git clone https://github.com/unitz007/sdlc.git
cd sdlc
go install .
```

Ensure your `$(go env GOPATH)/bin` is in your system `PATH`.

## Usage

Navigate to your project directory and run:

```bash
# Run the project (auto-detects single or multi-module)
sdlc run

# Run with watch mode enabled
sdlc run --watch

# Run a specific module in a monorepo
sdlc run --module backend

# Ignore specific modules in a monorepo
sdlc run --ignore frontend

# Test the project
sdlc test

# Build the project
sdlc build

# Install dependencies
sdlc install

# Clean build artifacts
sdlc clean
```

### Command Reference

| Command | Description |
|---------|-------------|
| `run`   | Runs the application (e.g., `go run`, `npm start`). |
| `test`  | Runs the test suite (e.g., `go test`, `npm test`). |
| `build` | Compiles the application (e.g., `go build`, `npm build`). |
| `install`| Installs dependencies (e.g., `go mod download`, `npm install`). |
| `clean` | Removes build artifacts (e.g., `go clean`, `rm -rf dist`). |

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--watch` | `-w` | Enable watch mode to restart on file changes. |
| `--all` | `-a` | Run command for all detected modules (default behavior if >1 module found). |
| `--module` | `-m` | Specify a single module/path to run (relative path). |
| `--ignore` | `-i` | Ignore specific modules in a multi-module project (supports multiple flags). |
| `--dir` | `-d` | Specify an absolute path to the project directory (default: current dir). |
| `--extra-args` | `-e` | Pass additional arguments to the underlying build tool. |
| `--config` | `-c` | Path to a custom configuration directory. |

## Configuration

### 1. Project Definitions (`.sdlc.json`)

SDLC looks for a `.sdlc.json` file in your home directory or project root to define how to handle different file types.

**Example `~/.sdlc.json`:**
```json
{
  "go.mod": {
    "run": "go run .",
    "test": "go test ./...",
    "build": "go build -o app",
    "install": "go mod download",
    "clean": "go clean"
  },
  "package.json": {
    "run": "npm start",
    "test": "npm test",
    "build": "npm run build",
    "install": "npm install",
    "clean": "rm -rf node_modules"
  }
}
```

### 2. Environment & Flags (`.sdlc.conf`)

You can place a `.sdlc.conf` file in your project root or any module subdirectory to inject environment variables or flags specific to that scope.

**Example `.sdlc.conf`:**
```properties
# Environment Variables
PORT=8080
DB_HOST=localhost

# Extra Flags (appended to the command)
--debug
--verbose
```

## Multi-Module Projects

If `sdlc` detects multiple projects (e.g., a `go.mod` in one folder and `package.json` in another), it will treat them as modules.

- **`sdlc run`**: Runs all detected modules concurrently.
- **`sdlc run -m <folder>`**: Runs only the specified module.
- **Output**: Logs from different modules are prefixed and color-coded for clarity.

## Watch Mode

Enable watch mode with `--watch` (or `-w`). SDLC will monitor all files in the project (respecting `.gitignore` and ignoring build artifacts like `node_modules`).

- **Single Module**: Restarts the process on change.
- **Multi-Module**: Restarts all modules to ensure consistency (smart partial restarts coming soon).

## Roadmap

- **Enhanced Watch Mode**: smarter partial restarts for individual modules.
- **Plugin System**: allow community-contributed project definitions and commands.
- **CI Integration**: built‑in support for common CI/CD pipelines (GitHub Actions, GitLab CI).
- **Documentation Site**: generate a static site from the README and examples.
- **Cross‑Platform Binaries**: pre‑compiled binaries for Windows, macOS, and Linux.
- **Extensive Test Coverage**: increase unit and integration test coverage.



Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

## Contributing

We welcome contributions from the community! To get started:

1. **Fork the repository** and clone your fork locally.
2. **Create a new branch** for your feature or bug fix.
3. Ensure the code builds and all tests pass:
   ```bash
   go test ./...
   ```
4. Follow the existing code style and formatting (run `go fmt ./...`).
5. Write or update tests to cover your changes.
6. Commit your changes with clear, descriptive messages.
7. Push the branch to your fork and open a Pull Request against the `main` branch.

Please make sure your PR:
- Includes a concise description of the change.
- Passes all existing CI checks.
- Does not introduce new linting or formatting issues.

We appreciate your help in making **SDLC** better!

## License

## License

[Apache 2.0](http://www.apache.org/licenses/LICENSE-2.0)
