package main

import (
	"os"

	"sdlc/cmd"

	"github.com/spf13/cobra"
)

func main() {
	// Register dynamic sub-commands (custom actions and plugins) after flag
	// parsing but before command execution. We use a persistent pre-run hook
	// that fires once for the root command and all its children.
	var setupDone bool
	cmd.RootCmd.PersistentPreRunE = func(rc *cobra.Command, args []string) error {
		if !setupDone {
			setupDone = true
			cmd.SetupDynamicCommands()
		}
		return nil
	}
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
