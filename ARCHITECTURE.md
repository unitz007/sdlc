# SDLC — Architecture Document

> A detailed architectural breakdown of the **SDLC** project — a lightweight, unified CLI tool that simplifies the software development lifecycle across multiple languages and build tools.

---

## Table of Contents

1. [Overview](#overview)
2. [High-Level Architecture](#high-level-architecture)
3. [Project Structure](#project-structure)
4. [Component Breakdown](#component-breakdown)
   - [Entry Point (`main.go`)](#1-entry-point-maingo)
   - [CLI Layer (`cmd/`)](#2-cli-layer-cmd)
   - [Configuration Layer (`config/`)](#3-configuration-layer-config)
   - [Engine Layer (`engine/`)](#4-engine-layer-engine)
   - [Library Layer (`lib/`)](#5-library-layer-lib)
5. [Data Flow](#data-flow)
6. [Configuration System](#configuration-system)
7. [Project Detection & Multi-Module Support](#project-detection--multi-module-support)
8. [Watch Mode](#watch-mode)
9. [Command Execution Pipeline](#command-execution-pipeline)
10. [Signal Handling & Graceful Shutdown](#signal-handling--graceful-shutdown)
11. [Key Design Decisions](#key-design-decisions)
12. [Dependencies](#dependencies)
13. [Testing Strategy](#testing-strategy)
14. [Future Considerations](#future-considerations)

---

## Overview

**SDLC** (Software Development Lifecycle) is a Go-based CLI application that provides a **single, unified interface** for common development commands — `run`, `test`, `build`, `install`, and `clean` — across different project types. Instead of remembering language-specific commands, developers use `sdlc <action>` and the tool auto-detects the project type, selects the right command, and executes it.

### Core Capabilities

| Capability | Description |
|---|---|
| **Auto-detection** | Scans the working directory for known build files (`go.mod`, `package.json`, `pom.xml`, `Package.swift`) to identify project type |
| **Multi-module support** | Detects and manages multiple projects in a single repository (monorepos) with concurrent execution |
| **Watch mode** | Monitors file changes and automatically restarts the running process |
| **Interactive selection** | Prompts the user to select which modules to run when multiple are detected |
| **Dry-run mode** | Simulates what would be executed without actually running commands |
| **Color-coded output** | Assigns distinct colors per module in multi-module setups for easy log reading |
| **Environment & flag injection** | Supports project-specific env vars and extra flags via `.sdlc.conf` files |

---

## High-Level Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                         User (CLI)                           │
│  $ sdlc run --watch --module backend --extra-args="--debug"  │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│                     CLI Layer (cmd/)                         │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌───────┐ │
│  │ root.go │ │commands │ │executor │ │ PrefixWr │ │filter │  │
│  │ (flags) │ │  (run,  │ │ (bridge │ │  iter    │ │(select│  │
│  │         │ │ test…)  │ │ to lib) │ │  out)    │ │ mods) │  │
│  └─────────┘ └─────────┘ └─────────┘ └──────────┘ └───────┘  │
└──────────┬───────────────────────────┬───────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────────┐  ┌────────────────────────────────┐
│  Configuration (config/)│  │    Engine (engine/)             │
│  ┌──────────┐ ┌──────┐ │  │  ┌────────────────────────┐    │
│  │ .sdlc.j. │ │ .sdl │ │  │  │ DetectProjects()       │    │
│  │ (tasks)  │ │ c.co │ │  │  │  - Scan root dir       │    │
│  │          │ │ nf   │ │  │  │  - Scan subdirs        │    │
│  └──────────┘ └──────┘ │  │  │  - Merge local config  │    │
│  Load, LoadLocal,       │  │  │  - Deduplicate dirs    │    │
│  LoadEnvConfig          │  │  └────────────────────────┘    │
└──────────┬──────────────┘  └──────────────┬─────────────────┘
           │                                 │
           ▼                                 ▼
┌──────────────────────────────────────────────────────────────┐
│                    Library Layer (lib/)                       │
│  ┌──────────────┐        ┌──────────────────┐                │
│  │   Task       │        │    Executor       │                │
│  │  (command    │        │  (os/exec wrapper,│                │
│  │   mapping)   │        │   signal handling,│                │
│  │              │        │   env injection)  │                │
│  └──────────────┘        └──────────────────┘                │
└──────────────────────────────────────────────────────────────┘
                         │
                         ▼
            ┌─────────────────────────┐
            │   Operating System      │
            │   (shell execution)     │
            └─────────────────────────┘
```

---

## Project Structure

```
sdlc/
├── main.go              # Application entry point
├── go.mod               # Go module definition (Go 1.20)
├── go.sum               # Dependency checksums
├── .sdlc.json           # Default project configuration (build file → commands)
├── .gitignore           # Git ignore rules
├── LICENSE              # Apache 2.0 license
├── README.md            # User-facing documentation
│
├── cmd/                 # CLI layer — Cobra commands and execution orchestration
│   ├── root.go          # Root command, global flags, working directory resolution
│   ├── commands.go      # Sub-commands (run/test/build/install/clean), watch mode, output formatting
│   └── executor.go      # Bridge between CLI and lib.Executor
│
├── config/              # Configuration loading and parsing
│   └── config.go        # .sdlc.json and .sdlc.conf loading, merging, parsing
│
├── engine/              # Project detection engine
│   └── engine.go        # Build file scanning, project discovery, local config merging
│
└── lib/                 # Core library types and utilities
    ├── task.go          # Task struct — maps action names to shell commands
    ├── executor.go      # Executor struct — command execution with env/IO/signal support
    ├── task_test.go     # Unit tests for Task
    └── executor_test.go # Unit tests for Executor
```

---

## Component Breakdown

### 1. Entry Point (`main.go`)

**File:** `main.go`

Minimal entry point that delegates entirely to the CLI layer:

```go
func main() {
    cmd.Execute()
}
```

- **Responsibility:** Bootstrap the CLI. No business logic.
- **Pattern:** Thin main, all logic in `cmd` package.

---

### 2. CLI Layer (`cmd/`)

The CLI layer is the largest and most orchestrating component. It is built on [Cobra](https://github.com/spf13/cobra) and [promptui](https://github.com/manifoldco/promptui).

#### `cmd/root.go`

Defines the root `sdlc` command and all **global persistent flags**:

| Flag | Short | Type | Default | Description |
|---|---|---|---|---|
| `--dir` | `-d` | string | `""` (cwd) | Absolute path to project directory |
| `--extra-args` | `-e` | string | `""` | Extra arguments to pass to the build tool |
| `--config` | `-c` | string | `""` | Path to a custom config directory |
| `--module` | `-m` | string | `""` | Specific module/path to run |
| `--ignore` | `-i` | stringSlice | `[]` | Ignore specific modules |
| `--all` | `-a` | bool | `false` | Run command for all detected modules |
| `--watch` | `-w` | bool | `false` | Enable watch mode |
| `--dry-run` | `-n` | bool | `false` | Simulate without executing |

Also contains `resolveWorkDir()` which handles tilde (`~`) expansion for the `--dir` flag.

#### `cmd/commands.go`

The core orchestrator. Defines five sub-commands, each delegating to `executeTask()`:

| Command | Action Passed |
|---|---|
| `sdlc run` | `"run"` |
| `sdlc test` | `"test"` |
| `sdlc build` | `"build"` |
| `sdlc install` | `"install"` |
| `sdlc clean` | `"clean"` |

**Key functions:**

- **`executeTask()`** — Entry point for all commands. Resolves working directory, creates a cancellable context, and delegates to `runTask()`.
- **`runTask()`** — The main orchestration pipeline:
  1. Load configuration (local then global)
  2. Detect projects via `engine.DetectProjects()`
  3. Load root `.sdlc.conf` for environment variables
  4. Filter projects based on `--module`, `--ignore`, `--all` flags
  5. If multiple projects and no flags set → interactive selection via `promptModuleSelection()`
  6. Print multi-module status with color-coded output
  7. Dry-run mode: print commands without executing
  8. Watch mode: delegate to `watchAndRunLoop()`
  9. Normal mode: execute all selected projects concurrently with `sync.WaitGroup`
- **`filterProjects()`** — Applies `--ignore`, `--module`, and `--all` flags to narrow down the detected project list.
- **`promptModuleSelection()`** — Interactive multi-select using `promptui.Select` in a toggle loop. Default: all modules selected.
- **`watchAndRunLoop()`** — Polling-based file watcher:
  - Checks for file modifications every 500ms
  - Restarts individual modules when changes are detected
  - Manages per-module state (cancel context, wait group, last-modified time)
  - Handles graceful shutdown on context cancellation with 5s timeout
- **`runProject()`** — Prepares and executes a single project:
  1. Cleans up stale `.vite-temp` directories
  2. Resolves the command string from `Task.Command(action)`
  3. Appends extra args
  4. Performs environment variable substitution (`$KEY` and `${KEY}`) in the command string
  5. Passes to `runCommand()` for actual execution
- **`prepareProjectEnv()`** — Merges environment settings from root and module-level `.sdlc.conf` files, plus CLI `--extra-args`.
- **`PrefixWriter`** — Custom `io.Writer` that prepends a color-coded `[module-name]` prefix to each line of output for multi-module log readability.
- **`hasChanges()`** — Filesystem walker that checks modification times. Skips hidden dirs, `node_modules`, `dist`, `build`, `target`, `bin`, `pkg`, `.log/.tmp/.lock/.pid/.swp` files.
- **`printBanner()`** — ASCII art banner in cyan.

#### `cmd/executor.go`

Thin bridge between the CLI layer and `lib.Executor`:

```go
func runCommand(ctx, commandStr, dir, stdout, stderr, env) error
```

Creates an `lib.Executor`, configures it (dir, output, env), and calls `Execute()`.

---

### 3. Configuration Layer (`config/`)

**File:** `config/config.go`

Manages two configuration files:

#### `.sdlc.json` — Task Definitions

Maps **build file names** to **Task objects** (shell commands for each lifecycle action):

```json
{
  "go.mod": {
    "run": "go run main.go",
    "test": "go test .",
    "build": "go build -v"
  },
  "package.json": {
    "run": "npm run dev",
    "test": "npm test",
    "build": "npm run build"
  }
}
```

**Loading functions:**

| Function | Purpose |
|---|---|
| `Load(confDir)` | Loads from home directory (or specified dir). Creates file if missing. |
| `LoadLocal(confDir)` | Loads from project directory. Returns `nil` if missing (no creation). |

**Resolution order:** Local `.sdlc.json` (project root) is tried first; falls back to global `~/.sdlc.json`.

#### `.sdlc.conf` — Environment & Flags

Properties-style file for per-module environment variables and CLI flags:

```properties
# Environment Variables
$PORT=8080
$DB_HOST=localhost

# Extra Flags
--debug
--verbose
```

| Prefix | Parsed As |
|---|---|
| `$KEY=VALUE` | Environment variable |
| `--flag` or `--flag=value` | Extra argument / flag |

**Loading function:** `LoadEnvConfig(dir)` — reads from the given directory, returns `nil` if file doesn't exist.

---

### 4. Engine Layer (`engine/`)

**File:** `engine/engine.go`

The **Project Detection Engine**. Core type and function:

```go
type Project struct {
    Name    string   // Build file name (e.g., "go.mod")
    Path    string   // Relative path to the directory
    AbsPath string   // Absolute path to the directory
    Task    lib.Task  // Resolved task definition
}

func DetectProjects(workDir string, tasks map[string]lib.Task) ([]Project, error)
```

**Detection algorithm:**

1. **Check root directory** — scan for build files matching configured task keys (e.g., `go.mod`, `package.json`)
2. **Check immediate subdirectories** — one level deep only
3. **Deduplicate** — use `seenDirs` map (resolved via `filepath.EvalSymlinks`) to prevent duplicate entries for symlinked paths
4. **One project per directory** — first matching build file wins; subsequent build files in the same directory are ignored
5. **Local config merging** — for each directory, loads local `.sdlc.json` and merges with global tasks (local overrides global)
6. **Skipped directories** — `.git`, `.idea`, `.planner`, `node_modules`

---

### 5. Library Layer (`lib/`)

The foundational layer providing core types and utilities. Has no dependencies on other project packages.

#### `lib/task.go` — Task Type

```go
type Task struct {
    Run     string `json:"run"`
    Test    string `json:"test"`
    Build   string `json:"build"`
    Install string `json:"install"`
    Clean   string `json:"clean"`
}

func (c Task) Command(field string) (string, error)
```

A simple mapping from lifecycle **action names** to **shell command strings**. The `Command()` method validates the action name and returns the corresponding command, or an error for unknown actions.

#### `lib/executor.go` — Command Executor

```go
type Executor struct {
    cmd    *exec.Cmd
    Stdout io.Writer
    Stderr io.Writer
    Stdin  io.Reader
}
```

Wraps Go's `os/exec.Cmd` with:

- **Context-based cancellation** — uses `exec.CommandContext()` for timeout/signal propagation
- **Process group management** — sets `Setpgid: true` via `syscall.SysProcAttr` to create a new process group
- **Graceful termination** — `Cancel` function sends `SIGTERM` to the entire process group (not just the primary process), ensuring child processes are also terminated
- **Environment injection** — `SetEnv()` merges custom env vars with the current process environment
- **Working directory** — `SetDir()` sets the command's working directory
- **IO streaming** — stdout/stderr are streamed in real-time to the configured writers

---

## Data Flow

Below is the complete execution flow for a typical `sdlc run` command:

```
User runs: sdlc run --watch -m backend
         │
         ▼
    ┌─────────────┐
    │  root.go    │  Parse global flags (--watch, -m backend, etc.)
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │ commands.go │  executeTask() → resolveWorkDir()
    │             │  Create cancellable context (SIGINT/SIGTERM)
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  config/    │  Load(".sdlc.json") → local first, then global
    │  config.go  │  LoadEnvConfig() → root .sdlc.conf
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  engine/    │  DetectProjects()
    │  engine.go  │  Scan root + subdirs for matching build files
    │             │  Merge local config, deduplicate
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │ commands.go │  filterProjects() → apply --module, --ignore, --all
    │             │  (If ambiguous → promptModuleSelection())
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │ commands.go │  watchMode? ──Yes──→ watchAndRunLoop()
    │             │      │                    │
    │             │      No                  │ Poll 500ms
    │             │      │                   │ hasChanges()
    │             │      ▼                   │ Restart module
    │             │  For each project:       │
    │             │  prepareProjectEnv()     │
    │             │  runProject()            │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  lib/       │  Executor.Execute()
    │ executor.go │  - Set env, dir, IO
    │             │  - Start command
    │             │  - Stream output
    │             │  - Wait for completion
    └─────────────┘
```

---

## Configuration System

SDLC uses a **two-file configuration approach** with a clear resolution hierarchy:

### Resolution Order

```
1. CLI flags (highest priority)
   ↓
2. Module-level .sdlc.conf (env vars & args for specific module)
   ↓
3. Root-level .sdlc.conf (env vars & args for all modules)
   ↓
4. Local .sdlc.json (project root — task definitions)
   ↓
5. Global ~/.sdlc.json (home directory — fallback task definitions)
   ↓
6. Built-in defaults (lowest priority)
```

### Environment Variable Substitution

Command strings in `.sdlc.json` support environment variable interpolation:

- `$KEY` — simple substitution
- `${KEY}` — braced substitution

Variables are resolved from the merged environment map (root `.sdlc.conf` → module `.sdlc.conf`) at execution time. Keys are sorted by length (longest first) to prevent partial matches.

### Configuration Merging in Engine

When `DetectProjects()` checks a directory:

1. Load local `.sdlc.json` from that directory
2. If local tasks exist, create a merged map: global tasks as base, local tasks overlay
3. Use the merged map for build file matching in that directory

This allows project-level overrides of global task definitions.

---

## Project Detection & Multi-Module Support

### Detection Strategy

The engine scans at **two levels** only:

| Level | Description |
|---|---|
| Root | The working directory itself |
| Depth-1 subdirs | Immediate children of the working directory |

This deliberate limitation keeps detection fast and predictable for large monorepos.

### Multi-Module Behavior

| Scenario | Behavior |
|---|---|
| Single project detected | Run directly, no prompting |
| Multiple projects, `--all` flag | Run all concurrently |
| Multiple projects, `--module X` | Run only the specified module |
| Multiple projects, `--ignore Y` | Run all except the ignored module(s) |
| Multiple projects, no flags | Interactive multi-select prompt |

### Concurrent Execution

Multiple modules are launched as goroutines with a `sync.WaitGroup`. Each module gets:
- Its own goroutine
- Color-coded output via `PrefixWriter`
- Its own environment (merged from root + module configs)
- The original index preserved for consistent coloring

---

## Watch Mode

Activated with `--watch` / `-w`. Implements a **polling-based file watcher**:

```
┌──────────────────────────────────────┐
│         watchAndRunLoop()            │
│                                      │
│  ┌────────────────────────────────┐  │
│  │  Per-module state:             │  │
│  │    - cancel context            │  │
│  │    - wait group                │  │
│  │    - last modification time    │  │
│  └────────────────────────────────┘  │
│                                      │
│  Every 500ms:                        │
│    for each module:                  │
│      hasChanges(absPath, lastMod)?   │
│        Yes → cancel() + wait()       │
│             → Sleep 500ms (release)  │
│             → startProject() again   │
│                                      │
│  On context cancellation:            │
│    Cancel all module contexts        │
│    Wait for graceful stop (5s max)   │
└──────────────────────────────────────┘
```

### Files Ignored During Watch

| Category | Example |
|---|---|
| Hidden directories | `.git`, `.idea`, etc. |
| Build output | `node_modules`, `dist`, `build`, `target`, `bin`, `pkg` |
| Hidden files | Any file starting with `.` |
| Temp/log files | `*.log`, `*.tmp`, `*.lock`, `*.pid`, `*.swp` |

### Special Handling

- **Vite temp files** — Before running a Node.js/Vite project, `runProject()` cleans up `node_modules/.vite-temp` to prevent EPERM errors on restart.
- **Restart delay** — A 500ms sleep after cancellation ensures file handles are released before restarting.

---

## Command Execution Pipeline

For each module, the execution follows this pipeline:

```
1. resolveWorkDir()           → Determine working directory
2. config.Load() / LoadLocal() → Get task definitions
3. engine.DetectProjects()     → Find matching build files
4. config.LoadEnvConfig()      → Get env vars and flags (root + per-module)
5. filterProjects()            → Apply --module, --ignore, --all
6. Prepare: prepareProjectEnv() → Merge env: root.conf → module.conf → --extra-args
7. Resolve: Task.Command(action) → Get command string for the requested action
8. Append: extra args merged   → Append flags/args from conf and CLI
9. Substitute: env vars       → Replace $KEY and ${KEY} in command string
10. Execute: lib.Executor      → Run command with env, dir, IO streaming
```

---

## Signal Handling & Graceful Shutdown

SDLC handles OS signals at multiple levels:

### CLI Level
```go
ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
defer stop()
```
Creates a context that is cancelled on `Ctrl+C` or `SIGTERM`.

### Executor Level
```go
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
cmd.Cancel = func() error {
    return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}
```
- Creates a **separate process group** for each child process
- On context cancellation, sends `SIGTERM` to the **entire process group** (negative PID)
- This ensures that child processes spawned by the command (e.g., npm scripts) are also terminated

### Watch Mode Level
- On context cancellation, all module contexts are cancelled
- Each module's cancel function is called, then its wait group is observed
- A 5-second timeout prevents indefinite blocking

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| **Cobra for CLI** | Industry-standard Go CLI framework. Provides flag parsing, sub-commands, help generation out of the box. |
| **polling-based watch** | Simpler and more cross-platform than `fsnotify`. 500ms polling interval is a reasonable tradeoff between responsiveness and resource usage. |
| **Depth-1 detection only** | Prevents deep recursive scans in large monorepos. Keeps detection fast and predictable. |
| **One project per directory** | Avoids ambiguity when multiple build files (e.g., `go.mod` and `package.json`) exist in the same directory. |
| **Global + local config merge** | Allows sensible defaults at the user level while enabling project-specific overrides. |
| **Process groups** | Essential for proper cleanup of child processes (e.g., when a `go run` spawns a child process or npm scripts chain-spawn). |
| **PrefixWriter for output** | Custom `io.Writer` avoids external dependencies and provides clean, color-coded multi-module log prefixing. |
| **Simple string-split command parsing** | `strings.Split(command, " ")` is intentionally simple. Complex shell features (pipes, redirects) are not supported; users needing those should wrap commands in shell scripts. |
| **No YAML for config** | JSON is simpler, universally supported, and sufficient for the flat key-value mapping needed. `.sdlc.conf` uses a simple line-based format for env vars/flags. |

---

## Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| [github.com/spf13/cobra](https://github.com/spf13/cobra) | v1.8.1 | CLI framework: commands, flags, help |
| [github.com/manifoldco/promptui](https://github.com/manifoldco/promptui) | v0.9.0 | Interactive terminal prompts for module selection |

### Indirect Dependencies

| Dependency | Purpose |
|---|---|
| `github.com/chzyer/readline` | Terminal input handling (used by promptui) |
| `github.com/inconshreveable/mousetrap` | Windows CLI support (used by cobra) |
| `github.com/spf13/pflag` | POSIX/GNU-style flag parsing (used by cobra) |
| `golang.org/x/sys` | System call interfaces |

---

## Testing Strategy

### Unit Tests

| File | Coverage |
|---|---|
| `lib/task_test.go` | Tests `Task.Command()` for all valid actions, invalid actions, empty fields, and empty tasks |
| `lib/executor_test.go` | Tests `NewExecutor()` command parsing (single/multi-word) and `Execute()` for success and failure cases |

### Test Execution

```bash
go test ./...
```

### Testing Gaps

The following areas currently lack test coverage:

- **`cmd/`** — No tests for command orchestration, flag parsing, filtering, or watch mode
- **`config/`** — No tests for config loading, merging, `.sdlc.conf` parsing, or env variable handling
- **`engine/`** — No tests for project detection, deduplication, or local config merging
- **Integration** — No end-to-end tests that verify the full pipeline from CLI invocation to command execution

---

## Future Considerations

Based on the current architecture, the following areas could benefit from future development:

1. **Recursive detection depth** — Allow configurable scan depth beyond the current one-level limit for deeply nested monorepos.
2. **fsnotify-based watch** — Replace polling with filesystem event notifications for lower latency and reduced CPU usage.
3. **Shell syntax support** — Support pipes, redirects, and subshells in command strings (currently limited to simple space-split tokens).
4. **Parallel test execution** — Run tests for multiple modules in parallel with aggregated results.
5. **Plugin system** — Allow user-defined project types and commands via a plugin or hook mechanism.
6. **Comprehensive test coverage** — Add tests for `cmd/`, `config/`, and `engine/` packages.
7. **Smart partial restarts** — In watch mode with multi-module projects, only restart modules affected by file changes rather than all modules.
8. **Output buffering** — Buffer and merge interleaved output lines from concurrent goroutines to prevent garbled terminal output.
9. **Windows compatibility** — Current signal handling (`Setpgid`, `SIGTERM`) is Unix-specific; Windows support would require platform-specific implementations.
10. **Structured logging** — Replace `fmt.Printf` with a structured logger for better debuggability and log level control.