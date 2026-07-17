package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPlugins_NoDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	plugins := discoverPlugins(tmpDir)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func TestDiscoverPlugins_GlobalPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake "home" directory with plugins
	homeDir := filepath.Join(tmpDir, "home")
	pluginsDir := filepath.Join(homeDir, ".sdlc", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a plugin file
	pluginPath := filepath.Join(pluginsDir, "my-plugin")
	if err := os.WriteFile(pluginPath, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	// Temporarily override HOME to point to our fake home
	t.Setenv("HOME", homeDir)

	plugins := discoverPlugins(tmpDir)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if _, ok := plugins["my-plugin"]; !ok {
		t.Error("expected 'my-plugin' plugin")
	}
	if !plugins["my-plugin"].IsLocal {
		t.Error("expected global plugin, got local")
	}
}

func TestDiscoverPlugins_ProjectOverridesGlobal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create global plugin
	homeDir := filepath.Join(tmpDir, "home")
	globalPluginsDir := filepath.Join(homeDir, ".sdlc", "plugins")
	if err := os.MkdirAll(globalPluginsDir, 0755); err != nil {
		t.Fatal(err)
	}
	globalPlugin := filepath.Join(globalPluginsDir, "shared-plugin")
	if err := os.WriteFile(globalPlugin, []byte("#!/bin/sh\necho global"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create local plugin with the same name
	localPluginsDir := filepath.Join(tmpDir, ".sdlc", "plugins")
	if err := os.MkdirAll(localPluginsDir, 0755); err != nil {
		t.Fatal(err)
	}
	localPlugin := filepath.Join(localPluginsDir, "shared-plugin")
	if err := os.WriteFile(localPlugin, []byte("#!/bin/sh\necho local"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", homeDir)

	plugins := discoverPlugins(tmpDir)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin (local overrides global), got %d", len(plugins))
	}
	if !plugins["shared-plugin"].IsLocal {
		t.Error("expected local plugin to override global")
	}
}

func TestDiscoverPlugins_NonExecutableIgnored(t *testing.T) {
	tmpDir := t.TempDir()

	homeDir := filepath.Join(tmpDir, "home")
	pluginsDir := filepath.Join(homeDir, ".sdlc", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a non-executable file
	nonExecPath := filepath.Join(pluginsDir, "not-a-plugin")
	if err := os.WriteFile(nonExecPath, []byte("not executable"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an executable file
	execPath := filepath.Join(pluginsDir, "is-a-plugin")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho yes"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", homeDir)

	plugins := discoverPlugins(tmpDir)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if _, ok := plugins["is-a-plugin"]; !ok {
		t.Error("expected 'is-a-plugin' plugin")
	}
}

func TestIsExecutable(t *testing.T) {
	// Create temp files with different permissions
	tmpDir := t.TempDir()

	execPath := filepath.Join(tmpDir, "exec-file")
	if err := os.WriteFile(execPath, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(execPath)
	if !isExecutable(info) {
		t.Error("expected 0755 file to be executable")
	}

	nonExecPath := filepath.Join(tmpDir, "non-exec-file")
	if err := os.WriteFile(nonExecPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	info2, _ := os.Stat(nonExecPath)
	if isExecutable(info2) {
		t.Error("expected 0644 file to NOT be executable")
	}
}
