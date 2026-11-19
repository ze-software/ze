// Design: docs/architecture/plugin/rib-storage-design.md -- independent forwarding and learned scopes.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestManualForwardingWinnerDoesNotHideLearnedRelease keeps the manual FIB
// winner throughout retain/release of a separately leased learned candidate.
// The useful learned snapshot must disappear even though the FIB winner does
// not change and therefore cannot supply a departed-source bestPrev record.
func TestManualForwardingWinnerDoesNotHideLearnedRelease(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, jsonRail := range []bool{false, true} {
			rail := "wire/"
			if native {
				rail = "native/"
			}
			t.Run(rail+testReceiptRailName(jsonRail, false), func(t *testing.T) {
				r, calls := sourceLeaseTestRIB(t, native)
				// Stop table observers before either fixture releases stores.
				defer r.SetLocRIB(nil)
				t.Cleanup(func() {
					for _, routes := range r.ribInPool[bgpProtocolID] {
						routes.Release()
					}
				})
				source, _ := sourceIdentityPeers()
				prefix := netip.MustParsePrefix("10.77.1.0/24")
				raw := []byte{24, 10, 77, 1}
				source.state(t, r, jsonRail, rpc.SessionStateUp, rpc.ReasonAttached)
				source.receive(t, r, jsonRail, raw, 100)
				require.Equal(t, 1, calls.count(rpc.SourceLeaseAcquire, source.receipt))

				loc := r.locRIB.Load()
				require.NotNil(t, loc)
				manualHop := netip.MustParseAddr("10.0.0.2")
				loc.Insert(family.IPv4Unicast, netip.PrefixFrom(manualHop, 32), locrib.Path{
					Source: redistevents.RegisterProtocol("connected"), Interface: "manual-release0",
				})
				status, _, err := r.handleCommand("request bgp rib inject", "", []string{
					"192.0.2.199", family.IPv4Unicast.String(), prefix.String(),
					"nhop", manualHop.String(), "localpref", "300",
				})
				require.NoError(t, err)
				require.Equal(t, statusDone, status)
				forwarding, found := loc.Best(family.IPv4Unicast, prefix)
				require.True(t, found)
				require.Equal(t, manualHop, forwarding.NextHop, "the manual candidate must actually win forwarding")
				snapshot, err := r.advertiseSnapshot()
				require.NoError(t, err)
				require.Len(t, snapshot, 1, "manual forwarding must not hide useful ordinary learned output")
				require.Equal(t, source.owner, snapshot[0].route.SourceOwner)
				require.Equal(t, source.receipt, snapshot[0].route.SourceReceipt)
				require.Equal(t, raw, snapshot[0].route.NLRI)

				retained, err := r.retainRoutes(sourceIdentityAddress, nil, false)
				require.NoError(t, err)
				require.Equal(t, map[string]any{"retained-peers": 1}, retained)
				require.Equal(t, 1, calls.count(rpc.SourceLeaseRetain, source.receipt))
				require.Equal(t, map[string]any{"released-peers": 1}, r.releaseRoutes(sourceIdentityAddress))
				snapshot, err = r.advertiseSnapshot()
				require.NoError(t, err)
				require.Empty(t, snapshot, "release must reconcile the learned scope hidden behind the manual winner")
				forwarding, found = loc.Best(family.IPv4Unicast, prefix)
				require.True(t, found, "source release must preserve useful manual forwarding")
				require.Equal(t, manualHop, forwarding.NextHop)
				require.Len(t, r.ribInPool[bgpProtocolID], 1)
				require.NoError(t, r.sourceLeases.settle(t.Context()))
				require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, source.receipt))
			})
		}
	}
}
