// Design: docs/architecture/plugin/rib-storage-design.md -- exact source identity in elections.
// Related: rib_source_identity_test.go -- authenticated same-address source events.
package rib

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRIBSourceOwnerElectionTieDoesNotDependOnArrival distinguishes equal
// paths from different owners even when their address and socket facts agree.
// The owner is the final stable tie after the BGP comparisons, so arrival order
// cannot choose a different original receipt for the same candidate set.
func TestRIBSourceOwnerElectionTieDoesNotDependOnArrival(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		for _, reversed := range []bool{false, true} {
			name := testReceiptRailName(jsonRail, false)
			if reversed {
				name += "/reversed"
			}
			t.Run(name, func(t *testing.T) {
				r, _ := sourceLeaseTestRIB(t, true)
				first, second := sourceIdentityPeers()
				peers := []sourceIdentityPeer{first, second}
				if reversed {
					peers[0], peers[1] = peers[1], peers[0]
				}
				nlri := []byte{24, 10, 77, 7}
				for _, peer := range peers {
					peer.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
					peer.receive(t, r, jsonRail, nlri, 100)
				}
				require.Len(t, gatherCandidatesHeld(t, r, family.IPv4Unicast, nlri, false), 2)
				current, err := r.advertiseSnapshot()
				require.NoError(t, err)
				require.Len(t, current, 1)
				require.Equal(t, first.receipt, current[0].route.SourceReceipt,
					"equal candidates must retain the same winning owner in either arrival order")
			})
		}
	}
}

// TestRIBSourceOwnerElectionPublishesSameAddressSuccessor removes the elected
// source while its equal-attribute sibling remains. Publication must replace
// the original source receipt without an intermediate absence or loss of the
// sibling's retained lease.
func TestRIBSourceOwnerElectionPublishesSameAddressSuccessor(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(testReceiptRailName(jsonRail, false), func(t *testing.T) {
			r, calls := sourceLeaseTestRIB(t, true)
			first, second := sourceIdentityPeers()
			nlri := []byte{24, 10, 77, 8}
			first.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			first.receive(t, r, jsonRail, nlri, 100)
			second.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
			second.receive(t, r, jsonRail, nlri, 100)
			before, err := r.advertiseSnapshot()
			require.NoError(t, err)
			require.Len(t, before, 1)
			require.Equal(t, first.receipt, before[0].route.SourceReceipt)

			first.state(t, r, jsonRail, rpc.SessionStateDown, rpc.ReasonPeerRemoved)

			after, err := r.advertiseSnapshot()
			require.NoError(t, err)
			require.Len(t, after, 1, "the surviving owner must remain selected")
			require.Equal(t, second.receipt, after[0].route.SourceReceipt)
			require.Greater(t, after[0].revision, before[0].revision)
			require.Equal(t, before[0].route.Attributes, after[0].route.Attributes)
			require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, first.receipt))
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, second.receipt))
		})
	}
}
