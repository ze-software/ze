// Design: docs/architecture/plugin/rib-storage-design.md -- retained selected attributes.
// Related: rfc4271_selected_boundaries_test.go -- real ordinary-speaker fixture.
// Related: rs_departure_reconnect_test.go -- real destination Session replacement.

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
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

// partialSessionWireTarget names one valid recognized optional-transitive value.
// All are short attributes, and the AGGREGATOR uses the negotiated four-octet ASN.
type partialSessionWireTarget struct {
	code  attribute.AttributeCode
	value []byte
}

// partialSessionWireTargets supplies distinct ordinary values for the four
// dedicated pooled optional-transitive attributes, independent of path controls.
func partialSessionWireTargets() []partialSessionWireTarget {
	return []partialSessionWireTarget{
		{attribute.AttrAggregator, []byte{0, 0, 0xfd, 0xe9, 192, 0, 2, 9}},
		{attribute.AttrCommunity, []byte{0, 1, 0, 1}},
		{attribute.AttrExtCommunity, []byte{0, 2, 0xfd, 0xe9, 0, 0, 0, 1}},
		{attribute.AttrLargeCommunity, []byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 2, 0, 0, 0, 3}},
	}
}

// requirePartialSessionWireRoute checks the current useful route and every
// announcement, so a later correction cannot hide earlier bad attribute bytes.
func requirePartialSessionWireRoute(t *testing.T, view ownershipLiveProjection, prefix netip.Prefix, flags byte) {
	t.Helper()
	require.Len(t, view.routes, 1, "the receiver holds the useful source prefix")
	require.Contains(t, view.routes, prefix)
	require.NotEmpty(t, view.announcements[prefix], "the destination Session receives the useful route")
	require.Zero(t, view.withdrawals[prefix])
	requirePartialSessionWireAttributes(t, view.routes[prefix], flags)
	for _, attrs := range view.announcements[prefix] {
		requirePartialSessionWireAttributes(t, attrs, flags)
	}
}

// requirePartialSessionWireAttributes checks useful path fields and the separate
// iBGP LOCAL_PREF projection before the four optional-transitive attributes.
func requirePartialSessionWireAttributes(t *testing.T, attrs []byte, flags byte) {
	t.Helper()
	for _, control := range []struct {
		code  attribute.AttributeCode
		value []byte
	}{
		{attribute.AttrOrigin, []byte{0}},
		{attribute.AttrASPath, []byte{2, 1, 0, 0, 0xfd, 0xf2}},
		{attribute.AttrNextHop, []byte{198, 18, 249, 1}},
		{attribute.AttrLocalPref, []byte{0, 0, 0, 100}},
	} {
		_, _, value, found := attribute.AttrFind(attrs, control.code)
		require.True(t, found, "useful attribute %d reaches the peer", control.code)
		require.Equal(t, control.value, value, "attribute %d value", control.code)
	}
	requirePartialSessionWireTargets(t, attrs, flags)
}

// requirePartialSessionWireTargets checks retained values and all flags except
// the independently legal Extended Length bit (RFC 4271 Section 4.3).
func requirePartialSessionWireTargets(t *testing.T, attrs []byte, flags byte) {
	t.Helper()
	for _, target := range partialSessionWireTargets() {
		_, received, value, found := attribute.AttrFind(attrs, target.code)
		require.True(t, found, "recognized attribute %d survives", target.code)
		require.Equal(t, target.value, value, "attribute %d retains its exact value", target.code)
		require.Equal(t, flags, byte(received)&^byte(attribute.FlagExtLength),
			"attribute %d retains its received flags apart from header width", target.code)
	}
}
