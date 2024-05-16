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

type CLI struct {
	Cmd *cobra.Command
}

func NewCommand(command, description string, exec func(cmd *cobra.Command, args []string)) *CLI {
	cmd := &cobra.Command{
		Use:   command,
		Short: description,
		Long:  description,
		Run:   exec,
	}

	cmd.Flags().StringP("dir", "d", "", "Absolute path to project directory")
	cmd.Flags().StringP("extraArgs", "e", "", "Extra arguments to pass to the build tool")

	return &CLI{
		Cmd: cmd,
	}
}

func getConfigFile() []byte {
	homePath, err := os.UserHomeDir()
	configFile := path.Join(homePath, configFileName)
	fileContent, err := os.ReadFile(configFile)
	if err != nil {
		_, err := os.Create(configFile)
		if err != nil {
			FatalPrint(configFileError + err.Error())
		}
	}

	return fileContent
}

func GetBuilds() map[string]lib.Task {
	var j map[string]lib.Task

	err := json.Unmarshal(getConfigFile(), &j)
	if err != nil {
		FatalPrint("invalid configuration structure")
	}

	return j
}
