package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// mockWriter implements threadSafeWriter for testing.
type mockWriter struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (m *mockWriter) write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.Write(b)
}

func (m *mockWriter) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

// TestSharedPrefixWriter_SingleLine tests that a simple line gets the prefix.
func TestSharedPrefixWriter_SingleLine(t *testing.T) {
	mock := &mockWriter{}
	spw := NewSharedPrefixWriter(mock)
	sw := spw.SourceWriter("[MOD] ")

	_, err := sw.Write([]byte("hello world\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got := mock.String()
	want := "[MOD] hello world\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSharedPrefixWriter_MultipleLines tests multiple lines in a single Write.
func TestSharedPrefixWriter_MultipleLines(t *testing.T) {
	mock := &mockWriter{}
	spw := NewSharedPrefixWriter(mock)
	sw := spw.SourceWriter("[APP] ")

	_, err := sw.Write([]byte("line one\nline two\nline three\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got := mock.String()
	want := "[APP] line one\n[APP] line two\n[APP] line three\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSharedPrefixWriter_PartialLine tests that data without a trailing newline
// is buffered and flushed on Flush().
func TestSharedPrefixWriter_PartialLine(t *testing.T) {
	mock := &mockWriter{}
	spw := NewSharedPrefixWriter(mock)
	sw := spw.SourceWriter("[SRV] ")

	// Write without newline
	_, err := sw.Write([]byte("partial"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Nothing should be written yet
	if mock.String() != "" {
		t.Errorf("expected no output yet, got %q", mock.String())
	}

	// Flush should output the partial line
	err = sw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	got := mock.String()
	want := "[SRV] partial"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSharedPrefixWriter_LineSplitAcrossWrites tests that a line split across
// multiple Write calls is assembled correctly.
func TestSharedPrefixWriter_LineSplitAcrossWrites(t *testing.T) {
	mock := &mockWriter{}
	spw := NewSharedPrefixWriter(mock)
	sw := spw.SourceWriter("[DB] ")

	_, err := sw.Write([]byte("hello "))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	_, err = sw.Write([]byte("world\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got := mock.String()
	want := "[DB] hello world\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSharedPrefixWriter_ConcurrentWrites tests that concurrent writes from
// multiple goroutines do not interleave at the byte level. Each goroutine
// writes a known pattern, and we verify every line is intact.
func TestSharedPrefixWriter_ConcurrentWrites(t *testing.T) {
	mock := &mockWriter{}
	spw := NewSharedPrefixWriter(mock)

	const numGoroutines = 10
	const linesPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		prefix := fmt.Sprintf("[G%02d] ", i)
		sw := spw.SourceWriter(prefix)

		go func(gid int) {
			defer wg.Done()
			for j := 0; j < linesPerGoroutine; j++ {
				msg := fmt.Sprintf("line %d from goroutine %d\n", j, gid)
				if _, err := sw.Write([]byte(msg)); err != nil {
					t.Errorf("goroutine %d Write failed: %v", gid, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Flush all writers to ensure partial lines are handled.
	// Note: in real usage, Flush should be called per SourceWriter.
	// Here all lines end with \n so there should be nothing to flush.

	output := mock.String()
	lines := strings.Split(output, "\n")
	// Last element after split is empty string if output ends with \n
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	expectedLines := numGoroutines * linesPerGoroutine
	if len(lines) != expectedLines {
		t.Fatalf("expected %d lines, got %d", expectedLines, len(lines))
	}

	// Verify each line starts with a valid prefix and is well-formed.
	validPrefixes := make(map[string]bool)
	for i := 0; i < numGoroutines; i++ {
		validPrefixes[fmt.Sprintf("[G%02d] ", i)] = true
	}

	for _, line := range lines {
		found := false
		for prefix := range validPrefixes {
			if strings.HasPrefix(line, prefix) {
				found = true
				// Verify the content after the prefix is correct
				gidStr := prefix[1:3] // e.g., "00", "01"
				content := line[len(prefix):]
				if !strings.HasPrefix(content, "line ") {
					t.Errorf("malformed line content: %q", line)
				}
				_ = gidStr // We've validated structure
				break
			}
		}
		if !found {
			t.Errorf("line has no valid prefix: %q", line)
		}
	}
}

// TestSharedPrefixWriter_ConcurrentWritesLargeLines tests with larger line
// content to increase the chance of detecting interleaving issues.
func TestSharedPrefixWriter_ConcurrentWritesLargeLines(t *testing.T) {
	mock := &mockWriter{}
	spw := NewSharedPrefixWriter(mock)

	const numGoroutines = 20
	const linesPerGoroutine = 30
	const lineContentLen = 200

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		prefix := fmt.Sprintf("[M%d] ", i)
		sw := spw.SourceWriter(prefix)

		go func(gid int) {
			defer wg.Done()
			content := strings.Repeat("x", lineContentLen)
			for j := 0; j < linesPerGoroutine; j++ {
				msg := fmt.Sprintf("G%d-L%03d-%s\n", gid, j, content)
				if _, err := sw.Write([]byte(msg)); err != nil {
					t.Errorf("goroutine %d Write failed: %v", gid, err)
				}
			}
		}(i)
	}

	wg.Wait()

	output := mock.String()
	lines := strings.Split(output, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	expectedLines := numGoroutines * linesPerGoroutine
	if len(lines) != expectedLines {
		t.Fatalf("expected %d lines, got %d", expectedLines, len(lines))
	}

	// Verify each line has a proper prefix and the expected length.
	for _, line := range lines {
		// Each line should be: [M<N>] G<N>-L<NNN>-<200 x's>
		if !strings.HasPrefix(line, "[M") || !strings.Contains(line, "] ") {
			t.Errorf("malformed prefix in line: %q (first 50 chars)", line[:min(50, len(line))])
			continue
		}

		// After "] " we expect "G<N>-L<NNN>-<200 x's>"
		parts := strings.SplitN(line, "] ", 2)
		if len(parts) != 2 {
			t.Errorf("malformed line structure: %q", line)
			continue
		}

		content := parts[1]
		if !strings.HasSuffix(content, strings.Repeat("x", lineContentLen)) {
			t.Errorf("line content corrupted: got len %d for content %q", len(content), content[:min(30, len(content))])
		}
	}
}

// TestSharedPrefixWriter_EmptyWrite tests that empty writes don't cause issues.
func TestSharedPrefixWriter_EmptyWrite(t *testing.T) {
	mock := &mockWriter{}
	spw := NewSharedPrefixWriter(mock)
	sw := spw.SourceWriter("[T] ")

	n, err := sw.Write([]byte{})
	if err != nil {
		t.Fatalf("empty Write failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}

	err = sw.Flush()
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if mock.String() != "" {
		t.Errorf("expected no output, got %q", mock.String())
	}
}

// TestSharedPrefixWriter_MultipleSourceWritersSamePrefix tests that multiple
// SourceWriters with the same prefix still produce clean output.
func TestSharedPrefixWriter_MultipleSourceWritersSamePrefix(t *testing.T) {
	mock := &mockWriter{}
	spw := NewSharedPrefixWriter(mock)

	sw1 := spw.SourceWriter("[SVC] ")
	sw2 := spw.SourceWriter("[SVC] ")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			sw1.Write([]byte(fmt.Sprintf("writer1 line %d\n", i)))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			sw2.Write([]byte(fmt.Sprintf("writer2 line %d\n", i)))
		}
	}()

	wg.Wait()

	output := mock.String()
	lines := strings.Split(output, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) != 40 {
		t.Fatalf("expected 40 lines, got %d", len(lines))
	}

	for _, line := range lines {
		if !strings.HasPrefix(line, "[SVC] ") {
			t.Errorf("line missing prefix: %q", line)
		}
		// Each line should be complete
		if !strings.HasPrefix(line, "[SVC] writer1 line ") && !strings.HasPrefix(line, "[SVC] writer2 line ") {
			t.Errorf("line content corrupted: %q", line)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
