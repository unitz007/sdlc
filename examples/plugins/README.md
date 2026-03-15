# Sample Plugin Repository

This `examples/plugins` directory demonstrates how to create custom plugins for **SDLC**.

Each sub‑directory represents a plugin that provides custom lifecycle commands for a specific type of project.

## Directory Layout

```
examples/plugins/
│
├─ lint/
│   └─ .sdlc.json   # Example: code linting plugin (runs golint)
├─ test/
│   └─ .sdlc.json   # Example: test execution plugin (npm based)
└─ deploy/
    └─ .sdlc.json   # Example: generic Go deployment plugin
```

### How to use these plugins

1. **Copy a plugin into your project** – place the desired plugin directory somewhere inside your repository (for example, `./plugins/lint`).
2. **SDLC will automatically detect it** – the engine scans the working directory and its immediate sub‑directories for a `.sdlc.json` file. Any directory containing such a file becomes a module with its own set of commands.
3. Run SDLC as usual:
   ```bash
   sdlc run        # runs the `run` command of each detected plugin/module
   sdlc test       # runs the `test` command defined in the plugin
   sdlc build       # etc.
   ```

You can also combine multiple plugins in a monorepo; SDLC will prefix the output of each module with a coloured label so you can easily distinguish the logs.

## Plugin Details

### Lint Plugin (`lint/.sdlc.json`)
```json
{
  "*": {
    "run": "golint ./...",
    "test": "",
    "build": "",
    "install": "",
    "clean": ""
  }
}
```
Runs `golint` across the whole repository.

### Test Plugin (`test/.sdlc.json`)
```json
{
  "package.json": {
    "run": "npm start",
    "test": "npm test",
    "build": "npm run build",
    "install": "npm install",
    "clean": "rm -rf node_modules"
  }
}
```
Provides typical Node.js commands.

### Deploy Plugin (`deploy/.sdlc.json`)
```json
{
  "*.go": {
    "run": "go run .",
    "test": "go test ./...",
    "build": "go build -o app",
    "install": "go mod download",
    "clean": "go clean"
  }
}
```
Standard Go lifecycle commands.

## Registering Plugins Globally (optional)
If you want these plugins to be available for all projects without copying them each time, place the plugin directories under a common location (e.g., `~/.sdlc/plugins`) and add the path to your `SDLC_PLUGIN_PATH` environment variable. SDLC will search the directories listed in this variable for additional `.sdlc.json` files.

---

Feel free to experiment by creating your own plugins—just add a `.sdlc.json` file with the commands you need and SDLC will pick it up automatically!
