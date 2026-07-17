package lib

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestSyncWriter_SingleLine(t *testing.T) {
	var buf bytes.Buffer
	sw := NewSyncWriter(&buf, "[mod] ")

	n, err := sw.Write([]byte("hello world\n"))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != 12 {
		t.Fatalf("Write returned %d, want 12", n)
	}

	got := buf.String()
	want := "[mod] hello world\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSyncWriter_PartialThenComplete(t *testing.T) {
	var buf bytes.Buffer
	sw := NewSyncWriter(&buf, "[mod] ")

	sw.Write([]byte("hel"))
	sw.Write([]byte("lo wo"))
	sw.Write([]byte("rld\n"))

	got := buf.String()
	want := "[mod] hello world\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSyncWriter_MultipleLines(t *testing.T) {
	var buf bytes.Buffer
	sw := NewSyncWriter(&buf, "[mod] ")

	sw.Write([]byte("line one\nline two\n"))

	got := buf.String()
	want := "[mod] line one\n[mod] line two\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSyncWriter_FlushPartial(t *testing.T) {
	var buf bytes.Buffer
	sw := NewSyncWriter(&buf, "[mod] ")

	sw.Write([]byte("partial line"))
	err := sw.Flush()
	if err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}

	got := buf.String()
	want := "[mod] partial line\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSyncWriter_FlushEmpty(t *testing.T) {
	var buf bytes.Buffer
	sw := NewSyncWriter(&buf, "[mod] ")

	err := sw.Flush()
	if err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected empty buffer after Flush on empty writer, got %q", buf.String())
	}
}

func TestSyncWriter_ConcurrentSafety(t *testing.T) {
	var buf bytes.Buffer
	sw := NewSyncWriter(&buf, "[mod] ")

	var wg sync.WaitGroup
	lines := 100
	repeats := 50

	for i := 0; i < lines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			msg := fmt.Sprintf("line-%03d from goroutine\n", id)
			for j := 0; j < repeats; j++ {
				sw.Write([]byte(msg))
			}
		}(i)
	}
	wg.Wait()

	// Every output line should start with the prefix.
	result := buf.String()
	outputLines := strings.Split(result, "\n")
	// Last element after split may be empty
	if outputLines[len(outputLines)-1] == "" {
		outputLines = outputLines[:len(outputLines)-1]
	}

	for i, line := range outputLines {
		if !strings.HasPrefix(line, "[mod] ") {
			t.Errorf("line %d does not start with prefix: %q", i, line)
		}
	}

	// We expect exactly lines * repeats output lines.
	if len(outputLines) != lines*repeats {
		t.Errorf("expected %d output lines, got %d", lines*repeats, len(outputLines))
	}
}

func TestSyncWriter_ChunkedWrites(t *testing.T) {
	// Simulate how a PTY/process might emit data in arbitrary chunks
	var buf bytes.Buffer
	sw := NewSyncWriter(&buf, "[mod] ")

	data := "A quick brown fox\njumps over\nthe lazy dog\n"
	// Write in small chunks that split mid-line and mid-word
	chunks := []string{
		"A qui",
		"ck brown f",
		"ox\njump",
		"s over\n",
		"the la",
		"zy dog\n",
	}

	for _, chunk := range chunks {
		sw.Write([]byte(chunk))
	}

	got := buf.String()
	want := "[mod] A quick brown fox\n[mod] jumps over\n[mod] the lazy dog\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
