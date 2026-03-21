# Product Owner Onboarding: sdlc

## What This Product Does

**sdlc** is a lightweight, unified CLI tool that provides a consistent interface for running, testing, building, installing, and cleaning software projects — regardless of the underlying language, framework, or build tool. Instead of remembering `go run .`, `npm start`, `mvn spring-boot:run`, or `swift run`, a developer types `sdlc run` and the tool figures out the right command.

The tool is written in Go (1.20+), uses `cobra` for CLI parsing and `promptui` for interactive prompts, and is distributed as a single binary via `go install`. It targets developers working in terminal environments, particularly those managing **monorepos** with mixed-language modules (e.g., a Go backend + a Node.js frontend in the same repo).

The core workflow is:
1. **Detect** — scan the working directory (and immediate subdirectories) for known build files (`go.mod`, `package.json`, `pom.xml`, `Package.swift`, etc.) defined in `.sdlc.json`.
2. **Select** — if multiple modules are found, either auto-run all, filter via `--module`/`--ignore` flags, or interactively prompt the user to pick modules.
3. **Execute** — run the mapped shell command concurrently across selected modules, with color-coded, prefixed output for clarity.
4. **Watch** (optional) — poll for file changes every 500ms and restart affected modules.

## Target Users & Personas

### Persona 1: "Polyglot Developer" (Primary)
- Works across 2–4 languages/frameworks daily (Go backend, Node/React frontend, maybe a Java microservice).
- Manages a monorepo with multiple subdirectories, each a different project type.
- Pain: constantly context-switches between `go test ./...`, `npm test`, `mvn test`, etc.
- Wants: one command (`sdlc test`) that "just works" everywhere.

### Persona 2: "Team Lead / DevOps"
- Wants consistent onboarding: new team members shouldn't need to learn project-specific build commands.
- Values the `.sdlc.json` and `.sdlc.conf` files as project-level documentation of how to build/run things.
- Pain: onboarding docs get stale; build commands drift from README.

### Persona 3: "Solo Hacker / Side-Project Developer"
- Jumps between many small repos and forgets the exact command for each.
- Wants zero-config auto-detection with sensible defaults.
- Values the `--watch` flag for rapid dev loops.

## Core Value Proposition

**One CLI, every project.** sdlc eliminates the cognitive overhead of remembering per-project build commands by auto-detecting project types and mapping them to a uniform `run | test | build | install | clean` interface. For monorepo teams, it provides concurrent multi-module execution with color-coded output and interactive module selection — replacing shell scripts, `Makefile` targets, or manual terminal multiplexing.

## Key Features (observed)

### 1. Auto-Detection Engine (`engine/engine.go`)
- Scans the working directory and its **immediate subdirectories** for known build files.
- Skips `.git`, `.idea`, `.planner`, `node_modules` directories.
- Enforces one project per directory (prevents duplicates when both `go.mod` and `package.json` exist in the same folder).
- Merges local `.sdlc.json` overrides with global/home config.

### 2. Five Lifecycle Commands (`cmd/commands.go`)
- `sdlc run` — runs the application (e.g., `go run .`, `npm start`).
- `sdlc test` — runs tests.
- `sdlc build` — compiles/builds.
- `sdlc install` — installs dependencies.
- `sdlc clean` — removes build artifacts.
- All commands share the same execution pipeline via `executeTask()`.

### 3. Multi-Module Support
- Concurrent execution via goroutines with `sync.WaitGroup`.
- `--module <path>` to target a single module.
- `--ignore <path>` (repeatable) to exclude modules.
- `--all` to explicitly run all detected modules.
- Color-coded, prefixed output via `PrefixWriter` (`cmd/commands.go` lines 524–559) — each module gets a distinct color (cyan, green, magenta, yellow, blue).

### 4. Interactive Module Selection (`cmd/commands.go` lines 608–715)
- When multiple modules are detected and no flags are set, a `promptui.Select`-based interactive UI appears.
- Users toggle modules on/off with a checklist-style loop, then confirm with "[Done] Run selected modules".
- Status: **in_progress** per `.planner/features.md` (FEAT-005).

### 5. Watch Mode (`cmd/commands.go` lines 269–389)
- `--watch` / `-w` flag enables polling-based file watching (500ms interval).
- Uses `filepath.Walk` to detect changes, skipping hidden dirs, `node_modules`, `dist`, `build`, `target`, `bin`, `pkg`, and temp files (`.log`, `.tmp`, `.lock`, `.pid`, `.swp`).
- On change: cancels the running module's context (sends SIGTERM to process group), waits for graceful shutdown (5s timeout), then restarts.
- Currently restarts **all** modules on any change; README notes "smart partial restarts coming soon."

### 6. Configuration System (`config/config.go`)
- **`.sdlc.json`** — maps build file names to Task definitions (run/test/build/install/clean commands). Loaded from: project root → home directory (with auto-creation of empty file).
- **`.sdlc.conf`** — per-module env vars and extra flags. Lines starting with `$` are env vars (`$PORT=8080`), lines starting with `-` are flags (`--debug`).
- Config cascade: root `.sdlc.conf` → module `.sdlc.conf` → CLI `--extra-args`.
- Environment variable substitution in commands: `$VAR` and `${VAR}` patterns are replaced.

### 7. Dry-Run Mode
- `--dry-run` / `-n` flag prints what commands would execute (with env var substitution applied) without actually running them.

### 8. Process Management (`lib/executor.go`)
- Creates a new process group (`Setpgid: true`) for proper signal handling.
- On context cancellation, sends SIGTERM to the entire process group (not just the parent).
- Streams stdout/stderr in real-time.

### 9. Built-in Project Types (`.sdlc.json` in repo root)
- Go (`go.mod`), Node.js (`package.json`), Java/Maven (`pom.xml`), Swift (`Package.swift`).

## Product Gaps & Opportunities

### High-Impact Gaps

1. **No `.gitignore`-aware file watching** — The `hasChanges()` function skips dot-directories and hardcoded build dirs but does **not** read `.gitignore`. This means watched files may include compiled outputs in non-standard directories, or miss files that should be watched. This is a correctness and performance issue for real-world repos.

2. **Polling-based watch (500ms) instead of native FS events** — The current implementation walks the entire directory tree every 500ms. For large monorepos this is expensive. Using `fsnotify` (a well-established Go library) would provide instant, event-driven file watching with dramatically lower CPU usage.

3. **No smart/partial restart in watch mode** — The README explicitly calls this out as "coming soon." Currently, a file change in the frontend module restarts the backend too. This is the #1 friction point for monorepo users.

4. **Shallow directory scanning (one level only)** — `DetectProjects()` only checks the root and immediate subdirectories. Nested monorepos (e.g., `apps/web/frontend/`) won't be detected. This limits applicability to flat or two-level repo structures.

5. **No `lint` or `format` commands** — Modern dev workflows heavily rely on `eslint`, `golangci-lint`, `prettier`, `gofmt`, etc. Adding `sdlc lint` and `sdlc fmt` as lifecycle actions would significantly increase the tool's daily value.

### Medium-Impact Opportunities

6. **No shell-aware command parsing** — `lib/executor.go` splits commands on spaces (`strings.Split(command, " ")`). This breaks commands with quoted arguments, e.g., `go run -ldflags "-X main.version=1.0"`. Should use `sh -c` or a proper shell parser.

7. **No `deploy` or `ci` lifecycle action** — The `Task.Command()` method returns `"invalid command"` for anything beyond the five hardcoded actions. Making the action set extensible (or at least adding `deploy`/`ci`) would future-proof the tool.

8. **No version command** — There's no `sdlc version` or `sdlc --version` output. Basic for any CLI tool.

9. **No shell completion** — Cobra supports bash/zsh/fish completions out of the box, but no completion command is registered.

10. **No test coverage or integration tests** — Only unit tests exist for `lib/executor.go` and `lib/task.go`. The `engine`, `config`, and `cmd` packages have zero test coverage. The interactive selection, watch mode, and multi-module execution paths are completely untested.

### Low-Impact / Nice-to-Have

11. **No logging framework** — Uses `fmt.Printf` throughout. A structured logger (e.g., `slog` in Go 1.21+) would enable `--verbose`/`--quiet` modes and JSON log output for CI.

12. **No configuration validation** — Invalid `.sdlc.json` or `.sdlc.conf` files produce generic error messages. A `sdlc validate` command or schema validation would improve DX.

13. **No `init` command** — A `sdlc init` that scaffolds `.sdlc.json` and `.sdlc.conf` in the current project would lower the barrier to adoption.

## Suggested Priorities

### P0 — Must Fix (Correctness & Reliability)
| Item | Description |
|------|-------------|
| Shell-aware command parsing | Fix `strings.Split` to handle quoted args — this silently breaks real-world commands |
| `.gitignore`-aware watch | Prevent false-positive restarts and unnecessary filesystem scanning |
| Replace polling with `fsnotify` | Eliminate 500ms latency and CPU waste in watch mode |

### P1 — High Value (Adoption & Usability)
| Item | Description |
|------|-------------|
| Smart partial restart | Only restart the module whose files changed — the #1 monorepo pain point |
| Add `lint` / `fmt` actions | Covers the most common dev workflow beyond run/test/build |
| `sdlc version` command | Basic CLI hygiene |
| Deep/nested module detection | Support real-world monorepo layouts (3+ levels deep) |

### P2 — Growth (Ecosystem & DX)
| Item | Description |
|------|-------------|
| `sdlc init` scaffolding | Reduce friction for first-time users |
| Shell completions | Leverage cobra's built-in support |
| Extensible action set | Allow custom actions beyond the hardcoded five |
| Structured logging | Enable `--verbose`/`--quiet` and CI-friendly output |

### P3 — Technical Debt
| Item | Description |
|------|-------------|
| Test coverage for `engine`, `config`, `cmd` | Currently 0% — high risk for regressions |
| Extract `hasChanges` watch logic | Should be its own package with proper abstraction |
| Remove Vite-specific cleanup hack | `runProject()` has hardcoded `.vite-temp` cleanup (line 421) — should be configurable |

## Risks & Unknowns

1. **Command injection via env var substitution** — The `runProject()` function performs naive string replacement of `$VAR` and `${VAR}` in command strings. If an env value contains shell metacharacters (e.g., `; rm -rf /`), it will be interpreted by the shell. This is a **security concern** for CI environments where `.sdlc.conf` may be committed to the repo.

2. **Windows compatibility** — The code uses `syscall.SIGTERM`, `syscall.Kill` with process groups, and Unix-style path handling. No Windows build tags or conditional logic exist. The tool likely does not work on Windows.

3. **No release/distribution strategy** — The README only mentions `go install`. There are no GitHub Releases, Homebrew formula, or pre-built binaries. For a CLI tool targeting developers, distribution friction is a real adoption barrier.

4. **Single maintainer risk** — The repo appears to be a solo project (unitz007). All issues are assigned to `AI_AGENT`. There's no CONTRIBUTING.md guide, no CI/CD pipeline, and no issue templates.

5. **Config file auto-creation side effect** — `config.Load("")` creates an empty `.sdlc.json` in the user's home directory if it doesn't exist (line 151 of `config.go`). This is a surprising side effect for a read operation and could fail in read-only environments.

6. **Watch mode restarts all modules** — As noted, this is a known limitation. But it also means that a change in a large module (e.g., a `node_modules` reinstall that wasn't filtered) could cause all modules to restart, leading to significant downtime during development.

7. **No graceful degradation for missing tools** — If `npm` isn't installed but a `package.json` is detected, the tool will fail with a raw OS error. A friendlier message ("npm not found — is Node.js installed?") would improve UX.
