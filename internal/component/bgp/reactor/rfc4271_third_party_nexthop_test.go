// Design: docs/architecture/route-selection.md -- the next hop a forwarded route leaves with
// Related: peer_forward_facts.go -- precomputeNextHop and applyFactsNextHop, the next-hop-self rewrite
// Related: reactor_a2_rfc8950_forward_test.go -- a2Forward and a2Parts, the harness

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// a2ThirdPartyNextHop is the NEXT_HOP a2InlinePayload carries: 192.0.2.254, an
// address that is neither the forwarding speaker's nor the destination's, so
// toward an internal peer it is a third-party next hop.
var a2ThirdPartyNextHop = []byte{192, 0, 2, 254}

// TestRFC4271NextHopSelfDisablesThirdPartyNextHopOnTheWire drives RFC 4271
// Section 5.1.3: "A BGP speaker MUST be able to support the disabling
// advertisement of third party NEXT_HOP attributes in order to handle
// imperfectly bridged media."
//
// Method: a route learned from an external peer with NEXT_HOP 192.0.2.254 is
// forwarded on the general rail to two internal destinations in one fan-out.
// One is configured `next-hop self` with local address 10.0.0.254; the other
// keeps the default, under which an internal peer is sent the received next hop.
//
// VALIDATES: the next-hop-self destination is sent NEXT_HOP 10.0.0.254, so the
// third-party address does not reach it; the default destination in the same
// fan-out is sent 192.0.2.254, which shows the route did carry a third-party
// next hop and the setting is what removed it.
// PREVENTS: next-hop self armed in the forwarding facts but never reaching the
// NEXT_HOP the destination is sent.
//
// RFC requirement: RFC4271-5.1.3-3 positive -- an internal destination configured next-hop self with a local address is sent that local address as NEXT_HOP on the general forward rail, while a default internal destination in the same fan-out is sent the received third-party NEXT_HOP.
func TestRFC4271NextHopSelfDisablesThirdPartyNextHopOnTheWire(t *testing.T) {
	self := a2Dest(t, "192.0.2.61", 65000, netip.Addr{}, false)
	self.settings.NextHopMode = NextHopSelf
	self.refreshForwardFacts()
	passing := a2Dest(t, "192.0.2.62", 65000, netip.Addr{}, false)

	got := a2Forward(t, false, a2InlinePayload(), self, passing)

	toSelf, ok := got[netip.MustParseAddr("192.0.2.61")]
	require.True(t, ok, "the next-hop-self destination is owed the route")
	require.Equal(t, a2InlinePrefix, toSelf.nlri)
	require.Equal(t, []byte{10, 0, 0, 254}, toSelf.nextHop, "next-hop self replaces the third-party NEXT_HOP")

	toPassing, ok := got[netip.MustParseAddr("192.0.2.62")]
	require.True(t, ok, "the default destination is owed the route")
	require.Equal(t, a2InlinePrefix, toPassing.nlri)
	require.Equal(t, a2ThirdPartyNextHop, toPassing.nextHop, "the default internal destination is sent the received next hop")
}
