package engine

import (
	"os"
	"path/filepath"
	"sdlc/lib"
	"testing"
)

// tasksWithGoMod returns a task map that recognizes go.mod files.
func tasksWithGoMod() map[string]lib.Task {
	return map[string]lib.Task{
		"go.mod": {Run: "go run .", Test: "go test ./...", Build: "go build .", Install: "go install .", Clean: "go clean"},
	}
}

func TestDetectProjects_TwoSubdirectories(t *testing.T) {
	dir := t.TempDir()

	backendDir := filepath.Join(dir, "backend")
	frontendDir := filepath.Join(dir, "frontend")
	if err := os.MkdirAll(backendDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backendDir, "go.mod"), []byte("module backend\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(frontendDir, "go.mod"), []byte("module frontend\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithGoMod())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	expectedPaths := map[string]bool{"backend": true, "frontend": true}
	for _, p := range projects {
		if !expectedPaths[p.Path] {
			t.Errorf("unexpected project path %q", p.Path)
		}
	}
}

func TestDetectProjects_NestedModule(t *testing.T) {
	dir := t.TempDir()

	nestedDir := filepath.Join(dir, "services", "api")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "go.mod"), []byte("module api\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithGoMod())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Path != filepath.Join("services", "api") {
		t.Errorf("expected path %q, got %q", filepath.Join("services", "api"), projects[0].Path)
	}
}

func TestDetectProjects_SingleModule(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module root\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithGoMod())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Path != "." {
		t.Errorf("expected path %q, got %q", ".", projects[0].Path)
	}
}

func TestDetectProjects_SkipsDotDirectories(t *testing.T) {
	dir := t.TempDir()

	hiddenDir := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "go.mod"), []byte("module hidden\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithGoMod())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	for _, p := range projects {
		if p.Path == ".hidden" || p.Path == filepath.Join(".hidden") {
			t.Errorf("expected .hidden directory to be skipped, but found project with path %q", p.Path)
		}
	}
}

func TestDetectProjects_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()

	nmDir := filepath.Join(dir, "node_modules", "pkg")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "go.mod"), []byte("module pkg\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithGoMod())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	for _, p := range projects {
		if p.Path == "node_modules" || p.Path == filepath.Join("node_modules", "pkg") {
			t.Errorf("expected node_modules to be skipped, but found project with path %q", p.Path)
		}
	}
}

func TestDetectProjects_RootAndSubdir(t *testing.T) {
	dir := t.TempDir()

	backendDir := filepath.Join(dir, "backend")
	if err := os.MkdirAll(backendDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module root\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backendDir, "go.mod"), []byte("module backend\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithGoMod())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	expectedPaths := map[string]bool{".": true, "backend": true}
	for _, p := range projects {
		if !expectedPaths[p.Path] {
			t.Errorf("unexpected project path %q", p.Path)
		}
	}
}
