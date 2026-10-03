// Design: docs/architecture/bgp/structural-forwarding.md -- the next hop an originated route carries
// Related: link_scope.go -- linkScope.linkLocalNextHop, which appends the configured Link-Local
// Related: draft_ietf_idr_linklocal_capability_announce_test.go -- newOneHopInternalPeer, the fixture
// RFC: rfc/short/rfc2545.md
//
// Written red by BGP c18 as a defect probe; green since the R55/R56 fix in
// linkScope.linkLocalNextHop (BGP c19, 2026-10-01), and now a tagged unit.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOwnLinkLocalNotPairedWithAnotherRoutersGlobalOnTheSharedLink originates a
// route whose next hop is ANOTHER router (2001:db8:1::99) on the link the
// speaker shares with the one-hop internal peer.
//
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop." The second address
// is the next hop's, so the speaker's own fe80::1 is not it: a peer resolving it
// would send ::99's traffic to the speaker. Ze does not learn ::99's Link-Local,
// so the global goes alone (the include half is the RFC2545-3-6 gap).
//
// VALIDATES: the route is sent with ::99 alone, length octet 16, and never with
// the pair [2001:db8:1::99, fe80::1].
// PREVENTS: the own Link-Local appended after every global on a connected
// subnet, the defect BGP c18 found (linkScope.linkLocalNextHop).
//
// RFC requirement: RFC2545-3-1 negative -- a locally originated route whose explicit next hop is another router on the shared link (2001:db8:1::99, neither the speaker nor the internal peer) sent through sendStaticRoutes to the one-hop internal peer carries 2001:db8:1::99 alone (length octet 16), never followed by the speaker's own Link-Local fe80::1, which is not the link-local address of that next hop.
func TestOwnLinkLocalNotPairedWithAnotherRoutersGlobalOnTheSharedLink(t *testing.T) {
	peer, conn := newOneHopInternalPeer(t)
	route := staticRouteAt("2001:db8:77::/64", "2001:db8:1::99")

	sent := peer.sendStaticRoutes(peer.currentSession(), []StaticRoute{route}, false, 4096, false)

	require.Len(t, sent, 1, "the route reached the wire")
	written := string(conn.written())
	assert.Contains(t, written, string(mpReachIPv6Attr(t, llAnnounceNLRI, "2001:db8:1::99")),
		"the other router's global alone, length octet 0x10")
	assert.NotContains(t, written, string(mpReachIPv6Attr(t, llAnnounceNLRI, "2001:db8:1::99", "fe80::1")),
		"the speaker's own Link-Local is never paired with another router's global")
}
