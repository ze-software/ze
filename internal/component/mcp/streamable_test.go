package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestStreamable returns a Streamable wired with a trivial dispatcher and
// an httptest.Server. Caller MUST call returned cleanup.
func newTestStreamable(t *testing.T, cfg StreamableConfig) (*Streamable, *httptest.Server, func()) {
	t.Helper()
	if cfg.Dispatch == nil {
		cfg.Dispatch = func(cmd, _, _ string) (string, error) { return "ok: " + cmd, nil }
	}
	srv, err := NewStreamable(cfg)
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	hs := httptest.NewServer(srv)
	return srv, hs, func() {
		hs.Close()
		srv.Close()
	}
}

// closeBody closes resp.Body ignoring error (test cleanup helper).
func closeBody(t *testing.T, body io.Closer) {
	t.Helper()
	if err := body.Close(); err != nil {
		t.Logf("body close: %v", err)
	}
}

type deadlineResponseRecorder struct {
	header      http.Header
	status      int
	deadlineSet bool
	deadline    time.Time
	flushed     bool
}

func newDeadlineResponseRecorder() *deadlineResponseRecorder {
	return &deadlineResponseRecorder{header: make(http.Header)}
}

func (r *deadlineResponseRecorder) Header() http.Header { return r.header }

func (r *deadlineResponseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return len(p), nil
}

func (r *deadlineResponseRecorder) WriteHeader(status int) { r.status = status }

func (r *deadlineResponseRecorder) Flush() { r.flushed = true }

func (r *deadlineResponseRecorder) SetWriteDeadline(t time.Time) error {
	r.deadlineSet = true
	r.deadline = t
	return nil
}

// initializeResult runs the initialize handshake and returns the negotiated
// session id, the full response status, and the decoded result body. The
// response body is closed inside the helper.
func initializeResult(t *testing.T, hs *httptest.Server) (sid string, status int, result map[string]any) {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer closeBody(t, resp.Body)
	status = resp.StatusCode
	sid = resp.Header.Get("Mcp-Session-Id")
	var parsed map[string]any
	if resp.StatusCode == http.StatusOK {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&parsed); decodeErr != nil {
			t.Fatalf("decode: %v", decodeErr)
		}
		if r, ok := parsed["result"].(map[string]any); ok {
			result = r
		}
	}
	return sid, status, result
}

// initializeSession returns only the negotiated session ID (fails the test on
// any non-200 response).
func initializeSession(t *testing.T, hs *httptest.Server) string {
	t.Helper()
	sid, status, _ := initializeResult(t, hs)
	if status != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200", status)
	}
	return sid
}

// initializeOrigin runs initialize with a custom Origin header, returning only
// the response status (body closed inside helper).
func initializeOrigin(t *testing.T, hs *httptest.Server, origin string) int {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	return resp.StatusCode
}

// initializeWithAuth runs initialize with an optional Authorization header and
// returns the response status.
func initializeWithAuth(t *testing.T, hs *httptest.Server, bearer string) int {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	return resp.StatusCode
}

// deleteSession sends DELETE /mcp with the given session id. Response body is
// closed inside the helper.
func deleteSession(t *testing.T, hs *httptest.Server, sid string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer closeBody(t, resp.Body)
	return resp.StatusCode
}

// postMethod sends a JSON-RPC request on an existing session and returns the
// status + parsed body (body closed inside helper).
func postMethod(t *testing.T, hs *httptest.Server, sid, protoVersion, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}
	if protoVersion != "" {
		req.Header.Set("MCP-Protocol-Version", protoVersion)
	}
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	var parsed map[string]any
	if resp.StatusCode == http.StatusOK {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&parsed); decodeErr != nil {
			t.Fatalf("decode: %v", decodeErr)
		}
	}
	return resp.StatusCode, parsed
}

func TestStreamableInitializeAssignsSessionID(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid, status, result := initializeResult(t, hs)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if sid == "" {
		t.Fatal("Mcp-Session-Id header missing on initialize response")
	}
	if !validSessionID(sid) {
		t.Fatalf("Mcp-Session-Id %q fails validity check", sid)
	}
	if result == nil {
		t.Fatal("no result in response")
	}
	if got, _ := result["protocolVersion"].(string); got != ProtocolVersion {
		t.Fatalf("protocolVersion = %q, want %q", got, ProtocolVersion)
	}
}

func TestStreamableProtocolVersionMissingAssumesLegacy(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid := initializeSession(t, hs)
	status, _ := postMethod(t, hs, sid, "", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
}

func TestStreamableProtocolVersionUnknownRejects(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid := initializeSession(t, hs)
	status, _ := postMethod(t, hs, sid, "9999-99-99", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

func TestStreamableGETOpensSSEStream(t *testing.T) {
	srv, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid := initializeSession(t, hs)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", sid)

	got, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer closeBody(t, got.Body)
	if got.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", got.StatusCode)
	}
	if ct := got.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	sess, ok := srv.registry.Get(sid)
	if !ok {
		t.Fatal("session not found in registry")
	}
	if err := sess.Send([]byte(`{"msg":"hello"}`)); err != nil {
		t.Fatalf("Send: %v", err)
	}

	reader := bufio.NewReader(got.Body)
	line, err := readWithTimeout(reader, time.Second)
	if err != nil {
		t.Fatalf("read SSE frame: %v", err)
	}
	if !strings.HasPrefix(line, "data: ") {
		t.Fatalf("SSE frame missing 'data: ' prefix: %q", line)
	}
	if !strings.Contains(line, `"hello"`) {
		t.Fatalf("SSE frame missing payload: %q", line)
	}
}

func TestStreamableGETClearsWriteDeadline(t *testing.T) {
	srv, err := NewStreamable(StreamableConfig{})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer srv.Close()

	sess, err := srv.registry.Create(ProtocolVersion, Identity{})
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, Endpoint, http.NoBody).WithContext(ctx)
	req.Header.Set("Accept", mimeEventStream)
	req.Header.Set("Mcp-Session-Id", sess.ID())
	rec := newDeadlineResponseRecorder()

	srv.handleGET(rec, req)

	if rec.status != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.status)
	}
	if !rec.deadlineSet {
		t.Fatal("SetWriteDeadline was not called")
	}
	if !rec.deadline.IsZero() {
		t.Fatalf("deadline = %v, want zero time", rec.deadline)
	}
	if !rec.flushed {
		t.Fatal("Flush was not called")
	}
}

func readWithTimeout(r *bufio.Reader, d time.Duration) (string, error) {
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := r.ReadString('\n')
		ch <- result{line: line, err: err}
	}()
	select {
	case res := <-ch:
		return res.line, res.err
	case <-time.After(d):
		return "", context.DeadlineExceeded
	}
}

func TestStreamableOriginRejection(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://friend.example.com"},
	})
	defer cleanup()

	status := initializeOrigin(t, hs, "https://evil.example.com")
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", status)
	}
}

func TestStreamableOriginAllowListAccepts(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://friend.example.com"},
	})
	defer cleanup()

	status := initializeOrigin(t, hs, "https://friend.example.com")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
}

func TestStreamableExpiredSession404(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	status, _ := postMethod(t, hs, "nonexistent-session-id-xxx", "", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
}

func TestStreamableMissingSessionIDFails(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	status, parsed := postMethod(t, hs, "", "", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (JSON-RPC error body)", status)
	}
	if _, ok := parsed["error"].(map[string]any); !ok {
		t.Fatalf("no error in response: %v", parsed)
	}
}

func TestStreamableDELETEClosesSession(t *testing.T) {
	srv, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid := initializeSession(t, hs)
	if _, ok := srv.registry.Get(sid); !ok {
		t.Fatal("session not in registry after initialize")
	}

	if status := deleteSession(t, hs, sid); status != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", status)
	}

	if _, ok := srv.registry.Get(sid); ok {
		t.Fatal("session still in registry after DELETE")
	}

	status, _ := postMethod(t, hs, sid, "", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if status != http.StatusNotFound {
		t.Fatalf("post-DELETE status = %d, want 404", status)
	}
}

func TestStreamableConcurrentSessionsIsolated(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = make(map[string]struct{})
	)
	for range 10 {
		wg.Go(func() {
			sid := initializeSession(t, hs)
			if sid == "" {
				t.Error("empty session id")
				return
			}
			mu.Lock()
			if _, dup := seen[sid]; dup {
				t.Errorf("duplicate session id %q", sid)
			}
			seen[sid] = struct{}{}
			mu.Unlock()
		})
	}
	wg.Wait()
	if len(seen) != 10 {
		t.Fatalf("unique sessions = %d, want 10", len(seen))
	}
}

func TestStreamableBearerAuthRejectsMissingToken(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{Token: "secret"})
	defer cleanup()

	if status := initializeWithAuth(t, hs, ""); status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
}

func TestStreamableBearerAuthAcceptsValidToken(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{Token: "secret"})
	defer cleanup()

	if status := initializeWithAuth(t, hs, "secret"); status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
}

func TestStreamableCanonicalOrigin(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		want      string
		expectErr bool
	}{
		{"plain https", "https://foo.com", "https://foo.com", false},
		{"https with default port", "https://foo.com:443", "https://foo.com", false},
		{"https with explicit port", "https://foo.com:8443", "https://foo.com:8443", false},
		{"http with default port", "http://foo.com:80", "http://foo.com", false},
		{"trailing slash dropped", "https://foo.com/", "https://foo.com", false},
		{"path dropped", "https://foo.com/some/path", "https://foo.com", false},
		{"uppercase scheme lowercased", "HTTPS://FOO.COM", "https://foo.com", false},
		{"null literal", "null", "null", false},
		{"NULL case-insensitive", "NULL", "null", false},
		{"missing scheme", "foo.com", "", true},
		{"empty", "", "", true},
		{"IPv6 with brackets default port", "http://[::1]:80", "http://[::1]", false},
		{"IPv6 with brackets explicit port", "http://[::1]:8080", "http://[::1]:8080", false},
		{"IPv6 loopback uppercase host", "https://[::1]", "https://[::1]", false},
		{"user-info stripped", "http://user:pass@foo.com", "http://foo.com", false},
		{"fragment stripped", "https://foo.com/path#section", "https://foo.com", false},
		{"query stripped", "https://foo.com/?q=1", "https://foo.com", false},
		{"non-numeric port rejected", "http://foo.com:abc", "", true},
		{"zero port rejected", "http://foo.com:0", "", true},
		{"too-large port rejected", "http://foo.com:99999", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := canonicalOrigin(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("canonicalOrigin(%q) error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("canonicalOrigin(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestStreamableOriginCanonicalisedBothSides(t *testing.T) {
	// Allowlist entry with default port; request with explicit default port.
	// Both canonicalize to the same key and the request is accepted.
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://friend.example.com:443"},
	})
	defer cleanup()

	if status := initializeOrigin(t, hs, "https://friend.example.com/"); status != http.StatusOK {
		t.Fatalf("canonicalised origin match: status = %d, want 200", status)
	}
}

func TestStreamableLoopbackRejectedWhenAllowListSet(t *testing.T) {
	// Allowlist is non-empty. Loopback origin NOT in allowlist must be rejected —
	// once the operator has enumerated friends, localhost is no longer free.
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://friend.example.com"},
	})
	defer cleanup()

	if status := initializeOrigin(t, hs, "http://localhost:3000"); status != http.StatusForbidden {
		t.Fatalf("loopback with explicit allowlist: status = %d, want 403", status)
	}
}

func TestStreamableLoopbackOriginAcceptedWhenAllowListEmpty(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	cases := []string{
		"http://localhost",
		"http://localhost:3000",
		"https://127.0.0.1:8080",
		"http://[::1]",
		"http://[::1]:3000",
		"https://[::1]:8443",
		"null",
	}
	for _, origin := range cases {
		t.Run(origin, func(t *testing.T) {
			if status := initializeOrigin(t, hs, origin); status != http.StatusOK {
				t.Fatalf("loopback default-allowlist %q: status = %d, want 200", origin, status)
			}
		})
	}
}

func TestStreamableNewStreamableRejectsBadOrigin(t *testing.T) {
	_, err := NewStreamable(StreamableConfig{
		Dispatch:       func(_, _, _ string) (string, error) { return "", nil },
		AllowedOrigins: []string{"not a url"},
	})
	if err == nil {
		t.Fatal("NewStreamable accepted malformed origin; want error")
	}
}

func TestStreamableGETBadAcceptReturns406(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid := initializeSession(t, hs)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Mcp-Session-Id", sid)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusNotAcceptable {
		t.Fatalf("status = %d, want 406", resp.StatusCode)
	}
}

func TestStreamableGETHeartbeatTickerTouchesLastSeen(t *testing.T) {
	// Exercise the heartbeat-ticker branch of handleGET (distinct from the
	// frame-delivery branch covered by TestStreamableGETHeartbeatTouchesLastSeen).
	srv, hs, cleanup := newTestStreamable(t, StreamableConfig{SessionTTL: minSessionTTL})
	defer cleanup()
	srv.heartbeatEvery = 40 * time.Millisecond

	sid := initializeSession(t, hs)
	sess, ok := srv.registry.Get(sid)
	if !ok {
		t.Fatal("session not found")
	}
	// Force an older baseline so refresh is observable at millisecond resolution.
	sess.mu.Lock()
	baseline := sess.lastSeenAt.Add(-time.Second)
	sess.lastSeenAt = baseline
	sess.mu.Unlock()

	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", sid)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer closeBody(t, resp.Body)

	// Read the first heartbeat frame to prove the ticker fired.
	reader := bufio.NewReader(resp.Body)
	line, err := readWithTimeout(reader, 2*time.Second)
	if err != nil {
		t.Fatalf("read heartbeat: %v", err)
	}
	if !strings.HasPrefix(line, ": heartbeat") {
		t.Fatalf("expected heartbeat comment, got %q", line)
	}
	cancel()

	sess.mu.Lock()
	seen := sess.lastSeenAt
	sess.mu.Unlock()
	if !seen.After(baseline) {
		t.Fatalf("lastSeenAt not refreshed by heartbeat: baseline=%v now=%v", baseline, seen)
	}
}

func TestStreamableTouchOnClosedSessionIsNoop(t *testing.T) {
	r := newSessionRegistry(time.Minute, 0, 0)
	defer r.Close()

	s, err := r.Create("v1", Identity{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	before := s.lastSeenAt
	s.Close()

	s.Touch(time.Now().Add(time.Hour))
	if !s.lastSeenAt.Equal(before) {
		t.Fatalf("Touch mutated lastSeenAt on closed session: before=%v after=%v", before, s.lastSeenAt)
	}
}

func TestStreamableDuplicateGETReturns409(t *testing.T) {
	// Regression for pass-4 finding 2: only one concurrent SSE stream per
	// session; a second GET with the same Mcp-Session-Id must be rejected.
	srv, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid := initializeSession(t, hs)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	first, err := http.NewRequestWithContext(ctx, http.MethodGet, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	first.Header.Set("Accept", "text/event-stream")
	first.Header.Set("Mcp-Session-Id", sid)
	firstResp, err := hs.Client().Do(first)
	if err != nil {
		t.Fatalf("first GET: %v", err)
	}
	defer closeBody(t, firstResp.Body)
	if firstResp.StatusCode != http.StatusOK {
		t.Fatalf("first GET status = %d, want 200", firstResp.StatusCode)
	}
	// Prove streamActive is set by inspecting the session directly. No sleep
	// needed: hs.Client().Do returns after response headers, which is AFTER
	// the CompareAndSwap call in handleGET.
	sess, ok := srv.registry.Get(sid)
	if !ok {
		t.Fatal("session not found")
	}
	if !sess.streamActive.Load() {
		t.Fatal("streamActive not set after first GET")
	}

	second, err := http.NewRequestWithContext(t.Context(), http.MethodGet, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	second.Header.Set("Accept", "text/event-stream")
	second.Header.Set("Mcp-Session-Id", sid)
	secondResp, err := hs.Client().Do(second)
	if err != nil {
		t.Fatalf("second GET: %v", err)
	}
	defer closeBody(t, secondResp.Body)
	if secondResp.StatusCode != http.StatusConflict {
		t.Fatalf("second GET status = %d, want 409", secondResp.StatusCode)
	}
	body, readErr := io.ReadAll(secondResp.Body)
	if readErr != nil {
		t.Fatalf("read 409 body: %v", readErr)
	}
	if strings.Contains(string(body), "mcp:") {
		t.Fatalf("409 body leaks internal prefix: %q", body)
	}
}

func TestStreamableGETStreamActiveReleasedAfterCancel(t *testing.T) {
	// Regression for pass-5 finding 2: after the first stream ends (context
	// cancel → handler returns → deferred Store(false) runs), a second GET
	// on the same session must succeed.
	srv, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid := initializeSession(t, hs)

	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", sid)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("first GET: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first GET status = %d, want 200", resp.StatusCode)
	}
	sess, ok := srv.registry.Get(sid)
	if !ok {
		t.Fatal("session not found")
	}
	if !sess.streamActive.Load() {
		t.Fatal("streamActive not set after first GET")
	}

	// Cancel the first stream. The server's r.Context() cancels, handleGET's
	// select observes ctx.Done(), returns, deferred Store(false) fires.
	cancel()

	// Poll briefly for the deferred Store(false) — the server handler runs
	// on its own goroutine.
	deadline := time.Now().Add(2 * time.Second)
	for sess.streamActive.Load() {
		if time.Now().After(deadline) {
			t.Fatal("streamActive not released after stream cancel")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Second GET on the same session must now succeed.
	req2, err := http.NewRequestWithContext(t.Context(), http.MethodGet, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req2.Header.Set("Accept", "text/event-stream")
	req2.Header.Set("Mcp-Session-Id", sid)
	resp2, err := hs.Client().Do(req2)
	if err != nil {
		t.Fatalf("second GET: %v", err)
	}
	defer closeBody(t, resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second GET after cancel: status = %d, want 200", resp2.StatusCode)
	}
}

func TestStreamableIDNOriginEndToEnd(t *testing.T) {
	// Integration counterpart to TestStreamableIDNOriginMatch: configure the
	// allowlist with the unicode form, send Origin in punycode form, expect
	// the request to be accepted.
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		AllowedOrigins: []string{"https://münchen.example.com"},
	})
	defer cleanup()

	if status := initializeOrigin(t, hs, "https://xn--mnchen-3ya.example.com"); status != http.StatusOK {
		t.Fatalf("punycode origin against unicode allowlist: status = %d, want 200", status)
	}
}

func TestStreamableNewStreamableRejectsMaxLifetimeShorterThanTTL(t *testing.T) {
	_, err := NewStreamable(StreamableConfig{
		Dispatch:           func(_, _, _ string) (string, error) { return "", nil },
		SessionTTL:         30 * time.Minute,
		MaxSessionLifetime: 10 * time.Minute,
	})
	if err == nil {
		t.Fatal("NewStreamable accepted MaxSessionLifetime < SessionTTL; want error")
	}
}

func TestStreamableSSEResponseHasAntiBufferingHeader(t *testing.T) {
	// Regression for pass-4 finding 5: X-Accel-Buffering: no must be set so
	// nginx / CDN fronts don't batch the SSE heartbeat.
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid := initializeSession(t, hs)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", sid)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer closeBody(t, resp.Body)
	if got := resp.Header.Get("X-Accel-Buffering"); got != "no" {
		t.Fatalf("X-Accel-Buffering = %q, want no", got)
	}
}

func TestStreamableIDNOriginMatch(t *testing.T) {
	// Regression for pass-4 finding 3: an allowlist entry in Unicode form
	// must match an incoming Origin in punycode (and vice versa), both
	// canonicalizing via idna.Lookup.ToASCII.
	got, err := canonicalOrigin("https://münchen.example.com")
	if err != nil {
		t.Fatalf("canonicalOrigin unicode: %v", err)
	}
	got2, err := canonicalOrigin("https://xn--mnchen-3ya.example.com")
	if err != nil {
		t.Fatalf("canonicalOrigin punycode: %v", err)
	}
	if got != got2 {
		t.Fatalf("IDN mismatch: unicode=%q punycode=%q", got, got2)
	}
	if !strings.HasPrefix(got, "https://xn--mnchen-3ya") {
		t.Fatalf("expected ASCII-compatible form, got %q", got)
	}
}

func TestStreamableHeartbeatIntervalClampedToMin(t *testing.T) {
	// Regression for pass-4 finding 6: sub-minimum overrides must not
	// saturate the scheduler.
	srv := &Streamable{}
	srv.heartbeatEvery = 1 * time.Nanosecond
	if got := srv.heartbeatInterval(); got < minHeartbeatInterval {
		t.Fatalf("heartbeatInterval = %v, want >= %v", got, minHeartbeatInterval)
	}
	srv.heartbeatEvery = 100 * time.Millisecond
	if got := srv.heartbeatInterval(); got != 100*time.Millisecond {
		t.Fatalf("heartbeatInterval = %v, want 100ms", got)
	}
	srv.heartbeatEvery = 0
	if got := srv.heartbeatInterval(); got != sessionHeartbeatWindow {
		t.Fatalf("heartbeatInterval = %v, want default %v", got, sessionHeartbeatWindow)
	}
}

func TestStreamableGETHeartbeatTouchesLastSeen(t *testing.T) {
	// Regression for /ze-review finding: an idle GET stream must refresh the
	// session's lastSeenAt so TTL sweep does not reap it.
	srv, hs, cleanup := newTestStreamable(t, StreamableConfig{SessionTTL: minSessionTTL})
	defer cleanup()

	sid := initializeSession(t, hs)
	sess, ok := srv.registry.Get(sid)
	if !ok {
		t.Fatal("session not found")
	}
	baseline := sess.lastSeenAt

	ctx, cancel := context.WithCancel(t.Context())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hs.URL+Endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", sid)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer closeBody(t, resp.Body)

	// Force a frame so Touch fires on delivery, without waiting for the
	// 20 s heartbeat ticker in CI.
	if err := sess.Send([]byte(`{"tick":1}`)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	reader := bufio.NewReader(resp.Body)
	if _, err := readWithTimeout(reader, time.Second); err != nil {
		t.Fatalf("read frame: %v", err)
	}
	cancel()

	// After the stream write, lastSeenAt must be strictly after baseline.
	sess.mu.Lock()
	seen := sess.lastSeenAt
	sess.mu.Unlock()
	if !seen.After(baseline) {
		t.Fatalf("lastSeenAt not refreshed on stream write: baseline=%v now=%v", baseline, seen)
	}
}

func TestStreamableIsLoopbackOrigin(t *testing.T) {
	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost", true},
		{"https://localhost:8080", true},
		{"http://127.0.0.1", true},
		{"http://127.0.0.1:9090", true},
		{"http://[::1]", true},
		{"http://[::1]:8080", true},
		{"https://[::1]", true},
		{"null", true},
		{"http://example.com", false},
		{"https://192.168.1.1", false},
		{"https://127.0.0.1.evil.com", false},
		{"http://[::2]", false},
	}
	for _, tc := range cases {
		t.Run(tc.origin, func(t *testing.T) {
			if got := isLoopbackOrigin(tc.origin); got != tc.want {
				t.Fatalf("isLoopbackOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

func TestStreamableAcceptsEventStream(t *testing.T) {
	cases := []struct {
		accept string
		want   bool
	}{
		{"text/event-stream", true},
		{"application/json, text/event-stream", true},
		{"*/*", true},
		{"application/json", false},
		{"", false},
		{"text/event-stream;q=1", true},
	}
	for _, tc := range cases {
		t.Run(tc.accept, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/mcp", http.NoBody)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			if got := acceptsEventStream(req); got != tc.want {
				t.Fatalf("acceptsEventStream(%q) = %v, want %v", tc.accept, got, tc.want)
			}
		})
	}
}

func TestStreamableToolsListAfterInit(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	sid := initializeSession(t, hs)
	status, parsed := postMethod(t, hs, sid, ProtocolVersion, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	result, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result: %v", parsed)
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("tools list empty: %v", result)
	}
}

func TestStreamableParseInitializeProtocolVersion(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		want      string
		expectErr bool
	}{
		{"2025-06-18", `{"protocolVersion":"2025-06-18"}`, "2025-06-18", false},
		{"2025-03-26", `{"protocolVersion":"2025-03-26"}`, "2025-03-26", false},
		{"2024-11-05", `{"protocolVersion":"2024-11-05"}`, "2024-11-05", false},
		{"unknown -> error", `{"protocolVersion":"9999-99-99"}`, "", true},
		{"empty params -> default", ``, ProtocolVersion, false},
		{"invalid json -> default", `not-json`, ProtocolVersion, false},
		{"missing field -> default", `{"capabilities":{}}`, ProtocolVersion, false},
		{"empty string -> default", `{"protocolVersion":""}`, ProtocolVersion, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw json.RawMessage
			if tc.body != "" {
				raw = json.RawMessage(tc.body)
			}
			req := &request{Params: raw}
			got, err := parseInitializeProtocolVersion(req)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.body, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseInitializeProtocolVersion(%q) error: %v", tc.body, err)
			}
			if got != tc.want {
				t.Fatalf("parseInitializeProtocolVersion(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestStreamableInitializeRejectsUnsupportedVersion(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"9999-99-99"}}`
	status, parsed := postMethod(t, hs, "", "", body)
	// parseInitialize failure lands as a JSON-RPC error body with -32602.
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 with JSON-RPC error body", status)
	}
	errObj, ok := parsed["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error in response: %v", parsed)
	}
	code, _ := errObj["code"].(float64)
	if int(code) != -32602 {
		t.Fatalf("error code = %v, want -32602", code)
	}
}

func TestStreamableInitializeCapExhaustion(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{MaxSessions: 2})
	defer cleanup()

	for i := range 2 {
		if status := initializeWithAuth(t, hs, ""); status != http.StatusOK {
			t.Fatalf("session %d: status = %d, want 200", i, status)
		}
	}
	// Third initialize must be rejected with 429.
	body := `{"jsonrpc":"2.0","id":3,"method":"initialize","params":{}}`
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
	if ra := resp.Header.Get("Retry-After"); ra == "" {
		t.Fatal("Retry-After header missing")
	}
}

// VALIDATES: capabilities.elicitation = {} in the initialize params flips
// the session's clientElicit bit on; session.Elicit will therefore proceed
// instead of returning ErrElicitUnsupported.
// PREVENTS: a regression where the server sends elicitation/create even
// though the client never declared support (spec MUST violation).
func TestStreamable_InitializeReadsClientCapabilities(t *testing.T) {
	tests := []struct {
		name         string
		capabilities map[string]any
		wantElicit   bool
	}{
		{"empty caps", map[string]any{}, false},
		{"elicitation declared", map[string]any{"elicitation": map[string]any{}}, true},
		{"other cap only", map[string]any{"tools": map[string]any{}}, false},
		{"elicitation null", map[string]any{"elicitation": nil}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, hs, cleanup := newTestStreamable(t, StreamableConfig{
				Dispatch: func(cmd, _, _ string) (string, error) { return "ok", nil },
			})
			defer cleanup()

			body, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "initialize",
				"params": map[string]any{
					"protocolVersion": ProtocolVersion,
					"capabilities":    tt.capabilities,
				},
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.URL+Endpoint, strings.NewReader(string(body)))
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := hs.Client().Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer closeBody(t, resp.Body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			sid := resp.Header.Get("Mcp-Session-Id")
			sess, ok := srv.registry.Get(sid)
			if !ok {
				t.Fatalf("session %q missing from registry", sid)
			}
			if got := sess.ClientSupportsElicit(); got != tt.wantElicit {
				t.Errorf("ClientSupportsElicit = %v, want %v", got, tt.wantElicit)
			}
		})
	}
}
