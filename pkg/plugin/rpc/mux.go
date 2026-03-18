// Design: docs/architecture/api/ipc_protocol.md — multiplexed plugin RPC
// Related: conn.go — Conn type and persistent reader (readFrame)

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// ErrMuxConnClosed is returned when CallRPC is called on a closed MuxConn.
var ErrMuxConnClosed = fmt.Errorf("mux conn closed")

// maxConsecutiveBadLines is the threshold of consecutive malformed or orphaned
// lines before readLoop closes the connection. Protects against a malicious
// plugin flooding the engine with junk.
const maxConsecutiveBadLines = 100

// MuxConn wraps a *Conn to support concurrent CallRPC calls.
// A background reader goroutine routes responses to callers by #<id> prefix,
// eliminating the callMu serialization bottleneck in Conn.CallRPC.
//
// MuxConn owns the Conn's reader exclusively -- do not call ReadRequest
// on the underlying Conn after creating a MuxConn.
type MuxConn struct {
	conn *Conn

	// pending maps request ID (string) to a buffered channel for the response.
	// Written by CallRPC, read+deleted by the background reader.
	pending sync.Map

	// done is closed when the background reader exits.
	done chan struct{}

	// readerErr stores the terminal read error for late callers.
	readerErr atomic.Pointer[error]

	// closeOnce ensures Close() only runs once.
	closeOnce sync.Once

	// consecutiveBad counts consecutive malformed or orphaned lines in readLoop.
	// Only accessed by readLoop -- no synchronization needed.
	consecutiveBad uint32
}

// NewMuxConn creates a MuxConn wrapping the given Conn.
// Starts a background reader goroutine that routes responses by #<id> prefix.
func NewMuxConn(conn *Conn) *MuxConn {
	m := &MuxConn{
		conn: conn,
		done: make(chan struct{}),
	}
	go m.readLoop()
	return m
}

// Close stops the background reader and closes the underlying connection.
// All pending CallRPC callers will unblock with an error.
// Safe to call multiple times.
func (m *MuxConn) Close() error {
	var err error
	m.closeOnce.Do(func() {
		err = m.conn.Close()
	})
	return err
}

// CallRPC sends an RPC request and waits for the matching response.
// Returns the result JSON payload on success, or an *RPCCallError on RPC error.
// Safe for concurrent use by multiple goroutines. Each caller gets its
// own response channel keyed by request ID.
func (m *MuxConn) CallRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	// Check if reader is already dead.
	if errPtr := m.readerErr.Load(); errPtr != nil {
		return nil, fmt.Errorf("mux conn read error: %w", *errPtr)
	}

	// Generate request ID.
	id := m.conn.NextID()
	idStr := strconv.FormatUint(id, 10)

	// Create buffered response channel (capacity 1 so reader never blocks).
	respCh := make(chan []byte, 1)
	m.pending.Store(idStr, respCh)

	// Marshal params.
	var paramsRaw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			m.pending.Delete(idStr)
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		paramsRaw = b
	}

	// Format and send request line: #<id> <method> [<json>]\n
	line := FormatRequest(id, method, paramsRaw)
	if err := m.conn.writeLineWithContext(ctx, line); err != nil {
		m.pending.Delete(idStr)
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Wait for response, context cancellation, or reader death.
	select {
	case body := <-respCh:
		return interpretResponse(body)
	case <-ctx.Done():
		m.pending.Delete(idStr)
		return nil, ctx.Err()
	case <-m.done:
		m.pending.Delete(idStr)
		if errPtr := m.readerErr.Load(); errPtr != nil {
			return nil, fmt.Errorf("mux conn read error: %w", *errPtr)
		}
		return nil, ErrMuxConnClosed
	}
}

// readLoop is the background reader goroutine. It reads response lines
// from the connection and routes them to waiting callers by #<id> prefix.
// Runs until the connection is closed or a read error occurs.
//
// Uses conn.readFrame() to consume from the persistent reader's channel,
// ensuring only one goroutine ever reads from the underlying FrameReader.
func (m *MuxConn) readLoop() {
	defer close(m.done)

	for {
		data, err := m.conn.readFrame(context.Background())
		if err != nil {
			m.readerErr.Store(&err)
			return
		}

		line := string(data)

		// Extract #<id> prefix using simple string operations (no JSON parsing).
		if !strings.HasPrefix(line, "#") {
			slog.Warn("mux conn: line missing # prefix", "line", truncate(line, 80))
			m.consecutiveBad++
			if m.consecutiveBad > maxConsecutiveBadLines {
				err := fmt.Errorf("mux conn: %d consecutive malformed lines, closing", m.consecutiveBad)
				m.readerErr.Store(&err)
				return
			}
			continue
		}

		idStr, body, ok := strings.Cut(line[1:], " ")
		if !ok {
			slog.Warn("mux conn: line has no body after ID", "line", truncate(line, 80))
			m.consecutiveBad++
			if m.consecutiveBad > maxConsecutiveBadLines {
				err := fmt.Errorf("mux conn: %d consecutive malformed lines, closing", m.consecutiveBad)
				m.readerErr.Store(&err)
				return
			}
			continue
		}

		// Look up and deliver to the waiting caller.
		val, found := m.pending.LoadAndDelete(idStr)
		if !found {
			slog.Warn("mux conn: orphaned response", "id", idStr)
			m.consecutiveBad++
			if m.consecutiveBad > maxConsecutiveBadLines {
				err := fmt.Errorf("mux conn: %d consecutive malformed lines, closing", m.consecutiveBad)
				m.readerErr.Store(&err)
				return
			}
			continue
		}

		ch, ok := val.(chan []byte)
		if !ok {
			continue
		}

		// Valid response routed successfully -- reset bad counter.
		m.consecutiveBad = 0

		// Send the body (verb + payload) to the caller.
		ch <- []byte(body)
	}
}

// interpretResponse parses the body after #<id> (e.g., "ok {...}" or "error {...}")
// and returns the result payload on success or an RPCCallError on error.
func interpretResponse(body []byte) (json.RawMessage, error) {
	s := string(body)
	verb, payload, _ := strings.Cut(s, " ")

	if verb == "ok" {
		if payload == "" {
			return nil, nil
		}
		return json.RawMessage(payload), nil
	}
	if verb == "error" {
		return nil, parseRPCError([]byte(payload))
	}
	return nil, fmt.Errorf("unexpected response verb %q", verb)
}
