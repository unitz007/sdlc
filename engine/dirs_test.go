package engine

import (
	"os"
	"path/filepath"
	"testing"

	"sdlc/lib"
)

// createTestDir creates a temporary directory with the given structure.
// structure is a map of relative path -> content. Directories are created
// implicitly. If content is "", a directory is created; otherwise a file
// is created with that content.
func createTestDir(t *testing.T, structure map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sdlc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	for path, content := range structure {
		fullPath := filepath.Join(dir, path)
		if content == "" {
			// Create directory
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				t.Fatalf("failed to create dir %s: %v", path, err)
			}
		} else {
			// Create file (and parent dirs)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatalf("failed to create parent dirs for %s: %v", path, err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				t.Fatalf("failed to create file %s: %v", path, err)
			}
		}
	}

	return dir
}

func TestDetectProjects_Depth0(t *testing.T) {
	// depth 0: only the root directory is checked
	dir := createTestDir(t, map[string]string{
		"go.mod":          "module example",
		"frontend/go.mod": "module frontend",
	})

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run ."},
	}

	projects, err := DetectProjects(dir, tasks, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "go.mod" {
		t.Errorf("expected project name 'go.mod', got %q", projects[0].Name)
	}
	if projects[0].Path != "." {
		t.Errorf("expected project path '.', got %q", projects[0].Path)
	}
}

func TestDetectProjects_Depth1(t *testing.T) {
	// depth 1: root + immediate children (the default)
	dir := createTestDir(t, map[string]string{
		"go.mod":             "module example",
		"frontend/go.mod":    "module frontend",
		"backend/go.mod":     "module backend",
		"frontend/src/main.go": "package main",
		"deep/nested/go.mod": "module deep",
	})

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run ."},
	}

	projects, err := DetectProjects(dir, tasks, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find root + frontend + backend, but NOT deep/nested
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d: %+v", len(projects), projects)
	}

	foundPaths := make(map[string]bool)
	for _, p := range projects {
		foundPaths[p.Path] = true
	}

	for _, expected := range []string{".", "frontend", "backend"} {
		if !foundPaths[expected] {
			t.Errorf("expected to find project at %q", expected)
		}
	}
	if foundPaths[filepath.Join("deep", "nested")] {
		t.Error("should NOT have found project at deep/nested with depth 1")
	}
}

func TestDetectProjects_Depth2(t *testing.T) {
	// depth 2: should find deeply nested projects
	dir := createTestDir(t, map[string]string{
		"go.mod":              "module example",
		"services/api/go.mod": "module api",
		"services/web/go.mod": "module web",
		"deep/nested/go.mod":  "module deep",
	})

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run ."},
	}

	projects, err := DetectProjects(dir, tasks, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundPaths := make(map[string]bool)
	for _, p := range projects {
		foundPaths[p.Path] = true
	}

	// All projects should be found
	for _, expected := range []string{".", "services/api", "services/web", "deep/nested"} {
		if !foundPaths[expected] {
			t.Errorf("expected to find project at %q with depth 2", expected)
		}
	}
}

func TestDetectProjects_NegativeDepth(t *testing.T) {
	// Negative depth should behave like unlimited (up to safety limit)
	dir := createTestDir(t, map[string]string{
		"go.mod":              "module example",
		"a/b/c/d/e/go.mod":    "module deep",
	})

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run ."},
	}

	projects, err := DetectProjects(dir, tasks, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) < 2 {
		t.Fatalf("expected at least 2 projects with unlimited depth, got %d", len(projects))
	}

	foundDeep := false
	for _, p := range projects {
		if p.Path == filepath.Join("a", "b", "c", "d", "e") {
			foundDeep = true
		}
	}
	if !foundDeep {
		t.Error("should have found deeply nested project with unlimited depth")
	}
}

func TestDetectProjects_SkipDirs(t *testing.T) {
	// Should skip well-known non-project directories like node_modules, vendor, etc.
	dir := createTestDir(t, map[string]string{
		"go.mod":                       "module example",
		"node_modules/pkg/go.mod":      "module pkg",
		"vendor/lib/go.mod":            "module lib",
		".git/hooks/go.mod":            "module hooks",
		"real_subproject/go.mod":       "module real",
	})

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run ."},
	}

	projects, err := DetectProjects(dir, tasks, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("expected 2 projects (root + real_subproject), got %d", len(projects))
	}

	foundPaths := make(map[string]bool)
	for _, p := range projects {
		foundPaths[p.Path] = true
	}

	if !foundPaths["."] {
		t.Error("expected to find root project")
	}
	if !foundPaths["real_subproject"] {
		t.Error("expected to find real_subproject")
	}
}

func TestDetectProjects_NoProjects(t *testing.T) {
	dir := createTestDir(t, map[string]string{
		"README.md": "nothing here",
	})

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run ."},
	}

	projects, err := DetectProjects(dir, tasks, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(projects))
	}
}

func TestDetectProjects_Deduplication(t *testing.T) {
	// Ensure one project per directory even if multiple marker files match
	dir := createTestDir(t, map[string]string{
		"go.mod":    "module example",
		"Makefile":  "all:",
		"sub/go.mod": "module sub",
	})

	tasks := map[string]lib.Task{
		"go.mod":   {Run: "go run ."},
		"Makefile": {Build: "make all"},
	}

	projects, err := DetectProjects(dir, tasks, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Root should only be detected once despite having both go.mod and Makefile
	rootCount := 0
	for _, p := range projects {
		if p.Path == "." {
			rootCount++
		}
	}
	if rootCount != 1 {
		t.Errorf("expected root project to be detected exactly once, got %d", rootCount)
	}
}

func TestDetectProjects_RelativePaths(t *testing.T) {
	// Verify that project paths are relative to the working directory
	dir := createTestDir(t, map[string]string{
		"go.mod":              "module example",
		"services/api/go.mod": "module api",
	})

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run ."},
	}

	projects, err := DetectProjects(dir, tasks, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range projects {
		if filepath.IsAbs(p.Path) {
			t.Errorf("expected relative path, got absolute: %q", p.Path)
		}
		if !filepath.IsAbs(p.AbsPath) {
			t.Errorf("expected absolute AbsPath, got relative: %q", p.AbsPath)
		}
	}
}

func TestDefaultSkipDirs(t *testing.T) {
	skipDirs := defaultSkipDirs()

	expectedSkips := []string{
		"node_modules", "vendor", "venv", ".git", ".svn", ".hg",
		"__pycache__", ".idea", "target", "build", "dist",
		".next", ".nuxt", ".gradle", ".cache",
	}

	for _, dir := range expectedSkips {
		if !skipDirs[dir] {
			t.Errorf("expected %q to be in skipDirs", dir)
		}
	}
}

func TestDetectProjects_PreserveExistingBehaviour(t *testing.T) {
	// Verify that depth 1 gives the same results as the old behaviour
	// (root + immediate non-skipped children)
	dir := createTestDir(t, map[string]string{
		"go.mod":          "module example",
		"frontend/go.mod": "module frontend",
		"backend/go.mod":  "module backend",
		".git/go.mod":     "module git",
		".idea/go.mod":    "module idea",
	})

	tasks := map[string]lib.Task{
		"go.mod": {Run: "go run .", Test: "go test ./..."},
	}

	projects, err := DetectProjects(dir, tasks, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 3 {
		t.Fatalf("expected 3 projects (root + frontend + backend), got %d: %+v", len(projects), projects)
	}
}
