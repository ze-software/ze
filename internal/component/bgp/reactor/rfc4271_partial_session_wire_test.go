// Design: docs/architecture/plugin/rib-storage-design.md -- retained selected attributes.
// Related: partial_session_wire_test.go -- retained attribute and wire assertions.
// Related: rfc4271_selected_boundaries_test.go -- real ordinary-speaker fixture.

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestReceivedPartialSurvivesOrdinarySessionWire sends valid received routes
// through actual Sessions, RIB storage, ordinary selection and recipient wire.
// Only the destination reconnects: its fresh capture must contain the retained
// selection without another source UPDATE. The complete twin is a software
// control, not a claim that Section 5 requires acceptance of every complete path.
//
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS."
// The observation boundary is native peer wire; foreign acceptance is separate.
// RFC requirement: RFC4271-5-4 positive -- received Partial AGGREGATOR, COMMUNITY, EXTENDED_COMMUNITY and LARGE_COMMUNITY retain Partial, exact values and useful IPv4 NLRI on ordinary eBGP-to-iBGP Session wire output with four-octet ASNs on both sessions, including destination reconnect without a new source UPDATE.
func TestReceivedPartialSurvivesOrdinarySessionWire(t *testing.T) {
	for _, tc := range []struct {
		name   string
		flags  byte
		prefix netip.Prefix
	}{
		{"complete", 0xc0, netip.MustParsePrefix("198.19.235.0/24")},
		{"partial", 0xe0, netip.MustParsePrefix("198.19.236.0/24")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peers := ordinaryBoundaryPeers(t,
				acceptingLiveSettings(t, "192.0.2.225", 65000, 65010),
				acceptingLiveSettings(t, "192.0.2.226", 65000, 65000))
			source, destination := peers[0], peers[1]
			require.False(t, source.peer.IsIBGP())
			require.True(t, destination.peer.IsIBGP())
			for _, peer := range peers {
				negotiated := peer.peer.negotiated.Load()
				require.NotNil(t, negotiated)
				require.True(t, negotiated.ASN4, "both real sessions negotiate four-octet ASNs")
			}
			sourceSession := source.peer.currentSession()
			require.NotNil(t, sourceSession)
			attrs := synthesizedNextHopAttrs(netip.MustParseAddr("198.18.249.1"), 65010)
			for _, target := range partialSessionWireTargets() {
				attrs = append(attrs, makeAttr(tc.flags, byte(target.code), target.value)...)
			}
			// RFC 4271 Section 5: receive the existing attributes over the source
			// socket; no RIB setter, selected RPC or receipt is supplied here.
			ownershipLiveSend(t, source, nil, attrs, tc.prefix)
			ordinarySelectedStored(t, source, tc.prefix)
			ordinarySelectedBest(t, tc.prefix, "198.18.249.1")
			ordinaryBoundaryDrain(t, source)
			address := tc.prefix.Addr().As4()
			stored := extendedRecoveryStoredAttributes(source.peer.Settings().Address.String(), address[:3])
			require.NotEmpty(t, stored, "the valid source candidate is retained in the actual RIB")
			requirePartialSessionWireTargets(t, stored, tc.flags)
			ordinarySelectedWire(t, destination, tc.prefix, "198.18.249.1")
			ordinaryBoundaryCapture(t, destination)
			view := ownershipLiveRead(t, destination)
			requirePartialSessionWireRoute(t, view, tc.prefix, tc.flags)

			updates := source.peer.Stats().UpdatesReceived
			require.Positive(t, updates)
			previousSession := destination.peer.currentSession()
			require.NotNil(t, previousSession)
			identifier := destination.peer.RemoteRouterID()
			lowLiveDepart(t, destination)
			select {
			case <-destination.stopped:
			case <-time.After(5 * time.Second):
				t.Fatal("the departed destination capture did not stop")
			}
			// RFC 4271 Section 5: the new destination Session must receive the
			// current stored selection while the original source stays connected.
			reconnected := lowLiveReconnect(t, destination, identifier)
			replaySession := reconnected.peer.currentSession()
			require.NotNil(t, replaySession)
			require.NotSame(t, previousSession, replaySession)
			require.Equal(t, identifier, reconnected.peer.RemoteRouterID())
			ordinarySelectedWire(t, reconnected, tc.prefix, "198.18.249.1")
			ordinaryBoundaryDrain(t, source)
			ordinaryBoundaryCapture(t, reconnected)
			view = ownershipLiveRead(t, reconnected)
			requirePartialSessionWireRoute(t, view, tc.prefix, tc.flags)
			require.Same(t, sourceSession, source.peer.currentSession())
			require.Equal(t, updates, source.peer.Stats().UpdatesReceived,
				"destination replay must not depend on a new source UPDATE")
			stored = extendedRecoveryStoredAttributes(source.peer.Settings().Address.String(), address[:3])
			require.NotEmpty(t, stored)
			requirePartialSessionWireTargets(t, stored, tc.flags)
			ownershipLiveEstablished(t, []*lowLivePeer{source, reconnected})
		})
	}
}
