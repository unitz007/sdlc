package io

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sdlc/lib"
	"testing"
)

func TestNewCommand_ReturnsNonNil(t *testing.T) {
	cli := NewCommand("test-cmd", "A test command", nil)
	if cli == nil {
		t.Fatal("NewCommand returned nil")
	}
	if cli.Cmd == nil {
		t.Fatal("NewCommand returned CLI with nil Cmd")
	}
}

func TestNewCommand_SetsUseAndDescription(t *testing.T) {
	cli := NewCommand("run", "Runs the project", nil)

	if cli.Cmd.Use != "run" {
		t.Errorf("Cmd.Use = %q, want %q", cli.Cmd.Use, "run")
	}
	if cli.Cmd.Short != "Runs the project" {
		t.Errorf("Cmd.Short = %q, want %q", cli.Cmd.Short, "Runs the project")
	}
	if cli.Cmd.Long != "Runs the project" {
		t.Errorf("Cmd.Long = %q, want %q", cli.Cmd.Long, "Runs the project")
	}
}

func TestNewCommand_RegistersFlags(t *testing.T) {
	cli := NewCommand("build", "Builds the project", nil)

	flags := []struct {
		name      string
		shorthand string
	}{
		{"dir", "d"},
		{"extraArgs", "e"},
		{"config", "c"},
	}

	for _, f := range flags {
		flag := cli.Cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("expected flag %q to be registered", f.name)
			continue
		}
		if flag.Shorthand != f.shorthand {
			t.Errorf("flag %q shorthand = %q, want %q", f.name, flag.Shorthand, f.shorthand)
		}
	}
}

func TestGetBuilds_ValidConfig(t *testing.T) {
	// Create a temp directory with a valid .sdlc.json config file
	tmpDir := t.TempDir()
	config := map[string]lib.Task{
		"go.mod": {
			Run:   "go run main.go",
			Test:  "go test .",
			Build: "go build -v",
		},
		"pom.xml": {
			Run:   "mvn spring-boot:run",
			Test:  "mvn test",
			Build: "mvn build",
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	configPath := filepath.Join(tmpDir, ".sdlc.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	builds := GetBuilds(tmpDir)

	if len(builds) != 2 {
		t.Fatalf("GetBuilds returned %d entries, want 2", len(builds))
	}

	goTask, ok := builds["go.mod"]
	if !ok {
		t.Fatal("GetBuilds missing \"go.mod\" entry")
	}
	if goTask.Run != "go run main.go" {
		t.Errorf("go.mod Run = %q, want %q", goTask.Run, "go run main.go")
	}
	if goTask.Test != "go test ." {
		t.Errorf("go.mod Test = %q, want %q", goTask.Test, "go test .")
	}
	if goTask.Build != "go build -v" {
		t.Errorf("go.mod Build = %q, want %q", goTask.Build, "go build -v")
	}

	pomTask, ok := builds["pom.xml"]
	if !ok {
		t.Fatal("GetBuilds missing \"pom.xml\" entry")
	}
	if pomTask.Run != "mvn spring-boot:run" {
		t.Errorf("pom.xml Run = %q, want %q", pomTask.Run, "mvn spring-boot:run")
	}
}

func TestGetBuilds_SingleEntry(t *testing.T) {
	tmpDir := t.TempDir()
	config := map[string]lib.Task{
		"Package.swift": {
			Run:   "swift run",
			Test:  "swift test",
			Build: "swift build",
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	configPath := filepath.Join(tmpDir, ".sdlc.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	builds := GetBuilds(tmpDir)

	if len(builds) != 1 {
		t.Fatalf("GetBuilds returned %d entries, want 1", len(builds))
	}

	swiftTask, ok := builds["Package.swift"]
	if !ok {
		t.Fatal("GetBuilds missing \"Package.swift\" entry")
	}
	if swiftTask.Run != "swift run" {
		t.Errorf("Package.swift Run = %q, want %q", swiftTask.Run, "swift run")
	}
}
