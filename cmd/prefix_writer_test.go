package cmd

import (
	"strings"
	"sync"
	"testing"
)

// fakeWriter is a minimal bytes.Buffer wrapper for testing.
type fakeWriter struct {
	mu  sync.Mutex
	buf []byte
}

func (f *fakeWriter) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buf = append(f.buf, p...)
	return len(p), nil
}

func (f *fakeWriter) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return string(f.buf)
}

func (f *fakeWriter) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.buf)
}

// ---------------------------------------------------------------------------
// Basic functional tests
// ---------------------------------------------------------------------------

func TestPrefixWriter_SingleLine(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, ">>> ")

	_, err := pw.Write([]byte("hello\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != ">>> hello\n" {
		t.Errorf("got %q, want %q", buf.String(), ">>> hello\n")
	}
}

func TestPrefixWriter_MultipleLines(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, "[test] ")

	_, err := pw.Write([]byte("line one\nline two\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "[test] line one\n[test] line two\n"
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestPrefixWriter_PartialLine(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, ">> ")

	// Write a partial line — nothing should be flushed yet.
	_, err := pw.Write([]byte("partial"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "" {
		t.Errorf("expected empty output for partial line, got %q", buf.String())
	}

	// Complete the line.
	_, err = pw.Write([]byte(" text\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != ">> partial text\n" {
		t.Errorf("got %q, want %q", buf.String(), ">> partial text\n")
	}
}

func TestPrefixWriter_FlushPartialLine(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, ">>> ")

	_, err := pw.Write([]byte("no newline at end"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Flush should output the remaining partial line with a trailing newline.
	err = pw.Flush()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := ">>> no newline at end\n"
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestPrefixWriter_FlushEmpty(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, ">>> ")

	// Flushing when nothing is buffered should be a no-op.
	err := pw.Flush()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "" {
		t.Errorf("expected empty output after flush on empty buffer, got %q", buf.String())
	}
}

func TestPrefixWriter_EmptyPrefix(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, "")

	_, err := pw.Write([]byte("hello\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "hello\n" {
		t.Errorf("got %q, want %q", buf.String(), "hello\n")
	}
}

func TestPrefixWriter_EmptyWrite(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, ">>> ")

	n, err := pw.Write([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}
}

func TestPrefixWriter_MultipleWritesBeforeNewline(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, ">>> ")

	// Write in chunks that split a line.
	pw.Write([]byte("hel"))
	pw.Write([]byte("lo w"))
	pw.Write([]byte("orld\n"))

	expected := ">>> hello world\n"
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestPrefixWriter_MultipleFlushCalls(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, ">>> ")

	pw.Write([]byte("data"))
	pw.Flush()
	pw.Flush() // Second flush should be no-op

	expected := ">>> data\n"
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestPrefixWriter_MixedCompleteAndPartialLines(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, "[p] ")

	pw.Write([]byte("first line\nsecond part"))
	// "first line" should be flushed
	if buf.String() != "[p] first line\n" {
		t.Errorf("after first write, got %q", buf.String())
	}

	pw.Write([]byte(" continues\nthird\n"))
	// Now "second part continues" and "third" should be flushed
	expected := "[p] first line\n[p] second part continues\n[p] third\n"
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}

	// Nothing should remain in the buffer
	pw.Flush()
	if buf.String() != expected {
		t.Errorf("after flush, got %q, want %q", buf.String(), expected)
	}
}

func TestPrefixWriter_OnlyNewlines(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, ">>> ")

	pw.Write([]byte("\n\n"))
	expected := ">>> \n>>> \n"
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestPrefixWriter_ColoredPrefix(t *testing.T) {
	var buf fakeWriter
	prefix := "\033[32m[mymod]\033[0m "
	pw := NewPrefixWriter(&buf, prefix)

	pw.Write([]byte("output line\n"))
	expected := "\033[32m[mymod]\033[0m output line\n"
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

// ---------------------------------------------------------------------------
// Concurrency tests — verify thread safety under parallel writes
// ---------------------------------------------------------------------------

func TestPrefixWriter_ConcurrentWritesNoGarbling(t *testing.T) {
	var buf fakeWriter
	prefixA := "[moduleA] "
	prefixB := "[moduleB] "

	pwA := NewPrefixWriter(&buf, prefixA)
	pwB := NewPrefixWriter(&buf, prefixB)

	var wg sync.WaitGroup
	const linesPerWriter = 50

	// Concurrently write lines from two writers.
	for i := 0; i < linesPerWriter; i++ {
		wg.Add(2)

		go func(idx int) {
			defer wg.Done()
			msg := []byte(fmt.Sprintf("log line %03d\n", idx))
			if _, err := pwA.Write(msg); err != nil {
				t.Errorf("pwA write error: %v", err)
			}
		}(i)

		go func(idx int) {
			defer wg.Done()
			msg := []byte(fmt.Sprintf("event %03d\n", idx))
			if _, err := pwB.Write(msg); err != nil {
				t.Errorf("pwB write error: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Flush any remaining partial lines.
	pwA.Flush()
	pwB.Flush()

	output := buf.String()

	// Verify every output line is properly prefixed — no garbling.
	lines := strings.Split(output, "\n")
	// The last element may be empty from the trailing newline.
	for _, line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "[moduleA] ") && !strings.HasPrefix(line, "[moduleB] ") {
			t.Errorf("garbled line detected (no valid prefix): %q", line)
		}
	}

	// Verify total count.
	prefixACount := strings.Count(output, "[moduleA] ")
	prefixBCount := strings.Count(output, "[moduleB] ")
	if prefixACount != linesPerWriter {
		t.Errorf("expected %d lines from moduleA, got %d", linesPerWriter, prefixACount)
	}
	if prefixBCount != linesPerWriter {
		t.Errorf("expected %d lines from moduleB, got %d", linesPerWriter, prefixBCount)
	}
}

func TestPrefixWriter_ConcurrentWritesWithPartialLines(t *testing.T) {
	var buf fakeWriter
	pw := NewPrefixWriter(&buf, "[worker] ")

	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Write partial then complete.
			pw.Write([]byte(fmt.Sprintf("partial-%d", id)))
			pw.Write([]byte(fmt.Sprintf("-complete-%d\n", id)))
		}(i)
	}

	wg.Wait()
	pw.Flush()

	output := buf.String()

	// Each goroutine should produce exactly one line.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != goroutines {
		t.Errorf("expected %d lines, got %d", goroutines, len(lines))
	}

	for _, line := range lines {
		if !strings.HasPrefix(line, "[worker] ") {
			t.Errorf("garbled line (missing prefix): %q", line)
		}
		// Verify the line content is intact.
		content := strings.TrimPrefix(line, "[worker] ")
		if !strings.HasPrefix(content, "partial-") || !strings.Contains(content, "-complete-") {
			t.Errorf("line content appears garbled: %q", content)
		}
	}
}

func TestPrefixWriter_ConcurrentFlush(t *testing.T) {
	var buf fakeWriter
	pwA := NewPrefixWriter(&buf, "[A] ")
	pwB := NewPrefixWriter(&buf, "[B] ")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			pwA.Write([]byte("msg\n"))
			pwA.Flush()
		}()

		go func() {
			defer wg.Done()
			pwB.Write([]byte("msg\n"))
			pwB.Flush()
		}()
	}

	wg.Wait()

	output := buf.String()
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "[A] ") && !strings.HasPrefix(line, "[B] ") {
			t.Errorf("garbled line: %q", line)
		}
	}

	aCount := strings.Count(output, "[A] msg\n")
	bCount := strings.Count(output, "[B] msg\n")
	if aCount != 20 {
		t.Errorf("expected 20 lines from A, got %d", aCount)
	}
	if bCount != 20 {
		t.Errorf("expected 20 lines from B, got %d", bCount)
	}
}
