package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// excludedDirs are directory names skipped during recursive AddDir and event filtering.
var excludedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"bin":          true,
	"pkg":          true,
}

// Config holds the configuration for a new Watcher.
type Config struct {
	Debounce       time.Duration
	IgnorePatterns []string
}

// Watcher wraps fsnotify.Watcher with debouncing and directory filtering.
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	debounce  time.Duration
	patterns  []string
	changes   chan string
	done      chan struct{}
	mu        sync.Mutex
	pending   map[string]struct{}
}

// New creates a new Watcher with the given configuration. It starts a
// background goroutine that reads fsnotify events, filters excluded
// directories and ignore patterns, debounces rapid changes, and sends
// representative changed paths on the Changes channel.
func New(cfg Config) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify new watcher: %w", err)
	}

	w := &Watcher{
		fsWatcher: fw,
		debounce:  cfg.Debounce,
		patterns:  cfg.IgnorePatterns,
		changes:   make(chan string),
		done:      make(chan struct{}),
		pending:   make(map[string]struct{}),
	}

	go w.eventLoop()

	return w, nil
}

// eventLoop reads from the fsnotify watcher, filters events, and debounces
// them before sending a representative path on the changes channel.
func (w *Watcher) eventLoop() {
	var timer *time.Timer

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			if w.shouldIgnore(event.Name) {
				continue
			}
			w.mu.Lock()
			w.pending[event.Name] = struct{}{}
			if timer != nil {
				timer.Stop()
			}
			deb := w.debounce
			w.mu.Unlock()

			timer = time.AfterFunc(deb, func() {
				w.mu.Lock()
				defer w.mu.Unlock()
				if len(w.pending) == 0 {
					return
				}
				// Send one representative changed path.
				var representative string
				for p := range w.pending {
					representative = p
					break
				}
				// Clear pending map.
				for k := range w.pending {
					delete(w.pending, k)
				}
				select {
				case w.changes <- representative:
				case <-w.done:
				}
			})

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "[SDLC] watcher error: %v\n", err)
		}
	}
}

// shouldIgnore returns true if the path is inside an excluded directory or
// matches one of the configured ignore patterns.
func (w *Watcher) shouldIgnore(path string) bool {
	// Check excluded directory names in the path components.
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if excludedDirs[part] {
			return true
		}
	}
	// Check ignore patterns.
	for _, pat := range w.patterns {
		if strings.Contains(path, pat) {
			return true
		}
	}
	return false
}

// AddDir adds the given directory and all its subdirectories (excluding
// the standard excluded directory names) to the fsnotify watcher.
func (w *Watcher) AddDir(dir string) error {
	if err := w.fsWatcher.Add(dir); err != nil {
		return fmt.Errorf("watcher add %s: %w", dir, err)
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors (e.g. permission denied)
		}
		if !d.IsDir() {
			return nil
		}
		if path == dir {
			return nil // already added above
		}
		if excludedDirs[d.Name()] {
			return filepath.SkipDir
		}
		return w.fsWatcher.Add(path)
	})
}

// Changes returns the channel on which debounced change notifications are sent.
// Each notification is a representative file path that triggered the change.
func (w *Watcher) Changes() <-chan string {
	return w.changes
}

// Close stops the watcher, cleaning up all resources.
func (w *Watcher) Close() error {
	close(w.done)
	err := w.fsWatcher.Close()
	close(w.changes)
	return err
}
