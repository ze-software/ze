// Design: docs/architecture/mcp/overview.md -- per-POST reply sinks for MCP 2025-06-18
// Related: elicit.go -- session.Elicit calls UpgradeToSSE to flip the sink mid-call
// Related: streamable.go -- handlePOST binds a jsonReplySink before method dispatch

// Per-POST reply sinks implementing replySink.
//
// jsonReplySink is the default: handlePOST binds it to a new session-
// scoped slot before running a tool handler. If the handler never elicits,
// handlePOST writes the terminal JSON-RPC response via the sink as a
// single `application/json` body (or uses writeJSONResponse directly; the
// sink's WriteFrame accomplishes the same).
//
// sseReplySink is the upgrade target. session.Elicit calls
// session.UpgradeCurrentSinkToSSE, which invokes jsonReplySink.UpgradeToSSE,
// which writes the SSE response headers + a flush and returns an
// sseReplySink wrapping the same underlying http.ResponseWriter. Every
// subsequent WriteFrame on the session's sink lands as an SSE `data: ...`
// frame, including the terminal tool result that runMethod produces.
//
// Reference: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
package mcp

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// jsonReplySink wraps the originating POST's http.ResponseWriter for the
// single-response case. Safe under concurrent callers (WriteFrame and
// UpgradeToSSE both acquire the internal lock) even though handlePOST
// only uses one at a time -- defense against a future regression that
// shares the sink across goroutines.
type jsonReplySink struct {
	w  http.ResponseWriter
	mu sync.Mutex
	// written flips true after the first terminal write (WriteFrame) OR
	// after UpgradeToSSE claims the writer. Either outcome consumes the
	// single-response slot; subsequent calls on this sink return errors.
	written bool
}

// newJSONReplySink returns a fresh jsonReplySink wrapping w. Caller MUST
// bind it to the session via SetActivePostSink.
func newJSONReplySink(w http.ResponseWriter) *jsonReplySink {
	return &jsonReplySink{w: w}
}

// WriteFrame writes a single JSON body with Content-Type application/json.
// Subsequent calls return errJSONSinkAlreadyWritten so a misbehaving
// handler cannot emit two responses on one POST.
func (s *jsonReplySink) WriteFrame(frame []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.written {
		return errJSONSinkAlreadyWritten
	}
	s.written = true
	s.w.Header().Set("Content-Type", "application/json")
	_, err := s.w.Write(frame)
	return err
}

// IsSSE reports false -- this sink is the JSON variant.
func (*jsonReplySink) IsSSE() bool { return false }

// UpgradeToSSE writes SSE response headers + a flush and returns an
// sseReplySink wrapping the same writer. Returns an error if the body
// has already been written (headers already committed) or if the
// writer does not implement http.Flusher (streaming unavailable).
//
// RFC / spec rationale: MCP 2025-06-18 basic/transports permits either
// application/json OR text/event-stream as the POST response body; the
// server decides based on whether sub-messages (elicitation/create,
// task notifications) fire during the call. Phase 3 implements the
// elicitation leg of that choice.
func (s *jsonReplySink) UpgradeToSSE() (replySink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.written {
		return nil, errJSONSinkAlreadyWritten
	}
	flusher, ok := s.w.(http.Flusher)
	if !ok {
		return nil, errors.New("mcp: streaming unsupported by the HTTP transport")
	}
	// Set SSE headers. Origin / session / proto headers were set earlier
	// in handlePOST (setMainPathCORS); we only set the streaming-specific
	// ones here so behavior matches handleGET.
	h := s.w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Defeat intermediary buffering (nginx, common CDNs) so the first
	// elicit frame reaches the client promptly instead of sitting until
	// a buffer page fills. Same rationale as handleGET's X-Accel-Buffering.
	h.Set("X-Accel-Buffering", "no")
	s.w.WriteHeader(http.StatusOK)
	flusher.Flush()
	s.written = true
	return &sseReplySink{w: s.w, flusher: flusher}, nil
}

// sseReplySink writes SSE `data: <frame>\n\n` events to the upgraded
// POST response. Each WriteFrame flushes so the client sees frames
// immediately. Flushing is what makes the POST-upgrade path work as a
// real stream -- without it, frames sit in the Go net/http buffer until
// the handler returns.
type sseReplySink struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

// WriteFrame emits one SSE data event carrying the given JSON-RPC frame.
//
// INVARIANT: `frame` MUST be a single-line JSON document with no literal
// newline bytes. SSE terminates an event on a blank line, so an embedded
// `\n` would fragment the event and the client's SSE parser would emit
// separate data lines that, when concatenated, are not the frame the
// server intended. In practice callers build frames with `json.Marshal`
// (used by `buildElicitFrame`) which escapes newlines as `\n` escape
// sequences; if a future caller assembles a frame by hand it must
// respect the same invariant.
func (s *sseReplySink) WriteFrame(frame []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", frame); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// IsSSE reports true -- the upgrade has happened.
func (*sseReplySink) IsSSE() bool { return true }

// UpgradeToSSE on an already-SSE sink is a no-op that returns self.
func (s *sseReplySink) UpgradeToSSE() (replySink, error) { return s, nil }

// errJSONSinkAlreadyWritten is returned by jsonReplySink when WriteFrame
// or UpgradeToSSE is called after the single-response slot was consumed.
var errJSONSinkAlreadyWritten = errors.New("mcp: json reply sink already written")
