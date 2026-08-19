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
	"log/slog"
	"net"
	"os"
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

	// Write watchdog for transports that do NOT implement SetWriteDeadline
	// (stdio via NewWithIO, SSH channels via adhoc.go). Deadline-capable
	// transports (net.Conn, net.Pipe) never arm the watchdog -- their write
	// path uses SetWriteDeadline instead, so this is zero-overhead for them.
	//
	// The timer is reused across writes (armed with Reset, disarmed with Stop
	// under c.mu) so the hot path allocates nothing after the first arm. When
	// a write blocks past watchdogWindow, fireWatchdog logs a warning, calls
	// the package write-watchdog hook (Prometheus counter), and closes the
	// connection to fail-fast (A-7). All watchdog fields are guarded by c.mu.
	watchdog       *time.Timer
	watchdogWindow time.Duration // 0 disables the watchdog.
	label          string        // Plugin/connection identity for logs+metrics.
}

// NewConn creates a Conn that reads from reader and writes to writer.
// For single-socket use, pass the same conn for both arguments.
// The arguments accept any io.ReadCloser/io.WriteCloser; net.Conn satisfies both.
// When the writer supports SetWriteDeadline (e.g., net.Conn), writes use
// deadline-based timeouts. Otherwise (e.g., os.Stdout), deadlines are skipped.
func NewConn(reader io.ReadCloser, writer io.WriteCloser) *Conn {
	return &Conn{
		reader:         NewFrameReader(reader),
		writer:         NewFrameWriter(writer),
		readCloser:     reader,
		writeCloser:    writer,
		watchdogWindow: defaultWriteDeadline,
	}
}

// SetLabel records a human-readable identity (typically the plugin name) used
// in write-watchdog warnings and metrics. Call once right after construction,
// before any concurrent writes.
func (c *Conn) SetLabel(label string) {
	c.mu.Lock()
	c.label = label
	c.mu.Unlock()
}

// SetWatchdogWindow overrides the write-watchdog window for transports that do
// not support SetWriteDeadline. A value <= 0 disables the watchdog. Call once
// right after construction, before any concurrent writes. Deadline-capable
// transports ignore this (they use SetWriteDeadline instead).
func (c *Conn) SetWatchdogWindow(d time.Duration) {
	c.mu.Lock()
	c.watchdogWindow = d
	c.mu.Unlock()
}

// writeWatchdogHook, when set, is invoked from fireWatchdog with the transport
// kind and connection label whenever a write stalls past the watchdog window.
// The engine wires this to a Prometheus counter at startup; it stays nil in
// standalone plugin processes (which have no engine metrics registry), where
// the warning log and fail-fast close still apply.
var writeWatchdogHook atomic.Pointer[func(transport, label string)]

// SetWriteWatchdogHook installs (or, with nil, clears) the process-wide write
// watchdog observer. Safe to call concurrently. Idempotent by last-writer.
func SetWriteWatchdogHook(fn func(transport, label string)) {
	if fn == nil {
		writeWatchdogHook.Store(nil)
		return
	}
	writeWatchdogHook.Store(&fn)
}

// transportKind returns a low-cardinality label describing the write transport,
// suitable for a Prometheus label. Only non-writeDeadliner writers reach the
// watchdog (net.Conn and *os.File pipes take the deadline path instead), so in
// practice this returns "pipe" (io.PipeWriter) or "stream" (SSH channels and
// other opaque io.WriteCloser transports).
func transportKind(w io.WriteCloser) string {
	switch w.(type) {
	case *os.File:
		return "file"
	case *io.PipeWriter:
		return "pipe"
	default:
		return "stream"
	}
}

// armWatchdogLocked starts (or restarts) the reusable watchdog timer. Caller
// MUST hold c.mu. No-op when the watchdog is disabled (window <= 0). The first
// arm allocates the timer via AfterFunc; subsequent arms reuse it via Reset,
// so the steady-state write path allocates nothing.
func (c *Conn) armWatchdogLocked() {
	if c.watchdogWindow <= 0 {
		return
	}
	if c.watchdog == nil {
		c.watchdog = time.AfterFunc(c.watchdogWindow, c.fireWatchdog)
		return
	}
	c.watchdog.Reset(c.watchdogWindow)
}

// disarmWatchdogLocked stops the watchdog after a write completes. Caller MUST
// hold c.mu. If the timer already fired, Stop reports false and the connection
// is already being torn down by fireWatchdog -- the in-flight write returns an
// error, which is the intended fail-fast outcome.
func (c *Conn) disarmWatchdogLocked() {
	if c.watchdog != nil {
		c.watchdog.Stop()
	}
}

// fireWatchdog runs (in the timer goroutine) when a write blocks past the
// watchdog window. It logs, notifies the metric hook, and closes both ends to
// unblock the stalled write. It must NOT take c.mu: the stalled writer holds it.
func (c *Conn) fireWatchdog() {
	kind := transportKind(c.writeCloser)
	slog.Warn("plugin rpc write stalled past watchdog window; closing connection (fail-fast)",
		"transport", kind, "label", c.label, "window", c.watchdogWindow)
	if p := writeWatchdogHook.Load(); p != nil {
		(*p)(kind, c.label)
	}
	_ = c.readCloser.Close()
	_ = c.writeCloser.Close()
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
//
// A frame the reader already delivered outranks the reader's terminal error: a
// peer that writes its last line and closes the connection stores that error
// while the line is still queued, and the line is data the peer did send.
func (c *Conn) readFrame(ctx context.Context) ([]byte, error) {
	c.startReader()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case data := <-c.frameCh:
		return data, nil
	case <-c.readerDone:
		return c.queuedFrame()
	}
}

// queuedFrame returns a frame the reader delivered before it stopped, and the
// reason it stopped once none is left. The error was stored before readerDone
// was closed, so it is readable here.
func (c *Conn) queuedFrame() ([]byte, error) {
	select {
	case data := <-c.frameCh:
		return data, nil
	default:
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

// writeAppended formats a line into a pooled buffer via the appender,
// appends the newline terminator, and writes the single buffer to the
// underlying writer under c.mu. Callers provide an Append* helper from
// message.go as the appender; the helper writes into the pool buffer
// with no intermediate allocation.
//
// This is the hot-path alternative to writeLineWithContext: it avoids
// both the Format*-side []byte allocation and FrameWriter.Write's
// per-call copy.
func (c *Conn) writeAppended(ctx context.Context, appender func([]byte) []byte) error {
	bp := getFrameBuf()
	defer putFrameBuf(bp)

	buf := appender(*bp)
	buf = append(buf, '\n')
	*bp = buf

	return c.writeFrame(ctx, buf)
}

// writeFrame writes one already-framed message, its newline terminator
// included, to the underlying writer under c.mu. The lock spans the deadline
// set, the write, and the deadline clear, so two goroutines writing at once
// never interleave a partial frame.
//
// It is the shared write core: writeAppended formats into a pooled buffer and
// calls it, and answerWriter hands it a line another package framed.
func (c *Conn) writeFrame(ctx context.Context, buf []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dl, hasDeadline := ctx.Deadline()
	deadline := dl
	if !hasDeadline {
		deadline = time.Now().Add(defaultWriteDeadline)
	}

	if len(buf) > MaxMessageSize+1 {
		return fmt.Errorf("message exceeds maximum size %d", MaxMessageSize)
	}

	c.mu.Lock()
	dlWriter, hasDL := c.writeCloser.(writeDeadliner)
	if hasDL {
		if err := dlWriter.SetWriteDeadline(deadline); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("set write deadline: %w", err)
		}
	} else {
		c.armWatchdogLocked()
	}
	_, writeErr := c.writer.RawWriter().Write(buf)
	if hasDL {
		_ = dlWriter.SetWriteDeadline(time.Time{})
	} else {
		c.disarmWatchdogLocked()
	}
	c.mu.Unlock()

	if writeErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if hasDeadline {
			return context.DeadlineExceeded
		}
		return fmt.Errorf("write frame: %w", writeErr)
	}
	return nil
}

// AnswerWriter returns the writer one answer sequence is written to. Each Write
// takes ONE framed line, its newline included, and puts it on the wire under
// the connection's write lock, so no other writer interleaves a line into the
// middle of an answer. The producer is WriteAnswer
// (internal/component/plugin/dispatch.go), which frames every line it writes.
//
// The returned writer is for the one answer it is created for, and it MUST NOT
// outlive the call that created it. Safe for concurrent use in the sense that
// each Write is one atomic frame; the lines of one answer are written in order
// by the one goroutine producing them.
func (c *Conn) AnswerWriter(ctx context.Context) io.Writer {
	return answerWriter{conn: c, ctx: ctx}
}

// answerWriter is the one-answer writer AnswerWriter returns.
type answerWriter struct {
	conn *Conn
	// ctx is held rather than passed because io.Writer states no context
	// parameter, and every line of an answer must carry the deadline of the
	// call that produced it. AnswerWriter's doc states the lifetime that makes
	// this safe.
	ctx context.Context //nolint:containedctx // io.Writer has no ctx parameter; see AnswerWriter
}

// Write puts one framed answer line on the wire. line MUST already end with the
// newline that terminates a message, and MUST be one whole line: this writer
// frames nothing and buffers nothing.
func (w answerWriter) Write(line []byte) (int, error) {
	if err := w.conn.writeFrame(w.ctx, line); err != nil {
		return 0, err
	}
	return len(line), nil
}

// writeLineWithContext writes a line with context-derived write deadline.
// The deadline set, write, and deadline clear are all performed under c.mu
// to prevent interleaving when multiple goroutines write concurrently.
// When the writer does not support SetWriteDeadline (e.g., os.Stdout),
// deadline setting is skipped and writes may block longer.
//
// Retained for tests and external callers that already hold a
// pre-formatted []byte. Production hot paths use writeAppended to skip
// both the Format*-side allocation and FrameWriter's per-write copy.
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
	} else {
		c.armWatchdogLocked()
	}
	writeErr := c.writer.Write(line)
	if hasDL {
		_ = dlWriter.SetWriteDeadline(time.Time{})
	} else {
		c.disarmWatchdogLocked()
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
	return c.writeAppended(ctx, func(buf []byte) []byte {
		return AppendResult(buf, id, result)
	})
}

// SendOK sends an empty successful RPC response.
func (c *Conn) SendOK(ctx context.Context, id uint64) error {
	return c.writeAppended(ctx, func(buf []byte) []byte {
		return AppendOK(buf, id)
	})
}

// SendError sends an error RPC response.
func (c *Conn) SendError(ctx context.Context, id uint64, message string) error {
	payload := NewErrorPayload("error", message)
	return c.writeAppended(ctx, func(buf []byte) []byte {
		return AppendError(buf, id, payload)
	})
}

// SendCodedError sends an error RPC response with a specific error code.
func (c *Conn) SendCodedError(ctx context.Context, id uint64, code, message string) error {
	payload := NewErrorPayload(code, message)
	return c.writeAppended(ctx, func(buf []byte) []byte {
		return AppendError(buf, id, payload)
	})
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

	writeErr := c.writeAppended(ctx, func(buf []byte) []byte {
		return AppendRequest(buf, id, method, paramsRaw)
	})
	if writeErr != nil {
		return nil, fmt.Errorf("send request: %w", writeErr)
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
	} else {
		c.armWatchdogLocked()
	}
	writeErr := WriteBatchFrame(c.writer.RawWriter(), id, events)
	if hasDL {
		_ = dlWriter.SetWriteDeadline(time.Time{})
	} else {
		c.disarmWatchdogLocked()
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
