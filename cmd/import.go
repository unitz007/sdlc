package cmd

import (
    "encoding/json"
    "fmt"
    "os"
    "sdlc/config"
    "github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
    Use:   "import",
    Short: "Import workflow definition from a YAML file",
    RunE:  runImport,
}

var importFile string

func init() {
    RootCmd.AddCommand(importCmd)
    importCmd.Flags().StringVarP(&importFile, "file", "f", "", "Path to YAML file to import (required)")
    importCmd.MarkFlagRequired("file")
}

func runImport(cmd *cobra.Command, args []string) error {
    // Load tasks from YAML
    tasks, err := config.LoadYAML(importFile)
    if err != nil {
        return fmt.Errorf("failed to load YAML: %w", err)
    }
    // Get path to global config (JSON)
    cfgPath, err := config.GetConfigFilePath("")
    if err != nil {
        return err
    }
    // Marshal to JSON
    data, err := json.MarshalIndent(tasks, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal tasks to JSON: %w", err)
    }
    // Write file
    if err := os.WriteFile(cfgPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write config file: %w", err)
    }
    fmt.Printf("Imported workflow definition into %s\n", cfgPath)
    return nil
}
