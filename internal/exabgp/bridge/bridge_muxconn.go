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
	"strconv"
	"strings"
	"sync"
)

// parseMuxLine parses a MuxConn wire format line: #<id> <verb> [<payload>].
// Returns the request ID, verb (method name or "ok"/"error"), and optional payload.
func parseMuxLine(line string) (id uint64, verb, payload string, err error) {
	if !strings.HasPrefix(line, "#") {
		return 0, "", "", fmt.Errorf("line missing # prefix: %q", truncate(line, 80))
	}

	rest := line[1:] // strip #

	idStr, after, hasAfter := strings.Cut(rest, " ")
	if !hasAfter || after == "" {
		return 0, "", "", fmt.Errorf("line has no verb after #%s", idStr)
	}

	id, err = strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid id %q: %w", idStr, err)
	}

	verb, payload, _ = strings.Cut(after, " ")

	return id, verb, payload, nil
}

// formatMuxOK formats a successful MuxConn response: #<id> ok.
func formatMuxOK(id uint64) string {
	return fmt.Sprintf("#%d ok", id)
}

// formatDispatchRequest formats a MuxConn dispatch-command request:
// #<id> ze-plugin-engine:dispatch-command {"command":"<cmd>"}.
func formatDispatchRequest(id uint64, command string) string {
	payload, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		// command is always a plain string; Marshal cannot fail here.
		// Log defensively and fall back to unescaped embedding.
		return fmt.Sprintf("#%d ze-plugin-engine:dispatch-command {\"command\":%q}", id, command)
	}
	return fmt.Sprintf("#%d ze-plugin-engine:dispatch-command %s", id, string(payload))
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
// #<id> ze-bgp:peer-flush {"selector":"<addr>"}.
func formatFlushRequest(id uint64, selector string) string {
	payload, err := json.Marshal(map[string]string{"selector": selector})
	if err != nil {
		return fmt.Sprintf("#%d ze-bgp:peer-flush {\"selector\":%q}", id, selector)
	}
	return fmt.Sprintf("#%d ze-bgp:peer-flush %s", id, string(payload))
}

// extractPeerAddress extracts the peer address from a translated ZeBGP command.
// Commands have the format "peer <addr> update text ...". Returns "" if no peer prefix.
func extractPeerAddress(command string) string {
	if !strings.HasPrefix(command, "peer ") {
		return ""
	}
	addr, _, ok := strings.Cut(command[5:], " ")
	if !ok {
		return ""
	}
	return addr
}

// isRouteCommand returns true if the translated command is a route update.
func isRouteCommand(command string) bool {
	return strings.Contains(command, "update text")
}

// pendingResponses tracks in-flight RPC requests that need a response.
// The command goroutine registers a channel before sending the request,
// then blocks on it. The event goroutine signals the channel when the
// response arrives.
type pendingResponses struct {
	mu      sync.Mutex
	waiters map[uint64]chan struct{}
}

func newPendingResponses() *pendingResponses {
	return &pendingResponses{waiters: make(map[uint64]chan struct{})}
}

// register creates a channel for the given request ID. MUST be called before
// the request is sent (before the response can arrive).
func (p *pendingResponses) register(id uint64) chan struct{} {
	ch := make(chan struct{}, 1)
	p.mu.Lock()
	p.waiters[id] = ch
	p.mu.Unlock()
	return ch
}

// signal delivers the response for the given request ID, if anyone is waiting.
// Returns true if a waiter was found and signaled.
func (p *pendingResponses) signal(id uint64) bool {
	p.mu.Lock()
	ch, found := p.waiters[id]
	if found {
		delete(p.waiters, id)
	}
	p.mu.Unlock()
	if found {
		close(ch)
	}
	return found
}

// wait blocks until the response arrives or the context is canceled.
func (p *pendingResponses) wait(ctx context.Context, ch chan struct{}) error {
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
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
	if _, err := fmt.Fprintln(sw.w, s); err != nil {
		slog.Warn("syncWriter: write failed", "error", err)
	}
}
