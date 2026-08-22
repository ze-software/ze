// Design: docs/architecture/core-design.md — MuxConn wire format parsing for bridge runtime
// Overview: bridge.go — startup protocol, bridge runtime
// Related: bridge_event.go — ZeBGP to ExaBGP JSON event translation
// Related: bridge_command.go — ExaBGP text command translation

package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// parseMuxLine parses a MuxConn wire format line: #<len>:<id> <verb> [<payload>].
// Returns the request ID, verb (method name or "ok"/"error"), and optional payload.
//
// The line grammar has one reader, rpc.ParseLine, so the bridge and the daemon
// cannot disagree about where a field starts. This wrapper only puts the result
// in the string shape the bridge works in.
func parseMuxLine(line string) (id uint64, verb, payload string, err error) {
	id, verb, payloadBytes, err := rpc.ParseLine([]byte(line))
	if err != nil {
		return 0, "", "", err
	}
	return id, verb, string(payloadBytes), nil
}

// formatMuxOK formats a successful MuxConn response: #<len>:<id> ok.
func formatMuxOK(id uint64) string {
	return string(rpc.AppendOK(nil, id))
}

// formatDispatchRequest formats a MuxConn dispatch-command request:
// #<len>:<id> ze-plugin-engine:dispatch-command {"command":"<cmd>"}.
func formatDispatchRequest(id uint64, command string) string {
	payload, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		// command is always a plain string, so Marshal cannot fail here. Build
		// the one-member object by hand rather than drop the command.
		var b textbuf.Buffer
		payload = json.RawMessage(b.Str(`{"command":`).Quoted(command).Byte('}').String())
	}
	return string(rpc.AppendRequest(nil, id, "ze-plugin-engine:dispatch-command", payload))
}

// extractBatchEvents extracts event strings from a deliver-batch JSON payload.
// The payload format is: {"events":["<json-string>","<json-string>",...]}.
func extractBatchEvents(payload string) ([]string, error) {
	var batch struct {
		Events []string `json:"events"`
	}
	if err := json.Unmarshal([]byte(payload), &batch); err != nil {
		return nil, fmt.Errorf("unmarshal deliver-batch: %w", err)
	}
	return batch.Events, nil
}

// formatFlushRequest formats a MuxConn peer-flush RPC request:
// #<len>:<id> ze-bgp:peer-flush {"selector":"<addr>"}.
func formatFlushRequest(id uint64, selector string) string {
	payload, err := json.Marshal(map[string]string{"selector": selector})
	if err != nil {
		// selector is always a plain string, so this is the same unreachable
		// branch formatDispatchRequest carries, built the same way.
		var b textbuf.Buffer
		payload = json.RawMessage(b.Str(`{"selector":`).Quoted(selector).Byte('}').String())
	}
	return string(rpc.AppendRequest(nil, id, "ze-bgp:peer-flush", payload))
}

// ExtractPeerAddress extracts the peer address from a translated ZeBGP command.
// Commands have the format "peer <addr> update text ...". Returns "" if no peer prefix.
func ExtractPeerAddress(command string) string {
	if !strings.HasPrefix(command, "peer ") {
		return ""
	}
	addr, _, ok := strings.Cut(command[5:], " ")
	if !ok {
		return ""
	}
	return addr
}

// IsRouteCommand returns true if the translated command is a route update.
func IsRouteCommand(command string) bool {
	return strings.Contains(command, "update text")
}

// pendingResult captures the outcome of a dispatched request: success or
// the error message sent back as the MuxConn error payload.
type pendingResult struct {
	ok      bool
	errText string
}

// pendingResponses tracks in-flight RPC requests that need a response.
// The command goroutine registers a channel before sending the request,
// then blocks on it. The event goroutine signals the channel when the
// response arrives.
type pendingResponses struct {
	mu      sync.Mutex
	waiters map[uint64]chan pendingResult
}

func newPendingResponses() *pendingResponses {
	return &pendingResponses{waiters: make(map[uint64]chan pendingResult)}
}

// register creates a channel for the given request ID. MUST be called before
// the request is sent (before the response can arrive).
func (p *pendingResponses) register(id uint64) chan pendingResult {
	ch := make(chan pendingResult, 1)
	p.mu.Lock()
	p.waiters[id] = ch
	p.mu.Unlock()
	return ch
}

// isWaiting reports whether a waiter is currently registered for the given
// request ID, without consuming it (unlike signal).
//
// Test-only introspection: it has no production caller by design. Tests use it
// to observe that the command goroutine has reached its register/wait point
// before delivering a response, replacing timing-based ordering guesses. It
// takes p.mu and does not mutate state, so it is safe alongside the live path.
func (p *pendingResponses) isWaiting(id uint64) bool {
	p.mu.Lock()
	_, found := p.waiters[id]
	p.mu.Unlock()
	return found
}

// signal delivers a result for the given request ID. Returns true if a
// waiter was found and signaled. Callers must pass pendingResult{ok: true}
// for plain "ok" responses and {ok: false, errText: ...} for errors.
func (p *pendingResponses) signal(id uint64, result pendingResult) bool {
	p.mu.Lock()
	ch, found := p.waiters[id]
	if found {
		delete(p.waiters, id)
	}
	p.mu.Unlock()
	if found {
		ch <- result
	}
	return found
}

// wait blocks until the response arrives or the context is canceled.
// Returns the pendingResult on success or a zero-value result plus ctx.Err().
//
// id is the request ID registered via p.register; on ctx cancel we remove
// the waiter so a late signal does not leak a map entry.
func (p *pendingResponses) wait(ctx context.Context, id uint64, ch chan pendingResult) (pendingResult, error) {
	select {
	case r := <-ch:
		return r, nil
	case <-ctx.Done():
		p.mu.Lock()
		delete(p.waiters, id)
		p.mu.Unlock()
		return pendingResult{}, ctx.Err()
	}
}

// syncWriter wraps an io.Writer with a mutex for safe concurrent writes.
// Both bridge goroutines (event and command) write to ze stdout after stage 5.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// Fprintln writes a line to the underlying writer, protected by the mutex.
func (sw *syncWriter) Fprintln(s string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if _, err := fmt.Fprintln(sw.w, s); err != nil { //nolint:errcheck // output
		slog.Warn("syncWriter: write failed", "error", err)
	}
}
