package engine

import (
	"os"
	"path/filepath"
	"testing"
	"sdlc/lib"
)

func TestDetectProjects_MaxDepthZero(t *testing.T) {
	// maxDepth 0: only scan root, no subdirectories
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, "sub", "deep"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "sub", "go.mod"), []byte("module sub\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module root\n"), 0644)

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run .", Test: "go test ./..."},
	}

	projects, err := DetectProjects(tmpDir, tasks, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project at maxDepth=0, got %d", len(projects))
	}
	if projects[0].Name != "go.mod" {
		t.Errorf("expected go.mod, got %s", projects[0].Name)
	}
	if filepath.Base(projects[0].AbsPath) != filepath.Base(tmpDir) {
		t.Errorf("expected root dir, got %s", projects[0].AbsPath)
	}
}

func TestDetectProjects_MaxDepthOne(t *testing.T) {
	// maxDepth 1: root + immediate subdirectories
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	deepDir := filepath.Join(subDir, "deep")
	_ = os.MkdirAll(deepDir, 0755)

	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module root\n"), 0644)
	_ = os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module sub\n"), 0644)
	_ = os.WriteFile(filepath.Join(deepDir, "go.mod"), []byte("module deep\n"), 0644)

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run .", Test: "go test ./..."},
	}

	projects, err := DetectProjects(tmpDir, tasks, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("expected 2 projects at maxDepth=1, got %d", len(projects))
	}

	names := map[string]bool{}
	for _, p := range projects {
		names[p.Name] = true
	}
	if !names["go.mod"] {
		t.Error("expected to find root go.mod project")
	}
}

func TestDetectProjects_MaxDepthTwo(t *testing.T) {
	// maxDepth 2: root + sub + deep
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	deepDir := filepath.Join(subDir, "deep")
	_ = os.MkdirAll(deepDir, 0755)

	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module root\n"), 0644)
	_ = os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module sub\n"), 0644)
	_ = os.WriteFile(filepath.Join(deepDir, "go.mod"), []byte("module deep\n"), 0644)

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run .", Test: "go test ./..."},
	}

	projects, err := DetectProjects(tmpDir, tasks, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 3 {
		t.Fatalf("expected 3 projects at maxDepth=2, got %d", len(projects))
	}
}

func TestDetectProjects_SkippedDirs(t *testing.T) {
	// Directories in lib.SkippedDirs should not be traversed
	tmpDir := t.TempDir()
	nodeModules := filepath.Join(tmpDir, "node_modules", "pkg")
	gitDir := filepath.Join(tmpDir, ".git", "hooks")
	_ = os.MkdirAll(nodeModules, 0755)
	_ = os.MkdirAll(gitDir, 0755)

	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module root\n"), 0644)
	_ = os.WriteFile(filepath.Join(nodeModules, "go.mod"), []byte("module nm-pkg\n"), 0644)
	_ = os.WriteFile(filepath.Join(gitDir, "go.mod"), []byte("module git-hooks\n"), 0644)

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run .", Test: "go test ./..."},
	}

	projects, err := DetectProjects(tmpDir, tasks, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project (skipped node_modules and .git), got %d", len(projects))
	}
}

func TestDetectProjects_NoDuplicates(t *testing.T) {
	// Symlink or re-visiting the same directory should not create duplicates
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	_ = os.MkdirAll(subDir, 0755)

	// Root has go.mod, sub does not
	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module root\n"), 0644)

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run .", Test: "go test ./..."},
	}

	projects, err := DetectProjects(tmpDir, tasks, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
}

func TestDetectProjects_MultipleBuildFiles(t *testing.T) {
	// One project per directory: if both go.mod and package.json exist in same dir,
	// only the first one found (by os.ReadDir order) should be used.
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module root\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{\"name\": \"root\"}\n"), 0644)

	tasks := map[string]lib.Task{
		"go.mod":        {Run: "go run ."},
		"package.json":  {Run: "npm start"},
	}

	projects, err := DetectProjects(tmpDir, tasks, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only one project per directory
	if len(projects) != 1 {
		t.Fatalf("expected 1 project (one per dir), got %d", len(projects))
	}
}
