// VALIDATES: the keying of labeled unicast and VPN routes on the bridge ingest path
// beyond the IPv4 basics in labeled_vpn_key_test.go: ADD-PATH, IPv6, a withdrawal
// whose Compatibility field holds any value, a re-announcement under a new label,
// the route name an operator reads in show and revalidate, and the StoredRoute a
// replay hands the engine. Each test names the break that turns it red.
// PREVENTS: a path identifier read as NLRI octets, an IPv6 route keyed as IPv4, a
// withdrawal that only matches 0x800000, a stale label kept after a re-announce,
// "invalid Prefix" in operator output, and a replay that relays different bytes
// from those stored.

package adj_rib_in

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var (
	ipv6Labeled = family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel}
	ipv6VPN     = family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
)

// prefix2001db8 is the six prefix octets of 2001:db8:<third>::/48.
func prefix2001db8(third byte) []byte {
	return []byte{0x20, 0x01, 0x0d, 0xb8, 0x00, third}
}

// pathID prefixes an NLRI with its RFC 7911 Section 3 Path Identifier.
func pathID(id byte, nlri []byte) []byte {
	return append([]byte{0, 0, 0, id}, nlri...)
}

// labeledWithdrawal is the RFC 8277 Section 2.4 withdrawal of an IPv4 /24, with
// the given three octets in its Compatibility field.
func labeledWithdrawal(compat []byte, prefix ...byte) []byte {
	out := append([]byte{byte(24 + 8*len(prefix))}, compat...)
	return append(out, prefix...)
}

// TestComplexAddPathKeysOnPathID negotiates ADD-PATH for labeled unicast: two
// paths to one prefix are two routes, each stored without its identifier, and a
// withdrawal of one path leaves the other.
// MUTATION: nlri.SplitPathID(raw, false) in installComplexNLRIs, or in
// removeComplexNLRIs, turns this red.
func TestComplexAddPathKeysOnPathID(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("192.0.2.1")
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{ipv4Labeled: true})
	route := labeledNLRI(labelOneBottom, 10, 0, 1)

	feedStructuredContext(t, r, source, mpAnnounce(ipv4Labeled, []byte{10, 0, 0, 1}, pathID(1, route), pathID(2, route)), ctx)

	require.NotNil(t, r.ribIn[source])
	assert.Equal(t, 2, r.ribIn[source].Len(), "two paths to one prefix are two routes")
	for _, id := range []uint32{1, 2} {
		stored, ok := r.ribIn[source].Get(routeKeyFromStrings(ipv4Labeled, "10.0.1.0/24", id))
		require.True(t, ok, "path %d is keyed on its identifier and its prefix", id)
		assert.Equal(t, hex.EncodeToString(route), stored.NLRIHex, "the stored NLRI carries no path identifier")
		assert.Equal(t, id, stored.PathID)
	}

	feedStructuredContext(t, r, source, mpWithdraw(ipv4Labeled, pathID(1, labeledWithdrawal(compatibility, 10, 0, 1))), ctx)
	assert.Equal(t, 1, r.ribIn[source].Len(), "the withdrawal removes one path")
	_, ok := r.ribIn[source].Get(routeKeyFromStrings(ipv4Labeled, "10.0.1.0/24", 2))
	assert.True(t, ok, "the path the withdrawal did not name stays")
}

// TestIPv6LabeledRoutesKeyOnTheirPrefix is the IPv6 labeled unicast proof: two
// /48s behind one label are two routes, keyed on the IPv6 prefix.
// MUTATION: nlri.WirePrefixToKey(cidr, family.IPv4Unicast) in routeKeyFromNLRI
// turns this red.
func TestIPv6LabeledRoutesKeyOnTheirPrefix(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("192.0.2.1")
	nextHop := netip.MustParseAddr("2001:db8::1").AsSlice()

	first := labeledNLRI(labelOneBottom, prefix2001db8(1)...)
	second := labeledNLRI(labelOneBottom, prefix2001db8(2)...)
	feedStructured(t, r, source, mpAnnounce(ipv6Labeled, nextHop, first, second))

	require.NotNil(t, r.ribIn[source])
	assert.Equal(t, 2, r.ribIn[source].Len())
	_, ok := r.ribIn[source].Get(routeKeyFromStrings(ipv6Labeled, "2001:db8:1::/48", 0))
	assert.True(t, ok, "the route is keyed on the IPv6 prefix behind its label")

	withdrawn := append([]byte{24 + 48}, compatibility...)
	feedStructured(t, r, source, mpWithdraw(ipv6Labeled, append(withdrawn, prefix2001db8(1)...)))
	assert.Equal(t, 1, r.ribIn[source].Len())
	_, ok = r.ribIn[source].Get(routeKeyFromStrings(ipv6Labeled, "2001:db8:2::/48", 0))
	assert.True(t, ok, "the withdrawal removes the route it names and no other")
}

// TestIPv6VPNRoutesKeyOnDistinguisherAndPrefix is the IPv6 VPN proof: one
// prefix under two RDs is two routes, and withdrawing one leaves the other.
// MUTATION: keying on identity[9:] (the identity without its length and RD) in
// routeKeyFromNLRI turns this red.
func TestIPv6VPNRoutesKeyOnDistinguisherAndPrefix(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("192.0.2.1")
	target := netip.MustParseAddr("192.0.2.9")
	nextHop := append(make([]byte, 8), netip.MustParseAddr("2001:db8::1").AsSlice()...)

	underOne := vpnNLRI(labelOneBottom, rd(1), prefix2001db8(1)...)
	underTwo := vpnNLRI(labelOneBottom, rd(2), prefix2001db8(1)...)
	feedStructured(t, r, source, mpAnnounce(ipv6VPN, nextHop, underOne, underTwo))

	require.NotNil(t, r.ribIn[source])
	assert.Equal(t, 2, r.ribIn[source].Len(), "RD and prefix together identify an IPv6 VPN route")

	withdrawn := append([]byte{24 + 64 + 48}, compatibility...)
	withdrawn = append(withdrawn, rd(1)...)
	feedStructured(t, r, source, mpWithdraw(ipv6VPN, append(withdrawn, prefix2001db8(1)...)))
	assert.Equal(t, []string{hex.EncodeToString(underTwo)}, replayedNLRIs(r, target, source),
		"withdrawing RD 1 leaves the same prefix under RD 2")
}

// TestLabeledWithdrawalIgnoresCompatibilityValue withdraws with Compatibility
// fields other than the 0x800000 a sender SHOULD set. RFC 8277 Section 2.4:
// "Upon reception, the value of the Compatibility field MUST be ignored."
// MUTATION: ignoring a withdrawal in removeComplexNLRIs unless its field reads
// 0x800000 turns this red.
func TestLabeledWithdrawalIgnoresCompatibilityValue(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("192.0.2.1")
	nextHop := []byte{10, 0, 0, 1}

	feedStructured(t, r, source, mpAnnounce(ipv4Labeled, nextHop,
		labeledNLRI(labelOneBottom, 10, 0, 1), labeledNLRI(labelOneBottom, 10, 0, 2)))
	require.NotNil(t, r.ribIn[source])
	require.Equal(t, 2, r.ribIn[source].Len())

	feedStructured(t, r, source, mpWithdraw(ipv4Labeled, labeledWithdrawal([]byte{0, 0, 0}, 10, 0, 1)))
	assert.Equal(t, 1, r.ribIn[source].Len(), "a zero Compatibility field still withdraws")
	feedStructured(t, r, source, mpWithdraw(ipv4Labeled, labeledWithdrawal(labelOneBottom, 10, 0, 2)))
	assert.Equal(t, 0, r.ribIn[source].Len(), "a Compatibility field holding the route's own label still withdraws")
}

// TestLabeledReannounceWithNewLabelReplaces re-announces a stored prefix under a
// new label: it is the same route, so the store holds one entry carrying the new
// label, and a replay relays the new label.
// MUTATION: skipping the Put in installComplexNLRIs when the key is already
// stored turns this red.
func TestLabeledReannounceWithNewLabelReplaces(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("192.0.2.1")
	target := netip.MustParseAddr("192.0.2.9")
	nextHop := []byte{10, 0, 0, 1}
	labelTwoBottom := []byte{0x00, 0x7d, 0x01} // label 2000, S bit set

	feedStructured(t, r, source, mpAnnounce(ipv4Labeled, nextHop, labeledNLRI(labelOneBottom, 10, 0, 1)))
	relabeled := labeledNLRI(labelTwoBottom, 10, 0, 1)
	feedStructured(t, r, source, mpAnnounce(ipv4Labeled, nextHop, relabeled))

	require.NotNil(t, r.ribIn[source])
	assert.Equal(t, 1, r.ribIn[source].Len(), "a new label for a stored prefix replaces the route")
	assert.Equal(t, []string{hex.EncodeToString(relabeled)}, replayedNLRIs(r, target, source))
}

// TestShowAndRevalidateNameNLRIKeyedRoutes reads a VPN route back through show
// and revalidate, where the key names the route by its identity hex: the length
// in bits once the label is dropped, the RD, then the prefix.
// MUTATION: routeName returning k.Prefix.String() for every key turns this red
// ("invalid Prefix").
func TestShowAndRevalidateNameNLRIKeyedRoutes(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("192.0.2.1")
	nextHop := []byte{0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 1}
	feedStructured(t, r, source, mpAnnounce(ipv4VPN, nextHop, vpnNLRI(labelOneBottom, rd(1), 10, 0, 3)))

	const identity = "580000fde9000000010a0003"
	shown, ok := r.show("*").(map[string]any)
	require.True(t, ok)
	byPeer, ok := shown["adj-rib-in"].(map[string][]map[string]any)
	require.True(t, ok)
	require.Len(t, byPeer[source.String()], 1)
	assert.Equal(t, ipv4VPN.String()+":"+identity, byPeer[source.String()][0]["key"])

	status, data, err := r.revalidateCommand([]string{ipv4VPN.String(), identity})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)
	result, ok := data.(map[string]any)
	require.True(t, ok)
	routes, ok := result["routes"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, routes, 1, "revalidate selects the route by the name show prints")
	assert.Equal(t, identity, routes[0]["prefix"])
}

// TestReplayRelaysComplexRoutesAsStored drives "request bgp adj-rib-in replay"
// and reads every field of the StoredRoute the engine is handed for an ADD-PATH
// labeled route: the NLRI without its identifier, the identifier in PathID, the
// framing that tells the engine so, and the attributes and next hop as received.
// MUTATION: storing PathID 0 in installComplexNLRIs turns this red.
func TestReplayRelaysComplexRoutesAsStored(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("192.0.2.1")
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{ipv4Labeled: true})
	route := labeledNLRI(labelOneBottom, 10, 0, 1)
	feedStructuredContext(t, r, source, mpAnnounce(ipv4Labeled, []byte{10, 0, 0, 1}, pathID(7, route)), ctx)

	var relayed []rpc.StoredRoute
	r.routeRelayer = func(_ string, routes []rpc.StoredRoute) error {
		relayed = append(relayed, routes...)
		return nil
	}
	status, _, err := r.replayCommand([]string{"192.0.2.9"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Len(t, relayed, 1)

	got := relayed[0]
	assert.Equal(t, source.String(), got.SourcePeer)
	assert.Equal(t, ipv4Labeled.String(), got.Family)
	assert.Equal(t, hex.EncodeToString(route), got.NLRIHex)
	assert.Equal(t, uint32(7), got.PathID)
	assert.Equal(t, rpc.NLRIFramingPrefixOnly, got.NLRIFraming)
	assert.Equal(t, "0a000001", got.NextHopHex)
	// The whole attribute section as received, MP_REACH_NLRI included: the
	// engine strips types 14 and 15 and re-synthesizes MP_REACH from NLRIHex and
	// NextHopHex (reactor_api_relay.go), so those two fields are what it relays.
	assert.Equal(t, "40010100400200"+"900e0014000104040a000001000000000730003e810a0001", got.AttrHex)
}
