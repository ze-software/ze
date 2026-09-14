// Conformance tests for draft-ietf-idr-linklocal-capability, the Link-Local Next
// Hop Capability (code 77), over the next-hop forms ze actually produces.
//
// The draft text is rfc/drafts/draft-ietf-idr-linklocal-capability.txt and the
// extracted checklist is rfc/short/draft-ietf-idr-linklocal-capability.md.
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
)

// llnhFacts runs the next-hop precompute and the Section 3 upgrade over one peer,
// and returns the forwarding facts that decide the MP_REACH Next Hop field.
//
// connected is the host interface table the speaker reads, peer is the address
// the route is being advertised to, and linkLocal is the operator's configured
// link-local address for the session.
func llnhFacts(connected []netip.Prefix, peer, global, linkLocal netip.Addr) *peerForwardFacts {
	settings := &PeerSettings{
		Address:      peer,
		LocalAddress: global,
		LinkLocal:    linkLocal,
		NextHopMode:  NextHopSelf,
	}
	facts := &peerForwardFacts{}
	precomputeNextHop(settings, facts)
	applyLinkLocalNextHop(settings, facts, newLinkScopeFrom(connected, peer))
	return facts
}

var (
	llnhConnected = []netip.Prefix{netip.MustParsePrefix("2001:db8:1::/64")}
	llnhGlobal    = netip.MustParseAddr("2001:db8:1::1")
	llnhLinkLocal = netip.MustParseAddr("fe80::1")
	llnhOnLink    = netip.MustParseAddr("2001:db8:1::2")
	llnhOffLink   = netip.MustParseAddr("2001:db8:9::2")
)

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2 positive -- when ze sends both a
// Global and a Link-Local forwarding address, the Next Hop field is 32 octets long and holds
// both: applyLinkLocalNextHop raises the wire form to nhModeSelfV6LL and writes the global
// address in the first 16 octets and the link-local address in the second 16.
func TestLinkLocalBothAddressesUseThirtyTwoOctets(t *testing.T) {
	facts := llnhFacts(llnhConnected, llnhOnLink, llnhGlobal, llnhLinkLocal)

	require.Equal(t, nhModeSelfV6LL, facts.nhMode, "both addresses must use the two-address form")
	assert.Len(t, facts.nhGlobalLL[:], 32, "the Next Hop field must be 32 octets")
	assert.Equal(t, llnhGlobal.As16(), [16]byte(facts.nhGlobalLL[:16]), "the Global address comes first")
	assert.Equal(t, llnhLinkLocal.As16(), [16]byte(facts.nhGlobalLL[16:]), "the Link-Local address comes second")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2 negative -- the 32-octet form is
// written only when ze really intends to send both addresses: with no link-local address
// configured, the Next Hop field stays the single-address form rather than being padded to
// 32 octets around a second address ze does not have.
func TestLinkLocalSingleAddressKeepsSixteenOctets(t *testing.T) {
	facts := llnhFacts(llnhConnected, llnhOnLink, llnhGlobal, netip.Addr{})

	require.Equal(t, nhModeSelfV6, facts.nhMode, "one address must keep the single-address form")
	assert.Equal(t, llnhGlobal.As16(), facts.nhGlobal, "the Global address is the whole field")
	assert.Equal(t, [32]byte{}, facts.nhGlobalLL, "no 32-octet field is built")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2 positive -- a peer more than one
// IP hop away gets no Link-Local IPv6 next hop: newLinkScopeFrom finds no connected subnet
// holding the peer's address, so linkScope.linkLocalNextHop returns nothing and the Next Hop
// field carries the Global address alone.
func TestLinkLocalNotIncludedForPeerMoreThanOneHopAway(t *testing.T) {
	facts := llnhFacts(llnhConnected, llnhOffLink, llnhGlobal, llnhLinkLocal)

	assert.Equal(t, nhModeSelfV6, facts.nhMode, "an off-link peer must not get the two-address form")
	assert.Equal(t, [32]byte{}, facts.nhGlobalLL, "no Link-Local address is included")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2 negative -- that exclusion is
// keyed on the hop distance and is not a blanket refusal: the same speaker, the same
// configured link-local address and a peer that IS one IP hop away does include the
// Link-Local IPv6 next hop.
func TestLinkLocalIncludedForPeerOneHopAway(t *testing.T) {
	facts := llnhFacts(llnhConnected, llnhOnLink, llnhGlobal, llnhLinkLocal)

	require.Equal(t, nhModeSelfV6LL, facts.nhMode, "an on-link peer does get the two-address form")
	assert.Equal(t, llnhLinkLocal.As16(), [16]byte(facts.nhGlobalLL[16:]), "the Link-Local address is included")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 positive -- a multihop EBGP peer,
// which is multiple IP hops away from the speaker, gets no Link-Local IPv6 next hop: the
// same connected-subnet test refuses an external peer address that no interface subnet
// holds, whatever the peer's AS.
func TestLinkLocalNotIncludedForMultihopExternalPeer(t *testing.T) {
	multihop := netip.MustParseAddr("2001:db8:ff::1")

	facts := llnhFacts(llnhConnected, multihop, llnhGlobal, llnhLinkLocal)

	assert.Equal(t, nhModeSelfV6, facts.nhMode, "a multihop peer must not get the two-address form")
	assert.Equal(t, [32]byte{}, facts.nhGlobalLL, "no Link-Local address is included")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8 negative -- a directly attached
// external peer is not caught by that rule: it is one IP hop away, so the Link-Local IPv6
// next hop is included and the multihop exclusion is shown to be about the hop count.
func TestLinkLocalIncludedForDirectlyAttachedExternalPeer(t *testing.T) {
	facts := llnhFacts(llnhConnected, llnhOnLink, llnhGlobal, llnhLinkLocal)

	require.Equal(t, nhModeSelfV6LL, facts.nhMode, "a directly attached peer does get the two-address form")
	assert.Equal(t, llnhGlobal.As16(), [16]byte(facts.nhGlobalLL[:16]), "the Global address is still first")
}

// llnhUnchangedFacts runs the same two producers over a peer whose next-hop mode
// leaves the received next hop alone. That is the case where the announced
// network is reachable through ANOTHER router: the speaker is not the next hop,
// so no address of its own belongs in the field.
func llnhUnchangedFacts(connected []netip.Prefix, peer, global, linkLocal netip.Addr) *peerForwardFacts {
	settings := &PeerSettings{
		Address:      peer,
		LocalAddress: global,
		LinkLocal:    linkLocal,
		NextHopMode:  NextHopUnchanged,
	}
	facts := &peerForwardFacts{}
	precomputeNextHop(settings, facts)
	applyLinkLocalNextHop(settings, facts, newLinkScopeFrom(connected, peer))
	return facts
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3 positive -- a route reachable
// through the speaker itself, announced to a one-hop internal peer, carries the speaker's
// OWN Link-Local IPv6 address in the next hop: precomputeNextHop puts the speaker's own
// address in the Global slot (next-hop-self), and applyLinkLocalNextHop appends the
// link-local address configured for that session behind it.
func TestLinkLocalOwnAddressIncludedForRouteReachableThroughTheSpeaker(t *testing.T) {
	facts := llnhFacts(llnhConnected, llnhOnLink, llnhGlobal, llnhLinkLocal)

	require.Equal(t, nhModeSelfV6LL, facts.nhMode, "the next hop must carry both addresses")
	assert.Equal(t, llnhGlobal.As16(), [16]byte(facts.nhGlobalLL[:16]),
		"the speaker's own Global address is the next hop")
	assert.Equal(t, llnhLinkLocal.As16(), [16]byte(facts.nhGlobalLL[16:]),
		"the speaker's own Link-Local address is included")
}

// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3 negative -- that inclusion is keyed
// on the speaker being the next hop and is not written into every next hop: for the same peer
// on the same link, a route whose announced network is reachable through another router
// (next-hop unchanged) keeps the next hop it arrived with, and no Link-Local address of the
// speaker's own is inserted.
func TestLinkLocalOwnAddressNotInsertedWhenAnotherRouterIsTheNextHop(t *testing.T) {
	facts := llnhUnchangedFacts(llnhConnected, llnhOnLink, llnhGlobal, llnhLinkLocal)

	require.Equal(t, nhModeNone, facts.nhMode, "an unchanged next hop is rewritten by nothing")
	assert.Equal(t, [32]byte{}, facts.nhGlobalLL, "no Link-Local address of the speaker's own")
}
