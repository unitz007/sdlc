package lib

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestBufferedPrefixWriter_SingleLine(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()
	pw := NewBufferedPrefixWriter(&buf, "[test] ", lock)

	n, err := pw.Write([]byte("hello world\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 12 {
		t.Errorf("expected 12 bytes written, got %d", n)
	}

	expected := "[test] hello world\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestBufferedPrefixWriter_MultipleLines(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()
	pw := NewBufferedPrefixWriter(&buf, "[mod] ", lock)

	_, err := pw.Write([]byte("line one\nline two\nline three\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "[mod] line one\n[mod] line two\n[mod] line three\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestBufferedPrefixWriter_PartialLine(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()
	pw := NewBufferedPrefixWriter(&buf, "[app] ", lock)

	// Write partial line
	_, err := pw.Write([]byte("partial"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Nothing should be written yet
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer, got %q", buf.String())
	}

	// Complete the line
	_, err = pw.Write([]byte(" message\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "[app] partial message\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestBufferedPrefixWriter_Flush(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()
	pw := NewBufferedPrefixWriter(&buf, "[srv] ", lock)

	// Write without newline
	_, err := pw.Write([]byte("no newline"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected empty buffer before flush, got %q", buf.String())
	}

	// Flush should write the partial line
	err = pw.Flush()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "[srv] no newline"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestBufferedPrefixWriter_FlushEmpty(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()
	pw := NewBufferedPrefixWriter(&buf, "[x] ", lock)

	err := pw.Flush()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer, got %q", buf.String())
	}
}

func TestBufferedPrefixWriter_MultipleWritesAndFlush(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()
	pw := NewBufferedPrefixWriter(&buf, "[a] ", lock)

	_, _ = pw.Write([]byte("first\n"))
	_, _ = pw.Write([]byte("second line part one "))
	_, _ = pw.Write([]byte("part two\n"))
	_, _ = pw.Write([]byte("incomplete"))

	err := pw.Flush()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "[a] first\n[a] second line part one part two\n[a] incomplete"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestBufferedPrefixWriter_ConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()

	// Create multiple writers with different prefixes sharing the same lock
	pw1 := NewBufferedPrefixWriter(&buf, "[mod1] ", lock)
	pw2 := NewBufferedPrefixWriter(&buf, "[mod2] ", lock)
	pw3 := NewBufferedPrefixWriter(&buf, "[mod3] ", lock)

	var wg sync.WaitGroup
	linesPerWriter := 100

	for _, pw := range []*BufferedPrefixWriter{pw1, pw2, pw3} {
		wg.Add(1)
		go func(w *BufferedPrefixWriter) {
			defer wg.Done()
			for i := 0; i < linesPerWriter; i++ {
				msg := fmt.Sprintf("message %d\n", i)
				_, _ = w.Write([]byte(msg))
			}
		}(pw)
	}

	wg.Wait()

	// Verify the output: every line must start with exactly one prefix
	// and no two prefixes appear on the same line (no interleaving)
	output := buf.String()
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")

	if len(lines) != 3*linesPerWriter {
		t.Fatalf("expected %d lines, got %d", 3*linesPerWriter, len(lines))
	}

	prefixes := []string{"[mod1] ", "[mod2] ", "[mod3] "}
	for i, line := range lines {
		prefixed := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(line, prefix) {
				// Verify no other prefix appears in the middle of the line
				rest := line[len(prefix):]
				for _, other := range prefixes {
					if strings.Contains(rest, other) {
						t.Errorf("line %d has interleaved prefix: %q", i, line)
					}
				}
				prefixed = true
				break
			}
		}
		if !prefixed {
			t.Errorf("line %d does not start with any known prefix: %q", i, line)
		}
	}
}

func TestBufferedPrefixWriter_ConcurrentWritesWithFlush(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()

	pw1 := NewBufferedPrefixWriter(&buf, "[alpha] ", lock)
	pw2 := NewBufferedPrefixWriter(&buf, "[beta] ", lock)

	var wg sync.WaitGroup
	messagesPerWriter := 50

	for _, pw := range []*BufferedPrefixWriter{pw1, pw2} {
		wg.Add(1)
		go func(w *BufferedPrefixWriter) {
			defer wg.Done()
			for i := 0; i < messagesPerWriter; i++ {
				// Write some complete lines
				_, _ = w.Write([]byte(fmt.Sprintf("line %d\n", i)))
			}
			// Write a partial line and flush
			_, _ = w.Write([]byte("final"))
			_ = w.Flush()
		}(pw)
	}

	wg.Wait()

	output := buf.String()
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")

	// Each writer produces messagesPerWriter complete lines + 1 flushed partial = messagesPerWriter + 1
	expectedLines := 2 * (messagesPerWriter + 1)
	if len(lines) != expectedLines {
		t.Fatalf("expected %d lines, got %d", expectedLines, len(lines))
	}

	prefixes := []string{"[alpha] ", "[beta] "}
	for i, line := range lines {
		prefixed := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(line, prefix) {
				prefixed = true
				break
			}
		}
		if !prefixed {
			t.Errorf("line %d missing prefix: %q", i, line)
		}
	}
}

func TestBufferedPrefixWriter_EmptyWrite(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()
	pw := NewBufferedPrefixWriter(&buf, "[x] ", lock)

	n, err := pw.Write([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer, got %q", buf.String())
	}
}

func TestBufferedPrefixWriter_WriteByteByByte(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()
	pw := NewBufferedPrefixWriter(&buf, "[dev] ", lock)

	// Write a complete line one byte at a time
	for _, b := range []byte("byte by byte\n") {
		_, err := pw.Write([]byte{b})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Only the complete line should appear
	expected := "[dev] byte by byte\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestBufferedPrefixWriter_MultipleNewlines(t *testing.T) {
	var buf bytes.Buffer
	lock := NewOutputLock()
	pw := NewBufferedPrefixWriter(&buf, "[x] ", lock)

	_, err := pw.Write([]byte("a\n\nb\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "[x] a\n[x] \n[x] b\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}
