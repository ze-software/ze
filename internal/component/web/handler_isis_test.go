// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- IS-IS web view tests.
// Related: handler_isis.go -- the neighbor/database handlers under test.
//
// VALIDATES: the IS-IS web handlers dispatch the right show command, return the
// engine JSON verbatim under JSON content negotiation, render an HTML shell that
// embeds the initial snapshot and subscribes to the SSE stream, and the SSE
// stream emits a named event carrying the snapshot then closes on client
// disconnect (no goroutine leak).
// PREVENTS: a web view that 500s when the engine is unavailable, swallows the
// snapshot, or leaks an SSE goroutine after the client navigates away.

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// fakeDispatch returns a canned JSON payload for a given command and records the
// command it was asked to run.
func fakeISISDispatch(payload string, gotCmd *string) CommandDispatcher {
	return func(_ context.Context, _ plugin.CallerIdentity, command string) (*plugin.Response, error) {
		if gotCmd != nil {
			*gotCmd = command
		}
		if payload == "" {
			return plugin.NewResponse(plugin.StatusDone, nil), nil
		}
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(payload)), nil
	}
}

// TestISISNeighborsJSON: the neighbor handler dispatches `show isis neighbor`
// and returns the engine JSON verbatim for a JSON request.
func TestISISNeighborsJSON(t *testing.T) {
	var got string
	h := &ISISHandlers{Dispatch: fakeISISDispatch(`[{"system-id":"0000.0000.0002","state":"up"}]`, &got)}
	req := httptest.NewRequest("GET", "/isis?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleISISNeighbors()(rec, req)

	if got != "show isis neighbor" {
		t.Errorf("dispatched %q, want 'show isis neighbor'", got)
	}
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "0000.0000.0002") {
		t.Errorf("body missing neighbor data: %q", rec.Body.String())
	}
}

// TestISISDatabaseJSON: the database handler dispatches `show isis database`.
func TestISISDatabaseJSON(t *testing.T) {
	var got string
	h := &ISISHandlers{Dispatch: fakeISISDispatch(`[{"lsp-id":"0000.0000.0001.00-00","sequence":1}]`, &got)}
	req := httptest.NewRequest("GET", "/isis/database?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleISISDatabase()(rec, req)

	if got != "show isis database" {
		t.Errorf("dispatched %q, want 'show isis database'", got)
	}
	if !strings.Contains(rec.Body.String(), "0000.0000.0001.00-00") {
		t.Errorf("body missing LSP data: %q", rec.Body.String())
	}
}

// TestISISNeighborsHTML: an HTML request renders a page shell embedding the
// initial snapshot and pointing at the SSE stream.
func TestISISNeighborsHTML(t *testing.T) {
	h := &ISISHandlers{Dispatch: fakeISISDispatch(`[]`, nil)}
	req := httptest.NewRequest("GET", "/isis", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleISISNeighbors()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "IS-IS Neighbors") {
		t.Errorf("HTML missing title: %q", body)
	}
	if !strings.Contains(body, "/isis/neighbors/stream") {
		t.Errorf("HTML missing SSE stream path: %q", body)
	}
}

// TestISISNoDispatch: with no dispatcher wired, the handler returns 503 rather
// than panicking.
func TestISISNoDispatch(t *testing.T) {
	h := &ISISHandlers{}
	req := httptest.NewRequest("GET", "/isis?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleISISNeighbors()(rec, req)
	if rec.Code != 503 {
		t.Errorf("status = %d, want 503 when dispatch unavailable", rec.Code)
	}
}

// TestISISSSEEmitsAndCloses: the SSE stream emits a named event carrying the
// snapshot and returns when the client disconnects (ctx canceled), proving the
// loop honors ctx.Done (no goroutine leak).
func TestISISSSEEmitsAndCloses(t *testing.T) {
	h := &ISISHandlers{Dispatch: fakeISISDispatch(`[{"system-id":"0000.0000.0003"}]`, nil)}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/isis/neighbors/stream", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.HandleISISNeighborsSSE()(rec, req)
		close(done)
	}()

	// Cancel quickly; the initial push happens before the first tick.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not return after client disconnect (goroutine leak)")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: neighbors") {
		t.Errorf("SSE body missing named event: %q", body)
	}
	if !strings.Contains(body, "0000.0000.0003") {
		t.Errorf("SSE body missing snapshot data: %q", body)
	}
}
