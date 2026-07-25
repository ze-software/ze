// VALIDATES: spec-ospf-13 AC-11 -- the generic snapshot view (shared by the IS-IS/OSPF
// adapters) dispatches the configured command, returns the engine JSON under JSON
// negotiation, renders an HTML shell embedding the snapshot + SSE subscription, and 503s
// when no dispatcher is wired.
// PREVENTS: the shared view leaking the snapshot, mis-negotiating, or panicking when the
// engine is unavailable.
package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

func testSnapshotHandlers(payload string, gotCmd *string) *snapshotHandlers {
	return &snapshotHandlers{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, command string) (*plugin.Response, error) {
			if gotCmd != nil {
				*gotCmd = command
			}
			if payload == "" {
				return plugin.NewResponse(plugin.StatusDone, nil), nil
			}
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(payload)), nil
		},
		errNoDispatch:  errors.New("dispatch unavailable"),
		unavailableMsg: "test engine unavailable",
		jsonWarnMsg:    "test view json write",
		dataID:         "test-data",
		refresh:        time.Second,
	}
}

func TestSnapshotViewJSON(t *testing.T) {
	var got string
	h := testSnapshotHandlers(`[{"k":42}]`, &got)
	v := viewSpec{command: "show test neighbor", title: "T Neighbors", streamPath: "/t/stream", eventName: "neighbors"}
	req := httptest.NewRequest("GET", "/t?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.handleView(v)(rec, req)

	if got != "show test neighbor" {
		t.Errorf("dispatched %q, want 'show test neighbor'", got)
	}
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"k":42`) {
		t.Errorf("body missing snapshot data: %q", rec.Body.String())
	}
}

func TestSnapshotViewHTML(t *testing.T) {
	h := testSnapshotHandlers(`[]`, nil)
	v := viewSpec{command: "show test", title: "T Neighbors", streamPath: "/t/stream", eventName: "neighbors"}
	req := httptest.NewRequest("GET", "/t", http.NoBody)
	rec := httptest.NewRecorder()
	h.handleView(v)(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "T Neighbors") {
		t.Errorf("HTML missing title: %q", body)
	}
	if !strings.Contains(body, "/t/stream") {
		t.Errorf("HTML missing SSE stream path: %q", body)
	}
	if !strings.Contains(body, `id="test-data"`) {
		t.Errorf("HTML missing data element id: %q", body)
	}
}

func TestSSESnapshotNewlineSafety(t *testing.T) {
	// A valid-JSON payload with an embedded newline must not split the SSE frame. The
	// unified dispatcher flattens the snapshot through json.Marshal, which compacts
	// insignificant whitespace, so the embedded newline is normalized away before the
	// snapshot reaches the SSE writer and the frame stays one intact event.
	h := testSnapshotHandlers("{\n\"k\":1}", nil)
	v := viewSpec{command: "show test", title: "T", streamPath: "/t/stream", eventName: "neighbors"}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/t/stream", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.sse(v)(rec, req)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not return after disconnect")
	}

	body := rec.Body.String()
	// test-relax: the unified typed dispatcher flattens the snapshot via json.Marshal,
	// which compacts insignificant whitespace, so the embedded newline is normalized away
	// before the payload reaches the SSE writer. The frame therefore carries the compacted
	// snapshot as a single intact `data:` line and stays one event rather than splitting.
	if !strings.Contains(body, "data: {\"k\":1}\n\n") {
		t.Errorf("compacted snapshot not emitted as a single intact SSE data frame: %q", body)
	}
	// The raw newline must never leak into the frame (which would terminate the event early).
	if strings.Contains(body, "{\n\"k\"") {
		t.Errorf("raw newline leaked into the SSE frame: %q", body)
	}
}

func TestSnapshotViewNoDispatch(t *testing.T) {
	h := &snapshotHandlers{errNoDispatch: errors.New("no dispatch"), unavailableMsg: "test engine unavailable"}
	v := viewSpec{command: "show test", title: "T", streamPath: "/t/stream", eventName: "e"}
	req := httptest.NewRequest("GET", "/t?format=json", http.NoBody)
	rec := httptest.NewRecorder()
	h.handleView(v)(rec, req)
	if rec.Code != 503 {
		t.Errorf("status = %d, want 503 when dispatch unavailable", rec.Code)
	}
}
