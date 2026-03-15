package cmd

import (
    "encoding/json"
    "fmt"
    "os"
    "os/signal"
    "strings"
    "syscall"

    "github.com/spf13/cobra"
)

// Workflow defines a set of stages to be executed.
type Workflow struct {
    Name   string  `json:"name" yaml:"name"`
    Stages []Stage `json:"stages" yaml:"stages"`
}

// Stage represents a single step within a workflow.
type Stage struct {
    Name   string `json:"name" yaml:"name"`
    Action string `json:"action" yaml:"action"`
}

var (
    workflowFile   string
    selectedStages []string
    cfgFile        string // reuse existing root flag for config if needed
    workDir        string // reuse existing flag for working directory
)

var workflowCmd = &cobra.Command{
    Use:   "workflow [file]",
    Short: "Execute an SDLC workflow defined in a JSON/YAML file",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // Determine the workflow file path.
        if workflowFile == "" && len(args) > 0 {
            workflowFile = args[0]
        }
        if workflowFile == "" {
            return fmt.Errorf("workflow file path is required (provide via --file or as positional argument)")
        }

        wf, err := loadWorkflow(workflowFile)
        if err != nil {
            return fmt.Errorf("failed to load workflow %s: %w", workflowFile, err)
        }

        // Determine which stages to run.
        var stagesToRun []Stage
        if len(selectedStages) > 0 {
            // User supplied a list of stage names (comma‑separated via flag).
            for _, name := range selectedStages {
                found := false
                for _, st := range wf.Stages {
                    if st.Name == name {
                        stagesToRun = append(stagesToRun, st)
                        found = true
                        break
                    }
                }
                if !found {
                    return fmt.Errorf("stage %s not found in workflow %s", name, wf.Name)
                }
            }
        } else {
            stagesToRun = wf.Stages
        }

        if len(stagesToRun) == 0 {
            fmt.Printf("[Workflow] No stages to run for workflow %s\n", wf.Name)
            return nil
        }

        fmt.Printf("[Workflow] Executing workflow %s with %d stage(s)\n", wf.Name, len(stagesToRun))
        for i, st := range stagesToRun {
            fmt.Printf("[Workflow] Stage %d/%d: %s (action: %s)\n", i+1, len(stagesToRun), st.Name, st.Action)
            // Reuse existing task execution logic.
            // Pass an empty cobra.Command to satisfy the signature.
            if err := executeTask(workflowCmd, st.Action); err != nil {
                return fmt.Errorf("stage %s failed: %w", st.Name, err)
            }
        }
        fmt.Printf("[Workflow] Completed workflow %s\n", wf.Name)
        return nil
    },
}

func init() {
    RootCmd.AddCommand(workflowCmd)
    workflowCmd.Flags().StringVarP(&workflowFile, "file", "f", "", "Path to workflow JSON/YAML file (or positional argument)")
    workflowCmd.Flags().StringSliceVarP(&selectedStages, "stages", "s", []string{}, "Comma‑separated list of stage names to execute (default all)")
}

// loadWorkflow reads a workflow definition from a file. Currently JSON is supported.
func loadWorkflow(path string) (*Workflow, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var wf Workflow
    if err := json.Unmarshal(data, &wf); err != nil {
        return nil, err
    }
    // Normalise stage names for matching.
    for i := range wf.Stages {
        wf.Stages[i].Name = strings.TrimSpace(wf.Stages[i].Name)
        wf.Stages[i].Action = strings.TrimSpace(wf.Stages[i].Action)
    }
    return &wf, nil
}
