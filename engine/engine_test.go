package engine

import (
	"testing"

	"sdlc/lib"
)

func TestMergeTasks_BuiltInOverride(t *testing.T) {
	global := lib.Task{
		Run:   "go run main.go",
		Build: "go build -v",
	}
	local := lib.Task{
		Run:   "go run cmd/server/main.go",
		Build: "go build -o bin/server",
	}

	merged := mergeTasks(global, local)
	if merged.Run != "go run cmd/server/main.go" {
		t.Errorf("merged.Run = %q, want %q", merged.Run, "go run cmd/server/main.go")
	}
	if merged.Build != "go build -o bin/server" {
		t.Errorf("merged.Build = %q, want %q", merged.Build, "go build -o bin/server")
	}
}

func TestMergeTasks_CustomActions(t *testing.T) {
	global := lib.Task{
		Custom: map[string]string{
			"lint":    "golangci-lint run",
			"deploy":  "kubectl apply -f k8s/",
		},
	}
	local := lib.Task{
		Custom: map[string]string{
			"deploy":  "kubectl apply -f k8s/overridden/",
			"migrate": "go run migrations/main.go",
		},
	}

	merged := mergeTasks(global, local)

	if merged.Custom["lint"] != "golangci-lint run" {
		t.Errorf("merged.Custom[lint] = %q, want %q", merged.Custom["lint"], "golangci-lint run")
	}
	if merged.Custom["deploy"] != "kubectl apply -f k8s/overridden/" {
		t.Errorf("merged.Custom[deploy] = %q, want %q", merged.Custom["deploy"], "kubectl apply -f k8s/overridden/")
	}
	if merged.Custom["migrate"] != "go run migrations/main.go" {
		t.Errorf("merged.Custom[migrate] = %q, want %q", merged.Custom["migrate"], "go run migrations/main.go")
	}
}

func TestMergeTasks_Hooks(t *testing.T) {
	global := lib.Task{
		Hooks: lib.TaskHooks{
			Pre: map[string]string{
				"build": "echo global pre-build",
			},
			Post: map[string]string{
				"run": "echo global post-run",
			},
		},
	}
	local := lib.Task{
		Hooks: lib.TaskHooks{
			Pre: map[string]string{
				"build": "echo local pre-build",
				"test":  "echo local pre-test",
			},
			Post: map[string]string{
				"run":  "echo local post-run",
				"test": "echo local post-test",
			},
		},
	}

	merged := mergeTasks(global, local)

	if merged.Hooks.Pre["build"] != "echo local pre-build" {
		t.Errorf("merged.Hooks.Pre[build] = %q, want %q", merged.Hooks.Pre["build"], "echo local pre-build")
	}
	if merged.Hooks.Pre["test"] != "echo local pre-test" {
		t.Errorf("merged.Hooks.Pre[test] = %q, want %q", merged.Hooks.Pre["test"], "echo local pre-test")
	}
	if merged.Hooks.Post["run"] != "echo local post-run" {
		t.Errorf("merged.Hooks.Post[run] = %q, want %q", merged.Hooks.Post["run"], "echo local post-run")
	}
	if merged.Hooks.Post["test"] != "echo local post-test" {
		t.Errorf("merged.Hooks.Post[test] = %q, want %q", merged.Hooks.Post["test"], "echo local post-test")
	}
}

func TestMergeTasks_NilCustom(t *testing.T) {
	global := lib.Task{}
	local := lib.Task{
		Custom: map[string]string{
			"deploy": "kubectl apply",
		},
	}

	merged := mergeTasks(global, local)
	if merged.Custom["deploy"] != "kubectl apply" {
		t.Errorf("merged.Custom[deploy] = %q, want %q", merged.Custom["deploy"], "kubectl apply")
	}
}

func TestMergeTasks_Empty(t *testing.T) {
	merged := mergeTasks(lib.Task{}, lib.Task{})
	if merged.Run != "" || merged.Build != "" {
		t.Error("expected empty merge")
	}
}
