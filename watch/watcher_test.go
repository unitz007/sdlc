package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	w, err := New(Config{Debounce: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
}

func TestWatcherSingleChange(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	w, err := New(Config{Debounce: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	defer w.Close()

	if err := w.AddDir(tmpDir); err != nil {
		t.Fatalf("AddDir() returned error: %v", err)
	}

	// Write a file in the subdirectory to trigger a change.
	testFile := filepath.Join(subDir, "testfile.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Wait for the debounced change with a 2-second timeout.
	select {
	case changed := <-w.Changes():
		t.Logf("received change: %s", changed)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for change event")
	}
}

func TestWatcherDebounce(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := New(Config{Debounce: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	defer w.Close()

	if err := w.AddDir(tmpDir); err != nil {
		t.Fatalf("AddDir() returned error: %v", err)
	}

	// Rapidly write 5 files with 20ms sleep between each.
	for i := 0; i < 5; i++ {
		f := filepath.Join(tmpDir, "file"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
			t.Fatalf("failed to write file %d: %v", i, err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for the debounced change with a 2-second timeout.
	// We expect exactly 1 change event (debounced).
	select {
	case changed := <-w.Changes():
		t.Logf("received debounced change: %s", changed)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for debounced change event")
	}

	// Verify no second event arrives within a short window.
	select {
	case extra := <-w.Changes():
		t.Fatalf("received unexpected second change: %s", extra)
	case <-time.After(500 * time.Millisecond):
		// Good — no extra event.
	}
}

func TestWatcherExcludedDirs(t *testing.T) {
	tmpDir := t.TempDir()
	nmDir := filepath.Join(tmpDir, "node_modules")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatalf("failed to create node_modules directory: %v", err)
	}

	w, err := New(Config{Debounce: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	defer w.Close()

	if err := w.AddDir(tmpDir); err != nil {
		t.Fatalf("AddDir() returned error: %v", err)
	}

	// Write a file inside node_modules — should be ignored by excludedDirs.
	testFile := filepath.Join(nmDir, "pkg", "index.js")
	if err := os.MkdirAll(filepath.Dir(testFile), 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	if err := os.WriteFile(testFile, []byte("ignored"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Assert no change was received within 500ms.
	select {
	case changed := <-w.Changes():
		t.Fatalf("received unexpected change for ignored path: %s", changed)
	case <-time.After(500 * time.Millisecond):
		// Good — no event for ignored directory.
	}
}

func TestWatcherIgnorePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	ignoreDir := filepath.Join(tmpDir, "myignore")
	if err := os.MkdirAll(ignoreDir, 0755); err != nil {
		t.Fatalf("failed to create myignore directory: %v", err)
	}

	w, err := New(Config{
		Debounce:       100 * time.Millisecond,
		IgnorePatterns: []string{"myignore"},
	})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	defer w.Close()

	if err := w.AddDir(tmpDir); err != nil {
		t.Fatalf("AddDir() returned error: %v", err)
	}

	// Write a file inside myignore — should be filtered by IgnorePatterns.
	testFile := filepath.Join(ignoreDir, "data.txt")
	if err := os.WriteFile(testFile, []byte("ignored"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Assert no change was received within 500ms.
	select {
	case changed := <-w.Changes():
		t.Fatalf("received unexpected change for ignore-pattern path: %s", changed)
	case <-time.After(500 * time.Millisecond):
		// Good — no event for ignored directory.
	}
}
