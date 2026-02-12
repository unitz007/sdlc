# SDLC

**SDLC** is a lightweight CLI tool that provides a unified interface for common software development lifecycle commands — **run**, **test**, and **build** — across different project types.

Instead of remembering the specific build commands for each language or framework, SDLC detects your project type automatically and runs the right command for you.

## Features

- 🔍 **Auto-detection** — Identifies your project type by scanning for known build files (`pom.xml`, `go.mod`, `Package.swift`, etc.)
- 🔧 **Unified commands** — Run `sdlc run`, `sdlc test`, or `sdlc build` regardless of the underlying toolchain
- 📁 **Custom working directory** — Target any project directory with the `--dir` flag
- ⚙️ **Configurable** — Define your own project types and commands via a simple JSON config file

## Installation

Build from source using [Git](https://git-scm.com) and [Go](https://go.dev) (1.20+):

```bash
git clone https://github.com/unitz007/sdlc.git
cd sdlc
go build -v
```

## Usage

```bash
# Show help
sdlc --help

# Run your project
sdlc run

# Test your project
sdlc test

# Build your project
sdlc build
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--dir` | `-d` | Absolute path to the project directory |
| `--extraArgs` | `-e` | Extra arguments to pass to the build tool |
| `--config` | `-c` | Path to directory containing the config file |

### Examples

```bash
# Run a project in a specific directory
sdlc run -d /path/to/project

# Build with extra arguments
sdlc build -e "-ldflags '-s -w'"
```

## Configuration

SDLC looks for a `.sdlc.json` configuration file in your home directory (or the path specified with `--config`). The file maps build files to their corresponding lifecycle commands:

```json
{
  "pom.xml": {
    "run": "mvn spring-boot:run",
    "test": "mvn test",
    "build": "mvn build"
  },
  "go.mod": {
    "run": "go run main.go",
    "test": "go test .",
    "build": "go build -v"
  },
  "Package.swift": {
    "run": "swift run",
    "test": "swift test",
    "build": "swift build"
  }
}
```

You can also set the config location via the `SDLC_CONFIG_LOCATION` environment variable.

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

[Apache 2.0](http://www.apache.org/licenses/LICENSE-2.0)