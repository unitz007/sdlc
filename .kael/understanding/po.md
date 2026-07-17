# Product Owner Onboarding: sdlc

## What This Product Does

**sdlc** is a lightweight, unified CLI tool that provides a consistent interface for running, testing, building, installing, and cleaning software projects — regardless of the underlying language or build tool. Instead of remembering `go run .`, `npm start`, `mvn spring-boot:run`, or `swift run`, a developer types `sdlc run` and the tool auto-detects the project type and executes the correct command.

The tool is written in Go (1.20+), uses `cobra` for CLI parsing, and is structured into four packages:
- **`cmd/`** — CLI commands (`run`, `test`, `build`, `install`, `clean`), global flags, watch loop, interactive module selection, and output prefixing.
- **`engine/`** — Recursive project detection via `filepath.WalkDir`, matching build files (e.g., `go.mod`, `package.json`, `pom.xml`, `Package.swift`) against a task registry.
- **`config/`** — Two-layer configuration: `.sdlc.json` (project type → command mappings) and `.sdlc.conf` (per-scope env vars and extra flags), with local-override-global merging.
- **`lib/`** — Core types (`Task` struct with Run/Test/Build/Install/Clean fields) and `Executor` (wraps `os/exec` with process group management and SIGTERM-based graceful shutdown).

## Target Users & Personas

### Persona 1: "Polyglot Developer" (Primary)
Works across Go backends, Node.js/React frontends, and possibly Java or Swift services. Manages a monorepo with multiple sub-projects. Constantly context-switches between `go test ./...`, `npm test`, `mvn test`, etc. Wants **one mental model** for all lifecycle commands.

### Persona 2: "Monorepo Maintainer"
Owns a repository with `backend/` (Go), `frontend/` (Node.js), and `mobile/` (Swift). Needs to run all services concurrently during development, filter by module, and see color-coded logs per module.

### Persona 3: "Team Lead / DevOps"
Wants a standardized `sdlc run`, `sdlc test`, `sdlc build` that works identically on every developer's machine and in CI, reducing onboarding friction for new team members.

## Core Value Proposition

**One command to rule them all.** `sdlc` eliminates the cognitive overhead of remembering language-specific build tool commands. A developer who joins a polyglot monorepo only needs to learn five verbs: `run`, `test`, `build`, `install`, `clean`. The tool auto-detects what to do.

Secondary value: **Zero-config monorepo support.** Drop `sdlc` into any repo and it discovers modules by scanning for known build files. No Makefile, no task runner config, no scripts — it just works.

## Key Features (observed)

### 1. Auto-Detection of Project Types (`engine/engine.go`)
Recursively walks the working directory, skipping `.git`, `node_modules`, `vendor`, `dist`, `build`, `target`, `bin`, `pkg`, `.vscode`, `.zed`, `.kael_index`, and all dot-directories. Matches files against the task registry from `.sdlc.json`. Enforces one project per directory to avoid duplicates. Resolves symlinks and tracks seen directories.

### 2. Five Lifecycle Commands (`cmd/commands.go`)
`run`, `test`, `build`, `install`, `clean` — each maps to the corresponding field in the `lib.Task` struct. All share the same execution pipeline: detect → filter → (optionally prompt) → execute.

### 3. Multi-Module Concurrent Execution (`cmd/commands.go:244-266`)
When multiple projects are detected, they run concurrently via goroutines with a `sync.WaitGroup`. Each module's output is prefixed and color-coded using a custom `PrefixWriter` (`cmd/commands.go:512-547`).

### 4. Module Filtering (`cmd/commands.go:549-594`)
- `--module <path>` / `-m`: Run a single specific module.
- `--ignore <path>` / `-i`: Exclude modules (supports multiple flags).
- `--all` / `-a`: Explicitly run all detected modules.

### 5. Interactive Module Selection (`cmd/commands.go:596-703`)
When multiple modules are detected and no filter flags are set, `sdlc` presents a `promptui`-based interactive checklist. Users toggle modules on/off and press "Done" to execute. Unselected modules are added to the ignore list for display purposes.

### 6. Watch Mode (`cmd/commands.go:269-389`)
`--watch` / `-w` enables a polling loop (500ms ticker) that monitors file changes using `filepath.Walk`. On change, the affected module is cancelled (via context) and restarted with a 500ms grace period. Skips `.log`, `.tmp`, `.lock`, `.pid`, `.swp` files and common artifact directories.

### 7. Two-Layer Configuration
- **`.sdlc.json`** (project type definitions): Maps build file names to command templates. Loaded from `~/.sdlc.json` (global, auto-created if missing), then overridden by local `.sdlc.json` in the working directory or any subdirectory during detection.
- **`.sdlc.conf`** (env vars & flags): `KEY=VALUE` lines become environment variables; `--flag=value` lines become extra CLI arguments. Supports comments (`#`), blank lines, quoted values, and empty values. Module-level `.sdlc.conf` overrides root-level env vars; args are appended (not replaced).

### 8. Environment Variable Substitution (`cmd/commands.go:446-458`)
Command strings support `$VAR` and `${VAR}` substitution from the merged env config. Keys are sorted by length (longest first) to prevent partial matches.

### 9. Dry-Run Mode (`cmd/commands.go:197-230`)
`--dry-run` / `-n` prints what commands would be executed without running them, including env var substitution.

### 10. Graceful Process Management (`lib/executor.go`)
Uses `syscall.SysProcAttr{Setpgid: true}` to create a new process group. On context cancellation, sends `SIGTERM` to the entire group (not just the parent process), ensuring child processes are cleaned up.

### 11. Vite Temp Cleanup Hack (`cmd/commands.go:408-416`)
Before running a module, removes `node_modules/.vite-temp` to prevent EPERM errors on Windows during restart — a pragmatic workaround for a known Vite issue.

## Product Gaps & Opportunities

### High-Impact Gaps

1. **No `lint` or `format` commands.** Modern development workflows rely heavily on `gofmt`, `eslint --fix`, `prettier`, `swiftformat`, etc. Adding `sdlc lint` and `sdlc format` as first-class lifecycle actions would significantly increase daily utility.

2. **No `.gitignore`-aware file watching.** The watch mode (`hasChanges` in `commands.go:469-509`) manually skips dot-directories and a hardcoded list of artifact dirs, but does not read `.gitignore`. This means custom-ignored paths (e.g., `coverage/`, `*.generated.go`) will trigger unnecessary restarts.

3. **No smart/partial restart in watch mode.** The README mentions "smart partial restarts coming soon." Currently, any file change in a module restarts that entire module. For large monorepos, restarting only the affected service (or running only affected tests) would be a major productivity win.

4. **No shell quoting support in command parsing.** `lib/executor.go:25-27` splits commands on spaces (`strings.Split(command, " ")`). Commands with quoted arguments like `go build -ldflags "-s -w"` or paths with spaces will break. This is a correctness bug affecting real-world usage.

5. **No `--shell` or shell-invocation mode.** The executor runs the binary directly (not via `sh -c`), which means shell features like pipes (`|`), redirects (`>`), and `&&` are not available in command definitions.

### Medium-Impact Opportunities

6. **No version command (`sdlc version` or `sdlc --version`).** Standard for any CLI tool. Essential for debugging and CI reproducibility.

7. **No shell completion support.** Cobra natively supports bash/zsh/fish completions. Adding this would improve UX significantly.

8. **No `sdlc init` command.** Users must manually create `.sdlc.json`. An `init` command that scaffolds the config with common project types would lower the barrier to entry.

9. **No exit code aggregation for multi-module.** When running multiple modules concurrently, if one fails, the others continue but the overall exit code may not reflect the failure (the current code returns `nil` from `runTask` after `wg.Wait()` regardless of individual goroutine errors).

10. **No logging/verbosity control.** There's no `--verbose` or `--quiet` flag. The tool always prints the banner and detection info, which is noisy in CI pipelines.

11. **No Windows support validation.** The Vite temp cleanup hack suggests Windows usage, but the tool uses Unix-specific signals (`syscall.SIGTERM`, process groups via `Setpgid`). Windows compatibility is uncertain.

### Low-Impact / Nice-to-Have

12. **No plugin/extension system.** Users cannot add custom lifecycle actions beyond the hardcoded five. A plugin system or at least user-defined aliases in `.sdlc.json` would increase extensibility.

13. **No telemetry or usage analytics.** No way to understand which project types are most used, which commands are most popular, or where users encounter errors.

14. **No CI/CD integration guide.** The README focuses on local development. A guide for using `sdlc` in GitHub Actions, GitLab CI, etc. would broaden adoption.

## Suggested Priorities

### P0 — Must Fix (Correctness & Reliability)
1. **Fix command parsing to handle shell quoting** (lib/executor.go). Use `sh -c` invocation or a proper shell-word parser. This is a correctness bug that will bite users with paths containing spaces or complex flags.
2. **Aggregate exit codes in multi-module execution** (cmd/commands.go). The tool should return a non-zero exit code if any module fails.

### P1 — High Value (Adoption & Daily Use)
3. **Add `.gitignore`-aware watch mode.** Read `.gitignore` files and respect them during file change detection. This prevents spurious restarts.
4. **Add `sdlc version` command.** Trivial to implement with cobra, high value for CI and debugging.
5. **Add `sdlc lint` and `sdlc format` commands.** These are the most frequently used developer workflows after run/test/build.
6. **Add `--quiet` flag.** Suppress banner and detection output for CI/pipe-friendly usage.

### P2 — Growth (Reach & Ecosystem)
7. **Add `sdlc init` scaffolding command.** Lowers barrier to entry for new users.
8. **Add shell completion support.** Cobra makes this straightforward.
9. **Smart partial restarts in watch mode.** Differentiate between "source file changed" (restart) and "test file changed" (re-run tests only).
10. **CI/CD integration documentation and examples.**

### P3 — Future
11. **Plugin/extension system for custom lifecycle actions.**
12. **Cross-platform (Windows) testing and validation.**
13. **Homebrew / apt / scoop distribution packages.**

## Risks & Unknowns

1. **Command injection via env var substitution.** The `$VAR` / `${VAR}` substitution in `cmd/commands.go:446-458` does simple string replacement. If an env value contains shell metacharacters and the command is eventually run via a shell, this could lead to injection. Currently safe because commands are run directly (not via shell), but this changes if `sh -c` is adopted (which is needed for pipes/redirects).

2. **Watch mode polling performance.** The 500ms polling interval with `filepath.Walk` on every tick could be expensive on large monorepos with many files. A proper file system watcher (e.g., `fsnotify`) would be more efficient, though the current approach is simpler and cross-platform.

3. **No integration tests.** All tests are unit tests (task parsing, config parsing, project detection with temp dirs). There are no end-to-end tests that actually run `sdlc run` or `sdlc test` against real projects. This means the core user-facing flow is untested.

4. **Interactive selection UX in non-TTY environments.** The `promptModuleSelection` function (commands.go:596) assumes a terminal is available. In CI or piped environments, this will fail or hang. The code has a comment acknowledging this ("assume terminal is available if we are here") but no fallback.

5. **Config file auto-creation side effect.** `config.Load("")` (config.go:119) auto-creates `~/.sdlc.json` if it doesn't exist. This is a surprising side effect for a read operation and could cause issues in read-only environments or containerized builds.

6. **No semantic versioning or release process.** The project has no version tags, no release workflow, and no `goreleaser` or similar tooling. This makes distribution and dependency management difficult.

7. **Single maintainer risk.** The `.planner/issues.md` shows all issues assigned to `AI_AGENT`. There's no indication of a human maintainer, community, or contribution guidelines beyond the brief "Pull requests welcome" in the README.
