// Package io provides CLI construction, configuration loading, and formatted
// console output for the SDLC tool.
package io

import (
	"encoding/json"
	"os"
	"path"
	"sdlc/lib"

	"github.com/spf13/cobra"
)

const (
	configFileName  = ".sdlc.json"
	configFileError = "could not create config file:"
)

// CLI wraps a cobra.Command and provides a convenient constructor for creating
// SDLC subcommands with common flags pre-registered.
type CLI struct {
	Cmd *cobra.Command
}

// NewCommand creates a new CLI instance with the given command name, description,
// and run function. The returned command has --dir, --extraArgs, and --config
// flags pre-registered.
func NewCommand(command, description string, exec func(cmd *cobra.Command, args []string)) *CLI {
	cmd := &cobra.Command{
		Use:   command,
		Short: description,
		Long:  description,
		Run:   exec,
	}

	cmd.Flags().StringP("dir", "d", "", "Absolute path to project directory")
	cmd.Flags().StringP("extraArgs", "e", "", "Extra arguments to pass to the build tool")
	cmd.Flags().StringP("config", "c", "", "Path to directory for configuration file")

	return &CLI{
		Cmd: cmd,
	}
}

// getConfigFile reads the .sdlc.json configuration file from the given directory
// path. If conf is empty, it defaults to the user's home directory. If the file
// does not exist, an empty file is created.
func getConfigFile(conf string) []byte {
	var configFile string
	if conf != "" {
		configFile = path.Join(conf, configFileName)
	} else {
		homePath, _ := os.UserHomeDir()
		configFile = path.Join(homePath, configFileName)
	}

	fileContent, err := os.ReadFile(configFile)
	if err != nil {
		_, err := os.Create(configFile)
		if err != nil {
			FatalPrint(configFileError + err.Error())
		}
	}

	return fileContent
}

// GetBuilds reads and unmarshals the SDLC configuration file at the given path
// into a map of build-file names to their corresponding Task definitions.
// It calls FatalPrint and exits if the JSON is invalid.
func GetBuilds(path string) map[string]lib.Task {
	var j map[string]lib.Task

	err := json.Unmarshal(getConfigFile(path), &j)
	if err != nil {
		FatalPrint("invalid configuration structure")
	}

	return j
}
