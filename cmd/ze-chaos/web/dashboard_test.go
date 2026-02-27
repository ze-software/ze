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
	writePeerGridFiltered(&buf, state, "up")
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
	// Other stats must still be present.
	if !strings.Contains(html, "Msgs Sent") {
		t.Error("renderStats missing Msgs Sent stat")
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
