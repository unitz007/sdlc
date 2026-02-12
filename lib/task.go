// Package lib provides core types and utilities for executing software
// development lifecycle commands.
package lib

import "errors"

// Task represents the set of lifecycle commands associated with a specific
// project type. Each field maps to a shell command that performs the
// corresponding action (run, test, or build).
type Task struct {
	Run   string `json:"run"`
	Test  string `json:"test"`
	Build string `json:"build"`
}

// Command returns the shell command string for the given lifecycle action.
// Valid values for field are "run", "test", and "build". An error is returned
// if field does not match any known action.
func (c Task) Command(field string) (string, error) {
	switch field {
	case "run":
		return c.Run, nil
	case "test":
		return c.Test, nil
	case "build":
		return c.Build, nil
	default:
		return "", errors.New("invalid command")
	}
}
