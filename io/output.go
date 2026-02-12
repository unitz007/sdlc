package io

import (
	"fmt"
	"os"
)

// FatalPrint prints the given message to stdout with the "[SDLC]:" prefix
// and immediately exits the process with a non-zero status code.
func FatalPrint(v string) {
	fmt.Println("[SDLC]:", v)
	os.Exit(1)
}

// Print prints the given message to stdout with the "[SDLC]" prefix.
func Print(v string) {
	fmt.Println("[SDLC]", v)
}
