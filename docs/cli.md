# CLI Guide

## Overview

The `sdlc` CLI provides a unified interface for common development tasks across multiple languages and project types.

## Commands

- `sdlc run` – Run the application.
- `sdlc test` – Run tests.
- `sdlc build` – Build the project.
- `sdlc install` – Install dependencies.
- `sdlc clean` – Clean build artifacts.

## Flags

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--watch` | `-w` | Watch for file changes.
| `--module` | `-m` | Target a specific module (path).
| `--ignore` | `-i` | Ignore specific modules.
| `--config` | `-c` | Path to custom config directory.
| `--dir` | `-d` | Specify working directory.
| `--extra-args` | `-e` | Pass extra args to underlying tool.
| `--dry-run` | `-n` | Show commands without executing.

## Examples

```bash
# Run with watch mode
sdlc run --watch

# Build a specific module
sdlc build --module backend

# Test all modules except frontend
sdlc test --ignore frontend
```
