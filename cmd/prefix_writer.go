package cmd

import (
	"bytes"
	"io"
	"sync"
)

// globalOutputMutex is a shared mutex that ensures only one PrefixWriter
// flushes to the underlying writer at a time, preventing interleaved output
// from concurrent goroutines (e.g., multi-module execution or watch mode).
var globalOutputMutex sync.Mutex

// PrefixWriter wraps an io.Writer and prefixes each complete line with a
// given prefix string. It buffers partial lines internally and only flushes
// complete lines (ending in '\n') atomically under the shared global mutex,
// ensuring that concurrent goroutines never produce garbled/interleaved output.
//
// When a writer is done producing output (e.g., after a subprocess exits),
// Flush() must be called to write any remaining partial line content.
type PrefixWriter struct {
	w      io.Writer
	prefix []byte
	buf    bytes.Buffer
}

// NewPrefixWriter creates a new PrefixWriter. All PrefixWriter instances share
// a global mutex (globalOutputMutex), so even when multiple goroutines each
// have their own PrefixWriter writing to the same underlying writer (e.g.,
// os.Stdout), only one flushes at a time.
func NewPrefixWriter(w io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{
		w:      w,
		prefix: []byte(prefix),
	}
}

// Write buffers data and flushes complete lines atomically. Lines are
// identified by '\n' delimiters. Partial lines (not ending in '\n') are
// buffered internally and will be flushed on the next Write call that
// completes the line, or by calling Flush().
func (pw *PrefixWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Append all incoming data to our buffer first.
	pw.buf.Write(p)

	// Extract and flush all complete lines.
	for {
		// Find the next newline in the buffer.
		nlIdx := bytes.IndexByte(pw.buf.Bytes(), '\n')
		if nlIdx < 0 {
			// No complete line left in the buffer.
			break
		}

		// Extract the line including the newline character.
		line := make([]byte, nlIdx+1)
		copy(line, pw.buf.Bytes())
		// Remove the consumed portion from the buffer.
		pw.buf.Next(nlIdx + 1)

		// Build the prefixed line and write it atomically.
		var out bytes.Buffer
		out.Write(pw.prefix)
		out.Write(line)

		globalOutputMutex.Lock()
		_, err = pw.w.Write(out.Bytes())
		globalOutputMutex.Unlock()

		if err != nil {
			return len(p) - pw.buf.Len(), err
		}
	}

	return len(p), nil
}

// Flush writes any remaining buffered partial line content followed by a
// newline. This must be called when the writer is done producing output
// (e.g., after a subprocess exits) to ensure no data is lost.
// Flush is safe to call multiple times; subsequent calls after the buffer
// is empty are no-ops.
func (pw *PrefixWriter) Flush() error {
	if pw.buf.Len() == 0 {
		return nil
	}

	// Build the prefixed partial line (add a newline for cleanliness).
	var out bytes.Buffer
	out.Write(pw.prefix)
	out.Write(pw.buf.Bytes())
	out.WriteByte('\n')

	pw.buf.Reset()

	globalOutputMutex.Lock()
	_, err := pw.w.Write(out.Bytes())
	globalOutputMutex.Unlock()

	return err
}
