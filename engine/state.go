package engine

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
)

// StageStatus represents the execution status of a workflow stage.
type StageStatus string

const (
    // Pending indicates the stage has not yet started.
    Pending StageStatus = "pending"
    // Completed indicates the stage finished successfully.
    Completed StageStatus = "completed"
    // Failed indicates the stage finished with an error.
    Failed StageStatus = "failed"
)

// Stage represents a single step in a workflow for a given project and action.
type Stage struct {
    ProjectPath string      `json:"project_path"`
    Action      string      `json:"action"`
    Status      StageStatus `json:"status"`
    // ErrorMessage captures the error text when Status is Failed.
    ErrorMessage string `json:"error_message,omitempty"`
}

// WorkflowState aggregates all stages of a workflow execution.
type WorkflowState struct {
    Stages []Stage `json:"stages"`
}

// SaveState serializes the given WorkflowState to the specified file path in JSON format.
// It creates the parent directory if it does not exist.
func SaveState(path string, state WorkflowState) error {
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal workflow state: %w", err)
    }

    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return fmt.Errorf("failed to create directory for state file: %w", err)
    }

    // Write file atomically by writing to a temp file then renaming.
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        return fmt.Errorf("failed to write temporary state file: %w", err)
    }
    if err := os.Rename(tmp, path); err != nil {
        return fmt.Errorf("failed to rename temporary state file: %w", err)
    }
    return nil
}

// LoadState reads a WorkflowState from the given JSON file path.
// It returns an error if the file cannot be read or the JSON is malformed.
func LoadState(path string) (WorkflowState, error) {
    var state WorkflowState

    f, err := os.Open(path)
    if err != nil {
        if os.IsNotExist(err) {
            // Return empty state if file does not exist.
            return state, nil
        }
        return state, fmt.Errorf("failed to open state file: %w", err)
    }
    defer f.Close()

    content, err := io.ReadAll(f)
    if err != nil {
        return state, fmt.Errorf("failed to read state file: %w", err)
    }
    if len(content) == 0 {
        return state, nil
    }
    if err := json.Unmarshal(content, &state); err != nil {
        return state, fmt.Errorf("invalid workflow state JSON: %w", err)
    }
    return state, nil
}
