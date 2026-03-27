package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// VALIDATES: AC-12 -- dashboard parses summary JSON into header + peer data.
// PREVENTS: malformed JSON silently accepted or fields dropped.
func TestDashboardParseSnapshot(t *testing.T) {
	input := `{
  "summary": {
    "router-id": "1.2.3.4",
    "local-as": 65000,
    "uptime": "1h30m0s",
    "peers-configured": 2,
    "peers-established": 1,
    "peers": [
      {
        "address": "10.0.0.1",
        "remote-as": 65001,
        "state": "established",
        "uptime": "1h0m0s",
        "updates-received": 100,
        "updates-sent": 50,
        "keepalives-received": 200,
        "keepalives-sent": 200,
        "eor-received": 1,
        "eor-sent": 1
      },
      {
        "address": "10.0.0.2",
        "remote-as": 65002,
        "state": "active",
        "uptime": "0s",
        "updates-received": 0,
        "updates-sent": 0,
        "keepalives-received": 0,
        "keepalives-sent": 0,
        "eor-received": 0,
        "eor-sent": 0
      }
    ]
  }
}`
	snap, err := parseDashboardSnapshot(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if snap.RouterID != "1.2.3.4" {
		t.Errorf("router-id: got %q, want %q", snap.RouterID, "1.2.3.4")
	}
	if snap.LocalAS != 65000 {
		t.Errorf("local-as: got %d, want 65000", snap.LocalAS)
	}
	if snap.Uptime != "1h30m0s" {
		t.Errorf("uptime: got %q, want %q", snap.Uptime, "1h30m0s")
	}
	if snap.PeersConfigured != 2 {
		t.Errorf("peers-configured: got %d, want 2", snap.PeersConfigured)
	}
	if snap.PeersEstablished != 1 {
		t.Errorf("peers-established: got %d, want 1", snap.PeersEstablished)
	}
	if len(snap.Peers) != 2 {
		t.Fatalf("peers: got %d, want 2", len(snap.Peers))
	}

	p := snap.Peers[0]
	if p.Address != "10.0.0.1" {
		t.Errorf("peer[0].address: got %q", p.Address)
	}
	if p.RemoteAS != 65001 {
		t.Errorf("peer[0].remote-as: got %d", p.RemoteAS)
	}
	if p.State != "established" {
		t.Errorf("peer[0].state: got %q", p.State)
	}
	if p.UpdatesReceived != 100 {
		t.Errorf("peer[0].updates-received: got %d", p.UpdatesReceived)
	}
	if p.UpdatesSent != 50 {
		t.Errorf("peer[0].updates-sent: got %d", p.UpdatesSent)
	}
}

// VALIDATES: AC-13 -- rate column displays updates/sec from counter diffs.
// PREVENTS: rate computation using wrong formula or wrong time base.
func TestDashboardRateComputation(t *testing.T) {
	ds := &dashboardState{}

	// First poll: rate should be "--" (no previous data).
	now := time.Now()
	snap1 := &dashboardSnapshot{
		Peers: []dashboardPeer{
			{Address: "10.0.0.1", UpdatesReceived: 100},
			{Address: "10.0.0.2", UpdatesReceived: 200},
		},
	}
	ds.updateRates(snap1, now)

	r1 := ds.peerRate("10.0.0.1")
	if r1 != "--" {
		t.Errorf("first poll rate: got %q, want %q", r1, "--")
	}

	// Second poll: 10 updates in 2 seconds = 5.0/s.
	later := now.Add(2 * time.Second)
	snap2 := &dashboardSnapshot{
		Peers: []dashboardPeer{
			{Address: "10.0.0.1", UpdatesReceived: 110},
			{Address: "10.0.0.2", UpdatesReceived: 220},
		},
	}
	ds.updateRates(snap2, later)

	r2 := ds.peerRate("10.0.0.1")
	if r2 != "5.0/s" {
		t.Errorf("second poll rate: got %q, want %q", r2, "5.0/s")
	}
	r3 := ds.peerRate("10.0.0.2")
	if r3 != "10.0/s" {
		t.Errorf("second poll rate for peer2: got %q, want %q", r3, "10.0/s")
	}
}

// VALIDATES: AC-15 -- counter decrease shows "--", baseline reset.
// PREVENTS: negative rate displayed on peer restart.
func TestDashboardRateCounterDecrease(t *testing.T) {
	ds := &dashboardState{}

	now := time.Now()
	snap1 := &dashboardSnapshot{
		Peers: []dashboardPeer{{Address: "10.0.0.1", UpdatesReceived: 500}},
	}
	ds.updateRates(snap1, now)

	// Second poll: normal increase.
	later := now.Add(2 * time.Second)
	snap2 := &dashboardSnapshot{
		Peers: []dashboardPeer{{Address: "10.0.0.1", UpdatesReceived: 510}},
	}
	ds.updateRates(snap2, later)

	r1 := ds.peerRate("10.0.0.1")
	if r1 != "5.0/s" {
		t.Errorf("normal rate: got %q, want %q", r1, "5.0/s")
	}

	// Third poll: counter decreased (peer restarted).
	evenLater := later.Add(2 * time.Second)
	snap3 := &dashboardSnapshot{
		Peers: []dashboardPeer{{Address: "10.0.0.1", UpdatesReceived: 5}},
	}
	ds.updateRates(snap3, evenLater)

	r2 := ds.peerRate("10.0.0.1")
	if r2 != "--" {
		t.Errorf("counter decrease rate: got %q, want %q", r2, "--")
	}

	// Fourth poll: counter increasing again from new baseline.
	muchLater := evenLater.Add(2 * time.Second)
	snap4 := &dashboardSnapshot{
		Peers: []dashboardPeer{{Address: "10.0.0.1", UpdatesReceived: 15}},
	}
	ds.updateRates(snap4, muchLater)

	r3 := ds.peerRate("10.0.0.1")
	if r3 != "5.0/s" {
		t.Errorf("post-reset rate: got %q, want %q", r3, "5.0/s")
	}
}

// VALIDATES: Rate computation -- short interval skips computation.
// PREVENTS: artificial rate spikes from sub-second jitter.
func TestDashboardRateShortInterval(t *testing.T) {
	ds := &dashboardState{}

	now := time.Now()
	snap1 := &dashboardSnapshot{
		Peers: []dashboardPeer{{Address: "10.0.0.1", UpdatesReceived: 100}},
	}
	ds.updateRates(snap1, now)

	// Second poll with normal interval.
	later := now.Add(2 * time.Second)
	snap2 := &dashboardSnapshot{
		Peers: []dashboardPeer{{Address: "10.0.0.1", UpdatesReceived: 110}},
	}
	ds.updateRates(snap2, later)

	r1 := ds.peerRate("10.0.0.1")
	if r1 != "5.0/s" {
		t.Errorf("normal rate: got %q, want %q", r1, "5.0/s")
	}

	// Third poll with very short interval (< 0.5s): should keep previous rate.
	shortLater := later.Add(200 * time.Millisecond)
	snap3 := &dashboardSnapshot{
		Peers: []dashboardPeer{{Address: "10.0.0.1", UpdatesReceived: 112}},
	}
	ds.updateRates(snap3, shortLater)

	r2 := ds.peerRate("10.0.0.1")
	if r2 != "5.0/s" {
		t.Errorf("short interval rate: got %q, want %q (should keep previous)", r2, "5.0/s")
	}
}

// VALIDATES: AC-4, AC-5 -- sort column cycles and direction reverses.
// PREVENTS: wrong sort order or column cycle sequence.
func TestDashboardSortPeers(t *testing.T) {
	peers := []dashboardPeer{
		{Address: "10.0.0.3", RemoteAS: 65003, State: "established", UpdatesReceived: 50},
		{Address: "10.0.0.1", RemoteAS: 65001, State: "active", UpdatesReceived: 200},
		{Address: "10.0.0.2", RemoteAS: 65002, State: "established", UpdatesReceived: 100},
	}

	// Sort by address ascending.
	sorted := sortDashboardPeers(peers, sortColumnAddress, true)
	if sorted[0].Address != "10.0.0.1" || sorted[1].Address != "10.0.0.2" || sorted[2].Address != "10.0.0.3" {
		t.Errorf("sort by address asc: got %s, %s, %s", sorted[0].Address, sorted[1].Address, sorted[2].Address)
	}

	// Sort by address descending.
	sorted = sortDashboardPeers(peers, sortColumnAddress, false)
	if sorted[0].Address != "10.0.0.3" {
		t.Errorf("sort by address desc: first got %s, want 10.0.0.3", sorted[0].Address)
	}

	// Sort by ASN ascending.
	sorted = sortDashboardPeers(peers, sortColumnASN, true)
	if sorted[0].RemoteAS != 65001 {
		t.Errorf("sort by ASN asc: first got %d, want 65001", sorted[0].RemoteAS)
	}

	// Sort by updates-received descending.
	sorted = sortDashboardPeers(peers, sortColumnRx, false)
	if sorted[0].UpdatesReceived != 200 {
		t.Errorf("sort by Rx desc: first got %d, want 200", sorted[0].UpdatesReceived)
	}

	// Test column cycling.
	col := sortColumnAddress
	for _, expected := range []dashboardSortColumn{sortColumnASN, sortColumnState, sortColumnUptime, sortColumnRx, sortColumnTx, sortColumnRate, sortColumnAddress} {
		col = col.next()
		if col != expected {
			t.Errorf("cycle: got %d, want %d", col, expected)
		}
	}
}

// VALIDATES: AC-11 -- selection preserved by peer address across re-sort.
// PREVENTS: selection jumping to wrong peer after data refresh.
func TestDashboardSelectionPersistence(t *testing.T) {
	ds := &dashboardState{
		selectedAddr: "10.0.0.2",
	}
	peers := []dashboardPeer{
		{Address: "10.0.0.3"},
		{Address: "10.0.0.1"},
		{Address: "10.0.0.2"},
	}

	idx := ds.resolveSelectedIndex(peers)
	if idx != 2 {
		t.Errorf("selection index: got %d, want 2 (10.0.0.2 is at index 2)", idx)
	}

	// After re-sort, peer order changes.
	resorted := []dashboardPeer{
		{Address: "10.0.0.1"},
		{Address: "10.0.0.2"},
		{Address: "10.0.0.3"},
	}
	idx2 := ds.resolveSelectedIndex(resorted)
	if idx2 != 1 {
		t.Errorf("after resort index: got %d, want 1", idx2)
	}

	// If selected peer disappears, clamp to 0.
	gone := []dashboardPeer{
		{Address: "10.0.0.1"},
		{Address: "10.0.0.3"},
	}
	idx3 := ds.resolveSelectedIndex(gone)
	if idx3 != 0 {
		t.Errorf("peer gone index: got %d, want 0", idx3)
	}
}

// VALIDATES: AC-12 -- header shows AS, router-id, uptime, peers established/total.
// PREVENTS: header missing required fields.
func TestDashboardRenderHeader(t *testing.T) {
	snap := &dashboardSnapshot{
		RouterID:         "1.2.3.4",
		LocalAS:          65000,
		Uptime:           "2h30m0s",
		PeersConfigured:  3,
		PeersEstablished: 2,
	}

	header := renderDashboardHeader(snap, "", 120)
	for _, want := range []string{"65000", "1.2.3.4", "2h30m0s", "2/3"} {
		if !strings.Contains(header, want) {
			t.Errorf("header missing %q in: %s", want, header)
		}
	}

	// With error.
	errHeader := renderDashboardHeader(snap, "connection lost", 120)
	if !strings.Contains(errHeader, "connection lost") {
		t.Errorf("error header missing error message: %s", errHeader)
	}

	// Nil snapshot.
	nilHeader := renderDashboardHeader(nil, "", 120)
	if !strings.Contains(nilHeader, "waiting for data") {
		t.Errorf("nil snapshot header: got %q", nilHeader)
	}
}

// VALIDATES: AC-1 -- peer table displays all columns with correct data.
// PREVENTS: columns missing or showing wrong data.
func TestDashboardRenderPeerTable(t *testing.T) {
	peers := []dashboardPeer{
		{Address: "10.0.0.1", RemoteAS: 65001, State: "established", Uptime: "1h0m0s", UpdatesReceived: 100, UpdatesSent: 50},
		{Address: "10.0.0.2", RemoteAS: 65002, State: "active", Uptime: "0s", UpdatesReceived: 0, UpdatesSent: 0},
	}
	ds := &dashboardState{
		selectedAddr: "10.0.0.1",
		rates:        map[string]*peerRateEntry{"10.0.0.1": {rate: "5.0/s"}, "10.0.0.2": {rate: "--"}},
	}

	table := renderDashboardPeerTable(peers, ds, sortColumnAddress, true, 120, 0)
	for _, want := range []string{"10.0.0.1", "65001", "established", "1h0m0s", "100", "50", "5.0/s"} {
		if !strings.Contains(table, want) {
			t.Errorf("table missing %q", want)
		}
	}
	// Sort indicator should be present on Peer column.
	if !strings.Contains(table, "Peer") {
		t.Errorf("table missing Peer header")
	}
}

// VALIDATES: AC-14 -- zero peers shows empty table with message.
// PREVENTS: crash or blank screen on zero peers.
func TestDashboardRenderZeroPeers(t *testing.T) {
	ds := &dashboardState{rates: map[string]*peerRateEntry{}}
	table := renderDashboardPeerTable(nil, ds, sortColumnAddress, true, 120, 0)
	if !strings.Contains(table, "no peers configured") {
		t.Errorf("zero peers: got %q", table)
	}
}

// VALIDATES: AC-17 -- columns dropped at narrow terminal widths.
// PREVENTS: table overflowing narrow terminal.
func TestDashboardRenderNarrowTerminal(t *testing.T) {
	// Full width: all 7 columns visible.
	fullCols := visibleColumns(120)
	if len(fullCols) != 7 {
		t.Errorf("full width: got %d columns, want 7", len(fullCols))
	}

	// Narrow: should drop some columns.
	narrowCols := visibleColumns(50)
	if len(narrowCols) >= 7 {
		t.Errorf("narrow (50 cols): got %d columns, want fewer than 7", len(narrowCols))
	}

	// Very narrow: should keep at minimum Peer.
	tinyCols := visibleColumns(20)
	if len(tinyCols) < 1 {
		t.Fatalf("tiny width: got 0 columns")
	}
	if tinyCols[0].col.String() != "Peer" {
		t.Errorf("tiny width: first column is %q, want Peer", tinyCols[0].col.String())
	}

	// Verify Peer is always kept in narrow columns.
	hasPeer := false
	for _, c := range narrowCols {
		if c.col == sortColumnAddress {
			hasPeer = true
		}
	}
	if !hasPeer {
		t.Error("narrow columns dropped Peer")
	}

	// Footer renders without panic at narrow width.
	footer := renderDashboardFooter("2s ago", 40)
	if footer == "" {
		t.Error("footer empty at narrow width")
	}

	// Counter formatting.
	if got := formatCounter(1234567); got != "1,234,567" {
		t.Errorf("formatCounter(1234567): got %q, want %q", got, "1,234,567")
	}
	if got := formatCounter(42); got != "42" {
		t.Errorf("formatCounter(42): got %q, want %q", got, "42")
	}
}

// VALIDATES: AC-4,AC-5,AC-6,AC-7,AC-8,AC-9,AC-10 -- key handling in dashboard.
// PREVENTS: keys not working or wrong navigation behavior.
func TestDashboardKeyHandling(t *testing.T) {
	snap := &dashboardSnapshot{
		Peers: []dashboardPeer{
			{Address: "10.0.0.1"},
			{Address: "10.0.0.2"},
			{Address: "10.0.0.3"},
		},
	}
	m := NewCommandModel()
	m.dashboard = &dashboardState{
		snapshot: snap,
		sortAsc:  true,
		rates:    map[string]*peerRateEntry{},
	}

	// j moves selection down.
	m.handleDashboardKey("j")
	if m.dashboard.selectedIdx != 1 {
		t.Errorf("j: selectedIdx got %d, want 1", m.dashboard.selectedIdx)
	}

	// k moves selection up.
	m.handleDashboardKey("k")
	if m.dashboard.selectedIdx != 0 {
		t.Errorf("k: selectedIdx got %d, want 0", m.dashboard.selectedIdx)
	}

	// k at top stays at 0.
	m.handleDashboardKey("k")
	if m.dashboard.selectedIdx != 0 {
		t.Errorf("k at top: selectedIdx got %d, want 0", m.dashboard.selectedIdx)
	}

	// s cycles sort column.
	if m.dashboard.sortColumn != sortColumnAddress {
		t.Fatalf("initial sort column: got %d, want %d", m.dashboard.sortColumn, sortColumnAddress)
	}
	m.handleDashboardKey("s")
	if m.dashboard.sortColumn != sortColumnASN {
		t.Errorf("s: sort column got %d, want %d", m.dashboard.sortColumn, sortColumnASN)
	}

	// S reverses sort direction.
	m.handleDashboardKey("S")
	if m.dashboard.sortAsc {
		t.Error("S: sort should be descending")
	}

	// Enter enters detail view.
	m.handleDashboardKey("enter")
	if m.dashboard.detailAddr == "" {
		t.Error("enter: should be in detail view")
	}

	// Esc in detail view returns to table.
	m.handleDashboardKey("esc")
	if m.dashboard.detailAddr != "" {
		t.Errorf("esc in detail: detailAddr should be empty, got %q", m.dashboard.detailAddr)
	}

	// Esc in table view exits dashboard.
	m.handleDashboardKey("esc")
	if m.dashboard != nil {
		t.Error("esc in table: dashboard should be nil")
	}
}

// VALIDATES: AC-16 -- poll failure shows error, preserves last good data.
// PREVENTS: dashboard crash or blank screen on poll error.
func TestDashboardPollFailure(t *testing.T) {
	m := NewCommandModel()
	m.dashboard = &dashboardState{
		sortAsc: true,
		rates:   map[string]*peerRateEntry{},
		snapshot: &dashboardSnapshot{
			RouterID:         "1.2.3.4",
			LocalAS:          65000,
			PeersConfigured:  1,
			PeersEstablished: 1,
			Peers: []dashboardPeer{
				{Address: "10.0.0.1", State: "established"},
			},
		},
	}

	// Simulate poll failure.
	result, _ := m.handleDashboardData(dashboardDataMsg{err: fmt.Errorf("connection refused")})
	m, _ = result.(Model) //nolint:errcheck // test assertion follows

	if m.dashboard.pollError == "" {
		t.Error("poll error should be set")
	}
	if !strings.Contains(m.dashboard.pollError, "connection refused") {
		t.Errorf("poll error: got %q", m.dashboard.pollError)
	}
	// Last good data should be preserved.
	if m.dashboard.snapshot == nil {
		t.Error("snapshot should be preserved after poll failure")
	}
	if m.dashboard.snapshot.RouterID != "1.2.3.4" {
		t.Errorf("preserved snapshot router-id: got %q", m.dashboard.snapshot.RouterID)
	}
}

// VALIDATES: AC-8 -- detail view auto-refreshes on poll tick.
// PREVENTS: stale detail view data.
func TestDashboardDetailAutoRefresh(t *testing.T) {
	m := NewCommandModel()
	m.dashboard = &dashboardState{
		sortAsc:    true,
		rates:      map[string]*peerRateEntry{},
		detailAddr: "10.0.0.1",
		snapshot: &dashboardSnapshot{
			Peers: []dashboardPeer{
				{Address: "10.0.0.1", State: "established", UpdatesReceived: 100},
			},
		},
	}

	// Simulate poll with updated data while in detail view.
	newData := `{"summary":{"router-id":"1.2.3.4","local-as":65000,"uptime":"1h","peers-configured":1,"peers-established":1,"peers":[{"address":"10.0.0.1","remote-as":65001,"state":"established","uptime":"1h","updates-received":200,"updates-sent":0,"keepalives-received":0,"keepalives-sent":0,"eor-received":0,"eor-sent":0}]}}`
	result, _ := m.handleDashboardData(dashboardDataMsg{data: newData})
	m, _ = result.(Model) //nolint:errcheck // test assertion follows

	// Should still be in detail view.
	if m.dashboard.detailAddr != "10.0.0.1" {
		t.Errorf("should still be in detail view, got detailAddr=%q", m.dashboard.detailAddr)
	}
	// Data should be updated.
	if m.dashboard.snapshot.Peers[0].UpdatesReceived != 200 {
		t.Errorf("updates-received: got %d, want 200", m.dashboard.snapshot.Peers[0].UpdatesReceived)
	}
}

// VALIDATES: Detail view -- peer disappears while viewing.
// PREVENTS: stuck detail view for disconnected peer.
func TestDashboardDetailPeerDisappears(t *testing.T) {
	m := NewCommandModel()
	m.dashboard = &dashboardState{
		sortAsc:    true,
		rates:      map[string]*peerRateEntry{},
		detailAddr: "10.0.0.1",
		snapshot: &dashboardSnapshot{
			Peers: []dashboardPeer{
				{Address: "10.0.0.1", State: "established"},
			},
		},
	}

	// Poll returns data without the peer we're viewing.
	newData := `{"summary":{"router-id":"1.2.3.4","local-as":65000,"uptime":"1h","peers-configured":1,"peers-established":1,"peers":[{"address":"10.0.0.2","remote-as":65002,"state":"established","uptime":"1h","updates-received":0,"updates-sent":0,"keepalives-received":0,"keepalives-sent":0,"eor-received":0,"eor-sent":0}]}}`
	result, _ := m.handleDashboardData(dashboardDataMsg{data: newData})
	m, _ = result.(Model) //nolint:errcheck // test assertion follows

	// Should return to table view.
	if m.dashboard.detailAddr != "" {
		t.Errorf("should return to table, got detailAddr=%q", m.dashboard.detailAddr)
	}
	if m.statusMessage != "peer disconnected" {
		t.Errorf("status: got %q, want %q", m.statusMessage, "peer disconnected")
	}
}
