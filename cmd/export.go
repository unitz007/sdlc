package cmd

import (
    "encoding/json"
    "fmt"
    "os"
    "sdlc/lib"
    "sdlc/config"
    "github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
    Use:   "export",
    Short: "Export current workflow definition to a YAML file",
    RunE:  runExport,
}

var exportFile string

func init() {
    RootCmd.AddCommand(exportCmd)
    exportCmd.Flags().StringVarP(&exportFile, "file", "f", "", "Path to output YAML file (required)")
    exportCmd.MarkFlagRequired("file")
}

func runExport(cmd *cobra.Command, args []string) error {
    // Load current tasks from JSON config (global or local)
    cfgPath, err := config.GetConfigFilePath("")
    if err != nil {
        return err
    }
    // Read JSON config
    content, err := os.ReadFile(cfgPath)
    if err != nil {
        return fmt.Errorf("failed to read config file: %w", err)
    }
    var tasks map[string]lib.Task
    if len(content) > 0 {
        if err := json.Unmarshal(content, &tasks); err != nil {
            return fmt.Errorf("invalid JSON config: %w", err)
        }
    } else {
        tasks = make(map[string]config.Task)
    }
    // Export to YAML
    if err := config.ExportYAML(exportFile, tasks); err != nil {
        return err
    }
    fmt.Printf("Exported workflow definition to %s\n", exportFile)
    return nil
}
