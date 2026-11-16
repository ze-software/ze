// Design: docs/architecture/update-building.md -- current selected authority.
// Related: selected_current.go -- no historical revision table.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedCurrentAcceptsOldCurrentRevision proves a scope's unrelated
// changes do not stale a still-current native key at its original revision.
func TestSelectedCurrentAcceptsOldCurrentRevision(t *testing.T) {
	route := rpc.SelectedRoute{Kind: rpc.SelectedRouteReceived, Revision: 3}
	current := rpc.CurrentSelectionResult{Presence: rpc.CurrentSelectionPresent, CurrentRevision: 3}
	require.NoError(t, selectedCurrentRevision(route, current, 500))
}

// TestSelectedCurrentRefusesDeletedAndUnpublished proves an old present value
// cannot return after an absence, and a caller cannot publish a future value.
func TestSelectedCurrentRefusesDeletedAndUnpublished(t *testing.T) {
	route := rpc.SelectedRoute{Kind: rpc.SelectedRouteReceived, Revision: 10}
	require.ErrorIs(t, selectedCurrentRevision(route,
		rpc.CurrentSelectionResult{Presence: rpc.CurrentSelectionAbsent}, 11), errSelectedRevision)
	require.ErrorIs(t, selectedCurrentRevision(route,
		rpc.CurrentSelectionResult{Presence: rpc.CurrentSelectionPresent, CurrentRevision: 10}, 9), errSelectedRevision)
}

// TestSelectedCurrentAbsenceUsesCurrentCut proves unrelated changes after a
// removal need no historical per-key absence revision to finish that removal.
func TestSelectedCurrentAbsenceUsesCurrentCut(t *testing.T) {
	route := rpc.SelectedRoute{Kind: rpc.SelectedRouteAbsent, Revision: 11}
	require.NoError(t, selectedCurrentRevision(route,
		rpc.CurrentSelectionResult{Presence: rpc.CurrentSelectionAbsent}, 500))
	require.ErrorIs(t, selectedCurrentRevision(route,
		rpc.CurrentSelectionResult{Presence: rpc.CurrentSelectionPresent, CurrentRevision: 12}, 500), errSelectedRevision)
}

// TestSelectedSourceWriteRequiresExistingLease proves knowledge of an issued
// receipt never implicitly acquires a selecting process's forwarding rights.
func TestSelectedSourceWriteRequiresExistingLease(t *testing.T) {
	peer, facts := sourceReceiptFixture(t)
	var store sourceReceiptStore
	live, err := store.issue(facts)
	require.NoError(t, err)
	defer live.close()
	owner, _ := sourceReceiptOwner(t)
	_, err = store.admitWrite(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.ErrorContains(t, err, "lease")
	lease, err := store.acquire(live.ID(), owner)
	require.NoError(t, err)
	write, err := store.admitWrite(live.ID(), owner, peer.sourcePolicyEpoch.Load())
	require.NoError(t, err)
	require.NoError(t, write.current())
	write.complete()
	require.NoError(t, lease.revoke(t.Context()))
}
