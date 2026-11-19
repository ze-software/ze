// Design: docs/architecture/plugin/rib-storage-design.md -- held original validation references.
package rib

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"testing"
)

// A comparable key is publishable only with the held original owner and
// connection. Native framing and legal path identifier zero remain intact.
func TestFlowSpecKeyRequiresHeldOriginalConnection(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.188")
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	raw := []byte{5, 1, 24, 10, 77, 0}
	key, ok := flowSpecKey(peer, 1701, 41, fam, raw, false)
	require.True(t, ok)
	require.Equal(t, uint64(41), key.ConnectionID)
	require.Equal(t, string(raw), key.NLRI)
	_, ok = flowSpecKey(peer, 1701, 0, fam, raw, false)
	require.False(t, ok)
	_, ok = flowSpecKey(peer, 0, 41, fam, raw, false)
	require.False(t, ok)
}

// Actual typed/framed SDK lease acquisition precedes received storage. Fresh
// UP metadata cannot relabel a held original FlowSpec rule. The query/path
// primitives are proved here; actual GR-to-role wire completion stays a
// separate integration question.
func TestFlowSpecValidationUsesHeldOriginalConnection(t *testing.T) {
	for _, native := range []bool{true, false} {
		t.Run(map[bool]string{true: "typed-lease", false: "framed-lease"}[native], func(t *testing.T) {
			bus := newTestEventBus()
			SetEventBus(bus)
			r, calls := sourceLeaseTestRIBWithForwarding(t, native, false)
			calls.mu.Lock()
			calls.issued = map[uint64]uint64{1701: 1701, 1702: 1702}
			calls.mu.Unlock()
			peer := netip.MustParseAddr("192.0.2.188")
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			raw := []byte{5, 1, 24, 10, 77, 0}
			flowValidationReceive(t, r, 1702, netip.MustParseAddr("192.0.2.189"), 65001, 70, family.IPv4Unicast, []byte{8, 10}, flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 0, nil), false)
			event := flowValidationReceive(t, r, 1701, peer, 65000, 71, fam, raw, flowValidationAttrs(nil, netip.Addr{}, 0, nil), false)
			require.Equal(t, uint64(41), event.ConnectionID)
			require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, 1701))
			original := ribevents.ValidationRoute{Peer: peer, PeerOwner: 1701, ConnectionID: 41, Family: fam, NLRI: string(raw)}
			require.True(t, r.flowSpecEligible(original, 71))
			require.True(t, r.flowSpecPresent(original))
			path, found := r.flowSpecPath(original)
			require.True(t, found)
			require.Equal(t, uint64(1701), path.SourceReceipt)
			require.NotEmpty(t, path.Attributes)
			require.NotEmpty(t, flowValidationEvents(bus), "a useful authorized rule is published")
			r.handleStructuredState(&rpc.StructuredEvent{EventType: rpc.EventKindState, State: rpc.SessionStateUp, PeerAddress: peer.String(), PeerOwner: 1701, ConnectionID: 42, DestinationSession: 43, PeerAS: rpc.RemoteASOf(65000), LocalAS: 65000})
			require.True(t, r.flowSpecEligible(original, 71))
			require.True(t, r.flowSpecPresent(original))
			_, found = r.flowSpecPath(original)
			require.True(t, found, "fresh metadata does not retag retained original bytes")
			wrong := original
			wrong.ConnectionID = 42
			require.False(t, r.flowSpecEligible(wrong, 71))
			require.False(t, r.flowSpecPresent(wrong))
			_, found = r.flowSpecPath(wrong)
			require.False(t, found, "same-owner successor cannot borrow original attributes")
			wrong.ConnectionID = 0
			require.False(t, r.flowSpecEligible(wrong, 71))
			require.True(t, r.flowSpecEligible(original, 71), "rejecting an alias cannot reject the original rule")
		})
	}
}
