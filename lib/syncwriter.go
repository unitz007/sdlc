package lib

import (
	"bytes"
	"io"
	"sync"
)

// SyncWriter wraps an io.Writer with a mutex so that concurrent goroutines
// cannot interleave partial writes. Each line is prefixed with the given label
// and written atomically.
//
// It is safe for concurrent use by multiple goroutines.
type SyncWriter struct {
	mu     sync.Mutex
	w      io.Writer
	prefix []byte
	buf    bytes.Buffer
}

// NewSyncWriter returns a SyncWriter that prefixes every line with label
// before writing to w.
func NewSyncWriter(w io.Writer, label string) *SyncWriter {
	return &SyncWriter{
		w:      w,
		prefix: []byte(label),
	}
}

// Write buffers data and flushes complete lines atomically. Partial lines
// (data that does not end with '\n') are held until the next Write that
// completes them, or until Flush is called.
func (sw *SyncWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	return sw.writeLocked(p)
}

// writeLocked performs the actual buffering and flushing. Caller must hold sw.mu.
func (sw *SyncWriter) writeLocked(p []byte) (int, error) {
	if sw.buf.Len() == 0 && bytes.IndexByte(p, '\n') == -1 {
		// Fast path: no buffered data and no newline in this write.
		// Buffer the partial line for the next call.
		return sw.buf.Write(p)
	}

	// Write incoming data to the buffer.
	if _, err := sw.buf.Write(p); err != nil {
		return len(p), err
	}

	total := len(p)

	for {
		// Find the next newline in the buffer.
		idx := bytes.IndexByte(sw.buf.Bytes(), '\n')
		if idx == -1 {
			break
		}

		// Extract the complete line including the newline.
		line := make([]byte, sw.buf.Len())
		n := copy(line, sw.buf.Bytes())
		sw.buf.Reset()
		// Put the remaining data back in the buffer.
		if n > idx+1 {
			sw.buf.Write(line[idx+1:])
		}

		// Build the full output: prefix + line content.
		var out []byte
		out = append(out, sw.prefix...)
		out = append(out, line[:idx+1]...)

		if _, err := sw.w.Write(out); err != nil {
			return total, err
		}
	}

	return total, nil
}

// Flush writes any remaining buffered (partial) data as a final line, with the
// prefix. Call this when the source of data has finished (e.g. after a command
// completes).
func (sw *SyncWriter) Flush() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.buf.Len() == 0 {
		return nil
	}

	remaining := sw.buf.Bytes()
	sw.buf.Reset()

	var out []byte
	out = append(out, sw.prefix...)
	out = append(out, remaining...)
	if !bytes.HasSuffix(remaining, []byte{'\n'}) {
		out = append(out, '\n')
	}

	_, err := sw.w.Write(out)
	return err
}

// Ensure SyncWriter implements io.Writer.
var _ io.Writer = (*SyncWriter)(nil)
