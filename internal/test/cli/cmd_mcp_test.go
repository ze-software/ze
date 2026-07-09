package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestStreamableGETviaZeTest verifies the ze-test mcp client's GET /mcp SSE
// directives (sse-listen + sse-expect): startSSE opens the stream with
// Accept: text/event-stream, and sseExpect receives a server-initiated
// notifications/tasks/status frame off the stream.
// VALIDATES: spec-followup-subsystem AC-8 (GET-SSE client directive).
func TestStreamableGETviaZeTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		// A heartbeat comment then a server-initiated task-status notification,
		// framed exactly as streamable.go's handleGET writes it.
		if _, err := io.WriteString(w, ": heartbeat\n\n"); err != nil {
			return
		}
		if fl != nil {
			fl.Flush()
		}
		frame := `data: {"jsonrpc":"2.0","method":"notifications/tasks/status","params":{"taskId":"t1","status":"completed"}}` + "\n\n"
		if _, err := io.WriteString(w, frame); err != nil {
			return
		}
		if fl != nil {
			fl.Flush()
		}
		// Return after the frame: the stream ends (EOF), the client's reader
		// goroutine exits, and srv.Close() can reap the connection. Blocking
		// here on r.Context().Done() deadlocks Close against the client's
		// still-open GET stream (found the hard way: 10m test timeout).
	}))
	defer srv.Close()

	c := &mcpClient{addr: strings.TrimPrefix(srv.URL, "http://"), http: srv.Client()}
	if err := c.startSSE(); err != nil {
		t.Fatalf("startSSE: %v", err)
	}
	if err := c.sseExpect("notifications/tasks/status", 5*time.Second); err != nil {
		t.Fatalf("sseExpect: %v", err)
	}
}

// TestSSEExpectRequiresListen verifies sse-expect fails clearly when sse-listen
// was not run first.
// VALIDATES: AC-8 client directive ordering guard.
func TestSSEExpectRequiresListen(t *testing.T) {
	c := &mcpClient{addr: "127.0.0.1:1", http: &http.Client{}}
	if err := c.sseExpect("notifications/tasks/status", 100*time.Millisecond); err == nil {
		t.Fatalf("expected error when sse-expect runs before sse-listen")
	}
}
