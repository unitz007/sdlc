# Developer Onboarding: sdlc

## Technical Stack

- **Language**: Go 1.20+
- **CLI Framework**: [spf13/cobra](https://github.com/spf13/cobra) v1.8.1 — command routing and flag parsing
- **Interactive Prompts**: [manifoldco/promptui](https://github.com/manifoldco/promptui) v0.9.0 — multi-module selection UI
- **No external build/deploy tooling** — pure `go build` / `go install`
- **Total codebase**: ~1,390 lines of Go across 10 source files

## Repository Structure

```
.
├── main.go                  # Entry point — calls cmd.Execute()
├── go.mod / go.sum          # Module definition (module name: "sdlc")
├── .sdlc.json               # Project's own config (defines go.mod, package.json, pom.xml, Package.swift)
├── .gitignore               # Ignores the compiled binary ("sdlc")
├── cmd/
│   ├── root.go              # Root cobra command, global flags (--watch, --module, --dir, etc.), resolveWorkDir()
│   ├── commands.go          # Subcommands (run/test/build/install/clean), execution orchestration, watch mode, PrefixWriter, interactive module selection
│   └── executor.go          # Thin wrapper: cmd.runCommand() → lib.NewExecutor()
├── engine/
│   └── engine.go            # Project detection: scans root + immediate subdirs for known build files, returns []Project
├── config/
│   └── config.go            # Config loading: .sdlc.json (task definitions), .sdlc.conf (env vars + flags), local vs global merge
├── lib/
│   ├── task.go              # Task struct (Run/Test/Build/Install/Clean) with Command() accessor
│   ├── executor.go          # Executor: wraps os/exec.Cmd with process group management, SIGTERM on cancel
│   ├── task_test.go         # Unit tests for Task.Command() — passing
│   └── executor_test.go     # Unit tests for Executor — BROKEN (see Known Pain Points)
└── .planner/                # Project planning artifacts (features.md, issues.md, sprints.md, project_purpose.md)
```

## Entry Points & Main Flow

1. **`main.go`** → `cmd.Execute()` → cobra dispatches to subcommand
2. **Subcommand handler** (e.g., `runCmd.RunE`) → `executeTask(cmd, "run")`
3. **`executeTask`** (`cmd/commands.go:97`):
   - Resolves working directory (supports `~` expansion)
   - Creates signal-aware context (SIGINT/SIGTERM)
   - Calls `runTask(ctx, wd, action)`
4. **`runTask`** (`cmd/commands.go:112`):
   - Loads config: `config.Load(cfgFile)` or `config.LoadLocal(wd)` with fallback to `config.Load("")` (home dir)
   - Detects projects: `engine.DetectProjects(wd, tasks)` — scans root + 1 level of subdirectories
   - Loads root `.sdlc.conf` for env/flags
   - Filters projects via `filterProjects()` (respects `--module`, `--ignore`, `--all`)
   - If multiple projects and no explicit selection → interactive `promptModuleSelection()`
   - If `--dry-run` → prints commands without executing
   - If `--watch` → enters `watchAndRunLoop()` (polling every 500ms, restarts on file change)
   - Otherwise → runs all selected projects concurrently via goroutines + `sync.WaitGroup`
5. **`runProject`** (`cmd/commands.go:419`):
   - Cleans up `.vite-temp` if present (workaround for Vite EPERM errors)
   - Resolves command string via `p.Task.Command(action)`
   - Merges env vars (root `.sdlc.conf` → module `.sdlc.conf` → CLI `--extra-args`)
   - Performs `$VAR` / `${VAR}` substitution in command string
   - Delegates to `cmd.runCommand()` → `lib.NewExecutor(ctx, cmdStr)` → `executor.Execute()`

**Key data flow**: `.sdlc.json` → `map[string]lib.Task` → `engine.DetectProjects()` → `[]engine.Project` → execution

## External Dependencies & Integrations

| Dependency | Version | Purpose |
|---|---|---|
| `github.com/spf13/cobra` | v1.8.1 | CLI command routing, flag parsing |
| `github.com/spf13/pflag` | v1.0.5 | Flag handling (transitive via cobra) |
| `github.com/manifoldco/promptui` | v0.9.0 | Interactive module selection prompt |
| `github.com/chzyer/readline` | — | Line editing (transitive via promptui) |
| `golang.org/x/sys` | — | Syscall wrappers (transitive via promptui) |

**No database, network, or cloud dependencies.** The tool is entirely local — it shells out to build tools (go, npm, mvn, swift, etc.) found on the user's PATH.

**Configuration files consumed:**
- `.sdlc.json` — JSON mapping build-file names (e.g., `go.mod`) to task command strings
- `.sdlc.conf` — Key-value env vars (`$KEY=VALUE`) and CLI flags (`--flag=value`), parsed line-by-line

## Build / Test / Deploy

### Build
```bash
go build -o sdlc .        # Produces binary named "sdlc"
go install .              # Installs to $GOPATH/bin
```

### Test
```bash
go test ./...             # Currently FAILS — see Known Pain Points
```

### Deploy
No CI/CD pipeline detected. The `.gitignore` only contains the compiled `sdlc` binary. Installation is manual via `go install .` from source.

## Code Quality Notes

**Strengths:**
- Clean separation of concerns: `lib` (core types/execution), `config` (file I/O), `engine` (detection), `cmd` (orchestration/UI)
- Proper process group management in `lib/executor.go` — uses `Setpgid: true` and sends SIGTERM to the entire process group on cancellation, preventing orphan processes
- Signal handling via `signal.NotifyContext` for graceful shutdown
- `PrefixWriter` in `cmd/commands.go` provides color-coded, line-prefixed output for multi-module scenarios
- Config layering: global `~/.sdlc.json` → local `.sdlc.json` → module `.sdlc.conf` → CLI flags

**Weaknesses:**
- **Tests are broken**: `lib/executor_test.go` calls `NewExecutor(string)` but the signature was changed to `NewExecutor(context.Context, string)` — all 5 test functions fail to compile
- **No tests for `cmd/`, `config/`, or `engine/` packages** — the orchestration, config loading, and project detection logic are completely untested
- **Command splitting is naive**: `lib/executor.go:25` splits on spaces (`strings.Split(command, " ")`), which breaks for commands with quoted arguments or paths containing spaces
- **Watch mode uses polling** (500ms interval via `filepath.Walk`) rather than OS-native file watchers (e.g., `fsnotify`) — this is CPU-inefficient for large projects
- **Environment variable substitution** (`cmd/commands.go:458-470`) is done via simple string replacement, which can cause unintended substitutions (e.g., `$HOME` inside a longer word)
- **Hardcoded Vite workaround** (`cmd/commands.go:421-428`) — cleans `node_modules/.vite-temp` on every run, which is a domain-specific hack in generic tooling
- **`promptModuleSelection`** mutates the global `ignoreMods` slice as a side effect (line 706), which is fragile and non-obvious

## Known Pain Points & Technical Debt

1. **Broken test suite** (`lib/executor_test.go`): The `NewExecutor` function signature was changed from `(string)` to `(context.Context, string)` but the tests were not updated. All 5 executor tests fail to compile. This must be fixed before any CI can run.

2. **Zero test coverage for core logic**: The `config`, `engine`, and `cmd` packages have no tests at all. The project detection, config loading/merging, filtering, watch loop, and env substitution logic are all untested.

3. **Naive command parsing**: `strings.Split(command, " ")` in `lib/executor.go:25-27` does not handle quoted arguments, escaped spaces, or shell metacharacters. Should use `sh -c` or a proper shell-word parser.

4. **Polling-based file watcher**: The watch mode in `cmd/commands.go:269-389` walks the entire directory tree every 500ms. For large monorepos this will be slow and CPU-intensive. Should integrate `github.com/fsnotify/fsnotify` for event-driven watching.

5. **Single-level directory scanning**: `engine.DetectProjects` only checks the root and immediate subdirectories. Deeply nested projects (e.g., `apps/api/backend/go.mod`) are not detected.

6. **No `.gitignore` awareness in project detection**: The engine hardcodes a skip list (`.git`, `.idea`, `.planner`, `node_modules`) rather than reading `.gitignore`, so custom-ignored directories will still be scanned.

7. **Global mutable state**: `cmd/commands.go` uses package-level `var` for all CLI flags (`workDir`, `extraArgs`, `targetMod`, etc.) and `promptModuleSelection` mutates `ignoreMods` as a side effect. This makes the code harder to test and reason about.

8. **No versioning or release mechanism**: No `Makefile`, no goreleaser config, no semantic versioning. The binary name is just `sdlc` with no version flag.
