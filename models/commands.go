package models

import (
	"github.com/spf13/cobra"
)

type CLI struct {
	Cmd *cobra.Command
}

func NewCommand(command, description string, exec func(cmd *cobra.Command, args []string)) *CLI {
	return &CLI{
		Cmd: &cobra.Command{
			Use:   command,
			Short: description,
			Long:  description,
			Run:   exec,
		},
	}
}
