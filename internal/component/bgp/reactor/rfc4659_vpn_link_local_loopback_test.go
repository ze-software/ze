// Design: docs/architecture/update-building.md -- VPN next-hop link-local condition.
// Related: link_scope.go -- linkLocalConditionFor, which family answers which condition.
// Related: rfc4659_vpn_link_local_test.go -- the same rails with a global on the common subnet.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"
)

// The speaker's global is a loopback address, 2001:db8:99::1, outside the
// connected 2001:db8:1::/64 the recipient vpnLLOnLink shares with it.
const (
	vpnLoopbackGlobal    = "2001:db8:99::1"
	vpnLoopbackGlobalHex = "20010db8009900000000000000000001"
)

// TestRFC4659VPNv6LinkLocalLoopbackGlobal sends a VPN-IPv6 route whose global
// next hop is the speaker's loopback to a recipient on the speaker's subnet,
// over the origination rail, the relay normalizer and the next-hop-self rewrite.
//
// VALIDATES: RFC 4659 Section 3.2.1.1 is read literally: the speaker shares a
// common subnet with the peer, so the 48-octet field carries the speaker's
// link-local fe80::2 even though the loopback global is on no subnet the peer
// shares. The plain IPv6 route with the same global keeps RFC 2545 Section 3's
// joint condition and goes out as the 16-octet global alone.
// PREVENTS: the VPN-IPv6 decision borrowing RFC 2545's requirement that the
// global next hop share the subnet too, or the RFC 2545 decision losing it.
//
// RFC requirement: RFC4659-3.2.1.1-3 positive -- a VPN-IPv6 route whose global next hop is the speaker's loopback 2001:db8:99::1, outside the connected 2001:db8:1::/64, sent to a recipient on that /64 carries the 48-octet Next Hop field (zero RD, 2001:db8:99::1, zero RD, fe80::2) on the configured-route, relay and next-hop-self rails.
func TestRFC4659VPNv6LinkLocalLoopbackGlobal(t *testing.T) {
	vpnv6 := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
	single := vpnLLRDZero + vpnLoopbackGlobalHex
	pair := single + vpnLLRDZero + vpnLLLL
	nextHop := netip.MustParseAddr(vpnLoopbackGlobal)

	t.Run("configured route", func(t *testing.T) {
		peer := vpnNHRewritePeer(t, vpnLLOnLink, vpnLoopbackGlobal, NextHopUnchanged, "")
		route := &StaticRoute{
			Prefix:  netip.MustParsePrefix("2001:db8:5601::/48"),
			RD:      "65001:100",
			RDBytes: [8]byte{0, 0, 0xfd, 0xe9, 0, 0, 0, 100},
			Labels:  []uint32{1000},
		}
		ub := message.NewUpdateBuilder(65000, true, true, false)
		update, err := buildStaticRouteUpdateNew(ub, route, nextHop, peer.linkLocalNextHopFor(nil, nextHop, route.Family()), peer.sendCtx.Load(), false)
		require.NoError(t, err)
		assert.Equal(t, pair, vpnLLNextHopField(t, update.PathAttributes))
	})

	t.Run("relay", func(t *testing.T) {
		dest := vpnNHRewritePeer(t, vpnLLOnLink, vpnLoopbackGlobal, NextHopUnchanged, "")
		assert.Equal(t, pair, vpnNHRelay(t, dest, vpnv6, single, vpnNHNLRIv6), "raised to the 48-octet pair")
		assert.Equal(t, pair, vpnNHRelay(t, dest, vpnv6, pair, vpnNHNLRIv6), "a received pair is kept")
	})

	t.Run("next-hop self", func(t *testing.T) {
		dest := vpnNHRewritePeer(t, vpnLLOnLink, vpnLoopbackGlobal, NextHopSelf, "")
		assert.Equal(t, pair, vpnNHRelay(t, dest, vpnv6, vpnNHReceived, vpnNHNLRIv6))
	})

	t.Run("off-link recipient", func(t *testing.T) {
		dest := vpnNHRewritePeer(t, vpnLLOffLink, vpnLoopbackGlobal, NextHopSelf, "")
		assert.Equal(t, single, vpnNHRelay(t, dest, vpnv6, vpnNHReceived, vpnNHNLRIv6))
	})
}

// TestRFC2545IPv6LinkLocalLoopbackGlobalOmitted sends the plain IPv6 form of
// the same next hop to the same recipient.
//
// VALIDATES: RFC 2545 Section 3 asks for ONE subnet holding the global next hop
// and the peer; the loopback global is on none, so the 16-octet field goes out
// alone on the configured-route and relay rails, and a received pair loses its
// link-local half.
// PREVENTS: the RFC 4659 literal reading leaking into ordinary IPv6.
func TestRFC2545IPv6LinkLocalLoopbackGlobalOmitted(t *testing.T) {
	nextHop := netip.MustParseAddr(vpnLoopbackGlobal)
	peer := vpnNHRewritePeer(t, vpnLLOnLink, vpnLoopbackGlobal, NextHopUnchanged, "")
	assert.False(t, peer.linkLocalNextHopFor(nil, nextHop, family.IPv6Unicast).IsValid())

	nlri, err := hex.DecodeString("20" + "20010db8")
	require.NoError(t, err)
	for _, field := range []string{vpnLoopbackGlobalHex, vpnLoopbackGlobalHex + vpnLLLL} {
		fieldBytes, err := hex.DecodeString(field)
		require.NoError(t, err)
		src := buildMPReachSource(uint16(family.AFIIPv6), byte(family.SAFIUnicast), fieldBytes, nlri)
		var mods filterapi.ModAccumulator
		applyEgressNextHopScope(peer, peer.forwardFacts(), &mods, buildUpdatePayload(src, nil), family.IPv6Unicast)
		out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, mods.Ops())
		require.True(t, ok, "normalization emits the MP_REACH attribute")
		assert.Equal(t, vpnLoopbackGlobalHex, vpnLLNextHopField(t, out), "field %s", field)
	}
}
