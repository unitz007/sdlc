# Features

## Implemented
- [x] **Auto-detection**: Identifies project type by scanning for build files (e.g., `pom.xml`, `go.mod`, `Package.swift`).
- [x] **Unified Commands**: `run`, `test`, `build` commands that map to underlying toolchain commands.
- [x] **Custom Working Directory**: `--dir` flag to specify the project directory.
- [x] **Extra Arguments**: `--extraArgs` flag to pass additional arguments to the build tool.
- [x] **Configuration**: `.sdlc.json` configuration file to define custom project types and commands.
- [x] **Multi-module Support**: Detect and manage multiple projects within a single repository (monorepo).
- [x] **Live Reload**: Watch for file changes and automatically restart the application or re-run tests.
- [x] **Module-specific Configuration**: Support `.sdlc.json` in module directories to override global settings.
- [x] **Multi-module Argument Passing**: Support `.sdlc.conf` file in root and module directories to define environment variables and flags.

## Planned
- [ ] **Interactive Selection**: Prompt user to select which module to run if multiple are detected.
