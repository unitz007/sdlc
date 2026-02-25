package cmd

import (
	"context"
	"fmt"
	"io"
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
