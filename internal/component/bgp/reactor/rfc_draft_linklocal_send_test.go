// Conformance tests for draft-ietf-idr-linklocal-capability Sections 3 and 5,
// the two rules that decide the LENGTH of the MP_REACH Network Address of Next
// Hop field and which addresses go in it.
//
// The draft text is rfc/drafts/draft-ietf-idr-linklocal-capability.txt and the
// extracted checklist is rfc/short/draft-ietf-idr-linklocal-capability.md. The
// Section 4 rules over the same forms are proven in rfc_draft_linklocal_test.go
// and rfc_draft_linklocal_advertise_test.go.
//
// Section 2 scopes every procedure in Sections 3 to 6 to a session on which the
// capability was negotiated, so the session half of each rule is driven through
// Peer.linkLocalOnlyNextHopPermitted (peer.go), which reads what capability 77
// and RFC 8950 Extended Next Hop Encoding negotiated to.

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// llnhBuildUnicast builds the UPDATE one unicast route produces toward a peer,
// with the Next Hop field the two addresses given ask for.
//
// linkLocal is the second address of the RFC 2545 Section 3 pair; the zero Addr
// asks for a field carrying nextHop alone. extendedNextHop is RFC 8950's
// cross-family encoding, which is what carries an IPv4 prefix behind an IPv6
// next hop.
func llnhBuildUnicast(t *testing.T, prefix string, nextHop, linkLocal netip.Addr, extendedNextHop bool) *message.Update {
	t.Helper()
	ub := message.GetUpdateBuilder(65000, false /*isIBGP*/, true /*asn4*/, false /*addPath*/)
	t.Cleanup(func() { message.PutUpdateBuilder(ub) })
	return ub.BuildUnicast(&message.UnicastParams{
		Prefix:             netip.MustParsePrefix(prefix),
		NextHop:            nextHop,
		LinkLocalNextHop:   linkLocal,
		Origin:             attribute.OriginIGP,
		UseExtendedNextHop: extendedNextHop,
	})
}

// llnhPeer builds a peer whose session negotiated the capabilities named.
//
// llnh is capability 77 (draft-ietf-idr-linklocal-capability Section 2) and
// extendedNextHop is RFC 8950 Extended Next Hop Encoding for IPv4 unicast, which
// Section 5 makes the second half of the combination it names.
func llnhPeer(t *testing.T, llnh, extendedNextHop bool) *Peer {
	t.Helper()
	peer := NewPeer(&PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("2001:db8:1::2"),
		LocalAS:    65000,
		PeerAS:     65000,
	})
	peer.negotiated.Store(&NegotiatedCapabilities{LinkLocalNextHop: llnh})
	extNH := map[family.Family]family.AFI{}
	if extendedNextHop {
		extNH[family.IPv4Unicast] = family.AFIIPv6
	}
	peer.sendCtx.Store(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{},
		&capability.EncodingCaps{ASN4: true, ExtendedNextHop: extNH},
		bgpctx.DirectionSend))
	return peer
}

// llnhLinkLocalNextHop is the route next hop an operator configures to send a
// single IPv6 Link-Local forwarding address.
var llnhLinkLocalNextHop = bgptypes.RouteNextHop{
	Policy: bgptypes.NextHopExplicit,
	Addr:   netip.MustParseAddr("fe80::1"),
}

// VALIDATES: a route ze intends to send with one IPv6 Link-Local forwarding
// address leaves with a 16-octet Next Hop field holding that address alone.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1 positive -- "If an
// implementation intends to send a single IPv6 Link-Local forwarding address in the
// Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field
// to 16 and include only the IPv6 Link-Local address in the Next Hop field" (Section
// 3). buildMPReach (message/update_build.go) writes the one address it was given, so
// the Length of Next Hop Network Address octet reads 16 and the field is the
// link-local address and nothing else. The session half is
// Peer.linkLocalOnlyNextHopPermitted (peer.go): a session that negotiated capability
// 77 admits that address, which is what lets the route reach this form at all.
//
// PREVENTS: the form ze advertises capability 77 for and could not produce. Before
// this, resolveNextHop handed the address on with no session test and no producer
// ever chose the 16-octet link-local-only field deliberately.
func TestLinkLocalOnlyNextHopFieldIsSixteenOctets(t *testing.T) {
	peer := llnhPeer(t, true /*llnh*/, false /*extendedNextHop*/)

	nextHop, err := peer.resolveNextHop(peer.session, llnhLinkLocalNextHop, family.IPv6Unicast)
	require.NoError(t, err, "a negotiated session admits a link-local next hop")

	field := llnhNextHopField(t, llnhBuildUnicast(t, "2001:db8:7::/64", nextHop, netip.Addr{}, false))

	assert.Len(t, field, 16, "the Next Hop field length is 16")
	assert.Equal(t, netip.MustParseAddr("fe80::1").As16(), [16]byte(field),
		"the field holds only the IPv6 Link-Local address")
	assert.True(t, attribute.IsLinkLocalOnlyNextHop(field),
		"the field is a Link-Local-only Next Hop as Section 3 defines it")
}

// VALIDATES: the 16-octet link-local-only field is written only where ze intends a
// SINGLE link-local address, not wherever a link-local address is involved.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1 negative -- the same
// session and the same link-local address, with a Global IPv6 address also to be
// sent, produce the 32-octet field of Section 3's next paragraph rather than a
// 16-octet one holding the link-local address alone. The requirement is keyed on the
// sender's intent to send ONE address, and this is the case where it intends two.
//
// PREVENTS: a producer that read "link-local" and always wrote 16 octets, which
// would drop the Global address every RFC 2545 Section 3 peer expects first.
func TestLinkLocalNextHopWithAGlobalIsNotSentAlone(t *testing.T) {
	field := llnhNextHopField(t, llnhBuildUnicast(t, "2001:db8:7::/64",
		llnhGlobal, llnhLinkLocal, false))

	require.Len(t, field, 32, "two addresses fill 32 octets")
	assert.Equal(t, llnhGlobal.As16(), [16]byte(field[:16]), "the Global address comes first")
	assert.Equal(t, llnhLinkLocal.As16(), [16]byte(field[16:]), "the Link-Local address comes second")
	assert.False(t, attribute.IsLinkLocalOnlyNextHop(field),
		"a 32-octet field is never the Link-Local-only form")
}

// VALIDATES: IPv4 NLRI carried with an IPv6 next hop leaves with 32 octets on a
// session that did not negotiate the combination, and the link-local address is
// carried rather than dropped.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1 positive -- "When this
// combination has not been negotiated, a sender MUST follow the rules in Section 3 of
// [RFC8950] and encode the Next Hop as 32 octets" (Section 5). The session negotiated
// RFC 8950 Extended Next Hop Encoding and NOT capability 77, so
// Peer.linkLocalOnlyNextHopPermitted (peer.go) refuses the 16-octet Link-Local-only
// field for this IPv4 NLRI, and buildMPReach (message/update_build.go) writes the
// Global address followed by the Link-Local one in a field of 32 octets.
//
// PREVENTS: the 16-octet field this sentence forbids outside the combination.
// buildMPReach keyed the second address on the PREFIX being IPv6, so every IPv4 NLRI
// with an IPv6 next hop lost its link-local address and went out in 16 octets.
func TestIPv4NLRINextHopIsThirtyTwoOctetsWithoutTheCombination(t *testing.T) {
	peer := llnhPeer(t, false /*llnh*/, true /*extendedNextHop*/)

	require.False(t, peer.linkLocalOnlyNextHopPermitted(family.IPv4Unicast),
		"without capability 77 the combination is not negotiated")
	_, err := peer.resolveNextHop(peer.session, llnhLinkLocalNextHop, family.IPv4Unicast)
	require.ErrorIs(t, err, ErrNextHopLinkLocalOnly,
		"a Link-Local-only Next Hop is refused for IPv4 NLRI outside the combination")

	field := llnhNextHopField(t, llnhBuildUnicast(t, "192.0.2.0/24",
		llnhGlobal, llnhLinkLocal, true /*extendedNextHop*/))

	assert.Len(t, field, 32, "the Next Hop is encoded as 32 octets")
	assert.Equal(t, llnhGlobal.As16(), [16]byte(field[:16]), "the Global IPv6 address comes first")
	assert.Equal(t, llnhLinkLocal.As16(), [16]byte(field[16:]), "the Link-Local address follows it")
}

// VALIDATES: the 32-octet rule is keyed on the combination being absent, and the
// 16-octet Link-Local-only field is sent for IPv4 NLRI once both capabilities are
// negotiated.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1 negative -- "When both the
// Link-Local Next Hop Capability defined in this document and the Extended Next Hop
// Encoding capability ([RFC8950]) have been negotiated between two peers, a 16-octet
// Link-Local-only Next Hop is permitted for IPv4 NLRI carried with an IPv6 Next Hop"
// (Section 5). The same IPv4 prefix and the same address, on a session that
// negotiated both, are admitted by Peer.linkLocalOnlyNextHopPermitted (peer.go) and
// encoded in 16 octets.
//
// PREVENTS: a fix that forced 32 octets on every IPv4 NLRI, which would deny the
// form Section 5 permits and make the positive half above vacuous.
func TestIPv4NLRILinkLocalOnlyNextHopNeedsTheCombination(t *testing.T) {
	peer := llnhPeer(t, true /*llnh*/, true /*extendedNextHop*/)

	require.True(t, peer.linkLocalOnlyNextHopPermitted(family.IPv4Unicast),
		"both capabilities negotiated is the combination Section 5 names")
	nextHop, err := peer.resolveNextHop(peer.session, llnhLinkLocalNextHop, family.IPv4Unicast)
	require.NoError(t, err, "the combination admits a Link-Local-only Next Hop for IPv4 NLRI")

	field := llnhNextHopField(t, llnhBuildUnicast(t, "192.0.2.0/24",
		nextHop, netip.Addr{}, true /*extendedNextHop*/))

	assert.Len(t, field, 16, "the permitted form is 16 octets")
	assert.True(t, attribute.IsLinkLocalOnlyNextHop(field),
		"the field holds the Link-Local address alone")
}
