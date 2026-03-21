package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the sdlc version string. It can be overridden at build time via:
//
//	go build -ldflags "-X sdlc/cmd.Version=1.0.0"
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the sdlc version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("sdlc version %s\n", Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
