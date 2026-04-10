// Package lib provides core types and utilities for executing software
// development lifecycle commands.
package lib

import "errors"

// TaskHooks defines pre and post hooks for lifecycle actions.
// Keys in Pre and Post maps are action names (e.g., "build", "run", "deploy"),
// and values are shell commands to execute.
type TaskHooks struct {
	Pre  map[string]string `json:"pre"`
	Post map[string]string `json:"post"`
}

// Task represents the set of lifecycle commands associated with a specific
// project type. Each field maps to a shell command that performs the
// corresponding action (run, test, or build).
type Task struct {
	Run     string            `json:"run"`
	Test    string            `json:"test"`
	Build   string            `json:"build"`
	Install string            `json:"install"`
	Clean   string            `json:"clean"`
	Custom  map[string]string `json:"custom,omitempty"`
	Hooks   TaskHooks         `json:"hooks,omitempty"`
}

// Command returns the shell command string for the given lifecycle action.
// Valid values for field are "run", "test", "build", "install", and "clean".
// If field is not one of the built-in actions, it checks the Custom map.
// An error is returned if field does not match any known action and no custom
// action is defined for it.
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

// HasCustomActions returns true if the task has any custom actions defined.
func (c Task) HasCustomActions() bool {
	return len(c.Custom) > 0
}

// CustomActionNames returns a sorted list of custom action names defined for this task.
func (c Task) CustomActionNames() []string {
	if c.Custom == nil {
		return nil
	}
	names := make([]string, 0, len(c.Custom))
	for name := range c.Custom {
		names = append(names, name)
	}
	return names
}

// PreHook returns the pre-hook command for the given action, or empty string if none.
func (c Task) PreHook(action string) string {
	if c.Hooks.Pre != nil {
		return c.Hooks.Pre[action]
	}
	return ""
}

// PostHook returns the post-hook command for the given action, or empty string if none.
func (c Task) PostHook(action string) string {
	if c.Hooks.Post != nil {
		return c.Hooks.Post[action]
	}
	return ""
}
