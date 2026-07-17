package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Registry is the central store for all registered plugins and their hooks.
// It is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	plugins []Plugin
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make([]Plugin, 0),
	}
}

// Register adds a Plugin to the registry. Each hook in the plugin is
// validated before being registered; if any hook is invalid the entire
// plugin is rejected and the first validation error is returned.
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, h := range p.Hooks {
		if err := h.Validate(); err != nil {
			return fmt.Errorf("plugin %q: %w", p.Name, err)
		}
	}

	r.plugins = append(r.plugins, p)
	return nil
}

// RegisterHook adds a single hook to a plugin named pluginName. If a plugin
// with that name does not exist, a new one is created.
func (r *Registry) RegisterHook(pluginName string, h Hook) error {
	if err := h.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for i, p := range r.plugins {
		if p.Name == pluginName {
			r.plugins[i].Hooks = append(r.plugins[i].Hooks, h)
			return nil
		}
	}

	r.plugins = append(r.plugins, Plugin{
		Name:        pluginName,
		Hooks:       []Hook{h},
		ProjectType: h.ProjectType,
	})
	return nil
}

// Hooks returns all registered hooks, flattened and sorted by priority.
func (r *Registry) Hooks() []Hook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []Hook
	for _, p := range r.plugins {
		all = append(all, p.Hooks...)
	}
	return SortHooks(all)
}

// GetHooks returns all hooks with the given name, filtered by project type
// (if projectType is non-empty), sorted by priority.
func (r *Registry) GetHooks(name, projectType string) []Hook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []Hook
	for _, p := range r.plugins {
		for _, h := range p.Hooks {
			if h.Name == name && h.MatchesProject(projectType) {
				matched = append(matched, h)
			}
		}
	}
	return SortHooks(matched)
}

// Plugins returns a copy of all registered plugins.
func (r *Registry) Plugins() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Plugin, len(r.plugins))
	copy(out, r.plugins)
	return out
}

// Run executes all hooks matching the given name and project type, using the
// provided options. It returns a slice of RunResult — one per hook executed.
func (r *Registry) Run(ctx context.Context, hookName string, opts RunOpts) []RunResult {
	hooks := r.GetHooks(hookName, opts.ProjectType)
	runner := NewHookRunner()
	return runner.RunAll(ctx, hooks, opts, false)
}

// RunWithStopOnError executes all matching hooks but stops on the first error.
func (r *Registry) RunWithStopOnError(ctx context.Context, hookName string, opts RunOpts) []RunResult {
	hooks := r.GetHooks(hookName, opts.ProjectType)
	runner := NewHookRunner()
	return runner.RunAll(ctx, hooks, opts, true)
}

// List returns all registered hook names, sorted alphabetically.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	var names []string
	for _, p := range r.plugins {
		for _, h := range p.Hooks {
			if !seen[h.Name] {
				seen[h.Name] = true
				names = append(names, h.Name)
			}
		}
	}
	return sortedNames(names)
}

// Clear removes all registered plugins and hooks.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = make([]Plugin, 0)
}

// Unregister removes the plugin with the given name and returns true if found.
func (r *Registry) Unregister(pluginName string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, p := range r.plugins {
		if p.Name == pluginName {
			r.plugins = append(r.plugins[:i], r.plugins[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveHook removes a specific hook (by name and command) from a named plugin.
func (r *Registry) RemoveHook(pluginName, hookName string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, p := range r.plugins {
		if p.Name != pluginName {
			continue
		}
		for j, h := range p.Hooks {
			if h.Name == hookName {
				r.plugins[i].Hooks = append(r.plugins[i].Hooks[:j], r.plugins[i].Hooks[j+1:]...)
				return true
			}
		}
	}
	return false
}

// LoadFile reads a JSON plugin manifest from the given file path and registers
// all plugins defined within it.
//
// The file format is:
//
//	{
//	  "plugins": [
//	    {
//	      "name": "my-plugin",
//	      "project_type": "node",
//	      "hooks": [
//	        {
//	          "name": "pre-build",
//	          "command": "npm run lint",
//	          "description": "Run linter before build"
//	        }
//	      ]
//	    }
//	  ]
//	}
func (r *Registry) LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading plugin file %q: %w", path, err)
	}

	return r.loadFromJSON(data, path)
}

// loadFromJSON parses JSON data and registers plugins.
func (r *Registry) loadFromJSON(data []byte, source string) error {
	var manifest pluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parsing plugin file %q: %w", source, err)
	}

	for _, p := range manifest.Plugins {
		if err := r.Register(p); err != nil {
			return fmt.Errorf("plugin file %q: %w", source, err)
		}
	}

	return nil
}

// LoadDir reads all .json files from a directory and registers the plugins
// found in each. Files that are not valid plugin manifests are skipped
// with an error logged to stderr.
func (r *Registry) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading plugin directory %q: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isJSONFile(entry.Name()) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := r.LoadFile(path); err != nil {
			// Log but don't fail — other files may be valid
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, err)
			continue
		}
	}

	return nil
}

// pluginManifest is the top-level JSON structure for a plugin file.
type pluginManifest struct {
	Plugins []Plugin `json:"plugins"`
}

func isJSONFile(name string) bool {
	return len(name) >= 5 && name[len(name)-5:] == ".json"
}

func sortedNames(names []string) []string {
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}
