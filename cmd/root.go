package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	workDir    string
	extraArgs  string
	targetMod  string
	ignoreMods []string
	runAllMods bool
	watchMode  bool
	dryRun     bool
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "sdlc",
	Short: "SDLC helps manage the full lifecycle of your software project",
	Long: `SDLC is a lightweight CLI tool that provides a unified interface 
for common software development lifecycle commands — run, test, and build — 
across different project types.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Resolve working directory for plugin and dynamic command registration
		wd, err := resolveWorkDir(workDir)
		if err != nil {
			return fmt.Errorf("directory error: %w", err)
		}

		// Register plugin commands (only once)
		registerPluginCommands(wd)

		// Register dynamic commands from custom actions (only once)
		registerDynamicCommands(wd)

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&workDir, "dir", "d", "", "Absolute path to project directory")
	RootCmd.PersistentFlags().StringVarP(&extraArgs, "extra-args", "e", "", "Extra arguments to pass to the build tool")
	RootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Path to directory for configuration file")
	RootCmd.PersistentFlags().StringVarP(&targetMod, "module", "m", "", "Specific module/path to run (relative path)")
	RootCmd.PersistentFlags().StringSliceVarP(&ignoreMods, "ignore", "i", []string{}, "Ignore specific modules (by path or name)")
	RootCmd.PersistentFlags().BoolVarP(&runAllMods, "all", "a", false, "Run command for all detected modules")
	RootCmd.PersistentFlags().BoolVarP(&watchMode, "watch", "w", false, "Watch for file changes and restart")
	RootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would happen without executing commands (dry run)")
}

// resolveWorkDir handles the directory resolution logic including tilde expansion
func resolveWorkDir(dirFlag string) (string, error) {
	wd := dirFlag
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	if strings.HasPrefix(wd, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		wd = strings.ReplaceAll(wd, "~", homeDir)
	}
	return wd, nil
}
