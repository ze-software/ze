// Design: docs/architecture/plugin/rib-storage-design.md -- retained selected attributes.
// Design: docs/architecture/api/process-protocol.md -- stored-route relay rail.
// Related: rfc4271_partial_session_wire_test.go -- positive Session preservation test.
// Related: rfc4271_relay_partial_test.go -- positive Section 4.3 clear on the relay rail.
// Related: partial_session_wire_test.go -- retained attribute and wire assertions.

package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC4271CompleteRecognizedTransitiveStaysClearOnSessionWire sends
// recognized optional transitive attributes with Partial clear over an eBGP
// Session and checks stored RIB bytes and iBGP Session wire output, including
// destination reconnect without a new source UPDATE.
//
// VALIDATES: AGGREGATOR, COMMUNITY, EXTENDED_COMMUNITY and LARGE_COMMUNITY
// arriving with flags 0xC0 remain 0xC0 in RIB storage and on ordinary iBGP
// Session wire output, rather than unconditionally forcing Partial to 1.
//
// PREVENTS: Satisfying RFC 4271 Section 5's prohibition on clearing Partial by
// forcing Partial to 1 on every stored or relayed optional transitive attribute
// regardless of whether a previous AS set it.
//
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS."
// RFC requirement: RFC4271-5-4 negative -- recognized optional transitive attributes AGGREGATOR, COMMUNITY, EXTENDED_COMMUNITY and LARGE_COMMUNITY received with Partial clear retain Partial clear in RIB storage and on ordinary eBGP-to-iBGP ASN4 Session wire output, including destination reconnect without a new source UPDATE, proving Partial preservation is conditioned on a received Partial bit rather than unconditionally setting Partial.
func TestRFC4271CompleteRecognizedTransitiveStaysClearOnSessionWire(t *testing.T) {
	const completeFlags byte = 0xc0
	const expectedNextHop = "198.18.249.1"
	prefix := netip.MustParsePrefix("198.19.237.0/24")

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

	attrs := synthesizedNextHopAttrs(netip.MustParseAddr(expectedNextHop), 65010)
	for _, target := range partialSessionWireTargets() {
		require.True(t, target.code.Recognized(), "attribute %d is recognized", target.code)
		attrs = append(attrs, makeAttr(completeFlags, byte(target.code), target.value)...)
	}

	// RFC 4271 Section 5: receive complete (Partial=0) recognized transitive
	// attributes over the real source Session and inspect stored + relayed wire.
	ownershipLiveSend(t, source, nil, attrs, prefix)
	ordinarySelectedStored(t, source, prefix)
	ordinarySelectedBest(t, prefix, expectedNextHop)
	ordinaryBoundaryDrain(t, source)

	address := prefix.Addr().As4()
	stored := extendedRecoveryStoredAttributes(source.peer.Settings().Address.String(), address[:3])
	require.NotEmpty(t, stored, "the valid source candidate is retained in the actual RIB")
	requirePartialSessionWireTargets(t, stored, completeFlags)

	ordinarySelectedWire(t, destination, prefix, expectedNextHop)
	ordinaryBoundaryCapture(t, destination)
	requirePartialSessionWireRoute(t, ownershipLiveRead(t, destination), prefix, completeFlags)

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

	// RFC 4271 Section 5: destination replay reconstructs the stored RouteEntry
	// without a new source UPDATE and must still emit Partial=0.
	reconnected := lowLiveReconnect(t, destination, identifier)
	replaySession := reconnected.peer.currentSession()
	require.NotNil(t, replaySession)
	require.NotSame(t, previousSession, replaySession)
	require.Equal(t, identifier, reconnected.peer.RemoteRouterID())
	ordinarySelectedWire(t, reconnected, prefix, expectedNextHop)
	ordinaryBoundaryDrain(t, source)
	ordinaryBoundaryCapture(t, reconnected)
	requirePartialSessionWireRoute(t, ownershipLiveRead(t, reconnected), prefix, completeFlags)
	require.Same(t, sourceSession, source.peer.currentSession())
	require.Equal(t, updates, source.peer.Stats().UpdatesReceived,
		"destination replay must not depend on a new source UPDATE")
	stored = extendedRecoveryStoredAttributes(source.peer.Settings().Address.String(), address[:3])
	require.NotEmpty(t, stored)
	requirePartialSessionWireTargets(t, stored, completeFlags)
	ownershipLiveEstablished(t, []*lowLivePeer{source, reconnected})
}

// TestRFC4271PartialClearedOutsideOptionalTransitiveOnRelayedWire drives the
// receive and transparent route-server relay rail with Partial set on two
// attribute classes outside optional transitive: well-known ORIGIN (0x60) and
// optional non-transitive MULTI_EXIT_DISC (0xA0).
//
// VALIDATES: Outside RFC 4271 Section 5's transitive optional scope, Ze clears
// a received Partial bit back to 0 on ORIGIN (0x40) and MED (0x80) before
// relaying the stored UPDATE to a destination peer.
//
// PREVENTS: Passing RFC 4271-5-4 positive checks via a relay path that never
// clears the Partial bit on any attribute class.
//
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS."
// RFC requirement: RFC4271-5-4 negative -- outside transitive optional attributes, a Partial bit received on well-known ORIGIN (0x60) or optional non-transitive MULTI_EXIT_DISC (0xA0) is cleared back to 0 (0x40 and 0x80) when published and relayed to a route-server client, proving preservation is scoped to optional transitive attributes rather than a path that never clears Partial.
func TestRFC4271PartialClearedOutsideOptionalTransitiveOnRelayedWire(t *testing.T) {
	attrs := []byte{
		0x60, 0x01, 0x01, 0x00, // ORIGIN, well-known, Partial set on the wire
		0x40, 0x02, 0x0e, 0x02, 0x03, 0x00, 0x00, 0xfb, 0xf0, 0x00, 0x00, 0xfc, 0x00, 0x00, 0x00, 0xfb, 0xf1, // AS_PATH
		0x40, 0x03, 0x04, 0x01, 0x01, 0x01, 0x01, // NEXT_HOP 1.1.1.1
		0xa0, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a, // MED, optional non-transitive, Partial set
	}
	nlri := []byte{24, 10, 0, 1}

	s := newValidateSession()
	s.negotiated = &capability.Negotiated{ASN4: true}

	// RFC 4271 Section 4.3 and Section 5: publishBase clears Partial only on
	// well-known and optional non-transitive attributes.
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(makeUpdateBody(nil, attrs, nlri), 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionNone, action,
		"Partial on ORIGIN or MED is not malformed under RFC 7606 Section 3(c)")

	published, err := wu.Attrs()
	require.NoError(t, err, "the published UPDATE must index")
	route := rpc.StoredRoute{
		SourcePeer: "10.0.0.1",
		Family:     "ipv4/unicast",
		AttrHex:    hex.EncodeToString(published.Packed()),
		NextHopHex: "01010101",
		NLRIHex:    "180a0001",
	}

	api, _, dispatched, mu, done := relayFixture(t)
	dst := api.r.peers[netip.MustParseAddrPort("10.0.0.2:179")]
	require.NotNil(t, dst, "fixture must expose the destination peer")
	dst.settings.RSClient = true
	dst.refreshForwardFacts()

	relayed := relayDispatchedBodyFor(t, api, dispatched, mu, done, route)

	assert.Equal(t, byte(0x40), rfc4271PublishedFlags(t, relayed, byte(attribute.AttrOrigin)),
		"outside optional transitive, well-known ORIGIN has Partial cleared to 0")
	assert.Equal(t, byte(0x80), rfc4271PublishedFlags(t, relayed, byte(attribute.AttrMED)),
		"outside optional transitive, optional non-transitive MED has Partial cleared to 0")
}
