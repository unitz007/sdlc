// Package watcher provides fsnotify-based file watching with per-project debouncing
// and configurable directory ignore patterns.
package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ChangeEvent represents a file change detected within a watched project.
type ChangeEvent struct {
	ProjectPath string // relative path like "backend"
	FilePath    string // absolute path of the changed file
}

// Watcher monitors project directories for file changes using fsnotify,
// debouncing events per-project and skipping ignored directories.
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	projects  map[string]string // project relative path → absolute path
	debounce  time.Duration
	timers    map[string]*time.Timer // per-project debounce timers
	mu        sync.Mutex             // protects timers map
	onChange  func(ChangeEvent)
	ignoreDirs []string
}

// defaultIgnoreDirs is the list of directory names to skip during recursive
// watching and event processing.
var defaultIgnoreDirs = []string{
	"node_modules", ".git", "vendor", "dist", "build",
	"target", "bin", "pkg", ".idea", ".sdlc",
}

// NewWatcher creates a new Watcher with the given debounce interval and
// change callback. The debounce duration controls how long to wait after
// the last change in a project before firing the callback.
func NewWatcher(debounce time.Duration, onChange func(ChangeEvent)) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify watcher: %w", err)
	}

	ignoreDirs := make([]string, len(defaultIgnoreDirs))
	copy(ignoreDirs, defaultIgnoreDirs)

	return &Watcher{
		fsWatcher:  fw,
		projects:   make(map[string]string),
		debounce:   debounce,
		timers:     make(map[string]*time.Timer),
		onChange:   onChange,
		ignoreDirs: ignoreDirs,
	}, nil
}

// AddProject registers a project for watching. projectPath is the relative
// path (e.g. "backend") and absPath is the absolute filesystem path.
// It recursively adds all subdirectories except those matching ignoreDirs.
func (w *Watcher) AddProject(projectPath, absPath string) error {
	w.projects[projectPath] = absPath
	return w.addRecursive(absPath)
}

// addRecursive walks the directory tree rooted at dir, adding each
// non-ignored subdirectory to the fsnotify watcher.
func (w *Watcher) addRecursive(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors (e.g. permission denied)
		}

		if d.IsDir() {
			if w.shouldIgnore(d.Name()) {
				return filepath.SkipDir
			}
			if err := w.fsWatcher.Add(path); err != nil {
				// Log but don't fail — some dirs may be unreadable
				fmt.Fprintf(os.Stderr, "[SDLC] warning: cannot watch %s: %v\n", path, err)
			}
		}
		return nil
	})
}

// Watch starts the main event loop. It blocks until ctx is cancelled,
// processing file change events from fsnotify with per-project debouncing.
func (w *Watcher) Watch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event)
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "[SDLC] watcher error: %v\n", err)
		}
	}
}

// handleEvent processes a single fsnotify event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// If a new directory was created, watch it recursively and skip
	// further processing — directory creation itself is not a file change.
	if event.Op&fsnotify.Create == fsnotify.Create {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if !w.shouldIgnore(filepath.Base(event.Name)) {
				_ = w.addRecursive(event.Name)
			}
			return
		}
	}

	// Skip events in ignored directories
	if w.shouldIgnorePath(event.Name) {
		return
	}

	// Resolve which project this file belongs to
	projectPath := w.resolveProject(event.Name)
	if projectPath == "" {
		return
	}

	// Debounce: reset the timer for this project
	w.mu.Lock()
	defer w.mu.Unlock()

	if existing, ok := w.timers[projectPath]; ok {
		existing.Stop()
	}

	w.timers[projectPath] = time.AfterFunc(w.debounce, func() {
		w.onChange(ChangeEvent{
			ProjectPath: projectPath,
			FilePath:    event.Name,
		})
	})
}

// Close stops the fsnotify watcher and cancels any pending debounce timers.
func (w *Watcher) Close() error {
	w.mu.Lock()
	for key, t := range w.timers {
		t.Stop()
		delete(w.timers, key)
	}
	w.mu.Unlock()
	return w.fsWatcher.Close()
}

// shouldIgnore returns true if the directory name matches an ignored pattern.
func (w *Watcher) shouldIgnore(name string) bool {
	for _, d := range w.ignoreDirs {
		if name == d {
			return true
		}
	}
	return false
}

// shouldIgnorePath returns true if any segment of the path matches an ignored directory.
func (w *Watcher) shouldIgnorePath(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if w.shouldIgnore(segment) {
			return true
		}
	}
	return false
}

// resolveProject finds the project whose absolute path is the longest
// prefix of the given absolute file path. This handles nested projects
// correctly by preferring the most specific match.
func (w *Watcher) resolveProject(absFilePath string) string {
	var bestMatch string
	bestLen := 0

	for relPath, absPath := range w.projects {
		// Ensure the project path ends with a separator for proper prefix matching
		prefix := absPath
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}

		if strings.HasPrefix(absFilePath, prefix) && len(prefix) > bestLen {
			bestLen = len(prefix)
			bestMatch = relPath
		}
	}

	return bestMatch
}

// sortedProjectPaths returns project paths sorted by absolute path length
// descending, used internally for deterministic longest-prefix matching.
func (w *Watcher) sortedProjectPaths() []string {
	paths := make([]string, 0, len(w.projects))
	for p := range w.projects {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		return len(w.projects[paths[i]]) > len(w.projects[paths[j]])
	})
	return paths
}
