# API Reference

This section provides details about the Go packages that power **SDLC**.

## `config`

- `Load(path string) (map[string]lib.Task, error)`: Load a configuration file.
- `LoadLocal(dir string) (map[string]lib.Task, error)`: Load a local config from a directory.
- `LoadEnvConfig(dir string) (map[string]string, error)`: Load environment variables from `.sdlc.conf`.

## `engine`

- `DetectProjects(workDir string, tasks map[string]lib.Task) ([]Project, error)`: Detect projects in a directory.

## `lib`

- `Task` struct defines commands for `run`, `test`, `build`, `install`, `clean`.
- `Task.Command(action string) (string, error)`: Retrieve the command string.

## `cmd`

- `RootCmd` – Cobra root command.
- Subcommands: `run`, `test`, `build`, `install`, `clean`.

For deeper documentation, generate GoDoc via `go doc ./...`.
