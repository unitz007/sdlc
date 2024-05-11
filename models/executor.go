package models

import (
	"bufio"
	"fmt"
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
	_, err = e.cmd.StderrPipe()

	//cmdOutput := io.MultiReader(stdOutput)

	if err != nil {
		return err
	}

	if err = e.cmd.Start(); err != nil {
		return err
	}

	bufOutput := bufio.NewReader(stdOutput)
	line, err := bufOutput.ReadString('\n')
	for err == nil {
		fmt.Print(line)
		line, err = bufOutput.ReadString('\n')
	}

	return nil
}
