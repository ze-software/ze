// Design: docs/architecture/api/ipc_protocol.md — plugin RPC connection
// Related: message.go — Request type, line parsing/formatting
// Related: framing.go — newline-delimited FrameReader/FrameWriter
// Related: mux.go — MuxConn multiplexer (consumes readFrame)
// Related: bridge.go — DirectBridge for internal plugins
//
// Package rpc defines the canonical wire-format types and shared connection
// logic for the ze plugin RPC protocol.
//
// Wire format: #<id> <verb> [<json>]\n
// Requests:    #<id> <method> [<json-params>]\n
// Success:     #<id> ok [<json-result>]\n
// Error:       #<id> error [<json-error>]\n
//
// Both the engine (internal/plugin) and the SDK (pkg/plugin/sdk) import this
// package to ensure a single source of truth for RPC message structures and
// connection handling.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// writeDeadliner is an optional interface for writers that support deadlines.
// net.Conn implements this; os.Stdout and SSH channels do not.
// When the writer does not support deadlines, writes may block longer
// but context cancellation on reads still prevents hangs.
type writeDeadliner interface {
	SetWriteDeadline(time.Time) error
}

// defaultWriteDeadline is used when the context has no deadline.
// Generous enough to never trigger during normal operation, but prevents
// writes from blocking indefinitely if the peer hangs.
const defaultWriteDeadline = 30 * time.Second

// Conn provides newline-framed RPC communication over a network connection.
//
// Wire format: #<id> <verb> [<json>]\n
//
// A persistent reader goroutine (started lazily on first read operation) owns
// the FrameReader exclusively. Callers receive frames via a channel, avoiding
// per-call goroutine spawning. Writes use SetWriteDeadline for context
// cancellation instead of a goroutine bridge.
//
// Conn supports two wiring modes:
//   - Single-socket: NewConn(conn, conn) -- read and write on the same socket.
//   - Cross-socket: NewConn(readConn, writeConn) -- read from one connection,
//     write to another. Used in tests with separate read/write endpoints.
//   - Stdio: NewConn(os.Stdin, os.Stdout) -- read from stdin, write to stdout.
//     Deadline-based write timeouts are skipped when the writer does not implement
//     SetWriteDeadline (e.g., os.File, SSH channels).
//
// Callers must call Close() to release resources. Close() unblocks the
// persistent reader by closing the read connection.
type Conn struct {
	reader      *FrameReader
	writer      *FrameWriter
	readCloser  io.ReadCloser
	writeCloser io.WriteCloser

	mu     sync.Mutex // Protects writes
	callMu sync.Mutex // Serializes CallRPC (write + read must be atomic)
	idSeq  atomic.Uint64

	// Persistent reader state (lazy-initialized via readerOnce).
	readerOnce sync.Once
	frameCh    chan []byte           // Successful frames from reader goroutine.
	readerDone chan struct{}         // Closed when reader goroutine exits.
	readerErr  atomic.Pointer[error] // Terminal error stored by reader on exit.
}

// NewConn creates a Conn that reads from reader and writes to writer.
// For single-socket use, pass the same conn for both arguments.
// The arguments accept any io.ReadCloser/io.WriteCloser; net.Conn satisfies both.
// When the writer supports SetWriteDeadline (e.g., net.Conn), writes use
// deadline-based timeouts. Otherwise (e.g., os.Stdout), deadlines are skipped.
func NewConn(reader io.ReadCloser, writer io.WriteCloser) *Conn {
	return &Conn{
		reader:      NewFrameReader(reader),
		writer:      NewFrameWriter(writer),
		readCloser:  reader,
		writeCloser: writer,
	}
}

// WriteConn returns the underlying write connection as a net.Conn, or nil
// if the writer is not a net.Conn. Used for out-of-band operations
// (SCM_RIGHTS fd passing) that need the raw net.Conn.
func (c *Conn) WriteConn() net.Conn {
	if nc, ok := c.writeCloser.(net.Conn); ok {
		return nc
	}
	return nil
}

// Close closes the read connection, unblocking the persistent reader goroutine.
// Safe to call multiple times. Does not close the write connection separately
// (in single-socket mode they are the same connection).
func (c *Conn) Close() error {
	return c.readCloser.Close()
}

// startReader lazily starts the persistent reader goroutine. Safe to call
// multiple times -- sync.Once ensures the goroutine starts exactly once.
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
		// Reader exited -- error was stored before readerDone was closed.
		if errPtr := c.readerErr.Load(); errPtr != nil {
			return nil, *errPtr
		}
		return nil, fmt.Errorf("connection closed")
	}
}

// NextID generates a unique request ID.
func (c *Conn) NextID() uint64 {
	return c.idSeq.Add(1)
}

// writeLineWithContext writes a line with context-derived write deadline.
// The deadline set, write, and deadline clear are all performed under c.mu
// to prevent interleaving when multiple goroutines write concurrently.
// When the writer does not support SetWriteDeadline (e.g., os.Stdout),
// deadline setting is skipped and writes may block longer.
func (c *Conn) writeLineWithContext(ctx context.Context, line []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dl, hasDeadline := ctx.Deadline()
	deadline := dl
	if !hasDeadline {
		deadline = time.Now().Add(defaultWriteDeadline)
	}

	c.mu.Lock()
	dlWriter, hasDL := c.writeCloser.(writeDeadliner)
	if hasDL {
		if err := dlWriter.SetWriteDeadline(deadline); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("set write deadline: %w", err)
		}
	}
	writeErr := c.writer.Write(line)
	if hasDL {
		_ = dlWriter.SetWriteDeadline(time.Time{})
	}
	c.mu.Unlock()

	if writeErr != nil {
		// When the write deadline came from the context, a write timeout
		// IS a context deadline exceeded. Check ctx.Err() but also handle
		// the race where the kernel fires the write timeout before Go's
		// context timer updates ctx.Err().
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if hasDeadline {
			return context.DeadlineExceeded
		}
		return writeErr
	}
	return nil
}

// ReadRequest reads the next incoming RPC request from the read connection.
// Parses #<id> <method> [<json>] format.
// Uses the persistent reader -- no goroutine is spawned per call.
func (c *Conn) ReadRequest(ctx context.Context) (*Request, error) {
	data, err := c.readFrame(ctx)
	if err != nil {
		return nil, err
	}
	id, verb, payload, err := ParseLine(data)
	if err != nil {
		return nil, fmt.Errorf("parse request: %w", err)
	}
	return &Request{
		ID:     id,
		Method: verb,
		Params: json.RawMessage(payload),
	}, nil
}

// SendResult sends a successful RPC response.
func (c *Conn) SendResult(ctx context.Context, id uint64, data any) error {
	var result json.RawMessage
	if data != nil {
		var err error
		result, err = json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal result data: %w", err)
		}
	}
	return c.writeLineWithContext(ctx, FormatResult(id, result))
}

// SendOK sends an empty successful RPC response.
func (c *Conn) SendOK(ctx context.Context, id uint64) error {
	return c.writeLineWithContext(ctx, FormatOK(id))
}

// SendError sends an error RPC response.
func (c *Conn) SendError(ctx context.Context, id uint64, message string) error {
	payload := NewErrorPayload("error", message)
	return c.writeLineWithContext(ctx, FormatError(id, payload))
}

// SendCodedError sends an error RPC response with a specific error code.
func (c *Conn) SendCodedError(ctx context.Context, id uint64, code, message string) error {
	payload := NewErrorPayload(code, message)
	return c.writeLineWithContext(ctx, FormatError(id, payload))
}

// CallRPC sends an RPC request and waits for the response.
// Returns the result JSON payload on success, or an *RPCCallError on RPC error.
// Serialized via callMu: concurrent callers block until the previous call completes.
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

	line := FormatRequest(id, method, paramsRaw)
	if err := c.writeLineWithContext(ctx, line); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Read response frame via persistent reader.
	data, err := c.readFrame(ctx)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseResponse(data, id)
}

// batchFrameOverhead is the maximum size of the non-event portion of a batch
// frame: #<id> ze-plugin-callback:deliver-batch {"events":[]}\n
// Conservative upper bound covering a 20-digit ID.
const batchFrameOverhead = 128

// CallBatchRPC writes deliver-batch frame(s) and reads response(s). If the
// events would produce a frame exceeding MaxMessageSize, the batch is split
// into sub-batches that each fit within the limit. Serialized via callMu.
func (c *Conn) CallBatchRPC(ctx context.Context, events [][]byte) (json.RawMessage, error) {
	c.callMu.Lock()
	defer c.callMu.Unlock()

	// Estimate total frame size.
	frameSize := batchFrameOverhead
	for i, e := range events {
		if i > 0 {
			frameSize++ // comma separator
		}
		frameSize += len(e)
	}

	// Fast path: single frame (common case).
	if frameSize <= MaxMessageSize {
		return c.callBatchOnce(ctx, events)
	}

	// Slow path: split into sub-batches that each fit within MaxMessageSize.
	maxPayload := MaxMessageSize - batchFrameOverhead
	var lastResp json.RawMessage
	start := 0

	for start < len(events) {
		end := start
		size := 0

		for end < len(events) {
			eventSize := len(events[end])
			if end > start {
				eventSize++ // comma separator
			}
			if size+eventSize > maxPayload && end > start {
				break
			}
			size += eventSize
			end++
		}

		resp, err := c.callBatchOnce(ctx, events[start:end])
		if err != nil {
			return nil, err
		}
		lastResp = resp
		start = end
	}

	return lastResp, nil
}

// callBatchOnce writes a single deliver-batch frame and reads its response.
// MUST be called with callMu held.
func (c *Conn) callBatchOnce(ctx context.Context, events [][]byte) (json.RawMessage, error) {
	id := c.idSeq.Add(1)

	if err := c.writeBatchWithDeadline(ctx, id, events); err != nil {
		return nil, fmt.Errorf("send batch: %w", err)
	}

	data, err := c.readFrame(ctx)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseResponse(data, id)
}

// writeBatchWithDeadline writes a batch frame with a write deadline derived
// from the context. Falls back to defaultWriteDeadline if ctx has no deadline.
// The deadline set, write, and deadline clear are all performed under c.mu
// to prevent interleaving when multiple goroutines write concurrently.
// When the writer does not support SetWriteDeadline, deadline setting is skipped.
func (c *Conn) writeBatchWithDeadline(ctx context.Context, id uint64, events [][]byte) error {
	dl, hasDeadline := ctx.Deadline()
	deadline := dl
	if !hasDeadline {
		deadline = time.Now().Add(defaultWriteDeadline)
	}

	c.mu.Lock()
	dlWriter, hasDL := c.writeCloser.(writeDeadliner)
	if hasDL {
		if err := dlWriter.SetWriteDeadline(deadline); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("set write deadline: %w", err)
		}
	}
	writeErr := WriteBatchFrame(c.writer.RawWriter(), id, events)
	if hasDL {
		_ = dlWriter.SetWriteDeadline(time.Time{})
	}
	c.mu.Unlock()

	// Match writeLineWithContext: prioritize writeErr, translate to ctx error.
	if writeErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if hasDeadline {
			return context.DeadlineExceeded
		}
		return writeErr
	}
	return nil
}

// WriteRawFrame writes pre-framed data (including newline terminator) directly.
// Used by batch delivery to bypass json.Marshal and per-frame allocation.
func (c *Conn) WriteRawFrame(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writer.WriteRaw(data)
}

// parseResponse interprets a response line and returns the result payload
// or an RPCCallError. Verifies the response ID matches the expected ID.
func parseResponse(data []byte, expectedID uint64) (json.RawMessage, error) {
	id, verb, payload, err := ParseLine(data)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if id != expectedID {
		return nil, fmt.Errorf("response id mismatch: sent %d, got %d", expectedID, id)
	}

	if verb == StatusOK {
		if len(payload) == 0 {
			return nil, nil
		}
		return json.RawMessage(payload), nil
	}
	if verb == StatusError {
		return nil, parseRPCError(payload)
	}
	return nil, fmt.Errorf("unexpected response verb %q", verb)
}
