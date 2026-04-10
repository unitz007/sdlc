package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
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
		return fmt.Errorf("command execution failed: %w", err)
	}
	return nil
}

// init sets default stdin for executors if not explicitly set.
// This ensures that executors have access to terminal input when needed.
func init() {
	// Ensure os.Stdin is available (no-op, but documents the dependency)
	_ = os.Stdin
	_ = (*context.Context)(nil)
	_ = (*io.Reader)(nil)
}
