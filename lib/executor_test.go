package lib

import (
	"testing"
)

func TestNewExecutor_SingleWord(t *testing.T) {
	executor := NewExecutor("echo")
	if executor == nil {
		t.Fatal("NewExecutor(\"echo\") returned nil")
	}
	if executor.cmd == nil {
		t.Fatal("NewExecutor(\"echo\").cmd is nil")
	}
}

func TestNewExecutor_MultiWord(t *testing.T) {
	executor := NewExecutor("echo hello world")
	if executor == nil {
		t.Fatal("NewExecutor(\"echo hello world\") returned nil")
	}
	if executor.cmd == nil {
		t.Fatal("NewExecutor(\"echo hello world\").cmd is nil")
	}
}

func TestNewExecutor_CommandParsing(t *testing.T) {
	executor := NewExecutor("go build -v")
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
	executor := NewExecutor("echo hello")
	err := executor.Execute()
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
}

func TestExecute_InvalidProgram(t *testing.T) {
	executor := NewExecutor("nonexistent_binary_xyz")
	err := executor.Execute()
	if err == nil {
		t.Fatal("Execute() with invalid program expected error, got nil")
	}
}
