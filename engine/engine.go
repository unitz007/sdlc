package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sdlc/config"
	"sdlc/lib"
)

// Project represents a detected project with its location and task definition
type Project struct {
	Name    string   // Name of the build file (e.g. go.mod)
	Path    string   // Relative path to the directory containing the build file
	AbsPath string   // Absolute path to the directory
	Task    lib.Task // The task definition
}

// DetectProjects scans the working directory and its subdirectories up to
// maxDepth levels deep for known build files defined in the config.
// It returns a list of detected projects.
//
// Depth semantics:
//   - maxDepth 0: scan only the root working directory.
//   - maxDepth 1: root + immediate subdirectories (default, backward-compatible).
//   - maxDepth N: root + up to N levels of subdirectories.
//
// Directories listed in lib.SkippedDirs are never traversed.
func DetectProjects(workDir string, tasks map[string]lib.Task, maxDepth int) ([]Project, error) {
	var projects []Project
	seenDirs := make(map[string]bool)

	// Helper to check a directory for build files
	checkDir := func(dir string) error {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		realDir, err := filepath.EvalSymlinks(absDir)
		if err != nil {
			realDir = absDir
		}

		if seenDirs[realDir] {
			return nil
		}
		seenDirs[realDir] = true

		// Try to load local configuration
		localTasks, err := config.LoadLocal(dir)
		if err != nil {
			fmt.Printf("Warning: failed to read local config in %s: %v\n", dir, err)
		}

		// Merge with global tasks
		effectiveTasks := tasks
		if len(localTasks) > 0 {
			effectiveTasks = make(map[string]lib.Task)
			for k, v := range tasks {
				effectiveTasks[k] = v
			}
			for k, v := range localTasks {
				if existing, ok := effectiveTasks[k]; ok {
					// Merge: local overrides built-in fields, but custom and hooks are merged
					effectiveTasks[k] = mergeTasks(existing, v)
				} else {
					effectiveTasks[k] = v
				}
			}
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if task, ok := effectiveTasks[entry.Name()]; ok {
				// Check if project already exists to prevent duplicates
				// We enforce one project per directory to avoid running multiple tasks for the same project
				exists := false
				for _, p := range projects {
					if p.AbsPath == absDir {
						exists = true
						break
					}
				}
				if exists {
					continue
				}

				relPath, err := filepath.Rel(workDir, dir)
				if err != nil {
					relPath = dir
				}

				projects = append(projects, Project{
					Name:    entry.Name(),
					Path:    relPath,
					AbsPath: absDir,
					Task:    task,
				})
			}
		}
		return nil
	}

	// BFS queue entries: (directory path, current depth relative to workDir)
	type queueEntry struct {
		dir   string
		depth int
	}

	// Check root directory
	if err := checkDir(workDir); err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", workDir, err)
	}

	if maxDepth < 1 {
		return projects, nil
	}

	// Seed the BFS queue with immediate subdirectories of workDir
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", workDir, err)
	}

	queue := make([]queueEntry, 0)
	for _, entry := range entries {
		if entry.IsDir() && !lib.SkippedDirs[entry.Name()] {
			queue = append(queue, queueEntry{
				dir:   filepath.Join(workDir, entry.Name()),
				depth: 1,
			})
		}
	}

	// BFS traversal
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		// Check this directory for build files
		_ = checkDir(cur.dir) // Ignore errors in subdirectories to keep going

		// If we haven't reached maxDepth, enqueue child directories
		if cur.depth < maxDepth {
			subEntries, err := os.ReadDir(cur.dir)
			if err != nil {
				continue // Skip unreadable directories
			}
			for _, entry := range subEntries {
				if entry.IsDir() && !lib.SkippedDirs[entry.Name()] {
					queue = append(queue, queueEntry{
						dir:   filepath.Join(cur.dir, entry.Name()),
						depth: cur.depth + 1,
					})
				}
			}
		}
	}

	return projects, nil
}

// mergeTasks merges two Task structs where local (b) overrides global (a)
// for built-in fields, but custom actions and hooks are combined (local wins
// on conflicts).
func mergeTasks(a, b lib.Task) lib.Task {
	merged := lib.Task{
		// Built-in fields: local overrides global
		Run:     b.Run,
		Test:    b.Test,
		Build:   b.Build,
		Install: b.Install,
		Clean:   b.Clean,
	}

	// Merge custom actions: start with global, override with local
	merged.Custom = make(map[string]string)
	for k, v := range a.Custom {
		merged.Custom[k] = v
	}
	for k, v := range b.Custom {
		merged.Custom[k] = v
	}
	if len(merged.Custom) == 0 {
		merged.Custom = nil
	}

	// Merge hooks: start with global, override with local
	merged.Hooks.Pre = make(map[string]string)
	for k, v := range a.Hooks.Pre {
		merged.Hooks.Pre[k] = v
	}
	for k, v := range b.Hooks.Pre {
		merged.Hooks.Pre[k] = v
	}
	if len(merged.Hooks.Pre) == 0 {
		merged.Hooks.Pre = nil
	}

	merged.Hooks.Post = make(map[string]string)
	for k, v := range a.Hooks.Post {
		merged.Hooks.Post[k] = v
	}
	for k, v := range b.Hooks.Post {
		merged.Hooks.Post[k] = v
	}
	if len(merged.Hooks.Post) == 0 {
		merged.Hooks.Post = nil
	}

	return merged
}
