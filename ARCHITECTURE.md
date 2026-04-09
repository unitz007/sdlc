# SDLC Architecture

> A detailed architectural breakdown of the **SDLC** project — a lightweight, unified CLI tool that simplifies the software development lifecycle across different languages and build systems.

---

## Table of Contents

1. [Overview](#overview)
2. [High-Level Architecture](#high-level-architecture)
3. [Project Structure](#project-structure)
4. [Package Breakdown](#package-breakdown)
   - [cmd — CLI & Command Orchestration](#cmd--cli--command-orchestration)
   - [config — Configuration Loading](#config--configuration-loading)
   - [engine — Project Detection](#engine--project-detection)
   - [lib — Core Library](#lib--core-library)
5. [Data Flow](#data-flow)
6. [Configuration System](#configuration-system)
7. [Multi-Module / Monorepo Support](#multi-module--monorepo-support)
8. [Watch Mode](#watch-mode)
9. [Concurrency Model](#concurrency-model)
10. [Key Design Decisions](#key-design-decisions)
11. [External Dependencies](#external-dependencies)
12. [Future Considerations](#future-considerations)

---

## Overview

SDLC (`sdlc`) is a Go-based CLI that provides a **unified interface** for common development lifecycle commands — `run`, `test`, `build`, `install`, and `clean` — across different project types (Go, Node.js, Maven, Swift, etc.). It auto-detects project types by scanning for known build files (`go.mod`, `package.json`, `pom.xml`, etc.) and maps them to appropriate shell commands via a JSON configuration file.

The core value propositions are:

- **Auto-detection** — no manual configuration required for common project types.
- **Multi-module support** — handles monorepos natively with concurrent execution.
- **Watch mode** — live reload on file changes.
- **Extensibility** — custom project types can be added via `.sdlc.json`.

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        User (CLI)                           │
│  sdlc run | test | build | install | clean [flags]          │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                     cmd (Cobra Commands)                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐      │
│  │  runCmd  │ │ testCmd  │ │ buildCmd │ │ installCmd│ ...  │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘      │
│       └────────────┴───────────┴─────────────┘             │
│                         │                                    │
│               executeTask(cmd, action)                       │
│                         │                                    │
│              ┌──────────┴──────────┐                        │
│              │    runTask(ctx,wd,   │                        │
│              │       action)        │                        │
│              └──────────┬──────────┘                        │
└─────────────────────────┼────────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
   ┌─────────────┐ ┌────────────┐ ┌───────────────┐
   │   config    │ │   engine   │ │    lib         │
   │ .sdlc.json  │ │ Detect-    │ │ Executor+Task  │
   │ .sdlc.conf  │ │ Projects() │ │               │
   └──────┬──────┘ └─────┬──────┘ └───────┬───────┘
          │               │                │
          └───────────────┴────────────────┘
                          │
                          ▼
               ┌────────────────────┐
               │  OS / Shell        │
               │  (subprocess exec)  │
               └────────────────────┘
```

---

## Project Structure

```
sdlc/
├── main.go                 # Entry point — calls cmd.Execute()
├── go.mod                  # Go module definition (Go 1.20)
├── go.sum                  # Dependency checksums
├── .sdlc.json              # Default project-type definitions (checked into repo)
├── README.md               # User-facing documentation
├── cmd/
│   ├── root.go             # Cobra root command, global flags, workdir resolution
│   ├── commands.go         # Subcommands (run/test/build/install/clean) and orchestration
│   └── executor.go         # Thin wrapper around lib.Executor for the cmd package
├── config/
│   └── config.go           # Configuration loading (.sdlc.json + .sdlc.conf)
├── engine/
│   └── engine.go           # Project auto-detection and scanning
└── lib/
    ├── executor.go          # Subprocess execution with context & signal handling
    ├── executor_test.go     # Executor tests
    ├── task.go              # Task type — maps lifecycle actions to shell commands
    └── task_test.go          # Task tests
```

---

## Package Breakdown

### `cmd` — CLI & Command Orchestration

**Files:**

| File | Purpose |
|------|---------|
| `root.go` | Defines the Cobra root command (`sdlc`), persistent flags (`--dir`, `--watch`, `--module`, `--ignore`, `--all`, `--extra-args`, `--config`, `--dry-run`), and `resolveWorkDir()` for directory/tilde handling. |
| `commands.go` | Registers subcommands (`run`, `test`, `build`, `install`, `clean`), contains the **core orchestration logic** (`executeTask`, `runTask`, `watchAndRunLoop`, `prepareProjectEnv`, `runProject`, `filterProjects`, `promptModuleSelection`), and ANSI color-coded output handling (`PrefixWriter`). |
| `executor.go` | Thin adapter that bridges `cmd` → `lib.Executor`, passing context, working directory, environment, and I/O writers. |

**Key Types and Functions:**

- **`executeTask(cmd, action)`** — Entry point for all subcommands. Resolves working directory, sets up signal-aware context, delegates to `runTask`.
- **`runTask(ctx, wd, action)`** — Loads configuration, detects projects, filters/selection, dry-run mode, watch-mode dispatch, or concurrent execution.
- **`watchAndRunLoop(ctx, projects, allProjects, action, rootEnvConfig)`** — Polling-based file watcher (500ms interval). Manages per-project goroutines with cancel/restart semantics. Respects `.gitignore` patterns and skips common build artifact directories.
- **`PrefixWriter`** — `io.Writer` wrapper that prefixes each line with a colored `[module-path]` tag for multi-module log disambiguation.
- **`promptModuleSelection(projects)`** — Interactive multi-select using `promptui` when multiple modules are detected without explicit `--module` or `--all` flags.

**Global Flags (defined in `root.go`):**

| Flag | Short | Type | Default | Purpose |
|------|-------|------|---------|---------|
| `--dir` | `-d` | string | `""` (cwd) | Absolute path to project directory |
| `--extra-args` | `-e` | string | `""` | Extra arguments passed to build tool |
| `--config` | `-c` | string | `""` | Config directory (defaults to `$HOME`) |
| `--module` | `-m` | string | `""` | Target a specific module |
| `--ignore` | `-i` | stringSlice | `[]` | Ignore specific modules |
| `--all` | `-a` | bool | `false` | Run for all detected modules |
| `--watch` | `-w` | bool | `false` | Enable watch/live-reload mode |
| `--dry-run` | `-n` | bool | `false` | Simulate without executing |

---

### `config` — Configuration Loading

**File:** `config/config.go`

**Constants:**

| Name | Value | Purpose |
|------|-------|---------|
| `configFileName` | `.sdlc.json` | Project type definitions (build-file → commands mapping) |
| `envConfigName` | `.sdlc.conf` | Per-directory environment variables and flags |

**Types:**

```go
type EnvSettings struct {
    Env  map[string]string  // Environment variables ($KEY=VALUE)
    Args []string           // CLI flags (--flag or --flag=value)
}
```

**Functions:**

| Function | Description |
|----------|-------------|
| `Load(confDir)` | Reads `.sdlc.json` from `confDir` (or `$HOME` if empty). Creates the file if it doesn't exist. Returns `map[string]lib.Task`. |
| `LoadLocal(dir)` | Reads `.sdlc.json` from `dir`. Returns `nil` (without error) if the file doesn't exist — used for project-local overrides. |
| `LoadEnvConfig(dir)` | Parses `.sdlc.conf` from `dir`. Lines starting with `$` are env vars, lines starting with `-` are flags. Comments (`#`) and blank lines are skipped. |

**Configuration Resolution Order:**

1. **Global config:** `~/.sdlc.json` (loaded via `Load("")`)
2. **Local config:** `.sdlc.json` in project directory (loaded via `LoadLocal(wd)`)
3. **Merge:** Local definitions override global ones for the same build-file key
4. **Env config:** `.sdlc.conf` files are loaded per-module and merged with root `.sdlc.conf`

---

### `engine` — Project Detection

**File:** `engine/engine.go`

**Types:**

```go
type Project struct {
    Name    string   // Build file name (e.g., "go.mod", "package.json")
    Path    string   // Relative path from workDir (e.g., ".", "backend")
    AbsPath string   // Absolute path to the directory
    Task    lib.Task // The resolved task definition
}
```

**Functions:**

| Function | Description |
|----------|-------------|
| `DetectProjects(workDir, tasks)` | Scans `workDir` and immediate subdirectories for build files matching keys in `tasks`. Returns `[]Project`. Skips `.git`, `.idea`, `.planner`, `node_modules` directories. Merges local `.sdlc.json` with global tasks for each directory. Enforces at most one project per directory. |

**Detection Strategy:**

```
workDir/
├── go.mod           → Project{Name:"go.mod", Path:".", ...}
├── backend/
│   └── pom.xml      → Project{Name:"pom.xml", Path:"backend", ...}
├── frontend/
│   └── package.json → Project{Name:"package.json", Path:"frontend", ...}
└── .git/            → (skipped)
```

- Scans root first, then immediate child directories (depth 1).
- Uses `filepath.EvalSymlinks` to resolve symlinks and prevent duplicates.
- Local `.sdlc.json` in each directory overlays onto the global task definitions.

---

### `lib` — Core Library

**Files:**

| File | Purpose |
|------|---------|
| `task.go` | Defines the `Task` struct and its `Command(field)` method |
| `executor.go` | Defines the `Executor` struct for subprocess execution |
| `task_test.go` | Unit tests for `Task.Command()` |
| `executor_test.go` | Unit tests for `Executor` creation and execution |

**Types:**

```go
type Task struct {
    Run     string `json:"run"`     // Command to run the project
    Test    string `json:"test"`    // Command to run tests
    Build   string `json:"build"`  // Command to build the project
    Install string `json:"install"` // Command to install dependencies
    Clean   string `json:"clean"`  // Command to clean artifacts
}

type Executor struct {
    cmd    *exec.Cmd
    Stdout io.Writer
    Stderr io.Writer
    Stdin  io.Reader
}
```

**`Task.Command(field)`** — Returns the shell command string for a given action (`"run"`, `"test"`, `"build"`, `"install"`, `"clean"`). Returns an error for unknown actions.

**`Executor`** lifecycle:

1. `NewExecutor(ctx, command)` — Parses command string, creates `exec.Cmd` with context, sets process group (`Setpgid: true`) for signal propagation, configures `Cancel` function to send `SIGTERM` to the process group.
2. `SetDir(dir)` — Sets working directory.
3. `SetEnv(env)` — Merges custom env vars onto `os.Environ()`.
4. `SetOutput(stdout, stderr)` — Redirects I/O (used for color-coded `PrefixWriter` in multi-module mode).
5. `Execute()` — Starts and waits for the command. Streams output in real-time.

---

## Data Flow

### Single Command Execution (e.g., `sdlc run`)

```
1. User invokes: sdlc run
2. Cobra dispatches → executeTask(cmd, "run")
3. resolveWorkDir(workDir) resolves the working directory
4. Signal-aware context created (SIGINT/SIGTERM → cancel)
5. runTask(ctx, wd, "run"):
   a. Load configuration:
      - Try LoadLocal(wd) first (project .sdlc.json)
      - Fall back to Load("") (global ~/.sdlc.json)
   b. DetectProjects(wd, tasks) → []Project
   c. LoadEnvConfig(wd) → root env settings
   d. filterProjects(projects):
      - Apply --ignore, --module, --all flags
      - Interactive prompt if multiple modules detected
   e. Dry-run? Print commands and return.
   f. Watch mode? Enter watchAndRunLoop.
   g. Otherwise, concurrent execution:
      - For each project, goroutine calls runProject()
      - runProject() resolves env vars ($KEY) in command string
      - runCommand() → lib.Executor → subprocess
      - PrefixWriter tags multi-module output
6. Wait for all goroutines → return
```

### Watch Mode Flow

```
1. Initial: Start all projects concurrently
2. Every 500ms: Poll filesystem for changes
   - Walk directory tree (skip .git, node_modules, dist, build, target, bin, pkg)
   - Check ModTime > lastMod for each file
   - If changed: cancel existing goroutine → restart
3. On signal (SIGINT/SIGTERM): Cancel all contexts, wait up to 5s
```

---

## Configuration System

### `.sdlc.json` — Project Type Definitions

A JSON file mapping build-file names to lifecycle commands:

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

**Resolution order:**
1. Local `.sdlc.json` in project root (takes precedence)
2. Global `~/.sdlc.json` (fallback)

### `.sdlc.conf` — Environment & Flags

A properties-style file per directory:

```properties
# Environment variables
$PORT=8080
$DB_HOST=localhost

# Extra flags
--debug
--verbose
```

**Resolution order:**
1. Root `.sdlc.conf` (in working directory)
2. Module `.sdlc.conf` (in each project subdirectory) — merges on top
3. CLI `--extra-args` flag — appended last

**Environment variable substitution:** Variables defined in `.sdlc.conf` are substituted into command strings using `${KEY}` or `$KEY` syntax.

---

## Multi-Module / Monorepo Support

When multiple projects are detected:

1. **Auto-detection:** `engine.DetectProjects` scans root + immediate subdirectories for known build files.
2. **Filtering:** `--module`, `--ignore`, and `--all` flags allow selective execution.
3. **Interactive selection:** If multiple modules are found with no flags, `promptModuleSelection` presents a toggleable checklist.
4. **Concurrent execution:** Each module runs in its own goroutine; output is prefixed and color-coded using a rotating palette of 5 colors.
5. **Environment merging:** Root + per-module `.sdlc.conf` settings are merged hierarchically.

---

## Watch Mode

Enabled via `--watch` / `-w` flag:

- **Polling:** 500ms interval filesystem walk using `filepath.Walk`.
- **Ignored directories:** `.git`, `.idea`, hidden dirs, `node_modules`, `dist`, `build`, `target`, `bin`, `pkg`.
- **Ignored files:** Hidden files, `.log`, `.tmp`, `.lock`, `.pid`, `.swp` files.
- **Restart:** On detected change, cancels running goroutine, waits for cleanup, then restarts.
- **Graceful shutdown:** On SIGINT/SIGTERM, cancels all contexts, waits up to 5 seconds for goroutines to finish.
- **Vite temp cleanup:** Removes `node_modules/.vite-temp` on module restart to prevent EPERM errors.

---

## Concurrency Model

```
┌────────────────────────────────┐
│        Main Goroutine          │
│  (signal.NotifyContext)        │
│                                │
│  ┌──────────┐ ┌──────────┐    │
│  │ Goroutine│ │ Goroutine│    │  Each project gets its own
│  │ Project A│ │ Project B│    │  goroutine in multi-module
│  └──────────┘ └──────────┘    │
│                                │
│  sync.WaitGroup for completion │
└────────────────────────────────┘

Watch Mode:
┌──────────────────────────────────┐
│        Main Goroutine            │
│  (ticker + select loop)          │
│                                  │
│  Per project: context.CancelFunc │
│  Per project: sync.WaitGroup    │
│                                  │
│  Restart = cancel → wait →      │
│    new context → new goroutine  │
└──────────────────────────────────┘
```

- **Non-watch:** `sync.WaitGroup` ensures all project goroutines complete before exit.
- **Watch:** Each project has a `projectState` struct tracking its `cancel`, `wg`, and `lastMod`. Restarts are serialized per-project with a mutex.
- **Process groups:** `Executor` uses `syscall.Setpgid: true` and `syscall.Kill(-pid, SIGTERM)` to propagate signals to child processes.

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Cobra for CLI** | De facto standard for Go CLIs; provides flag parsing, subcommands, and help generation. |
| **`promptui` for interactive selection** | Enables toggling of modules in multi-module projects with a familiar terminal UI. |
| **JSON for project definitions** | Simple, human-readable, easily extensible for new project types. |
| **Properties-style `.sdlc.conf`** | Lightweight config for env vars and flags; no need for JSON complexity here. |
| **Polling-based watch** | No filesystem dependencies (e.g., inotify, FSEvents); works cross-platform. |
| **Process groups** | Ensures child processes are properly cleaned up on cancellation (kill entire process group). |
| **One project per directory** | Prevents conflicting commands in the same directory (e.g., both `go.mod` and `package.json` in root). |
| **Config overlay (local overrides global)** | Allows project-specific command customization while maintaining sensible defaults. |
| **Direct `exec.CommandContext`** | Simple subprocess model — no complex build system integration needed for the CLI's scope. |

---

## External Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| [`github.com/spf13/cobra`](https://github.com/spf13/cobra) | v1.8.1 | CLI framework — commands, flags, help text |
| [`github.com/manifoldco/promptui`](https://github.com/manifoldco/promptui) | v0.9.0 | Interactive terminal selection UI |
| [`golang.org/x/sys`](https://pkg.go.dev/golang.org/x/sys) | (indirect) | Low-level OS/syscall support (indirect dep of promptui) |

**Standard library highlights:**
- `os/exec` — Subprocess execution
- `os/signal` — SIGINT/SIGTERM handling
- `context` — Cancellation propagation
- `encoding/json` — Config file parsing
- `path/filepath` — Directory walking and symlink resolution
- `sync` — WaitGroup, Mutex for concurrency
- `syscall` — Process group management

---

## Future Considerations

1. **Filesystem watch via fsnotify** — Replace polling with OS-native file events for lower CPU usage and faster change detection.
2. **Smart partial restarts** — In watch mode, only restart the module whose files changed rather than all modules.
3. **Config validation** — Add schema validation for `.sdlc.json` to provide clear error messages.
4. **Plugin system** — Allow custom lifecycle hooks (pre-build, post-test) in `.sdlc.conf`.
5. **Parallel test execution** — Support for running tests across modules in parallel with aggregation.
6. **Output buffering** — Buffer and time-stamp multi-module output for deterministic log ordering.
7. **Nested monorepo support** — Extend detection beyond depth-1 subdirectories for deeply nested module structures.
8. **Windows compatibility** — Process group handling currently uses Unix-specific `syscall.SysProcAttr`; needs conditional compilation for Windows.