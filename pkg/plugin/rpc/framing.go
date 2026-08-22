// Design: docs/architecture/api/ipc_protocol.md — newline-delimited frame I/O
// Detail: batch.go — batched event delivery frame construction
// Related: conn.go — Conn uses FrameReader/FrameWriter for RPC framing
// Related: message.go — RPC wire message types and line parsing

package rpc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
)

// frameBufInitial is the initial capacity of a pooled frame buffer. Most
// RPC lines (<1KB: #id method json-payload) fit without growth. Larger
// events (e.g. UPDATE structured events) grow the underlying slice via
// append; buffers are returned to the pool only if their capacity stays
// below frameBufMax to avoid a single large batch permanently inflating
// the pool.
const (
	frameBufInitial = 4 * 1024
	frameBufMax     = 64 * 1024
)

// framePool provides reusable buffers for every per-request, per-
// response, and per-event RPC line. One buffer is acquired, the
// caller's Append* helper writes into it, the newline is appended, the
// single buf is written to the wire, then the buffer is returned.
//
// sync.Pool is the right shape: plugin-rpc Conns are shared across
// goroutines (SendResult/SendOK/SendError from dispatch goroutines,
// CallRPC from the caller goroutine, event emitters) and sync.Pool's
// per-P local cache removes Get/Put contention.
var framePool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, frameBufInitial)
		return &buf
	},
}

// getFrameBuf returns a pooled frame buffer re-sliced to zero length.
func getFrameBuf() *[]byte {
	bp, ok := framePool.Get().(*[]byte)
	if !ok {
		b := make([]byte, 0, frameBufInitial)
		bp = &b
	}
	*bp = (*bp)[:0]
	return bp
}

// putFrameBuf returns a frame buffer to the pool. Buffers that grew
// beyond frameBufMax are dropped so a single large frame cannot
// permanently inflate pooled memory.
func putFrameBuf(bp *[]byte) {
	if cap(*bp) > frameBufMax {
		return
	}
	*bp = (*bp)[:0]
	framePool.Put(bp)
}

// MaxMessageSize is the maximum allowed message size (16 MB).
const MaxMessageSize = 16 * 1024 * 1024

// initialBufSize is the initial read buffer size (64 KB).
const initialBufSize = 64 * 1024

// FrameReader reads one wire line at a time from an io.Reader.
//
// A line ends with exactly one `\n` and never with `\r\n`. An answer line
// states its own width, so it is taken by that count and a value inside it may
// hold a raw `\n` or `\r`; every other line ends at its newline
// (ScanAnswerLines).
type FrameReader struct {
	scanner *bufio.Scanner
}

// NewFrameReader creates a FrameReader that reads one wire line at a time.
func NewFrameReader(r io.Reader) *FrameReader {
	scanner := bufio.NewScanner(r)
	// MaxMessageSize+1 because bufio.Scanner's max is exclusive (token must be
	// < max), and a line of exactly MaxMessageSize needs its newline held too.
	scanner.Buffer(make([]byte, initialBufSize), MaxMessageSize+1)
	scanner.Split(ScanAnswerLines)
	return &FrameReader{scanner: scanner}
}

// ScanAnswerLines is the bufio.SplitFunc every reader of this protocol frames
// with. It replaces bufio.ScanLines, which is wrong here twice over.
//
// An answer line states its own width: every variable-width field carries its
// byte count, so this takes exactly that many bytes and REQUIRES the byte after
// them to be the newline that ends the line. No payload is searched for one, so
// a raw `\n` inside a value is DATA rather than a frame boundary and no writer
// has to rewrite it before it reaches the wire.
//
// And a line ends with exactly one `\n`, never with `\r\n`. bufio.ScanLines
// strips a trailing `\r`, which is right for a text file and wrong here: it
// would leave a stated count disagreeing with the bytes that arrived, and it
// would take a `\r` a producer put INSIDE a counted value. Nothing is stripped
// here, and a `\r` where the newline belongs is refused by name.
//
// A line that states no width -- a request, a response, an operator's plain
// text -- is framed by its newline, which is the one place this genuinely scans
// and it uses bytes.IndexByte to do it.
func ScanAnswerLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	switch width, state := answerLineWidth(data); state {
	case answerLineStated:
		return scanStatedLine(data, atEOF, width)
	case answerLinePartial:
		if atEOF {
			return 0, nil, fmt.Errorf("answer line states a field wider than the %d bytes that arrived before the stream ended", len(data))
		}
		return 0, nil, nil
	case answerLineOther:
		// Framed by its newline, below.
	}

	if end := bytes.IndexByte(data, '\n'); end >= 0 {
		return end + 1, data[:end], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// scanStatedLine frames one answer line by the width it states. The count is
// bounded BEFORE anything is sliced, so a count an attacker chose cannot become
// a buffer this reader grows to hold.
func scanStatedLine(data []byte, atEOF bool, width uint64) (int, []byte, error) {
	if width > MaxMessageSize {
		return 0, nil, fmt.Errorf("answer line states %d bytes, past the %d-byte maximum", width, MaxMessageSize)
	}
	end := int(width)
	if len(data) <= end {
		if atEOF {
			return 0, nil, fmt.Errorf("answer line states %d bytes and %d arrived before the stream ended", end, len(data))
		}
		return 0, nil, nil
	}
	if data[end] != '\n' {
		return 0, nil, fmt.Errorf("answer line of %d bytes is terminated by %q, want exactly one newline", end, string(rune(data[end])))
	}
	return end + 1, data[:end], nil
}

// Read returns the next newline-delimited message.
// Returns io.EOF when no more messages are available.
func (fr *FrameReader) Read() ([]byte, error) {
	if fr.scanner.Scan() {
		msg := fr.scanner.Bytes()
		// Return a copy to avoid scanner buffer reuse issues
		result := make([]byte, len(msg))
		copy(result, msg)
		return result, nil
	}
	if err := fr.scanner.Err(); err != nil {
		// Check if this is a token-too-long error (oversized message)
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, fmt.Errorf("message exceeds maximum size %d", MaxMessageSize)
		}
		return nil, err
	}
	return nil, io.EOF
}

// FrameWriter writes newline-terminated messages to an io.Writer.
type FrameWriter struct {
	w io.Writer
}

// NewFrameWriter creates a FrameWriter that writes newline-terminated messages.
func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{w: w}
}

// RawWriter returns the underlying io.Writer for direct access.
// Used by batch delivery to write pooled frames without FrameWriter allocation.
func (fw *FrameWriter) RawWriter() io.Writer {
	return fw.w
}

// Write sends a message followed by a newline terminator.
func (fw *FrameWriter) Write(msg []byte) error {
	if len(msg) > MaxMessageSize {
		return fmt.Errorf("message exceeds maximum size %d", MaxMessageSize)
	}
	buf := make([]byte, len(msg)+1)
	copy(buf, msg)
	buf[len(msg)] = '\n'
	_, err := fw.w.Write(buf)
	if err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}

// WriteRaw writes pre-framed data directly. The caller must include the newline
// terminator. Used by batch delivery to bypass the per-frame allocation.
func (fw *FrameWriter) WriteRaw(data []byte) error {
	_, err := fw.w.Write(data)
	if err != nil {
		return fmt.Errorf("write raw frame: %w", err)
	}
	return nil
}
