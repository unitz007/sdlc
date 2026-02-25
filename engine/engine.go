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

// DetectProjects scans the working directory and its immediate subdirectories
// for known build files defined in the config.
// It returns a list of detected projects.
func DetectProjects(workDir string, tasks map[string]lib.Task) ([]Project, error) {
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
		// fmt.Printf("DEBUG: Checking %s, tasks count: %d\n", dir, len(effectiveTasks))
		if len(localTasks) > 0 {
			effectiveTasks = make(map[string]lib.Task)
			for k, v := range tasks {
				effectiveTasks[k] = v
			}
			for k, v := range localTasks {
				effectiveTasks[k] = v
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

	// Check root directory
	if err := checkDir(workDir); err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", workDir, err)
	}

	// Check immediate subdirectories
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", workDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".git" && entry.Name() != ".idea" && entry.Name() != ".planner" && entry.Name() != "node_modules" {
			subDir := filepath.Join(workDir, entry.Name())
			// Ignore errors in subdirectories to keep going
			_ = checkDir(subDir)
		}
	}

	return projects, nil
}
