package main

import (
	"io/ioutil"
	"os"
	"sdlc/io"
	"sdlc/models"
	"strings"

	"github.com/spf13/cobra"
)

func main() {

	var (
		argCommand string
		extraArgs  string
	)

	// CLI arguments
	rootCmd := io.NewCommand("sdlc", "", func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			_ = cmd.Help()
			return
		}
	}).Cmd

	subCommands := []struct {
		command     string
		description string
		action      func(cmd *cobra.Command, args []string)
	}{
		{
			"run",
			"Runs your code",
			func(cmd *cobra.Command, args []string) {
				argCommand = "run"
				extraArgs = cmd.Flag("extraArgs").Value.String()
			},
		}, {
			"test",
			"Tests your code",
			func(cmd *cobra.Command, args []string) {
				argCommand = "test"
				extraArgs = cmd.Flag("extraArgs").Value.String()
			},
		}, {
			"build",
			"Builds your project",
			func(cmd *cobra.Command, args []string) {
				argCommand = "build"
				extraArgs = cmd.Flag("extraArgs").Value.String()
			},
		},
	}

	for _, subCommand := range subCommands {
		s := io.NewCommand(subCommand.command, subCommand.description, subCommand.action)
		rootCmd.AddCommand(s.Cmd)
	}

	if err := rootCmd.Execute(); err != nil || argCommand == "" {
		_ = rootCmd.Help()
		return
	}

	buildData := io.GetBuilds()

	var command strings.Builder
	workingDirectory, _ := os.Getwd()

	for buildFile, task := range buildData {

		filesInWorkingDirectory, _ := ioutil.ReadDir(workingDirectory)

		for _, file := range filesInWorkingDirectory {
			if file.Name() == buildFile {
				io.Print("Build file found: " + file.Name())
				command.WriteString(task.Command(argCommand))
			}
		}
	}

	if command.String() == "" {
		io.Print("No project configured")
		return
	}

	if extraArgs != "" {
		command.WriteString(" " + extraArgs)
	}

	execute := models.NewExecutor(command.String())
	if err := execute.Execute(); err != nil {
		io.Print("Error executing command: " + err.Error())
		return
	}

	io.Print("Completed")
}

type ConfigFile struct {
	data string
}
