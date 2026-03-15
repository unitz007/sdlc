package plugins

import (
    "sync"
    "sdlc/engine"
)

type StagePlugin interface {
    // Command returns the command string for a given project.
    // Implementations can inspect the Project to build a custom command.
    Command(p engine.Project) (string, error)
}

var (
    registry = make(map[string]StagePlugin)
    mu       sync.RWMutex
)

// Register registers a plugin for a specific stage name (e.g., "build", "test").
// It should be called from the plugin's init() function.
func Register(name string, p StagePlugin) {
    mu.Lock()
    defer mu.Unlock()
    registry[name] = p
}

// GetPlugin retrieves a registered plugin for the given stage name.
func GetPlugin(name string) (StagePlugin, bool) {
    mu.RLock()
    defer mu.RUnlock()
    p, ok := registry[name]
    return p, ok
}
