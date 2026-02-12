package lib

import (
	"testing"
)

func TestCommand_Run(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("run")
	if err != nil {
		t.Fatalf("Command(\"run\") returned unexpected error: %v", err)
	}
	if got != task.Run {
		t.Errorf("Command(\"run\") = %q, want %q", got, task.Run)
	}
}

func TestCommand_Test(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("test")
	if err != nil {
		t.Fatalf("Command(\"test\") returned unexpected error: %v", err)
	}
	if got != task.Test {
		t.Errorf("Command(\"test\") = %q, want %q", got, task.Test)
	}
}

func TestCommand_Build(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("build")
	if err != nil {
		t.Fatalf("Command(\"build\") returned unexpected error: %v", err)
	}
	if got != task.Build {
		t.Errorf("Command(\"build\") = %q, want %q", got, task.Build)
	}
}

func TestCommand_InvalidField(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("deploy")
	if err == nil {
		t.Fatal("Command(\"deploy\") expected error, got nil")
	}
	if got != "" {
		t.Errorf("Command(\"deploy\") = %q, want empty string", got)
	}
	if err.Error() != "invalid command" {
		t.Errorf("Command(\"deploy\") error = %q, want %q", err.Error(), "invalid command")
	}
}

func TestCommand_EmptyField(t *testing.T) {
	task := Task{
		Run:   "go run main.go",
		Test:  "go test .",
		Build: "go build -v",
	}

	got, err := task.Command("")
	if err == nil {
		t.Fatal("Command(\"\") expected error, got nil")
	}
	if got != "" {
		t.Errorf("Command(\"\") = %q, want empty string", got)
	}
}

func TestCommand_EmptyTask(t *testing.T) {
	task := Task{}

	got, err := task.Command("run")
	if err != nil {
		t.Fatalf("Command(\"run\") on empty task returned unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("Command(\"run\") on empty task = %q, want empty string", got)
	}
}
