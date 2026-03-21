# Architect Onboarding: sdlc

## High-Level Architecture

SDLC is a lightweight, single-binary Go CLI tool that provides a unified interface for running, testing, building, installing, and cleaning software projects across different languages and build systems. It follows a classic layered architecture with four packages, each with a clear responsibility boundary.

```
┌─────────────────────────────────────────────────────────┐
│                      CLI Layer (cmd/)                    │
│  root.go  ·  commands.go  ·  executor.go                │
│  Cobra commands, flag parsing, orchestration, watch loop │
├─────────────────────────────────────────────────────────┤
│                   Engine Layer (engine/)                 │
│  engine.go                                               │
│  Project detection, directory scanning, module merging   │
├─────────────────────────────────────────────────────────┤
│                  Config Layer (config/)                  │
│  config.go                                               │
│  .sdlc.json loading, .sdlc.conf parsing, env/flag merge │
├─────────────────────────────────────────────────────────┤
│                    Lib Layer (lib/)                      │
│  task.go  ·  executor.go                                 │
│  Task domain type, OS process spawning, signal handling  │
└─────────────────────────────────────────────────────────┘
```

**Entry point:** `main.go` → `cmd.Execute()` → Cobra root command dispatch.

**Key dependencies:**
- `github.com/spf13/cobra` — CLI framework
- `github.com/manifoldco/promptui` — Interactive module selection prompts
- Go standard library (`os/exec`, `syscall`, `os/signal`, `filepath`)

## Component Responsibilities

### `main.go`
Minimal entry point. Delegates entirely to `cmd.Execute()`.

### `cmd/` — CLI & Orchestration Layer
The heaviest package; owns the full request lifecycle.

| File | Responsibility |
|------|---------------|
| `root.go` | Defines the `sdlc` root Cobra command and all persistent flags (`--dir`, `--watch`, `--module`, `--ignore`, `--all`, `--extra-args`, `--config`, `--dry-run`). Contains `resolveWorkDir()` for tilde expansion and CWD resolution. |
| `commands.go` | Registers five subcommands (`run`, `test`, `build`, `install`, `clean`), each delegating to `executeTask()`. Contains the core orchestration logic: config loading → project detection → filtering → interactive selection → execution (single-shot or watch mode). Also implements `PrefixWriter` for color-coded multi-module output, `hasChanges()` for file-watching polling, and `promptModuleSelection()` for interactive multi-select. |
| `executor.go` | Thin adapter that bridges `cmd` to `lib.Executor`. Constructs an executor, applies directory/output/env settings, and calls `Execute()`. |

### `engine/` — Project Detection Engine
Single file `engine.go` with one exported function:

- **`DetectProjects(workDir, tasks) → []Project`**: Scans the working directory and its **immediate** subdirectories for known build files (keys in the `tasks` map). Merges local `.sdlc.json` overrides with global task definitions. Deduplicates by real directory path (resolving symlinks). Enforces one project per directory.

The `Project` struct ties together a detected build file name, relative/absolute paths, and the associated `lib.Task`.

### `config/` — Configuration Layer
Single file `config.go` handling two config formats:

- **`.sdlc.json`** (JSON): Maps build-file names (e.g. `"go.mod"`) to `lib.Task` definitions. Loaded via `Load()` (global/home, auto-creates if missing) or `LoadLocal()` (project/module-scoped, returns nil if absent).
- **`.sdlc.conf`** (key=value / flags): Per-directory environment variables (`$KEY=VALUE`) and CLI flags (`--flag`). Loaded via `LoadEnvConfig()`.

Config resolution order (per `runTask` in `commands.go`):
1. Explicit `--config` directory
2. Local `.sdlc.json` in working directory
3. Global `~/.sdlc.json` (fallback)

### `lib/` — Core Domain & Execution
| File | Responsibility |
|------|---------------|
| `task.go` | Defines the `Task` struct (Run, Test, Build, Install, Clean command strings) with a `Command(field)` accessor method. Pure domain type with no I/O. |
| `executor.go` | Wraps `os/exec.Cmd` with process-group signal handling (`Setpgid: true`, SIGTERM to process group on context cancellation). Provides builder-pattern methods: `SetDir()`, `SetEnv()`, `SetOutput()`, `Execute()`. |

## Data Flow

### Normal Execution (e.g. `sdlc run --watch`)

```
User
 │
 ▼
┌──────────────┐
│  Cobra CLI   │  Parse flags, resolve --dir
│  root.go     │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ executeTask  │  Create signal-aware context (SIGINT/SIGTERM)
│ commands.go  │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  runTask     │  1. Load config (local → global fallback)
│ commands.go  │  2. Detect projects via engine.DetectProjects()
│              │  3. Load root .sdlc.conf
│              │  4. Filter projects (--module, --ignore, --all)
│              │  5. Interactive prompt if ambiguous
│              │  6. Dry-run or execute
└──────┬───────┘
       │
       ├── [single-shot] ──► goroutine per project ──► runProject()
       │                                              │
       │                                              ▼
       │                                         prepareProjectEnv()
       │                                         (merge root + module .sdlc.conf + CLI args)
       │                                              │
       │                                              ▼
       │                                         runCommand() ──► lib.Executor.Execute()
       │
       └── [watch mode] ──► watchAndRunLoop()
                              │
                              ├── Start all projects (goroutines)
                              ├── Poll every 500ms via hasChanges()
                              │   (filepath.Walk, skip .git/node_modules/etc.)
                              └── On change: cancel old context → restart module
```

### Config Merging Flow

```
~/.sdlc.json (global defaults)
        │
        ▼
local .sdlc.json (project overrides, merged on top)
        │
        ▼
engine.DetectProjects() → []Project (each with merged Task)
        │
        ▼
root .sdlc.conf (env vars + flags)
        │
        ▼
module .sdlc.conf (overrides root env/flags)
        │
        ▼
CLI --extra-args (appended last)
        │
        ▼
Final command string with $VAR substitution
```

## API Surface & Contracts

### CLI Commands

| Command | Action Key | Description |
|---------|-----------|-------------|
| `sdlc run` | `"run"` | Run the application |
| `sdlc test` | `"test"` | Run tests |
| `sdlc build` | `"build"` | Build the project |
| `sdlc install` | `"install"` | Install dependencies |
| `sdlc clean` | `"clean"` | Clean build artifacts |

### Global Flags

| Flag | Short | Type | Default | Purpose |
|------|-------|------|---------|---------|
| `--dir` | `-d` | string | `""` (CWD) | Project directory |
| `--watch` | `-w` | bool | `false` | Enable file-watch restart loop |
| `--all` | `-a` | bool | `false` | Run all detected modules |
| `--module` | `-m` | string | `""` | Target specific module by path |
| `--ignore` | `-i` | []string | `[]` | Exclude modules by path or name |
| `--extra-args` | `-e` | string | `""` | Append arguments to underlying command |
| `--config` | `-c` | string | `""` | Custom config directory path |
| `--dry-run` | `-n` | bool | `false` | Print commands without executing |

### Configuration File Contracts

**`.sdlc.json`** — `map[string]lib.Task` serialized as JSON:
```json
{
  "go.mod": {
    "run": "go run .",
    "test": "go test ./...",
    "build": "go build -o app",
    "install": "go mod download",
    "clean": "go clean"
  }
}
```

**`.sdlc.conf`** — Line-oriented, `#` comments, two line types:
- `$KEY=VALUE` → environment variable
- `--flag` or `--flag=value` → extra CLI argument

### Internal Go API

| Package | Key Exported Symbols |
|---------|---------------------|
| `lib` | `Task` struct, `Task.Command(field)`, `NewExecutor(ctx, command)`, `Executor.SetDir/SetEnv/SetOutput/Execute` |
| `engine` | `Project` struct, `DetectProjects(workDir, tasks) → ([]Project, error)` |
| `config` | `Load(confDir)`, `LoadLocal(confDir)`, `LoadEnvConfig(dir)` |

## Scalability & Performance Considerations

### Current Strengths
- **Concurrent module execution**: Multi-module projects run in parallel goroutines with `sync.WaitGroup`.
- **Process group isolation**: Each spawned process gets its own process group (`Setpgid: true`), enabling clean SIGTERM-based shutdown without orphan processes.
- **Shallow directory scanning**: `DetectProjects` only scans the root and immediate subdirectories (not recursive), keeping detection O(n) where n is the number of top-level entries.

### Bottlenecks & Risks
1. **Watch mode polling**: `hasChanges()` performs a full `filepath.Walk` every 500ms per module. For large monorepos with many files, this is expensive. No `.gitignore` awareness — only hardcoded directory exclusions (`.git`, `node_modules`, `dist`, `build`, `target`, `bin`, `pkg`).
2. **Command splitting fragility**: `lib.NewExecutor` splits commands on spaces (`strings.Split(command, " ")`). This breaks for commands with quoted arguments, e.g., `go run -ldflags "-X main.version=1.0"`. Should use shell parsing or `bash -c`.
3. **No output buffering coordination**: Multiple goroutines write to `os.Stdout`/`os.Stderr` concurrently via `PrefixWriter`. While `PrefixWriter` is not thread-safe for interleaved writes from the same module, cross-module interleaving could produce garbled output under high throughput.
4. **Watch mode restarts all modules**: A file change in any module triggers a restart of **all** modules (documented as "smart partial restarts coming soon"). This is wasteful for independent modules.

## Security Posture

### Current State
- **No privilege escalation**: Runs as the invoking user; no `sudo` or setuid.
- **Process group isolation**: Prevents signal leakage between modules.
- **No network exposure**: Purely local CLI tool; no server, no remote config fetching.

### Concerns
1. **Arbitrary command execution**: The `.sdlc.json` config defines shell commands that are executed directly. A malicious `.sdlc.json` in a cloned repo could execute arbitrary code. No sandboxing, no allow-listing of commands.
2. **Environment variable injection**: `.sdlc.conf` can set arbitrary environment variables (e.g., `PATH`, `LD_PRELOAD`). No validation or restriction.
3. **No config file integrity**: No signature verification or hash checking for config files. Supply-chain risk if a project's `.sdlc.json` is tampered with.
4. **Command injection via env substitution**: The `$VAR` substitution in `runProject()` uses simple `strings.ReplaceAll`, which could be exploited if env values contain shell metacharacters and the command is later interpreted by a shell (currently it isn't — `exec.Command` doesn't use a shell — but this is fragile).

## Structural Improvement Suggestions

### 1. Shell-Aware Command Parsing (Critical)
**File:** `lib/executor.go:25`
**Problem:** `strings.Split(command, " ")` cannot handle quoted arguments.
**Fix:** Use `exec.Command("sh", "-c", command)` or a proper shell-word parser like `github.com/kballard/go-shellquote`.

### 2. Native File Watcher (High Priority)
**File:** `cmd/commands.go:481` (`hasChanges`)
**Problem:** Polling `filepath.Walk` every 500ms is CPU-intensive and doesn't respect `.gitignore`.
**Fix:** Use `github.com/fsnotify/fsnotify` for native OS file events. Parse `.gitignore` (or reuse `golang.org/x/tools/go/buildutil` ignore patterns) to filter events.

### 3. Extract Orchestration from `cmd/` (Medium Priority)
**File:** `cmd/commands.go` (716 lines)
**Problem:** This file handles CLI definition, config loading, project filtering, interactive prompts, watch loop, output formatting, and env merging. It's difficult to test without a full Cobra setup.
**Fix:** Extract an `orchestrator` package (or expand `engine/`) to own the `runTask` / `watchAndRunLoop` / `filterProjects` logic. The `cmd/` package should be a thin CLI adapter.

### 4. Per-Module Watch Restart (Medium Priority)
**File:** `cmd/commands.go:269` (`watchAndRunLoop`)
**Problem:** Any file change restarts all modules.
**Fix:** Track which module's directory the changed file belongs to and only restart that module. This is partially scaffolded (the loop already iterates per-project) but the `hasChanges` check doesn't scope to the triggering module.

### 5. Config Validation & Safety (Medium Priority)
**Files:** `config/config.go`, `cmd/commands.go`
**Problem:** No validation of config values; arbitrary commands and env vars are accepted silently.
**Fix:** Add a `--trusted-config` flag or a config signature mechanism. At minimum, warn when executing commands from an untrusted local `.sdlc.json`.

### 6. Structured Logging (Low Priority)
**Problem:** All output goes through `fmt.Printf`/`fmt.Println` with ANSI color codes inline.
**Fix:** Introduce a `log/slog`-based logger with configurable output format (plain, JSON, colorized). This would also enable log-level control (`--verbose`, `--quiet`).

### 7. Test Coverage Expansion (Low Priority)
**Problem:** Tests exist only for `lib/` (task and executor). No tests for `engine/`, `config/`, or the orchestration logic in `cmd/`.
**Fix:** Add table-driven tests for `DetectProjects` (with temp directories), `LoadEnvConfig` (with fixture files), and `filterProjects`. Extract `runTask` logic to a testable function that accepts interfaces for config loading and project detection.

### 8. Remove Hardcoded Vite Workaround (Low Priority)
**File:** `cmd/commands.go:421-428`
**Problem:** `runProject` has a hardcoded cleanup of `node_modules/.vite-temp`. This is a framework-specific hack.
**Fix:** Make this configurable via `.sdlc.conf` (e.g., a `clean_before_run` directive) or remove it entirely and let users handle it in their build commands.
