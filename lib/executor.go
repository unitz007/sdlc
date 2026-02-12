package lib

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// Executor wraps an os/exec.Cmd to run a shell command, stream its combined
// stdout/stderr output, and handle OS interrupt signals gracefully.
type Executor struct {
	cmd *exec.Cmd
}

// NewExecutor creates a new Executor for the given command string. The command
// is split on spaces — the first token is used as the program name and the
// remaining tokens as arguments.
func NewExecutor(command string) *Executor {
	program := strings.Split(command, " ")[0]
	cmd := exec.Command(program, strings.Split(command, " ")[1:]...)

	return &Executor{cmd: cmd}

}

// Execute starts the underlying command, streams its combined stdout and stderr
// output to the console in real-time, and listens for SIGINT/SIGTERM signals to
// handle graceful shutdown.
func (e *Executor) Execute() error {

	stdOutput, err := e.cmd.StdoutPipe()
	stdErr, err := e.cmd.StderrPipe()

	cmdOutput := io.MultiReader(stdOutput, stdErr)

	if err != nil {
		return err
	}

	if err = e.cmd.Start(); err != nil {
		return err
	}

	sig := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		_ = <-sig
		fmt.Printf("\nProject exited with ")
		done <- true
	}()

	bufOutput := bufio.NewReader(cmdOutput)
	line, err := bufOutput.ReadByte()
	for err == nil {
		fmt.Printf("%s", string(line))
		line, err = bufOutput.ReadByte()
	}

	return nil
}
