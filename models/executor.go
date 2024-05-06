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
	fmt.Println(e.cmd.String())

	pipedOutput, err := e.cmd.StdoutPipe()

	if err != nil {
		return err
	}

	if err = e.cmd.Start(); err != nil {
		return err
	}

	buf := bufio.NewReader(pipedOutput)
	line, err := buf.ReadString('\n')
	for err == nil {
		fmt.Print(line)
		line, err = buf.ReadString('\n')
	}

	return nil
}
