package lib

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewExecutor_SingleWord(t *testing.T) {
	executor := NewExecutor(context.Background(), "echo")
	if executor == nil {
		t.Fatal(`NewExecutor("echo") returned nil`)
	}
	if executor.cmd == nil {
		t.Fatal(`NewExecutor("echo").cmd is nil`)
	}
}

func TestNewExecutor_MultiWord(t *testing.T) {
	executor := NewExecutor(context.Background(), "echo hello world")
	if executor == nil {
		t.Fatal(`NewExecutor("echo hello world") returned nil`)
	}
	if executor.cmd == nil {
		t.Fatal(`NewExecutor("echo hello world").cmd is nil`)
	}
}

func TestNewExecutor_CommandParsing(t *testing.T) {
	executor := NewExecutor(context.Background(), "go build -v")
	if executor.cmd.Path == "" {
		t.Error("expected cmd.Path to be set")
	}
	args := executor.cmd.Args
	// With sh -c construction, args are ["sh", "-c", "go build -v"]
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(args), args)
	}
	if args[0] != "sh" {
		t.Errorf("args[0] = %q, want %q", args[0], "sh")
	}
	if args[1] != "-c" {
		t.Errorf("args[1] = %q, want %q", args[1], "-c")
	}
	if args[2] != "go build -v" {
		t.Errorf("args[2] = %q, want %q", args[2], "go build -v")
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

// --- Shell syntax tests ---

func TestExecute_PipeCommand(t *testing.T) {
	var buf bytes.Buffer
	executor := NewExecutor(context.Background(), "echo hello | grep h")
	executor.SetOutput(&buf, &buf)
	err := executor.Execute()
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "hello" {
		t.Errorf("pipe command output = %q, want %q", got, "hello")
	}
}

func TestExecute_RedirectCommand(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "sdlc_test_redirect.txt")
	executor := NewExecutor(context.Background(), "echo hello > "+outPath)
	err := executor.Execute()
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read redirect output file: %v", err)
	}
	got := strings.TrimSpace(string(data))
	if got != "hello" {
		t.Errorf("redirect file content = %q, want %q", got, "hello")
	}
}

func TestExecute_SubshellCommand(t *testing.T) {
	var buf bytes.Buffer
	executor := NewExecutor(context.Background(), "echo $(echo nested)")
	executor.SetOutput(&buf, &buf)
	err := executor.Execute()
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "nested" {
		t.Errorf("subshell command output = %q, want %q", got, "nested")
	}
}

func TestExecute_QuotedArguments(t *testing.T) {
	var buf bytes.Buffer
	executor := NewExecutor(context.Background(), `echo "hello world"`)
	executor.SetOutput(&buf, &buf)
	err := executor.Execute()
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "hello world" {
		t.Errorf("quoted args output = %q, want %q", got, "hello world")
	}
}

func TestExecute_SimpleCommandNoRegression(t *testing.T) {
	var buf bytes.Buffer
	executor := NewExecutor(context.Background(), "echo hello")
	executor.SetOutput(&buf, &buf)
	err := executor.Execute()
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "hello" {
		t.Errorf("simple command output = %q, want %q", got, "hello")
	}
}

func TestExecute_AndOrOperators(t *testing.T) {
	// Test && operator
	var buf bytes.Buffer
	executor := NewExecutor(context.Background(), "echo ok && echo yes")
	executor.SetOutput(&buf, &buf)
	err := executor.Execute()
	if err != nil {
		t.Fatalf("Execute() with && returned unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "ok\nyes" {
		t.Errorf("&& command output = %q, want %q", got, "ok\nyes")
	}

	// Test || operator (first command fails, second succeeds)
	buf.Reset()
	executor = NewExecutor(context.Background(), "false || echo fallback")
	executor.SetOutput(&buf, &buf)
	err = executor.Execute()
	if err != nil {
		t.Fatalf("Execute() with || returned unexpected error: %v", err)
	}
	got = strings.TrimSpace(buf.String())
	if got != "fallback" {
		t.Errorf("|| command output = %q, want %q", got, "fallback")
	}
}

// --- SetDir and SetEnv tests ---

func TestNewExecutor_SetDir(t *testing.T) {
	executor := NewExecutor(context.Background(), "pwd")
	dir := "/tmp"
	executor.SetDir(dir)
	if executor.cmd.Dir != dir {
		t.Errorf("cmd.Dir = %q, want %q", executor.cmd.Dir, dir)
	}
}

func TestNewExecutor_SetEnv(t *testing.T) {
	executor := NewExecutor(context.Background(), "echo test")
	executor.SetEnv(map[string]string{"FOO": "bar"})
	found := false
	for _, entry := range executor.cmd.Env {
		if entry == "FOO=bar" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected FOO=bar in cmd.Env, not found")
	}
}

func TestNewExecutor_SysProcAttr(t *testing.T) {
	executor := NewExecutor(context.Background(), "echo test")
	if executor.cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set")
	}
	if !executor.cmd.SysProcAttr.Setpgid {
		t.Error("expected Setpgid to be true")
	}
}
