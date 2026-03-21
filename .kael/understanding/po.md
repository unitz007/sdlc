# Product Owner Onboarding: sdlc

## What This Product Does

**SDLC** is a lightweight, unified CLI tool that provides a consistent interface for running, testing, building, installing, and cleaning software projects — regardless of the underlying language, framework, or build tool. Instead of remembering `go run .`, `npm start`, `mvn spring-boot:run`, or `swift run`, a developer types `sdlc run` and the tool figures out the right command.

The tool auto-detects project types by scanning for well-known build files (`go.mod`, `package.json`, `pom.xml`, `Package.swift`, etc.) defined in a configuration file (`.sdlc.json`). It supports **monorepos** by detecting multiple modules across subdirectories and running them concurrently with color-coded, prefixed output. A **watch mode** (`--watch`) monitors file changes and restarts affected modules automatically.

The project is written in Go (1.20+), uses `cobra` for CLI parsing, `promptui` for interactive module selection, and is licensed under Apache 2.0. The repository lives at `github.com/unitz007/sdlc`.

## Target Users & Personas

1. **Polyglot Developer ("Alex")** — Works across Go backends, Node.js/React frontends, and possibly Java or Swift services. Constantly context-switches between `go test ./...`, `npm test`, `mvn test`, etc. Wants a single mental model: `sdlc test`.

2. **Monorepo Maintainer ("Jordan")** — Manages a repository with a Go API server in `backend/` and a Node.js frontend in `frontend/`. Needs to run both simultaneously during development, see interleaved logs clearly, and selectively run/test individual modules.

3. **DevEx / Platform Engineer ("Sam")** — Wants to standardize developer workflows across teams. Can define a shared `.sdlc.json` with organization-specific build commands and distribute it, reducing onboarding friction for new team members.

4. **Open-Source Contributor** — Looking for a simple, dependency-light tool (only 2 direct Go dependencies) that they can extend with custom project types.

## Core Value Proposition

**One command, any project.** SDLC eliminates the cognitive overhead of remembering language-specific build tool commands. The value compounds in monorepo environments where multiple languages coexist: a single `sdlc run --watch` replaces separate terminal tabs, separate watch scripts, and separate `Makefile` targets.

Key differentiators from alternatives (Make, Taskfile, Just, Nx):
- **Zero-config for common project types** — ships with sensible defaults for Go, Node.js, Maven, and Swift out of the box.
- **Automatic project detection** — no `Makefile` or `Taskfile.yml` needed; just run `sdlc run` in any directory.
- **Built-in monorepo awareness** — detects modules in subdirectories automatically, runs them concurrently, and provides interactive selection.
- **Per-module environment and flag injection** via `.sdlc.conf` files, enabling module-specific configuration without polluting the global config.

## Key Features (observed)

### 1. Five Lifecycle Commands (`cmd/commands.go`)
- `sdlc run` — runs the application (e.g., `go run .`, `npm start`)
- `sdlc test` — runs the test suite
- `sdlc build` — compiles/builds the project
- `sdlc install` — installs dependencies
- `sdlc clean` — removes build artifacts

### 2. Auto-Detection Engine (`engine/engine.go`)
- Scans the working directory and immediate subdirectories for known build files.
- Resolves symlinks and deduplicates (one project per directory).
- Merges local `.sdlc.json` overrides with global/home config.
- Skips `.git`, `.idea`, `.planner`, `node_modules` directories during scanning.

### 3. Multi-Module Support (`cmd/commands.go` — `filterProjects`, `promptModuleSelection`)
- Concurrent execution of multiple modules via goroutines.
- `--module <path>` / `-m` to target a single module.
- `--ignore <path>` / `-i` to exclude specific modules (supports multiple flags).
- `--all` / `-a` to explicitly run all modules.
- **Interactive selection** via `promptui`: when multiple modules are detected and no flags are set, users get a toggle-based checklist to pick which modules to run. Unselected modules are marked `[IGNORED]` in the output.
- Color-coded log prefixes per module (cyan, green, magenta, yellow, blue) using a custom `PrefixWriter`.

### 4. Watch Mode (`cmd/commands.go` — `watchAndRunLoop`)
- `--watch` / `-w` flag enables file change monitoring.
- Polls every 500ms using `filepath.Walk`, skipping hidden dirs, `node_modules`, `dist`, `build`, `target`, `bin`, `pkg`, and temp files (`.log`, `.tmp`, `.lock`, `.pid`, `.swp`).
- On change detection, cancels the running module's context (SIGTERM to process group), waits for graceful shutdown (500ms delay), then restarts.
- 5-second timeout for forced shutdown if modules don't stop gracefully.

### 5. Configuration System (`config/config.go`)
- **`.sdlc.json`** — maps build file names to `Task` objects with `run`, `test`, `build`, `install`, `clean` commands. Loaded from: CLI `--config` flag → project-local → `~/.sdlc.json` (auto-created if missing).
- **`.sdlc.conf`** — per-directory env vars (`$KEY=VALUE`) and extra flags (`--flag`). Module-level configs are merged on top of root-level configs. Environment variables are substituted into command strings using both `$VAR` and `${VAR}` syntax.

### 6. Process Execution (`lib/executor.go`)
- Uses `exec.CommandContext` with process group (`Setpgid: true`) for proper signal handling.
- SIGTERM sent to entire process group on cancellation (not just the parent process).
- Supports custom working directory, environment variables, and stdout/stderr writers.

### 7. Dry-Run Mode
- `--dry-run` / `-n` flag prints what commands would be executed (with env var substitution) without actually running them. Useful for debugging configuration.

### 8. Global CLI Flags (`cmd/root.go`)
- `--dir` / `-d` — absolute path to project directory (supports `~/` expansion)
- `--extra-args` / `-e` — additional arguments appended to the underlying command
- `--config` / `-c` — custom configuration directory path

### 9. Built-in Project Types (`.sdlc.json` in repo root)
- `go.mod` → `go run main.go`, `go test .`, `go build -v`
- `package.json` → `npm run dev`, `npm test`, `npm run build`
- `pom.xml` → `mvn spring-boot:run`, `mvn test`, `mvn build`
- `Package.swift` → `swift run`, `swift test`, `swift build`

## Product Gaps & Opportunities

### High Impact
1. **No `.gitignore`-aware file watching** — The watch mode (`hasChanges`) manually hardcodes directories to skip (`.git`, `node_modules`, `dist`, etc.) but does not actually read `.gitignore`. This means custom-ignored paths (e.g., `vendor/`, `*.generated.go`) will trigger unnecessary restarts. Reading `.gitignore` would make watch mode significantly more reliable.

2. **No recursive module detection** — `DetectProjects` only scans the root and **immediate** subdirectories (depth = 1). A monorepo with nested modules like `apps/api/go.mod` and `apps/web/package.json` would not be detected. Supporting configurable depth or recursive scanning would unlock deeper monorepo structures.

3. **No `lint` or `format` commands** — Modern development workflows heavily rely on `golangci-lint run`, `eslint`, `prettier`, etc. Adding `sdlc lint` and `sdlc format` as first-class lifecycle actions would significantly increase the tool's daily utility.

4. **No shell-aware command parsing** — `lib/executor.go` splits commands on spaces (`strings.Split(command, " ")`), which breaks commands with quoted arguments (e.g., `go run -ldflags "-X main.version=1.0"`). This is a correctness bug that will surface with real-world configurations.

### Medium Impact
5. **No version command or self-update mechanism** — Users cannot check `sdlc --version` or update the tool easily. A `version` subcommand and optionally a self-update flow would improve distribution.

6. **No shell completion support** — Cobra natively supports shell completions (bash, zsh, fish, PowerShell). Adding `sdlc completion` would be a low-effort, high-value UX improvement.

7. **No exit code aggregation for multi-module** — When running multiple modules concurrently, the tool waits for all to finish but doesn't aggregate or report which modules failed. A summary table at the end (✅/❌ per module) would be valuable for CI-like usage.

8. **No CI-friendly mode** — The tool is designed for interactive use (banner, colors, promptui). A `--no-color` / `--ci` flag and non-interactive fallback (auto-run-all when multiple modules detected) would enable use in GitHub Actions, GitLab CI, etc.

### Low Impact / Nice-to-Have
9. **No `init` command** — A `sdlc init` command that scaffolds a `.sdlc.json` with detected project types would lower the barrier for first-time users.

10. **No plugin/extension system** — Currently, adding a new project type requires editing `.sdlc.json` manually. A registry of community-maintained project type definitions (e.g., `sdlc add-type python --build-file pyproject.toml`) could grow the ecosystem.

11. **Vite-specific cleanup hack** — `runProject` in `commands.go` has a hardcoded cleanup of `node_modules/.vite-temp` (lines 421-428). This is a workaround that should be generalized into a configurable pre-run hook or cleanup step.

## Suggested Priorities

### P0 — Fix Correctness Issues
| Item | Description |
|------|-------------|
| Shell-aware command parsing | Fix `strings.Split(command, " ")` in `lib/executor.go` to handle quoted arguments. Use `sh -c` or a proper shell parser. |
| `.gitignore`-aware watch mode | Read `.gitignore` in `hasChanges()` instead of hardcoding skip directories. |

### P1 — Expand Core Value
| Item | Description |
|------|-------------|
| `lint` and `format` commands | Add to `Task` struct and CLI. High daily-use value. |
| Recursive module detection | Support configurable scan depth for nested monorepos. |
| CI-friendly mode | Add `--no-color` flag and non-interactive auto-all behavior. |
| Exit code aggregation | Report per-module success/failure in multi-module runs. |

### P2 — Improve Distribution & DX
| Item | Description |
|------|-------------|
| `sdlc version` command | Trivial to add with Cobra. |
| Shell completions | `sdlc completion <shell>` via Cobra's built-in support. |
| `sdlc init` command | Scaffold `.sdlc.json` from detected project types. |
| Pre/post-run hooks | Generalize the Vite cleanup hack into configurable hooks. |

### P3 — Ecosystem Growth
| Item | Description |
|------|-------------|
| Plugin/extension system | Community-maintained project type definitions. |
| Self-update mechanism | `sdlc update` to pull latest release. |

## Risks & Unknowns

1. **Command splitting correctness** — The naive space-splitting in `lib/executor.go` is the single biggest correctness risk. Any command with quoted strings, environment variable expansions in arguments, or special characters will break silently or produce wrong behavior. This should be addressed before any wider adoption.

2. **Watch mode performance on large repos** — The current polling approach (`filepath.Walk` every 500ms) does not scale. On a large monorepo with thousands of files, this will consume significant CPU. A proper file system watcher (`fsnotify`) would be more efficient, though it introduces platform-specific complexity.

3. **No integration tests** — Only unit tests exist for `lib/executor.go` and `lib/task.go`. There are no tests for the critical paths: project detection, multi-module filtering, watch mode, config merging, or the interactive prompt. The `cmd/` and `engine/` packages have zero test coverage.

4. **Interactive mode in CI/pipe** — If `sdlc run` is invoked in a non-interactive context (CI pipeline, piped stdout) with multiple modules detected and no flags, the `promptui` interactive selector will fail or hang. There is no fallback to auto-run-all in non-TTY environments.

5. **Config auto-creation side effect** — `config.Load("")` auto-creates `~/.sdlc.json` if it doesn't exist (line 151 of `config.go`). This is a surprising side effect for a "read" operation and could cause issues in read-only filesystems or containerized environments.

6. **Environment variable substitution security** — The `$VAR` and `${VAR}` substitution in command strings (lines 466-470 of `commands.go`) is done via simple string replacement. If an env value contains shell metacharacters, it could lead to command injection. Values are not escaped before substitution.

7. **Single-language project scope** — The tool currently only supports 4 project types (Go, Node.js, Maven, Swift). Python (`pyproject.toml`, `requirements.txt`), Rust (`Cargo.toml`), Ruby (`Gemfile`), and .NET (`*.csproj`) are conspicuous absences for a tool claiming to be a "unified" interface.

8. **No telemetry or error reporting** — There is no way to know how the tool is being used, what project types are most common, or what errors users encounter. For an open-source tool aiming for adoption, even anonymous usage stats would help prioritize features.
