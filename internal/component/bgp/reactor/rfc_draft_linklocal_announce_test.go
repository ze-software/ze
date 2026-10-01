// Design: docs/architecture/bgp/structural-forwarding.md -- the next hop an originated route carries
// RFC: rfc/short/draft-ietf-idr-linklocal-capability.md
// Related: peer_static_wire.go -- sendStaticRoutes, the announce rail of a configured route
// Related: link_scope.go -- linkLocalNextHopFor, which decides the own Link-Local half
// Overview: rfc_draft_linklocal_test.go -- the same Section 4 sentence on the forward rail
//
// draft-ietf-idr-linklocal-capability Section 4: "If the route is directly
// connected to the speaker, or if the interface address of the router through
// which the announced network is reachable for the speaker is the internal
// peer's address, the next hop MUST include its own Link-Local IPv6 address."
// These tests originate a configured route on the announce rail toward a one-hop
// internal peer and read the MP_REACH_NLRI next hop the peer's connection
// received.
package reactor

import (
	"bufio"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
)

// llAnnounceNLRI is 2001:db8:77::/64 in the RFC 4760 prefix form.
var llAnnounceNLRI = []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x77, 0x00, 0x00}

// newOneHopInternalPeer returns an Established internal peer (AS 65000 both
// ends) at 2001:db8:1::2, one hop away: the speaker's own global 2001:db8:1::1
// and the peer share the connected subnet 2001:db8:1::/64, and the speaker's own
// Link-Local is fe80::1. The interface table is given, not read, so the test does
// not depend on the host's addresses.
func newOneHopInternalPeer(t *testing.T) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection:   ConnectionBoth,
		Address:      netip.MustParseAddr("2001:db8:1::2"),
		LocalAddress: netip.MustParseAddr("2001:db8:1::1"),
		LinkLocal:    netip.MustParseAddr("fe80::1"),
		LocalAS:      65000,
		PeerAS:       65000,
		RouterID:     0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{family.IPv6Unicast: true}})

	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()
	// The TCP local endpoint connectedTransport stores at connect: next hop self
	// resolves to it (connectedLocalAddress).
	session.transport.Store(&sessionTransport{local: settings.LocalAddress})

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	peer.refreshLinkScopeFrom([]netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")})
	return peer, conn
}

// TestLinkLocalOwnAddressIncludedForDirectlyConnectedOriginatedRoute drives the
// first condition of the sentence on the announce rail.
//
// VALIDATES: a configured route originated with next hop self (the speaker is
// the router the network is reachable through: it is directly connected) is
// sent to the one-hop internal peer with the speaker's global first and its own
// Link-Local second, length octet 32.
// PREVENTS: an originated directly connected route reaching a one-hop internal
// peer with the global address alone.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3 positive -- a locally originated route directly connected to the speaker (static route, next hop self) sent through sendStaticRoutes to a one-hop internal peer carries the 32-octet next hop: the speaker's global 2001:db8:1::1 followed by its own Link-Local fe80::1.
func TestLinkLocalOwnAddressIncludedForDirectlyConnectedOriginatedRoute(t *testing.T) {
	peer, conn := newOneHopInternalPeer(t)
	route := StaticRoute{Prefix: netip.MustParsePrefix("2001:db8:77::/64"), NextHop: bgptypes.NewNextHopSelf()}

	sent := peer.sendStaticRoutes(peer.currentSession(), []StaticRoute{route}, false, 4096, false)

	require.Len(t, sent, 1, "the route reached the wire")
	assert.Contains(t, string(conn.written()), string(mpReachIPv6Attr(t, llAnnounceNLRI, "2001:db8:1::1", "fe80::1")),
		"own global first, own Link-Local second, length octet 0x20")
}

// TestLinkLocalOwnAddressNotIncludedForRouteThroughAnotherRouter is the other
// branch of the condition: the network is reachable through a router that is
// neither the speaker nor the internal peer.
//
// VALIDATES: the same rail sends that route with the other router's global
// alone (length octet 16): the speaker's own Link-Local is not attached to a
// next hop that is not the speaker.
// PREVENTS: an encoder that appends its own Link-Local to every IPv6 next hop
// toward a one-hop peer, which would pass the positive test above.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3 negative -- a locally originated route reachable through another router (explicit next hop 2001:db8:dead::9, neither the speaker nor the internal peer, off the shared link) sent through sendStaticRoutes to the same one-hop internal peer carries that router's global alone, never followed by the speaker's own Link-Local fe80::1.
func TestLinkLocalOwnAddressNotIncludedForRouteThroughAnotherRouter(t *testing.T) {
	peer, conn := newOneHopInternalPeer(t)
	route := staticRouteAt("2001:db8:77::/64", "2001:db8:dead::9")

	sent := peer.sendStaticRoutes(peer.currentSession(), []StaticRoute{route}, false, 4096, false)

	require.Len(t, sent, 1, "the route reached the wire")
	written := string(conn.written())
	assert.Contains(t, written, string(mpReachIPv6Attr(t, llAnnounceNLRI, "2001:db8:dead::9")),
		"the other router's global alone, length octet 0x10")
	assert.NotContains(t, written, string(mpReachIPv6Attr(t, llAnnounceNLRI, "2001:db8:dead::9", "fe80::1")),
		"the speaker's own Link-Local is not attached to another router's next hop")
}

// mpReachNextHopField finds the one MP_REACH_NLRI attribute (flags 0x80, type
// 14, AFI 2, SAFI 1) in what the peer's connection received and returns its
// Length of Next Hop Network Address octet and the next-hop octets that octet
// announces. It fails the test when no such attribute was written, or when the
// octets the length claims run past the end of the stream.
func mpReachNextHopField(t *testing.T, written []byte) (byte, []byte) {
	t.Helper()
	header := []byte{0x80, 0x0E}
	for i := 0; i+7 <= len(written); i++ {
		if written[i] != header[0] || written[i+1] != header[1] {
			continue
		}
		// written[i+2] is the attribute length; the value opens with AFI(2) SAFI(1).
		if written[i+3] != 0x00 || written[i+4] != 0x02 || written[i+5] != 0x01 {
			continue
		}
		length := written[i+6]
		start := i + 7
		require.LessOrEqual(t, start+int(length), len(written), "the announced next-hop octets are all on the wire")
		return length, written[start : start+int(length)]
	}
	require.FailNow(t, "no IPv6 unicast MP_REACH_NLRI attribute was written")
	return 0, nil
}

// TestLinkLocalBothAddressesSetTheWireLengthOctetToThirtyTwo reads the Length
// octet the encoder wrote, not a field of the precomputed facts.
//
// VALIDATES: a route sent with both the speaker's global and its own Link-Local
// (next hop self toward the one-hop internal peer) leaves with the Length of
// Next Hop Network Address octet at 0x20, followed by exactly the global
// 2001:db8:1::1 and then the Link-Local fe80::1.
// PREVENTS: an encoder that writes both addresses under a 16-octet length, or a
// 32-octet length around anything but the two addresses.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2 positive -- a route sent through sendStaticRoutes with both the speaker's global 2001:db8:1::1 and its own Link-Local fe80::1 is written with the MP_REACH_NLRI Length of Next Hop Network Address octet at 32 (0x20), and the 32 octets it announces are the global followed by the Link-Local.
func TestLinkLocalBothAddressesSetTheWireLengthOctetToThirtyTwo(t *testing.T) {
	peer, conn := newOneHopInternalPeer(t)
	route := StaticRoute{Prefix: netip.MustParsePrefix("2001:db8:77::/64"), NextHop: bgptypes.NewNextHopSelf()}

	sent := peer.sendStaticRoutes(peer.currentSession(), []StaticRoute{route}, false, 4096, false)

	require.Len(t, sent, 1, "the route reached the wire")
	length, field := mpReachNextHopField(t, conn.written())
	assert.Equal(t, byte(32), length, "the Length of Next Hop octet is 32")
	global := netip.MustParseAddr("2001:db8:1::1").As16()
	linkLocal := netip.MustParseAddr("fe80::1").As16()
	assert.Equal(t, append(global[:], linkLocal[:]...), field, "the global first, the Link-Local second")
}

// TestLinkLocalSingleAddressSetsTheWireLengthOctetToSixteen is the other side:
// a next hop the speaker does not own carries no Link-Local of the speaker's.
//
// VALIDATES: the same rail, for a route through another router
// (2001:db8:dead::9), writes the Length octet 0x10 followed by that global
// alone: the 32-octet length is tied to sending both addresses.
// PREVENTS: an encoder that writes 32 for every IPv6 next hop toward a one-hop
// peer, which the positive test above would not catch.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2 negative -- a route sent through sendStaticRoutes with one address only (another router's global 2001:db8:dead::9) is written with the MP_REACH_NLRI Length of Next Hop Network Address octet at 16 (0x10), never 32, and the 16 octets it announces are that global.
func TestLinkLocalSingleAddressSetsTheWireLengthOctetToSixteen(t *testing.T) {
	peer, conn := newOneHopInternalPeer(t)
	route := staticRouteAt("2001:db8:77::/64", "2001:db8:dead::9")

	sent := peer.sendStaticRoutes(peer.currentSession(), []StaticRoute{route}, false, 4096, false)

	require.Len(t, sent, 1, "the route reached the wire")
	length, field := mpReachNextHopField(t, conn.written())
	assert.Equal(t, byte(16), length, "one address keeps the Length of Next Hop octet at 16")
	other := netip.MustParseAddr("2001:db8:dead::9").As16()
	assert.Equal(t, other[:], field, "the other router's global is the whole field")
}

// TestLinkLocalSecondConditionNeverReachesTheWire drives the draft's second
// condition on the announce rail: the interface address of the router through
// which the network is reachable is the internal peer's own address.
//
// The route never reaches the wire: RFC 4271 Section 5.1.3 says "A route
// originated by a BGP speaker SHALL NOT be advertised to a peer using an
// address of that peer as NEXT_HOP", and the write boundary withholds it
// (originatedNextHopIsPeerOwn, session_write.go). The draft condition's
// antecedent is therefore a route Ze is forbidden to send, so its row
// DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-15 is annotated {not-applicable} and
// names this test. UNTAGGED on purpose: `./le rfc check` refuses a tag beside
// a {not-applicable} annotation.
//
// VALIDATES: nothing is written to the peer for a route whose explicit next
// hop is the internal peer's own address 2001:db8:1::2.
// PREVENTS: a tag claiming a wire form no rail sends.
func TestLinkLocalSecondConditionNeverReachesTheWire(t *testing.T) {
	peer, conn := newOneHopInternalPeer(t)
	route := staticRouteAt("2001:db8:77::/64", "2001:db8:1::2")

	peer.sendStaticRoutes(peer.currentSession(), []StaticRoute{route}, false, 4096, false)

	assert.Empty(t, conn.written(), "a route naming the peer's own address as next hop is withheld")
}
