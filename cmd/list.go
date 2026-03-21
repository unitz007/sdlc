package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"strings"

	"sdlc/config"
	"sdlc/engine"
	"sdlc/lib"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all detected modules in the project",
	Long: `List all auto-detected modules in the project directory, showing their
relative path, project type (build file), and available commands.

This is an informational command — it never executes any build commands
and always exits with code 0.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		printBanner()

		// Resolve working directory (honors --dir flag)
		wd, err := resolveWorkDir(workDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: directory error: %v\n", err)
			fmt.Printf("No modules detected in %s\n", workDir)
			return nil
		}

		// Load configuration (same three-step logic as runTask)
		var tasks map[string]lib.Task
		if cfgFile != "" {
			tasks, err = config.LoadFromDir(cfgFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: config error: %v\n", err)
			}
		}
		if tasks == nil {
			tasks, err = config.LoadLocal(wd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: local config error: %v\n", err)
			}
			if tasks == nil {
				tasks, err = config.Load("")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: global config error: %v\n", err)
				}
			}
		}

		// Detect projects
		projects, err := engine.DetectProjects(wd, tasks)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: detection error: %v\n", err)
			fmt.Printf("No modules detected in %s\n", wd)
			return nil
		}

		// Apply --module and --ignore filters; treat errors as warnings
		filtered, err := filterProjects(projects)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			// On filter error, fall back to showing all detected projects
			filtered = projects
		}

		if len(filtered) == 0 {
			fmt.Printf("No modules detected in %s\n", wd)
			return nil
		}

		// Print formatted table
		actions := []string{"run", "test", "build", "install", "clean"}
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(tw, "MODULE PATH\tTYPE\tCOMMANDS")
		for _, p := range filtered {
			var cmds []string
			for _, action := range actions {
				cmdStr, _ := p.Task.Command(action)
				if cmdStr != "" {
					cmds = append(cmds, action)
				}
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", p.Path, p.Name, strings.Join(cmds, ", "))
		}
		tw.Flush()

		fmt.Printf("\n%d module(s) detected\n", len(filtered))
		return nil
	},
}
