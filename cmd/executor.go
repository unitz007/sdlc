package cmd

import (
	"context"
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
		code := executor.ExitCode()
		return &ExitCodeError{Code: code, Err: err}
	}
	return nil
}
