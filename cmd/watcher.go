package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"sdlc/engine"
)

// FileChangeEvent represents a debounced file change detected within a watched project.
type FileChangeEvent struct {
	ProjectPath string // the module root that contains the changed file
	FilePath    string // the full path of the changed file
}

// Watcher wraps fsnotify to monitor project directories for file changes,
// debouncing rapid events and mapping changed files back to their owning project.
type Watcher struct {
	fsWatcher        *fsnotify.Watcher
	events           chan FileChangeEvent
	debouncers       map[string]*time.Timer
	debounceInterval time.Duration
	projectRoots     map[string]string // maps any watched directory path back to its owning project's AbsPath
	done             chan struct{}
	loopDone         chan struct{} // closed when eventLoop goroutine exits
	mu               sync.Mutex   // protects debouncers map
}

// NewWatcher creates a Watcher that recursively monitors all directories within
// the given projects, debouncing file change events by the specified interval.
func NewWatcher(projects []engine.Project, debounceInterval time.Duration) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	projectRoots := make(map[string]string)
	for _, p := range projects {
		if err := addDirRecursive(fw, p.AbsPath, p.AbsPath, projectRoots); err != nil {
			fw.Close()
			return nil, fmt.Errorf("failed to watch project %s: %w", p.AbsPath, err)
		}
	}

	w := &Watcher{
		fsWatcher:        fw,
		events:           make(chan FileChangeEvent),
		debouncers:       make(map[string]*time.Timer),
		debounceInterval: debounceInterval,
		projectRoots:     projectRoots,
		done:             make(chan struct{}),
		loopDone:         make(chan struct{}),
	}

	go w.eventLoop()

	return w, nil
}

// addDirRecursive adds a directory and all its subdirectories (excluding ignored ones)
// to the fsnotify watcher, recording each watched path in projectRoots.
func addDirRecursive(w *fsnotify.Watcher, dir string, projectRoot string, projectRoots map[string]string) error {
	if err := w.Add(dir); err != nil {
		return err
	}
	projectRoots[dir] = projectRoot

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() && !shouldIgnoreDir(entry.Name()) {
			if err := addDirRecursive(w, filepath.Join(dir, entry.Name()), projectRoot, projectRoots); err != nil {
				return err
			}
		}
	}

	return nil
}

// shouldIgnoreDir returns true if the directory should be excluded from watching.
// Matches the skip logic in hasChanges (cmd/commands.go lines 489-494).
func shouldIgnoreDir(name string) bool {
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}
	switch name {
	case "node_modules", "dist", "build", "target", "bin", "pkg":
		return true
	}
	return false
}

// shouldIgnoreFile returns true if the file should be excluded from change events.
// Matches the skip logic in hasChanges (cmd/commands.go lines 500-506).
func shouldIgnoreFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	for _, suffix := range []string{".log", ".tmp", ".lock", ".pid", ".swp"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// eventLoop reads from the fsnotify watcher and emits debounced FileChangeEvents.
func (w *Watcher) eventLoop() {
	defer close(w.loopDone)
	for {
		select {
		case <-w.done:
			return
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "[SDLC] Watch error: %v\n", err)
		case e, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Ignore chmod operations entirely
			if e.Op == fsnotify.Chmod {
				continue
			}

			// Handle newly created directories — watch them recursively
			if e.Op&fsnotify.Create == fsnotify.Create {
				info, err := os.Stat(e.Name)
				if err == nil && info.IsDir() {
					if !shouldIgnoreDir(info.Name()) {
						_ = addDirRecursive(w.fsWatcher, e.Name, e.Name, w.projectRoots)
						// Also try to map this new dir to an existing project root
						dir := e.Name
						for {
							parent := filepath.Dir(dir)
							if parent == dir {
								break
							}
							if root, ok := w.projectRoots[parent]; ok {
								w.projectRoots[e.Name] = root
								break
							}
							dir = parent
						}
					}
					continue
				}
			}

			// For file events (Create, Write, Remove, Rename), check if the file should be ignored
			baseName := filepath.Base(e.Name)
			if shouldIgnoreFile(baseName) {
				continue
			}

			// Resolve the event's directory to a project root
			projectPath := w.resolveProject(e.Name)
			if projectPath == "" {
				continue
			}

			// Reset the per-project debounce timer
			w.mu.Lock()
			if existing, ok := w.debouncers[projectPath]; ok {
				existing.Stop()
			}
			w.debouncers[projectPath] = time.AfterFunc(w.debounceInterval, func() {
				w.events <- FileChangeEvent{
					ProjectPath: projectPath,
					FilePath:    e.Name,
				}
			})
			w.mu.Unlock()
		}
	}
}

// resolveProject walks up from the given path to find the owning project root
// in the projectRoots map.
func (w *Watcher) resolveProject(path string) string {
	dir := filepath.Dir(path)
	for {
		if root, ok := w.projectRoots[dir]; ok {
			return root
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// Events returns the channel on which debounced file change events are delivered.
func (w *Watcher) Events() <-chan FileChangeEvent {
	return w.events
}

// Close stops the watcher, cancels all pending debounce timers, and closes
// the events channel. It waits for the eventLoop goroutine to exit before
// closing the events channel, preventing a race where a debounce timer fires
// and sends on an already-closed channel.
func (w *Watcher) Close() {
	// Signal eventLoop to stop
	close(w.done)
	// Wait for eventLoop to fully exit (it can no longer create new timers)
	<-w.loopDone
	// Now safe to stop any remaining timers and close the events channel
	w.fsWatcher.Close()
	w.mu.Lock()
	for _, t := range w.debouncers {
		t.Stop()
	}
	w.mu.Unlock()
	close(w.events)
}
