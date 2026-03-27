// Design: docs/architecture/api/commands.md — dashboard sort and selection.
// Overview: model.go — editor model and update loop.
// Related: model_dashboard.go — dashboard state and lifecycle.
// Related: model_dashboard_render.go — dashboard rendering.

package cli

import (
	"sort"
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
func sortDashboardPeers(peers []dashboardPeer, col dashboardSortColumn, ascending bool) []dashboardPeer {
	sorted := make([]dashboardPeer, len(peers))
	copy(sorted, peers)

	sort.SliceStable(sorted, func(i, j int) bool {
		less := comparePeers(sorted[i], sorted[j], col)
		if !ascending {
			return !less
		}
		return less
	})

	return sorted
}

// comparePeers returns true if a should sort before b for the given column.
func comparePeers(a, b dashboardPeer, col dashboardSortColumn) bool {
	switch col {
	case sortColumnAddress:
		return a.Address < b.Address
	case sortColumnASN:
		return a.RemoteAS < b.RemoteAS
	case sortColumnState:
		return a.State < b.State
	case sortColumnUptime:
		return a.Uptime < b.Uptime
	case sortColumnRx:
		return a.UpdatesReceived < b.UpdatesReceived
	case sortColumnTx:
		return a.UpdatesSent < b.UpdatesSent
	case sortColumnRate:
		return a.UpdatesReceived < b.UpdatesReceived // rate sort uses Rx as proxy
	case numSortColumns:
		return a.Address < b.Address
	}
	return a.Address < b.Address
}
