// Design: docs/architecture/api/ipc_protocol.md — multiplexed plugin RPC

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/ipc"
)

// ErrMuxConnClosed is returned when CallRPC is called on a closed MuxConn.
var ErrMuxConnClosed = fmt.Errorf("mux conn closed")

// MuxConn wraps a *Conn to support concurrent CallRPC calls.
// A background reader goroutine routes responses to callers by request ID,
// eliminating the callMu serialization bottleneck in Conn.CallRPC.
//
// MuxConn owns the Conn's reader exclusively — do not call ReadRequest
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
}

// NewMuxConn creates a MuxConn wrapping the given Conn.
// Starts a background reader goroutine that routes responses by request ID.
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
// Safe for concurrent use by multiple goroutines. Each caller gets its
// own response channel keyed by request ID.
func (m *MuxConn) CallRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	// Check if reader is already dead.
	if errPtr := m.readerErr.Load(); errPtr != nil {
		return nil, fmt.Errorf("mux conn read error: %w", *errPtr)
	}

	// Generate request ID.
	id := m.conn.NextID()
	idStr := string(id)

	// Create buffered response channel (capacity 1 so reader never blocks).
	respCh := make(chan json.RawMessage, 1)
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

	req := &ipc.Request{
		Method: method,
		Params: paramsRaw,
		ID:     id,
	}

	// Send request (mu-protected write, safe for concurrent callers).
	if err := m.conn.WriteWithContext(ctx, req); err != nil {
		m.pending.Delete(idStr)
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Wait for response, context cancellation, or reader death.
	select {
	case raw := <-respCh:
		return raw, nil
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

// readLoop is the background reader goroutine. It reads response frames
// from the connection and routes them to waiting callers by request ID.
// Runs until the connection is closed or a read error occurs.
func (m *MuxConn) readLoop() {
	defer close(m.done)

	for {
		data, err := m.conn.reader.Read()
		if err != nil {
			// Store the error for late callers. Pending callers unblock
			// via the done channel (closed by defer). Don't close response
			// channels — a closed channel delivers a nil zero-value which
			// races with the done signal in CallRPC's select.
			m.readerErr.Store(&err)
			return
		}

		// Extract the response ID.
		var probe struct {
			ID json.RawMessage `json:"id"`
		}
		if unmarshalErr := json.Unmarshal(data, &probe); unmarshalErr != nil {
			slog.Warn("mux conn: unmarshal response id", "error", unmarshalErr)
			continue
		}

		idStr := string(probe.ID)

		// Look up and deliver to the waiting caller.
		val, ok := m.pending.LoadAndDelete(idStr)
		if !ok {
			// Orphaned response — caller timed out or canceled.
			slog.Warn("mux conn: orphaned response", "id", idStr)
			continue
		}

		ch, ok := val.(chan json.RawMessage)
		if !ok {
			continue
		}

		// Send the full raw frame to the caller.
		ch <- json.RawMessage(data)
	}
}
