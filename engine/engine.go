package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sdlc/config"
	"sdlc/lib"
	"strings"
)

// Project represents a detected project with its location and task definition
type Project struct {
	Name    string   // Name of the build file (e.g. go.mod)
	Path    string   // Relative path to the directory containing the build file
	AbsPath string   // Absolute path to the directory
	Task    lib.Task // The task definition
}

// maxDetectionDepth is the upper bound for depth to prevent runaway recursion
// when the user passes -1 (unlimited).
const maxDetectionDepth = 50

// DetectProjects scans the working directory for known build files defined
// in the config. It recurses up to maxDepth levels deep (0 = root only,
// 1 = root + immediate children, which is the previous default behaviour).
// A negative maxDepth means unlimited recursion up to maxDetectionDepth.
func DetectProjects(workDir string, tasks map[string]lib.Task, maxDepth int) ([]Project, error) {
	var projects []Project
	seenDirs := make(map[string]bool)

	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %q: %w", workDir, err)
	}

	// Clamp negative depth to the safety limit
	if maxDepth < 0 {
		maxDepth = maxDetectionDepth
	}

	skipDirs := defaultSkipDirs()

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

		// Merge with global tasks — local overrides global
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
					if p.AbsPath == dir {
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
					AbsPath: dir,
					Task:    task,
				})
			}
		}
		return nil
	}

	// Walk the directory tree with depth limiting
	err = filepath.WalkDir(absWorkDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip directories we can't access but continue walking
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		// Calculate depth relative to the working directory
		rel, err := filepath.Rel(absWorkDir, path)
		if err != nil {
			return nil
		}
		if rel == "." {
			// This is the root — always check it
			if err := checkDir(path); err != nil {
				fmt.Printf("Warning: failed to check directory %s: %v\n", path, err)
			}
			return nil
		}

		// Check depth constraint
		depth := strings.Count(rel, string(filepath.Separator))
		if depth > maxDepth {
			return fs.SkipDir
		}

		// Skip well-known non-project directories
		if skipDirs[d.Name()] {
			return fs.SkipDir
		}

		// Check this directory for build files
		if err := checkDir(path); err != nil {
			fmt.Printf("Warning: failed to check directory %s: %v\n", path, err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", workDir, err)
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
