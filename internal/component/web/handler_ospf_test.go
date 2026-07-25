// Design: plan/learned/967-ospf-13-cli-diag-interop.md -- OSPF web view tests.
// Related: handler_ospf.go -- the neighbor/database handlers under test.
//
// VALIDATES: spec-ospf-13 AC-11 -- the OSPF web handlers dispatch the right show command,
// return the engine JSON verbatim under JSON content negotiation, render an HTML shell
// embedding the initial snapshot + SSE subscription, and the SSE stream emits a named
// event then closes on client disconnect (no goroutine leak).
// PREVENTS: a web view that 500s when the engine is unavailable, swallows the snapshot,
// or leaks an SSE goroutine after the client navigates away.

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

func fakeOSPFDispatch(payload string, gotCmd *string) CommandDispatcher {
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

func TestOSPFNeighborsJSON(t *testing.T) {
	var got string
	h := &OSPFHandlers{Dispatch: fakeOSPFDispatch(`[{"router_id":"10.0.0.2","state":"Full"}]`, &got)}
	req := httptest.NewRequest("GET", "/ospf?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleOSPFNeighbors()(rec, req)

	if got != "show ospf neighbor" {
		t.Errorf("dispatched %q, want 'show ospf neighbor'", got)
	}
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "10.0.0.2") {
		t.Errorf("body missing neighbor data: %q", rec.Body.String())
	}
}

func TestOSPFDatabaseJSON(t *testing.T) {
	var got string
	h := &OSPFHandlers{Dispatch: fakeOSPFDispatch(`[{"area":"0.0.0.0","lsas":[]}]`, &got)}
	req := httptest.NewRequest("GET", "/ospf/database?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleOSPFDatabase()(rec, req)

	if got != "show ospf database" {
		t.Errorf("dispatched %q, want 'show ospf database'", got)
	}
	if !strings.Contains(rec.Body.String(), "0.0.0.0") {
		t.Errorf("body missing LSDB data: %q", rec.Body.String())
	}
}

func TestOSPFNeighborsHTML(t *testing.T) {
	h := &OSPFHandlers{Dispatch: fakeOSPFDispatch(`[]`, nil)}
	req := httptest.NewRequest("GET", "/ospf", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleOSPFNeighbors()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "OSPF Neighbors") {
		t.Errorf("HTML missing title: %q", body)
	}
	if !strings.Contains(body, "/ospf/neighbors/stream") {
		t.Errorf("HTML missing SSE stream path: %q", body)
	}
}

func TestOSPFNoDispatch(t *testing.T) {
	h := &OSPFHandlers{}
	req := httptest.NewRequest("GET", "/ospf?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.HandleOSPFNeighbors()(rec, req)
	if rec.Code != 503 {
		t.Errorf("status = %d, want 503 when dispatch unavailable", rec.Code)
	}
}

func TestOSPFSSEEmitsAndCloses(t *testing.T) {
	h := &OSPFHandlers{Dispatch: fakeOSPFDispatch(`[{"router_id":"10.0.0.3"}]`, nil)}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/ospf/neighbors/stream", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.HandleOSPFNeighborsSSE()(rec, req)
		close(done)
	}()

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
	if !strings.Contains(body, "10.0.0.3") {
		t.Errorf("SSE body missing snapshot data: %q", body)
	}
}
