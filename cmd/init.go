package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sdlc/config"
	"sdlc/lib"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a .sdlc.json configuration file in the current or specified directory",
	Long: `Scaffold a .sdlc.json configuration file with sensible defaults for common
project types (go.mod, package.json). The generated file serves as a starting
point that you can customize for your project.

If a .sdlc.json file already exists in the target directory, you will be
prompted to confirm before overwriting it.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir, err := resolveWorkDir(workDir)
		if err != nil {
			return fmt.Errorf("failed to resolve target directory: %w", err)
		}

		targetFile := filepath.Join(targetDir, config.ConfigFileName)

		// Check if file already exists and prompt before overwriting
		if _, err := os.Stat(targetFile); err == nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s already exists in %s\n", config.ConfigFileName, targetDir)
			prompt := promptui.Prompt{
				Label:     "Overwrite?",
				IsConfirm: true,
			}
			_, err := prompt.Run()
			if err != nil {
				return nil
			}
		}

		tasks := map[string]lib.Task{
			"go.mod": {
				Run:     "go run .",
				Test:    "go test ./...",
				Build:   "go build -o app",
				Install: "go mod download",
				Clean:   "go clean",
			},
			"package.json": {
				Run:     "npm start",
				Test:    "npm test",
				Build:   "npm run build",
				Install: "npm install",
				Clean:   "rm -rf node_modules",
			},
		}

		data, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}

		if err := os.WriteFile(targetFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetFile, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created %s in %s\n", config.ConfigFileName, targetDir)
		return nil
	},
}
