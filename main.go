package main

import (
	"os"
	"path/filepath"
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
		configFile       string
	)

	// CLI arguments
	rootCmd := io.NewCommand("sdlc", "", func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(1)
		}

		workingDirectory = cmd.Flag("dir").Value.String()

	}).Cmd

	var subcommand = func(com string, desc string) command {
		return command{
			command:     com,
			description: desc,
			action: func(cmd *cobra.Command, args []string) {
				argCommand = com
				extraArgs = cmd.Flag("extraArgs").Value.String()
				workingDirectory = cmd.Flag("dir").Value.String()
				configFile = cmd.Flag("config").Value.String()
			},
		}
	}

	var subCommands = []command{
		subcommand("run", "Runs your code"),
		subcommand("test", "Tests your code"),
		subcommand("build", "Builds your project"),
	}

	for _, subCommand := range subCommands {
		s := io.NewCommand(subCommand.command, subCommand.description, subCommand.action)
		rootCmd.AddCommand(s.Cmd)
	}

	if err := rootCmd.Execute(); err != nil || argCommand == "" {
		_ = rootCmd.Help()
		return
	}

	buildData := io.GetBuilds(configFile)
	var com strings.Builder

	workingDirectory = func(wd string) string {
		if strings.HasPrefix(workingDirectory, "~/") {
			homeDir, _ := os.UserHomeDir()
			return strings.ReplaceAll(workingDirectory, "~", homeDir)
		}

		return workingDirectory
	}(workingDirectory)

	if workingDirectory == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			io.FatalPrint("Unable to get working directory: " + err.Error())
		}
		_ = os.Chdir(workingDirectory)
	} else {
		_ = os.Chdir(workingDirectory)
	}

	for buildFile, task := range buildData {
		filepath.Walk(workingDirectory, func(path string, info os.FileInfo, err error) error {
			if info.Name() == buildFile {
				io.Print("Build file found: " + info.Name())
				output, err := task.Command(argCommand)
				if err != nil {
					io.Print(err.Error())

				}
				com.WriteString(output)
			}

			return nil
		})
	}

	if com.String() == "" {
		io.Print("No project configured")
		return
	}

	// Add extra arguments to the com
	if extraArgs != "" {
		com.WriteString(" " + extraArgs)
	}

	if configFile != "" {
		com.WriteString(" " + configFile)
	}

	execute := lib.NewExecutor(com.String())
	if err := execute.Execute(); err != nil {
		io.Print("Error executing com: " + err.Error())
		return
	}
}

type command struct {
	command     string
	description string
	action      func(cmd *cobra.Command, args []string)
}
