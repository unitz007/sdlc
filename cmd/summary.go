package cmd

import (
	"fmt"
	"io"
	"regexp"

	"sdlc/lib"
)

// ModuleResult captures the execution outcome for a single module in the summary table.
type ModuleResult struct {
	Path       string
	Command    string
	Err        error
	ColorIndex int
}

// ansiRe matches ANSI CSI escape sequences (e.g. \033[31m, \033[0m).
var ansiRe = regexp.MustCompile(`\033\[[0-9;]*m`)

// ansiStrip removes all ANSI escape sequences from s.
func ansiStrip(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// printSummaryTable writes a formatted summary table to w listing each module's
// path, resolved command, and pass/fail status.
func printSummaryTable(results []ModuleResult, w io.Writer) {
	fmt.Fprintf(w, "\n[SDLC] Execution Summary:\n")

	// Compute column widths based on visible (stripped) text.
	maxPathLen := len("MODULE")
	maxCmdLen := len("COMMAND")
	for _, r := range results {
		if pl := len(ansiStrip(r.Path)); pl > maxPathLen {
			maxPathLen = pl
		}
		if cl := len(ansiStrip(r.Command)); cl > maxCmdLen {
			maxCmdLen = cl
		}
	}

	// Header row.
	fmt.Fprintf(w, "  %s%-*s  %-*s  STATUS%s\n",
		lib.Colorize("", lib.DarkGrey),
		maxPathLen, "MODULE",
		maxCmdLen, "COMMAND",
		lib.Colorize("", lib.Reset))

	// Separator line.
	totalWidth := 2 + maxPathLen + 2 + maxCmdLen + 2 + len("STATUS")
	sep := make([]byte, totalWidth)
	for i := range sep {
		sep[i] = '-'
	}
	fmt.Fprintf(w, "  %s\n", string(sep))

	// Result rows.
	for _, r := range results {
		status := lib.Colorize("PASS", lib.Green)
		if r.Err != nil {
			status = lib.Colorize("FAIL", lib.Red)
		}
		fmt.Fprintf(w, "  %s%-*s%s  %-*s  %s\n",
			lib.Colorize(r.Path, lib.ModuleColor(r.ColorIndex)),
			maxPathLen-len(ansiStrip(r.Path)), "",
			lib.Colorize("", lib.Reset),
			maxCmdLen, r.Command,
			status)
	}
}
