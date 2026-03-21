# Developer Onboarding: sdlc

## Technical Stack

- **Language**: Go 1.20+
- **CLI Framework**: [spf13/cobra](https://github.com/spf13/cobra) v1.8.1 — command routing and flag parsing
- **Interactive Prompts**: [manifoldco/promptui](https://github.com/manifoldco/promptui) v0.9.0 — multi-module selection UI
- **No external build/deploy tooling** — standard `go build` / `go install`
- **No database, no network services** — purely a local CLI wrapper

## Repository Structure

```
.
├── main.go              # Entry point — calls cmd.Execute()
├── go.mod / go.sum      # Module definition (module name: "sdlc")
├── .sdlc.json           # Project's own task definitions (used for self-development)
├── cmd/
│   ├── root.go          # Root cobra command, global flags (--watch, --module, --dir, etc.)
│   ├── commands.go      # Subcommands (run/test/build/install/clean), execution orchestration,
│   │                    # watch mode loop, PrefixWriter, project filtering, interactive selection
│   └── executor.go      # Thin wrapper bridging cmd → lib.Executor
├── engine/
│   └── engine.go        # Project detection — scans working dir + immediate subdirs for build files
├── config/
│   └── config.go        # Config loading: .sdlc.json (task maps) and .sdlc.conf (env vars + flags)
└── lib/
    ├── task.go          # Task struct (Run/Test/Build/Install/Clean) + Command() accessor
    ├── executor.go      # Executor — wraps os/exec.Cmd with process group, SIGTERM on cancel
    ├── task_test.go     # Unit tests for Task.Command() — passing
    └── executor_test.go # Unit tests for Executor — BROKEN (see Known Pain Points)
```

## Entry Points & Main Flow

1. **`main.go`** → `cmd.Execute()` → cobra dispatches to subcommand
2. **`cmd/commands.go:executeTask()`** — the central orchestrator:
   - Resolves working directory (supports `~` expansion)
   - Creates a signal-aware context (SIGINT/SIGTERM)
   - Loads config: local `.sdlc.json` → fallback to `~/.sdlc.json`
   - Calls `engine.DetectProjects()` to find build files in cwd + immediate subdirs
   - Loads `.sdlc.conf` for env vars and extra flags
   - Filters projects by `--module`, `--ignore`, `--all` flags
   - If multiple projects and no flags: interactive `promptui` selection
   - **Watch mode** (`--watch`): enters `watchAndRunLoop()` — polls every 500ms, restarts modules on file change
   - **Normal mode**: launches all selected projects concurrently via goroutines + `sync.WaitGroup`
3. **`cmd/executor.go:runCommand()`** → creates `lib.Executor`, sets dir/output/env, calls `Execute()`
4. **`lib/executor.go`** → `exec.CommandContext` with process group (`Setpgid: true`), custom `Cancel` sends SIGTERM to process group

**Config resolution order** (for task definitions):
1. `--config` flag directory → `.sdlc.json`
2. Working directory → `.sdlc.json` (local override)
3. `~/.sdlc.json` (global fallback)
4. Local config is merged on top of global (local wins for duplicate keys)

**Env/flag resolution** (`.sdlc.conf`):
1. Root `.sdlc.conf` loaded first
2. Per-module `.sdlc.conf` merged on top (module wins)
3. CLI `--extra-args` appended last

## External Dependencies & Integrations

| Dependency | Version | Purpose |
|---|---|---|
| `github.com/spf13/cobra` | v1.8.1 | CLI command routing and flag parsing |
| `github.com/spf13/pflag` | v1.0.5 | Flag parsing (transitive via cobra) |
| `github.com/manifoldco/promptui` | v0.9.0 | Interactive module selection prompt |
| `github.com/chzyer/readline` | v0.0.0-20180603132655 | Line editing (transitive via promptui) |
| `golang.org/x/sys` | v0.0.0-20181122145206 | Syscall support (transitive) |

**No other integrations.** The tool shells out to user-configured commands (go, npm, mvn, swift, etc.) but has no direct dependency on any of them.

## Build / Test / Deploy

- **Build**: `go build -o sdlc .` or `go install .`
- **Test**: `go test ./...`
- **No CI/CD configuration** found (no `.github/`, `Makefile`, `Dockerfile`, etc.)
- **No linter configuration** (no `.golangci.yml`, no `golangci-lint` in go.mod)
- **No release tooling** — manual `go install` from source

## Code Quality Notes

### Test Coverage
- **`lib/task_test.go`** — 5 tests, all passing. Good coverage of `Task.Command()`.
- **`lib/executor_test.go`** — 5 tests, **all broken** (see Pain Points).
- **`config/`** — zero test files.
- **`engine/`** — zero test files.
- **`cmd/`** — zero test files (hardest to test due to cobra + os.Exec, but no integration tests either).

### Architecture
- Clean separation of concerns: `lib` (core types), `config` (file I/O), `engine` (detection), `cmd` (orchestration).
- `lib.Executor` is well-designed with process group management for clean shutdown.
- `PrefixWriter` in `cmd/commands.go` is a solid implementation for multi-module log interleaving.

### Documentation
- README is thorough with usage examples, command reference, and config format.
- Code comments are present but sparse — many functions lack doc comments (especially in `cmd/commands.go`).

## Known Pain Points & Technical Debt

### 1. Broken Tests (Critical)
`lib/executor_test.go` calls `NewExecutor("echo")` with one argument, but the function signature was changed to `NewExecutor(ctx context.Context, command string)`. All 5 tests fail to compile. This was likely introduced when context support was added to the executor.

### 2. Naive Command Splitting
`lib/executor.go:25-27` splits commands on spaces: `strings.Split(command, " ")`. This breaks for commands with quoted arguments (e.g., `go run -ldflags "-s -w" .`) or paths with spaces. Should use `sh -c` or a proper shell-word parser.

### 3. Polling-Based Watch Mode
`cmd/commands.go:333` uses a 500ms `time.Ticker` with `filepath.Walk` for change detection. This is CPU-intensive for large projects and slow to detect changes. Should use `fsnotify` for efficient inotify/kqueue-based watching.

### 4. Hardcoded Vite Cleanup
`cmd/commands.go:421-428` has a hardcoded cleanup of `node_modules/.vite-temp` to work around EPERM errors on Windows. This is a workaround for a specific tool's bug and shouldn't be in a general-purpose CLI.

### 5. `.sdlc.conf` Format Inconsistency
The README shows env vars as `PORT=8080` (no `$` prefix), but `config/config.go:50` expects `$KEY=VALUE` (with `$` prefix). The documentation and implementation disagree.

### 6. No Recursive Module Detection
`engine/engine.go:111-117` only scans the working directory and its **immediate** subdirectories. Nested monorepo structures (e.g., `apps/backend/api/`) won't be detected.

### 7. No `.gitignore` Awareness in Detection
The engine skips `.git`, `.idea`, `.planner`, and `node_modules` by hardcoded name checks. It doesn't actually parse `.gitignore`, so custom-ignored directories will still be scanned.

### 8. Global Mutable State in `cmd` Package
`cmd/root.go` uses package-level `var` for all flags (`workDir`, `extraArgs`, `targetMod`, etc.). `cmd/commands.go:promptModuleSelection()` also mutates the global `ignoreMods` slice. This makes testing and reasoning about state difficult.

### 9. No Error Aggregation in Multi-Module Mode
When running multiple modules concurrently, individual module errors are printed to stderr but not collected. The `runTask()` function returns `nil` even if some modules fail, because goroutines don't propagate errors back.

### 10. Stale Transitive Dependency
`golang.org/x/sys v0.0.0-20181122145206` (from Nov 2018) is very old. A `go get -u` would modernize the dependency tree.
