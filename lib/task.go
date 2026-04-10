// Package lib provides core types and utilities for executing software
// development lifecycle commands.
package lib

import "errors"

// TaskHooks defines pre/post lifecycle hooks for a project task.
// Each map key is the action name (e.g., "build", "run") and the value is
// the shell command to execute.
type TaskHooks struct {
	// Pre maps action names to commands that run before the main action.
	// Example: {"build": "echo starting build..."}
	Pre map[string]string `json:"pre,omitempty"`

	// Post maps action names to commands that run after the main action,
	// regardless of success or failure.
	// Example: {"run": "notify-send done"}
	Post map[string]string `json:"post,omitempty"`
}

// Hook returns the pre-hook command for the given action, or empty string
// if no pre-hook is defined.
func (h TaskHooks) Hook(phase, action string) string {
	switch phase {
	case "pre":
		if h.Pre != nil {
			return h.Pre[action]
		}
	case "post":
		if h.Post != nil {
			return h.Post[action]
		}
	}
	return ""
}

// Task represents the set of lifecycle commands associated with a specific
// project type. Each field maps to a shell command that performs the
// corresponding action (run, test, or build).
type Task struct {
	Run     string     `json:"run"`
	Test    string     `json:"test"`
	Build   string     `json:"build"`
	Install string     `json:"install"`
	Clean   string     `json:"clean"`
	Custom  map[string]string `json:"custom,omitempty"`
	Hooks   TaskHooks  `json:"hooks,omitempty"`
}

// Command returns the shell command string for the given lifecycle action.
// Valid values for field are "run", "test", "build", "install", and "clean".
// For any other field, it checks the Custom map. An error is returned
// if field does not match any known action and is not found in Custom.
func (c Task) Command(field string) (string, error) {
	switch field {
	case "run":
		return c.Run, nil
	case "test":
		return c.Test, nil
	case "build":
		return c.Build, nil
	case "install":
		return c.Install, nil
	case "clean":
		return c.Clean, nil
	default:
		if c.Custom != nil {
			if cmd, ok := c.Custom[field]; ok {
				return cmd, nil
			}
		}
		return "", errors.New("invalid command")
	}
}

// CustomActions returns a list of all custom action names defined in the task.
func (c Task) CustomActions() []string {
	var actions []string
	for k := range c.Custom {
		actions = append(actions, k)
	}
	return actions
}

// AllActions returns all available action names: the 5 built-in actions plus
// any custom actions.
func (c Task) AllActions() []string {
	actions := []string{"run", "test", "build", "install", "clean"}
	for k := range c.Custom {
		actions = append(actions, k)
	}
	return actions
}
