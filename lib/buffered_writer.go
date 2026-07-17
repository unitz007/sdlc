package lib

import (
	"bytes"
	"io"
	"sync"
)

// OutputLock provides a shared mutex that all concurrent BufferedPrefixWriter
// instances should share to prevent interleaved output when writing to the
// same underlying io.Writer (e.g., os.Stdout).
type OutputLock struct {
	mu sync.Mutex
}

// NewOutputLock creates a new OutputLock.
func NewOutputLock() *OutputLock {
	return &OutputLock{}
}

// Lock acquires the output lock.
func (ol *OutputLock) Lock() {
	ol.mu.Lock()
}

// Unlock releases the output lock.
func (ol *OutputLock) Unlock() {
	ol.mu.Unlock()
}

// BufferedPrefixWriter wraps an io.Writer and prepends a configurable prefix
// to each line of output. It is safe for concurrent use from multiple
// goroutines provided they all share the same OutputLock.
//
// Unlike a simple PrefixWriter, BufferedPrefixWriter uses a line buffer to
// accumulate partial writes until a newline is received, ensuring that the
// prefix and the full line are written atomically under the shared lock.
type BufferedPrefixWriter struct {
	lock   *OutputLock
	w      io.Writer
	prefix []byte

	mu       sync.Mutex     // protects buf and midLine
	buf      bytes.Buffer   // accumulates partial line data
	midLine  bool           // whether there is unbuffered partial-line content
}

// NewBufferedPrefixWriter creates a new BufferedPrefixWriter. All writers
// sharing the same OutputLock will have their output serialized, preventing
// garbled interleaved lines on the terminal.
func NewBufferedPrefixWriter(w io.Writer, prefix string, lock *OutputLock) *BufferedPrefixWriter {
	return &BufferedPrefixWriter{
		lock:   lock,
		w:      w,
		prefix: []byte(prefix),
	}
}

// Write buffers the data and writes complete lines atomically under the
// shared OutputLock. Partial lines (data without a trailing newline) are
// buffered internally and written when a subsequent Write delivers the
// newline, or when Flush is called.
//
// Each complete line is prefixed with the configured prefix string.
// When a partial line spans multiple Write calls, only the first segment
// receives the prefix.
func (bpw *BufferedPrefixWriter) Write(p []byte) (int, err error) {
	bpw.mu.Lock()
	defer bpw.mu.Unlock()

	bpw.buf.Write(p)

	// Process all complete lines in the buffer
	for {
		// Find the next newline
		data := bpw.buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx == -1 {
			// No complete line; keep the remainder buffered
			break
		}

		// Extract the complete line including the newline
		line := make([]byte, idx+1)
		copy(line, data[:idx+1])

		// Remove the processed data from the buffer
		bpw.buf.Next(idx + 1)

		// Write the complete line atomically under the shared lock
		bpw.lock.Lock()
		if !bpw.midLine {
			bpw.w.Write(bpw.prefix)
		}
		bpw.w.Write(line)
		bpw.midLine = false
		bpw.lock.Unlock()
	}

	// Mark that we have unbuffered content if buffer is non-empty
	bpw.midLine = bpw.buf.Len() > 0

	return len(p), nil
}

// Flush writes any remaining buffered (partial) data to the underlying
// writer. This should be called before the writer is discarded to ensure
// no output is lost. The write is performed atomically under the shared lock.
func (bpw *BufferedPrefixWriter) Flush() error {
	bpw.mu.Lock()
	defer bpw.mu.Unlock()

	if bpw.buf.Len() == 0 {
		return nil
	}

	data := bpw.buf.Bytes()

	bpw.lock.Lock()
	if !bpw.midLine {
		bpw.w.Write(bpw.prefix)
	}
	bpw.w.Write(data)
	bpw.midLine = false
	bpw.lock.Unlock()

	bpw.buf.Reset()
	return nil
}
