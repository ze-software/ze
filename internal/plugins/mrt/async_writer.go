// Design: docs/architecture/mrt.md -- async MRT write to decouple reactor from disk I/O

package mrt

import (
	"log/slog"

	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

const asyncWriterChanSize = 4096

// asyncWriter wraps an mrt.Writer with a channel-based async layer.
// The reactor goroutine copies records into the channel without blocking on
// disk I/O. A dedicated goroutine drains the channel and performs actual writes
// (including buffered flush and file rotation).
type asyncWriter struct {
	ch     chan []byte
	writer *mrtfmt.Writer
	logger *slog.Logger
	done   chan struct{}
}

func newAsyncWriter(w *mrtfmt.Writer, logger *slog.Logger) *asyncWriter {
	aw := &asyncWriter{
		ch:     make(chan []byte, asyncWriterChanSize),
		writer: w,
		logger: logger,
		done:   make(chan struct{}),
	}
	go aw.drain()
	return aw
}

// Write copies the record and sends it to the write goroutine.
// Non-blocking unless the channel is full (back-pressure).
func (aw *asyncWriter) Write(record []byte) {
	cp := make([]byte, len(record))
	copy(cp, record)
	select {
	case aw.ch <- cp:
	default:
		aw.logger.Warn("mrt: async write channel full, dropping record")
	}
}

// Close signals the drain goroutine to stop and waits for it to finish.
func (aw *asyncWriter) Close() error {
	close(aw.ch)
	<-aw.done
	return aw.writer.Close()
}

// Rotate forces a file rotation on the underlying writer.
func (aw *asyncWriter) Rotate() error {
	return aw.writer.Rotate()
}

func (aw *asyncWriter) drain() {
	defer close(aw.done)
	for record := range aw.ch {
		if err := aw.writer.Write(record); err != nil {
			aw.logger.Warn("mrt: async write", "error", err)
		}
	}
}
