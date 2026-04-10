package lib

import (
	"testing"
)

func TestCommand_Run(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("run")
	if err != nil {
		t.Fatalf("Command(\"run\") returned unexpected error: %v", err)
	}
	if got != task.Run {
		t.Errorf("Command(\"run\") = %q, want %q", got, task.Run)
	}
}

func TestCommand_Test(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("test")
	if err != nil {
		t.Fatalf("Command(\"test\") returned unexpected error: %v", err)
	}
	if got != task.Test {
		t.Errorf("Command(\"test\") = %q, want %q", got, task.Test)
	}
}

func TestCommand_Build(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("build")
	if err != nil {
		t.Fatalf("Command(\"build\") returned unexpected error: %v", err)
	}
	if got != task.Build {
		t.Errorf("Command(\"build\") = %q, want %q", got, task.Build)
	}
}

func TestCommand_Install(t *testing.T) {
	task := Task{
		Install: "go mod download",
	}

	got, err := task.Command("install")
	if err != nil {
		t.Fatalf("Command(\"install\") returned unexpected error: %v", err)
	}
	if got != task.Install {
		t.Errorf("Command(\"install\") = %q, want %q", got, task.Install)
	}
}

func TestCommand_Clean(t *testing.T) {
	task := Task{
		Clean: "go clean -cache",
	}

	got, err := task.Command("clean")
	if err != nil {
		t.Fatalf("Command(\"clean\") returned unexpected error: %v", err)
	}
	if got != task.Clean {
		t.Errorf("Command(\"clean\") = %q, want %q", got, task.Clean)
	}
}

func TestCommand_InvalidField(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("deploy")
	if err == nil {
		t.Fatal("Command(\"deploy\") expected error, got nil")
	}
	if got != "" {
		t.Errorf("Command(\"deploy\") = %q, want empty string", got)
	}
	if err.Error() != "invalid command" {
		t.Errorf("Command(\"deploy\") error = %q, want %q", err.Error(), "invalid command")
	}
}

func TestCommand_EmptyField(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("")
	if err == nil {
		t.Fatal("Command(\"\") expected error, got nil")
	}
	if got != "" {
		t.Errorf("Command(\"\") = %q, want empty string", got)
	}
}

func TestCommand_EmptyTask(t *testing.T) {
	task := Task{}

	got, err := task.Command("run")
	if err != nil {
		t.Fatalf("Command(\"run\") on empty task returned unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("Command(\"run\") on empty task = %q, want empty string", got)
	}
}

// --- Custom actions tests ---

func TestCommand_CustomAction(t *testing.T) {
	task := Task{
		Run: "go run main.go",
		Custom: map[string]string{
			"deploy": "kubectl apply -f k8s/",
			"lint":   "golangci-lint run",
		},
	}

	got, err := task.Command("deploy")
	if err != nil {
		t.Fatalf("Command(\"deploy\") returned unexpected error: %v", err)
	}
	if got != "kubectl apply -f k8s/" {
		t.Errorf("Command(\"deploy\") = %q, want %q", got, "kubectl apply -f k8s/")
	}

	got, err = task.Command("lint")
	if err != nil {
		t.Fatalf("Command(\"lint\") returned unexpected error: %v", err)
	}
	if got != "golangci-lint run" {
		t.Errorf("Command(\"lint\") = %q, want %q", got, "golangci-lint run")
	}
}

func TestCommand_CustomActionFallback(t *testing.T) {
	task := Task{
		Custom: map[string]string{
			"migrate": "go run migrations/main.go",
		},
	}

	// Built-in actions still work
	got, err := task.Command("run")
	if err != nil {
		t.Fatalf("Command(\"run\") returned unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("Command(\"run\") = %q, want empty string", got)
	}

	// Custom action works
	got, err = task.Command("migrate")
	if err != nil {
		t.Fatalf("Command(\"migrate\") returned unexpected error: %v", err)
	}
	if got != "go run migrations/main.go" {
		t.Errorf("Command(\"migrate\") = %q, want %q", got, "go run migrations/main.go")
	}

	// Unknown action still errors
	_, err = task.Command("nonexistent")
	if err == nil {
		t.Fatal("Command(\"nonexistent\") expected error, got nil")
	}
}

func TestCommand_CustomActionEmpty(t *testing.T) {
	task := Task{
		Custom: map[string]string{
			"empty": "",
		},
	}

	got, err := task.Command("empty")
	if err != nil {
		t.Fatalf("Command(\"empty\") returned unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("Command(\"empty\") = %q, want empty string", got)
	}
}

func TestTask_CustomActions(t *testing.T) {
	task := Task{
		Custom: map[string]string{
			"deploy": "kubectl apply -f k8s/",
			"lint":   "golangci-lint run",
		},
	}

	actions := task.CustomActions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 custom actions, got %d", len(actions))
	}

	// Convert to set for easier checking
	set := make(map[string]bool)
	for _, a := range actions {
		set[a] = true
	}
	if !set["deploy"] {
		t.Error("expected 'deploy' in custom actions")
	}
	if !set["lint"] {
		t.Error("expected 'lint' in custom actions")
	}
}

func TestTask_CustomActionsEmpty(t *testing.T) {
	task := Task{}
	actions := task.CustomActions()
	if len(actions) != 0 {
		t.Errorf("expected 0 custom actions, got %d", len(actions))
	}
}

func TestTask_AllActions(t *testing.T) {
	task := Task{
		Custom: map[string]string{
			"deploy": "kubectl apply",
		},
	}

	actions := task.AllActions()
	if len(actions) != 6 {
		t.Fatalf("expected 6 actions (5 built-in + 1 custom), got %d", len(actions))
	}

	set := make(map[string]bool)
	for _, a := range actions {
		set[a] = true
	}
	for _, expected := range []string{"run", "test", "build", "install", "clean", "deploy"} {
		if !set[expected] {
			t.Errorf("expected %q in all actions", expected)
		}
	}
}

// --- Hooks tests ---

func TestTaskHooks_PreHook(t *testing.T) {
	hooks := TaskHooks{
		Pre: map[string]string{
			"build": "echo starting build...",
			"run":   "npm install",
		},
	}

	got := hooks.Hook("pre", "build")
	if got != "echo starting build..." {
		t.Errorf("Hook(\"pre\", \"build\") = %q, want %q", got, "echo starting build...")
	}

	got = hooks.Hook("pre", "run")
	if got != "npm install" {
		t.Errorf("Hook(\"pre\", \"run\") = %q, want %q", got, "npm install")
	}

	// Non-existent hook returns empty string
	got = hooks.Hook("pre", "test")
	if got != "" {
		t.Errorf("Hook(\"pre\", \"test\") = %q, want empty string", got)
	}
}

func TestTaskHooks_PostHook(t *testing.T) {
	hooks := TaskHooks{
		Post: map[string]string{
			"run":  "notify-send done",
			"test": "go cover ./...",
		},
	}

	got := hooks.Hook("post", "run")
	if got != "notify-send done" {
		t.Errorf("Hook(\"post\", \"run\") = %q, want %q", got, "notify-send done")
	}

	got = hooks.Hook("post", "test")
	if got != "go cover ./..." {
		t.Errorf("Hook(\"post\", \"test\") = %q, want %q", got, "go cover ./...")
	}

	// Non-existent hook returns empty string
	got = hooks.Hook("post", "build")
	if got != "" {
		t.Errorf("Hook(\"post\", \"build\") = %q, want empty string", got)
	}
}

func TestTaskHooks_EmptyMaps(t *testing.T) {
	hooks := TaskHooks{}

	if got := hooks.Hook("pre", "build"); got != "" {
		t.Errorf("Hook(\"pre\", \"build\") on empty hooks = %q, want empty string", got)
	}
	if got := hooks.Hook("post", "build"); got != "" {
		t.Errorf("Hook(\"post\", \"build\") on empty hooks = %q, want empty string", got)
	}
	if got := hooks.Hook("unknown", "build"); got != "" {
		t.Errorf("Hook(\"unknown\", \"build\") on empty hooks = %q, want empty string", got)
	}
}

func TestTask_NilCustomDoesNotPanic(t *testing.T) {
	task := Task{} // Custom is nil

	_, err := task.Command("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown command with nil Custom")
	}
}
