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
	"time"
)

// ErrMuxConnClosed is returned when CallRPC is called on a closed MuxConn.
var ErrMuxConnClosed = fmt.Errorf("mux conn closed")

// maxConsecutiveBadLines is the threshold of consecutive malformed or orphaned
// lines before readLoop closes the connection. Protects against a malicious
// plugin flooding the engine with junk.
const maxConsecutiveBadLines = 100

// MuxConn wraps a *Conn to support concurrent CallRPC calls and inbound
// request dispatching on a single bidirectional connection.
//
// A background reader goroutine reads all incoming lines and routes them:
//   - Responses (verb is "ok" or "error") are routed to waiting CallRPC callers by #<id>.
//   - Requests (verb is a method name) are pushed to the Requests() channel.
//
// MuxConn owns the Conn's reader exclusively -- do not call ReadRequest
// on the underlying Conn after creating a MuxConn.
type MuxConn struct {
	conn *Conn

	// pending maps request ID (string) to a buffered channel for the response.
	// Written by CallRPC, read+deleted by the background reader.
	pending sync.Map

	// requestCh receives inbound requests from the remote side.
	// The readLoop pushes requests here when the verb is not "ok" or "error".
	requestCh chan *Request

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
// Starts a background reader goroutine that routes responses by #<id> prefix
// and inbound requests to the Requests() channel.
func NewMuxConn(conn *Conn) *MuxConn {
	m := &MuxConn{
		conn:      conn,
		requestCh: make(chan *Request, 16),
		done:      make(chan struct{}),
	}
	go m.readLoop()
	return m
}

// Requests returns a channel of inbound requests from the remote side.
// Requests are lines where the verb is a method name (not "ok" or "error").
// The caller should read from this channel in a dispatch loop.
func (m *MuxConn) Requests() <-chan *Request {
	return m.requestCh
}

// SendResult sends a successful RPC response for an inbound request.
func (m *MuxConn) SendResult(ctx context.Context, id uint64, data any) error {
	return m.conn.SendResult(ctx, id, data)
}

// SendOK sends an empty successful RPC response for an inbound request.
func (m *MuxConn) SendOK(ctx context.Context, id uint64) error {
	return m.conn.SendOK(ctx, id)
}

// SendError sends an error RPC response for an inbound request.
func (m *MuxConn) SendError(ctx context.Context, id uint64, message string) error {
	return m.conn.SendError(ctx, id, message)
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

	// Send request line: #<id> <method> [<json>]\n (appended into pool buffer).
	writeErr := m.conn.writeAppended(ctx, func(buf []byte) []byte {
		return AppendRequest(buf, id, method, paramsRaw)
	})
	if writeErr != nil {
		m.pending.Delete(idStr)
		return nil, fmt.Errorf("send request: %w", writeErr)
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
// Done returns a channel that is closed when the background reader exits.
func (m *MuxConn) Done() <-chan struct{} {
	return m.done
}

func (m *MuxConn) readLoop() {
	defer close(m.requestCh) // Unblock ReadRequest callers.
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

		// Determine if this is a response or an inbound request.
		// Responses have verb "ok" or "error"; requests have a method name.
		verb, _, _ := strings.Cut(body, " ")
		isResponse := verb == StatusOK || verb == StatusError

		if isResponse {
			// Route response to the waiting CallRPC caller.
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

			ch, chOK := val.(chan []byte)
			if !chOK {
				continue
			}

			m.consecutiveBad = 0
			ch <- []byte(body)
		} else {
			// Inbound request from the remote side -- parse and dispatch.
			id, parseErr := strconv.ParseUint(idStr, 10, 64)
			if parseErr != nil {
				slog.Warn("mux conn: bad request ID", "id", idStr)
				m.consecutiveBad++
				if m.consecutiveBad > maxConsecutiveBadLines {
					err := fmt.Errorf("mux conn: %d consecutive malformed lines, closing", m.consecutiveBad)
					m.readerErr.Store(&err)
					return
				}
				continue
			}

			_, payload, _ := strings.Cut(body, " ")
			req := &Request{
				ID:     id,
				Method: verb,
			}
			if payload != "" {
				req.Params = json.RawMessage(payload)
			}

			m.consecutiveBad = 0
			if !m.sendRequest(req) {
				slog.Warn("mux conn: request channel full, dropping inbound request",
					"id", id, "method", verb)
			}
		}
	}
}

// sendRequest attempts a non-blocking send to requestCh. Returns false if the
// channel is full (consumer is stalled). This prevents readLoop from blocking
// and starving all pending CallRPC callers.
func (m *MuxConn) sendRequest(req *Request) bool {
	select {
	case m.requestCh <- req:
		return true
	case <-time.After(time.Second):
		return false
	}
}

// interpretResponse parses the body after #<id> (e.g., "ok {...}" or "error {...}")
// and returns the result payload on success or an RPCCallError on error.
func interpretResponse(body []byte) (json.RawMessage, error) {
	s := string(body)
	verb, payload, _ := strings.Cut(s, " ")

	if verb == StatusOK {
		if payload == "" {
			return nil, nil
		}
		return json.RawMessage(payload), nil
	}
	if verb == StatusError {
		return nil, parseRPCError([]byte(payload))
	}
	return nil, fmt.Errorf("unexpected response verb %q", verb)
}
