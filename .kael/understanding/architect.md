# Architect Onboarding: sdlc

## High-Level Architecture

**sdlc** is a lightweight Go CLI tool that provides a unified interface for software development lifecycle commands (`run`, `test`, `build`, `install`, `clean`) across heterogeneous project types. It auto-detects project types by scanning for build files and executes the appropriate underlying toolchain commands.

The architecture follows a clean layered pattern with four packages:

```
┌─────────────────────────────────────────────────────┐
│                    main.go                          │
│                  (entry point)                      │
└──────────────────────┬──────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────┐
│              cmd/  (CLI Layer)                      │
│  root.go ─── commands.go ─── executor.go            │
│  Cobra commands, flags, orchestration, watch mode   │
└──────┬───────────────┬──────────────────┬───────────┘
       │               │                  │
┌──────▼──────┐ ┌──────▼──────┐  ┌───────▼──────────┐
│ engine/     │ │ config/     │  │ lib/             │
│ engine.go   │ │ config.go   │  │ task.go          │
│ Project     │ │ Task defs   │  │ executor.go      │
│ detection   │ │ Env/args    │  │ Process mgmt     │
└─────────────┘ └─────────────┘  └──────────────────┘
```

**Stack:** Go 1.20+, [spf13/cobra](https://github.com/spf13/cobra) (CLI framework), [manifoldco/promptui](https://github.com/manifoldco/promptui) (interactive prompts).

## Component Responsibilities

### `main.go`
Minimal entry point — delegates entirely to `cmd.Execute()`.

### `cmd/` — CLI & Orchestration Layer

| File | Responsibility |
|------|---------------|
| `root.go` | Defines the root `sdlc` Cobra command and all **global flags** (`--dir`, `--watch`, `--module`, `--ignore`, `--all`, `--extra-args`, `--config`, `--dry-run`). Contains `resolveWorkDir()` for tilde expansion and CWD resolution. |
| `commands.go` | Registers five subcommands (`run`, `test`, `build`, `install`, `clean`), each delegating to `executeTask()`. Houses the **core orchestration logic**: config loading, project detection, filtering, interactive selection, dry-run simulation, concurrent execution, and the **watch mode** loop. Also contains `PrefixWriter` for color-coded multi-module output and `hasChanges()` for file-watching. |
| `executor.go` | Thin adapter that bridges the `cmd` layer to `lib.Executor`, setting directory, output writers, and environment variables. |

### `engine/` — Project Detection Engine

| File | Responsibility |
|------|---------------|
| `engine.go` | `DetectProjects(workDir, tasks)` scans the working directory and its **immediate subdirectories** for known build files (e.g., `go.mod`, `package.json`). Returns `[]Project` structs. Handles symlink deduplication, one-project-per-directory enforcement, and merges local config overrides with global task definitions. |

### `config/` — Configuration Management

| File | Responsibility |
|------|---------------|
| `config.go` | Manages two config formats: **`.sdlc.json`** (task definitions mapping build-file names to lifecycle commands) and **`.sdlc.conf`** (environment variables and extra flags). Implements a three-tier loading strategy: explicit `--config` path → local project `.sdlc.json` → global `~/.sdlc.json`. Auto-creates an empty global config if missing. |

### `lib/` — Core Library Types

| File | Responsibility |
|------|---------------|
| `task.go` | Defines the `Task` struct with fields `Run`, `Test`, `Build`, `Install`, `Clean` (all `string`, JSON-tagged). Provides `Command(field)` to retrieve the command for a given action. |
| `executor.go` | Wraps `os/exec.Cmd` with process group management (`Setpgid: true`) for graceful shutdown. Sends `SIGTERM` to the entire process group on context cancellation. Supports configurable working directory, environment, and I/O streams. |

## Data Flow

### Primary Execution Flow (e.g., `sdlc run`)

```
User runs: sdlc run --watch --ignore frontend
         │
         ▼
    ┌────────────┐
    │  Cobra CLI  │  Parse flags, resolve --dir (tilde expansion)
    └─────┬──────┘
          │
          ▼
    ┌──────────────────────────────────────────┐
    │  Config Loading (config.Load / LoadLocal) │
    │  1. Try --config path                     │
    │  2. Try <workDir>/.sdlc.json              │
    │  3. Fallback to ~/.sdlc.json              │
    └─────────────┬────────────────────────────┘
                  │  map[string]lib.Task
                  ▼
    ┌──────────────────────────────────────────┐
    │  Project Detection (engine.DetectProjects)│
    │  Scan workDir + immediate subdirs for     │
    │  build files matching task config keys    │
    └─────────────┬────────────────────────────┘
                  │  []engine.Project
                  ▼
    ┌──────────────────────────────────────────┐
    │  Env Config Loading (config.LoadEnvConfig)│
    │  Load root .sdlc.conf + per-module confs  │
    └─────────────┬────────────────────────────┘
                  │
                  ▼
    ┌──────────────────────────────────────────┐
    │  Project Filtering (filterProjects)       │
    │  Apply --module, --ignore, --all flags    │
    └─────────────┬────────────────────────────┘
                  │
                  ▼
    ┌──────────────────────────────────────────┐
    │  Interactive Selection (if ambiguous)     │
    │  promptui multi-select toggle loop        │
    └─────────────┬────────────────────────────┘
                  │
          ┌───────┴────────┐
          │                │
     ┌────▼─────┐    ┌────▼─────┐
     │ --watch  │    │ one-shot │
     │  mode    │    │  mode    │
     └────┬─────┘    └────┬─────┘
          │               │
          ▼               ▼
   ┌──────────────┐  ┌──────────────────┐
   │ watchAndRun  │  │ Concurrent exec  │
   │ Loop:        │  │ (goroutine per   │
   │ 500ms poll,  │  │  selected module)│
   │ restart on   │  │                  │
   │ file change  │  │                  │
   └──────┬───────┘  └────────┬─────────┘
          │                   │
          ▼                   ▼
   ┌──────────────────────────────────────┐
   │  runProject (per module)             │
   │  1. Clean .vite-temp (workaround)    │
   │  2. Get command from Task            │
   │  3. Append extra args                │
   │  4. Substitute $VAR / ${VAR} in cmd  │
   │  5. lib.Executor.Execute()           │
   └──────────────────────────────────────┘
```

### Configuration Cascade

```
Priority (highest → lowest):

Task Definitions:
  --config/.sdlc.json  →  <project>/.sdlc.json  →  ~/.sdlc.json
  (explicit path)        (local overrides)         (global defaults)

Environment & Args:
  CLI --extra-args  →  <module>/.sdlc.conf  →  <root>/.sdlc.conf
  (appended last)      (module-level)           (root-level)
```

### Watch Mode Flow

```
watchAndRunLoop()
  │
  ├── startProject() for each selected module
  │     ├── Cancel previous run (if restarting)
  │     ├── Create child context
  │     └── Launch goroutine → runProject()
  │
  └── 500ms ticker loop
        ├── On ctx.Done(): cancel all, wait with 5s timeout
        └── On tick: hasChanges() via filepath.Walk
              ├── Skip: dotfiles, node_modules, dist, build, target, bin, pkg
              ├── Skip: .log, .tmp, .lock, .pid, .swp files
              └── If changed: startProject() to restart
```

## API Surface & Contracts

### CLI Commands

| Command | Action Key | Description |
|---------|-----------|-------------|
| `sdlc run` | `run` | Run the application |
| `sdlc test` | `test` | Run the test suite |
| `sdlc build` | `build` | Compile the project |
| `sdlc install` | `install` | Install dependencies |
| `sdlc clean` | `clean` | Remove build artifacts |

### Global Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--dir` | `-d` | string | `""` (CWD) | Project directory (supports `~/`) |
| `--watch` | `-w` | bool | `false` | Enable file-watch restart mode |
| `--all` | `-a` | bool | `false` | Run all detected modules |
| `--module` | `-m` | string | `""` | Target a specific module by path |
| `--ignore` | `-i` | []string | `[]` | Exclude modules by path or name |
| `--extra-args` | `-e` | string | `""` | Append arguments to underlying command |
| `--config` | `-c` | string | `""` | Custom config directory path |
| `--dry-run` | `-n` | bool | `false` | Print commands without executing |

### Configuration File Contracts

**`.sdlc.json`** — Maps build-file basenames to `Task` objects:
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

**`.sdlc.conf`** — Key-value env vars (`$KEY=VALUE`) and flags (`--flag`):
```properties
$PORT=8080
--verbose
```

### Key Internal Types

```go
// lib.Task — maps build-file names to lifecycle commands
type Task struct {
    Run     string `json:"run"`
    Test    string `json:"test"`
    Build   string `json:"build"`
    Install string `json:"install"`
    Clean   string `json:"clean"`
}
func (c Task) Command(field string) (string, error)

// engine.Project — a detected project in the workspace
type Project struct {
    Name    string   // build file name (e.g., "go.mod")
    Path    string   // relative path from workDir
    AbsPath string   // absolute path
    Task    lib.Task // associated task definition
}

// config.EnvSettings — parsed .sdlc.conf contents
type EnvSettings struct {
    Env  map[string]string
    Args []string
}

// lib.Executor — wraps os/exec.Cmd with process group management
type Executor struct { ... }
func NewExecutor(ctx context.Context, command string) *Executor
func (e *Executor) SetDir(dir string)
func (e *Executor) SetEnv(env map[string]string)
func (e *Executor) SetOutput(stdout, stderr io.Writer)
func (e *Executor) Execute() error
```

## Scalability & Performance Considerations

### Current Strengths
- **Concurrent module execution**: Multi-module projects run via goroutines with `sync.WaitGroup`, providing parallelism.
- **Process group isolation**: Each spawned process runs in its own process group (`Setpgid: true`), enabling clean shutdown without orphan processes.
- **Shallow directory scanning**: `DetectProjects` only scans the root and immediate subdirectories (depth=1), keeping detection fast even in large repos.

### Bottlenecks & Risks
1. **Watch mode polling**: Uses `filepath.Walk` every 500ms across all project directories. For large codebases (100k+ files), this becomes CPU-intensive. A platform-native file watcher (`fsnotify`) would be significantly more efficient.
2. **Command splitting naively**: `NewExecutor` splits commands on spaces (`strings.Split(command, " ")`), which breaks commands with quoted arguments or complex shell syntax (e.g., `go run -ldflags "-X main.version=1.0"`). Should use `sh -c` or a proper shell parser.
3. **No output buffering coordination**: Multiple `PrefixWriter` instances write to `os.Stdout`/`os.Stderr` concurrently without synchronization, which can interleave output from different modules mid-line.
4. **Hard-coded 500ms sleep on restart**: `watchAndRunLoop` sleeps 500ms after cancelling a process before restarting, which is arbitrary and may be too long for fast rebuilds or too short for slow cleanup.
5. **No depth control for nested monorepos**: Detection is hardcoded to depth=1. Deeply nested monorepos (e.g., `apps/web/frontend/`) won't be detected.

## Security Posture

### Current State
- **No privilege escalation**: Runs as the invoking user; no setuid or elevated permissions.
- **Process group isolation**: Child processes are isolated in their own process groups, preventing signal leakage.
- **Graceful shutdown**: Uses `SIGTERM` (not `SIGKILL`) for cancellation, allowing child processes to clean up.

### Concerns
1. **Arbitrary command execution**: The `.sdlc.json` config file defines shell commands that are executed directly. A malicious or compromised `.sdlc.json` in a cloned repo will execute arbitrary code when `sdlc` is run. There is no sandboxing, no prompt before first execution, and no signature verification.
2. **Environment variable injection**: `.sdlc.conf` can set arbitrary environment variables (e.g., `PATH`, `LD_PRELOAD`) that are passed to child processes. No validation or allowlist exists.
3. **No config file integrity check**: No checksum or signature verification for `.sdlc.json` or `.sdlc.conf`.
4. **Command injection via env substitution**: The `$VAR` / `${VAR}` substitution in `runProject` uses simple `strings.ReplaceAll`, which could be exploited if env values contain shell metacharacters (though this is somewhat mitigated by `exec.Command` not using a shell).

### Recommendations
- Add a `--trust` or confirmation prompt when running in a new/unknown project for the first time.
- Consider an allowlist of permitted environment variable names in `.sdlc.conf`.
- Document the trust model clearly: users must trust the `.sdlc.json` in any repo they run `sdlc` in.

## Structural Improvement Suggestions

### 1. Use a Proper Shell for Command Execution
**File:** `lib/executor.go:25-27`

The current space-splitting approach is fragile. Commands with quoted arguments, pipes, or redirections will break.
```go
// Current (fragile):
program := strings.Split(command, " ")[0]
cmd := exec.CommandContext(ctx, program, strings.Split(command, " ")[1:]...)

// Recommended:
cmd := exec.CommandContext(ctx, "sh", "-c", command)
```
This preserves shell semantics while keeping the process group management.

### 2. Replace Polling with `fsnotify`
**File:** `cmd/commands.go:481-521` (`hasChanges`) and `cmd/commands.go:333` (ticker)

The 500ms polling loop with `filepath.Walk` is the biggest performance concern. Using [fsnotify](https://github.com/fsnotify/fsnotify) would provide instant, event-driven file change detection with minimal CPU overhead.

### 3. Extract Orchestration from CLI Layer
**File:** `cmd/commands.go`

The `commands.go` file (726 lines) contains both CLI concerns (banner printing, color codes, promptui interaction) and core orchestration logic (watch loop, project filtering, env merging, command substitution). This should be split:

```
cmd/commands.go     → CLI wiring only (thin)
engine/orchestrator.go → runTask, watchAndRunLoop, prepareProjectEnv
engine/watcher.go      → file watching abstraction
engine/filter.go       → project filtering logic
```

### 4. Synchronized Output Writer
**File:** `cmd/commands.go:524-559` (`PrefixWriter`)

Multiple goroutines write through independent `PrefixWriter` instances to shared `os.Stdout`/`os.Stderr`. Add a mutex:
```go
type PrefixWriter struct {
    w       io.Writer
    prefix  []byte
    midLine bool
    mu      sync.Mutex  // Add this
}
```

### 5. Config Validation
**File:** `config/config.go`

Add validation when loading `.sdlc.json`:
- Warn on empty command strings (e.g., `"install": ""` means the action is a no-op).
- Validate that keys look like filenames (no path separators, no spaces).
- Schema versioning for forward compatibility.

### 6. Make Watch Mode Smarter
**File:** `cmd/commands.go:269-389`

Current watch mode restarts ALL modules when ANY file changes. The README mentions "smart partial restarts coming soon." Implement per-module change detection so only the affected module restarts.

### 7. Add Integration Tests
**Files:** `lib/executor_test.go`, `lib/task_test.go`

Only unit tests exist for `lib/`. The `engine/` and `config/` packages have zero test coverage. Critical paths like `DetectProjects`, config merging, and the watch loop should have integration tests using temporary directories.

### 8. Remove Hard-Coded Vite Workaround
**File:** `cmd/commands.go:420-428`

The `.vite-temp` cleanup is a domain-specific hack embedded in generic orchestration code. This should be configurable (e.g., a `preRun` hook in `.sdlc.json`) or removed entirely.

### 9. Consistent Error Handling
**File:** `engine/engine.go:115`

Subdirectory scanning silently swallows errors (`_ = checkDir(subDir)`). At minimum, these should be logged. Consider returning a warning slice alongside the projects.

### 10. Define a `Context` Interface for the Executor
**File:** `lib/executor.go`

The `Executor` directly depends on `os/exec` and `syscall`. For testability, consider extracting a `ProcessRunner` interface so the orchestration layer can be tested with mock executors without actually spawning processes.
