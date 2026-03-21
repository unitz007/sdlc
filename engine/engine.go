package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sdlc/config"
	"sdlc/lib"
	"sort"
	"strings"
)

// Project represents a detected project with its location and task definition
type Project struct {
	Name    string   // Name of the build file (e.g. go.mod)
	Path    string   // Relative path to the directory containing the build file
	AbsPath string   // Absolute path to the directory
	Task    lib.Task // The task definition
}

// skipDirs contains directory names that should never be scanned for build files.
var skipDirs = map[string]bool{
	".git":        true,
	".idea":       true,
	".planner":    true,
	"node_modules": true,
	"vendor":      true,
	"dist":        true,
	"build":       true,
	"target":      true,
	"bin":         true,
	"pkg":         true,
	".vscode":     true,
	".zed":        true,
	".kael_index": true,
}

// DetectProjects recursively walks the working directory tree
// for known build files defined in the config.
// It returns a list of detected projects sorted by path.
func DetectProjects(workDir string, tasks map[string]lib.Task) ([]Project, error) {
	var projects []Project
	seenDirs := make(map[string]bool)

	err := filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip non-directory entries
		if !d.IsDir() {
			return nil
		}

		// Skip directories in the skipDirs list
		if skipDirs[d.Name()] {
			return filepath.SkipDir
		}

		// Skip dot-directories (except the root workDir itself)
		if strings.HasPrefix(d.Name(), ".") && path != workDir {
			return filepath.SkipDir
		}

		// Resolve symlinks and track seen directories
		absDir, err := filepath.Abs(path)
		if err != nil {
			return nil
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
		localTasks, err := config.LoadLocal(path)
		if err != nil {
			fmt.Printf("Warning: failed to read local config in %s: %v\n", path, err)
		}

		// Merge with global tasks
		effectiveTasks := tasks
		if len(localTasks) > 0 {
			effectiveTasks = make(map[string]lib.Task)
			for k, v := range tasks {
				effectiveTasks[k] = v
			}
			for k, v := range localTasks {
				effectiveTasks[k] = v
			}
		}

		// Read directory entries and match build files
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if task, ok := effectiveTasks[entry.Name()]; ok {
				// Enforce one project per directory to avoid duplicates
				exists := false
				for _, p := range projects {
					if p.AbsPath == path {
						exists = true
						break
					}
				}
				if exists {
					continue
				}

				relPath, err := filepath.Rel(workDir, path)
				if err != nil {
					relPath = path
				}

				projects = append(projects, Project{
					Name:    entry.Name(),
					Path:    relPath,
					AbsPath: path,
					Task:    task,
				})
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", workDir, err)
	}

	// Sort projects by Path for deterministic ordering
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})

	return projects, nil
}
