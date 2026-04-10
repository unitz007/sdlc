// Package plugin provides a hook-based plugin system for extending SDLC with
// user-defined project types, lifecycle commands, and custom tooling.
//
// # Quick Start
//
// Register hooks programmatically:
//
//	registry := plugin.NewRegistry()
//	registry.Register(plugin.Hook{
//	    Name:    "pre-build",
//	    Command: "npm run lint",
//	    ProjectType: "node",
//	})
//
// Or load plugins from JSON files:
//
//	registry := plugin.NewRegistry()
//	err := registry.LoadFile(".sdlc/plugins.json")
//
// Execute hooks at any lifecycle point:
//
//	output, err := registry.Run(ctx, "pre-build", plugin.RunOpts{
//	    Dir:   "/path/to/project",
//	    Stdout: os.Stdout,
//	})
package plugin

// Plugin represents a single plugin that may define multiple hooks.
type Plugin struct {
	// Name is a human-readable name for the plugin (e.g. "my-linter").
	Name string `json:"name"`

	// ProjectType is an optional project-type selector (e.g. "node", "go").
	// If non-empty, the plugin's hooks only fire for projects of this type.
	ProjectType string `json:"project_type,omitempty"`

	// Hooks is the list of lifecycle hooks contributed by this plugin.
	Hooks []Hook `json:"hooks"`
}
