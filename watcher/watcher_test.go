package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWatcher_DetectsFileChange verifies that modifying a file in a watched
// project triggers the onChange callback within a reasonable timeout.
func TestWatcher_DetectsFileChange(t *testing.T) {
	var mu sync.Mutex
	var events []ChangeEvent

	w, err := NewWatcher(100*time.Millisecond, func(e ChangeEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	if err := w.AddProject("myproject", tmpDir); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Watch(ctx)
	}()

	// Give the watcher a moment to start
	time.Sleep(50 * time.Millisecond)

	// Create and modify a file
	testFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait for the debounced callback
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 {
		t.Fatal("expected onChange to be called, got 0 events")
	}
	if events[0].ProjectPath != "myproject" {
		t.Errorf("expected ProjectPath %q, got %q", "myproject", events[0].ProjectPath)
	}
	if events[0].FilePath != testFile {
		t.Errorf("expected FilePath %q, got %q", testFile, events[0].FilePath)
	}
}

// TestWatcher_IgnoresExcludedDirs verifies that modifying files inside
// node_modules/ and vendor/ does NOT trigger the onChange callback.
func TestWatcher_IgnoresExcludedDirs(t *testing.T) {
	var count int32

	w, err := NewWatcher(100*time.Millisecond, func(e ChangeEvent) {
		atomic.AddInt32(&count, 1)
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()

	// Create excluded directories with files
	for _, dir := range []string{"node_modules", "vendor"} {
		dirPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dirPath, "lib.js"), []byte("lib"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	if err := w.AddProject("myproject", tmpDir); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Watch(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Modify files inside excluded directories
	if err := os.WriteFile(filepath.Join(tmpDir, "node_modules", "lib.js"), []byte("changed"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "vendor", "lib.js"), []byte("changed"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait long enough for any debounce to fire
	time.Sleep(500 * time.Millisecond)

	if got := atomic.LoadInt32(&count); got != 0 {
		t.Errorf("expected 0 onChange calls for ignored dirs, got %d", got)
	}
}

// TestWatcher_Debounce verifies that rapid successive file modifications
// result in only a single onChange callback after the debounce period.
func TestWatcher_Debounce(t *testing.T) {
	var mu sync.Mutex
	var events []ChangeEvent

	w, err := NewWatcher(200*time.Millisecond, func(e ChangeEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	if err := w.AddProject("myproject", tmpDir); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Watch(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Rapidly modify the same file 5 times
	testFile := filepath.Join(tmpDir, "app.go")
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		time.Sleep(20 * time.Millisecond) // short interval between writes
	}

	// Wait for debounce to settle
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 1 {
		t.Errorf("expected exactly 1 onChange call after debounce, got %d", len(events))
	}
}

// TestWatcher_NewDirectoryWatched verifies that a new subdirectory created
// after watching starts is automatically picked up, and file changes within
// it are detected.
func TestWatcher_NewDirectoryWatched(t *testing.T) {
	var mu sync.Mutex
	var events []ChangeEvent

	w, err := NewWatcher(100*time.Millisecond, func(e ChangeEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	if err := w.AddProject("myproject", tmpDir); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Watch(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Create a new subdirectory after watching has started
	newDir := filepath.Join(tmpDir, "newpkg")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Give fsnotify a moment to process the Create event and add the new dir
	time.Sleep(100 * time.Millisecond)

	// Create a file inside the new directory
	newFile := filepath.Join(newDir, "helper.go")
	if err := os.WriteFile(newFile, []byte("package newpkg\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait for the debounced callback
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 {
		t.Fatal("expected onChange to be called for file in new directory, got 0 events")
	}
	if events[0].FilePath != newFile {
		t.Errorf("expected FilePath %q, got %q", newFile, events[0].FilePath)
	}
}

// TestWatcher_ContextCancellation verifies that cancelling the context
// causes Watch() to return cleanly without error.
func TestWatcher_ContextCancellation(t *testing.T) {
	w, err := NewWatcher(100*time.Millisecond, func(e ChangeEvent) {})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	tmpDir := t.TempDir()
	if err := w.AddProject("myproject", tmpDir); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- w.Watch(ctx)
	}()

	// Give the watcher a moment to start its loop
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected Watch() to return nil on context cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch() did not return within 2 seconds after context cancellation")
	}
}
