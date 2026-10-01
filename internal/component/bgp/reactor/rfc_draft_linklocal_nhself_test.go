// Design: docs/architecture/bgp/structural-forwarding.md -- forwarded routes and next-hop self
// Related: forward_next_hop.go -- egressNextHopLinkLocalOnlyRefused, the forward rails' gate
// Related: peer.go -- linkLocalOnlyNextHopRefused, the predicate the announce rail asks too
// Related: reactor_a2_rfc8950_forward_test.go -- a2Forward and a2Parts, the harness

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// nhSelfLinkLocalEndpoint is the TCP local endpoint of a session that runs over
// a link-local address, so next hop self resolves to a link-local address.
var nhSelfLinkLocalEndpoint = netip.MustParseAddr("fe80::77")

// nhSelfReceivedNextHop is the Global next hop the forwarded route arrives with.
var nhSelfReceivedNextHop = netip.MustParseAddr("2001:db8:1::9")

// nhSelfLinkLocalPayload announces 2001:db8:7::/64 as IPv6 unicast with the
// 16-octet Global next hop nhSelfReceivedNextHop, AS_PATH [65001].
func nhSelfLinkLocalPayload() []byte {
	nh := nhSelfReceivedNextHop.As16()
	// AFI 2 (IPv6), SAFI 1 (unicast), Length of Next Hop Network Address 16.
	value := []byte{0x00, 0x02, 0x01, 0x10}
	value = append(value, nh[:]...)
	// The Reserved octet, then 2001:db8:7::/64 as one NLRI.
	value = append(value, 0x00, 0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x07, 0x00, 0x00)

	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN igp
	aspValue := []byte{0x02, 0x01, 0x00, 0x00}
	binary.BigEndian.PutUint16(aspValue[2:], 65001)
	attrs = append(attrs, 0x40, 0x02, byte(len(aspValue)))
	attrs = append(attrs, aspValue...)
	attrs = append(attrs, 0x80, 0x0e, byte(len(value)))
	attrs = append(attrs, value...)
	return buildUpdatePayload(attrs, nil)
}

// nhSelfLinkLocalDest builds an established internal IPv6 destination. With
// nextHopSelf it is configured `next-hop self` under `local ip auto`, and its
// session's TCP local endpoint is nhSelfLinkLocalEndpoint. llnh says whether the
// session negotiated the Link-Local Next Hop Capability (code 77).
//
// The session is attached only while the forwarding facts are built, as in
// autoLocalSelfDest (rfc4271_third_party_nexthop_test.go), so the route-server
// rail's direct write does not take the item for a session with no connection.
func nhSelfLinkLocalDest(t *testing.T, addr string, nextHopSelf, llnh bool) *Peer {
	t.Helper()
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       netip.MustParseAddr(addr),
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65000,
		RouterID:      0x0a000001,
	}
	if nextHopSelf {
		settings.NextHopMode = NextHopSelf
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:         map[family.Family]bool{family.IPv6Unicast: true},
		LinkLocalNextHop: llnh,
	})
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	peer.llScope.Store(newLinkScopeFrom(llnhSegment, settings.Address))
	if nextHopSelf {
		session := NewSession(settings)
		session.transport.Store(&sessionTransport{local: nhSelfLinkLocalEndpoint})
		peer.session = session
	}
	peer.fwdFacts.Store(peer.buildForwardFacts())
	peer.session = nil
	return peer
}

// a2NextHopField returns the Network Address of Next Hop field of an MP_REACH_NLRI
// value, or nil when the value is absent.
func a2NextHopField(t *testing.T, mpReach []byte) []byte {
	t.Helper()
	if len(mpReach) < 4 {
		return nil
	}
	nhLen := int(mpReach[3])
	require.GreaterOrEqual(t, len(mpReach), 4+nhLen, "the next-hop field must fit the attribute")
	return mpReach[4 : 4+nhLen]
}

// TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails drives the
// announce rail's link-local refusal (Peer.resolveNextHop) on the two forward
// rails, which build next hop self from the same connected endpoint.
//
// draft-ietf-idr-linklocal-capability Section 2: "When the capability has not
// been negotiated, the procedures in this document do not apply." The
// Link-Local-only Next Hop of its Section 3 is one of those procedures, and RFC
// 2545 Section 3 has no 16-octet link-local form, so a session that did not
// negotiate code 77 may not be sent one. Section 4: "If, after completing these
// procedures, there are no IPv6 next hop addresses included in the next hop, the
// BGP route MUST not be advertised to its peer."
//
// Method: three internal destinations receive one IPv6 route with the Global
// next hop 2001:db8:1::9, on the general rail and on the route-server rail. Two
// are configured next-hop self under `local ip auto` over a session whose local
// endpoint is fe80::77; one negotiated capability 77, one did not. The third
// keeps the default next-hop mode.
//
// VALIDATES: the destination that negotiated capability 77 is sent the 16-octet
// Link-Local-only Next Hop fe80::77; the one that did not is sent nothing; the
// default destination is sent the route with the received Global next hop, so
// the route itself was forwardable.
// PREVENTS: the forward rails sending a link-local next hop self that the
// announce rail refuses for the same session.
func TestNextHopSelfLinkLocalEndpointNeedsTheCapabilityOnTheForwardRails(t *testing.T) {
	permitted := netip.MustParseAddr("2001:db8:1::31")
	refused := netip.MustParseAddr("2001:db8:1::32")
	passing := netip.MustParseAddr("2001:db8:1::33")
	for _, rs := range []bool{false, true} {
		got := a2Forward(t, rs, nhSelfLinkLocalPayload(),
			nhSelfLinkLocalDest(t, permitted.String(), true /*nextHopSelf*/, true /*llnh*/),
			nhSelfLinkLocalDest(t, refused.String(), true /*nextHopSelf*/, false /*llnh*/),
			nhSelfLinkLocalDest(t, passing.String(), false /*nextHopSelf*/, false /*llnh*/))

		toPermitted, ok := got[permitted]
		require.True(t, ok, "rs=%v: the destination that negotiated code 77 is owed the route", rs)
		require.Equal(t, nhSelfLinkLocalEndpoint.AsSlice(), a2NextHopField(t, toPermitted.mpReach),
			"rs=%v: next hop self is the 16-octet Link-Local-only form", rs)

		toRefused, ok := got[refused]
		if ok {
			require.Empty(t, toRefused.mpReach, "rs=%v: no announcement without code 77", rs)
		}

		toPassing, ok := got[passing]
		require.True(t, ok, "rs=%v: the default destination is owed the route", rs)
		require.Equal(t, nhSelfReceivedNextHop.AsSlice(), a2NextHopField(t, toPassing.mpReach),
			"rs=%v: the default destination is sent the received Global next hop", rs)
	}
}
