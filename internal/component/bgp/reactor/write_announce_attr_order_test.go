// Design: docs/architecture/wire/mp-nlri-ordering.md -- outbound attribute order
// Related: reactor_wire.go -- writeAnnounceUpdate, the writer under test

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestWriteAnnounceUpdateAttributesAscend pins the attribute order of the
// direct announce writer, writeAnnounceUpdate, for a route that carries every
// optional attribute it can write.
//
// Goal: RFC 4271 Section 5, "The sender of an UPDATE message SHOULD order path
// attributes within the UPDATE message in ascending order of attribute type",
// with MP_REACH_NLRI first for IPv6 (RFC 7606 Section 5.1).
// Method: a four-octet local AS toward an OLD eBGP peer forces AS4_PATH (17),
// and the route carries COMMUNITY (8), EXTENDED_COMMUNITIES (16) and
// LARGE_COMMUNITY (32). The test walks the written block and compares the
// codes to the exact expected order.
// PREVENTS: LARGE_COMMUNITY (32) written before EXTENDED_COMMUNITIES (16) and
// AS4_PATH (17), which the writer did until 2026-10-19.
func TestWriteAnnounceUpdateAttributesAscend(t *testing.T) {
	cases := []struct {
		name    string
		prefix  string
		nextHop string
		want    []int
	}{
		{"ipv4", "10.0.0.0/24", "192.168.1.1", []int{1, 2, 3, 4, 8, 16, 17, 32}},
		{"ipv6", "2001:db8::/32", "2001:db8::1", []int{14, 1, 2, 4, 8, 16, 17, 32}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			attrs := attribute.NewBuilder()
			attrs.SetMED(10)
			attrs.AddCommunity(65000, 1)
			attrs.AddExtendedCommunity(attribute.ExtendedCommunity{0x00, 0x02, 0xfd, 0xe8, 0, 0, 0, 1})
			attrs.AddLargeCommunity(65000, 1, 2)
			route := bgptypes.RouteSpec{
				Prefix:  netip.MustParsePrefix(c.prefix),
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(c.nextHop)),
				Wire:    attribute.NewAttributesWire(attrs.Build(), bgpctx.APIContextID),
			}

			buf := make([]byte, 4096)
			n := writeAnnounceUpdate(buf, 0, route, netip.Addr{}, nonMappableAS, false /*eBGP*/, false /*OLD*/, false)
			require.NotZero(t, n, "the writer reports zero bytes only when it refuses the next hop")

			codes := attrCodes(t, announceMsgPathAttrs(t, buf, n))
			assert.Equal(t, c.want, codes, "writeAnnounceUpdate attribute order")
			assertAscending(t, codes)
		})
	}
}
