package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// Plugin represents a discovered executable plugin.
type Plugin struct {
	Name    string // The plugin command name (derived from the filename)
	Path    string // Absolute path to the plugin executable
	IsLocal bool   // Whether the plugin is project-level (vs global)
}

// discoverPlugins scans the project-level and global plugin directories for
// executable files and returns a map of plugin name → Plugin.
// Project-level plugins override global plugins of the same name.
// If neither directory exists, no error is raised.
func discoverPlugins(workDir string) map[string]Plugin {
	plugins := make(map[string]Plugin)

	// Global plugins directory: ~/.sdlc/plugins/
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalPluginsDir := filepath.Join(homeDir, ".sdlc", "plugins")
		loadPluginsFromDir(globalPluginsDir, plugins, false)
	}

	// Project-level plugins directory: <workDir>/.sdlc/plugins/
	if workDir != "" {
		localPluginsDir := filepath.Join(workDir, ".sdlc", "plugins")
		loadPluginsFromDir(localPluginsDir, plugins, true)
	}

	return plugins
}

// loadPluginsFromDir scans a directory for executable files and adds them
// to the plugins map. Returns silently if the directory doesn't exist.
func loadPluginsFromDir(dir string, plugins map[string]Plugin, isLocal bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Directory doesn't exist or can't be read — silently skip
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

	 fullPath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check if file is executable
		if !isExecutable(info) {
			continue
		}

		// Derive plugin name from filename (strip extension for Windows .exe)
		name := strings.TrimSuffix(entry.Name(), ".exe")

		// Only add project-level plugins if they override global, or always add global
		if isLocal || !plugins[name].IsLocal {
			plugins[name] = Plugin{
				Name:    name,
				Path:    fullPath,
				IsLocal: isLocal,
			}
		}
	}
}

// isExecutable checks if a file has any executable bit set.
func isExecutable(info os.FileInfo) bool {
	// Check regular file modes
	m := info.Mode()
	if !m.IsRegular() {
		return false
	}

	// On Windows, check for .exe extension
	if runtime.GOOS == "windows" {
		return strings.HasSuffix(strings.ToLower(info.Name()), ".exe")
	}

	// On Unix, check if any executable bit is set
	return m.Perm()&0111 != 0
}

// registerPluginCommands discovers plugins and registers each as a Cobra
// sub-command on the root command.
func registerPluginCommands(workDir string) {
	plugins := discoverPlugins(workDir)
	if len(plugins) == 0 {
		return
	}

	// Sort for deterministic ordering
	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		plugin := plugins[name]
		pluginCmd := &cobra.Command{
			Use:                name,
			Short:              fmt.Sprintf("Run plugin: %s", name),
			DisableFlagParsing: true, // Pass all args directly to the plugin
			RunE: func(cmd *cobra.Command, args []string) error {
				return runPlugin(plugin, args)
			},
		}
		RootCmd.AddCommand(pluginCmd)
	}
}

// runPlugin executes a plugin with the given arguments.
// It passes --dir and the action (from args) to the plugin.
func runPlugin(plugin Plugin, args []string) error {
	// Resolve working directory
	wd, err := resolveWorkDir(workDir)
	if err != nil {
		return fmt.Errorf("directory error: %w", err)
	}

	// Build plugin arguments: --dir <workdir> [args...]
	pluginArgs := []string{"--dir", wd}
	pluginArgs = append(pluginArgs, args...)

	cmd := exec.Command(plugin.Path, pluginArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
