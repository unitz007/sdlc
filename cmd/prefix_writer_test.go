package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Basic correctness
// ---------------------------------------------------------------------------

func TestPrefixWriter_SingleLine(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPrefixWriter(&buf, "[test] ")

	pw.Write([]byte("hello\n"))
	pw.Flush()

	got := buf.String()
	want := "[test] hello\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrefixWriter_MultipleLines(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPrefixWriter(&buf, "[app] ")

	pw.Write([]byte("line one\nline two\nline three\n"))
	pw.Flush()

	got := buf.String()
	want := "[app] line one\n[app] line two\n[app] line three\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrefixWriter_SplitAcrossWrites(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPrefixWriter(&buf, "[svc] ")

	// Write one line in three chunks.
	pw.Write([]byte("hel"))
	pw.Write([]byte("lo wo"))
	pw.Write([]byte("rld\n"))
	pw.Flush()

	got := buf.String()
	want := "[svc] hello world\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrefixWriter_SplitAtNewline(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPrefixWriter(&buf, "[a] ")

	// First write ends exactly at the newline.
	pw.Write([]byte("first line\nsec"))
	pw.Write([]byte("ond line\n"))
	pw.Flush()

	got := buf.String()
	want := "[a] first line\n[a] second line\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrefixWriter_EmptyWrite(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPrefixWriter(&buf, "[x] ")

	pw.Write([]byte{})
	pw.Write(nil)
	pw.Flush()

	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

// ---------------------------------------------------------------------------
// Flush behaviour
// ---------------------------------------------------------------------------

func TestPrefixWriter_FlushPartialLine(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPrefixWriter(&buf, "[p] ")

	pw.Write([]byte("no newline"))
	pw.Flush()

	got := buf.String()
	want := "[p] no newline\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrefixWriter_FlushIdempotent(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPrefixWriter(&buf, "[i] ")

	pw.Write([]byte("done\n"))
	pw.Flush()
	pw.Flush()
	pw.Flush()

	got := buf.String()
	want := "[i] done\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrefixWriter_FlushAfterCompleteLines(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPrefixWriter(&buf, "[c] ")

	pw.Write([]byte("line one\nline two\n"))
	// The buffer should be empty after Write (both lines complete).
	pw.Flush() // should be a no-op for buffered data

	got := buf.String()
	want := "[c] line one\n[c] line two\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Concurrency stress tests
// ---------------------------------------------------------------------------

func TestPrefixWriter_ConcurrentWritesNoGarbling(t *testing.T) {
	var buf bytes.Buffer
	pw1 := NewPrefixWriter(&buf, "[alpha] ")
	pw2 := NewPrefixWriter(&buf, "[beta] ")

	const linesPerWriter = 50
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer 1
	go func() {
		defer wg.Done()
		for i := 0; i < linesPerWriter; i++ {
			pw1.Write([]byte(fmt.Sprintf("line %03d\n", i)))
		}
		pw1.Flush()
	}()

	// Writer 2
	go func() {
		defer wg.Done()
		for i := 0; i < linesPerWriter; i++ {
			pw2.Write([]byte(fmt.Sprintf("line %03d\n", i)))
		}
		pw2.Flush()
	}()

	wg.Wait()

	// Verify every line is atomic — starts with the correct prefix and ends
	// with a newline, never interleaved.
	lines := strings.Split(buf.String(), "\n")
	// Last element after Split is empty string (trailing newline).
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) != linesPerWriter*2 {
		t.Fatalf("expected %d lines, got %d", linesPerWriter*2, len(lines))
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "[alpha] ") && !strings.HasPrefix(line, "[beta] ") {
			t.Errorf("line %d has unexpected prefix: %q", i, line)
		}
	}
}

func TestPrefixWriter_ConcurrentWritesWithPartialLines(t *testing.T) {
	var buf bytes.Buffer
	pw1 := NewPrefixWriter(&buf, "[A] ")
	pw2 := NewPrefixWriter(&buf, "[B] ")

	const numGoroutines = 10
	const writesPer = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		pw := pw1
		if g%2 == 1 {
			pw = pw2
		}
		go func(pw *PrefixWriter, id int) {
			defer wg.Done()
			for i := 0; i < writesPer; i++ {
				// Split each write into two chunks to test partial line buffering.
				part1 := fmt.Sprintf("goroutine-%d msg-%d ", id, i)
				part2 := fmt.Sprintf("value %d\n", id*i)
				pw.Write([]byte(part1))
				pw.Write([]byte(part2))
			}
			pw.Flush()
		}(pw, g)
	}

	wg.Wait()

	// Verify no line interleaving.
	lines := strings.Split(buf.String(), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	expected := numGoroutines * writesPer
	if len(lines) != expected {
		t.Fatalf("expected %d lines, got %d", expected, len(lines))
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "[A] ") && !strings.HasPrefix(line, "[B] ") {
			t.Errorf("line %d has unexpected prefix: %q", i, line)
		}
	}
}

func TestPrefixWriter_ConcurrentFlush(t *testing.T) {
	var buf bytes.Buffer
	pw := NewPrefixWriter(&buf, "[f] ")

	// Write data with no trailing newline, then have many goroutines flush concurrently.
	pw.Write([]byte("partial data"))
	pw.Write([]byte(" still no newline"))

	var wg sync.WaitGroup
	const flushers = 20
	wg.Add(flushers)
	for i := 0; i < flushers; i++ {
		go func() {
			defer wg.Done()
			pw.Flush()
		}()
	}
	wg.Wait()

	got := buf.String()
	// Flush should have written the partial content exactly once (the first
	// Flush resets the buffer; subsequent ones are no-ops).
	want := "[f] partial data still no newline\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
