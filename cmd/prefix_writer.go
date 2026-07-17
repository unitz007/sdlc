package cmd

import (
	"bytes"
	"io"
	"sync"
)

// globalOutputMutex serializes writes across ALL PrefixWriter instances so that
// concurrent goroutines (multi-module execution) never interleave bytes on the
// underlying writer (e.g. os.Stdout).
var globalOutputMutex sync.Mutex

// PrefixWriter is a thread-safe, line-buffered io.Writer that prepends a fixed
// prefix to every complete line.  It buffers partial writes internally and only
// flushes a complete line (one ending in '\n') atomically under a global mutex.
//
// Usage:
//
//	pw := NewPrefixWriter(os.Stdout, "[backend] ")
//	// ... use pw as an io.Writer for a subprocess ...
//	pw.Flush() // must be called after the subprocess exits
type PrefixWriter struct {
	w      io.Writer // underlying destination (e.g. os.Stdout)
	prefix []byte    // per-instance prefix bytes
	buf    bytes.Buffer
}

// NewPrefixWriter creates a PrefixWriter that writes to w with the given prefix.
func NewPrefixWriter(w io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{
		w:      w,
		prefix: []byte(prefix),
	}
}

// Write appends p to the internal buffer, then flushes all complete lines
// (those ending in '\n') to the underlying writer under the global mutex.
// Partial data is retained in the buffer until a subsequent Write or Flush.
func (pw *PrefixWriter) Write(p []byte) (int, error) {
	pw.buf.Write(p) // always succeeds per bytes.Buffer contract
	pw.flushLines()
	return len(p), nil
}

// flushLines extracts and writes all complete lines from the buffer.
// The remaining (partial) bytes stay in pw.buf for the next call.
func (pw *PrefixWriter) flushLines() {
	for {
		// Find the next newline.
		line, err := pw.buf.ReadBytes('\n')
		if err != nil {
			// No newline found — put the data back and stop.
			// bytes.Buffer.ReadBytes returns the data AND io.EOF when
			// the delimiter is not found; the data has already been
			// consumed from the buffer, so we must prepend it.
			pw.buf.Reset()
			pw.buf.Write(line)
			return
		}
		// We have a complete line (includes the trailing '\n').
		// Write it atomically under the global mutex.
		globalOutputMutex.Lock()
		pw.w.Write(pw.prefix)
		pw.w.Write(line)
		globalOutputMutex.Unlock()
	}
}

// Flush writes any remaining partial line content from the buffer to the
// underlying writer, appending a trailing newline if the content does not
// already end with one.  Flush is safe to call multiple times; subsequent
// calls after the first are no-ops.
//
// Flush MUST be called after the subprocess that writes to this PrefixWriter
// has exited, otherwise the last partial line of output will be lost.
func (pw *PrefixWriter) Flush() {
	globalOutputMutex.Lock()
	defer globalOutputMutex.Unlock()

	if pw.buf.Len() == 0 {
		return
	}

	remaining := pw.buf.Bytes()
	pw.buf.Reset()

	pw.w.Write(pw.prefix)
	pw.w.Write(remaining)
	if len(remaining) == 0 || remaining[len(remaining)-1] != '\n' {
		pw.w.Write([]byte{'\n'})
	}
}
