package main

import (
	"os"
	"sdlc/io"
	"sdlc/lib"
	"strings"

	"github.com/spf13/cobra"
)

func main() {

	var (
		argCommand       string
		extraArgs        string
		workingDirectory string
	)

	// CLI arguments
	rootCmd := io.NewCommand("sdlc", "", func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
      return
		}

		workingDirectory = cmd.Flag("dir").Value.String()
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
				workingDirectory = cmd.Flag("dir").Value.String()
			},
		}, {
			"test",
			"Tests your code",
			func(cmd *cobra.Command, args []string) {
				argCommand = "test"
				extraArgs = cmd.Flag("extraArgs").Value.String()
				workingDirectory = cmd.Flag("dir").Value.String()
			},
		}, {
			"build",
			"Builds your project",
			func(cmd *cobra.Command, args []string) {
				argCommand = "build"
				extraArgs = cmd.Flag("extraArgs").Value.String()
				workingDirectory = cmd.Flag("dir").Value.String()
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

	workingDirectory = func() string {
		if strings.HasPrefix(workingDirectory, "~/") {
			homeDir, _ := os.UserHomeDir()
			return strings.ReplaceAll(workingDirectory, "~", homeDir)
		}

		return workingDirectory
	}()

	if workingDirectory == "" {
		workingDirectory, _ = os.Getwd()
	}

	io.Print("Working Directory: " + workingDirectory)

	// change directory
	_ = os.Chdir(workingDirectory)

	for buildFile, task := range buildData {

		filesInWorkingDirectory, _ := os.ReadDir(workingDirectory)

		for _, file := range filesInWorkingDirectory {
			if file.Name() == buildFile {
				io.Print("Build file found: " + file.Name())
				com, err := task.Command(argCommand)
				if err != nil {
					io.Print(err.Error())
					return
				}
				command.WriteString(com)
			}
		}
	}

	if command.String() == "" {
		io.Print("No project configured")
		return
	}

	// Add extra arguments to the command
	if extraArgs != "" {
		command.WriteString(" " + extraArgs)
	}

	execute := lib.NewExecutor(command.String())
	if err := execute.Execute(); err != nil {
		io.Print("Error executing command: " + err.Error())
		return
	}
}
