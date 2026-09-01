package cli

import (
	"slices"
	"testing"
	"time"
)

// VALIDATES: the peer column orders addresses by their numeric value, so a
// host of 9 comes before a host of 10 and IPv4 comes before IPv6.
// PREVENTS: text ordering, which puts 10.0.0.10 above 10.0.0.9 and interleaves
// the two families by their first character.
func TestDashboardSortAddressByValue(t *testing.T) {
	peers := []dashboardPeer{
		{Address: "2001:db8::1"},
		{Address: "10.0.0.10"},
		{Address: "10.0.0.9"},
		{Address: "9.0.0.1"},
	}

	got := addressesOf(sortDashboardPeers(peers, sortColumnAddress, true, noRates))
	want := []string{"9.0.0.1", "10.0.0.9", "10.0.0.10", "2001:db8::1"}
	assertOrder(t, "address ascending", got, want)

	got = addressesOf(sortDashboardPeers(peers, sortColumnAddress, false, noRates))
	want = []string{"2001:db8::1", "10.0.0.10", "10.0.0.9", "9.0.0.1"}
	assertOrder(t, "address descending", got, want)
}

// VALIDATES: an address the daemon sent in a form netip cannot read orders
// after every readable one.
// PREVENTS: an unreadable address being treated as 0.0.0.0 and taking the top
// row, where it reads as the lowest-numbered peer.
func TestDashboardSortUnreadableAddressLast(t *testing.T) {
	peers := []dashboardPeer{
		{Address: "not-an-address"},
		{Address: "10.0.0.1"},
	}

	got := addressesOf(sortDashboardPeers(peers, sortColumnAddress, true, noRates))
	assertOrder(t, "unreadable address", got, []string{"10.0.0.1", "not-an-address"})
}

// VALIDATES: the uptime column orders by the duration measured, not the text.
// PREVENTS: the minute boundary reordering the table under the cursor, because
// "1m0s" sorts before "59s" as text while it is one second longer.
func TestDashboardSortUptimeByDuration(t *testing.T) {
	peers := []dashboardPeer{
		{Address: "10.0.0.1", Uptime: "1m0s"},
		{Address: "10.0.0.2", Uptime: "59s"},
		{Address: "10.0.0.3", Uptime: "2h3m0s"},
		{Address: "10.0.0.4", Uptime: "9s"},
	}

	got := addressesOf(sortDashboardPeers(peers, sortColumnUptime, true, noRates))
	assertOrder(t, "uptime ascending", got, []string{"10.0.0.4", "10.0.0.2", "10.0.0.1", "10.0.0.3"})
}

// VALIDATES: the state column orders by how far the session has progressed.
// PREVENTS: alphabetical order, which puts "active" above "established" and
// "idle-hold" between them.
func TestDashboardSortStateByProgress(t *testing.T) {
	peers := []dashboardPeer{
		{Address: "10.0.0.1", State: "established"},
		{Address: "10.0.0.2", State: "active"},
		{Address: "10.0.0.3", State: "idle-hold"},
		{Address: "10.0.0.4", State: "connecting"},
	}

	got := addressesOf(sortDashboardPeers(peers, sortColumnState, true, noRates))
	assertOrder(t, "state ascending", got, []string{"10.0.0.3", "10.0.0.4", "10.0.0.2", "10.0.0.1"})
}

// VALIDATES: rows whose sorted column holds the same value come out in address
// order, in both directions, whatever order the daemon sent them in.
// PREVENTS: the row under the cursor moving between two polls that carry the
// same data, which is what makes a peer impossible to pick.
func TestDashboardSortTiesAreTotal(t *testing.T) {
	established := func(addr string) dashboardPeer {
		return dashboardPeer{Address: addr, State: "established", Uptime: "1m0s"}
	}
	first := []dashboardPeer{established("10.0.0.3"), established("10.0.0.1"), established("10.0.0.2")}
	second := []dashboardPeer{established("10.0.0.2"), established("10.0.0.3"), established("10.0.0.1")}

	for _, ascending := range []bool{true, false} {
		want := addressesOf(sortDashboardPeers(first, sortColumnState, ascending, noRates))
		got := addressesOf(sortDashboardPeers(second, sortColumnState, ascending, noRates))
		assertOrder(t, "tie order independent of input order", got, want)
	}

	// Ascending and descending are each other's reverse. A comparison that
	// reports an equal pair as less in both directions breaks that.
	up := addressesOf(sortDashboardPeers(first, sortColumnState, true, noRates))
	down := addressesOf(sortDashboardPeers(first, sortColumnState, false, noRates))
	reversed := slices.Clone(down)
	slices.Reverse(reversed)
	assertOrder(t, "descending reverses ascending", reversed, up)
}

// VALIDATES: the rate column orders by the rate the table prints, and a peer
// with no measured rate orders after every peer that has one.
// PREVENTS: the sort reading updates-received as a stand-in, which ordered two
// peers with one counter and two rates against what the column showed.
func TestDashboardSortRateByMeasuredRate(t *testing.T) {
	// Every peer carries the same counter, so a sort on updates-received
	// leaves this table in address order and proves nothing about the rate.
	peers := []dashboardPeer{
		{Address: "10.0.0.1", UpdatesReceived: 100},
		{Address: "10.0.0.2", UpdatesReceived: 100},
		{Address: "10.0.0.3", UpdatesReceived: 100},
	}
	measured := map[string]float64{"10.0.0.1": 9.5, "10.0.0.2": 0.5}
	rate := func(addr string) (float64, bool) {
		value, ok := measured[addr]
		return value, ok
	}

	got := addressesOf(sortDashboardPeers(peers, sortColumnRate, true, rate))
	assertOrder(t, "rate ascending", got, []string{"10.0.0.2", "10.0.0.1", "10.0.0.3"})

	got = addressesOf(sortDashboardPeers(peers, sortColumnRate, false, rate))
	assertOrder(t, "rate descending", got, []string{"10.0.0.3", "10.0.0.1", "10.0.0.2"})
}

// VALIDATES: the rate the dashboard sorts on is the rate it prints, measured
// from two polls rather than read from the counter.
// PREVENTS: the column and the sort drifting apart again, which they can only
// do if the number and the string stop coming from one place.
func TestDashboardRateColumnSortsWhatItPrints(t *testing.T) {
	start := time.Now()
	ds := &dashboardState{sortAsc: true, sortColumn: sortColumnRate}
	first := &dashboardSnapshot{Peers: []dashboardPeer{
		{Address: "10.0.0.1", UpdatesReceived: 0},
		{Address: "10.0.0.2", UpdatesReceived: 0},
	}}
	ds.updateRates(first, start)

	// One peer takes 20 updates over the second, the other takes 2, and both
	// counters end far above where they started.
	second := &dashboardSnapshot{Peers: []dashboardPeer{
		{Address: "10.0.0.1", UpdatesReceived: 2},
		{Address: "10.0.0.2", UpdatesReceived: 20},
	}}
	ds.updateRates(second, start.Add(time.Second))
	ds.snapshot = second

	if got := ds.peerRate("10.0.0.2"); got != "20.0/s" {
		t.Fatalf("printed rate: got %s, want 20.0/s", got)
	}
	got := addressesOf(ds.sortedPeers())
	assertOrder(t, "rate ascending through the dashboard", got, []string{"10.0.0.1", "10.0.0.2"})
}

// VALIDATES: changing the sort column or direction keeps the highlight on the
// peer it was on.
// PREVENTS: the highlight staying at a row NUMBER while the table re-orders
// under it, so the operator opens a session they never pointed at.
func TestDashboardSelectionFollowsResort(t *testing.T) {
	ds := &dashboardState{
		sortAsc: true,
		snapshot: &dashboardSnapshot{Peers: []dashboardPeer{
			{Address: "10.0.0.1", RemoteAS: 65003},
			{Address: "10.0.0.2", RemoteAS: 65002},
			{Address: "10.0.0.3", RemoteAS: 65001},
		}},
	}

	// Select the middle row of the address-ordered table.
	ds.moveSelection(1)
	if ds.selectedAddr != "10.0.0.2" {
		t.Fatalf("after one move down: selected %s, want 10.0.0.2", ds.selectedAddr)
	}

	// Sorting by ASN reverses the table. The peer must not change, and the
	// index must be where that peer now sits.
	ds.sortColumn = sortColumnASN
	ds.followSelection()
	if ds.selectedAddr != "10.0.0.2" {
		t.Errorf("after re-sort: selected %s, want 10.0.0.2", ds.selectedAddr)
	}
	if peers := ds.sortedPeers(); peers[ds.selectedIdx].Address != "10.0.0.2" {
		t.Errorf("after re-sort: row %d holds %s, want 10.0.0.2", ds.selectedIdx, peers[ds.selectedIdx].Address)
	}

	// Reversing the direction moves the same peer to the other end.
	ds.sortAsc = false
	ds.followSelection()
	if peers := ds.sortedPeers(); peers[ds.selectedIdx].Address != "10.0.0.2" {
		t.Errorf("after reverse: row %d holds %s, want 10.0.0.2", ds.selectedIdx, peers[ds.selectedIdx].Address)
	}
}

// VALIDATES: navigation stops at the first and last row of the sorted table.
// PREVENTS: an index outside the table, and an index counted against the
// unsorted snapshot rather than the rows on screen.
func TestDashboardMoveSelectionBounds(t *testing.T) {
	ds := &dashboardState{
		sortAsc: true,
		snapshot: &dashboardSnapshot{Peers: []dashboardPeer{
			{Address: "10.0.0.1"},
			{Address: "10.0.0.2"},
		}},
	}

	ds.moveSelection(-1)
	if ds.selectedIdx != 0 || ds.selectedAddr != "10.0.0.1" {
		t.Errorf("up from the first row: idx %d addr %s, want 0 10.0.0.1", ds.selectedIdx, ds.selectedAddr)
	}

	ds.moveSelection(1)
	ds.moveSelection(1)
	if ds.selectedIdx != 1 || ds.selectedAddr != "10.0.0.2" {
		t.Errorf("down past the last row: idx %d addr %s, want 1 10.0.0.2", ds.selectedIdx, ds.selectedAddr)
	}

	// No snapshot: nothing to select, and nothing to panic over.
	empty := &dashboardState{}
	empty.moveSelection(1)
	empty.followSelection()
	if empty.selectedIdx != 0 || empty.selectedAddr != "" {
		t.Errorf("empty dashboard: idx %d addr %q, want 0 and empty", empty.selectedIdx, empty.selectedAddr)
	}
}

// noRates is the rate lookup for a table where nothing has been measured yet.
// Every column but Rate is ordered without one, and the Rate column's own test
// states the rates it sorts.
func noRates(string) (float64, bool) { return 0, false }

func addressesOf(peers []dashboardPeer) []string {
	out := make([]string, 0, len(peers))
	for _, p := range peers {
		out = append(out, p.Address)
	}
	return out
}

func assertOrder(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}
