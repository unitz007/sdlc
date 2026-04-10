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
| **Watch mode** | Monitors file changes via `fsnotify` and automatically restarts only the affected module |
| **Interactive selection** | Prompts the user to select which modules to run when multiple are detected |
| **Dry-run mode** | Simulates what would be executed without actually running commands |
| **Color-coded output** | Assigns distinct colors per module in multi-module setups for easy log reading |
| **Environment & flag injection** | Supports project-specific env vars and extra flags via `.sdlc.conf` files |
| **Custom actions** | User-defined commands in `.sdlc.json` that become first-class `sdlc` sub-commands |
| **Pre/Post hooks** | Lifecycle hooks that run before/after any action; pre-hook failure skips the main command |
| **Plugin system** | Both executable plugins (`.sdlc/plugins/`) and JSON-manifest plugins (`.sdlc/plugins.json`) with hook-based lifecycle integration |
| **Configurable detection depth** | Adjustable recursion depth via `--depth` flag (0 = root only, 1 = default, -1 = unlimited) |
| **Dependency tracking** | Inter-module `depends=` declarations in `.sdlc.conf` for cascade restarts in watch mode |

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
│  ┌─────────────┐  ┌───────────────┐                          │
│  │ plugins.go  │  │ dynamic cmds  │                          │
│  │ (exec plug) │  │ (custom act.) │                          │
│  └─────────────┘  └───────────────┘                          │
└──────────┬───────────────────────────┬───────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────────┐  ┌────────────────────────────────┐
│  Configuration (config/)│  │    Engine (engine/)             │
│  ┌──────────┐ ┌──────┐ │  │  ┌────────────────────────┐    │
│  │ .sdlc.j. │ │ .sdl │ │  │  │ DetectProjects()       │    │
│  │ (tasks)  │ │ c.co │ │  │  │  - Walk with depth     │    │
│  │          │ │ nf   │ │  │  │  - Merge local config  │    │
│  └──────────┘ └──────┘ │  │  │  - Deduplicate dirs    │    │
│  Load, LoadLocal,       │  │  └────────────────────────┘    │
│  LoadEnvConfig          │  │  ┌────────────────────────┐    │
└──────────┬──────────────┘  │  │ defaultSkipDirs()      │    │
           │                  │  └────────────────────────┘    │
           ▼                  └──────────────┬─────────────────┘
┌──────────────────────────────────────────────────────────────┐
│                    Library Layer (lib/)                       │
│  ┌──────────────┐        ┌──────────────────┐                │
│  │   Task       │        │    Executor       │                │
│  │  (command    │        │  (os/exec wrapper,│                │
│  │   mapping,   │        │   signal handling,│                │
│  │   custom,    │        │   env injection)  │                │
│  │   hooks)     │        └──────────────────┘                │
│  └──────────────┘        ┌──────────────────┐                │
│  ┌──────────────────┐    │ BufferedPrefix   │                │
│  │  lib/plugin/     │    │  Writer,         │                │
│  │  Plugin, Hook,   │    │  SyncWriter      │                │
│  │  Registry,       │    └──────────────────┘                │
│  │  HookRunner      │                                        │
│  └──────────────────┘                                        │
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
├── main.go                  # Application entry point
├── go.mod                   # Go module definition (Go 1.24)
├── go.sum                   # Dependency checksums
├── .sdlc.json               # Default project configuration (build file → commands)
├── .gitignore               # Git ignore rules
├── LICENSE                  # Apache 2.0 license
├── README.md                # User-facing documentation
├── ARCHITECTURE.md          # This file
│
├── cmd/                     # CLI layer — Cobra commands and execution orchestration
│   ├── root.go              # Root command, global flags, working directory resolution
│   ├── commands.go          # Sub-commands (run/test/build/install/clean), custom actions, watch mode, hooks, output formatting
│   ├── executor.go          # Bridge between CLI and lib.Executor
│   ├── prefix_writer.go     # PrefixWriter — line-buffered, color-coded io.Writer for multi-module output
│   ├── prefix_writer_test.go
│   ├── plugins.go           # Executable plugin discovery, registration, and execution
│   └── plugins_test.go      # Tests for plugin discovery
│
├── config/                  # Configuration loading and parsing
│   └── config.go            # .sdlc.json and .sdlc.conf loading, merging, parsing
│
├── engine/                  # Project detection engine
│   ├── engine.go            # Build file scanning, project discovery, local config merging, recursive depth
│   ├── engine_test.go       # Tests for task merging and recursive detection
│   ├── dirs.go              # defaultSkipDirs() — consolidated list of directories to skip
│   └── dirs_test.go         # Tests for skip dirs and detection scenarios
│
└── lib/                     # Core library types and utilities
    ├── task.go              # Task struct — maps action names to shell commands, custom actions, hooks
    ├── task_test.go         # Unit tests for Task
    ├── executor.go          # Executor struct — command execution with env/IO/signal support
    ├── executor_test.go     # Unit tests for Executor
    ├── buffered_writer.go   # BufferedPrefixWriter — line-buffered writer with shared OutputLock
    ├── buffered_writer_test.go
    ├── syncwriter.go        # SyncWriter — mutex-serialized per-instance line-prefix writer
    ├── syncwriter_test.go
    └── plugin/              # Hook-based plugin system
        ├── plugin.go        # Plugin type definition
        ├── hook.go          # Hook type, validation, sorting, project-type matching
        ├── registry.go      # Registry — central plugin store, LoadFile, LoadDir
        ├── runner.go        # HookRunner — sequential hook execution with RunOpts/RunResult
        ├── hook_test.go
        ├── registry_test.go
        ├── runner_test.go
        └── loader_test.go
```

---

## Component Breakdown

### 1. Entry Point (`main.go`)

**File:** `main.go`

Minimal entry point that registers dynamic commands and delegates to the CLI layer:

```go
func main() {
    var setupDone bool
    cmd.RootCmd.PersistentPreRunE = func(rc *cobra.Command, args []string) error {
        if !setupDone {
            setupDone = true
            cmd.SetupDynamicCommands()
        }
        return nil
    }
    if err := cmd.RootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

- **Responsibility:** Bootstrap the CLI, ensure dynamic commands are registered once after flag parsing.
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
| `--depth` | `-D` | int | `1` | Max recursion depth for project detection |

Also contains `resolveWorkDir()` which handles tilde (`~`) expansion for the `--dir` flag, and `SetupDynamicCommands()` which registers custom actions and plugin commands.

#### `cmd/commands.go`

The core orchestrator. Defines five built-in sub-commands plus dynamic custom actions:

| Command | Action Passed |
|---|---|
| `sdlc run` | `"run"` |
| `sdlc test` | `"test"` |
| `sdlc build` | `"build"` |
| `sdlc install` | `"install"` |
| `sdlc clean` | `"clean"` |
| `sdlc <custom>` | custom action name |

**Key functions:**

- **`RegisterDynamicCommands()`** — Scans loaded config for custom actions and registers each as a Cobra sub-command under the "Custom Commands:" group.
- **`loadConfigForDiscovery()`** — Loads config (local first, then global) to discover custom actions for dynamic sub-command registration.
- **`executeTask()`** — Entry point for all commands. Resolves working directory, creates a cancellable context, and delegates to `runTask()`.
- **`runTask()`** — The main orchestration pipeline:
  1. Load configuration (local then global)
  2. Detect projects via `engine.DetectProjects()` with configured depth
  3. Load root `.sdlc.conf` for environment variables
  4. Filter projects based on `--module`, `--ignore`, `--all` flags
  5. If multiple projects and no flags set → interactive selection via `promptModuleSelection()`
  6. Print multi-module status with color-coded output
  7. Dry-run mode: print commands without executing
  8. Watch mode: delegate to `watchAndRunLoop()`
  9. Normal mode: execute all selected projects concurrently with `sync.WaitGroup`
- **`filterProjects()`** — Applies `--ignore`, `--module`, and `--all` flags to narrow down the detected project list.
- **`promptModuleSelection()`** — Interactive multi-select using `promptui.Select` in a toggle loop. Default: all modules selected.
- **`watchAndRunLoop()`** — Event-based file watcher using `fsnotify`:
  - Creates a single `fsnotify.Watcher` with recursive directory watching
  - Tracks per-module state (`projectState` struct: `cancel`, `wg`, `lastMod`, `debounce`, `changedFile`)
  - Debounces rapid file events with 300ms `time.AfterFunc`
  - Only restarts the module whose files changed (smart partial restarts)
  - Supports dependency-aware cascade restarts via `restartWithCascade()` and `reverseDeps` map
  - Handles graceful shutdown on context cancellation with 5s timeout
- **`runProject()`** — Prepares and executes a single project:
  1. Cleans up stale `.vite-temp` directories
  2. Runs pre-hook if defined (failure skips main command)
  3. Resolves the command string from `Task.Command(action)`
  4. Performs environment variable substitution (`$KEY` and `${KEY}`)
  5. Passes to `runCommand()` for actual execution
  6. Runs post-hook regardless of main command success/failure
- **`runHook()`** — Executes a pre/post hook command for a given project and action.
- **`runPostHookIfNeeded()`** — Conditionally runs the post-hook if defined.
- **`prepareProjectEnv()`** — Merges environment settings from root and module-level `.sdlc.conf` files, plus CLI `--extra-args`.
- **`printBanner()`** — ASCII art banner in cyan.

#### `cmd/plugins.go`

Discovers and manages executable plugins:

- **`DiscoverPlugins()`** — Scans `<project>/.sdlc/plugins/` (higher priority) and `~/.sdlc/plugins/` (lower priority) for executable files. Project plugins override global plugins of the same name.
- **`scanPluginDir()`** — Scans a single directory for executable files (checks execute bit).
- **`RegisterPluginCommands()`** — Registers each discovered plugin as a Cobra sub-command under the "Plugins:" group. Plugin executables receive `--dir` and the action name as arguments.
- **`executePlugin()`** — Runs a plugin executable with `--dir <workdir> <args...>`.

#### `cmd/prefix_writer.go`

Thread-safe, line-buffered `io.Writer` that prepends a color-coded prefix to each line of output:

- **`PrefixWriter`** — Uses a global `sync.Mutex` to serialize writes across all instances, preventing garbled interleaved output in multi-module concurrent execution.
- **`Write()`** — Buffers partial writes and flushes complete lines atomically under the mutex.
- **`Flush()`** — Writes any remaining partial line content, appending a trailing newline if needed.

#### `cmd/executor.go`

Thin bridge between the CLI layer and `lib.Executor`:

```go
func runCommand(ctx, commandStr, dir, stdout, stderr, env) error
```

Creates a `lib.Executor`, configures it (dir, output, env), and calls `Execute()`.

---

### 3. Configuration Layer (`config/`)

**File:** `config/config.go`

Manages two configuration files:

#### `.sdlc.json` — Task Definitions

Maps **build file names** to **Task objects** (shell commands for each lifecycle action, plus custom actions and hooks):

```json
{
  "go.mod": {
    "run": "go run main.go",
    "test": "go test .",
    "build": "go build -v",
    "custom": {
      "lint": "golangci-lint run",
      "deploy": "kubectl apply -f k8s/"
    },
    "hooks": {
      "pre": {
        "build": "go vet ./..."
      },
      "post": {
        "test": "go test -race ./..."
      }
    }
  }
}
```

**Loading functions:**

| Function | Purpose |
|---|---|
| `Load(confDir)` | Loads from home directory (or specified dir). Creates file if missing. |
| `LoadLocal(confDir)` | Loads from project directory. Returns `nil` if missing (no creation). |

**Resolution order:** Local `.sdlc.json` (project root) is tried first; falls back to global `~/.sdlc.json`.

#### `.sdlc.conf` — Environment, Flags & Dependencies

Properties-style file for per-module environment variables, CLI flags, and dependency declarations:

```properties
# Environment Variables (prefix with $)
$PORT=8080
$DB_HOST=localhost

# Extra Flags
--debug
--verbose

# Dependency declarations (for cascade restarts in watch mode)
depends=backend,shared-lib
```

| Syntax | Parsed As |
|---|---|
| `$KEY=VALUE` | Environment variable |
| `--flag` or `--flag=value` | Extra argument / flag |
| `depends=path1,path2` | Inter-module dependency declaration |

**Loading function:** `LoadEnvConfig(dir)` — reads from the given directory, returns `nil` if file doesn't exist. Returns `EnvSettings` struct with `Env`, `Args`, and `Depends` fields.

---

### 4. Engine Layer (`engine/`)

#### `engine/engine.go` — Project Detection Engine

```go
type Project struct {
    Name    string   // Build file name (e.g., "go.mod")
    Path    string   // Relative path to the directory
    AbsPath string   // Absolute path to the directory
    Task    lib.Task // Resolved task definition
}

func DetectProjects(workDir string, tasks map[string]lib.Task, maxDepth int) ([]Project, error)
```

**Detection algorithm:**

1. **Configurable depth** — accepts `maxDepth` parameter:
   - `0` = root directory only
   - `1` = root + immediate children (default, backwards-compatible)
   - Positive integers = root + N levels deep
   - `-1` = unlimited, clamped to `maxDetectionDepth = 50` for safety
2. **`filepath.WalkDir`-based traversal** — walks the directory tree with depth calculation using `strings.Count(rel, separator)`; returns `fs.SkipDir` when depth exceeds `maxDepth`
3. **Skipped directories** — `defaultSkipDirs()` returns a consolidated map of 23 well-known non-project directories (`node_modules`, `vendor`, `venv`, `.git`, `.svn`, `.hg`, `__pycache__`, `.idea`, `.planner`, `target`, `build`, `dist`, `.next`, `.nuxt`, `coverage`, `.tox`, `.pytest_cache`, `.mypy_cache`, `.gradle`, `.cache`, `Pods`, `Carthage`, `.terraform`)
4. **Deduplicate** — use `seenDirs` map (resolved via `filepath.EvalSymlinks`) to prevent duplicate entries for symlinked paths
5. **One project per directory** — first matching build file wins; subsequent build files in the same directory are ignored
6. **Local config merging** — for each directory, loads local `.sdlc.json` and merges with global tasks via `mergeTasks()` (local overrides global for built-in fields; custom actions and hooks are merged, local wins on conflicts)

#### `engine/dirs.go` — Skip Directory Configuration

Consolidated list of directories to skip during project detection, extracted into its own file for maintainability. The `defaultSkipDirs()` function returns a `map[string]bool` with 23 entries covering build outputs, caches, VCS directories, IDE config, and language-specific directories.

---

### 5. Library Layer (`lib/`)

The foundational layer providing core types and utilities. Has no dependencies on other project packages (except `lib/plugin/` depends on `lib` for `Executor`).

#### `lib/task.go` — Task Type

```go
type TaskHooks struct {
    Pre  map[string]string `json:"pre"`
    Post map[string]string `json:"post"`
}

type Task struct {
    Run     string            `json:"run"`
    Test    string            `json:"test"`
    Build   string            `json:"build"`
    Install string            `json:"install"`
    Clean   string            `json:"clean"`
    Custom  map[string]string `json:"custom,omitempty"`
    Hooks   TaskHooks         `json:"hooks,omitempty"`
}
```

- **`Command(field)`** — Returns the shell command for a lifecycle action. Checks built-in actions first, then falls back to the `Custom` map. Returns error for unknown actions.
- **`PreHook(action)` / `PostHook(action)`** — Returns the pre/post hook command for a given action.
- **`HasCustomActions()` / `CustomActionNames()`** — Introspection helpers for custom actions.

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
- **Graceful termination** — `Cancel` function sends `SIGTERM` to the entire process group
- **Environment injection** — `SetEnv()` merges custom env vars with the current process environment
- **Working directory** — `SetDir()` sets the command's working directory
- **IO streaming** — stdout/stderr are streamed in real-time to the configured writers

#### `lib/buffered_writer.go` — Buffered Prefix Writer

```go
type OutputLock struct { mu sync.Mutex }

type BufferedPrefixWriter struct {
    lock   *OutputLock
    w      io.Writer
    prefix []byte
    buf    bytes.Buffer
    midLine bool
}
```

Line-buffered `io.Writer` that prepends a configurable prefix to each complete line. Multiple instances sharing the same `OutputLock` have their output serialized, preventing garbled interleaved lines on the terminal. Partial writes are buffered internally until a newline is received or `Flush()` is called.

#### `lib/syncwriter.go` — Sync Writer

```go
type SyncWriter struct {
    mu     sync.Mutex
    w      io.Writer
    prefix []byte
    buf    bytes.Buffer
}
```

Mutex-serialized per-instance line-prefix writer. Each `SyncWriter` instance has its own mutex and buffer, providing thread-safe line buffering. Complete lines are written atomically under the per-instance lock.

#### `lib/plugin/` — Plugin System

A hook-based plugin system for extending SDLC with user-defined lifecycle commands.

##### `lib/plugin/plugin.go` — Plugin Type

```go
type Plugin struct {
    Name        string `json:"name"`
    ProjectType string `json:"project_type,omitempty"`
    Hooks       []Hook `json:"hooks"`
}
```

Represents a single plugin that may define multiple hooks. `ProjectType` is an optional filter that restricts hooks to projects of a specific type.

##### `lib/plugin/hook.go` — Hook Type

```go
type Hook struct {
    Name        string `json:"name"`
    Command     string `json:"command"`
    Description string `json:"description,omitempty"`
    Priority    int    `json:"priority,omitempty"`
    ProjectType string `json:"project_type,omitempty"`
    HookPhase   string `json:"hook_phase,omitempty"`
}
```

- **`Phase()`** — Returns `"pre"` or `"post"` based on the hook name prefix (`pre-` / `post-`).
- **`MatchesProject(projectType)`** — Returns true if the hook's project type filter is empty or matches (case-insensitive).
- **`Validate()`** — Ensures required fields are set and hook names follow the `pre-`/`post-` convention.
- **`ByPriority`** — Implements `sort.Interface` for priority-based ordering (lower values first, stable by name).

##### `lib/plugin/registry.go` — Plugin Registry

```go
type Registry struct {
    mu      sync.RWMutex
    plugins []Plugin
}
```

Thread-safe central store for all registered plugins and hooks:

- **`Register(p)`** — Validates all hooks in a plugin and adds it to the registry.
- **`RegisterHook(pluginName, h)`** — Adds a single hook, creating a plugin if needed.
- **`GetHooks(name, projectType)`** — Returns all matching hooks sorted by priority, filtered by project type.
- **`Run(ctx, hookName, opts)`** — Executes all matching hooks sequentially, returns `[]RunResult`.
- **`RunWithStopOnError(ctx, hookName, opts)`** — Same as `Run` but stops on first failure.
- **`LoadFile(path)`** — Reads a JSON plugin manifest and registers all plugins within it.
- **`LoadDir(dir)`** — Reads all `.json` files from a directory and registers their plugins.

##### `lib/plugin/runner.go` — Hook Runner

```go
type RunOpts struct {
    Dir         string
    Stdout      io.Writer
    Stderr      io.Writer
    Env         map[string]string
    ProjectType string
}

type RunResult struct {
    Hook     Hook
    ExitCode int
    Err      error
}
```

- **`Hook.Run(ctx, opts)`** — Executes a single hook using `lib.Executor`, returns a `RunResult`.
- **`HookRunner.RunAll(ctx, hooks, opts, stopOnError)`** — Executes hooks sequentially in priority order, collecting results. Stops on first error if `stopOnError` is true.

---

## Data Flow

Below is the complete execution flow for a typical `sdlc run` command:

```
User runs: sdlc run --watch -m backend
         │
         ▼
    ┌─────────────┐
    │  main.go    │  PersistentPreRunE → SetupDynamicCommands()
    │             │  - RegisterPluginCommands() → discover & register executable plugins
    │             │  - RegisterDynamicCommands() → discover & register custom actions
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  root.go    │  Parse global flags (--watch, -m backend, --depth, etc.)
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
    │  config/    │  LoadLocal(".sdlc.json") → local first, then global
    │  config.go  │  LoadEnvConfig() → root .sdlc.conf (env, args, depends)
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │  engine/    │  DetectProjects(wd, tasks, maxDepth)
    │  engine.go  │  Walk with configurable depth
    │             │  Merge local config, deduplicate, skip known dirs
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
    │             │      No                  │ fsnotify events
    │             │      │                   │ 300ms debounce
    │             │      ▼                   │ per-module restart
    │             │  For each project:       │ cascade via depends=
    │             │  prepareProjectEnv()     │
    │             │  runProject()            │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │ commands.go │  runProject():
    │             │  1. pre-hook (if defined) — failure skips main
    │             │  2. Task.Command(action) + env substitution
    │             │  3. runCommand() → lib.Executor.Execute()
    │             │  4. post-hook (always runs)
    └─────────────┘
```

---

## Configuration System

SDLC uses a **two-file configuration approach** with a clear resolution hierarchy:

### Resolution Order

```
1. CLI flags (highest priority)
   ↓
2. Module-level .sdlc.conf (env vars, args, depends for specific module)
   ↓
3. Root-level .sdlc.conf (env vars, args, depends for all modules)
   ↓
4. Local .sdlc.json (project root — task definitions, custom actions, hooks)
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
2. If local tasks exist, create a merged map via `mergeTasks()`:
   - **Built-in fields** (`run`, `test`, `build`, `install`, `clean`): local overrides global
   - **Custom actions**: merged — global as base, local overlays; local wins on conflicts
   - **Hooks** (`pre`, `post`): merged — global as base, local overlays; local wins on conflicts
3. Use the merged map for build file matching in that directory

This allows project-level overrides of global task definitions while preserving any additional custom actions or hooks from the global config.

---

## Project Detection & Multi-Module Support

### Detection Strategy

The engine scans at a **configurable depth** controlled by the `--depth` / `-D` flag:

| Depth | Description |
|---|---|
| `0` | Root directory only |
| `1` | Root + immediate children (default, backwards-compatible) |
| `2+` | Root + N levels of nesting |
| `-1` | Unlimited recursion (clamped to `maxDetectionDepth = 50`) |

The algorithm uses `filepath.WalkDir` and calculates depth as `strings.Count(relPath, separator)`. When depth exceeds `maxDepth`, the directory is skipped via `fs.SkipDir`.

### Skipped Directories

The `defaultSkipDirs()` function in `engine/dirs.go` returns a consolidated list of 23 directories that are automatically skipped during detection:

| Category | Directories |
|---|---|
| VCS | `.git`, `.svn`, `.hg` |
| Dependencies | `node_modules`, `vendor`, `venv`, `Pods`, `Carthage` |
| Build output | `target`, `build`, `dist` |
| Caches | `__pycache__`, `.next`, `.nuxt`, `.tox`, `.pytest_cache`, `.mypy_cache`, `.gradle`, `.cache`, `coverage` |
| IDE/Tools | `.idea`, `.planner`, `.terraform` |

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

Activated with `--watch` / `-w`. Implements an **event-based file watcher** using `github.com/fsnotify/fsnotify`:

```
┌──────────────────────────────────────┐
│         watchAndRunLoop()            │
│                                      │
│  ┌────────────────────────────────┐  │
│  │  Single fsnotify.Watcher       │  │
│  │  - Recursive directory watch   │  │
│  │  - dirToProject map            │  │
│  └────────────────────────────────┘  │
│                                      │
│  ┌────────────────────────────────┐  │
│  │  Per-module projectState:      │  │
│  │    - cancel context            │  │
│  │    - wait group                │  │
│  │    - debounce timer (300ms)    │  │
│  │    - changedFile path          │  │
│  └────────────────────────────────┘  │
│                                      │
│  On fsnotify.Write/Create event:     │
│    1. Skip ignored paths             │
│    2. Find owning project            │
│    3. Reset debounce timer           │
│    4. After 300ms quiet:             │
│       - Cancel existing module       │
│       - Wait for graceful stop       │
│       - Sleep 500ms (release)        │
│       - startProject() again         │
│                                      │
│  Cascade restarts (depends=):        │
│    - Build reverse dependency graph  │
│    - BFS to find all transitive      │
│      dependents                      │
│    - Restart all in the set          │
│                                      │
│  On context cancellation:            │
│    Cancel all module contexts        │
│    Wait for graceful stop (5s max)   │
└──────────────────────────────────────┘
```

### Debouncing

A 300ms debounce interval (`watchDebounceInterval`) coalesces rapid successive file events (e.g., editor auto-save writing multiple files) into a single restart per module. Each module has its own `time.AfterFunc` timer that is reset on each event.

### Smart Partial Restarts

Unlike the earlier polling approach that restarted all modules on any change, the current implementation:

1. Tracks per-module state independently (`projectState` struct)
2. Maps each watched directory back to its owning project (`dirToProject`)
3. Only restarts the module whose files actually changed
4. Walks up the directory tree to find the owning project for any file path

### Cascade Restarts

When inter-module dependencies are declared via `depends=` in `.sdlc.conf`:

1. The `reverseDeps` map tracks which modules depend on each other
2. When a module changes, `restartWithCascade()` performs BFS to find all transitive dependents
3. All modules in the restart set (the changed module + all dependents) are restarted

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
- **New directory detection** — When `fsnotify` reports a `Create` event for a directory, it is automatically added to the watcher and mapped to the owning project.

---

## Command Execution Pipeline

For each module, the execution follows this pipeline:

```
1. resolveWorkDir()           → Determine working directory
2. config.Load() / LoadLocal() → Get task definitions (including custom actions & hooks)
3. engine.DetectProjects()     → Find matching build files (with --depth)
4. config.LoadEnvConfig()      → Get env vars, flags, and depends (root + per-module)
5. filterProjects()            → Apply --module, --ignore, --all
6. Prepare: prepareProjectEnv() → Merge env: root.conf → module.conf → --extra-args
7. Pre-hook: runHook("pre")    → Execute pre-hook if defined (failure → skip main)
8. Resolve: Task.Command(action) → Get command string (built-in or custom action)
9. Append: extra args merged   → Append flags/args from conf and CLI
10. Substitute: env vars       → Replace $KEY and ${KEY} in command string
11. Execute: lib.Executor      → Run command with env, dir, IO streaming
12. Post-hook: runPostHook()   → Execute post-hook if defined (always runs)
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
- All debounce timers are stopped
- Each module's cancel function is called, then its wait group is observed
- A 5-second timeout prevents indefinite blocking

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| **Cobra for CLI** | Industry-standard Go CLI framework. Provides flag parsing, sub-commands, help generation out of the box. |
| **fsnotify-based watch** | Event-driven file watching is more responsive and efficient than polling. 300ms debouncing coalesces rapid successive saves. |
| **Per-module state tracking** | Each module in watch mode has its own `projectState` with independent cancel/wg/debounce, enabling smart partial restarts. |
| **Configurable detection depth** | `--depth` flag with `-1` for unlimited (capped at 50) provides flexibility for both flat and deeply nested monorepos. Depth 1 maintains backward compatibility. |
| **Consolidated skip dirs** | `defaultSkipDirs()` in `engine/dirs.go` centralizes the list of 23 directories to skip, making it easy to maintain and extend. |
| **One project per directory** | Avoids ambiguity when multiple build files (e.g., `go.mod` and `package.json`) exist in the same directory. |
| **Global + local config merge** | Allows sensible defaults at the user level while enabling project-specific overrides. Custom actions and hooks are merged (not replaced). |
| **Process groups** | Essential for proper cleanup of child processes (e.g., when a `go run` spawns a child process or npm scripts chain-spawn). |
| **PrefixWriter for output** | Custom `io.Writer` avoids external dependencies and provides clean, color-coded multi-module log prefixing with a global mutex. |
| **Hook-based plugin system** | JSON-manifest plugins with `Plugin` → `Hook` → `Registry` → `HookRunner` provide a clean extension point. Hooks are validated, sorted by priority, and filterable by project type. |
| **Dual plugin types** | Executable plugins (`.sdlc/plugins/`) for simple scripts and JSON-manifest plugins (`.sdlc/plugins.json`) for structured hook definitions address different use cases. |
| **Simple string-split command parsing** | `strings.Split(command, " ")` is intentionally simple. Complex shell features (pipes, redirects) are not supported; users needing those should wrap commands in shell scripts. |
| **No YAML for config** | JSON is simpler, universally supported, and sufficient for the flat key-value mapping needed. `.sdlc.conf` uses a simple line-based format for env vars/flags. |

---

## Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| [github.com/spf13/cobra](https://github.com/spf13/cobra) | v1.8.1 | CLI framework: commands, flags, help |
| [github.com/manifoldco/promptui](https://github.com/manifoldco/promptui) | v0.9.0 | Interactive terminal prompts for module selection |
| [github.com/fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) | v1.9.0 | Event-based file watching for watch mode |

### Indirect Dependencies

| Dependency | Purpose |
|---|---|
| `github.com/chzyer/logex` | Log utilities (used by promptui) |
| `github.com/chzyer/readline` | Terminal input handling (used by promptui) |
| `github.com/inconshreveable/mousetrap` | Windows CLI support (used by cobra) |
| `github.com/spf13/pflag` | POSIX/GNU-style flag parsing (used by cobra) |
| `golang.org/x/sys` | System call interfaces |

---

## Testing Strategy

### Unit Tests

| File | Coverage |
|---|---|
| `lib/task_test.go` | Tests `Task.Command()` for all valid actions, invalid actions, empty fields, empty tasks, custom actions, hooks |
| `lib/executor_test.go` | Tests `NewExecutor()` command parsing (single/multi-word) and `Execute()` for success and failure cases |
| `lib/buffered_writer_test.go` | Tests `BufferedPrefixWriter` line buffering, partial writes, flush, concurrent safety |
| `lib/syncwriter_test.go` | Tests `SyncWriter` line buffering, partial writes, flush, thread safety |
| `lib/plugin/hook_test.go` | Tests `Hook.Validate()`, `Phase()`, `MatchesProject()`, `SortHooks()` |
| `lib/plugin/registry_test.go` | Tests `Registry.Register()`, `GetHooks()`, `LoadFile()`, `LoadDir()`, filtering |
| `lib/plugin/runner_test.go` | Tests `HookRunner.RunAll()`, stop-on-error, `RunOpts` |
| `lib/plugin/loader_test.go` | Tests plugin manifest loading, edge cases |
| `engine/engine_test.go` | Tests `mergeTasks()` for built-in fields, custom actions, hooks, nil handling |
| `engine/dirs_test.go` | Tests `DetectProjects()` with depth 0/1/2/-1, skip dirs, deduplication, relative paths |
| `cmd/plugins_test.go` | Tests `DiscoverPlugins()` — no directories, global only, project overrides global, non-executable ignored |
| `cmd/prefix_writer_test.go` | Tests `PrefixWriter` line buffering, flush, multi-instance serialization |

### Test Execution

```bash
go test ./...
```

### Test Architecture

- **`lib/`** and **`lib/plugin/`** have comprehensive test coverage for types, validation, sorting, and execution.
- **`engine/`** has thorough tests for detection with different depths, skip-dir behavior, and deduplication.
- **`cmd/`** has tests for plugin discovery and prefix writer; command orchestration and watch mode remain untested.

---

## Future Considerations

Based on the current architecture, the following areas could benefit from future development:

1. **Shell syntax support** — Support pipes, redirects, and subshells in command strings (currently limited to simple space-split tokens).
2. **Parallel test execution** — Run tests for multiple modules in parallel with aggregated results.
3. **Windows compatibility** — Current signal handling (`Setpgid`, `SIGTERM`) is Unix-specific; Windows support would require platform-specific implementations.
4. **Structured logging** — Replace `fmt.Printf` with a structured logger for better debuggability and log level control.
5. **End-to-end tests** — Integration tests that verify the full pipeline from CLI invocation to command execution.
6. **Plugin sandboxing** — Run plugins in isolated environments for security.
7. **Config validation** — Schema validation for `.sdlc.json` with clear error messages for misconfigured tasks.
