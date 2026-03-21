package lib

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewExecutor_SingleWord(t *testing.T) {
	executor := NewExecutor(context.Background(), "echo")
	if executor == nil {
		t.Fatal("NewExecutor(\"echo\") returned nil")
	}
	if executor.cmd == nil {
		t.Fatal("NewExecutor(\"echo\").cmd is nil")
	}
}

func TestNewExecutor_MultiWord(t *testing.T) {
	executor := NewExecutor(context.Background(), "echo hello world")
	if executor == nil {
		t.Fatal("NewExecutor(\"echo hello world\") returned nil")
	}
	if executor.cmd == nil {
		t.Fatal("NewExecutor(\"echo hello world\").cmd is nil")
	}
}

func TestNewExecutor_CommandParsing(t *testing.T) {
	executor := NewExecutor(context.Background(), "go build -v")
	if executor.cmd.Path == "" {
		t.Error("expected cmd.Path to be set")
	}
	args := executor.cmd.Args
	// Args[0] is the program name, Args[1:] are the arguments
	if len(args) < 3 {
		t.Fatalf("expected at least 3 args, got %d: %v", len(args), args)
	}
	if args[1] != "build" {
		t.Errorf("args[1] = %q, want %q", args[1], "build")
	}
	if args[2] != "-v" {
		t.Errorf("args[2] = %q, want %q", args[2], "-v")
	}
}

func TestExecute_Success(t *testing.T) {
	executor := NewExecutor(context.Background(), "echo hello")
	err := executor.Execute()
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
}

func TestExecute_InvalidProgram(t *testing.T) {
	executor := NewExecutor(context.Background(), "nonexistent_binary_xyz")
	err := executor.Execute()
	if err == nil {
		t.Fatal("Execute() with invalid program expected error, got nil")
	}
}

func TestExecute_ExitCode_Success(t *testing.T) {
	executor := NewExecutor(context.Background(), "echo hello")
	err := executor.Execute()
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	if code := executor.ExitCode(); code != 0 {
		t.Errorf("ExitCode() = %d, want 0", code)
	}
}

func TestExecute_ExitCode_Failure(t *testing.T) {
	// Create a temp script that exits with code 42 (avoids space-splitting issues)
	script := filepath.Join(t.TempDir(), "exit42.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 42\n"), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}
	executor := NewExecutor(context.Background(), script)
	err := executor.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for exit 42, got nil")
	}
	if code := executor.ExitCode(); code != 42 {
		t.Errorf("ExitCode() = %d, want 42", code)
	}
}

func TestExecute_ExitCode_DefaultOnStartError(t *testing.T) {
	executor := NewExecutor(context.Background(), "nonexistent_binary_xyz")
	err := executor.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for nonexistent binary, got nil")
	}
	if code := executor.ExitCode(); code != 1 {
		t.Errorf("ExitCode() = %d, want 1 (default for start errors)", code)
	}
}

func TestExecute_ExitCode_SignalDeath(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("skipping signal death test on %s", runtime.GOOS)
	}
	// Create a temp script that kills itself with SIGKILL
	script := filepath.Join(t.TempDir(), "killself.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nkill -9 $$\n"), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}
	executor := NewExecutor(context.Background(), script)
	err := executor.Execute()
	if err == nil {
		t.Fatal("Execute() expected error for signal-killed process, got nil")
	}
	code := executor.ExitCode()
	// On Unix, a process killed by signal 9 (SIGKILL) reports -9 via WaitStatus
	// (negative on macOS, also negative on Linux).
	if code >= 0 {
		t.Errorf("ExitCode() = %d, want negative (signal convention) for signal-killed process", code)
	}
	t.Logf("Signal death exit code: %d", code)
}
