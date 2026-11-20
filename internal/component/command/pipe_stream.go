// Design: docs/architecture/api/commands.md -- shared remote stream event pipes.
// Related: pipe.go -- stream validation and event formatting.
// Related: pipe_partition.go -- overrideEventJudge, the judge of an override's events.

package command

import (
	"bytes"
	"io"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
)

// streamPipeEventBytesMax bounds one event across producer writes. Total stream
// output remains the transport's responsibility, because SSH streams stay open.
const streamPipeEventBytesMax = 4 * 1024 * 1024

// RunStreamPipes formats each newline-delimited producer event once. A final
// unterminated event is emitted only after producer success. The producer MUST
// stop writing before it returns and MUST propagate writer failures. The caller
// MUST provide the formatter returned by successful stream pipe validation.
// This call owns no workers and is not safe for concurrent writes.
func RunStreamPipes(output io.Writer, format func(string) string, produce func(io.Writer) error) error {
	return runStreamEvents(output, acceptStreamEvent, format, produce)
}

// runStreamEvents is RunStreamPipes with a judge of each event as the producer
// wrote it, run before format. An event judge refuses is not written, and its
// error stops the stream and is the error answered. StreamLine.Run hands it
// the judge of an override's events (overrideEventJudge).
func runStreamEvents(output io.Writer, judge func(string) error, format func(string) string, produce func(io.Writer) error) error {
	writer := &streamPipeWriter{output: output, judge: judge, format: format}
	if err := produce(writer); err != nil {
		return err
	}
	if writer.failure != nil {
		return writer.failure
	}
	if writer.pending.Len() == 0 {
		return nil
	}
	return writer.writeEvent()
}

// streamPipeWriter retains only one bounded event. It is constructed and owned
// by runStreamEvents; its zero value is not initialized for writes.
type streamPipeWriter struct {
	output  io.Writer
	judge   func(string) error
	format  func(string) string
	pending bytes.Buffer
	failure error
}

func (w *streamPipeWriter) Write(p []byte) (int, error) {
	if w.failure != nil {
		return 0, w.failure
	}
	originalLen := len(p)
	consumed := 0
	for len(p) > 0 {
		newline := bytes.IndexByte(p, '\n')
		eventBytes := len(p)
		if newline >= 0 {
			eventBytes = newline
		}
		if eventBytes > streamPipeEventBytesMax-w.pending.Len() {
			w.failure = opererr.New("a live pipe event exceeds the 4 MiB limit",
				"the stream stopped before formatting or truncating this event",
				"remove the pipes and repeat the monitor command")
			return consumed, w.failure
		}
		w.pending.Write(p[:eventBytes]) //nolint:errcheck // bytes.Buffer.Write cannot fail
		if newline < 0 {
			return originalLen, nil
		}
		consumed += newline + 1
		if err := w.writeEvent(); err != nil {
			w.failure = err
			return consumed, err
		}
		p = p[newline+1:]
	}
	return originalLen, nil
}

func (w *streamPipeWriter) writeEvent() error {
	raw := strings.TrimSuffix(w.pending.String(), "\r")
	w.pending.Reset()
	if err := w.judge(raw); err != nil {
		return err
	}
	formatted := strings.TrimRight(w.format(raw), "\r\n")
	if IsPipeError(formatted) {
		return opererr.New("a live pipe could not format this event", formatted,
			"remove the refused pipe and repeat the monitor command")
	}
	if formatted == "" {
		return nil
	}
	if err := writeStreamText(w.output, formatted); err != nil {
		return err
	}
	return writeStreamText(w.output, "\n")
}

func writeStreamText(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}
