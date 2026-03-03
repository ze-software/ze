// Design: docs/architecture/api/ipc_protocol.md — plugin RPC types
// Related: mux.go — MuxConn multiplexer (consumes readFrame)
// Related: text_conn.go — TextConn text-mode framing alternative
// Related: bridge.go — DirectBridge for internal plugins
//
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
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/ipc"
)

// defaultWriteDeadline is used when the context has no deadline.
// Generous enough to never trigger during normal operation, but prevents
// writes from blocking indefinitely if the peer hangs.
const defaultWriteDeadline = 30 * time.Second

// Conn provides NUL-framed JSON RPC communication over a network connection.
//
// A persistent reader goroutine (started lazily on first read operation) owns
// the FrameReader exclusively. Callers receive frames via a channel, avoiding
// per-call goroutine spawning. Writes use SetWriteDeadline for context
// cancellation instead of a goroutine bridge.
//
// Conn supports two wiring modes:
//   - Single-socket: NewConn(conn, conn) — read and write on the same socket.
//   - Cross-socket: NewConn(readConn, writeConn) — read from one socket,
//     write to another. Used in tests to simulate the two-socket architecture.
//
// Callers must call Close() to release resources. Close() unblocks the
// persistent reader by closing the read connection.
type Conn struct {
	reader    *ipc.FrameReader
	writer    *ipc.FrameWriter
	readConn  net.Conn
	writeConn net.Conn

	mu     sync.Mutex // Protects writes
	callMu sync.Mutex // Serializes CallRPC (write + read must be atomic)
	idSeq  atomic.Uint64

	// Persistent reader state (lazy-initialized via readerOnce).
	readerOnce sync.Once
	frameCh    chan []byte           // Successful frames from reader goroutine.
	readerDone chan struct{}         // Closed when reader goroutine exits.
	readerErr  atomic.Pointer[error] // Terminal error stored by reader on exit.
}

// NewConn creates a Conn that reads from readConn and writes to writeConn.
// For single-socket use, pass the same conn for both arguments.
func NewConn(readConn, writeConn net.Conn) *Conn {
	return &Conn{
		reader:    ipc.NewFrameReader(readConn),
		writer:    ipc.NewFrameWriter(writeConn),
		readConn:  readConn,
		writeConn: writeConn,
	}
}

// Close closes the read connection, unblocking the persistent reader goroutine.
// Safe to call multiple times. Does not close the write connection separately
// (in single-socket mode they are the same connection).
func (c *Conn) Close() error {
	return c.readConn.Close()
}

// startReader lazily starts the persistent reader goroutine. Safe to call
// multiple times — sync.Once ensures the goroutine starts exactly once.
//
// MuxConn's readLoop calls readFrame (which calls startReader), so both
// Conn and MuxConn share the same persistent reader goroutine.
func (c *Conn) startReader() {
	c.readerOnce.Do(func() {
		c.frameCh = make(chan []byte, 1)
		c.readerDone = make(chan struct{})
		go c.readLoop()
	})
}

// readLoop is the persistent reader goroutine. It reads frames from the
// connection and pushes successful frames to frameCh. On error (EOF, broken
// pipe, close), it stores the error atomically and exits. The deferred
// close(readerDone) fires after readerErr is stored.
func (c *Conn) readLoop() {
	defer close(c.readerDone)
	for {
		data, err := c.reader.Read()
		if err != nil {
			c.readerErr.Store(&err)
			return
		}
		c.frameCh <- data
	}
}

// readFrame waits for the next frame from the persistent reader, respecting
// context cancellation. Returns the raw frame bytes or an error.
func (c *Conn) readFrame(ctx context.Context) ([]byte, error) {
	c.startReader()

	// Fast path: if reader already failed, return stored error immediately.
	if errPtr := c.readerErr.Load(); errPtr != nil {
		return nil, *errPtr
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case data := <-c.frameCh:
		return data, nil
	case <-c.readerDone:
		// Reader exited — error was stored before readerDone was closed.
		if errPtr := c.readerErr.Load(); errPtr != nil {
			return nil, *errPtr
		}
		return nil, fmt.Errorf("connection closed")
	}
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

// WriteRawFrame writes pre-framed data (including NUL terminator) directly.
// Used by batch delivery to bypass json.Marshal and per-frame allocation.
func (c *Conn) WriteRawFrame(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writer.WriteRaw(data)
}

// ReadRequest reads the next incoming RPC request from the read connection.
// Uses the persistent reader — no goroutine is spawned per call.
func (c *Conn) ReadRequest(ctx context.Context) (*ipc.Request, error) {
	data, err := c.readFrame(ctx)
	if err != nil {
		return nil, err
	}
	var req ipc.Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}
	return &req, nil
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
// Serialized via callMu: concurrent callers block until the previous call completes.
// Uses the persistent reader for the response and deadline-based writes.
func (c *Conn) CallRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.callMu.Lock()
	defer c.callMu.Unlock()

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

	// Read response frame via persistent reader.
	data, err := c.readFrame(ctx)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Verify response ID matches request ID.
	var probe struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("unmarshal response id: %w", err)
	}
	if string(probe.ID) != string(id) {
		return nil, fmt.Errorf("response id mismatch: sent %s, got %s", id, probe.ID)
	}

	return json.RawMessage(data), nil
}

// CallBatchRPC writes a deliver-batch frame using a pooled buffer and reads the response.
// Bypasses json.Marshal and FrameWriter.Write allocation. Serialized via callMu.
// Uses deadline-based writes and the persistent reader.
func (c *Conn) CallBatchRPC(ctx context.Context, events [][]byte) (json.RawMessage, error) {
	c.callMu.Lock()
	defer c.callMu.Unlock()

	id := c.idSeq.Add(1)

	// Write batch frame using deadline-based write.
	if err := c.writeBatchWithDeadline(ctx, id, events); err != nil {
		return nil, fmt.Errorf("send batch: %w", err)
	}

	// Read response frame via persistent reader.
	data, err := c.readFrame(ctx)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Verify response ID matches.
	var probe struct {
		ID uint64 `json:"id"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("unmarshal response id: %w", err)
	}
	if probe.ID != id {
		return nil, fmt.Errorf("response id mismatch: sent %d, got %d", id, probe.ID)
	}

	return json.RawMessage(data), nil
}

// writeBatchWithDeadline writes a batch frame with a write deadline derived
// from the context. Falls back to defaultWriteDeadline if ctx has no deadline.
func (c *Conn) writeBatchWithDeadline(ctx context.Context, id uint64, events [][]byte) error {
	deadline := writeDeadline(ctx)
	if err := c.writeConn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}
	c.mu.Lock()
	err := ipc.WriteBatchFrame(c.writer.RawWriter(), id, events)
	c.mu.Unlock()
	// Clear deadline regardless of write outcome.
	if clearErr := c.writeConn.SetWriteDeadline(time.Time{}); clearErr != nil {
		return fmt.Errorf("clear write deadline: %w", clearErr)
	}
	// Translate timeout to context error when context is also done.
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// WriteWithContext sends a frame with context-derived write deadline.
// Uses SetWriteDeadline instead of a goroutine bridge. If the write times
// out and the context is also done, returns ctx.Err() to preserve the
// caller's expected error semantics.
func (c *Conn) WriteWithContext(ctx context.Context, v any) error {
	// Fast check: if context is already done, don't attempt the write.
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline := writeDeadline(ctx)
	if err := c.writeConn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}
	writeErr := c.WriteFrame(v)
	// Clear deadline regardless of write outcome.
	if clearErr := c.writeConn.SetWriteDeadline(time.Time{}); clearErr != nil {
		return fmt.Errorf("clear write deadline: %w", clearErr)
	}
	// Translate timeout to context error when context is also done.
	if writeErr != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return writeErr
}

// writeDeadline extracts a deadline from ctx, falling back to defaultWriteDeadline.
func writeDeadline(ctx context.Context) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	return time.Now().Add(defaultWriteDeadline)
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
