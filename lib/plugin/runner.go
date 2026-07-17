package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"sdlc/lib"
)

// RunOpts controls how a hook is executed.
type RunOpts struct {
	// Dir is the working directory for the hook command.
	Dir string

	// Stdout and Stderr receive the hook's output. If nil, os.Stdout / os.Stderr
	// are used.
	Stdout io.Writer
	Stderr io.Writer

	// Env is extra environment variables to pass to the hook command.
	Env map[string]string

	// ProjectType is the current project's type, used to filter hooks.
	ProjectType string
}

// RunResult captures the outcome of a single hook execution.
type RunResult struct {
	// Hook is the hook that was executed.
	Hook Hook

	// ExitCode is the process exit code (0 = success).
	ExitCode int

	// Err is non-nil if the hook failed to start or was killed.
	Err error
}

// Run executes the hook using the given options and returns a RunResult.
func (h Hook) Run(ctx context.Context, opts RunOpts) RunResult {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	executor := lib.NewExecutor(ctx, h.Command)
	if opts.Dir != "" {
		executor.SetDir(opts.Dir)
	}
	executor.SetOutput(stdout, stderr)
	if opts.Env != nil {
		executor.SetEnv(opts.Env)
	}

	err := executor.Execute()
	result := RunResult{Hook: h}

	if err != nil {
		// Check if it's an exit error (non-zero exit code)
		result.Err = fmt.Errorf("hook %q failed: %w", h.Name, err)
		result.ExitCode = exitCodeFromError(err)
	} else {
		result.ExitCode = 0
	}

	return result
}

// exitCodeFromError extracts the exit code from an exec error if available.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	// os/exec.ExitError is handled by the executor; we return 1 as a default
	// for any non-nil error.
	return 1
}

// HookRunner executes multiple hooks sequentially, collecting results.
type HookRunner struct {
	mu sync.Mutex
}

// NewHookRunner creates a new HookRunner.
func NewHookRunner() *HookRunner {
	return &HookRunner{}
}

// RunAll executes all given hooks sequentially in priority order, collecting
// results. If stopOnError is true, execution stops after the first failure.
func (r *HookRunner) RunAll(ctx context.Context, hooks []Hook, opts RunOpts, stopOnError bool) []RunResult {
	sorted := SortHooks(hooks)
	results := make([]RunResult, 0, len(sorted))

	for _, hook := range sorted {
		// Apply project-type filter
		if !hook.MatchesProject(opts.ProjectType) {
			continue
		}

		result := hook.Run(ctx, opts)

		r.mu.Lock()
		results = append(results, result)
		r.mu.Unlock()

		if stopOnError && result.Err != nil {
			break
		}
	}

	return results
}
