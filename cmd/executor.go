package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sdlc/lib"
)

func runCommand(ctx context.Context, commandStr, dir string, stdout, stderr io.Writer, env map[string]string) error {
	executor := lib.NewExecutor(ctx, commandStr)
	if dir != "" {
		executor.SetDir(dir)
	}
	if stdout != nil {
		executor.SetOutput(stdout, stderr)
	}
	if env != nil {
		executor.SetEnv(env)
	}
	if err := executor.Execute(); err != nil {
		// Propagate the child process exit code so the CLI exits with it
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &ExitCodeError{Code: exitErr.ExitCode(), Err: err}
		}
		return fmt.Errorf("command execution failed: %w", err)
	}
	return nil
}
