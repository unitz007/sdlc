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
		argCommand              string
		extraArgs               string
		workingDirectoryCommand string
	)

	// CLI arguments
	rootCmd := io.NewCommand("sdlc", "SDLC helps manage the full lifecycle of your software project", func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			return
		}
	}).Cmd

	var subcommand = func(com string, desc string) command {
		return command{
			command:     com,
			description: desc,
			action: func(cmd *cobra.Command, args []string) {
				argCommand = com
				extraArgs = cmd.Flag("extraArgs").Value.String()
				workingDirectoryCommand = cmd.Flag("dir").Value.String()
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

	configFilePath := os.Getenv("SDLC_CONFIG_LOCATION")

	buildData := io.GetBuilds(configFilePath)
	var com strings.Builder
	workingDirectory := func() string {
		var wd string
		if workingDirectoryCommand == "" {
			wd, _ = os.Getwd()
		} else {
			wd = workingDirectoryCommand
		}

		if strings.HasPrefix(wd, "~/") {
			homeDir, _ := os.UserHomeDir()
			strings.ReplaceAll(wd, "~", homeDir)
		}

		return wd
	}()

	err := os.Chdir(workingDirectory)
	if err != nil {
		io.FatalPrint(err.Error())
	}

	files, err := os.ReadDir(workingDirectory)

	for buildFile, task := range buildData {
		for _, file := range files {
			if file.Name() == buildFile {
				io.Print("Build file found: " + file.Name())
				output, err := task.Command(argCommand)
				if err != nil {
					io.Print(err.Error())

				}
				com.WriteString(output)
			}

		}
	}

	if com.String() == "" {
		io.Print("No project configured")
		return
	}

	// Add extra arguments to the com
	if extraArgs != "" {
		com.WriteString(" " + extraArgs)
	}

	if configFilePath != "" {
		com.WriteString(" " + configFilePath)
	}

	execute := lib.NewExecutor(com.String())
	if err := execute.Execute(); err != nil {
		io.Print("Error executing com: " + err.Error())
		return
	}
}

// command holds the metadata for an SDLC CLI subcommand.
type command struct {
	command     string
	description string
	action      func(cmd *cobra.Command, args []string)
}
