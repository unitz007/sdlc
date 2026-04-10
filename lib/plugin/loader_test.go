package plugin

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile(t *testing.T) {
	r := NewRegistry()

	// Create a temporary JSON file
	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "plugins.json")
	content := `{
  "plugins": [
    {
      "name": "my-plugin",
      "project_type": "go",
      "hooks": [
        {
          "name": "pre-build",
          "command": "go vet ./...",
          "description": "Run go vet"
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(pluginFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.LoadFile(pluginFile); err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	hooks := r.GetHooks("pre-build", "go")
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0].Command != "go vet ./..." {
		t.Errorf("unexpected command: %s", hooks[0].Command)
	}

	// Should not match different project type
	hooks = r.GetHooks("pre-build", "node")
	if len(hooks) != 0 {
		t.Errorf("expected 0 hooks for node project type, got %d", len(hooks))
	}
}

func TestLoadFileInvalid(t *testing.T) {
	r := NewRegistry()

	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "bad.json")
	content := `{"plugins": [{"name": "bad", "hooks": [{"name": "invalid-name", "command": "echo"}]}]}`
	if err := os.WriteFile(pluginFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.LoadFile(pluginFile); err == nil {
		t.Error("expected error for invalid hook name")
	}
}

func TestLoadFileMalformedJSON(t *testing.T) {
	r := NewRegistry()

	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "malformed.json")
	if err := os.WriteFile(pluginFile, []byte(`{invalid json`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := r.LoadFile(pluginFile); err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadFile("/nonexistent/file.json"); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadDir(t *testing.T) {
	r := NewRegistry()

	tmpDir := t.TempDir()

	// Valid plugin file
	validJSON := `{
  "plugins": [
    {
      "name": "plugin-a",
      "hooks": [
        {"name": "pre-build", "command": "echo a"}
      ]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "plugin-a.json"), []byte(validJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Another valid file
	validJSON2 := `{
  "plugins": [
    {
      "name": "plugin-b",
      "hooks": [
        {"name": "post-test", "command": "echo b"}
      ]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "plugin-b.json"), []byte(validJSON2), 0644); err != nil {
		t.Fatal(err)
	}

	// Invalid file (should be skipped with warning)
	if err := os.WriteFile(filepath.Join(tmpDir, "bad.json"), []byte(`{invalid}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Non-JSON file (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("not a plugin"), 0644); err != nil {
		t.Fatal(err)
	}

	// Redirect stderr to capture warnings
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()
	var stderr bytes.Buffer
	os.Stderr = &stderr

	if err := r.LoadDir(tmpDir); err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}

	// Should have loaded 2 plugins
	if len(r.Plugins()) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(r.Plugins()))
	}

	// Should have logged a warning for bad.json
	if !strings.Contains(stderr.String(), "bad.json") {
		t.Error("expected warning about bad.json in stderr")
	}
}

func TestLoadDirEmpty(t *testing.T) {
	r := NewRegistry()
	tmpDir := t.TempDir()

	if err := r.LoadDir(tmpDir); err != nil {
		t.Fatalf("LoadDir() empty dir error = %v", err)
	}
	if len(r.Plugins()) != 0 {
		t.Error("expected no plugins from empty dir")
	}
}

func TestLoadDirNotExist(t *testing.T) {
	r := NewRegistry()
	if err := r.LoadDir("/nonexistent/dir"); err == nil {
		t.Error("expected error for nonexistent directory")
	}
}
