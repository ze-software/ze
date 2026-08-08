package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestBuildBatchAnnounceUpdate_RefusesANextHopWithNoWireForm covers the caller
// half of the MP_REACH next-hop guard.
//
// Peer.resolveNextHop (peer.go) hands an explicit next-hop back unvalidated, the
// zero netip.Addr included, and says so: "Returns invalid addr without error if
// that's what was configured." The IPv4-unicast branch of this builder has always
// guarded on IsValid; the MP_REACH branch did not, so a family other than IPv4
// unicast carried that Addr into attribute.NewMPReachNLRI.
//
// The remedy differs by branch and that is what the two sub-tests pin. IPv4
// unicast keeps the base's own NEXT_HOP and still announces the batch's prefixes,
// because those travel in the UPDATE's own NLRI field. Every other family carries
// its prefixes INSIDE MP_REACH, so skipping the contribution would announce the
// base's prefixes in place of the batch's; the whole UPDATE is refused instead.
//
// VALIDATES: an IPv6 batch with an unresolved next hop produces no UPDATE and
// errAnnounceNextHopUnencodable; the same batch with a real next hop is built.
// PREVENTS: the builder emitting an MP_REACH whose Length of Next Hop Network
// Address counts octets the writer never had (attribute.ErrUnencodableNextHop).
func TestBuildBatchAnnounceUpdate_RefusesANextHopWithNoWireForm(t *testing.T) {
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}

	build := func(t *testing.T, fam family.Family, prefix string, nextHop netip.Addr) (*message.Update, error) {
		t.Helper()
		batch := bgptypes.NLRIBatch{
			Family:  fam,
			NLRIs:   []nlri.NLRI{nlri.NewINET(fam, netip.MustParsePrefix(prefix), 0)},
			NextHop: bgptypes.NewNextHopExplicit(nextHop),
		}
		return adapter.buildBatchAnnounceUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen),
			batch, nextHop, false /*eBGP*/, false /*rsClient*/, true /*asn4*/, false /*addPath*/, 65000)
	}

	t.Run("ipv6 unicast is refused", func(t *testing.T) {
		update, err := build(t, family.IPv6Unicast, "2001:db8::/32", netip.Addr{})
		assert.Nil(t, update, "no UPDATE may be produced")
		require.ErrorIs(t, err, errAnnounceNextHopUnencodable)
	})

	t.Run("ipv6 unicast with a real next hop is built", func(t *testing.T) {
		update, err := build(t, family.IPv6Unicast, "2001:db8::/32", netip.MustParseAddr("2001:db8::1"))
		require.NoError(t, err)
		require.NotNil(t, update)

		// The MP_REACH the builder emitted must declare the sixteen octets it wrote.
		_, _, value, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
		require.True(t, found, "MP_REACH_NLRI must be present")
		require.GreaterOrEqual(t, len(value), 4)
		assert.Equal(t, byte(16), value[3], "Length of Next Hop Network Address")

		parsed, parseErr := attribute.ParseMPReachNLRI(value)
		require.NoError(t, parseErr)
		require.NoError(t, parsed.ValidateNextHops())
	})

	t.Run("ipv4 unicast keeps its own remedy", func(t *testing.T) {
		// Not refused: the NLRI travels in the UPDATE's own field, so the batch's
		// prefixes still reach the peer with whatever NEXT_HOP the base carried.
		update, err := build(t, family.IPv4Unicast, "10.0.0.0/24", netip.Addr{})
		require.NoError(t, err)
		require.NotNil(t, update)
		assert.NotEmpty(t, update.NLRI, "the batch's prefixes still go out")
	})
}

// TestAnnounceNLRIBatch_NextHopUnencodableIsNotFamilyMismatch drives the same
// refusal from the entry point a plugin actually enters.
//
// The builder test above proves buildBatchAnnounceUpdate RETURNS the sentinel.
// That is not the same claim as an operator seeing it: the sentinel then has to
// survive AnnounceNLRIBatch's acceptedCount == 0 switch, which rewrites everything
// it does not recognize into route.ErrNoPeersAcceptedFamily. Drop
// errAnnounceNextHopUnencodable from that switch's case list and the builder test
// still passes while the operator is told no peer carries the family
// (ai/rules/evidence.md: drive a guard from its entry point).
//
// VALIDATES: AnnounceNLRIBatch reports errAnnounceNextHopUnencodable, and not
// ErrNoPeersAcceptedFamily, when the only matching peer negotiated the family and
// the announce could not be encoded.
// PREVENTS: the cause being collapsed into "no peers accepted the family", which is
// untrue here and which DispatchNLRIGroups downgrades to a warning, so a route that
// never went out reads as a routine skip.
func TestAnnounceNLRIBatch_NextHopUnencodableIsNotFamilyMismatch(t *testing.T) {
	dest := netip.MustParseAddr("10.0.0.2")
	fam := family.IPv6Unicast

	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    dest,
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{families: map[family.Family]bool{fam: true}})

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	a := &reactorAPIAdapter{r: r}

	// An EXPLICIT next hop that never resolved. Peer.resolveNextHop returns it
	// unchanged and without an error, which is what carries the zero Addr into the
	// builder (TestResolveNextHop_ExplicitInvalid).
	batch := bgptypes.NLRIBatch{
		Family:  fam,
		NLRIs:   []nlri.NLRI{nlri.NewINET(fam, netip.MustParsePrefix("2001:db8::/32"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.Addr{}),
	}

	err := a.AnnounceNLRIBatch(selector.All(), batch)
	require.ErrorIs(t, err, errAnnounceNextHopUnencodable)
	assert.NotErrorIs(t, err, route.ErrNoPeersAcceptedFamily,
		"the family IS negotiated; a failure of this speaker must not be reported as a peer declining it")
}
