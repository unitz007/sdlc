package workflow

import (
    "os"
    "path/filepath"
    "testing"
)

func TestWorkflowBasic(t *testing.T) {
    // Create temp directory for persistence file
    dir := t.TempDir()
    persistFile := filepath.Join(dir, "state.json")

    wf, err := New(persistFile)
    if err != nil {
        t.Fatalf("failed to create workflow: %v", err)
    }

    // Flags for callbacks
    entered := make(map[string]bool)
    exited := make(map[string]bool)

    // Add stages
    stages := []string{"requirements", "design", "implementation"}
    for _, name := range stages {
        st, err := wf.AddStage(name)
        if err != nil {
            t.Fatalf("add stage %s: %v", name, err)
        }
        // Register callbacks
        n := name // capture
        st.OnEnter(func() error { entered[n] = true; return nil })
        st.OnExit(func() error { exited[n] = true; return nil })
    }

    // Add transitions
    if err := wf.AddTransition("requirements", "design", nil, nil); err != nil {
        t.Fatalf("add transition: %v", err)
    }
    if err := wf.AddTransition("design", "implementation", func() bool { return true }, nil); err != nil {
        t.Fatalf("add transition with condition: %v", err)
    }

    // Initial stage should be the first added
    if wf.Current() != "requirements" {
        t.Fatalf("expected initial stage 'requirements', got %s", wf.Current())
    }

    // Move to design
    if err := wf.Move("design"); err != nil {
        t.Fatalf("move to design: %v", err)
    }
    if wf.Current() != "design" {
        t.Fatalf("expected current stage 'design', got %s", wf.Current())
    }
    if !exited["requirements"] || !entered["design"] {
        t.Fatalf("callbacks not executed on transition to design")
    }

    // Move to implementation
    if err := wf.Move("implementation"); err != nil {
        t.Fatalf("move to implementation: %v", err)
    }
    if wf.Current() != "implementation" {
        t.Fatalf("expected current stage 'implementation', got %s", wf.Current())
    }
    if !exited["design"] || !entered["implementation"] {
        t.Fatalf("callbacks not executed on transition to implementation")
    }

    // Verify persistence: create new workflow loading same file
    wf2, err := New(persistFile)
    if err != nil {
        t.Fatalf("load persisted workflow: %v", err)
    }
    if wf2.Current() != "implementation" {
        t.Fatalf("expected persisted current stage 'implementation', got %s", wf2.Current())
    }

    // Cleanup
    _ = os.Remove(persistFile)
}
