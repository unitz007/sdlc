package cmd

import (
	"bytes"
	"sync"
)

// SharedPrefixWriter is a thread-safe line-buffered writer that prefixes each
// line with a per-source tag. It is designed to be shared across goroutines:
// each goroutine creates its own Writer via SourceWriter() and the underlying
// mutex ensures that complete lines are flushed atomically so that output from
// concurrent sources is never interleaved at the byte level.
type SharedPrefixWriter struct {
	mu   sync.Mutex
	w    threadSafeWriter
}

// threadSafeWriter abstracts sync.Locker so tests can inject mock writers.
// The standard io.Writer is not safe for concurrent use on os.Stdout / os.Stderr.
type threadSafeWriter interface {
	write(b []byte) (int, error)
}

// defaultStdWriter wraps an underlying writer but is only safe because all
// callers go through the SharedPrefixWriter mutex.
type defaultStdWriter struct {
	w innerWriter
}

type innerWriter interface {
	Write(b []byte) (int, error)
}

func (d *defaultStdWriter) write(b []byte) (int, error) {
	return d.w.Write(b)
}

// NewSharedPrefixWriter creates a SharedPrefixWriter that outputs to w.
func NewSharedPrefixWriter(w innerWriter) *SharedPrefixWriter {
	return &SharedPrefixWriter{
		w: &defaultStdWriter{w: w},
	}
}

// SourceWriter returns a new per-source writer that prefixes every line with
// the given prefix. Multiple SourceWriters backed by the same
// SharedPrefixWriter are safe to use concurrently from different goroutines.
func (spw *SharedPrefixWriter) SourceWriter(prefix string) *SourceWriter {
	return &SourceWriter{
		spw:    spw,
		prefix: []byte(prefix),
		buf:    &bytes.Buffer{},
	}
}

// SourceWriter is a per-goroutine writer that collects partial lines and
// flushes complete lines atomically via the parent SharedPrefixWriter.
type SourceWriter struct {
	spw    *SharedPrefixWriter
	prefix []byte
	buf    *bytes.Buffer
}

// Write appends data to the internal buffer. Complete lines (terminated by
// '\n') are flushed immediately under the shared mutex so they are never
// interleaved with lines from other SourceWriters.
func (sw *SourceWriter) Write(p []byte) (n int, err error) {
	sw.spw.mu.Lock()
	defer sw.spw.mu.Unlock()

	sw.buf.Reset()
	sw.buf.Write(p)

	// Process complete lines.
	for {
		line, err := sw.buf.ReadBytes('\n')
		if err != nil {
			// EOF means there's no newline at the end — put the remainder
			// back into the buffer for the next Write call.
			if len(line) > 0 {
				// Store partial line back into buf for next call.
				sw.buf.Reset()
				sw.buf.Write(line)
			}
			break
		}

		// Write the full prefixed line atomically.
		if _, werr := sw.spw.w.write(append(sw.prefix, line...)); werr != nil {
			return n, werr
		}
	}

	return len(p), nil
}

// Flush writes any remaining partial line (without a trailing newline) to
// the underlying writer. This should be called when a SourceWriter will no
// longer receive more data (e.g. after a subprocess finishes).
func (sw *SourceWriter) Flush() error {
	sw.spw.mu.Lock()
	defer sw.spw.mu.Unlock()

	if sw.buf.Len() == 0 {
		return nil
	}

	// Write remaining data with prefix and a trailing newline.
	remaining := sw.buf.Bytes()
	if _, err := sw.spw.w.write(append(sw.prefix, remaining...)); err != nil {
		return err
	}

	sw.buf.Reset()
	return nil
}
