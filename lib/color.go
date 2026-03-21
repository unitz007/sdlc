package lib

import (
	"os"
	"sync/atomic"
	"syscall"
)

// ANSI color escape sequences.
const (
	Reset    = "\033[0m"
	Red      = "\033[31m"
	Green    = "\033[32m"
	Yellow   = "\033[33m"
	Blue     = "\033[34m"
	Magenta  = "\033[35m"
	Cyan     = "\033[36m"
	White    = "\033[37m"
	DarkGrey = "\033[90m"
)

// moduleColors is the palette used to assign distinct colors to modules.
var moduleColors = []string{
	Cyan,
	Green,
	Magenta,
	Yellow,
	Blue,
}

// colorEnabled controls whether ANSI color codes are emitted.
var colorEnabled atomic.Bool

// InitColor initializes the color system. If forceDisable is true, colors are
// unconditionally disabled. Otherwise colors are enabled only when stdout is a
// TTY (so piped/CI output stays clean).
func InitColor(forceDisable bool) {
	if forceDisable {
		colorEnabled.Store(false)
		return
	}
	colorEnabled.Store(isTerminal(int(os.Stdout.Fd())))
}

// ModuleColor returns the ANSI color code for the module at the given index.
// If colors are disabled it returns an empty string.
func ModuleColor(index int) string {
	if !colorEnabled.Load() {
		return ""
	}
	return moduleColors[index%len(moduleColors)]
}

// Colorize wraps text with the given ANSI colorCode and Reset.
// If colors are disabled the text is returned unchanged.
func Colorize(text, colorCode string) string {
	if !colorEnabled.Load() || colorCode == "" {
		return text
	}
	return colorCode + text + Reset
}

// isTerminal returns true if the file descriptor refers to a terminal.
func isTerminal(fd int) bool {
	var st syscall.Stat_t
	err := syscall.Fstat(fd, &st)
	return err == nil && (st.Mode&syscall.S_IFMT) == syscall.S_IFCHR
}
