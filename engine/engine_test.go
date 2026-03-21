package engine

import (
	"os"
	"path/filepath"
	"sdlc/lib"
	"strings"
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

// tasksWithPackageJson returns a task map that recognizes package.json files.
func tasksWithPackageJson() map[string]lib.Task {
	return map[string]lib.Task{
		"package.json": {Run: "npm run start"},
	}
}

// tasksWithBoth returns a task map that recognizes both go.mod and package.json files.
func tasksWithBoth() map[string]lib.Task {
	return map[string]lib.Task{
		"go.mod":        {Run: "go run ."},
		"package.json":  {Run: "npm run start"},
	}
}

func TestDetectProjects_SkipsNodeModulesPackageJson(t *testing.T) {
	dir := t.TempDir()

	nmDir := filepath.Join(dir, "node_modules", "sub", "pkg")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "package.json"), []byte(`{"name":"pkg"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithPackageJson())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	for _, p := range projects {
		if strings.HasPrefix(p.Path, "node_modules") {
			t.Errorf("expected node_modules to be skipped, but found project with path %q", p.Path)
		}
	}
}

func TestDetectProjects_SkipsVendor(t *testing.T) {
	dir := t.TempDir()

	vendorDir := filepath.Join(dir, "vendor", "lib")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "go.mod"), []byte("module lib\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithGoMod())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	for _, p := range projects {
		if strings.HasPrefix(p.Path, "vendor") {
			t.Errorf("expected vendor to be skipped, but found project with path %q", p.Path)
		}
	}
}

func TestDetectProjects_SkipsGit(t *testing.T) {
	dir := t.TempDir()

	gitDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "some.go.mod"), []byte("module hooks\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithGoMod())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	for _, p := range projects {
		if strings.HasPrefix(p.Path, ".git") {
			t.Errorf("expected .git to be skipped, but found project with path %q", p.Path)
		}
	}
}

func TestDetectProjects_SkipsBuildDirs(t *testing.T) {
	dir := t.TempDir()

	buildDirs := []string{"dist", "build", "out", "target"}
	for _, bd := range buildDirs {
		appDir := filepath.Join(dir, bd, "app")
		if err := os.MkdirAll(appDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte(`{"name":"app"}`+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	projects, err := DetectProjects(dir, tasksWithPackageJson())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	for _, p := range projects {
		for _, bd := range buildDirs {
			if strings.HasPrefix(p.Path, bd) {
				t.Errorf("expected %s to be skipped, but found project with path %q", bd, p.Path)
			}
		}
	}
}

func TestDetectProjects_SkipsIDEDirs(t *testing.T) {
	dir := t.TempDir()

	ideaDir := filepath.Join(dir, ".idea", "runConfigurations")
	if err := os.MkdirAll(ideaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ideaDir, "package.json"), []byte(`{"name":"config"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	vscodeDir := filepath.Join(dir, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vscodeDir, "package.json"), []byte(`{"name":"vscode"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithPackageJson())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	for _, p := range projects {
		if strings.HasPrefix(p.Path, ".idea") || strings.HasPrefix(p.Path, ".vscode") {
			t.Errorf("expected IDE dirs to be skipped, but found project with path %q", p.Path)
		}
	}
}

func TestDetectProjects_DetectsValidModules(t *testing.T) {
	dir := t.TempDir()

	// Legitimate module
	apiDir := filepath.Join(dir, "services", "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(apiDir, "package.json"), []byte(`{"name":"api"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Excluded directory with build file
	nmDir := filepath.Join(dir, "node_modules", "some-pkg")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "package.json"), []byte(`{"name":"some-pkg"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projects, err := DetectProjects(dir, tasksWithPackageJson())
	if err != nil {
		t.Fatalf("DetectProjects returned error: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	expected := filepath.Join("services", "api")
	if projects[0].Path != expected {
		t.Errorf("expected path %q, got %q", expected, projects[0].Path)
	}
}

func TestDetectProjects_DetectsRootModule(t *testing.T) {
	dir := t.TempDir()

	// Root-level module
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module root\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Excluded vendor directory with build file
	vendorDir := filepath.Join(dir, "vendor", "lib")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "go.mod"), []byte("module lib\n"), 0644); err != nil {
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
