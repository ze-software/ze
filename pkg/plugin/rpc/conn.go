// Package rpc defines the canonical wire-format types and shared connection
// logic for the ze plugin RPC protocol.
//
// Both the engine (internal/plugin) and the SDK (pkg/plugin/sdk) import this
// package to ensure a single source of truth for RPC message structures and
// connection handling.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/ipc"
)

// Conn provides NUL-framed JSON RPC communication over a network connection.
//
// Conn supports two wiring modes:
//   - Single-socket: NewConn(conn, conn) — read and write on the same socket.
//     Used by the SDK where each end of a net.Pipe is wrapped independently.
//   - Cross-socket: NewConn(readConn, writeConn) — read from one socket,
//     write to another. Used in tests to simulate the two-socket architecture.
//
// Callers must call Close() to release resources and unblock any goroutines
// waiting on Read(). Without Close(), goroutines spawned by ReadRequest and
// CallRPC will leak if context cancellation races with a blocking read.
type Conn struct {
	reader   *ipc.FrameReader
	writer   *ipc.FrameWriter
	readConn net.Conn

	mu    sync.Mutex // Protects writes
	idSeq atomic.Uint64
}

// NewConn creates a Conn that reads from readConn and writes to writeConn.
// For single-socket use, pass the same conn for both arguments.
func NewConn(readConn, writeConn net.Conn) *Conn {
	return &Conn{
		reader:   ipc.NewFrameReader(readConn),
		writer:   ipc.NewFrameWriter(writeConn),
		readConn: readConn,
	}
}

// Close closes the read connection, unblocking any goroutines waiting on Read().
// Safe to call multiple times. Does not close the write connection separately
// (in single-socket mode they are the same connection).
func (c *Conn) Close() error {
	return c.readConn.Close()
}

// NextID generates a unique request ID as a JSON number.
func (c *Conn) NextID() json.RawMessage {
	id := c.idSeq.Add(1)
	return json.RawMessage(fmt.Sprintf("%d", id))
}

// WriteFrame marshals v to JSON and sends it as a NUL-terminated frame.
func (c *Conn) WriteFrame(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writer.Write(data)
}

// ReadRequest reads the next incoming RPC request from the read connection.
func (c *Conn) ReadRequest(ctx context.Context) (*ipc.Request, error) {
	type result struct {
		req *ipc.Request
		err error
	}
	ch := make(chan result, 1)
	go func() {
		data, err := c.reader.Read()
		if err != nil {
			ch <- result{nil, err}
			return
		}
		var req ipc.Request
		if err := json.Unmarshal(data, &req); err != nil {
			ch <- result{nil, fmt.Errorf("unmarshal request: %w", err)}
			return
		}
		ch <- result{&req, nil}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.req, r.err
	}
}

// SendResult sends a successful RPC response with the given ID and data.
func (c *Conn) SendResult(ctx context.Context, id json.RawMessage, data any) error {
	var result json.RawMessage
	if data != nil {
		var err error
		result, err = json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal result data: %w", err)
		}
	}
	resp := &ipc.RPCResult{
		Result: result,
		ID:     id,
	}
	return c.WriteWithContext(ctx, resp)
}

// SendOK sends an empty successful RPC response.
func (c *Conn) SendOK(ctx context.Context, id json.RawMessage) error {
	resp := &ipc.RPCResult{ID: id}
	return c.WriteWithContext(ctx, resp)
}

// SendError sends an error RPC response with the given ID and error name.
func (c *Conn) SendError(ctx context.Context, id json.RawMessage, errorName string) error {
	resp := &ipc.RPCError{
		Error: errorName,
		ID:    id,
	}
	return c.WriteWithContext(ctx, resp)
}

// CallRPC sends an RPC request and waits for the response.
// Returns the raw response frame for the caller to interpret.
func (c *Conn) CallRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.NextID()

	// Marshal params.
	var paramsRaw json.RawMessage
	if params != nil {
		var err error
		paramsRaw, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
	}

	req := &ipc.Request{
		Method: method,
		Params: paramsRaw,
		ID:     id,
	}

	if err := c.WriteWithContext(ctx, req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Read response frame (blocking).
	type readResult struct {
		data []byte
		err  error
	}
	readCh := make(chan readResult, 1)
	go func() {
		data, err := c.reader.Read()
		readCh <- readResult{data, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-readCh:
		if r.err != nil {
			return nil, fmt.Errorf("read response: %w", r.err)
		}

		// Verify response ID matches request ID.
		var probe struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(r.data, &probe); err != nil {
			return nil, fmt.Errorf("unmarshal response id: %w", err)
		}
		if string(probe.ID) != string(id) {
			return nil, fmt.Errorf("response id mismatch: sent %s, got %s", id, probe.ID)
		}

		return json.RawMessage(r.data), nil
	}
}

// WriteWithContext sends a frame with context cancellation support.
func (c *Conn) WriteWithContext(ctx context.Context, v any) error {
	ch := make(chan error, 1)
	go func() {
		ch <- c.WriteFrame(v)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		return err
	}
}

// CheckResponse verifies that a raw response is not an error.
// Returns nil if the response is successful, or an error if it contains an RPC error.
func CheckResponse(raw json.RawMessage) error {
	var probe struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	if probe.Error != "" {
		return fmt.Errorf("rpc error: %s", probe.Error)
	}
	return nil
}

// ParseResponse checks if the raw response is an error or a result.
// Returns the result data, or an error if the response is an RPCError.
func ParseResponse(raw json.RawMessage) (json.RawMessage, error) {
	var probe struct {
		Error  string          `json:"error,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if probe.Error != "" {
		return nil, fmt.Errorf("rpc error: %s", probe.Error)
	}
	return probe.Result, nil
}
