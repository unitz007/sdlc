package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	projectPluginsDir = ".sdlc/plugins"
	homePluginsDir    = ".sdlc/plugins"
)

// DiscoveredPlugin represents a plugin found during discovery.
type DiscoveredPlugin struct {
	Name string // Name of the plugin (executable filename)
	Path string // Absolute path to the plugin executable
}

// DiscoverPlugins scans the project-level and global-level plugin directories
// for executable files. Global plugins (home dir) are overridden by project-level
// plugins of the same name. If no plugins directory exists, no error is raised.
//
// Plugin directories searched:
// 1. <project-root>/.sdlc/plugins/
// 2. ~/.sdlc/plugins/
func DiscoverPlugins(workDir string) ([]DiscoveredPlugin, error) {
	plugins := make(map[string]DiscoveredPlugin) // name -> plugin (project overrides global)

	// Scan global plugins first (lower priority)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalPluginsPath := filepath.Join(homeDir, homePluginsDir)
		globalPlugins, _ := scanPluginDir(globalPluginsPath)
		for _, p := range globalPlugins {
			plugins[p.Name] = p
		}
	}

	// Scan project plugins (higher priority, overrides global)
	projectPluginsPath := filepath.Join(workDir, projectPluginsDir)
	projectPlugins, _ := scanPluginDir(projectPluginsPath)
	for _, p := range projectPlugins {
		plugins[p.Name] = p
	}

	if len(plugins) == 0 {
		return nil, nil
	}

	// Convert to sorted slice for deterministic ordering
	result := make([]DiscoveredPlugin, 0, len(plugins))
	for _, p := range plugins {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// scanPluginDir scans a directory for executable files and returns them
// as DiscoveredPlugin entries. Returns empty slice if directory doesn't exist.
func scanPluginDir(dir string) ([]DiscoveredPlugin, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Directory doesn't exist or is not readable — not an error
		return nil, nil
	}

	var plugins []DiscoveredPlugin
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check if the file is executable (any execute bit set)
		if info.Mode()&0111 == 0 {
			continue
		}

		absPath, err := filepath.Abs(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		plugins = append(plugins, DiscoveredPlugin{
			Name: entry.Name(),
			Path: absPath,
		})
	}

	return plugins, nil
}

// RegisterPluginCommands discovers plugins and registers each as a Cobra
// sub-command. Plugin executables receive --dir and the action name as arguments.
func RegisterPluginCommands(workDir string) {
	plugins, err := DiscoverPlugins(workDir)
	if err != nil || len(plugins) == 0 {
		return
	}

	// Check if plugin group already exists (added by RegisterDynamicCommands)
	groupExists := false
	for _, g := range RootCmd.Groups() {
		if g.ID == "custom" {
			groupExists = true
			break
		}
	}
	if !groupExists {
		RootCmd.AddGroup(&cobra.Group{
			ID:    "plugins",
			Title: "Plugins:",
		})
	}

	for _, plugin := range plugins {
		p := plugin // capture loop variable
		pluginCmd := &cobra.Command{
			Use:   p.Name,
			Short: fmt.Sprintf("Run plugin: %s", p.Name),
			Long:  fmt.Sprintf("Executes the plugin '%s' from %s", p.Name, p.Path),
			RunE: func(cmd *cobra.Command, args []string) error {
				return executePlugin(cmd, p, args)
			},
			GroupID: "plugins",
		}
		RootCmd.AddCommand(pluginCmd)
	}
}

// executePlugin runs a discovered plugin executable with appropriate arguments.
func executePlugin(cmd *cobra.Command, plugin DiscoveredPlugin, args []string) error {
	// Resolve working directory
	wd, err := resolveWorkDir(workDir)
	if err != nil {
		return fmt.Errorf("directory error: %w", err)
	}

	// Build plugin arguments: --dir <workdir> <action> <extra-args...>
	pluginArgs := []string{"--dir", wd}
	pluginArgs = append(pluginArgs, args...)

	// Construct the command string for the executor
	cmdStr := plugin.Path
	if len(pluginArgs) > 0 {
		cmdStr += " " + strings.Join(pluginArgs, " ")
	}

	// Use the existing runCommand infrastructure
	return runCommand(cmd.Context(), cmdStr, wd, os.Stdout, os.Stderr, nil)
}
