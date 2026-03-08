// Design: docs/architecture/api/ipc_protocol.md — multiplexed text plugin RPC
// Related: text.go — text handshake serialization
// Related: text_conn.go — TextConn text-mode framing
// Related: mux.go — JSON-mode MuxConn (NUL-framed, JSON ID routing)

package rpc

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// TextMuxConn wraps a *TextConn to support concurrent text-mode RPCs.
// A background reader goroutine reads response lines and routes them
// to waiting callers by the #N serial prefix.
//
// Request format:  #<id> <method> [<args>]
// Response format: #<id> ok [<result>]  or  #<id> error <message>
//
// TextMuxConn owns the TextConn's reader exclusively — do not call
// ReadMessage or ReadLine on the underlying TextConn after creating
// a TextMuxConn.
type TextMuxConn struct {
	tc *TextConn

	idSeq     atomic.Uint64
	pending   sync.Map
	done      chan struct{}
	readerErr atomic.Pointer[error]
	closeOnce sync.Once
}

// NewTextMuxConn creates a TextMuxConn wrapping the given TextConn.
// Starts a background reader goroutine that routes responses by #N prefix.
func NewTextMuxConn(tc *TextConn) *TextMuxConn {
	m := &TextMuxConn{
		tc:   tc,
		done: make(chan struct{}),
	}
	go m.readLoop()
	return m
}

// Close stops the background reader and closes the underlying connection.
// All pending CallRPC callers will unblock with an error.
// Safe to call multiple times.
func (m *TextMuxConn) Close() error {
	var err error
	m.closeOnce.Do(func() {
		err = m.tc.Close()
	})
	return err
}

// CallRPC sends a text RPC request "#<id> <method> [<args>]" and waits
// for the matching response "#<id> ok [<result>]".
// Returns the result string on success, or an error for failures.
// Safe for concurrent use by multiple goroutines.
func (m *TextMuxConn) CallRPC(ctx context.Context, method, args string) (string, error) {
	if errPtr := m.readerErr.Load(); errPtr != nil {
		return "", fmt.Errorf("text mux conn read error: %w", *errPtr)
	}

	id := m.idSeq.Add(1)
	idStr := strconv.FormatUint(id, 10)

	respCh := make(chan string, 1)
	m.pending.Store(idStr, respCh)

	var line string
	if args != "" {
		line = fmt.Sprintf("#%s %s %s", idStr, method, args)
	} else {
		line = fmt.Sprintf("#%s %s", idStr, method)
	}
	if err := m.tc.WriteLine(ctx, line); err != nil {
		m.pending.Delete(idStr)
		return "", fmt.Errorf("send request: %w", err)
	}

	select {
	case body := <-respCh:
		return parseTextResponse(body)
	case <-ctx.Done():
		m.pending.Delete(idStr)
		return "", ctx.Err()
	case <-m.done:
		m.pending.Delete(idStr)
		if errPtr := m.readerErr.Load(); errPtr != nil {
			return "", fmt.Errorf("text mux conn read error: %w", *errPtr)
		}
		return "", ErrMuxConnClosed
	}
}

// readLoop is the background reader goroutine. It reads lines from the
// connection, extracts the #N prefix, and routes to waiting callers.
func (m *TextMuxConn) readLoop() {
	defer close(m.done)

	for {
		line, err := m.tc.ReadLine(context.Background())
		if err != nil {
			m.readerErr.Store(&err)
			return
		}

		if !strings.HasPrefix(line, "#") {
			slog.Warn("text mux conn: line missing # prefix", "line", line)
			continue
		}

		rest := line[1:]
		idStr, body, ok := strings.Cut(rest, " ")
		if !ok {
			slog.Warn("text mux conn: line has no body after ID", "line", line)
			continue
		}

		val, ok := m.pending.LoadAndDelete(idStr)
		if !ok {
			slog.Warn("text mux conn: orphaned response", "id", idStr)
			continue
		}

		ch, ok := val.(chan string)
		if !ok {
			continue
		}

		ch <- body
	}
}

// parseTextResponse interprets a response body after the #N prefix.
// "ok" or "ok <result>" → success. "error <message>" → error.
func parseTextResponse(body string) (string, error) {
	if body == "ok" {
		return "", nil
	}
	if strings.HasPrefix(body, "ok ") {
		return body[3:], nil
	}
	if body == "error" {
		return "", fmt.Errorf("rpc error: (no message)")
	}
	if strings.HasPrefix(body, "error ") {
		return "", fmt.Errorf("rpc error: %s", body[6:])
	}
	return "", fmt.Errorf("text mux conn: unexpected response: %q", body)
}
