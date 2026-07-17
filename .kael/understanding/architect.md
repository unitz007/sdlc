# Architect Onboarding: sdlc

## High-Level Architecture

`sdlc` is a lightweight, single-binary Go CLI tool that provides a **unified interface** for software development lifecycle commands (`run`, `test`, `build`, `install`, `clean`) across heterogeneous project types. It auto-detects project types by scanning for well-known build files (`go.mod`, `package.json`, `pom.xml`, `Package.swift`, etc.) and executes the appropriate underlying toolchain command.

The architecture follows a clean layered pattern with four packages:

```
┌─────────────────────────────────────────────────────┐
│                    main.go                          │
│                  (entry point)                       │
└──────────────────────┬──────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────┐
│              cmd/  (CLI & Orchestration)             │
│  root.go ─ commands.go ─ executor.go                │
│  [cobra commands, flags, watch loop,                │
│   interactive selection, output prefixing]           │
└──────┬───────────────┬──────────────┬───────────────┘
       │               │              │
┌──────▼──────┐ ┌──────▼──────┐ ┌────▼───────────────┐
│  engine/    │ │  config/    │ │  lib/               │
│  engine.go  │ │  config.go  │ │  task.go            │
│  [project   │ │  [.sdlc.json│ │  executor.go        │
│   detection]│ │   & .sdlc   │ │  [Task type &       │
│             │ │   .conf     │ │   process spawning] │
│             │ │   loading]  │ │                     │
└─────────────┘ └─────────────┘ └─────────────────────┘
```

**Stack**: Go 1.20+, [spf13/cobra](https://github.com/spf13/cobra) (CLI framework), [manifoldco/promptui](https://github.com/manifoldco/promptui) (interactive prompts), standard library `os/exec` (process execution).

---

## Component Responsibilities

### `main.go`
Minimal entry point — calls `cmd.Execute()` and nothing else.

### `cmd/` — CLI & Orchestration Layer

| File | Responsibility |
|------|---------------|
| `root.go` | Defines the root `sdlc` cobra command and all **global flags** (`--dir`, `--watch`, `--module`, `--ignore`, `--all`, `--extra-args`, `--config`, `--dry-run`). Contains `resolveWorkDir()` for tilde expansion and CWD resolution. |
| `commands.go` | Registers five subcommands (`run`, `test`, `build`, `install`, `clean`), each delegating to `executeTask()`. Houses the **core orchestration logic**: config loading, project detection, filtering, interactive module selection, dry-run simulation, concurrent execution via goroutines, and the **watch-and-restart loop** (`watchAndRunLoop`). Also contains `PrefixWriter` for color-coded multi-module output and `hasChanges()` for file-watching via polling. |
| `executor.go` | Thin bridge that creates a `lib.Executor`, applies directory/output/env settings, and calls `Execute()`. |

### `engine/` — Project Detection Engine

| File | Responsibility |
|------|---------------|
| `engine.go` | `DetectProjects(workDir, tasks)` recursively walks the directory tree using `filepath.WalkDir`, matching files against the configured task map. Enforces **one project per directory**, skips well-known directories (`.git`, `node_modules`, `vendor`, `dist`, etc.), resolves symlinks, and merges per-directory local `.sdlc.json` overrides with global config. Returns `[]Project` sorted by path for deterministic ordering. |

### `config/` — Configuration Management

| File | Responsibility |
|------|---------------|
| `config.go` | Two config file types: **`.sdlc.json`** (JSON task definitions mapping build-file names → `lib.Task`) and **`.sdlc.conf`** (KEY=VALUE env vars and `--flag=value` extra args). Implements a three-tier config cascade: CLI `--config` flag → local `.sdlc.json` → global `~/.sdlc.json`. `MergeEnvSettings()` performs hierarchical env/arg merging (root → module override). |

### `lib/` — Core Types & Process Execution

| File | Responsibility |
|------|---------------|
| `task.go` | `Task` struct with five string fields (`Run`, `Test`, `Build`, `Install`, `Clean`) and a `Command(field)` method that maps action names to command strings. JSON-tagged for deserialization from `.sdlc.json`. |
| `executor.go` | `Executor` wraps `os/exec.Cmd` with process-group signal handling (`Setpgid: true`, SIGTERM on cancel via `cmd.Cancel`). Provides builder-pattern setters (`SetDir`, `SetEnv`, `SetOutput`) and `Execute()` which starts and waits for the subprocess. |

---

## Data Flow

### Primary Execution Flow (e.g., `sdlc run`)

```
User runs: sdlc run --watch --ignore frontend
         │
         ▼
    ┌────────────┐
    │ cobra parses│  flags bound to package-level vars
    │  CLI args  │  (workDir, watchMode, ignoreMods, etc.)
    └─────┬──────┘
          │
          ▼
    ┌──────────────────┐
    │ executeTask()    │
    │  1. resolveWorkDir()
    │  2. signal.NotifyContext (SIGINT/SIGTERM)
    └─────┬────────────┘
          │
          ▼
    ┌──────────────────┐
    │ runTask()        │
    │  1. Load config  │──── config.LoadLocal(wd) → fallback config.Load("")
    │     (.sdlc.json) │     Returns map[string]lib.Task
    │  2. Detect       │──── engine.DetectProjects(wd, tasks)
    │     projects     │     Returns []engine.Project
    │  3. Load env     │──── config.LoadEnvConfig(wd) → .sdlc.conf
    │  4. Filter       │──── filterProjects() applies --module, --ignore, --all
    │  5. Interactive? │──── promptModuleSelection() if ambiguous
    └─────┬────────────┘
          │
     ┌────┴────┐
     │ watch?  │
     ├─YES─────┤──NO
     │         │
     ▼         ▼
 watchAndRun  Concurrent goroutines
   Loop:      per project:
  ┌──────┐    ┌──────────────────────┐
  │poll  │    │ prepareProjectEnv()  │
  │500ms │    │  merge root+module   │
  │ticker│    │  .sdlc.conf + CLI    │
  └──┬───┘    └──────────┬───────────┘
     │                   │
     │ change?           ▼
     │           ┌──────────────────┐
     └──restart──│  runProject()    │
                │  1. Task.Command()│
                │  2. Env var       │
                │     substitution  │
                │  3. runCommand()  │── lib.NewExecutor()
                │     → Execute()   │── os/exec.Cmd
                └──────────────────┘
```

### Configuration Cascade

```
Priority (highest → lowest):

1. CLI flags (--config, --extra-args, --module, --ignore)
2. Local .sdlc.json  (project root, per-directory overrides during detection)
3. Global ~/.sdlc.json (user home, auto-created if missing)
4. Built-in defaults  (none — config is required)

Environment/Args merge:
  root .sdlc.conf  ──┐
                     ├── MergeEnvSettings() ──► final env + args
  module .sdlc.conf ─┘   (module overrides root env, appends args)
                     ──┐
  CLI --extra-args   ───┘── appended last
```

---

## API Surface & Contracts

### CLI Commands

| Command | Action Key | Description |
|---------|-----------|-------------|
| `sdlc run` | `"run"` | Run the application |
| `sdlc test` | `"test"` | Run tests |
| `sdlc build` | `"build"` | Compile/build |
| `sdlc install` | `"install"` | Install dependencies |
| `sdlc clean` | `"clean"` | Remove build artifacts |

### Global Flags

| Flag | Short | Type | Default | Purpose |
|------|-------|------|---------|---------|
| `--dir` | `-d` | string | `""` (CWD) | Project directory |
| `--watch` | `-w` | bool | `false` | Enable file-watch restart |
| `--all` | `-a` | bool | `false` | Run all modules explicitly |
| `--module` | `-m` | string | `""` | Target specific module |
| `--ignore` | `-i` | []string | `[]` | Exclude modules |
| `--extra-args` | `-e` | string | `""` | Append args to commands |
| `--config` | `-c` | string | `""` | Custom config directory |
| `--dry-run` | `-n` | bool | `false` | Show commands without executing |

### Configuration File Contracts

**`.sdlc.json`** — Maps build-file names to task definitions:
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

**`.sdlc.conf`** — Environment variables and extra flags:
```properties
PORT=8080
--verbose
```

### Key Internal Interfaces

- `lib.Task.Command(field string) (string, error)` — Returns the shell command for a lifecycle action. Valid fields: `"run"`, `"test"`, `"build"`, `"install"`, `"clean"`.
- `engine.DetectProjects(workDir string, tasks map[string]lib.Task) ([]Project, error)` — Scans directory tree, returns sorted project list.
- `config.Load(confDir string) (map[string]lib.Task, error)` — Loads task config (creates file if missing).
- `config.LoadEnvConfig(dir string) (*EnvSettings, error)` — Loads `.sdlc.conf` (returns nil if absent).
- `lib.NewExecutor(ctx context.Context, command string) *Executor` — Creates process executor with process-group signal handling.

---

## Scalability & Performance Considerations

### Current Strengths
- **Concurrent module execution**: Multi-module projects run via goroutines with `sync.WaitGroup`, providing parallelism.
- **Process group isolation**: `Setpgid: true` ensures child processes are properly terminated on cancel, preventing zombie processes.
- **Deterministic ordering**: Projects sorted by path ensures consistent behavior across runs.

### Bottlenecks & Risks
1. **Polling-based file watching**: `hasChanges()` uses `filepath.Walk` every 500ms. For large monorepos with many files, this is O(n) per tick. A platform-native file watcher (e.g., `fsnotify`) would reduce CPU usage significantly.
2. **Full restart on any change**: In watch mode, *all* modules restart when any file changes in any module. The README acknowledges "smart partial restarts coming soon." This is the biggest scalability gap for multi-module setups.
3. **Linear project detection**: `DetectProjects()` walks the entire tree depth-first. For very deep or wide directory structures, this could be slow. The `skipDirs` map mitigates this for common cases.
4. **Command splitting naiveté**: `lib.NewExecutor` splits commands on spaces (`strings.Split(command, " ")`), which breaks for quoted arguments or paths with spaces. This is a correctness bug, not just a performance issue.
5. **No output buffering strategy**: `PrefixWriter` writes directly to stdout/stderr. In high-throughput scenarios (e.g., test output from many modules), interleaved writes could cause garbled output.

---

## Security Posture

### Current State
- **No command sanitization**: User-provided config (`.sdlc.json`) directly maps to shell commands executed via `os/exec`. A malicious `.sdlc.json` in a cloned repo could execute arbitrary commands. This is an **inherent design trade-off** — the tool's purpose is to run configured commands.
- **Environment variable injection**: `.sdlc.conf` values are injected into subprocess environments without validation. Values from untrusted sources could override critical env vars (e.g., `PATH`).
- **No privilege escalation**: The tool runs as the invoking user with no elevated permissions.
- **Process group cleanup**: Proper SIGTERM-based shutdown with process groups prevents orphan processes.
- **No network surface**: The tool is entirely local — no network listeners, no outbound connections.

### Recommendations
- Document the trust model: `.sdlc.json` and `.sdlc.conf` should only be sourced from trusted locations.
- Consider a `--no-local-config` flag to prevent loading untrusted local configs.
- Validate that env var keys in `.sdlc.conf` don't override sensitive system variables unless explicitly intended.

---

## Structural Improvement Suggestions

### 1. Fix Command Parsing (Correctness — High Priority)
`lib/executor.go:25` splits on spaces, breaking for commands like `go run "my app/main.go"` or paths with spaces. Use `sh -c` wrapping or a proper shell-word parser.

```go
// Current (broken for quoted args):
program := strings.Split(command, " ")[0]
cmd := exec.CommandContext(ctx, program, strings.Split(command, " ")[1:]...)

// Suggested fix:
cmd := exec.CommandContext(ctx, "sh", "-c", command)
```

### 2. Replace Polling with `fsnotify` (Performance — Medium Priority)
The 500ms polling loop in `watchAndRunLoop` is CPU-wasteful for large repos. The Go ecosystem has a mature, cross-platform library:
```
github.com/fsnotify/fsnotify
```
This would provide instant, event-driven file change detection.

### 3. Implement Partial Module Restarts (Feature — Medium Priority)
Currently, any file change restarts all modules. Track which module a changed file belongs to and only restart that module. The `hasChanges()` function already returns the changed file path — the infrastructure is partially there.

### 4. Extract Orchestration from `cmd/commands.go` (Architecture — Medium Priority)
`commands.go` is 700+ lines and mixes CLI concerns (banner printing, color codes, interactive prompts) with orchestration logic (watch loop, env merging, command substitution). Extract an `orchestrator` package:

```
cmd/          → CLI definitions, flag binding, user interaction only
orchestrator/ → runTask(), watchAndRunLoop(), prepareProjectEnv()
```

### 5. Introduce Structured Logging (Observability — Low Priority)
Replace `fmt.Printf` calls with a structured logger (e.g., `slog` from Go 1.21+ stdlib). This would enable:
- Log level control (`--verbose`, `--quiet`)
- JSON log output for CI/CD integration
- Timestamps and module context in every log line

### 6. Add Integration Tests (Quality — Medium Priority)
Current tests cover `engine.DetectProjects`, `config.ParseEnvConfig`, `lib.Task.Command`, and `lib.Executor` in isolation. Missing:
- End-to-end tests that exercise the full `executeTask()` → `runProject()` → `runCommand()` path
- Watch mode behavior tests (mock file changes, verify restart)
- Config cascade tests (local + global + CLI flag interaction)

### 7. Make `skipDirs` Configurable (Flexibility — Low Priority)
The `skipDirs` map in `engine/engine.go` is hardcoded. Allow users to add custom skip patterns via `.sdlc.json` or a dedicated ignore file, similar to `.gitignore`.

### 8. Add Shell Completion Support (UX — Low Priority)
Cobra natively supports shell completions (bash, zsh, fish, PowerShell). Adding `completion` subcommands would improve developer experience with minimal effort.

---

## File Reference Map

| Path | Lines | Role |
|------|-------|------|
| `main.go` | 8 | Entry point |
| `cmd/root.go` | 70 | Root command, global flags, workdir resolution |
| `cmd/commands.go` | 714 | Subcommands, orchestration, watch loop, interactive UI |
| `cmd/executor.go` | 25 | Bridge to lib.Executor |
| `engine/engine.go` | 147 | Project detection via directory walking |
| `engine/engine_test.go` | 165 | Detection tests (6 cases) |
| `config/config.go` | 189 | Config loading, env parsing, merging |
| `config/config_test.go` | 259 | Config tests (8 cases) |
| `lib/task.go` | 36 | Task type definition |
| `lib/task_test.go` | 100 | Task tests (5 cases) |
| `lib/executor.go` | 86 | Process execution with signal handling |
| `lib/executor_test.go` | 60 | Executor tests (5 cases) |
| `.sdlc.json` | 22 | Project's own config (go.mod, package.json, pom.xml, Package.swift) |
