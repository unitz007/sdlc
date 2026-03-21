package lib

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Executor wraps an os/exec.Cmd to run a shell command, stream its combined
// stdout/stderr output, and handle OS interrupt signals gracefully.
type Executor struct {
	cmd      *exec.Cmd
	Stdout   io.Writer
	Stderr   io.Writer
	Stdin    io.Reader
	exitCode int
}

// NewExecutor creates a new Executor for the given command string. The command
// is split on spaces — the first token is used as the program name and the
// remaining tokens as arguments.
func NewExecutor(ctx context.Context, command string) *Executor {
	program := strings.Split(command, " ")[0]
	// Use CommandContext for cancellation support
	cmd := exec.CommandContext(ctx, program, strings.Split(command, " ")[1:]...)

	// Create a new process group for proper signal handling
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Kill the entire process group on cancellation
	cmd.Cancel = func() error {
		// Only try to kill if process is started
		if cmd.Process != nil {
			// Send SIGTERM to allow graceful cleanup
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		}
		return nil
	}

	return &Executor{
		cmd:    cmd,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
	}
}

// SetDir sets the working directory for the command.
func (e *Executor) SetDir(dir string) {
	e.cmd.Dir = dir
}

// SetEnv sets the environment variables for the command.
func (e *Executor) SetEnv(env map[string]string) {
	if e.cmd.Env == nil {
		e.cmd.Env = os.Environ()
	}

	for k, v := range env {
		e.cmd.Env = append(e.cmd.Env, k+"="+v)
	}
}

// SetOutput sets the stdout and stderr writers for the command.
func (e *Executor) SetOutput(stdout, stderr io.Writer) {
	e.Stdout = stdout
	e.Stderr = stderr
}

// Execute starts the underlying command, streams its combined stdout and stderr
// output to the console in real-time, and listens for SIGINT/SIGTERM signals to
// handle graceful shutdown.
func (e *Executor) Execute() error {
	e.cmd.Stdout = e.Stdout
	e.cmd.Stderr = e.Stderr
	e.cmd.Stdin = e.Stdin

	if err := e.cmd.Start(); err != nil {
		e.exitCode = 1
		return err
	}

	err := e.cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					e.exitCode = 128 + int(status.Signal())
				} else {
					e.exitCode = status.ExitStatus()
				}
			} else {
				e.exitCode = 1
			}
		} else {
			e.exitCode = 1
		}
		return err
	}

	e.exitCode = 0
	return nil
}

// ExitCode returns the exit code of the last executed command.
// Returns 0 if the command succeeded, the process exit code on failure,
// or 1 if the command failed to start or was cancelled.
func (e *Executor) ExitCode() int {
	return e.exitCode
}
