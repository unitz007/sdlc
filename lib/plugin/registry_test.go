package plugin

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()

	p := Plugin{
		Name:        "my-plugin",
		ProjectType: "node",
		Hooks: []Hook{
			{Name: "pre-build", Command: "npm run lint", Description: "Run linter"},
			{Name: "post-build", Command: "npm run deploy", Priority: 5},
		},
	}

	if err := r.Register(p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	plugins := r.Plugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name != "my-plugin" {
		t.Errorf("expected plugin name %q, got %q", "my-plugin", plugins[0].Name)
	}
	if len(plugins[0].Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(plugins[0].Hooks))
	}
}

func TestRegistryRegisterInvalid(t *testing.T) {
	r := NewRegistry()

	p := Plugin{
		Name: "bad-plugin",
		Hooks: []Hook{
			{Name: "pre-build", Command: ""}, // missing command
		},
	}

	if err := r.Register(p); err == nil {
		t.Error("expected error for invalid hook")
	}

	// Registry should be unchanged
	if len(r.Plugins()) != 0 {
		t.Error("registry should be empty after failed registration")
	}
}

func TestRegistryRegisterHook(t *testing.T) {
	r := NewRegistry()

	h := Hook{Name: "pre-build", Command: "go vet ./..."}
	if err := r.RegisterHook("go-tools", h); err != nil {
		t.Fatalf("RegisterHook() error = %v", err)
	}

	// Adding to existing plugin
	h2 := Hook{Name: "post-test", Command: "go cover ./..."}
	if err := r.RegisterHook("go-tools", h2); err != nil {
		t.Fatalf("RegisterHook() error = %v", err)
	}

	plugins := r.Plugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if len(plugins[0].Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(plugins[0].Hooks))
	}
}

func TestRegistryGetHooks(t *testing.T) {
	r := NewRegistry()

	r.Register(Plugin{
		Name: "p1",
		Hooks: []Hook{
			{Name: "pre-build", Command: "echo p1-pre", Priority: 5},
			{Name: "post-build", Command: "echo p1-post"},
		},
	})
	r.Register(Plugin{
		Name: "p2",
		ProjectType: "node",
		Hooks: []Hook{
			{Name: "pre-build", Command: "echo p2-pre", Priority: 1},
		},
	})

	t.Run("by name only", func(t *testing.T) {
		hooks := r.GetHooks("pre-build", "")
		if len(hooks) != 2 {
			t.Fatalf("expected 2 hooks, got %d", len(hooks))
		}
		// Should be sorted by priority: p2 (1) then p1 (5)
		if hooks[0].Command != "echo p2-pre" {
			t.Errorf("expected first hook from p2, got %q", hooks[0].Command)
		}
	})

	t.Run("by name and project type", func(t *testing.T) {
		hooks := r.GetHooks("pre-build", "node")
		if len(hooks) != 1 {
			t.Fatalf("expected 1 hook, got %d", len(hooks))
		}
		if hooks[0].Command != "echo p2-pre" {
			t.Errorf("expected hook from p2, got %q", hooks[0].Command)
		}
	})

	t.Run("nonexistent name", func(t *testing.T) {
		hooks := r.GetHooks("nonexistent", "")
		if len(hooks) != 0 {
			t.Errorf("expected 0 hooks, got %d", len(hooks))
		}
	})
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()

	r.Register(Plugin{
		Name: "p1",
		Hooks: []Hook{
			{Name: "post-test", Command: "echo"},
			{Name: "pre-build", Command: "echo"},
		},
	})
	r.Register(Plugin{
		Name: "p2",
		Hooks: []Hook{
			{Name: "pre-build", Command: "echo"},
		},
	})

	names := r.List()
	if len(names) != 2 {
		t.Fatalf("expected 2 unique names, got %d", len(names))
	}
	if names[0] != "pre-build" || names[1] != "post-test" {
		t.Errorf("expected [pre-build, post-test], got %v", names)
	}
}

func TestRegistryUnregister(t *testing.T) {
	r := NewRegistry()
	r.Register(Plugin{Name: "p1", Hooks: []Hook{{Name: "pre-build", Command: "echo"}}})
	r.Register(Plugin{Name: "p2", Hooks: []Hook{{Name: "pre-test", Command: "echo"}}})

	if !r.Unregister("p1") {
		t.Error("Unregister should return true for existing plugin")
	}
	if r.Unregister("nonexistent") {
		t.Error("Unregister should return false for missing plugin")
	}
	if len(r.Plugins()) != 1 {
		t.Errorf("expected 1 plugin after unregister, got %d", len(r.Plugins()))
	}
}

func TestRegistryRemoveHook(t *testing.T) {
	r := NewRegistry()
	r.Register(Plugin{
		Name: "p1",
		Hooks: []Hook{
			{Name: "pre-build", Command: "lint"},
			{Name: "post-build", Command: "deploy"},
		},
	})

	if !r.RemoveHook("p1", "pre-build") {
		t.Error("RemoveHook should return true")
	}
	if r.RemoveHook("p1", "nonexistent") {
		t.Error("RemoveHook should return false for missing hook")
	}
	hooks := r.GetHooks("post-build", "")
	if len(hooks) != 1 {
		t.Errorf("expected 1 remaining hook, got %d", len(hooks))
	}
}

func TestRegistryClear(t *testing.T) {
	r := NewRegistry()
	r.Register(Plugin{Name: "p1", Hooks: []Hook{{Name: "pre-build", Command: "echo"}}})
	r.Clear()
	if len(r.Plugins()) != 0 {
		t.Error("registry should be empty after Clear()")
	}
}

func TestRegistryRun(t *testing.T) {
	r := NewRegistry()

	r.Register(Plugin{
		Name: "echo-plugin",
		Hooks: []Hook{
			{Name: "pre-build", Command: "echo hello", Priority: 1},
			{Name: "pre-build", Command: "echo world", Priority: 2},
		},
	})

	var buf bytes.Buffer
	results := r.Run(context.Background(), "pre-build", RunOpts{
		Stdout: &buf,
	})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, res := range results {
		if res.Err != nil {
			t.Errorf("hook %q failed: %v", res.Hook.Command, res.Err)
		}
	}

	output := buf.String()
	if !strings.Contains(output, "hello") || !strings.Contains(output, "world") {
		t.Errorf("expected output to contain 'hello' and 'world', got %q", output)
	}
}

func TestRegistryRunWithStopOnError(t *testing.T) {
	r := NewRegistry()

	r.Register(Plugin{
		Name: "mixed-plugin",
		Hooks: []Hook{
			{Name: "pre-build", Command: "echo first", Priority: 1},
			{Name: "pre-build", Command: "false", Priority: 2}, // will fail
			{Name: "pre-build", Command: "echo third", Priority: 3},
		},
	})

	var buf bytes.Buffer
	results := r.RunWithStopOnError(context.Background(), "pre-build", RunOpts{
		Stdout: &buf,
	})

	// Should have stopped after the failing hook
	if len(results) != 2 {
		t.Fatalf("expected 2 results (stopped after failure), got %d", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("first hook should succeed: %v", results[0].Err)
	}
	if results[1].Err == nil {
		t.Error("second hook should fail")
	}
}
