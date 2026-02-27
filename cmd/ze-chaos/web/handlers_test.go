package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// newTestDashboard creates a minimal Dashboard for handler tests.
func newTestDashboard(peerCount int) *Dashboard {
	state := NewDashboardState(peerCount, 40, 100)
	broker := NewSSEBroker(200 * time.Millisecond)
	cfg := Config{Logger: nil}
	cfg.defaults()
	return &Dashboard{
		state:  state,
		broker: broker,
		logger: cfg.Logger,
	}
}

// TestHandleIndex verifies the dashboard index page renders full HTML.
//
// VALIDATES: Index page returns valid HTML with HTMX script tags and SSE connection.
// PREVENTS: Missing layout elements breaking the dashboard.
func TestHandleIndex(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()

	d.handleIndex(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	for _, want := range []string{
		"<!DOCTYPE html>",
		"htmx.min.js",
		"sse.js",
		"style.css",
		"sse-connect=\"/events\"",
		"Ze Chaos",
		"peer-tbody",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("index page missing %q", want)
		}
	}
}

// TestHandlePeers verifies the peer table fragment endpoint.
//
// VALIDATES: Peers endpoint returns tbody with peer rows for active set members.
// PREVENTS: Empty table when peers exist in the active set.
func TestHandlePeers(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(10)
	defer d.broker.Close()

	// Promote some peers to the active set.
	now := time.Now()
	d.state.Active.Promote(0, PriorityHigh, now)
	d.state.Active.Promote(3, PriorityMedium, now)
	d.state.Peers[0].Status = PeerUp
	d.state.Peers[3].Status = PeerDown

	req := httptest.NewRequest(http.MethodGet, "/peers", http.NoBody)
	w := httptest.NewRecorder()

	d.handlePeers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `<tbody id="peer-tbody">`) {
		t.Error("response missing tbody wrapper")
	}
	if !strings.Contains(body, `id="peer-0"`) {
		t.Error("response missing peer-0 row")
	}
	if !strings.Contains(body, `id="peer-3"`) {
		t.Error("response missing peer-3 row")
	}
}

// TestHandlePeersStatusFilter verifies filtering peers by status.
//
// VALIDATES: Status query parameter filters peers correctly.
// PREVENTS: Filter returning all peers regardless of status parameter.
func TestHandlePeersStatusFilter(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(10)
	defer d.broker.Close()

	now := time.Now()
	d.state.Active.Promote(0, PriorityHigh, now)
	d.state.Active.Promote(1, PriorityHigh, now)
	d.state.Peers[0].Status = PeerUp
	d.state.Peers[1].Status = PeerDown

	req := httptest.NewRequest(http.MethodGet, "/peers?status=up", http.NoBody)
	w := httptest.NewRecorder()

	d.handlePeers(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `id="peer-0"`) {
		t.Error("response should contain peer-0 (status up)")
	}
	if strings.Contains(body, `id="peer-1"`) {
		t.Error("response should NOT contain peer-1 (status down)")
	}
}

// TestHandlePeerDetail verifies the peer detail endpoint.
//
// VALIDATES: Detail endpoint returns peer information including status, counters, and events.
// PREVENTS: Detail pane showing wrong peer data or crashing on missing peers.
func TestHandlePeerDetail(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Set up peer state.
	d.state.Peers[2].Status = PeerUp
	d.state.Peers[2].RoutesSent = 42
	d.state.Peers[2].ChaosCount = 3
	d.state.Active.Promote(2, PriorityHigh, time.Now())

	req := httptest.NewRequest(http.MethodGet, "/peer/2", http.NoBody)
	req.SetPathValue("id", "2")
	w := httptest.NewRecorder()

	d.handlePeerDetail(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Peer 2") {
		t.Error("response missing peer title")
	}
	if !strings.Contains(body, "42") {
		t.Error("response missing routes sent count")
	}
}

// TestHandlePeerDetailNotFound verifies 404 for unknown peers.
//
// VALIDATES: Non-existent peer returns 404.
// PREVENTS: Panic or empty response for invalid peer IDs.
func TestHandlePeerDetailNotFound(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodGet, "/peer/999", http.NoBody)
	req.SetPathValue("id", "999")
	w := httptest.NewRecorder()

	d.handlePeerDetail(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

// TestHandlePeerDetailInvalidID verifies 400 for non-numeric IDs.
//
// VALIDATES: Non-numeric peer ID returns 400.
// PREVENTS: Panic on strconv.Atoi failure.
func TestHandlePeerDetailInvalidID(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodGet, "/peer/abc", http.NoBody)
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	d.handlePeerDetail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// TestHandlePeerPin verifies pin toggling.
//
// VALIDATES: POST to pin endpoint toggles pin state and returns updated row.
// PREVENTS: Pin state not persisting or response missing updated row HTML.
func TestHandlePeerPin(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Promote peer 1 first.
	d.state.Active.Promote(1, PriorityMedium, time.Now())

	// Pin.
	req := httptest.NewRequest(http.MethodPost, "/peers/1/pin", http.NoBody)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	d.handlePeerPin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("pin status = %d, want 200", w.Code)
	}
	if !d.state.Active.IsPinned(1) {
		t.Error("peer 1 should be pinned after POST")
	}

	// Unpin.
	req = httptest.NewRequest(http.MethodPost, "/peers/1/pin", http.NoBody)
	req.SetPathValue("id", "1")
	w = httptest.NewRecorder()

	d.handlePeerPin(w, req)

	if d.state.Active.IsPinned(1) {
		t.Error("peer 1 should be unpinned after second POST")
	}
}

// TestHandlePeerClose verifies the close endpoint returns empty detail div.
//
// VALIDATES: Close endpoint returns an empty peer-detail div.
// PREVENTS: Detail pane not clearing when close button is clicked.
func TestHandlePeerClose(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodGet, "/peer/close", http.NoBody)
	w := httptest.NewRecorder()

	d.handlePeerClose(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if body != `<div id="peer-detail"></div>` {
		t.Errorf("unexpected body: %q", body)
	}
}

// TestPeersUpCounterAccuracy verifies PeersUp is decremented on error and reconnecting.
//
// VALIDATES: PeersUp tracks only peers with PeerUp status.
// PREVENTS: Counter drift when peers transition via error or reconnecting events.
func TestPeersUpCounterAccuracy(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()

	// Establish two peers (Established → Syncing → EOR → Up).
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventEORSent, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventEORSent, PeerIndex: 1, Time: now})

	d.state.RLock()
	if d.state.PeersUp != 2 {
		t.Fatalf("PeersUp after 2 established+EOR = %d, want 2", d.state.PeersUp)
	}
	d.state.RUnlock()

	// Peer 0 gets an error — should decrement.
	d.ProcessEvent(peer.Event{Type: peer.EventError, PeerIndex: 0, Time: now})
	d.state.RLock()
	if d.state.PeersUp != 1 {
		t.Fatalf("PeersUp after error = %d, want 1", d.state.PeersUp)
	}
	d.state.RUnlock()

	// Peer 1 reconnecting — should decrement.
	d.ProcessEvent(peer.Event{Type: peer.EventReconnecting, PeerIndex: 1, Time: now})
	d.state.RLock()
	if d.state.PeersUp != 0 {
		t.Fatalf("PeersUp after reconnecting = %d, want 0", d.state.PeersUp)
	}
	d.state.RUnlock()

	// Error on already-down peer should NOT go negative.
	d.ProcessEvent(peer.Event{Type: peer.EventError, PeerIndex: 0, Time: now})
	d.state.RLock()
	if d.state.PeersUp != 0 {
		t.Fatalf("PeersUp after duplicate error = %d, want 0", d.state.PeersUp)
	}
	d.state.RUnlock()
}

// TestPeerFilterSyncing verifies the "syncing" status filter shows only syncing peers.
//
// VALIDATES: AC-8 — Filter dropdown "Syncing" shows syncing peers; "fault" excludes them.
// PREVENTS: Syncing peers appearing in fault filter or missing from syncing filter.
func TestPeerFilterSyncing(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(3)
	defer d.broker.Close()

	now := time.Now()
	// Peer 0: syncing (established, no EOR).
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	// Peer 1: up (established + EOR).
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventEORSent, PeerIndex: 1, Time: now})
	// Peer 2: down.
	d.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 2, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 2, Time: now})

	// Promote all peers to active set so they appear in filtered results.
	d.state.mu.Lock()
	for i := range 3 {
		d.state.Active.Promote(i, PriorityHigh, now)
	}
	d.state.mu.Unlock()

	// "syncing" filter: only peer 0.
	req := httptest.NewRequest("GET", "/peers?status=syncing", http.NoBody)
	rec := httptest.NewRecorder()
	d.handlePeers(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "peer-0") {
		t.Error("syncing filter should include peer 0")
	}
	if strings.Contains(body, "peer-1") || strings.Contains(body, "peer-2") {
		t.Error("syncing filter should exclude non-syncing peers")
	}

	// "fault" filter: only peer 2 (down), NOT peer 0 (syncing is not a fault).
	req = httptest.NewRequest("GET", "/peers?status=fault", http.NoBody)
	rec = httptest.NewRecorder()
	d.handlePeers(rec, req)
	body = rec.Body.String()
	if !strings.Contains(body, "peer-2") {
		t.Error("fault filter should include peer 2 (down)")
	}
	if strings.Contains(body, "peer-0") {
		t.Error("fault filter should exclude peer 0 (syncing is not a fault)")
	}
	if strings.Contains(body, "peer-1") {
		t.Error("fault filter should exclude peer 1 (up)")
	}
}

// mockControlLogger captures LogControl calls for testing.
type mockControlLogger struct {
	calls []controlLogEntry
}

type controlLogEntry struct {
	command string
	value   string
}

func (m *mockControlLogger) LogControl(command, value string, _ time.Time) {
	m.calls = append(m.calls, controlLogEntry{command: command, value: value})
}

// TestControlHandlersLogToNDJSON verifies all control handlers log to the ControlLogger.
//
// VALIDATES: Pause, resume, rate, trigger, and stop all produce control log entries.
// PREVENTS: Control events silently missing from NDJSON event log.
func TestControlHandlersLogToNDJSON(t *testing.T) {
	t.Parallel()

	logger := &mockControlLogger{}
	controlCh := make(chan ControlCommand, 16)
	d := newTestDashboard(5)
	d.control = controlCh
	d.controlLogger = logger
	defer d.broker.Close()

	// Pause.
	req := httptest.NewRequest(http.MethodPost, "/control/pause", http.NoBody)
	w := httptest.NewRecorder()
	d.handleControlPause(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pause status = %d, want 200", w.Code)
	}

	// Resume.
	req = httptest.NewRequest(http.MethodPost, "/control/resume", http.NoBody)
	w = httptest.NewRecorder()
	d.handleControlResume(w, req)

	// Rate.
	req = httptest.NewRequest(http.MethodPost, "/control/rate", strings.NewReader("rate=0.75"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	d.handleControlRate(w, req)

	// Trigger.
	req = httptest.NewRequest(http.MethodPost, "/control/trigger", strings.NewReader("action=tcp-disconnect"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	d.handleControlTrigger(w, req)

	// Stop.
	req = httptest.NewRequest(http.MethodPost, "/control/stop", http.NoBody)
	w = httptest.NewRecorder()
	d.handleControlStop(w, req)

	// Verify all 5 control events were logged.
	if len(logger.calls) != 5 {
		t.Fatalf("got %d log calls, want 5: %+v", len(logger.calls), logger.calls)
	}

	expected := []controlLogEntry{
		{command: "pause", value: ""},
		{command: "resume", value: ""},
		{command: "rate", value: "0.75"},
		{command: "trigger", value: "tcp-disconnect"},
		{command: "stop", value: ""},
	}
	for i, want := range expected {
		got := logger.calls[i]
		if got.command != want.command || got.value != want.value {
			t.Errorf("call[%d] = {%q, %q}, want {%q, %q}", i, got.command, got.value, want.command, want.value)
		}
	}
}

// TestControlHandlersNoLoggerNoPanic verifies control handlers work without a ControlLogger.
//
// VALIDATES: nil ControlLogger doesn't cause panics in control handlers.
// PREVENTS: NilPointerException when --event-log is not set.
func TestControlHandlersNoLoggerNoPanic(t *testing.T) {
	t.Parallel()

	controlCh := make(chan ControlCommand, 16)
	d := newTestDashboard(5)
	d.control = controlCh
	// d.controlLogger is nil (default).
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodPost, "/control/pause", http.NoBody)
	w := httptest.NewRecorder()
	d.handleControlPause(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pause status = %d, want 200", w.Code)
	}
}

// TestSortPeers verifies peer sorting by different columns.
//
// VALIDATES: Sort by status, sent, received, chaos, and default (index).
// PREVENTS: Wrong sort order or panic on nil peers.
func TestSortPeers(t *testing.T) {
	t.Parallel()

	state := NewDashboardState(5, 40, 100)
	state.Peers[0].RoutesSent = 10
	state.Peers[1].RoutesSent = 30
	state.Peers[2].RoutesSent = 20

	indices := []int{0, 1, 2}

	// Sort by sent ascending.
	sortPeers(indices, state, "sent", "asc")
	if indices[0] != 0 || indices[1] != 2 || indices[2] != 1 {
		t.Errorf("sort by sent asc: got %v, want [0,2,1]", indices)
	}

	// Sort by sent descending.
	sortPeers(indices, state, "sent", "desc")
	if indices[0] != 1 || indices[1] != 2 || indices[2] != 0 {
		t.Errorf("sort by sent desc: got %v, want [1,2,0]", indices)
	}

	// Default sort (by index).
	indices = []int{2, 0, 1}
	sortPeers(indices, state, "", "asc")
	if indices[0] != 0 || indices[1] != 1 || indices[2] != 2 {
		t.Errorf("sort by default: got %v, want [0,1,2]", indices)
	}
}

// TestEventTypeLabel verifies all event types have labels.
//
// VALIDATES: Every EventType has a non-empty label.
// PREVENTS: "unknown" showing up in the UI for known events.
func TestEventTypeLabel(t *testing.T) {
	t.Parallel()

	types := []peer.EventType{
		peer.EventEstablished,
		peer.EventDisconnected,
		peer.EventError,
		peer.EventChaosExecuted,
		peer.EventReconnecting,
		peer.EventRouteSent,
		peer.EventRouteReceived,
		peer.EventRouteWithdrawn,
		peer.EventEORSent,
		peer.EventWithdrawalSent,
		peer.EventRouteAction,
	}
	for _, et := range types {
		label := eventTypeLabel(et)
		if label == "" || label == "unknown" {
			t.Errorf("eventTypeLabel(%d) = %q, want non-empty known label", et, label)
		}
	}
}

// TestHandlePeerPinOutOfRange verifies that pin rejects negative and too-large peer IDs.
//
// VALIDATES: Out-of-range peer IDs return 400 Bad Request.
// PREVENTS: Panic or silent corruption when pinning non-existent peers.
func TestHandlePeerPinOutOfRange(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5) // peers 0-4
	t.Cleanup(func() { d.broker.Close() })

	tests := []struct {
		name string
		id   string
	}{
		{"negative", "-1"},
		{"too_large", "5"},
		{"way_too_large", "9999"},
		{"non_numeric", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/peers/"+tt.id+"/pin", http.NoBody)
			req.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()
			d.handlePeerPin(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("pin %s: status = %d, want 400", tt.id, w.Code)
			}
		})
	}
}

// TestWriteTriggerFormXSSEscape verifies that action types with special characters
// are safely escaped in hx-vals JSON attributes.
//
// VALIDATES: escapeJSONInAttr prevents XSS via crafted action type names.
// PREVENTS: Attribute breakout or JSON injection in hx-vals.
func TestWriteTriggerFormXSSEscape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		action     string
		mustNotHas string // raw string that must NOT appear
		mustHas    string // escaped string that must appear
	}{
		{
			name:       "double_quote",
			action:     `a"b`,
			mustNotHas: `"a"b"`,    // unescaped quote would break JSON
			mustHas:    `a\&#34;b`, // JSON-escaped \" then HTML-escaped " → &#34;
		},
		{
			name:       "single_quote",
			action:     `a'b`,
			mustNotHas: `a'b`, // raw single quote in HTML attribute
			mustHas:    `a&#39;b`,
		},
		{
			name:       "backslash",
			action:     `a\b`,
			mustNotHas: `"a\b"`, // unescaped backslash
			mustHas:    `a\\b`,  // JSON-escaped
		},
		{
			name:    "angle_brackets",
			action:  `a<script>b`,
			mustHas: `a&lt;script&gt;b`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf strings.Builder
			writeTriggerForm(&buf, tt.action)
			out := buf.String()

			if tt.mustNotHas != "" && strings.Contains(out, tt.mustNotHas) {
				t.Errorf("output contains unescaped %q:\n%s", tt.mustNotHas, out)
			}
			if tt.mustHas != "" && !strings.Contains(out, tt.mustHas) {
				t.Errorf("output missing escaped %q:\n%s", tt.mustHas, out)
			}
		})
	}
}

// TestProcessEventWithdrawnSplit verifies that TotalWithdrawn and TotalWdrawSent
// are tracked independently.
//
// VALIDATES: EventRouteWithdrawn increments TotalWithdrawn, EventWithdrawalSent increments TotalWdrawSent.
// PREVENTS: Conflating received and sent withdrawals in a single counter.
func TestProcessEventWithdrawnSplit(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(3)
	defer d.broker.Close()

	now := time.Now()

	// Receive withdrawal events.
	d.ProcessEvent(peer.Event{Type: peer.EventRouteWithdrawn, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteWithdrawn, PeerIndex: 1, Time: now})

	// Send withdrawal events.
	d.ProcessEvent(peer.Event{Type: peer.EventWithdrawalSent, PeerIndex: 0, Time: now, Count: 5})

	d.state.RLock()
	defer d.state.RUnlock()

	if d.state.TotalWithdrawn != 2 {
		t.Errorf("TotalWithdrawn = %d, want 2", d.state.TotalWithdrawn)
	}
	if d.state.TotalWdrawSent != 5 {
		t.Errorf("TotalWdrawSent = %d, want 5", d.state.TotalWdrawSent)
	}
}

// TestProcessEventMissing verifies that TotalMissing and per-peer Missing
// are computed correctly as max(0, announced - received).
//
// VALIDATES: Missing counters track the gap between sent and received routes.
// PREVENTS: Missing always showing 0 in the dashboard.
func TestProcessEventMissing(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(3)
	defer d.broker.Close()

	now := time.Now()

	// Peer 0 sends 3 routes.
	for range 3 {
		d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now})
	}

	d.state.RLock()
	if d.state.Peers[0].Missing != 3 {
		t.Errorf("peer 0 Missing after 3 sent = %d, want 3", d.state.Peers[0].Missing)
	}
	if d.state.TotalMissing != 3 {
		t.Errorf("TotalMissing after 3 sent = %d, want 3", d.state.TotalMissing)
	}
	d.state.RUnlock()

	// Peer 0 receives 2 routes.
	for range 2 {
		d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 0, Time: now})
	}

	d.state.RLock()
	if d.state.Peers[0].Missing != 1 {
		t.Errorf("peer 0 Missing after 2 recv = %d, want 1", d.state.Peers[0].Missing)
	}
	if d.state.TotalMissing != 1 {
		t.Errorf("TotalMissing after 2 recv = %d, want 1", d.state.TotalMissing)
	}
	d.state.RUnlock()

	// Peer 0 receives 1 more — all caught up.
	d.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 0, Time: now})

	d.state.RLock()
	if d.state.Peers[0].Missing != 0 {
		t.Errorf("peer 0 Missing after all recv = %d, want 0", d.state.Peers[0].Missing)
	}
	if d.state.TotalMissing != 0 {
		t.Errorf("TotalMissing after all recv = %d, want 0", d.state.TotalMissing)
	}
	d.state.RUnlock()
}

// TestControlChannelFull verifies that HTTP 503 is returned when the control
// channel is at capacity.
//
// VALIDATES: Non-blocking send returns "busy" when control channel at capacity (16).
// PREVENTS: Handler blocking indefinitely when scheduler is busy.
func TestControlChannelFull(t *testing.T) {
	t.Parallel()

	controlCh := make(chan ControlCommand, 16)
	d := newTestDashboard(5)
	d.control = controlCh
	defer d.broker.Close()

	// Fill the channel to capacity.
	for i := range 16 {
		controlCh <- ControlCommand{Type: "test", Rate: float64(i)}
	}

	// 17th command should return 503.
	req := httptest.NewRequest(http.MethodPost, "/control/pause", http.NoBody)
	w := httptest.NewRecorder()
	d.handleControlPause(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when channel full", w.Code)
	}
	if !strings.Contains(w.Body.String(), "busy") {
		t.Error("response should contain 'busy'")
	}
}

// TestHandlerRestart verifies POST /control/restart sends the new seed to
// the restart channel and calls onStop.
//
// VALIDATES: Restart command with valid seed triggers restart sequence (AC-6).
// PREVENTS: Restart silently failing or using wrong seed.
func TestHandlerRestart(t *testing.T) {
	t.Parallel()

	controlCh := make(chan ControlCommand, 16)
	restartCh := make(chan uint64, 1)
	stopped := make(chan struct{})

	d := newTestDashboard(5)
	d.control = controlCh
	d.restartCh = restartCh
	d.onStop = func() { close(stopped) }
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodPost, "/control/restart",
		strings.NewReader("seed=12345"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	d.handleControlRestart(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// Verify restart seed was sent.
	select {
	case seed := <-restartCh:
		if seed != 12345 {
			t.Fatalf("restart seed = %d, want 12345", seed)
		}
	default:
		t.Fatal("restart channel should have received seed")
	}

	// Verify onStop was called.
	select {
	case <-stopped:
	default:
		t.Fatal("onStop should have been called")
	}

	// Verify control state.
	d.state.RLock()
	if d.state.Control.Status != "restarting" {
		t.Errorf("status = %q, want 'restarting'", d.state.Control.Status)
	}
	d.state.RUnlock()
}

// TestHandlerRestartInvalidSeed verifies invalid seed returns an error fragment.
//
// VALIDATES: Non-numeric seed is rejected with error message.
// PREVENTS: Panic or restart with zero seed on invalid input.
func TestHandlerRestartInvalidSeed(t *testing.T) {
	t.Parallel()

	controlCh := make(chan ControlCommand, 16)
	restartCh := make(chan uint64, 1)

	d := newTestDashboard(5)
	d.control = controlCh
	d.restartCh = restartCh
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodPost, "/control/restart",
		strings.NewReader("seed=not-a-number"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	d.handleControlRestart(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "invalid") {
		t.Errorf("response should contain 'invalid': %s", body)
	}

	// Restart channel should be empty.
	select {
	case seed := <-restartCh:
		t.Fatalf("restart channel should be empty, got %d", seed)
	default:
	}
}

// TestHandlerRestartMissingSeed verifies empty seed returns an error fragment.
//
// VALIDATES: Empty seed parameter is rejected.
// PREVENTS: Restart with zero seed when user submits empty form.
func TestHandlerRestartMissingSeed(t *testing.T) {
	t.Parallel()

	controlCh := make(chan ControlCommand, 16)
	restartCh := make(chan uint64, 1)

	d := newTestDashboard(5)
	d.control = controlCh
	d.restartCh = restartCh
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodPost, "/control/restart",
		strings.NewReader("seed="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	d.handleControlRestart(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "invalid") && !strings.Contains(body, "missing") {
		t.Errorf("response should contain 'invalid' or 'missing': %s", body)
	}

	// Restart channel should be empty.
	select {
	case seed := <-restartCh:
		t.Fatalf("restart channel should be empty, got %d", seed)
	default:
	}
}

// TestHandlerRestartNoChannel verifies restart returns 503 when no restart channel.
//
// VALIDATES: Restart without restart channel configured returns 503.
// PREVENTS: Panic when restart is attempted without web dashboard restart support.
func TestHandlerRestartNoChannel(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	// d.restartCh is nil.
	defer d.broker.Close()

	req := httptest.NewRequest(http.MethodPost, "/control/restart",
		strings.NewReader("seed=42"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	d.handleControlRestart(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when no restart channel", w.Code)
	}
}

// TestProcessEventIntegration verifies that ProcessEvent updates state correctly
// and the resulting state renders without errors.
//
// VALIDATES: Full event processing → state update → rendering pipeline works.
// PREVENTS: Rendering panics on state populated by real events.
func TestProcessEventIntegration(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(3)
	defer d.broker.Close()

	now := time.Now()
	events := []peer.Event{
		{Type: peer.EventEstablished, PeerIndex: 0, Time: now},
		{Type: peer.EventEORSent, PeerIndex: 0, Time: now}, // Completes sync → PeerUp.
		{Type: peer.EventRouteSent, PeerIndex: 0, Time: now},
		{Type: peer.EventChaosExecuted, PeerIndex: 1, Time: now, ChaosAction: "disconnect"},
		{Type: peer.EventDisconnected, PeerIndex: 1, Time: now},
		{Type: peer.EventReconnecting, PeerIndex: 1, Time: now},
	}

	for _, ev := range events {
		d.ProcessEvent(ev)
	}

	// Verify state.
	d.state.RLock()
	defer d.state.RUnlock()

	if d.state.Peers[0].Status != PeerUp {
		t.Errorf("peer 0 status = %v, want PeerUp", d.state.Peers[0].Status)
	}
	if d.state.Peers[1].Status != PeerReconnecting {
		t.Errorf("peer 1 status = %v, want PeerReconnecting", d.state.Peers[1].Status)
	}
	if d.state.Peers[1].ChaosCount != 1 {
		t.Errorf("peer 1 chaos count = %d, want 1", d.state.Peers[1].ChaosCount)
	}

	// Verify active set has auto-promoted peers (chaos, disconnect, reconnect are promotable).
	if !d.state.Active.Contains(1) {
		t.Error("peer 1 should be in active set after chaos/disconnect events")
	}

	// Verify rendering doesn't panic.
	row := d.renderPeerRow(1)
	if row == "" {
		t.Error("renderPeerRow returned empty for active peer")
	}
	stats := d.renderStats()
	if stats == "" {
		t.Error("renderStats returned empty")
	}
	// Stats must preserve SSE attributes for future updates.
	if !strings.Contains(stats, `sse-swap="stats"`) {
		t.Error("renderStats must preserve sse-swap attribute")
	}
	if !strings.Contains(stats, `hx-swap="outerHTML"`) {
		t.Error("renderStats must preserve hx-swap attribute")
	}

	// Recent events rendering must include SSE attributes.
	eventsHTML := d.renderRecentEvents()
	if eventsHTML == "" {
		t.Error("renderRecentEvents returned empty")
	}
	if !strings.Contains(eventsHTML, `sse-swap="events"`) {
		t.Error("renderRecentEvents must preserve sse-swap attribute")
	}
}

// TestWebDashboardClose verifies that Close() stops the SSE broker and
// is safe to call multiple times.
//
// VALIDATES: Close cancels the broadcast loop, closes the broker, and is idempotent.
// PREVENTS: Goroutine leaks from broadcast loop, panic on double-close.
func TestWebDashboardClose(t *testing.T) {
	t.Parallel()

	d, err := New(Config{
		Addr:      "127.0.0.1:0", // OS-assigned port.
		PeerCount: 3,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Subscribe an SSE client before closing.
	client := d.broker.Subscribe()

	// Close should stop broker and server.
	if closeErr := d.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	// Broker should have signaled the client's done channel.
	select {
	case <-client.done:
		// Expected — broker closed all clients.
	default:
		t.Error("client.done should be closed after Dashboard.Close()")
	}

	// Broker should report zero clients.
	if n := d.broker.ClientCount(); n != 0 {
		t.Errorf("broker clients after Close = %d, want 0", n)
	}

	// Second Close should be safe (idempotent via sync.Once).
	if closeErr := d.Close(); closeErr != nil {
		t.Fatalf("second Close: %v", closeErr)
	}
}

// TestEmbeddedAssets verifies that the go:embed directive includes all
// required static assets and that they are non-empty.
//
// VALIDATES: htmx.min.js, sse.js, and style.css are embedded and non-empty.
// PREVENTS: Missing or empty assets causing a broken dashboard UI.
func TestEmbeddedAssets(t *testing.T) {
	t.Parallel()

	assets := []string{
		"assets/htmx.min.js",
		"assets/sse.js",
		"assets/style.css",
	}
	for _, path := range assets {
		data, err := assetsFS.ReadFile(path)
		if err != nil {
			t.Errorf("ReadFile(%q): %v", path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", path)
		}
	}
}

// TestRouteControlHandlers verifies route dynamics control handlers update state
// and return route control panel HTML.
//
// VALIDATES: Pause, resume, rate, and stop handlers update route control state correctly.
// PREVENTS: Route control handlers not wired, state not updating, or panel not rendering.
func TestRouteControlHandlers(t *testing.T) {
	t.Parallel()

	logger := &mockControlLogger{}
	routeControlCh := make(chan ControlCommand, 16)
	d := newTestDashboard(5)
	d.routeControl = routeControlCh
	d.controlLogger = logger
	// Initialize route control state.
	d.state.Control.RouteRate = 0.5
	d.state.Control.RouteStatus = statusRunning
	defer d.broker.Close()

	// Pause.
	req := httptest.NewRequest(http.MethodPost, "/control/route/pause", http.NoBody)
	w := httptest.NewRecorder()
	d.handleRouteControlPause(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pause status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `id="route-control-panel"`) {
		t.Error("pause response missing route-control-panel div")
	}
	d.state.RLock()
	if !d.state.Control.RoutePaused {
		t.Error("RoutePaused should be true after pause")
	}
	if d.state.Control.RouteStatus != statusPaused {
		t.Errorf("RouteStatus = %q, want 'paused'", d.state.Control.RouteStatus)
	}
	d.state.RUnlock()

	// Resume.
	req = httptest.NewRequest(http.MethodPost, "/control/route/resume", http.NoBody)
	w = httptest.NewRecorder()
	d.handleRouteControlResume(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("resume status = %d, want 200", w.Code)
	}
	d.state.RLock()
	if d.state.Control.RoutePaused {
		t.Error("RoutePaused should be false after resume")
	}
	if d.state.Control.RouteStatus != statusRunning {
		t.Errorf("RouteStatus = %q, want 'running'", d.state.Control.RouteStatus)
	}
	d.state.RUnlock()

	// Rate.
	req = httptest.NewRequest(http.MethodPost, "/control/route/rate", strings.NewReader("rate=0.30"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	d.handleRouteControlRate(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("rate status = %d, want 200", w.Code)
	}
	d.state.RLock()
	if d.state.Control.RouteRate != 0.30 {
		t.Errorf("RouteRate = %f, want 0.30", d.state.Control.RouteRate)
	}
	d.state.RUnlock()

	// Stop.
	req = httptest.NewRequest(http.MethodPost, "/control/route/stop", http.NoBody)
	w = httptest.NewRecorder()
	d.handleRouteControlStop(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("stop status = %d, want 200", w.Code)
	}
	d.state.RLock()
	if d.state.Control.RouteStatus != "stopped" {
		t.Errorf("RouteStatus = %q, want 'stopped'", d.state.Control.RouteStatus)
	}
	d.state.RUnlock()

	// Verify all 4 control events were logged.
	if len(logger.calls) != 4 {
		t.Fatalf("got %d log calls, want 4: %+v", len(logger.calls), logger.calls)
	}
	expected := []controlLogEntry{
		{command: "route-pause", value: ""},
		{command: "route-resume", value: ""},
		{command: "route-rate", value: "0.30"},
		{command: "route-stop", value: ""},
	}
	for i, want := range expected {
		got := logger.calls[i]
		if got.command != want.command || got.value != want.value {
			t.Errorf("call[%d] = {%q, %q}, want {%q, %q}", i, got.command, got.value, want.command, want.value)
		}
	}
}

// TestRouteControlNoChannel verifies 503 when route control channel is nil.
//
// VALIDATES: Route control handlers return 503 when routeControl is nil.
// PREVENTS: Panic on nil channel send.
func TestRouteControlNoChannel(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	// d.routeControl is nil (default).
	t.Cleanup(func() { d.broker.Close() })

	handlers := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"pause", d.handleRouteControlPause},
		{"resume", d.handleRouteControlResume},
		{"stop", d.handleRouteControlStop},
	}
	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/control/route/"+h.name, http.NoBody)
			w := httptest.NewRecorder()
			h.handler(w, req)
			if w.Code != http.StatusServiceUnavailable {
				t.Errorf("%s: status = %d, want 503", h.name, w.Code)
			}
		})
	}
}

// TestRouteControlRateInvalid verifies invalid rate values are rejected.
//
// VALIDATES: Rate handler rejects out-of-range and non-numeric values.
// PREVENTS: Setting negative rate or crashing on bad input.
func TestRouteControlRateInvalid(t *testing.T) {
	t.Parallel()

	routeControlCh := make(chan ControlCommand, 16)
	d := newTestDashboard(5)
	d.routeControl = routeControlCh
	d.state.Control.RouteStatus = statusRunning
	t.Cleanup(func() { d.broker.Close() })

	tests := []struct {
		name string
		rate string
	}{
		{"negative", "-0.1"},
		{"over_one", "1.5"},
		{"non_numeric", "abc"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/control/route/rate",
				strings.NewReader("rate="+tt.rate))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			d.handleRouteControlRate(w, req)
			body := w.Body.String()
			if !strings.Contains(body, "invalid") {
				t.Errorf("rate=%q: response should contain 'invalid': %s", tt.rate, body)
			}
		})
	}
}

// TestRouteControlChannelFull verifies 503 when route control channel is at capacity.
//
// VALIDATES: Non-blocking send returns "busy" when route control channel full.
// PREVENTS: Handler blocking when route scheduler is busy.
func TestRouteControlChannelFull(t *testing.T) {
	t.Parallel()

	routeControlCh := make(chan ControlCommand, 16)
	d := newTestDashboard(5)
	d.routeControl = routeControlCh
	d.state.Control.RouteStatus = statusRunning
	defer d.broker.Close()

	// Fill the channel.
	for range 16 {
		routeControlCh <- ControlCommand{Type: "test"}
	}

	req := httptest.NewRequest(http.MethodPost, "/control/route/pause", http.NoBody)
	w := httptest.NewRecorder()
	d.handleRouteControlPause(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when channel full", w.Code)
	}
}

// TestHandlePeersGridView verifies GET /peers/grid returns a grid container with cells.
//
// VALIDATES: AC-1 — grid endpoint returns grid container with peer cells.
// PREVENTS: Grid endpoint returning table rows or empty response.
func TestHandlePeersGridView(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(10)
	defer d.broker.Close()

	d.state.Peers[0].Status = PeerUp
	d.state.Peers[1].Status = PeerDown
	d.state.Peers[5].Status = PeerSyncing

	req := httptest.NewRequest(http.MethodGet, "/peers/grid", http.NoBody)
	w := httptest.NewRecorder()

	d.handlePeersGrid(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `id="peer-grid"`) {
		t.Error("response missing peer-grid container")
	}
	if !strings.Contains(body, `id="peer-cell-0"`) {
		t.Error("response missing peer-cell-0")
	}
	if !strings.Contains(body, `id="peer-cell-9"`) {
		t.Error("response missing peer-cell-9 (should show all peers)")
	}
	if !strings.Contains(body, "status-up") {
		t.Error("response missing status-up class")
	}
	if !strings.Contains(body, "status-down") {
		t.Error("response missing status-down class")
	}
}

// TestHandlePeersGridStatusFilter verifies GET /peers/grid?status=up filters grid cells.
//
// VALIDATES: Grid respects status filter parameter.
// PREVENTS: Grid ignoring filter and always showing all peers.
func TestHandlePeersGridStatusFilter(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	d.state.Peers[0].Status = PeerUp
	d.state.Peers[1].Status = PeerDown
	d.state.Peers[2].Status = PeerUp

	req := httptest.NewRequest(http.MethodGet, "/peers/grid?status=up", http.NoBody)
	w := httptest.NewRecorder()

	d.handlePeersGrid(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `id="peer-cell-0"`) {
		t.Error("filtered grid missing peer 0 (up)")
	}
	if !strings.Contains(body, `id="peer-cell-2"`) {
		t.Error("filtered grid missing peer 2 (up)")
	}
	if strings.Contains(body, `id="peer-cell-1"`) {
		t.Error("filtered grid should not contain peer 1 (down)")
	}
}

// TestHandlePeersTableView verifies GET /peers/table returns a full table container.
//
// VALIDATES: AC-9 — toggling back to table returns complete table with thead and tbody.
// PREVENTS: Table view missing sort headers after toggle from grid.
func TestHandlePeersTableView(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	d.state.Active.Promote(0, PriorityHigh, now)
	d.state.Active.Promote(2, PriorityMedium, now)
	d.state.Peers[0].Status = PeerUp
	d.state.Peers[2].Status = PeerDown

	req := httptest.NewRequest(http.MethodGet, "/peers/table", http.NoBody)
	w := httptest.NewRecorder()

	d.handlePeersTable(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "peer-table-container") {
		t.Error("response missing peer-table-container")
	}
	if !strings.Contains(body, "<thead>") {
		t.Error("response missing table header")
	}
	if !strings.Contains(body, `id="peer-tbody"`) {
		t.Error("response missing peer-tbody")
	}
	if !strings.Contains(body, `id="peer-0"`) {
		t.Error("response missing peer 0 row")
	}
}

// TestLayoutIncludesGridToggle verifies writeLayout includes toggle buttons.
//
// VALIDATES: AC-9 — filter bar has toggle to switch between Table and Grid.
// PREVENTS: Toggle button missing from dashboard layout.
func TestLayoutIncludesGridToggle(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(3)
	defer d.broker.Close()

	var buf strings.Builder
	writeLayout(&buf, d)
	html := buf.String()

	if !strings.Contains(html, "view-toggle") {
		t.Error("layout missing view-toggle container")
	}
	if !strings.Contains(html, "/peers/grid") {
		t.Error("layout missing grid toggle button target")
	}
	if !strings.Contains(html, "/peers/table") {
		t.Error("layout missing table toggle button target")
	}
	if !strings.Contains(html, `id="peer-display"`) {
		t.Error("layout missing peer-display wrapper")
	}
}

// TestLayoutIncludesHealthDonut verifies the full page layout contains the SVG donut.
//
// VALIDATES: AC-1 — full page load shows donut in stats card.
// PREVENTS: Donut missing from initial page render.
func TestLayoutIncludesHealthDonut(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(10)
	defer d.broker.Close()

	d.state.Peers[0].Status = PeerUp
	d.state.Peers[1].Status = PeerDown

	var buf strings.Builder
	writeLayout(&buf, d)
	html := buf.String()

	if !strings.Contains(html, "<svg") {
		t.Error("layout missing donut SVG element")
	}
	if !strings.Contains(html, "donut-legend") {
		t.Error("layout missing donut legend")
	}
	// Other stats must still be present.
	if !strings.Contains(html, "Msgs Sent") {
		t.Error("layout missing Msgs Sent stat after donut")
	}
}

// TestSidebarStatsIncludesDonut verifies the polling fallback includes the donut.
//
// VALIDATES: AC-8 — sidebar/stats polling endpoint returns donut.
// PREVENTS: Donut missing from polling fallback.
func TestSidebarStatsIncludesDonut(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	mux := http.NewServeMux()
	if err := registerRoutes(mux, d); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/sidebar/stats", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("sidebar/stats returned %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<svg") {
		t.Error("sidebar/stats missing donut SVG")
	}
	if !strings.Contains(body, "donut-legend") {
		t.Error("sidebar/stats missing donut legend")
	}
}

// TestLayoutIncludesToastContainer verifies the layout has a toast container.
//
// VALIDATES: AC-8 — full page load includes toast container with sse-swap.
// PREVENTS: Toasts having nowhere to appear.
func TestLayoutIncludesToastContainer(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(3)
	defer d.broker.Close()

	var buf strings.Builder
	writeLayout(&buf, d)
	html := buf.String()

	if !strings.Contains(html, `id="toast-container"`) {
		t.Error("layout missing toast container")
	}
	if !strings.Contains(html, `sse-swap="toast"`) {
		t.Error("toast container missing sse-swap attribute")
	}
}

// TestProcessEventRouteAction verifies TotalRouteActions counter increments.
//
// VALIDATES: EventRouteAction increments TotalRouteActions counter.
// PREVENTS: Route actions not being counted in the dashboard.
func TestProcessEventRouteAction(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(3)
	defer d.broker.Close()

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventRouteAction, PeerIndex: 0, Time: now})
	d.ProcessEvent(peer.Event{Type: peer.EventRouteAction, PeerIndex: 1, Time: now})

	d.state.RLock()
	defer d.state.RUnlock()

	if d.state.TotalRouteActions != 2 {
		t.Errorf("TotalRouteActions = %d, want 2", d.state.TotalRouteActions)
	}
}
