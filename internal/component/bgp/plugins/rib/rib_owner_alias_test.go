// Design: docs/architecture/plugin/rib-storage-design.md -- independent observed Peer owners.
// Related: sent_owner.go, rib_recovery.go, rib_peer_name.go -- exact owner consumers.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSentSameAddressOwnersRemainIndependent records two destinations at one
// address and two original sources at one address, then retires only source A.
func TestSentSameAddressOwnersRemainIndependent(t *testing.T) {
	r := newTestRIBManager(t)
	address := netip.MustParseAddr("192.0.2.20")
	source := netip.MustParseAddr("192.0.2.10")
	r.peerMeta[101] = &peerMetadata{Address: source}
	r.peerMeta[102] = &peerMetadata{Address: source}
	raw := []byte{24, 198, 51, 100}
	r.handleSent(nativeSentEvent(t, address, 201, 101, family.IPv4Unicast, raw, nativeSentAttrs(), false, false))
	r.handleSent(nativeSentEvent(t, address, 202, 102, family.IPv4Unicast, raw, nativeSentAttrs(), false, false))
	key := ribOutKey{Prefix: netip.MustParsePrefix("198.51.100.0/24")}
	t.Cleanup(func() {
		for _, families := range r.ribOut {
			for _, routes := range families {
				for _, entry := range routes {
					entry.release()
				}
			}
		}
	})
	require.Equal(t, uint64(101), r.ribOut[201][family.IPv4Unicast][key].SourceOwner)
	require.Equal(t, uint64(102), r.ribOut[202][family.IPv4Unicast][key].SourceOwner)
	r.peerMu.Lock()
	writes := r.pruneSentSourceLocked(101, nil)
	r.peerMu.Unlock()
	require.Len(t, writes, 1)
	require.Equal(t, uint64(201), writes[0].owner)
	require.Equal(t, address.String(), writes[0].peer)
	require.Empty(t, r.ribOut[201])
	require.Equal(t, uint64(102), r.ribOut[202][family.IPv4Unicast][key].SourceOwner)
	require.NotEmpty(t, r.collectRibOutRoutes(202, family.IPv4Unicast), "the sibling destination retains useful output")
}

// TestRecoverySameAddressOwnersExcludeIndependently gives two configured
// sources the same address and real SDK leases. Excluding A must still elect B,
// even when the destination uses that same observed address.
func TestRecoverySameAddressOwnersExcludeIndependently(t *testing.T) {
	r, _ := sourceLeaseTestRIB(t, true)
	recoveryTestCleanup(t, r)
	address := netip.MustParseAddr("192.0.2.10")
	raw := []byte{24, 198, 51, 100}
	recoveryReceived(t, r, address, 101, 701, raw, 200, 81, false, nil)
	recoveryReceived(t, r, address, 102, 702, raw, 100, 82, false, nil)
	request := ribevents.RecoveryRequest{Destination: address, DestinationOwner: 201, Family: family.IPv4Unicast,
		Keys: []ribevents.RecoveryKey{recoveryKey(raw, 101)}}
	outcomes, err := r.recoveryRoutes(request)
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.False(t, outcomes[0].Withdraw)
	require.Len(t, outcomes[0].Survivors, 1)
	survivor := outcomes[0].Survivors[0]
	require.Equal(t, address, survivor.Source)
	require.Equal(t, uint64(102), survivor.SourceOwner)
	require.Equal(t, uint64(702), survivor.SourceReceipt)
	require.Equal(t, uint64(82), survivor.MessageID)
	require.Equal(t, raw, survivor.NLRI)
	require.NotEmpty(t, survivor.Attributes)
	request.Keys[0].Excluded = []uint64{101, 102}
	outcomes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	require.True(t, outcomes[0].Withdraw)
	require.Empty(t, outcomes[0].Survivors)
}

// TestRIBSameOwnerLateDownKeepsReplacement distinguishes the configured Peer
// from its current Session. A predecessor DOWN cannot remove the replacement's
// useful received routes; the replacement's own DOWN still removes them.
func TestRIBSameOwnerLateDownKeepsReplacement(t *testing.T) {
	r := newTestRIBManager(t)
	r.dispatchHook = func(string) {}
	source := netip.MustParseAddr("192.0.2.10")
	t.Cleanup(func() {
		for _, routes := range r.bgpPeers {
			routes.Release()
		}
	})
	r.handleStructuredState(&rpc.StructuredEvent{PeerOwner: 101, PeerAddress: source.String(), State: rpc.SessionStateUp, DestinationSession: 11})
	lifecycleReceive(t, r, lifecycleFeed, source, 81, false)
	r.handleStructuredState(&rpc.StructuredEvent{PeerOwner: 101, PeerAddress: source.String(), State: rpc.SessionStateUp, DestinationSession: 12})
	lifecycleReceive(t, r, lifecycleFeed, source, 82, true)
	replacement := r.bgpPeers[101]
	require.NotNil(t, replacement)
	r.handleStructuredState(&rpc.StructuredEvent{PeerOwner: 101, PeerAddress: source.String(), State: rpc.SessionStateDown,
		DestinationSession: 11, Reason: rpc.ReasonTCPFailure})
	require.True(t, r.peerUp[101])
	require.Same(t, replacement, r.bgpPeers[101])
	require.Equal(t, uint64(12), r.peerMeta[101].Session)
	_, first := replacement.Lookup(family.IPv4Unicast, []byte{24, 192, 0, 2})
	_, second := replacement.Lookup(family.IPv4Unicast, []byte{24, 198, 51, 100})
	require.True(t, first, "the old DOWN must preserve the first useful received path")
	require.True(t, second, "the replacement's received path survives the old DOWN")
	r.handleStructuredState(&rpc.StructuredEvent{PeerOwner: 101, PeerAddress: source.String(), State: rpc.SessionStateDown,
		DestinationSession: 12, Reason: rpc.ReasonTCPFailure})
	require.NotContains(t, r.bgpPeers, uint64(101))
	require.False(t, r.peerUp[101])
}
