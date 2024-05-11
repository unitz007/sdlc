package models

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Executor struct {
	cmd *exec.Cmd
}

func NewExecutor(command string) *Executor {
	program := strings.Split(command, " ")[0]
	cmd := exec.Command(program, strings.Split(command, " ")[1:]...)

	return &Executor{cmd: cmd}

}

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

	bufOutput := bufio.NewReader(cmdOutput)
	_ = bufio.NewReader(stdErr)
	line, err := bufOutput.ReadString('\n')
	for err == nil {
		fmt.Print(line)
		line, err = bufOutput.ReadString('\n')
	}

	return nil
}
