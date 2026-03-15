package lib

import (
    "os"
    "path/filepath"
    "testing"
    "sdlc/engine"
)

func TestExportVisualization_DOT(t *testing.T) {
    projects := []engine.Project{{Name: "projA", Path: "path/to/projA"}, {Name: "projB", Path: "path/to/projB"}}
    tmpDir := t.TempDir()
    outPath := filepath.Join(tmpDir, "workflow.dot")
    if err := ExportVisualization(projects, outPath, "dot"); err != nil {
        t.Fatalf("ExportVisualization returned error: %v", err)
    }
    data, err := os.ReadFile(outPath)
    if err != nil {
        t.Fatalf("failed to read output file: %v", err)
    }
    content := string(data)
    // Basic checks for DOT format and node definitions
    if !contains(content, "digraph workflow") {
        t.Errorf("DOT output missing graph declaration")
    }
    for _, p := range projects {
        if !contains(content, p.Name) {
            t.Errorf("DOT output missing node for project %s", p.Name)
        }
    }
}

// Simple substring check helper
import "strings"

func contains(s, substr string) bool {
    return strings.Contains(s, substr)
}
