// Design: docs/architecture/api/ipc_protocol.md — text handshake framing
// Related: conn.go — JSON-mode Conn (NUL-framed)
// Related: text.go — text format/parse for handshake stages
// Related: text_mux.go — TextMuxConn concurrent text RPCs

package rpc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// ConnMode represents the detected protocol mode on a connection.
type ConnMode string

const (
	// ModeJSON indicates NUL-framed JSON-RPC protocol.
	ModeJSON ConnMode = "json"
	// ModeText indicates line-based text protocol.
	ModeText ConnMode = "text"
)

// PeekMode reads one byte from conn to detect the protocol mode.
// Returns the mode and a wrapped net.Conn that replays the peeked byte
// on the first Read call, preserving all other net.Conn methods.
func PeekMode(conn net.Conn) (ConnMode, net.Conn, error) {
	var buf [1]byte
	if _, err := io.ReadFull(conn, buf[:]); err != nil {
		return "", nil, fmt.Errorf("peek mode: %w", err)
	}
	wrapped := &peekConn{Conn: conn, first: buf[0]}
	if buf[0] == '{' {
		return ModeJSON, wrapped, nil
	}
	return ModeText, wrapped, nil
}

// peekConn wraps a net.Conn and replays one peeked byte on the first Read.
// All other net.Conn methods (deadlines, addresses, close) delegate
// to the embedded connection unchanged.
type peekConn struct {
	net.Conn
	first byte
	used  bool
}

func (p *peekConn) Read(b []byte) (int, error) {
	if !p.used && len(b) > 0 {
		p.used = true
		b[0] = p.first
		if len(b) == 1 {
			return 1, nil
		}
		n, err := p.Conn.Read(b[1:])
		return n + 1, err
	}
	return p.Conn.Read(b)
}

// TextConn provides text-mode framing over network connections.
// Messages are newline-separated lines terminated by a blank line.
// Used during the 5-stage handshake; post-handshake uses TextMuxConn.
type TextConn struct {
	readConn  net.Conn
	writeConn net.Conn
	scanner   *bufio.Scanner
	mu        sync.Mutex
}

// NewTextConn creates a TextConn that reads from readConn and writes to writeConn.
// For single-socket use, pass the same conn for both arguments.
func NewTextConn(readConn, writeConn net.Conn) *TextConn {
	s := bufio.NewScanner(readConn)
	// Set explicit buffer limit consistent with MaxMessageSize (16 MB).
	// Without this, a text-mode plugin could send arbitrarily large lines
	// causing memory exhaustion (default bufio.Scanner limit is 64KB).
	s.Buffer(make([]byte, 4096), MaxMessageSize+1)
	return &TextConn{
		readConn:  readConn,
		writeConn: writeConn,
		scanner:   s,
	}
}

// Close closes the read connection, unblocking any pending reads.
func (tc *TextConn) Close() error {
	return tc.readConn.Close()
}

// ReadMessage reads lines until a blank line (empty line), returning the
// accumulated text. The blank line terminator is consumed but not included
// in the result. Each content line includes its trailing newline.
//
// Context deadlines are enforced via SetReadDeadline on the underlying
// connection. After a deadline error the scanner is no longer usable,
// but during the sequential handshake a timeout is terminal anyway.
func (tc *TextConn) ReadMessage(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if dl, ok := ctx.Deadline(); ok {
		if err := tc.readConn.SetReadDeadline(dl); err != nil {
			return "", fmt.Errorf("text conn: set read deadline: %w", err)
		}
		defer func() { _ = tc.readConn.SetReadDeadline(time.Time{}) }()
	}

	var b strings.Builder
	found := false
	for tc.scanner.Scan() {
		line := tc.scanner.Text()
		if line == "" {
			found = true
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if err := tc.scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("text conn: read message: %w", err)
	}
	if !found {
		return "", fmt.Errorf("text conn: unexpected EOF reading message")
	}
	return b.String(), nil
}

// ReadLine reads a single line from the connection. Used for reading
// responses like "ok" or "error <message>".
func (tc *TextConn) ReadLine(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if dl, ok := ctx.Deadline(); ok {
		if err := tc.readConn.SetReadDeadline(dl); err != nil {
			return "", fmt.Errorf("text conn: set read deadline: %w", err)
		}
		defer func() { _ = tc.readConn.SetReadDeadline(time.Time{}) }()
	}
	if !tc.scanner.Scan() {
		if err := tc.scanner.Err(); err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", fmt.Errorf("text conn: read line: %w", err)
		}
		return "", fmt.Errorf("text conn: unexpected EOF reading line")
	}
	return tc.scanner.Text(), nil
}

// WriteMessage writes a text message to the connection. The text should
// already end with \n\n (blank line terminator) from format functions.
// Uses a write deadline derived from the context.
func (tc *TextConn) WriteMessage(ctx context.Context, text string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	deadline := writeDeadline(ctx)
	if err := tc.writeConn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("text conn: set write deadline: %w", err)
	}
	_, writeErr := io.WriteString(tc.writeConn, text)
	if clearErr := tc.writeConn.SetWriteDeadline(time.Time{}); clearErr != nil {
		return fmt.Errorf("text conn: clear write deadline: %w", clearErr)
	}
	if writeErr != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return writeErr
}

// WriteLine writes a single line followed by \n. Used for sending
// responses like "ok" or "error <message>".
func (tc *TextConn) WriteLine(ctx context.Context, line string) error {
	return tc.WriteMessage(ctx, line+"\n")
}
