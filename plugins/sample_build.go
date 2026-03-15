package plugins

import (
    "fmt"
    "sdlc/engine"
)

// SampleBuildPlugin demonstrates a custom build stage.
// It simply prefixes the existing build command with "echo Building:" for illustration.
type SampleBuildPlugin struct{}

func (p SampleBuildPlugin) Command(proj engine.Project) (string, error) {
    if proj.Task.Build == "" {
        return "", fmt.Errorf("no build command defined for project %s", proj.Path)
    }
    // Example: prepend an echo for demonstration purposes.
    return fmt.Sprintf("echo Building %s && %s", proj.Path, proj.Task.Build), nil
}

func init() {
    // Register the plugin for the "build" stage.
    Register("build", SampleBuildPlugin{})
}
