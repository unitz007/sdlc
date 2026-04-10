package plugin

import (
	"fmt"
	"sort"
	"strings"
)

// Hook defines a single lifecycle event and the command to execute when it
// fires. Multiple hooks can share the same Name; they run in priority order
// (lower Priority values execute first).
type Hook struct {
	// Name is the lifecycle event this hook listens to, e.g. "pre-build",
	// "post-test", "pre-release". Any user-defined name is valid.
	Name string `json:"name"`

	// Command is the shell command to execute. It is passed to
	// [lib.Executor] and supports the full shell syntax.
	Command string `json:"command"`

	// Description is an optional human-readable explanation of what the
	// hook does. Useful for "list" commands and documentation.
	Description string `json:"description,omitempty"`

	// Priority controls execution order among hooks with the same Name.
	// Lower values run first. Defaults to 0.
	Priority int `json:"priority,omitempty"`

	// ProjectType is an optional filter. If non-empty, the hook only fires
	// for projects whose type matches this value (case-insensitive).
	ProjectType string `json:"project_type,omitempty"`

	// HookPhase is the phase: "pre" or "post". This is derived from the
	// hook Name but can be set explicitly for validation purposes.
	HookPhase string `json:"hook_phase,omitempty"`
}

// Phase returns "pre" or "post" based on the hook name prefix.
// Falls back to the HookPhase field if set, otherwise returns "".
func (h Hook) Phase() string {
	if h.HookPhase != "" {
		return h.HookPhase
	}
	if strings.HasPrefix(h.Name, "pre-") {
		return "pre"
	}
	if strings.HasPrefix(h.Name, "post-") {
		return "post"
	}
	return ""
}

// MatchesProject returns true if the hook's project type filter is empty or
// matches the given project type (case-insensitive comparison).
func (h Hook) MatchesProject(projectType string) bool {
	if h.ProjectType == "" {
		return true
	}
	return strings.EqualFold(h.ProjectType, projectType)
}

// Validate checks that required fields are set and that the hook name follows
// the expected convention.
func (h Hook) Validate() error {
	if h.Name == "" {
		return fmt.Errorf("hook name is required")
	}
	if h.Command == "" {
		return fmt.Errorf("hook %q: command is required", h.Name)
	}
	// Warn about non-standard names but don't fail
	if h.Phase() == "" {
		return fmt.Errorf("hook %q: name should start with \"pre-\" or \"post-\" (e.g. \"pre-build\", \"post-test\")", h.Name)
	}
	return nil
}

// ByPriority implements sort.Interface for []Hook.
type ByPriority []Hook

func (h ByPriority) Len() int      { return len(h) }
func (h ByPriority) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h ByPriority) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	// Stable sort by name for equal priorities
	return h[i].Name < h[j].Name
}

// SortHooks returns a new slice sorted by priority (ascending).
func SortHooks(hooks []Hook) []Hook {
	sorted := make([]Hook, len(hooks))
	copy(sorted, hooks)
	sort.Sort(ByPriority(sorted))
	return sorted
}
