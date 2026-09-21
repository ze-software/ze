//go:build ze_bgp

package bridge

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"

	// The community JSON formatters are REGISTERED by this plugin's init, and
	// the registry is what appendAttributeJSON asks. Without the import this
	// package's test binary renders every community attribute as `attr-16`
	// hex, so a document assertion here would pass against a daemon that is
	// broken and fail against one that is not.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community"

	// The `mcast-vpn` family NAME and the MVPN NLRI decoder are registered by
	// this plugin's init. Without it the document says `ipv4 safi-5` and an
	// opaque carrier, which is what a daemon built without the plugin really
	// does, and not what this case is about.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/mvpn"
)

// mvpnFacts is the session the two UPDATEs below arrived on, as
// test/exabgp-compat/encoding/conf-mvpn.ci configures it.
var mvpnFacts = SessionFacts{
	Local:    netip.MustParseAddr("127.0.0.1"),
	Peer:     netip.MustParseAddr("127.0.0.1"),
	LocalAS:  65000,
	PeerAS:   65000,
	RouterID: 0x20202020,
}

// TestMVPNUpdateKeepsItsAttributesUnderBothAFIs renders the two MCAST-VPN
// UPDATEs of the ExaBGP compatibility fixture and asserts each keeps the
// extended community it carries.
//
// The two messages carry BYTE-IDENTICAL attribute blocks apart from their
// MP_REACH, so any difference between the two documents is a defect by
// construction. That is what makes this pair worth pinning rather than one
// message: the IPv4 one rendered correctly for months while the IPv6 one
// silently lost every attribute it had.
//
// VALIDATES: an MCAST-VPN UPDATE whose MP_REACH states AFI 2 and a 4-octet
// IPv4 next hop reaches the renderer with its attributes intact.
// PREVENTS: the next hop's family being decided by the NLRI's AFI. ze refused
// that combination, ParseMPReachNLRI returned an error, AttributesWire.All()
// gave up on the whole set, and the rendered document lost the origin, the
// local preference and the route target together. RFC 6514 Section 5 allows
// either address family here and RFC 8950 Section 3 reads the LENGTH.
func TestMVPNUpdateKeepsItsAttributesUnderBothAFIs(t *testing.T) {
	for _, c := range []struct {
		name   string
		wire   string
		family string
	}{
		{
			name:   "ipv4 mcast-vpn",
			family: "ipv4 mcast-vpn",
			wire: "0000005C400101004002004003040A0A060340050400000064800E390001" +
				"05040A0A06030006160000FDE80001869F0000FDE8200A63C70120EFFBFF" +
				"E407160000FDE80001869F0000FDE8200A630C0220EFFBFFE4C010080102" +
				"C0A85E0C0005",
		},
		{
			name:   "ipv6 mcast-vpn",
			family: "ipv6 mcast-vpn",
			wire: "0000008C400101004002004003040A0A060340050400000064800E690002" +
				"05040A0A060300062E0000FDE80001869F0000FDE880FD00000000000000" +
				"000000000000000180FF0E0000000000000000000000000001072E0000FD" +
				"E80001869F0000FDE880FD12000000000000000000000000000280FF0E00" +
				"00000000000000000000000001C010080102C0A85E0C0005",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			payload, err := hex.DecodeString(c.wire)
			require.NoError(t, err)

			document, err := WireUpdateToExabgpJSON(payload, mvpnFacts, rpc.DirectionReceived)
			require.NoError(t, err)

			update, ok := exabgpUpdateObject(document)
			require.True(t, ok, "the document states no update object")

			attributes, ok := update["attribute"].(map[string]any)
			require.True(t, ok, "the update states no attribute object")

			assert.Equal(t, "igp", attributes["origin"])
			assert.Equal(t, float64(100), attributes["local-preference"])

			communities, ok := attributes["extended-community"].([]any)
			require.True(t, ok, "the update states no extended community")
			require.Len(t, communities, 1)
			member, ok := communities[0].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "target:192.168.94.12:5", member["string"],
				"the community's text is what a script reads; a bare value says the "+
					"renderer never saw the parsed attribute")

			announce, ok := update["announce"].(map[string]map[string][]any)
			require.True(t, ok, "the update announces nothing")
			require.Contains(t, announce, c.family)
		})
	}
}
