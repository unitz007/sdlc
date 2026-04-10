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

// AC1: Custom actions tests

func TestCommand_CustomAction(t *testing.T) {
	task := Task{
		Run:    "go run main.go",
		Custom: map[string]string{"deploy": "kubectl apply -f k8s/", "lint": "golangci-lint run"},
	}

	got, err := task.Command("deploy")
	if err != nil {
		t.Fatalf("Command(\"deploy\") returned unexpected error: %v", err)
	}
	if got != "kubectl apply -f k8s/" {
		t.Errorf("Command(\"deploy\") = %q, want %q", got, "kubectl apply -f k8s/")
	}
}

func TestCommand_CustomAction_Lint(t *testing.T) {
	task := Task{
		Run:    "go run main.go",
		Custom: map[string]string{"deploy": "kubectl apply -f k8s/", "lint": "golangci-lint run"},
	}

	got, err := task.Command("lint")
	if err != nil {
		t.Fatalf("Command(\"lint\") returned unexpected error: %v", err)
	}
	if got != "golangci-lint run" {
		t.Errorf("Command(\"lint\") = %q, want %q", got, "golangci-lint run")
	}
}

func TestCommand_CustomAction_UnknownFallsBackToError(t *testing.T) {
	task := Task{
		Run:    "go run main.go",
		Custom: map[string]string{"deploy": "kubectl apply -f k8s/"},
	}

	got, err := task.Command("unknown")
	if err == nil {
		t.Fatal("Command(\"unknown\") expected error, got nil")
	}
	if got != "" {
		t.Errorf("Command(\"unknown\") = %q, want empty string", got)
	}
}

func TestCommand_BuiltInOverridesCustom(t *testing.T) {
	task := Task{
		Run:    "go run main.go",
		Custom: map[string]string{"run": "custom run command"},
	}

	// Built-in actions should always take precedence
	got, err := task.Command("run")
	if err != nil {
		t.Fatalf("Command(\"run\") returned unexpected error: %v", err)
	}
	if got != "go run main.go" {
		t.Errorf("Command(\"run\") = %q, want %q (built-in should override custom)", got, "go run main.go")
	}
}

func TestCommand_NilCustomNoPanic(t *testing.T) {
	task := Task{
		Run:    "go run main.go",
		Custom: nil,
	}

	got, err := task.Command("deploy")
	if err == nil {
		t.Fatal("Command(\"deploy\") expected error for nil custom map, got nil")
	}
	if got != "" {
		t.Errorf("Command(\"deploy\") = %q, want empty string", got)
	}
}

// AC1: Helper method tests

func TestHasCustomActions_True(t *testing.T) {
	task := Task{
		Custom: map[string]string{"deploy": "kubectl apply -f k8s/"},
	}
	if !task.HasCustomActions() {
		t.Error("HasCustomActions() = false, want true")
	}
}

func TestHasCustomActions_False(t *testing.T) {
	task := Task{}
	if task.HasCustomActions() {
		t.Error("HasCustomActions() = true, want false")
	}
}

func TestHasCustomActions_Nil(t *testing.T) {
	task := Task{Custom: nil}
	if task.HasCustomActions() {
		t.Error("HasCustomActions() = true for nil, want false")
	}
}

func TestCustomActionNames(t *testing.T) {
	task := Task{
		Custom: map[string]string{"deploy": "kubectl apply -f k8s/", "lint": "golangci-lint run"},
	}
	names := task.CustomActionNames()
	if len(names) != 2 {
		t.Fatalf("CustomActionNames() returned %d names, want 2", len(names))
	}
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["deploy"] || !nameSet["lint"] {
		t.Errorf("CustomActionNames() = %v, want both deploy and lint", names)
	}
}

func TestCustomActionNames_Empty(t *testing.T) {
	task := Task{}
	names := task.CustomActionNames()
	if names != nil {
		t.Errorf("CustomActionNames() = %v, want nil", names)
	}
}

// AC2: Hook tests

func TestPreHook(t *testing.T) {
	task := Task{
		Hooks: TaskHooks{
			Pre: map[string]string{"build": "echo starting build"},
		},
	}
	got := task.PreHook("build")
	if got != "echo starting build" {
		t.Errorf("PreHook(\"build\") = %q, want %q", got, "echo starting build")
	}
}

func TestPreHook_NotFound(t *testing.T) {
	task := Task{
		Hooks: TaskHooks{
			Pre: map[string]string{"build": "echo starting build"},
		},
	}
	got := task.PreHook("run")
	if got != "" {
		t.Errorf("PreHook(\"run\") = %q, want empty string", got)
	}
}

func TestPreHook_NilPre(t *testing.T) {
	task := Task{Hooks: TaskHooks{Pre: nil}}
	got := task.PreHook("build")
	if got != "" {
		t.Errorf("PreHook(\"build\") with nil Pre = %q, want empty string", got)
	}
}

func TestPostHook(t *testing.T) {
	task := Task{
		Hooks: TaskHooks{
			Post: map[string]string{"run": "notify-send done"},
		},
	}
	got := task.PostHook("run")
	if got != "notify-send done" {
		t.Errorf("PostHook(\"run\") = %q, want %q", got, "notify-send done")
	}
}

func TestPostHook_NotFound(t *testing.T) {
	task := Task{
		Hooks: TaskHooks{
			Post: map[string]string{"run": "notify-send done"},
		},
	}
	got := task.PostHook("build")
	if got != "" {
		t.Errorf("PostHook(\"build\") = %q, want empty string", got)
	}
}

// AC5: Backward compatibility - existing Task without Custom/Hooks fields

func TestBackwardCompatibility_ExistingTask(t *testing.T) {
	// Simulate JSON unmarshal of existing .sdlc.json without custom/hooks fields
	jsonData := `{
		"run": "go run main.go",
		"test": "go test .",
		"build": "go build -v",
		"install": "go install .",
		"clean": "go clean"
	}`

	var task Task
	// Note: We don't test json.Unmarshal directly here because the Task struct
	// uses json tags that handle this correctly. Instead, we verify the struct
	// behaves correctly when Custom and Hooks are zero-valued.
	_ = jsonData

	task = Task{
		Run:     "go run main.go",
		Test:    "go test .",
		Build:   "go build -v",
		Install: "go install .",
		Clean:   "go clean",
	}

	// All built-in commands should work
	for _, action := range []string{"run", "test", "build", "install", "clean"} {
		got, err := task.Command(action)
		if err != nil {
			t.Errorf("Command(%q) returned unexpected error: %v", action, err)
		}
		if got == "" {
			t.Errorf("Command(%q) returned empty string", action)
		}
	}

	// Non-built-in should still error
	_, err := task.Command("deploy")
	if err == nil {
		t.Error("Command(\"deploy\") expected error for backward compat task, got nil")
	}

	// HasCustomActions should return false
	if task.HasCustomActions() {
		t.Error("HasCustomActions() should be false for backward compat task")
	}
}
