package engine

import (
    "os"
    "path/filepath"
    "testing"
)

func TestSaveAndLoadState(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "state.json")

    original := WorkflowState{Stages: []Stage{{ProjectPath: "proj1", Action: "build", Status: Completed}}}
    if err := SaveState(path, original); err != nil {
        t.Fatalf("SaveState failed: %v", err)
    }

    loaded, err := LoadState(path)
    if err != nil {
        t.Fatalf("LoadState failed: %v", err)
    }
    if len(loaded.Stages) != len(original.Stages) {
        t.Fatalf("expected %d stages, got %d", len(original.Stages), len(loaded.Stages))
    }
    if loaded.Stages[0] != original.Stages[0] {
        t.Fatalf("stage mismatch: got %+v, want %+v", loaded.Stages[0], original.Stages[0])
    }
}

func TestLoadCorruptedState(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "state.json")
    // Write invalid JSON
    if err := os.WriteFile(path, []byte("{invalid json"), 0o644); err != nil {
        t.Fatalf("failed to write corrupted file: %v", err)
    }
    _, err := LoadState(path)
    if err == nil {
        t.Fatalf("expected error when loading corrupted state, got nil")
    }
}
