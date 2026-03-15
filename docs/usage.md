# Usage

## Running a project

```bash
sdlc run
```

The tool automatically detects the project type (Go, Node.js, etc.) and runs the appropriate command.

## Testing a project

```bash
sdlc test
```

## Building a project

```bash
sdlc build
```

## Additional flags

- `--watch` – Watch for file changes and restart automatically.
- `--module <path>` – Target a specific module in a monorepo.
- `--ignore <path>` – Exclude a module from execution.
- `--config <path>` – Load a custom configuration directory.
