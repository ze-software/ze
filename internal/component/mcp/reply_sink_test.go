package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// VALIDATES: jsonReplySink.WriteFrame emits a single application/json body.
// PREVENTS: accidental regression where the sink forgets to set Content-Type
// or writes twice.
func TestJSONReplySink_WriteFrame(t *testing.T) {
	rec := httptest.NewRecorder()
	sink := newJSONReplySink(rec)
	if err := sink.WriteFrame([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := rec.Body.String(); got != `{"ok":true}` {
		t.Errorf("body = %q, want {\"ok\":true}", got)
	}
	if sink.IsSSE() {
		t.Errorf("IsSSE() = true, want false before upgrade")
	}
}

// VALIDATES: WriteFrame called twice on jsonReplySink returns an error.
// PREVENTS: a misbehaving handler emitting two response bodies on one POST.
func TestJSONReplySink_DoubleWriteErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	sink := newJSONReplySink(rec)
	if err := sink.WriteFrame([]byte(`{}`)); err != nil {
		t.Fatalf("first WriteFrame: %v", err)
	}
	err := sink.WriteFrame([]byte(`{}`))
	if !errors.Is(err, errJSONSinkAlreadyWritten) {
		t.Fatalf("second WriteFrame err = %v, want errJSONSinkAlreadyWritten", err)
	}
}

// VALIDATES: UpgradeToSSE writes SSE headers, returns an sseReplySink that
// emits `data: ...\n\n` frames, and forbids the original JSON write.
// PREVENTS: upgrade path silently failing (no headers, no flush, wrong
// Content-Type) and leaving the client hanging on an application/json read.
func TestJSONReplySink_UpgradeToSSE(t *testing.T) {
	rec := httptest.NewRecorder()
	sink := newJSONReplySink(rec)
	upgraded, err := sink.UpgradeToSSE()
	if err != nil {
		t.Fatalf("UpgradeToSSE: %v", err)
	}
	if !upgraded.IsSSE() {
		t.Fatalf("upgraded sink IsSSE() = false, want true")
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if got := rec.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering = %q, want no", got)
	}
	if err := upgraded.WriteFrame([]byte(`{"method":"elicitation/create"}`)); err != nil {
		t.Fatalf("sseReplySink.WriteFrame: %v", err)
	}
	const wantFrame = "data: {\"method\":\"elicitation/create\"}\n\n"
	if got := rec.Body.String(); got != wantFrame {
		t.Errorf("body = %q, want %q", got, wantFrame)
	}
	// Subsequent WriteFrame on the original jsonReplySink must fail since
	// the single-response slot is consumed.
	if err := sink.WriteFrame([]byte(`{}`)); !errors.Is(err, errJSONSinkAlreadyWritten) {
		t.Errorf("post-upgrade jsonReplySink.WriteFrame = %v, want errJSONSinkAlreadyWritten", err)
	}
}

// VALIDATES: UpgradeToSSE after a body write returns an error (headers
// already committed).
// PREVENTS: silent corruption of an in-flight response if a handler tries
// to upgrade after the JSON body already went out.
func TestJSONReplySink_UpgradeAfterWriteFails(t *testing.T) {
	rec := httptest.NewRecorder()
	sink := newJSONReplySink(rec)
	if err := sink.WriteFrame([]byte(`{}`)); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	if _, err := sink.UpgradeToSSE(); !errors.Is(err, errJSONSinkAlreadyWritten) {
		t.Errorf("UpgradeToSSE after write = %v, want errJSONSinkAlreadyWritten", err)
	}
}

// VALIDATES: sseReplySink.WriteFrame emits repeated `data: ...\n\n`
// events that parse back to the original frames.
func TestSSEReplySink_MultipleFrames(t *testing.T) {
	rec := httptest.NewRecorder()
	sink := newJSONReplySink(rec)
	sseAny, err := sink.UpgradeToSSE()
	if err != nil {
		t.Fatalf("UpgradeToSSE: %v", err)
	}
	sse, ok := sseAny.(*sseReplySink)
	if !ok {
		t.Fatalf("UpgradeToSSE returned %T, want *sseReplySink", sseAny)
	}
	if err := sse.WriteFrame([]byte(`{"n":1}`)); err != nil {
		t.Fatalf("WriteFrame 1: %v", err)
	}
	if err := sse.WriteFrame([]byte(`{"n":2}`)); err != nil {
		t.Fatalf("WriteFrame 2: %v", err)
	}
	r := bufio.NewReader(strings.NewReader(rec.Body.String()))
	if got := string(readOneSSEFrame(t, r)); got != `{"n":1}` {
		t.Errorf("frame 1 = %q, want {\"n\":1}", got)
	}
	if got := string(readOneSSEFrame(t, r)); got != `{"n":2}` {
		t.Errorf("frame 2 = %q, want {\"n\":2}", got)
	}
}

// VALIDATES: the JSON-RPC response branch in handlePOST — a POST body
// carrying {"id": X, "result": {...}} with no method routes to the
// correlation map and returns 202 Accepted. Exercises the full HTTP
// path including Mcp-Session-Id header enforcement.
// PREVENTS: the response branch silently treating an elicit reply as a
// request or notification, or leaking 404 when the id is unknown.
func TestStreamable_JSONRPCResponseBranch(t *testing.T) {
	cfg := StreamableConfig{
		Dispatch: func(cmd string) (string, error) { return "ok:" + cmd, nil },
	}
	srv, hs, cleanup := newTestStreamable(t, cfg)
	defer cleanup()

	sid := initializeSession(t, hs)
	sess, ok := srv.registry.Get(sid)
	if !ok {
		t.Fatalf("session not found after init: %s", sid)
	}
	// initializeSession runs the current (Phase 3) initialize handler
	// which does not yet parse capabilities.elicitation (that is Phase 4).
	// Flip the flag in place so RegisterElicit below does not reject.
	// Phase 4 replaces this with a proper initialize params.capabilities
	// payload, and Phase 5 adds a TestStreamable_POSTUpgradesToSSEOnElicit
	// that drives the whole thing through a real tool handler.
	sess.clientElicit = true

	// Register a pending elicit from the server-side, then deliver the
	// client response via a POST on the elicit-response branch.
	_, ch, err := sess.RegisterElicit()
	if err != nil {
		t.Fatalf("RegisterElicit: %v", err)
	}
	id := firstPendingID(t, sess)

	responseBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"action":  "accept",
			"content": map[string]any{"answer": "42"},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.URL+Endpoint, strings.NewReader(string(responseBody)))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Mcp-Session-Id", sid)
	httpReq.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	resp, err := hs.Client().Do(httpReq)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 Accepted", resp.StatusCode)
	}

	select {
	case got := <-ch:
		if got.Action != "accept" {
			t.Errorf("action = %q, want accept", got.Action)
		}
		if got.Content["answer"] != "42" {
			t.Errorf("content[answer] = %v, want 42", got.Content["answer"])
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("correlation channel did not deliver")
	}
	if sess.PendingElicitCount() != 0 {
		t.Fatalf("correlation not cleaned up; count=%d", sess.PendingElicitCount())
	}
}

// VALIDATES: an unknown-id JSON-RPC response POST returns 202 without
// failing and does not perturb session state (AC-15b).
// PREVENTS: a malicious or stale client probing ids to leak which are live.
func TestStreamable_JSONRPCResponseUnknownID(t *testing.T) {
	cfg := StreamableConfig{
		Dispatch: func(cmd string) (string, error) { return "ok", nil },
	}
	_, hs, cleanup := newTestStreamable(t, cfg)
	defer cleanup()

	sid := initializeSession(t, hs)

	body := `{"jsonrpc":"2.0","id":"bogus-id","result":{"action":"accept","content":{}}}`
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", sid)
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 Accepted", resp.StatusCode)
	}
}

// VALIDATES: POST body with empty string id is rejected with 400.
// PREVENTS: a client that omits id from a response leaking past validation.
func TestStreamable_JSONRPCResponseEmptyID(t *testing.T) {
	cfg := StreamableConfig{
		Dispatch: func(cmd string) (string, error) { return "ok", nil },
	}
	_, hs, cleanup := newTestStreamable(t, cfg)
	defer cleanup()

	sid := initializeSession(t, hs)

	// Empty string id; no method either.
	body := `{"jsonrpc":"2.0","id":"","result":{"action":"accept"}}`
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", sid)
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", resp.StatusCode)
	}
}

// VALIDATES: a JSON-RPC error response to an elicit delivers Cancel to
// the suspended handler. MCP spec does not document elicit-error
// semantics; treating it as cancel keeps the handler's fallback path
// predictable (same as user dismissing the dialog).
// PREVENTS: handlers that branch on ErrElicitDeclined / ErrElicitCanceled
// receiving ErrElicitMalformed instead when a client returns a structured
// error response.
func TestStreamable_JSONRPCResponseErrorBranch(t *testing.T) {
	cfg := StreamableConfig{
		Dispatch: func(cmd string) (string, error) { return "ok", nil },
	}
	srv, hs, cleanup := newTestStreamable(t, cfg)
	defer cleanup()

	sid := initializeSession(t, hs)
	sess, ok := srv.registry.Get(sid)
	if !ok {
		t.Fatalf("session not found after init: %s", sid)
	}
	sess.clientElicit = true

	_, ch, err := sess.RegisterElicit()
	if err != nil {
		t.Fatalf("RegisterElicit: %v", err)
	}
	id := firstPendingID(t, sess)

	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    -32603,
			"message": "user rejected the elicit via client UI",
		},
	})
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.URL+Endpoint, strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", sid)
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 Accepted", resp.StatusCode)
	}
	select {
	case got := <-ch:
		if got.Action != elicitActionCancel {
			t.Errorf("error-branch action = %q, want cancel", got.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("correlation channel did not deliver")
	}
}

// VALIDATES: an accept response with non-object content is rejected with
// 400 instead of being silently converted to nil.
// PREVENTS: a misbehaving client sending `content: "hello"` (a string)
// and the server accepting it as "empty content, Action=accept".
func TestStreamable_JSONRPCResponseContentNotObject(t *testing.T) {
	cfg := StreamableConfig{
		Dispatch: func(cmd string) (string, error) { return "ok", nil },
	}
	srv, hs, cleanup := newTestStreamable(t, cfg)
	defer cleanup()

	sid := initializeSession(t, hs)
	sess, ok := srv.registry.Get(sid)
	if !ok {
		t.Fatalf("session not found after init: %s", sid)
	}
	sess.clientElicit = true

	_, _, err := sess.RegisterElicit()
	if err != nil {
		t.Fatalf("RegisterElicit: %v", err)
	}
	id := firstPendingID(t, sess)
	// content is a string, not a map -- a client protocol violation.
	body := `{"jsonrpc":"2.0","id":"` + id + `","result":{"action":"accept","content":"not-a-map"}}`
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", sid)
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", resp.StatusCode)
	}
	// Correlation still pending -- malformed reply did not resolve it.
	if got := sess.PendingElicitCount(); got != 1 {
		t.Errorf("pending correlations = %d, want 1 (malformed reply should not consume)", got)
	}
	// Clean up the pending elicit so the session-registry teardown is not
	// racing a live correlation channel.
	sess.CancelElicit(id)
}

// firstPendingID reads one pending-correlation id from the session's map.
// Test helper for cases where the id is not otherwise exposed.
func firstPendingID(t *testing.T, s *session) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.correlations {
		return id
	}
	t.Fatalf("no pending correlations")
	return ""
}

// readOneSSEFrame parses the next `data: <payload>\n\n` event from r and
// returns the payload bytes. Used by tests that assert on SSE framing.
// Helper kept here for future Phase 5 end-to-end tests.
func readOneSSEFrame(t *testing.T, r *bufio.Reader) []byte {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	line = strings.TrimRight(line, "\r\n")
	const prefix = "data: "
	if !strings.HasPrefix(line, prefix) {
		t.Fatalf("expected %q prefix, got %q", prefix, line)
	}
	payload := line[len(prefix):]
	// consume the blank line terminator
	if _, err := r.ReadString('\n'); err != nil {
		t.Fatalf("ReadString terminator: %v", err)
	}
	return []byte(payload)
}
