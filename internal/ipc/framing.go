// Package ipc provides NUL-byte terminated JSON framing for Ze IPC protocol.
//
// Messages are UTF-8 JSON objects terminated by a NUL byte (0x00).
// NUL cannot appear in valid JSON, making it an unambiguous delimiter.
package ipc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// MaxMessageSize is the maximum allowed message size (16 MB).
const MaxMessageSize = 16 * 1024 * 1024

// initialBufSize is the initial read buffer size (64 KB).
const initialBufSize = 64 * 1024

// FrameReader reads NUL-terminated messages from an io.Reader.
type FrameReader struct {
	scanner *bufio.Scanner
}

// NewFrameReader creates a FrameReader that reads NUL-terminated messages.
func NewFrameReader(r io.Reader) *FrameReader {
	scanner := bufio.NewScanner(r)
	// MaxMessageSize+1 because bufio.Scanner's max is exclusive (token must be < max)
	scanner.Buffer(make([]byte, initialBufSize), MaxMessageSize+1)
	scanner.Split(splitNUL)
	return &FrameReader{scanner: scanner}
}

// Read returns the next NUL-terminated message.
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

// splitNUL is a bufio.SplitFunc that splits on NUL bytes.
func splitNUL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		// No trailing NUL — return remaining data as partial message
		return len(data), data, nil
	}
	// Request more data
	return 0, nil, nil
}

// FrameWriter writes NUL-terminated messages to an io.Writer.
type FrameWriter struct {
	w io.Writer
}

// NewFrameWriter creates a FrameWriter that writes NUL-terminated messages.
func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{w: w}
}

// Write sends a message followed by a NUL terminator.
func (fw *FrameWriter) Write(msg []byte) error {
	buf := make([]byte, len(msg)+1)
	copy(buf, msg)
	buf[len(msg)] = 0
	_, err := fw.w.Write(buf)
	if err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}
