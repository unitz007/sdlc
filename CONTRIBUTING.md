# Contributing to SDLC

Thank you for your interest in contributing to **SDLC**! This guide will help you get started.

## Getting Started

### Prerequisites

- **Go 1.20+** installed — [Install Go](https://go.dev/dl/)
- **Git** for version control
- A GitHub account

### Fork, Clone, and Branch

1. **Fork** the repository on GitHub: click the "Fork" button at the top of [unitz007/sdlc](https://github.com/unitz007/sdlc).
2. **Clone** your fork locally:

   ```bash
   git clone https://github.com/<your-username>/sdlc.git
   cd sdlc
   ```

3. **Add the upstream remote** so you can keep your fork in sync:

   ```bash
   git remote add upstream https://github.com/unitz007/sdlc.git
   ```

4. **Create a branch** for your change. Use a descriptive name following this convention:

   | Type | Branch Name Pattern | Example |
   |------|---------------------|---------|
   | Feature | `feature/<short-description>` | `feature/add-fsnotify-watch` |
   | Bug fix | `fix/<short-description>` | `fix/quoted-args-splitting` |
   | Documentation | `docs/<short-description>` | `docs/update-readme` |
   | Refactor | `refactor/<short-description>` | `refactor/executor-context` |

   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Workflow

### Building

Build the project to verify it compiles:

```bash
go build -o sdlc .
```

Or install it directly to your `GOPATH/bin`:

```bash
go install .
```

### Running Tests

Run the full test suite before submitting a PR:

```bash
go test ./...
```

To see verbose output:

```bash
go test -v ./...
```

### Linting

There is currently no linter configured in the project. However, you should ensure your code:

- Passes `go vet`:

  ```bash
  go vet ./...
  ```

- Is formatted with `gofmt`:

  ```bash
  gofmt -d .
  ```

- Has no obvious issues checked by `go build`:

  ```bash
  go build ./...
  ```

### Making Changes

1. Make your changes on your feature branch.
2. Add or update tests to cover your changes.
3. Run the full test suite and vet checks to confirm everything passes.
4. Commit your changes following the [commit message guidelines](#commit-message-guidelines) below.

## Commit Message Guidelines

We follow the **Conventional Commits** style. Each commit message should have the format:

```
<type>(<scope>): <short description>

<optional longer description>
```

### Types

| Type | Description |
|------|-------------|
| `feat` | A new feature |
| `fix` | A bug fix |
| `docs` | Documentation changes only |
| `refactor` | Code changes that neither fix a bug nor add a feature |
| `test` | Adding or updating tests |
| `chore` | Maintenance tasks (dependencies, tooling, etc.) |

### Examples

```
feat(watch): add fsnotify-based file watching
fix(executor): handle quoted arguments in command splitting
docs(readme): add multi-module usage examples
test(executor): fix broken tests after context refactor
```

## Running CI Locally

There is currently no CI pipeline configured for this project. To replicate what a CI run would check, execute the following commands locally:

```bash
# 1. Verify the project compiles
go build ./...

# 2. Run static analysis
go vet ./...

# 3. Check formatting
gofmt -d .

# 4. Run all tests
go test ./...
```

All of these should complete without errors before you submit a pull request.

## Submitting a Pull Request

1. **Push your branch** to your fork:

   ```bash
   git push origin feature/your-feature-name
   ```

2. **Open a Pull Request** on GitHub against the `main` branch of the upstream repository.

3. **Fill in the PR description** with:
   - A clear summary of the change.
   - The motivation or context (link to any related issue).
   - How you tested the change.

4. **Request a review** by mentioning a maintainer or using the GitHub review request feature.

5. **Address feedback** — maintainers may request changes. Push additional commits to your branch; the PR will update automatically.

6. **Keep your branch up to date** — if the upstream `main` branch has changed, rebase your branch:

   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

## Reporting Bugs or Suggestions

### Bug Reports

If you find a bug, please [open an issue](https://github.com/unitz007/sdlc/issues) with the following information:

- **Description** — what happened and what you expected to happen.
- **Steps to reproduce** — minimal commands or configuration to trigger the bug.
- **Environment** — OS, Go version, and any relevant project details.
- **Logs** — any error output or stack traces.

### Feature Suggestions

We welcome ideas for improvements! When opening a feature request, please include:

- **Problem** — what problem does this solve or what capability is missing?
- **Proposed solution** — your idea for how it could work (the more detail, the better).
- **Alternatives considered** — any other approaches you thought about.

Thank you for helping make SDLC better!
