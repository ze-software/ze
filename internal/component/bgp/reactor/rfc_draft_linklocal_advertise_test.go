// Conformance tests for draft-ietf-idr-linklocal-capability Section 4, the rules
// that decide whether a route is advertised AT ALL when the next-hop procedures
// leave no IPv6 next hop address to carry.
//
// The draft text is rfc/drafts/draft-ietf-idr-linklocal-capability.txt and the
// extracted checklist is rfc/short/draft-ietf-idr-linklocal-capability.md. The
// next-hop FORM rules of the same section are proven in
// rfc_draft_linklocal_test.go, over the forwarding facts; these drive the rail
// that turns a stored route into an UPDATE, because "the route MUST NOT be
// announced" is a decision about the whole message rather than about a field.
//
// Section 2 scopes every procedure in Sections 3 to 6 to a session on which the
// capability was negotiated. Each rule proven here is enforced by ze on EVERY
// session, so it holds on a negotiated one as well.

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// llnhAdvertise returns the UPDATE the RIB-replay rail builds for one IPv6
// unicast route toward one peer, or nil when the rail refuses to advertise the
// route at all.
//
// isIBGP selects the internal-peer rail or the external-peer one, which is the
// division Section 4 makes. nextHop is the only other thing that varies: the
// zero netip.Addr is the state Section 4 calls "no IPv6 next hop addresses
// included in the next hop".
func llnhAdvertise(isIBGP bool, nextHop netip.Addr) *message.Update {
	route := rib.NewRouteWithASPath(
		nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8:7::/64"), 0),
		nextHop, nil, nil)
	return buildRIBRouteUpdate(make([]byte, message.MaxMsgLen), route, 65000,
		isIBGP, true /*asn4*/, false /*addPath*/)
}

// llnhNextHopField returns the MP_REACH Network Address of Next Hop field of an
// UPDATE the rail built, so a test can read its length and the addresses in it.
func llnhNextHopField(t *testing.T, update *message.Update) []byte {
	t.Helper()
	require.NotNil(t, update, "the rail must have built an UPDATE")
	_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	require.True(t, found, "MP_REACH_NLRI must be present")
	require.GreaterOrEqual(t, len(value), 4, "AFI, SAFI and the next-hop length octet")
	nhLen := int(value[3])
	require.GreaterOrEqual(t, len(value), 4+nhLen, "the next-hop field must fit the attribute")
	return value[4 : 4+nhLen]
}

// llnhHasLocalPref reports whether the UPDATE carries LOCAL_PREF, which is what
// tells the internal-peer rail from the external one: localPrefAllowedTo
// (forward_local_pref.go) keeps the attribute off an external session (RFC 4271
// Section 5.1.5).
func llnhHasLocalPref(update *message.Update) bool {
	_, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttrLocalPref)
	return found
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1 positive -- a route left with no
// IPv6 next hop address after the next-hop procedures is not advertised to its peer:
// the MP_REACH next hop has no wire form, attribute.ValidateNextHops refuses it, and
// buildRIBRouteUpdate returns no UPDATE at all. Ze reads the next hop alone here, so the
// refusal holds for the internal peer and for the external one, which together are the
// whole population Section 4 divides.
func TestLinkLocalRouteWithNoNextHopIsNotAdvertised(t *testing.T) {
	assert.Nil(t, llnhAdvertise(true, netip.Addr{}),
		"no UPDATE may be built toward an internal peer")
	assert.Nil(t, llnhAdvertise(false, netip.Addr{}),
		"no UPDATE may be built toward an external peer")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1 negative -- that suppression is
// keyed on the next hop being absent and is not a blanket refusal: the same route, the same
// rail and the same two peers, with one IPv6 next hop address included, produce an UPDATE
// whose Next Hop field carries that address in 16 octets.
func TestLinkLocalRouteWithANextHopIsAdvertised(t *testing.T) {
	for _, isIBGP := range []bool{true, false} {
		field := llnhNextHopField(t, llnhAdvertise(isIBGP, llnhGlobal))

		assert.Len(t, field, 16, "one IPv6 next hop address fills 16 octets")
		assert.Equal(t, llnhGlobal.As16(), [16]byte(field), "the included address is the one sent")
	}
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4 positive -- when the internal-peer
// procedures leave no IPv6 next hop included with the route, the route is not announced to the
// remote BGP speaker: buildRIBRouteUpdate on the internal-peer rail builds no UPDATE, so
// nothing reaches that speaker.
func TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnInternalPeer(t *testing.T) {
	assert.Nil(t, llnhAdvertise(true, netip.Addr{}),
		"an internal peer receives no UPDATE for a route with no IPv6 next hop")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4 negative -- that refusal is keyed on
// the missing next hop rather than on the peer being internal: the same internal-peer rail
// announces the same route once an IPv6 next hop is included, and the UPDATE it builds carries
// LOCAL_PREF, which is the attribute only the internal-peer rail contributes.
func TestLinkLocalRouteWithANextHopIsAnnouncedToAnInternalPeer(t *testing.T) {
	update := llnhAdvertise(true, llnhGlobal)

	field := llnhNextHopField(t, update)
	assert.Equal(t, llnhGlobal.As16(), [16]byte(field), "the internal peer gets the next hop")
	assert.True(t, llnhHasLocalPref(update), "the internal-peer rail contributes LOCAL_PREF")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7 positive -- toward an external peer,
// where the default is the address of the interface the speaker established the connection on,
// a route with no next hop included is not announced: buildRIBRouteUpdate on the external-peer
// rail builds no UPDATE.
func TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnExternalPeer(t *testing.T) {
	assert.Nil(t, llnhAdvertise(false, netip.Addr{}),
		"an external peer receives no UPDATE for a route with no next hop")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7 negative -- the external peer is not
// refused everything: the same rail announces the same route once a next hop is included, and
// the UPDATE carries no LOCAL_PREF, which is what shows the external-peer rail built it.
func TestLinkLocalRouteWithANextHopIsAnnouncedToAnExternalPeer(t *testing.T) {
	update := llnhAdvertise(false, llnhGlobal)

	field := llnhNextHopField(t, update)
	assert.Equal(t, llnhGlobal.As16(), [16]byte(field), "the external peer gets the next hop")
	assert.False(t, llnhHasLocalPref(update), "the external-peer rail contributes no LOCAL_PREF")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9 positive -- toward an external peer
// multiple IP hops away, no Link-Local IPv6 next hop is included (newLinkScopeFrom finds no
// connected subnet holding that peer), so the Global IPv6 next hop is the only address the
// route could carry; without one, buildRIBRouteUpdate builds no UPDATE and the route is not
// advertised to that peer.
func TestLinkLocalRouteWithNoGlobalNextHopIsNotAdvertisedToAMultihopExternalPeer(t *testing.T) {
	facts := llnhFacts(llnhConnected, llnhOffLink, llnhGlobal, llnhLinkLocal)
	require.Equal(t, nhModeSelfV6, facts.nhMode, "a multihop peer gets no Link-Local next hop")
	require.Equal(t, [32]byte{}, facts.nhGlobalLL, "no Link-Local address is included")

	assert.Nil(t, llnhAdvertise(false, netip.Addr{}),
		"with no Global IPv6 next hop there is nothing left to advertise")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9 negative -- the multihop external
// peer is refused only for want of a Global IPv6 next hop: the same peer and the same rail
// advertise the route when one is included, in a 16-octet Next Hop field holding that global
// address.
func TestLinkLocalRouteWithAGlobalNextHopIsAdvertisedToAMultihopExternalPeer(t *testing.T) {
	facts := llnhFacts(llnhConnected, llnhOffLink, llnhGlobal, llnhLinkLocal)
	require.Equal(t, nhModeSelfV6, facts.nhMode, "a multihop peer gets no Link-Local next hop")

	field := llnhNextHopField(t, llnhAdvertise(false, llnhGlobal))
	assert.Len(t, field, 16, "the Global IPv6 next hop is the whole field")
	assert.Equal(t, llnhGlobal.As16(), [16]byte(field), "the Global address is what is carried")
}
