package web

import (
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// TestRenderPeerCell verifies renderPeerCell produces correct HTML with status color class.
//
// VALIDATES: AC-2..AC-6 — each PeerStatus maps to the correct CSS class in the grid cell.
// PREVENTS: Grid cells with wrong status coloring.
func TestRenderPeerCell(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	d.state.Peers[0].Status = PeerUp

	cell := d.renderPeerCell(0)
	if cell == "" {
		t.Fatal("renderPeerCell returned empty string")
	}
	if !strings.Contains(cell, `id="peer-cell-0"`) {
		t.Error("cell missing id attribute")
	}
	if !strings.Contains(cell, "peer-cell") {
		t.Error("cell missing peer-cell class")
	}
	if !strings.Contains(cell, "status-up") {
		t.Error("cell missing status-up class for PeerUp")
	}
}

// TestRenderPeerCellTooltip verifies grid cell tooltip contains peer info.
//
// VALIDATES: AC-7 — hover over grid cell shows peer index, status, routes, last event.
// PREVENTS: Tooltip missing critical peer information.
func TestRenderPeerCellTooltip(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	d.state.Peers[2].Status = PeerUp
	d.state.Peers[2].RoutesSent = 42
	d.state.Peers[2].RoutesRecv = 37
	d.state.Peers[2].LastEvent = peer.EventRouteSent

	cell := d.renderPeerCell(2)
	// Title attribute should contain peer index and key stats.
	if !strings.Contains(cell, "title=") {
		t.Fatal("cell missing title attribute")
	}
	if !strings.Contains(cell, "2") {
		t.Error("tooltip should contain peer index")
	}
	if !strings.Contains(cell, "42") {
		t.Error("tooltip should contain routes sent count")
	}
	if !strings.Contains(cell, "37") {
		t.Error("tooltip should contain routes recv count")
	}
}

// TestRenderPeerCellClick verifies grid cell has hx-get for peer detail.
//
// VALIDATES: AC-8 — clicking a grid cell opens the peer detail pane.
// PREVENTS: Grid cells not being clickable or targeting wrong endpoint.
func TestRenderPeerCellClick(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	cell := d.renderPeerCell(3)
	if !strings.Contains(cell, `hx-get="/peer/3"`) {
		t.Error("cell missing hx-get for peer detail")
	}
	if !strings.Contains(cell, `hx-target="#peer-detail"`) {
		t.Error("cell missing hx-target for peer detail pane")
	}
}

// TestGridCellStatusColors verifies all 5 peer statuses produce correct CSS classes.
//
// VALIDATES: AC-2 (green/up), AC-3 (red/down), AC-4 (cyan/syncing),
// AC-5 (yellow/reconnecting), AC-6 (grey/idle).
// PREVENTS: Status color mismatch between grid cells and expected design.
func TestGridCellStatusColors(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	tests := []struct {
		status PeerStatus
		css    string
	}{
		{PeerUp, "status-up"},
		{PeerDown, "status-down"},
		{PeerSyncing, "status-syncing"},
		{PeerReconnecting, "status-reconnecting"},
		{PeerIdle, "status-idle"},
	}

	for _, tt := range tests {
		d.state.Peers[0].Status = tt.status
		cell := d.renderPeerCell(0)
		if !strings.Contains(cell, tt.css) {
			t.Errorf("status %v: cell missing CSS class %q in %q", tt.status, tt.css, cell)
		}
	}
}

// TestRenderPeerCellNilPeer verifies renderPeerCell returns empty for nil peer.
//
// VALIDATES: renderPeerCell handles missing peers gracefully.
// PREVENTS: Panic on nil peer state in grid rendering.
func TestRenderPeerCellNilPeer(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	cell := d.renderPeerCell(999) // Out of range.
	if cell != "" {
		t.Errorf("renderPeerCell for missing peer should return empty, got %q", cell)
	}
}

// TestWritePeerGrid verifies writePeerGrid produces a grid container with cells.
//
// VALIDATES: AC-1 — GET /peers/grid returns grid container with peer cells.
// VALIDATES: AC-10 — 500 peers rendered as cells in a compact wrapping grid.
// PREVENTS: Grid container missing cells or structural HTML.
func TestWritePeerGrid(t *testing.T) {
	t.Parallel()

	state := NewDashboardState(10, 40, 100)
	state.Peers[0].Status = PeerUp
	state.Peers[1].Status = PeerDown
	state.Peers[2].Status = PeerSyncing

	var buf strings.Builder
	writePeerGrid(&buf, state)
	html := buf.String()

	if !strings.Contains(html, `id="peer-grid"`) {
		t.Error("grid missing id attribute")
	}
	if !strings.Contains(html, "peer-grid") {
		t.Error("grid missing peer-grid class")
	}
	// Should contain cells for all 10 peers.
	for i := range 10 {
		cellID := `id="peer-cell-` + itoa(i) + `"`
		if !strings.Contains(html, cellID) {
			t.Errorf("grid missing cell for peer %d", i)
		}
	}
	// Verify status classes are present.
	if !strings.Contains(html, "status-up") {
		t.Error("grid missing status-up class")
	}
	if !strings.Contains(html, "status-down") {
		t.Error("grid missing status-down class")
	}
}

// TestWritePeerGridLargeCount verifies grid handles 500 peers.
//
// VALIDATES: AC-10 — grid scales to 500 peers.
// PREVENTS: Performance or rendering issues with large peer counts.
func TestWritePeerGridLargeCount(t *testing.T) {
	t.Parallel()

	state := NewDashboardState(500, 40, 100)
	// Set some statuses.
	for i := range 500 {
		switch i % 5 {
		case 0:
			state.Peers[i].Status = PeerUp
		case 1:
			state.Peers[i].Status = PeerDown
		case 2:
			state.Peers[i].Status = PeerSyncing
		case 3:
			state.Peers[i].Status = PeerReconnecting
		case 4:
			state.Peers[i].Status = PeerIdle
		}
	}

	var buf strings.Builder
	writePeerGrid(&buf, state)
	html := buf.String()

	// Spot-check a few cells.
	if !strings.Contains(html, `id="peer-cell-0"`) {
		t.Error("missing first cell")
	}
	if !strings.Contains(html, `id="peer-cell-499"`) {
		t.Error("missing last cell")
	}
	// Count cells — should be exactly 500.
	idCount := strings.Count(html, `id="peer-cell-`)
	if idCount != 500 {
		t.Errorf("grid has %d cells, want 500", idCount)
	}
}

// TestWritePeerGridPolling verifies the grid has HTMX polling attributes.
//
// VALIDATES: Grid container auto-refreshes via polling.
// PREVENTS: Stale grid data after initial load.
func TestWritePeerGridPolling(t *testing.T) {
	t.Parallel()

	state := NewDashboardState(3, 40, 100)
	var buf strings.Builder
	writePeerGrid(&buf, state)
	html := buf.String()

	if !strings.Contains(html, `hx-get="/peers/grid"`) {
		t.Error("grid missing hx-get polling attribute")
	}
	if !strings.Contains(html, "hx-trigger=") {
		t.Error("grid missing hx-trigger polling attribute")
	}
	if !strings.Contains(html, `hx-swap="outerHTML"`) {
		t.Error("grid missing hx-swap attribute")
	}
}

// TestWritePeerGridStatusFilter verifies the grid respects status filter.
//
// VALIDATES: Grid can be filtered by status query param.
// PREVENTS: Grid always showing all peers regardless of filter.
func TestWritePeerGridStatusFilter(t *testing.T) {
	t.Parallel()

	state := NewDashboardState(5, 40, 100)
	state.Peers[0].Status = PeerUp
	state.Peers[1].Status = PeerDown
	state.Peers[2].Status = PeerUp
	state.Peers[3].Status = PeerSyncing
	state.Peers[4].Status = PeerIdle

	var buf strings.Builder
	writePeerGridFiltered(&buf, state, "up", "")
	html := buf.String()

	// Should contain cells for peer 0 and 2 (both up).
	if !strings.Contains(html, `id="peer-cell-0"`) {
		t.Error("filtered grid missing peer 0 (up)")
	}
	if !strings.Contains(html, `id="peer-cell-2"`) {
		t.Error("filtered grid missing peer 2 (up)")
	}
	// Should NOT contain cells for peer 1 (down) or 4 (idle).
	if strings.Contains(html, `id="peer-cell-1"`) {
		t.Error("filtered grid should not contain peer 1 (down)")
	}
	if strings.Contains(html, `id="peer-cell-4"`) {
		t.Error("filtered grid should not contain peer 4 (idle)")
	}
}

// TestRenderPeerCellLastEvent verifies tooltip shows last event type.
//
// VALIDATES: AC-7 — tooltip includes last event information.
// PREVENTS: Missing event context in grid cell hover.
func TestRenderPeerCellLastEvent(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	d.state.Peers[0].Status = PeerDown
	d.state.Peers[0].LastEvent = peer.EventDisconnected
	d.state.Peers[0].LastEventAt = time.Now()

	cell := d.renderPeerCell(0)
	if !strings.Contains(cell, "disconnected") {
		t.Error("tooltip should contain last event type")
	}
}

// TestProcessEventSetsChaosActive verifies chaos events set ChaosActive.
//
// VALIDATES: AC-1, AC-2, AC-3, AC-4 — chaos event types set ChaosActive.
// PREVENTS: Chaos pulse not triggering on chaos events.
func TestProcessEventSetsChaosActive(t *testing.T) {
	t.Parallel()

	chaosEvents := []peer.EventType{
		peer.EventChaosExecuted,
		peer.EventDisconnected,
		peer.EventError,
		peer.EventReconnecting,
	}

	for _, et := range chaosEvents {
		d := newTestDashboard(5)
		now := time.Now()
		d.ProcessEvent(peer.Event{Type: et, PeerIndex: 0, Time: now})

		d.state.RLock()
		active := d.state.Peers[0].ChaosActive
		d.state.RUnlock()
		d.broker.Close()

		if !active {
			t.Errorf("event %v should set ChaosActive", et)
		}
	}
}

// TestProcessEventNoChaosActiveForRoute verifies route events don't set ChaosActive.
//
// VALIDATES: AC-5 — EventRouteSent does not trigger pulse.
// PREVENTS: Pulse firing on every route event.
func TestProcessEventNoChaosActiveForRoute(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: time.Now()})

	d.state.RLock()
	active := d.state.Peers[0].ChaosActive
	d.state.RUnlock()

	if active {
		t.Error("EventRouteSent should not set ChaosActive")
	}
}

// TestRenderPeerCellPulseClass verifies pulse class appears when ChaosActive.
//
// VALIDATES: AC-6 — grid cell has "pulse" class during chaos.
// PREVENTS: Pulse animation not triggering visually.
func TestRenderPeerCellPulseClass(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	d.state.Peers[0].Status = PeerUp
	d.state.Peers[0].ChaosActive = true

	cell := d.renderPeerCell(0)
	if !strings.Contains(cell, "pulse") {
		t.Error("cell should have pulse class when ChaosActive")
	}
}

// TestRenderPeerCellNoPulseClass verifies no pulse when ChaosActive is false.
//
// VALIDATES: AC-7 — grid cell lacks "pulse" class when not in chaos.
// PREVENTS: Permanent pulse animation.
func TestRenderPeerCellNoPulseClass(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	d.state.Peers[0].Status = PeerUp
	d.state.Peers[0].ChaosActive = false

	cell := d.renderPeerCell(0)
	if strings.Contains(cell, "pulse") {
		t.Error("cell should not have pulse class when ChaosActive is false")
	}
}

// TestProcessEventQueuesToast verifies toast-worthy events are queued.
//
// VALIDATES: AC-1, AC-4 — disconnect and chaos events produce toasts.
// PREVENTS: Toast-worthy events being silently ignored.
func TestProcessEventQueuesToast(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 2, Time: now, ChaosAction: "tcp-disconnect"})

	d.state.mu.Lock()
	toasts := d.state.ConsumePendingToasts()
	d.state.mu.Unlock()

	if len(toasts) != 1 {
		t.Fatalf("expected 1 toast, got %d", len(toasts))
	}
	if toasts[0].Label != "chaos" {
		t.Errorf("toast label = %q, want %q", toasts[0].Label, "chaos")
	}
	if toasts[0].PeerIndex != 2 {
		t.Errorf("toast peer = %d, want 2", toasts[0].PeerIndex)
	}
	if toasts[0].Detail != "tcp-disconnect" {
		t.Errorf("toast detail = %q, want %q", toasts[0].Detail, "tcp-disconnect")
	}
}

// TestProcessEventNonToastEvent verifies non-toast events don't queue.
//
// VALIDATES: AC-10 — EventRouteSent does NOT generate a toast.
// PREVENTS: Flooding toasts with routine events.
func TestProcessEventNonToastEvent(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	now := time.Now()
	d.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now})

	d.state.mu.Lock()
	toasts := d.state.ConsumePendingToasts()
	d.state.mu.Unlock()

	if len(toasts) != 0 {
		t.Errorf("non-toast event should not queue, got %d toasts", len(toasts))
	}
}

// TestToastQueueMaxFive verifies queue is bounded at 5.
//
// VALIDATES: AC-7 — only 5 most recent toasts kept.
// PREVENTS: Unbounded toast queue growth.
func TestToastQueueMaxFive(t *testing.T) {
	t.Parallel()

	state := NewDashboardState(10, 40, 100)
	for i := range 6 {
		state.QueueToast(ToastEntry{PeerIndex: i, Label: "test"})
	}

	toasts := state.ConsumePendingToasts()
	if len(toasts) != 5 {
		t.Fatalf("expected 5 toasts, got %d", len(toasts))
	}
	// Oldest (peer 0) should be dropped.
	if toasts[0].PeerIndex != 1 {
		t.Errorf("oldest toast peer = %d, want 1 (peer 0 should be dropped)", toasts[0].PeerIndex)
	}
}

// TestRenderToast verifies toast HTML structure and content.
//
// VALIDATES: AC-11 — toast shows peer index, event type, and detail.
// PREVENTS: Malformed toast HTML.
func TestRenderToast(t *testing.T) {
	t.Parallel()

	toast := ToastEntry{PeerIndex: 3, Label: "chaos", Detail: "tcp-disconnect", CSSClass: "toast-warn", Time: time.Now()}
	html := renderToast(toast)

	if !strings.Contains(html, "toast-warn") {
		t.Error("toast missing CSS class")
	}
	if !strings.Contains(html, "p3") {
		t.Error("toast missing peer index")
	}
	if !strings.Contains(html, "chaos") {
		t.Error("toast missing label")
	}
	if !strings.Contains(html, "tcp-disconnect") {
		t.Error("toast missing detail")
	}
	if !strings.Contains(html, "beforeend:#toast-container") {
		t.Error("toast missing hx-swap-oob attribute")
	}
}

// TestRenderToastColors verifies error vs warning color classes.
//
// VALIDATES: AC-9 — disconnect/error use toast-error; chaos/reconnecting use toast-warn.
// PREVENTS: Wrong color mapping.
func TestRenderToastColors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		label string
		css   string
	}{
		{"disconnected", "toast-error"},
		{"error", "toast-error"},
		{"chaos", "toast-warn"},
		{"reconnecting", "toast-warn"},
	}
	for _, tt := range tests {
		html := renderToast(ToastEntry{Label: tt.label, CSSClass: tt.css})
		if !strings.Contains(html, tt.css) {
			t.Errorf("toast %q missing class %q", tt.label, tt.css)
		}
	}
}

// TestStatusCounts verifies StatusCounts returns correct distribution.
//
// VALIDATES: AC-2 — donut has proportional segments for each status.
// PREVENTS: Incorrect status count computation.
func TestStatusCounts(t *testing.T) {
	t.Parallel()

	state := NewDashboardState(10, 40, 100)
	state.Peers[0].Status = PeerUp
	state.Peers[1].Status = PeerUp
	state.Peers[2].Status = PeerUp
	state.Peers[3].Status = PeerUp
	state.Peers[4].Status = PeerUp
	state.Peers[5].Status = PeerUp
	state.Peers[6].Status = PeerSyncing
	state.Peers[7].Status = PeerSyncing
	state.Peers[8].Status = PeerDown
	state.Peers[9].Status = PeerIdle

	counts := state.StatusCounts()
	if counts[PeerUp] != 6 {
		t.Errorf("Up count = %d, want 6", counts[PeerUp])
	}
	if counts[PeerSyncing] != 2 {
		t.Errorf("Syncing count = %d, want 2", counts[PeerSyncing])
	}
	if counts[PeerDown] != 1 {
		t.Errorf("Down count = %d, want 1", counts[PeerDown])
	}
	if counts[PeerIdle] != 1 {
		t.Errorf("Idle count = %d, want 1", counts[PeerIdle])
	}
	if counts[PeerReconnecting] != 0 {
		t.Errorf("Reconnecting count = %d, want 0", counts[PeerReconnecting])
	}
}

// TestStatusCountsZeroPeers verifies StatusCounts handles zero peers.
//
// VALIDATES: AC-5 — zero peers produces no division-by-zero.
// PREVENTS: Panic on empty peer map.
func TestStatusCountsZeroPeers(t *testing.T) {
	t.Parallel()

	state := NewDashboardState(0, 40, 100)
	counts := state.StatusCounts()
	for s, c := range counts {
		if c != 0 {
			t.Errorf("status %d has count %d, want 0", s, c)
		}
	}
}

// TestRenderDonut verifies donut SVG has correct segments for mixed statuses.
//
// VALIDATES: AC-2 — donut has segments proportional to peer status counts.
// PREVENTS: Missing SVG structure or incorrect segment rendering.
func TestRenderDonut(t *testing.T) {
	t.Parallel()

	counts := [5]int{1, 6, 1, 0, 2} // Idle=1, Up=6, Down=1, Reconnecting=0, Syncing=2
	var buf strings.Builder
	writeDonut(&buf, counts, 10)
	html := buf.String()

	if !strings.Contains(html, "<svg") {
		t.Error("donut missing <svg> element")
	}
	if !strings.Contains(html, "donut") {
		t.Error("donut missing donut class")
	}
	if !strings.Contains(html, "<circle") {
		t.Error("donut missing <circle> elements")
	}
	// Center text should show total.
	if !strings.Contains(html, ">10<") {
		t.Error("donut center should show total peer count")
	}
}

// TestRenderDonutAllUp verifies single-status donut renders as full ring.
//
// VALIDATES: AC-3 — all peers Up produces a solid green ring.
// PREVENTS: Full-ring edge case breaking SVG math.
func TestRenderDonutAllUp(t *testing.T) {
	t.Parallel()

	counts := [5]int{0, 10, 0, 0, 0} // All Up
	var buf strings.Builder
	writeDonut(&buf, counts, 10)
	html := buf.String()

	if !strings.Contains(html, "<svg") {
		t.Fatal("donut missing SVG")
	}
	// Should have exactly one circle segment (for Up).
	circleCount := strings.Count(html, "<circle")
	// Background circle + 1 segment = 2.
	if circleCount < 2 {
		t.Errorf("all-up donut should have at least 2 circles (bg + segment), got %d", circleCount)
	}
	if !strings.Contains(html, "var(--green)") {
		t.Error("all-up donut should use green color")
	}
}

// TestRenderDonutZeroPeers verifies zero peers produces empty ring.
//
// VALIDATES: AC-5 — zero peers shows empty ring with "0" center.
// PREVENTS: Division by zero in segment calculation.
func TestRenderDonutZeroPeers(t *testing.T) {
	t.Parallel()

	counts := [5]int{0, 0, 0, 0, 0}
	var buf strings.Builder
	writeDonut(&buf, counts, 0)
	html := buf.String()

	if !strings.Contains(html, "<svg") {
		t.Fatal("donut missing SVG for zero peers")
	}
	if !strings.Contains(html, ">0<") {
		t.Error("zero-peer donut should show 0 in center")
	}
}

// TestRenderDonutLegend verifies legend shows per-status counts.
//
// VALIDATES: AC-10 — legend below donut shows per-status count labels.
// PREVENTS: Missing legend or incorrect count labels.
func TestRenderDonutLegend(t *testing.T) {
	t.Parallel()

	counts := [5]int{2, 5, 1, 1, 1}
	var buf strings.Builder
	writeDonutLegend(&buf, counts)
	html := buf.String()

	if !strings.Contains(html, "donut-legend") {
		t.Error("legend missing donut-legend class")
	}
	// Check all statuses are labeled.
	for _, label := range []string{"Up", "Down", "Syncing", "Reconn", "Idle"} {
		if !strings.Contains(html, label) {
			t.Errorf("legend missing label %q", label)
		}
	}
	// Check counts.
	if !strings.Contains(html, ">5<") {
		t.Error("legend missing Up count 5")
	}
}

// TestRenderStatsIncludesDonut verifies renderStats output contains donut SVG.
//
// VALIDATES: AC-8 — SSE stats event includes donut.
// VALIDATES: AC-9 — all other stats still present after donut.
// PREVENTS: Donut missing from SSE updates, or other stats removed.
func TestRenderStatsIncludesDonut(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(10)
	defer d.broker.Close()

	d.state.Peers[0].Status = PeerUp
	d.state.PeersUp = 1

	html := d.renderStats()
	if !strings.Contains(html, "<svg") {
		t.Error("renderStats missing donut SVG")
	}
	if !strings.Contains(html, "donut-legend") {
		t.Error("renderStats missing donut legend")
	}
	// Other stats must still be present (stat-grid uses "Msgs" label, not "Msgs Sent").
	if !strings.Contains(html, "stat-grid") {
		t.Error("renderStats missing stat-grid")
	}
	if !strings.Contains(html, ">Msgs<") {
		t.Error("renderStats missing Msgs stat in grid")
	}
	if !strings.Contains(html, "Chaos") {
		t.Error("renderStats missing Chaos stat")
	}
	// SSE attributes must be preserved.
	if !strings.Contains(html, `sse-swap="stats"`) {
		t.Error("renderStats missing sse-swap attribute")
	}
}

// TestDonutSegmentColors verifies each status maps to correct CSS color.
//
// VALIDATES: AC-7 — Up=green, Syncing=cyan, Down=red, Reconnecting=yellow, Idle=grey.
// PREVENTS: Wrong color mapping in donut segments.
func TestDonutSegmentColors(t *testing.T) {
	t.Parallel()

	// One peer of each status.
	counts := [5]int{1, 1, 1, 1, 1}
	var buf strings.Builder
	writeDonut(&buf, counts, 5)
	html := buf.String()

	colors := []string{"var(--green)", "var(--red)", "var(--yellow)", "var(--accent)", "var(--text-muted)"}
	for _, c := range colors {
		if !strings.Contains(html, c) {
			t.Errorf("donut missing color %s", c)
		}
	}
}

// --- Chaos Rate Feedback Tests ---

// TestRenderStatsIncludesChaosRate verifies renderStats output contains chaos rate span.
//
// VALIDATES: AC-1 — stats panel shows chaos event rate with EMA smoothing.
// PREVENTS: Missing chaos rate readback in SSE stats updates.
func TestRenderStatsIncludesChaosRate(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Simulate some chaos events and a throughput update.
	d.state.TotalChaos = 10
	now := time.Now()
	d.state.UpdateThroughput(now)
	d.state.UpdateThroughput(now.Add(time.Second))

	html := d.renderStats()
	if !strings.Contains(html, "/s") {
		t.Error("renderStats missing chaos rate value with /s suffix")
	}
	if !strings.Contains(html, "rate-") {
		t.Error("renderStats missing rate color class")
	}
}

// TestRenderStatsIncludesSpeedFactor verifies speed readback when enabled.
//
// VALIDATES: AC-6 — stats panel shows current speed factor when enabled.
// PREVENTS: Missing speed readback.
func TestRenderStatsIncludesSpeedFactor(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	d.state.Control.SpeedAvailable = true
	d.state.Control.SpeedFactor = 100

	html := d.renderStats()
	if !strings.Contains(html, "Speed") {
		t.Error("renderStats missing 'Speed' label")
	}
	if !strings.Contains(html, "100x") {
		t.Error("renderStats missing speed factor value '100x'")
	}
}

// TestRenderStatsNoSpeedWhenDisabled verifies no speed readback when disabled.
//
// VALIDATES: AC-7 — no speed readback when SpeedAvailable is false.
// PREVENTS: Showing speed readback when not applicable.
func TestRenderStatsNoSpeedWhenDisabled(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	d.state.Control.SpeedAvailable = false

	html := d.renderStats()
	if strings.Contains(html, "Speed") {
		t.Error("renderStats should not contain 'Speed' when disabled")
	}
}

// TestBroadcastStatsWithChaosRate verifies stats SSE fragment has rate after chaos events.
//
// VALIDATES: AC-1 — stats fragment contains colored rate value after processing events.
// PREVENTS: Rate not being updated through SSE broadcast path.
func TestBroadcastStatsWithChaosRate(t *testing.T) {
	t.Parallel()

	d := newTestDashboard(5)
	defer d.broker.Close()

	// Process chaos events.
	now := time.Now()
	for i := range 5 {
		d.ProcessEvent(peer.Event{
			Type:      peer.EventChaosExecuted,
			PeerIndex: i % 5,
			Time:      now.Add(time.Duration(i) * time.Millisecond),
		})
	}

	// Simulate throughput update (normally done in broadcastDirty).
	d.state.mu.Lock()
	d.state.UpdateThroughput(now)
	d.state.UpdateThroughput(now.Add(time.Second))
	d.state.mu.Unlock()

	d.state.RLock()
	html := d.renderStats()
	d.state.RUnlock()

	// Chaos rate should be present (as X.Y/s value next to Chaos count).
	if !strings.Contains(html, "/s") {
		t.Error("stats after chaos events missing chaos rate")
	}
	// Rate should be non-zero.
	if strings.Contains(html, "0.0/s") {
		t.Error("chaos rate should be non-zero after events")
	}
}

// --- Control Strip Tests ---

// --- Trigger Button Tests ---

// TestChaosActionIcon verifies all action types have non-empty icons.
//
// VALIDATES: AC-2 — each trigger button shows a Unicode icon.
// PREVENTS: Empty icons on buttons.
func TestChaosActionIcon(t *testing.T) {
	t.Parallel()

	for _, at := range chaosActionTypes() {
		icon := chaosActionIcon(at)
		if icon == "" {
			t.Errorf("chaosActionIcon(%q) returned empty string", at)
		}
	}
}

// TestChaosActionLabel verifies all action types have short labels.
//
// VALIDATES: AC-2 — each trigger button shows a short label.
// PREVENTS: Empty labels or raw action-type strings on buttons.
func TestChaosActionLabel(t *testing.T) {
	t.Parallel()

	for _, at := range chaosActionTypes() {
		label := chaosActionLabel(at)
		if label == "" || label == at {
			t.Errorf("chaosActionLabel(%q) = %q, want a short human label", at, label)
		}
	}
}

// TestWriteTriggerButtons verifies button grid renders all 8 action buttons.
//
// VALIDATES: AC-1 — 8 individual trigger buttons visible.
// PREVENTS: Missing buttons in the trigger grid.
func TestWriteTriggerButtons(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	h := &htmlWriter{w: &buf}
	writeTriggerButtons(h, chaosActionTypes())
	html := buf.String()

	if !strings.Contains(html, "trigger-grid") {
		t.Error("missing trigger-grid container")
	}
	// Should have 8 trigger buttons (count class="badge trigger-btn").
	count := strings.Count(html, `class="badge trigger-btn"`)
	if count != 8 {
		t.Errorf("expected 8 trigger buttons, got %d", count)
	}
	// No dropdown elements.
	if strings.Contains(html, "<select") || strings.Contains(html, "<option") {
		t.Error("trigger buttons should not contain select/option elements")
	}
}

// TestWriteTriggerButtonHTMX verifies each button has correct hx-post and target.
//
// VALIDATES: AC-4 — clicking a button fires the action immediately via hx-post.
// PREVENTS: Broken HTMX wiring on trigger buttons.
func TestWriteTriggerButtonHTMX(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	h := &htmlWriter{w: &buf}
	writeTriggerButtons(h, chaosActionTypes())
	html := buf.String()

	if !strings.Contains(html, `hx-post="/control/trigger"`) {
		t.Error("missing hx-post on trigger buttons")
	}
	for _, at := range chaosActionTypes() {
		wantVal := `"action":"` + escapeJSONInAttr(at) + `"`
		if !strings.Contains(html, wantVal) {
			t.Errorf("missing hx-vals action for %q", at)
		}
	}
	if !strings.Contains(html, `hx-target="#trigger-result"`) {
		t.Error("missing hx-target on trigger buttons")
	}
}

// TestWriteTriggerButtonTooltips verifies each button has a title with impact text.
//
// VALIDATES: AC-3 — hover tooltip shows action impact description.
// PREVENTS: Missing tooltips on trigger buttons.
func TestWriteTriggerButtonTooltips(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	h := &htmlWriter{w: &buf}
	writeTriggerButtons(h, chaosActionTypes())
	html := buf.String()

	for _, at := range chaosActionTypes() {
		impact := chaosActionImpact(at)
		if !strings.Contains(html, impact[:20]) { // Check first 20 chars of impact
			t.Errorf("missing tooltip for action %q", at)
		}
	}
}

// TestWriteTriggerButtonIcons verifies each button contains a Unicode icon.
//
// VALIDATES: AC-2 — each button shows a Unicode icon character.
// PREVENTS: Buttons without visual icons.
func TestWriteTriggerButtonIcons(t *testing.T) {
	t.Parallel()

	var buf strings.Builder
	h := &htmlWriter{w: &buf}
	writeTriggerButtons(h, chaosActionTypes())
	html := buf.String()

	if !strings.Contains(html, "trigger-icon") {
		t.Error("missing trigger-icon class")
	}
	// Each action's icon should appear.
	for _, at := range chaosActionTypes() {
		icon := chaosActionIcon(at)
		if !strings.Contains(html, icon) {
			t.Errorf("missing icon %q for action %q", icon, at)
		}
	}
}

// TestControlPanelRendersTriggerButtons verifies writeControlSidebar uses buttons, not dropdown.
//
// VALIDATES: AC-1 — control panel renders trigger buttons and peer picker.
// PREVENTS: Missing trigger grid or peer picker.
func TestControlPanelRendersTriggerButtons(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "running", Rate: 0.5}
	var buf strings.Builder
	writeControlSidebar(&buf, cs)
	html := buf.String()

	if !strings.Contains(html, "trigger-grid") {
		t.Error("sidebar missing trigger-grid")
	}
	if !strings.Contains(html, "trigger-btn") {
		t.Error("sidebar missing trigger-btn")
	}
	// Peer picker with inline input and hidden peers value.
	if !strings.Contains(html, "trigger-peers") {
		t.Error("sidebar missing peer picker container")
	}
	if !strings.Contains(html, "tp-input") {
		t.Error("sidebar missing peer picker input")
	}
	if !strings.Contains(html, `name="peers"`) {
		t.Error("sidebar missing peers hidden input")
	}
}

// TestControlPanelNoTriggerWhenStopped verifies no trigger buttons when stopped.
//
// VALIDATES: AC-5 — trigger buttons hidden when Status=stopped.
// PREVENTS: Showing trigger controls when chaos is stopped.
func TestControlPanelNoTriggerWhenStopped(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "stopped"}
	var buf strings.Builder
	writeControlSidebar(&buf, cs)
	html := buf.String()

	if strings.Contains(html, "trigger-btn") {
		t.Error("stopped sidebar should not contain trigger buttons")
	}
}

// TestWriteControlStripRunning verifies the strip shows Running status with Pause button.
//
// VALIDATES: AC-1 — strip contains status dot, Pause button, rate slider, Stop button.
// PREVENTS: Missing core controls when chaos is running.
func TestWriteControlStripRunning(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "running", Rate: 0.5}
	var buf strings.Builder
	writeControlStrip(&buf, cs)
	html := buf.String()

	for _, want := range []string{
		`id="control-strip"`,
		`class="control-strip"`,
		"Running",
		"Pause",
		"/control/pause",
		"rate-slider",
		"Stop",
		"/control/stop",
		`hx-target="#control-strip"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("running strip missing %q", want)
		}
	}
	if strings.Contains(html, "Resume") {
		t.Error("running strip should not contain Resume button")
	}
}

// TestWriteControlStripPaused verifies the strip shows Paused status with Resume button.
//
// VALIDATES: AC-3 — paused state shows Resume and hides Pause.
// PREVENTS: Incorrect button when paused.
func TestWriteControlStripPaused(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "running", Paused: true, Rate: 0.0}
	var buf strings.Builder
	writeControlStrip(&buf, cs)
	html := buf.String()

	if !strings.Contains(html, "Paused") {
		t.Error("paused strip missing 'Paused' label")
	}
	if !strings.Contains(html, "Resume") {
		t.Error("paused strip missing Resume button")
	}
	if !strings.Contains(html, "/control/resume") {
		t.Error("paused strip missing /control/resume")
	}
}

// TestWriteControlStripStopped verifies the strip shows Stopped with no controls.
//
// VALIDATES: AC-6 — stopped state shows no pause/resume/rate/stop buttons.
// PREVENTS: Showing active controls when stopped.
func TestWriteControlStripStopped(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "stopped"}
	var buf strings.Builder
	writeControlStrip(&buf, cs)
	html := buf.String()

	if !strings.Contains(html, "Stopped") {
		t.Error("stopped strip missing 'Stopped' label")
	}
	if !strings.Contains(html, "status-down") {
		t.Error("stopped strip missing status-down CSS class")
	}
	for _, absent := range []string{"Pause", "Resume", "rate-slider", "/control/stop"} {
		if strings.Contains(html, absent) {
			t.Errorf("stopped strip should not contain %q", absent)
		}
	}
}

// TestWriteControlStripRestarting verifies the strip shows Restarting... status.
//
// VALIDATES: AC-7 — restarting state shows reconnecting CSS class.
// PREVENTS: Wrong status display during restart.
func TestWriteControlStripRestarting(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "restarting"}
	var buf strings.Builder
	writeControlStrip(&buf, cs)
	html := buf.String()

	if !strings.Contains(html, "Restarting...") {
		t.Error("restarting strip missing 'Restarting...' label")
	}
	if !strings.Contains(html, "status-reconnecting") {
		t.Error("restarting strip missing status-reconnecting CSS class")
	}
}

// TestWriteControlStripWithSpeed verifies speed buttons appear when available.
//
// VALIDATES: AC-8 — speed buttons rendered inline when SpeedAvailable=true.
// PREVENTS: Missing speed controls in strip.
func TestWriteControlStripWithSpeed(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "running", Rate: 0.5, SpeedAvailable: true, SpeedFactor: 10}
	var buf strings.Builder
	writeControlStrip(&buf, cs)
	html := buf.String()

	for _, want := range []string{"1x", "10x", "100x", "1000x", "/control/speed"} {
		if !strings.Contains(html, want) {
			t.Errorf("speed strip missing %q", want)
		}
	}
	// Active button should be highlighted.
	if !strings.Contains(html, `font-weight:bold`) {
		t.Error("active speed button should be bold")
	}
}

// TestWriteControlStripNoSpeed verifies no speed section when unavailable.
//
// VALIDATES: AC-9 — no speed controls when SpeedAvailable=false.
// PREVENTS: Showing speed controls when not applicable.
func TestWriteControlStripNoSpeed(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "running", Rate: 0.5}
	var buf strings.Builder
	writeControlStrip(&buf, cs)
	html := buf.String()

	if strings.Contains(html, "/control/speed") {
		t.Error("strip should not contain speed controls when SpeedAvailable=false")
	}
}

// TestWriteControlStripWithRestart verifies restart section appears when available.
//
// VALIDATES: AC-10 — seed input and New Seed button rendered inline.
// PREVENTS: Missing restart controls.
func TestWriteControlStripWithRestart(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "running", Rate: 0.5, RestartAvailable: true}
	var buf strings.Builder
	writeControlStrip(&buf, cs)
	html := buf.String()

	for _, want := range []string{"seed", "New Seed", "/control/restart"} {
		if !strings.Contains(html, want) {
			t.Errorf("restart strip missing %q", want)
		}
	}
}

// TestWriteControlStripNoRestart verifies no restart section when unavailable.
//
// VALIDATES: AC-10 complement — no restart UI when RestartAvailable=false.
// PREVENTS: Showing restart controls when not configured.
func TestWriteControlStripNoRestart(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "running", Rate: 0.5}
	var buf strings.Builder
	writeControlStrip(&buf, cs)
	html := buf.String()

	if strings.Contains(html, "/control/restart") {
		t.Error("strip should not contain restart when RestartAvailable=false")
	}
}

// TestWriteControlSidebar verifies the sidebar renders only trigger dropdown.
//
// VALIDATES: AC-11 — trigger dropdown in sidebar, no status/pause/rate/stop/speed.
// PREVENTS: Duplicate controls in sidebar.
func TestWriteControlSidebar(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "running", Rate: 0.5, SpeedAvailable: true}
	var buf strings.Builder
	writeControlSidebar(&buf, cs)
	html := buf.String()

	if !strings.Contains(html, "Trigger") {
		t.Error("sidebar missing Trigger heading")
	}
	if !strings.Contains(html, "trigger-result") {
		t.Error("sidebar missing trigger-result div")
	}
	for _, absent := range []string{"Pause", "Resume", "rate-slider", "/control/stop", "/control/speed"} {
		if strings.Contains(html, absent) {
			t.Errorf("sidebar should not contain %q (belongs in strip)", absent)
		}
	}
}

// TestWriteControlSidebarStopped verifies sidebar is empty when stopped.
//
// VALIDATES: AC-11 — sidebar hidden when chaos is stopped.
// PREVENTS: Showing trigger form when chaos is stopped.
func TestWriteControlSidebarStopped(t *testing.T) {
	t.Parallel()

	cs := &ControlState{Status: "stopped"}
	var buf strings.Builder
	writeControlSidebar(&buf, cs)
	html := buf.String()

	if html != "" {
		t.Errorf("sidebar should be empty when stopped, got %q", html)
	}
}
