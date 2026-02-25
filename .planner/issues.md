# Issues

## Backlog

### Bug Fixes
- [x] **ISSUE-001**
  - **Title**: Fix tilde expansion in working directory path
  - **Type**: bug
  - **Priority**: P1
  - **Effort**: S
  - **Description**: The tilde expansion logic in `main.go` uses `strings.ReplaceAll` but discards the result. This means paths starting with `~/` will not work correctly.
  - **Acceptance Criteria**:
    - [ ] Passing a path with `~/` to `--dir` correctly expands to the user's home directory.
  - **Dependencies**: none

- [x] **ISSUE-010**
  - **Title**: Fix multi-module detection duplication
  - **Type**: bug
  - **Priority**: P0
  - **Effort**: S
  - **Description**: Multi-module projects can be detected multiple times if multiple build files exist in the same directory (e.g. go.mod and package.json), leading to duplicate execution.
  - **Acceptance Criteria**:
    - [x] Enforce one project per directory
    - [x] Prevent duplicate execution of the same module
  - **Dependencies**: none

### Refactoring
- [x] **ISSUE-002**
  - **Title**: Refactor main function into smaller components
  - **Type**: refactor
  - **Priority**: P1
  - **Effort**: M
  - **Description**: The `main` function is getting large and contains logic for CLI parsing, configuration loading, and command execution. This should be split into separate functions or packages for better testability and maintainability.
  - **Acceptance Criteria**:
    - [ ] CLI logic is separated from business logic.
    - [ ] Configuration loading is isolated.
    - [ ] Command execution logic is isolated.
  - **Dependencies**: none

### Features
- [x] **ISSUE-003**
  - **Title**: Detect multiple modules in subdirectories
  - **Type**: feature
  - **Priority**: P1
  - **Effort**: M
  - **Description**: Extend the detection logic to scan subdirectories for build files, not just the root directory. This is the first step for multi-module support.
  - **Acceptance Criteria**:
    - [ ] Tool identifies `go.mod`, `pom.xml`, etc., in immediate subdirectories.
    - [ ] Returns a list of all detected modules/paths.
  - **Dependencies**: ISSUE-002 (Refactoring makes this easier)

- [x] **ISSUE-004**
  - **Title**: Multi-module execution flags
  - **Type**: feature
  - **Priority**: P2
  - **Effort**: M
  - **Description**: Add CLI flags to control which module(s) to run.
  - **Acceptance Criteria**:
    - [ ] `--module <name>` or `--path <path>` runs a specific module.
    - [ ] `--all` runs the command for all detected modules.
  - **Dependencies**: ISSUE-003

- [x] **ISSUE-005**
  - **Title**: Concurrent execution for 'run' command
  - **Type**: feature
  - **Priority**: P2
  - **Effort**: L
  - **Description**: When running multiple modules (e.g., backend and frontend), they should run in parallel/concurrently so one doesn't block the other.
  - **Acceptance Criteria**:
    - [ ] `sdlc run --all` starts processes concurrently.
    - [ ] Output from both processes is streamed (prefixed or managed).
    - [ ] Ctrl+C stops all processes.
  - **Dependencies**: ISSUE-004

- [x] **ISSUE-006**
  - **Title**: Watch mode for automatic restart
  - **Type**: feature
  - **Priority**: P2
  - **Effort**: M
  - **Description**: Add a `--watch` flag that monitors file changes in the project directory and restarts the `run` command or re-runs the `test` command automatically.
  - **Acceptance Criteria**:
    - [ ] `sdlc run --watch` restarts the process when files change.
    - [ ] `sdlc test --watch` re-runs tests when files change.
    - [ ] Debounce changes to prevent rapid restarts.
    - [ ] Gracefully kill the previous process before restarting.
  - **Dependencies**: ISSUE-002 (Refactoring makes this easier)

- [x] **ISSUE-007**
  - **Title**: Module-specific configuration
  - **Type**: feature
  - **Priority**: P2
  - **Effort**: M
  - **Description**: Allow `.sdlc.json` in module directories to override global settings.
  - **Acceptance Criteria**:
    - [x] Local config overrides global config
    - [x] Supports `run`, `test`, `build` commands
    - [x] Works with multi-module detection

- [x] **ISSUE-008**
  - **Title**: Multi-module argument passing (Revised)
  - **Type**: feature
  - **Priority**: P1
  - **Effort**: M
  - **Description**: Allow `.sdlc.conf` file in each module directory to define environment variables and flags for that module. Root `.sdlc.conf` provides global defaults.
  - **Acceptance Criteria**:
    - [x] Loads `.sdlc.conf` (text format: `$VAR=VAL`, `--flag=val`) from root and module directories
    - [x] Applies global env vars and args from root config
    - [x] Applies module-specific env vars and args from module config (overriding root)
    - [x] Supports environment variable substitution in commands

- [ ] **ISSUE-009**
  - **Title**: Interactive module selection
  - **Type**: feature
  - **Priority**: P2
  - **Effort**: M
  - **Description**: When running a command without specific flags in a multi-module project, prompt the user to select which module(s) to run.
  - **Acceptance Criteria**:
    - [ ] Uses a TUI library (e.g., bubbletea or promptui) for selection
    - [ ] Allows multi-selection
    - [ ] Defaults to "all" or remembers last selection? (Start with simple selection)
  - **Dependencies**: none
