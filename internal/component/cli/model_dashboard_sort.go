// Design: docs/architecture/api/commands.md — dashboard sort and selection.
// Overview: model.go — editor model and update loop.
// Related: model_dashboard.go — dashboard state and lifecycle.
// Related: model_dashboard_render.go — dashboard rendering.

package cli

import (
	"cmp"
	"net/netip"
	"slices"
	"strings"
	"time"
)

// dashboardSortColumn identifies which column the peer table is sorted by.
type dashboardSortColumn int

const (
	sortColumnAddress dashboardSortColumn = iota
	sortColumnASN
	sortColumnState
	sortColumnUptime
	sortColumnRx
	sortColumnTx
	sortColumnRate
	numSortColumns // sentinel for cycling
)

// next returns the next sort column in the cycle.
func (c dashboardSortColumn) next() dashboardSortColumn {
	return (c + 1) % numSortColumns
}

// String returns the column display name.
func (c dashboardSortColumn) String() string {
	names := [numSortColumns]string{"Peer", "ASN", "State", "Uptime", "Rx", "Tx", "Rate"}
	if c >= 0 && c < numSortColumns {
		return names[c]
	}
	return "?"
}

// sortDashboardPeers returns a sorted copy of the peer slice.
// The original slice is not modified.
//
// The order is TOTAL: comparePeers falls back to the peer address, which is
// unique in a snapshot, so no two rows ever compare equal. The highlight stays
// on its peer when the next poll arrives, even in a column where every row
// holds the same value.
//
// The direction reverses the comparison rather than negating a boolean. A
// boolean `!less` reports an equal pair as less in BOTH directions. That is not
// an ordering, and a sort given one is free to permute those rows.
func sortDashboardPeers(peers []dashboardPeer, col dashboardSortColumn, ascending bool) []dashboardPeer {
	sorted := make([]dashboardPeer, len(peers))
	copy(sorted, peers)

	slices.SortStableFunc(sorted, func(a, b dashboardPeer) int {
		order := comparePeers(a, b, col)
		if !ascending {
			return -order
		}
		return order
	})

	return sorted
}

// comparePeers orders two peer rows by the given column, negative when a comes
// first, and breaks a tie on the peer address.
//
// Every column is compared by its VALUE, never by the text the table prints.
// The two that differ are the ones an operator notices first: 10.0.0.9 belongs
// before 10.0.0.10, and an uptime of 59s belongs before one of 1m0s. Both come
// out of the daemon as strings, and both order backwards when compared as text.
func comparePeers(a, b dashboardPeer, col dashboardSortColumn) int {
	order := 0
	switch col {
	case sortColumnASN:
		order = cmp.Compare(a.RemoteAS, b.RemoteAS)
	case sortColumnState:
		order = cmp.Compare(stateRank(a.State), stateRank(b.State))
	case sortColumnUptime:
		order = compareUptime(a.Uptime, b.Uptime)
	case sortColumnRx:
		order = cmp.Compare(a.UpdatesReceived, b.UpdatesReceived)
	case sortColumnTx:
		order = cmp.Compare(a.UpdatesSent, b.UpdatesSent)
	case sortColumnRate:
		order = cmp.Compare(a.UpdatesReceived, b.UpdatesReceived) // rate sort uses Rx as proxy
	case sortColumnAddress, numSortColumns:
		order = 0 // the address is the tie-break below, so the column needs no case
	}
	if order != 0 {
		return order
	}
	return compareAddress(a.Address, b.Address)
}

// compareAddress orders two peer addresses by their numeric value, so 10.0.0.9
// comes before 10.0.0.10 and every IPv4 peer comes before every IPv6 one.
//
// An address the daemon sent in a form netip cannot read is ordered after every
// readable one, and against another unreadable one by text. The alternative is
// to read it as 0.0.0.0, which puts it on the top row as the lowest peer.
func compareAddress(a, b string) int {
	addrA, errA := netip.ParseAddr(a)
	addrB, errB := netip.ParseAddr(b)
	switch {
	case errA == nil && errB == nil:
		return addrA.Compare(addrB)
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(a, b)
}

// compareUptime orders two uptimes by the duration they measure.
//
// The daemon sends a Go duration string, so "59s" and "1m0s" are one second
// apart and sort a minute apart as text. The column changes every second, so
// text order rewrites the table under the operator's cursor on every poll.
//
// An unparsable uptime orders after every parsable one, and against another
// unparsable one by text.
func compareUptime(a, b string) int {
	durA, errA := time.ParseDuration(a)
	durB, errB := time.ParseDuration(b)
	switch {
	case errA == nil && errB == nil:
		return cmp.Compare(durA, durB)
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(a, b)
}

// The peer state names the dashboard knows, spelled as the daemon prints them
// (plugin.PeerState.String for the summary, the FSM names for the rest). They
// are declared once here because two views read them: the rank below and the
// color the peer table shows the state in (model_dashboard_render.go).
const (
	peerStateStopped     = "stopped"
	peerStateIdle        = "idle"
	peerStateIdleHold    = "idle-hold"
	peerStateConnecting  = "connecting"
	peerStateActive      = "active"
	peerStateOpenSent    = "opensent"
	peerStateOpenConfirm = "openconfirm"
	peerStateEstablished = "established"
)

// stateRank orders a peer state by how far the session has progressed. That is
// the order the state enum declares (plugin.PeerState) and the order an operator
// reads the column in. The printed name instead puts "active" above
// "established" and "idle-hold" between them, which is alphabet, not progress.
//
// A state this table does not know ranks after every state it does. A name the
// daemon adds therefore shows up at one end, rather than sorting as "stopped".
func stateRank(state string) int {
	switch state {
	case peerStateStopped:
		return 0
	case peerStateIdle, peerStateIdleHold:
		return 1
	case peerStateConnecting:
		return 2
	case peerStateActive:
		return 3
	case peerStateOpenSent:
		return 4
	case peerStateOpenConfirm:
		return 5
	case peerStateEstablished:
		return 6
	}
	return 7
}
