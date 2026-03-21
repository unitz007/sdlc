# Developer Onboarding: sdlc

## Technical Stack

- **Language**: Go 1.20+
- **CLI Framework**: [spf13/cobra](https://github.com/spf13/cobra) v1.8.1 — command routing and flag parsing
- **Interactive Prompts**: [manifoldco/promptui](https://github.com/manifoldco/promptui) v0.9.0 — multi-module selection UI
- **Standard Library**: `os/exec`, `os/signal`, `syscall`, `filepath.WalkDir`, `encoding/json`, `context`
- **No external build/lint tools** — vanilla `go build`, `go test`, `go vet`
- **Total codebase**: ~1,860 lines of Go across 12 files (including tests)

## Repository Structure

```
.
├── main.go                  # Entry point — calls cmd.Execute()
├── go.mod / go.sum          # Module definition (module name: "sdlc")
├── .sdlc.json               # Project's own config (defines go.mod, package.json, pom.xml, Package.swift)
├── .gitignore               # Ignores compiled binary "sdlc"
├── cmd/                     # CLI layer — cobra commands, orchestration, watch mode
│   ├── root.go              # Root command definition, global flags, resolveWorkDir()
│   ├── commands.go          # Subcommands (run/test/build/install/clean), executeTask(), watchAndRunLoop(), PrefixWriter
│   └── executor.go          # Thin wrapper: cmd.runCommand() → lib.NewExecutor()
├── engine/                  # Project detection engine
│   ├── engine.go            # DetectProjects() — recursive filesystem walk, build-file matching
│   └── engine_test.go       # 6 tests: single/multi/nested modules, skip dot-dirs, skip node_modules
├── config/                  # Configuration loading and parsing
│   ├── config.go            # .sdlc.json loader, .sdlc.conf parser, env merging
│   └── config_test.go       # 8 tests: env parsing, quoted values, merge behavior, nil inputs
└── lib/                     # Core types and process execution
    ├── task.go              # Task struct (Run/Test/Build/Install/Clean) + Command() method
    ├── task_test.go         # 6 tests: valid actions, invalid/empty fields, empty task
    ├── executor.go          # Executor — wraps os/exec.Cmd with process group, SIGTERM on cancel
    └── executor_test.go     # 5 tests: construction, arg parsing, success/failure execution
```

## Entry Points & Main Flow

1. **`main.go`** → `cmd.Execute()` → cobra dispatches to the matching subcommand.
2. **`cmd/commands.go:executeTask()`** is the central orchestrator for all actions:
   - Resolves working directory (supports `~` expansion)
   - Creates a signal-aware `context.Context` (SIGINT/SIGTERM)
   - Loads task definitions from config (local `.sdlc.json` → global `~/.sdlc.json`)
   - Calls `engine.DetectProjects()` to scan the filesystem for known build files
   - Loads `.sdlc.conf` for env vars and extra flags
   - Filters projects by `--module`, `--ignore`, `--all` flags
   - If multiple projects and no explicit selection → interactive `promptui` picker
   - **Watch mode** (`--watch`): enters `watchAndRunLoop()` — polls every 500ms via `filepath.Walk`, restarts changed modules
   - **Normal mode**: launches all selected projects concurrently via goroutines + `sync.WaitGroup`
3. **`cmd/executor.go:runCommand()`** bridges to `lib.NewExecutor()` which creates an `os/exec.Cmd` with process group isolation (`Setpgid: true`) and a custom `Cancel` function that sends SIGTERM to the process group.

**Key data flow**: `.sdlc.json` → `map[string]lib.Task` → `engine.DetectProjects()` → `[]engine.Project` → `lib.Task.Command(action)` → `lib.Executor.Execute()`

## External Dependencies & Integrations

| Dependency | Version | Purpose |
|---|---|---|
| `github.com/spf13/cobra` | v1.8.1 | CLI command routing, flag parsing |
| `github.com/manifoldco/promptui` | v0.9.0 | Interactive module selection prompt |
| `github.com/spf13/pflag` | v1.0.5 | Flag parsing (transitive via cobra) |
| `github.com/chzyer/readline` | v0.0.0-2018… | Line editing (transitive via promptui) |
| `golang.org/x/sys` | v0.0.0-2018… | Syscall wrappers (transitive via promptui) |

**No database, network, or cloud integrations.** The tool is entirely local — it shells out to build tools (go, npm, mvn, swift) found on the user's PATH.

## Build / Test / Deploy

- **Build**: `go build -o sdlc .` or `go install .`
- **Test**: `go test ./...` — all 3 packages have tests (25 tests total), all pass
- **Lint**: `go vet ./...` — clean, no issues
- **Install**: Clone repo, `go install .`, ensure `$(go env GOPATH)/bin` is in PATH
- **No CI/CD configuration** found in the repository (no `.github/`, `Makefile`, `Dockerfile`, etc.)
- **No release tooling** — binary is built locally

## Code Quality Notes

**Strengths:**
- Clean separation of concerns: `lib` (types + execution), `config` (parsing), `engine` (detection), `cmd` (orchestration)
- Good test coverage for `lib/`, `config/`, and `engine/` — 25 tests covering core logic
- Proper process group management in `lib/executor.go` (`Setpgid: true`, SIGTERM to process group on cancel)
- Context-aware cancellation throughout the execution pipeline
- Deterministic project ordering via `sort.Slice` in `DetectProjects()`
- Symlink resolution and deduplication in directory walking

**Areas for improvement:**
- `cmd/commands.go` is 714 lines — the largest file by far, handling commands, watch mode, filtering, prompting, prefix writing, and banner printing. It could be decomposed.
- No tests for `cmd/` package — the orchestration logic (filtering, watch loop, env substitution, dry-run) is untested
- The `Executor` splits commands on spaces (`strings.Split(command, " ")`) which breaks with quoted arguments or paths containing spaces
- Environment variable substitution in commands (`$VAR` / `${VAR}`) is done via naive `strings.ReplaceAll` — could match unintended substrings (e.g., `$PORT` inside `$PORTFOLIO`)
- The watch mode uses polling (500ms ticker + `filepath.Walk`) rather than OS-native file watchers (e.g., `fsnotify`) — this is less efficient for large projects
- The `.sdlc.conf` parser treats lines starting with `-` as flags (`--flag=value`), but the README example shows bare flags (`--debug`, `--verbose`) without `=`, which would be silently skipped

## Known Pain Points & Technical Debt

1. **Command splitting is fragile** (`lib/executor.go:25-27`): `strings.Split(command, " ")` cannot handle quoted arguments, escaped spaces, or complex shell syntax. Commands like `go run -ldflags "-X main.version=1.0" .` will break.

2. **No shell interpretation**: Commands are executed directly via `exec.Command`, not through a shell (`sh -c`). This means shell features like pipes (`|`), redirections (`>`), `&&`, glob patterns, and environment variable expansion are not available unless the user explicitly wraps in `sh -c`.

3. **Watch mode restarts ALL modules on any change** (`cmd/commands.go:382-385`): The README mentions "smart partial restarts coming soon" but currently any file change in any module triggers a restart of all modules. The `hasChanges()` function identifies which file changed but the restart logic doesn't use this to selectively restart only the affected module.

4. **Hardcoded Vite cleanup** (`cmd/commands.go:408-416`): There's a special-case cleanup of `node_modules/.vite-temp` before each run, which is a workaround for a specific Vite EPERM issue. This should be configurable or generalized.

5. **Color handling is not terminal-aware**: ANSI color codes are always emitted regardless of whether stdout is a TTY. Piping output to a file or another command will include escape sequences.

6. **No `.gitignore`-aware file watching**: The `hasChanges()` function has its own hardcoded skip list (`.git`, `node_modules`, `dist`, etc.) rather than reading `.gitignore`. This can diverge from what the user expects.

7. **`promptModuleSelection` mutates global state** (`cmd/commands.go:694`): Unselected modules are appended to the global `ignoreMods` slice as a side effect, which is fragile and makes the function harder to test.

8. **No versioning or help for config schema**: There's no schema validation for `.sdlc.json` — invalid or missing fields are silently accepted, and missing actions (e.g., no `install` command defined) will produce empty command strings that fail at execution time.
