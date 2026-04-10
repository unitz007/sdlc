package config

import (
	"sdlc/lib"
	"strings"
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	tasks := map[string]lib.Task{
		"main.go": {
			Run:   "go run .",
			Test:  "go test ./...",
			Build: "go build .",
		},
		"pom.xml": {
			Build: "mvn package",
			Test:  "mvn test",
			Custom: map[string]string{
				"deploy": "mvn deploy",
			},
			Hooks: lib.TaskHooks{
				Pre: map[string]string{
					"build": "echo 'building'",
				},
			},
		},
	}

	if err := Validate(tasks, ".sdlc.json"); err != nil {
		t.Errorf("expected no error for valid config, got: %s", err)
	}
}

func TestValidate_EmptyTaskEntry(t *testing.T) {
	tasks := map[string]lib.Task{
		"pom.xml": {},
	}

	err := Validate(tasks, ".sdlc.json")
	if err == nil {
		t.Fatal("expected error for empty task entry, got nil")
	}

	if !strings.Contains(err.Error(), `task "pom.xml" has no commands defined`) {
		t.Errorf("error should mention the empty task; got: %s", err)
	}
}

func TestValidate_CustomNameCollisionWithBuiltIn(t *testing.T) {
	tasks := map[string]lib.Task{
		"main.go": {
			Run: "go run .",
			Custom: map[string]string{
				"build": "custom build command",
			},
		},
	}

	err := Validate(tasks, ".sdlc.json")
	if err == nil {
		t.Fatal("expected error for custom name collision, got nil")
	}

	if !strings.Contains(err.Error(), `custom action "build" that conflicts with a built-in action`) {
		t.Errorf("error should mention the collision; got: %s", err)
	}
}

func TestValidate_EmptyCustomCommandString(t *testing.T) {
	tasks := map[string]lib.Task{
		"main.go": {
			Run: "go run .",
			Custom: map[string]string{
				"deploy": "   ",
			},
		},
	}

	err := Validate(tasks, ".sdlc.json")
	if err == nil {
		t.Fatal("expected error for empty custom command, got nil")
	}

	if !strings.Contains(err.Error(), `custom action "deploy" with an empty command`) {
		t.Errorf("error should mention the empty custom command; got: %s", err)
	}
}

func TestValidate_EmptyHookCommandString(t *testing.T) {
	tasks := map[string]lib.Task{
		"main.go": {
			Run: "go run .",
			Hooks: lib.TaskHooks{
				Pre: map[string]string{
					"test": "",
				},
				Post: map[string]string{
					"build": "  ",
				},
			},
		},
	}

	err := Validate(tasks, ".sdlc.json")
	if err == nil {
		t.Fatal("expected error for empty hook command, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, `pre-hook for action "test" with an empty command`) {
		t.Errorf("error should mention empty pre-hook command; got: %s", errMsg)
	}
	if !strings.Contains(errMsg, `post-hook for action "build" with an empty command`) {
		t.Errorf("error should mention empty post-hook command; got: %s", errMsg)
	}
}

func TestValidate_HookReferencingUndefinedAction(t *testing.T) {
	tasks := map[string]lib.Task{
		"main.go": {
			Run: "go run .",
			Hooks: lib.TaskHooks{
				Pre: map[string]string{
					"nonexistent": "echo 'pre'",
				},
				Post: map[string]string{
					"missing": "echo 'post'",
				},
			},
		},
	}

	err := Validate(tasks, ".sdlc.json")
	if err == nil {
		t.Fatal("expected error for hook referencing undefined action, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, `pre-hook referencing undefined action "nonexistent"`) {
		t.Errorf("error should mention undefined pre-hook action; got: %s", errMsg)
	}
	if !strings.Contains(errMsg, `post-hook referencing undefined action "missing"`) {
		t.Errorf("error should mention undefined post-hook action; got: %s", errMsg)
	}
}

func TestValidate_HookReferencingCustomAction(t *testing.T) {
	tasks := map[string]lib.Task{
		"main.go": {
			Run: "go run .",
			Custom: map[string]string{
				"deploy": "mvn deploy",
			},
			Hooks: lib.TaskHooks{
				Pre: map[string]string{
					"deploy": "echo 'pre-deploy'",
				},
			},
		},
	}

	if err := Validate(tasks, ".sdlc.json"); err != nil {
		t.Errorf("expected no error when hook references custom action, got: %s", err)
	}
}

func TestValidate_MultipleSimultaneousErrors(t *testing.T) {
	tasks := map[string]lib.Task{
		"pom.xml": {}, // empty task
		"main.go": {
			Run: "go run .",
			Custom: map[string]string{
				"test":  "custom test",  // collides with built-in
				"debug": "",            // empty command
			},
			Hooks: lib.TaskHooks{
				Pre: map[string]string{
					"unknown": "echo 'pre'", // undefined action
					"run":     "",           // empty command
				},
			},
		},
	}

	err := Validate(tasks, ".sdlc.json")
	if err == nil {
		t.Fatal("expected error for multiple validation issues, got nil")
	}

	errMsg := err.Error()

	// Check all expected errors are present
	expected := []string{
		`task "pom.xml" has no commands defined`,
		`custom action "test" that conflicts with a built-in action`,
		`custom action "debug" with an empty command`,
		`pre-hook referencing undefined action "unknown"`,
		`pre-hook for action "run" with an empty command`,
	}

	for _, exp := range expected {
		if !strings.Contains(errMsg, exp) {
			t.Errorf("expected error to contain %q, but got:\n%s", exp, errMsg)
		}
	}

	// Ensure we got all 5 errors (each expected string on its own line)
	lines := strings.Count(errMsg, "\n") + 1
	if lines != len(expected) {
		t.Errorf("expected %d error lines, got %d:\n%s", len(expected), lines, errMsg)
	}
}

func TestValidate_EmptyTasksMap(t *testing.T) {
	tasks := map[string]lib.Task{}

	if err := Validate(tasks, ".sdlc.json"); err != nil {
		t.Errorf("expected no error for empty tasks map, got: %s", err)
	}
}

func TestValidate_NilCustomAndHooks(t *testing.T) {
	tasks := map[string]lib.Task{
		"main.go": {
			Run:  "go run .",
			Test: "go test ./...",
		},
	}

	if err := Validate(tasks, ".sdlc.json"); err != nil {
		t.Errorf("expected no error when custom/hooks are nil, got: %s", err)
	}
}

func TestValidate_AllBuiltInActionsInHooks(t *testing.T) {
	tasks := map[string]lib.Task{
		"main.go": {
			Run:   "go run .",
			Build: "go build .",
			Hooks: lib.TaskHooks{
				Pre: map[string]string{
					"run":   "echo 'pre-run'",
					"build": "echo 'pre-build'",
					"test":  "echo 'pre-test'",
				},
				Post: map[string]string{
					"install": "echo 'post-install'",
					"clean":   "echo 'post-clean'",
				},
			},
		},
	}

	if err := Validate(tasks, ".sdlc.json"); err != nil {
		t.Errorf("expected no error for hooks referencing built-in actions, got: %s", err)
	}
}

func TestValidate_FilePathInErrorMessage(t *testing.T) {
	tasks := map[string]lib.Task{
		"pom.xml": {},
	}

	err := Validate(tasks, "/some/path/.sdlc.json")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "/some/path/.sdlc.json") {
		t.Errorf("error should include the file path; got: %s", err)
	}
}

func TestIsEmptyTask(t *testing.T) {
	tests := []struct {
		name  string
		task  lib.Task
		empty bool
	}{
		{
			name:  "fully empty",
			task:  lib.Task{},
			empty: true,
		},
		{
			name:  "has run command",
			task:  lib.Task{Run: "go run ."},
			empty: false,
		},
		{
			name:  "has custom action",
			task:  lib.Task{Custom: map[string]string{"deploy": "mvn deploy"}},
			empty: false,
		},
		{
			name:  "has pre hook",
			task:  lib.Task{Hooks: lib.TaskHooks{Pre: map[string]string{"build": "echo"}}},
			empty: false,
		},
		{
			name:  "has post hook",
			task:  lib.Task{Hooks: lib.TaskHooks{Post: map[string]string{"build": "echo"}}},
			empty: false,
		},
		{
			name:  "nil custom and hooks",
			task:  lib.Task{Custom: nil, Hooks: lib.TaskHooks{Pre: nil, Post: nil}},
			empty: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isEmptyTask(tc.task)
			if got != tc.empty {
				t.Errorf("isEmptyTask() = %v, want %v", got, tc.empty)
			}
		})
	}
}
