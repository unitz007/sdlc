# SDLC

**SDLC** is a lightweight, unified CLI tool designed to simplify your software development lifecycle. It abstracts away the complexity of different build tools and languages, providing a consistent interface for running, testing, and building your projects.

Whether you're working on a Go backend, a Node.js frontend, or a multi-module monorepo, `sdlc` figures out what to do so you don't have to remember every specific command.

```text
   _____ ____  __    ______
  / ___// __ \/ /   / ____/
  \__ \/ / / / /   / /     
 ___/ / /_/ / /___/ /___   
/____/_____/_____/_____/   
```

## Features

- 🔍 **Auto-detection**: Automatically identifies project types by scanning for build files (`go.mod`, `package.json`, `pom.xml`, etc.).
- 🔧 **Unified Interface**: Use `sdlc run`, `sdlc test`, `sdlc build`, `sdlc install`, and `sdlc clean` for everything.
- 📦 **Multi-Module Support**: Seamlessly detects and manages multiple projects within a single repository (monorepos).
- ⚡ **Live Reload (Watch Mode)**: Automatically restarts the affected module when files change (`--watch`). Uses `fsnotify` for instant, event-based file watching with per-module smart restarts.
- 🎨 **Color-Coded Output**: distinct colors for different modules in multi-module setups for easy log reading.
- ⚙️ **Flexible Configuration**:
    - Define custom project types via `.sdlc.json`.
    - Set environment variables and extra flags per module via `.sdlc.conf`.
    - Declare inter-module dependencies via `depends=` in `.sdlc.conf`.
- 🔌 **Plugin System**: Extend SDLC with executable plugins (`.sdlc/plugins/`) and JSON-manifest plugins (`.sdlc/plugins.json`). Plugins can define lifecycle hooks and custom commands.
- 🪝 **Pre/Post Hooks**: Run shell commands before or after any lifecycle action via `hooks.pre` and `hooks.post` in `.sdlc.json`.
- 🎯 **Custom Actions**: Define arbitrary named commands in `.sdlc.json` that become first-class `sdlc` sub-commands.
- 🏗️ **Configurable Detection Depth**: Control how deep SDLC scans for projects with `--depth` (0 = root only, 1 = root + children, -1 = unlimited).
- 🔬 **Dry-Run Mode**: Preview what commands would be executed without actually running them (`--dry-run`).

## Installation

You can build and install `sdlc` from source using Go (1.24+):

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

# Run with watch mode enabled (per-module smart restarts)
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

# Dry-run: preview what would happen
sdlc run --dry-run

# Use custom actions defined in .sdlc.json
sdlc lint
sdlc deploy

# Scan up to 3 levels deep for projects
sdlc run --depth 3

# Unlimited recursion depth (up to safety limit of 50)
sdlc run --depth -1
```

### Command Reference

| Command | Description |
|---------|-------------|
| `run`   | Runs the application (e.g., `go run`, `npm start`). |
| `test`  | Runs the test suite (e.g., `go test`, `npm test`). |
| `build` | Compiles the application (e.g., `go build`, `npm build`). |
| `install`| Installs dependencies (e.g., `go mod download`, `npm install`). |
| `clean` | Removes build artifacts (e.g., `go clean`, `rm -rf dist`). |
| *(custom)* | Any custom action defined in `.sdlc.json` — automatically registered as a sub-command. |

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--watch` | `-w` | Enable watch mode to restart on file changes (per-module smart restarts). |
| `--all` | `-a` | Run command for all detected modules (default behavior if >1 module found). |
| `--module` | `-m` | Specify a single module/path to run (relative path). |
| `--ignore` | `-i` | Ignore specific modules in a multi-module project (supports multiple flags). |
| `--dir` | `-d` | Specify an absolute path to the project directory (default: current dir). |
| `--extra-args` | `-e` | Pass additional arguments to the underlying build tool. |
| `--config` | `-c` | Path to a custom configuration directory. |
| `--depth` | `-D` | Max recursion depth for project detection. `0` = root only, `1` = root + children (default), `-1` = unlimited (capped at 50). |
| `--dry-run` | `-n` | Show what would be executed without actually running commands. |

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

### 2. Custom Actions

You can define custom actions in `.sdlc.json` that are automatically registered as `sdlc` sub-commands:

```json
{
  "go.mod": {
    "run": "go run .",
    "test": "go test ./...",
    "custom": {
      "lint": "golangci-lint run",
      "generate": "go generate ./...",
      "deploy": "go build -o bin/app && ./bin/app deploy"
    }
  }
}
```

After adding custom actions, they become available as commands:

```bash
sdlc lint       # Runs: golangci-lint run
sdlc generate   # Runs: go generate ./...
sdlc deploy     # Runs: go build -o bin/app && ./bin/app deploy
```

Custom actions appear in the `sdlc --help` output under the "Custom Commands:" group.

### 3. Pre/Post Hooks

Hooks let you run shell commands before or after any lifecycle action:

```json
{
  "go.mod": {
    "run": "go run .",
    "build": "go build -o app",
    "hooks": {
      "pre": {
        "build": "go vet ./...",
        "test": "golangci-lint run"
      },
      "post": {
        "build": "echo 'Build complete!'",
        "test": "go test -race ./..."
      }
    }
  }
}
```

- **Pre-hooks** run before the main command. If a pre-hook fails, the main command is skipped.
- **Post-hooks** run after the main command regardless of whether it succeeded or failed.

### 4. Environment & Flags (`.sdlc.conf`)

You can place a `.sdlc.conf` file in your project root or any module subdirectory to inject environment variables or flags specific to that scope.

**Example `.sdlc.conf`:**
```properties
# Environment Variables (prefix with $)
$PORT=8080
$DB_HOST=localhost

# Extra Flags (appended to the command)
--debug
--verbose

# Dependency declarations
depends=backend,shared-lib
```

The `$` prefix is required for environment variables. Lines starting with `-` are parsed as extra flags. The `depends=` syntax declares inter-module dependencies for watch mode cascade restarts.

## Multi-Module Projects

If `sdlc` detects multiple projects (e.g., a `go.mod` in one folder and `package.json` in another), it will treat them as modules.

- **`sdlc run`**: Runs all detected modules concurrently.
- **`sdlc run -m <folder>`**: Runs only the specified module.
- **Output**: Logs from different modules are prefixed and color-coded for clarity.

### Detection Depth

By default, SDLC scans the root directory and its immediate children (depth 1). You can control this with `--depth`:

| Value | Behavior |
|-------|----------|
| `0` | Only check the root directory |
| `1` | Root + immediate children (default) |
| `2+` | Root + N levels of nesting |
| `-1` | Unlimited recursion (capped at 50 levels for safety) |

Common non-project directories (`node_modules`, `vendor`, `.git`, `build`, `dist`, etc.) are automatically skipped during detection.

## Watch Mode

Enable watch mode with `--watch` (or `-w`). SDLC uses `fsnotify` for instant, event-based file watching with 300ms debouncing to coalesce rapid successive saves (e.g., editor auto-save).

- **Single Module**: Restarts the process when files in the module change.
- **Multi-Module**: Only the module whose files changed gets restarted (smart partial restarts), not all modules. Modules with `depends=` declarations in `.sdlc.conf` trigger cascade restarts — if a dependency changes, all dependents are restarted too.

## Plugins

SDLC supports two types of plugins:

### Executable Plugins

Place executable files in `.sdlc/plugins/` (project-level) or `~/.sdlc/plugins/` (global). Each executable becomes a `sdlc` sub-command:

```bash
# Create a plugin
mkdir -p .sdlc/plugins
cat > .sdlc/plugins/my-tool << 'EOF'
#!/bin/sh
echo "Running my custom tool!"
EOF
chmod +x .sdlc/plugins/my-tool

# Use it
sdlc my-tool --dir /path/to/project
```

Project-level plugins override global plugins of the same name. Plugin executables receive `--dir` and any extra arguments.

### JSON-Manifest Plugins

Define plugins in `.sdlc/plugins.json` with lifecycle hooks:

```json
{
  "plugins": [
    {
      "name": "my-linter",
      "project_type": "node",
      "hooks": [
        {
          "name": "pre-build",
          "command": "npm run lint",
          "description": "Run linter before build",
          "priority": 10
        },
        {
          "name": "post-test",
          "command": "npm run coverage",
          "description": "Generate coverage report after tests"
        }
      ]
    }
  ]
}
```

Hooks are filtered by project type (if specified) and run in priority order (lower values first). Hook names should start with `pre-` or `post-` (e.g., `pre-build`, `post-test`).

## Contributing

Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

## License

[Apache 2.0](http://www.apache.org/licenses/LICENSE-2.0)
