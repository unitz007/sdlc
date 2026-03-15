package visual

import (
    "fmt"
    "os"
    "os/exec"
    "strings"
    "sdlc/engine"
)

// ExportVisualization generates a Graphviz DOT representation of the given projects
// and writes it to the specified path. If format is "dot", the DOT source is written
// directly. For other supported Graphviz output formats (e.g., "png", "svg"), the
// function invokes the `dot` command to render the diagram. The default format is
// "png".
func ExportVisualization(projects []engine.Project, outPath string, format string) error {
    if format == "" {
        format = "png"
    }

    var sb strings.Builder
    sb.WriteString("digraph workflow {\n")
    sb.WriteString("    rankdir=LR;\n")
    sb.WriteString("    node [shape=box];\n")
    for _, p := range projects {
        label := fmt.Sprintf("%s\\n%s", p.Name, p.Path)
        sb.WriteString(fmt.Sprintf("    \"%s\" [label=\"%s\"];\n", p.Name, label))
    }
    sb.WriteString("}\n")
    dot := sb.String()

    if format == "dot" {
        return os.WriteFile(outPath, []byte(dot), 0644)
    }

    // Write to temporary file
    tmpFile, err := os.CreateTemp("", "workflow-*.dot")
    if err != nil {
        return fmt.Errorf("failed to create temporary DOT file: %w", err)
    }
    defer os.Remove(tmpFile.Name())
    if _, err := tmpFile.Write([]byte(dot)); err != nil {
        tmpFile.Close()
        return fmt.Errorf("failed to write DOT content: %w", err)
    }
    tmpFile.Close()

    cmd := exec.Command("dot", "-T"+format, "-o", outPath, tmpFile.Name())
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("dot command failed: %v, output: %s", err, string(out))
    }
    return nil
}
