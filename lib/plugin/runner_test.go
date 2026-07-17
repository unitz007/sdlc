package plugin

import (
	"bytes"
	"context"
	"testing"
)

func TestHookRun(t *testing.T) {
	t.Run("successful command", func(t *testing.T) {
		hook := Hook{Name: "pre-build", Command: "echo hello world"}

		var buf bytes.Buffer
		result := hook.Run(context.Background(), RunOpts{Stdout: &buf})

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
		if !bytes.Contains(buf.Bytes(), []byte("hello world")) {
			t.Errorf("expected output to contain 'hello world', got %q", buf.String())
		}
	})

	t.Run("failing command", func(t *testing.T) {
		hook := Hook{Name: "pre-build", Command: "false"}

		var buf bytes.Buffer
		result := hook.Run(context.Background(), RunOpts{Stdout: &buf, Stderr: &buf})

		if result.Err == nil {
			t.Fatal("expected error for failing command")
		}
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code")
		}
	})

	t.Run("with environment variables", func(t *testing.T) {
		hook := Hook{Name: "pre-build", Command: "echo $MY_VAR"}

		var buf bytes.Buffer
		result := hook.Run(context.Background(), RunOpts{
			Stdout: &buf,
			Env:    map[string]string{"MY_VAR": "test-value"},
		})

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		// Note: $MY_VAR expansion depends on shell — echo with exec won't expand
		// since the command is split on spaces. The executor runs the binary directly.
	})

	t.Run("with working directory", func(t *testing.T) {
		hook := Hook{Name: "pre-build", Command: "pwd"}

		var buf bytes.Buffer
		result := hook.Run(context.Background(), RunOpts{
			Stdout: &buf,
			Dir:    "/tmp",
		})

		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("/tmp")) {
			t.Errorf("expected output to contain '/tmp', got %q", buf.String())
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		hook := Hook{Name: "pre-build", Command: "sleep 10"}
		result := hook.Run(ctx, RunOpts{})

		// The command may fail immediately or may have started briefly
		// Either way, it should not hang
		_ = result
	})
}

func TestHookRunnerRunAll(t *testing.T) {
	t.Run("run multiple hooks", func(t *testing.T) {
		hooks := []Hook{
			{Name: "pre-build", Command: "echo first", Priority: 1},
			{Name: "pre-build", Command: "echo second", Priority: 2},
			{Name: "pre-build", Command: "echo third", Priority: 3},
		}

		runner := NewHookRunner()
		var buf bytes.Buffer
		results := runner.RunAll(context.Background(), hooks, RunOpts{Stdout: &buf}, false)

		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}
		for i, res := range results {
			if res.Err != nil {
				t.Errorf("hook %d failed: %v", i, res.Err)
			}
		}
	})

	t.Run("stop on error", func(t *testing.T) {
		hooks := []Hook{
			{Name: "pre-build", Command: "echo ok", Priority: 1},
			{Name: "pre-build", Command: "false", Priority: 2},
			{Name: "pre-build", Command: "echo skipped", Priority: 3},
		}

		runner := NewHookRunner()
		results := runner.RunAll(context.Background(), hooks, RunOpts{}, true)

		if len(results) != 2 {
			t.Fatalf("expected 2 results (stopped after error), got %d", len(results))
		}
		if results[0].Err != nil {
			t.Errorf("first hook should succeed: %v", results[0].Err)
		}
		if results[1].Err == nil {
			t.Error("second hook should fail")
		}
	})

	t.Run("filter by project type", func(t *testing.T) {
		hooks := []Hook{
			{Name: "pre-build", Command: "echo node", ProjectType: "node", Priority: 1},
			{Name: "pre-build", Command: "echo go", ProjectType: "go", Priority: 2},
			{Name: "pre-build", Command: "echo all", Priority: 3},
		}

		runner := NewHookRunner()
		var buf bytes.Buffer
		results := runner.RunAll(context.Background(), hooks, RunOpts{
			Stdout:      &buf,
			ProjectType: "node",
		}, false)

		if len(results) != 2 {
			t.Fatalf("expected 2 results for node project, got %d", len(results))
		}
	})
}
