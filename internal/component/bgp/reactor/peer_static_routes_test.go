package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// VALIDATES: Static route helpers (routeFamily, writeRawAttribute, packRawAttributes, routeGroupKey, groupRoutesByAttributes).
// PREVENTS: Wrong family detection, raw attribute encoding errors, incorrect route grouping.

func staticRoute(prefix string) StaticRoute {
	return StaticRoute{
		Prefix:  netip.MustParsePrefix(prefix),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("1.1.1.1")),
	}
}

// TestRouteFamily_IPv4Unicast verifies plain IPv4 route returns IPv4 unicast family.
func TestRouteFamily_IPv4Unicast(t *testing.T) {
	r := staticRoute("10.0.0.0/24")
	f := routeFamily(&r)
	assert.Equal(t, family.IPv4Unicast, f)
}

// TestRouteFamily_IPv6Unicast verifies plain IPv6 route returns IPv6 unicast family.
func TestRouteFamily_IPv6Unicast(t *testing.T) {
	r := staticRoute("2001:db8::/32")
	f := routeFamily(&r)
	assert.Equal(t, family.IPv6Unicast, f)
}

// TestRouteFamily_VPNv4 verifies VPN IPv4 route returns correct family.
func TestRouteFamily_VPNv4(t *testing.T) {
	r := staticRoute("10.0.0.0/24")
	r.RD = "100:100"
	f := routeFamily(&r)
	assert.Equal(t, family.AFIIPv4, f.AFI)
	assert.Equal(t, family.SAFI(128), f.SAFI)
}

// TestRouteFamily_VPNv6 verifies VPN IPv6 route returns correct family.
func TestRouteFamily_VPNv6(t *testing.T) {
	r := staticRoute("2001:db8::/32")
	r.RD = "100:100"
	f := routeFamily(&r)
	assert.Equal(t, family.AFIIPv6, f.AFI)
	assert.Equal(t, family.SAFI(128), f.SAFI)
}

// TestRouteFamily_LabeledIPv4 verifies labeled IPv4 route returns SAFI 4.
func TestRouteFamily_LabeledIPv4(t *testing.T) {
	r := staticRoute("10.0.0.0/24")
	r.Labels = []uint32{100}
	f := routeFamily(&r)
	assert.Equal(t, family.AFIIPv4, f.AFI)
	assert.Equal(t, family.SAFI(4), f.SAFI)
}

// TestWriteRawAttribute_StandardLength verifies standard (1-byte) length encoding.
func TestWriteRawAttribute_StandardLength(t *testing.T) {
	ra := RawAttribute{
		Flags: 0x40,         // Transitive
		Code:  0x01,         // ORIGIN
		Value: []byte{0x00}, // IGP
	}

	buf := make([]byte, 32)
	n := writeRawAttribute(buf, 0, ra)

	assert.Equal(t, 4, n, "flags(1) + code(1) + len(1) + value(1)")
	assert.Equal(t, byte(0x40), buf[0])
	assert.Equal(t, byte(0x01), buf[1])
	assert.Equal(t, byte(0x01), buf[2], "length = 1")
	assert.Equal(t, byte(0x00), buf[3], "value = IGP")
}

// TestWriteRawAttribute_ExtendedLength verifies extended (2-byte) length encoding.
func TestWriteRawAttribute_ExtendedLength(t *testing.T) {
	ra := RawAttribute{
		Flags: 0x50,              // Transitive + Extended Length
		Code:  0xFF,              // Custom
		Value: make([]byte, 300), // > 255 bytes
	}

	buf := make([]byte, 512)
	n := writeRawAttribute(buf, 0, ra)

	assert.Equal(t, 4+300, n, "flags(1) + code(1) + len(2) + value(300)")
	assert.Equal(t, byte(0x50), buf[0])
	assert.Equal(t, byte(0xFF), buf[1])
	assert.Equal(t, byte(0x01), buf[2], "high byte of 300")
	assert.Equal(t, byte(0x2C), buf[3], "low byte of 300")
}

// TestRawAttributeLen_Standard verifies length calculation for standard attributes.
func TestRawAttributeLen_Standard(t *testing.T) {
	ra := RawAttribute{Flags: 0x40, Code: 0x01, Value: []byte{0x00}}
	assert.Equal(t, 4, rawAttributeLen(ra))
}

// TestRawAttributeLen_Extended verifies length calculation for extended attributes.
func TestRawAttributeLen_Extended(t *testing.T) {
	ra := RawAttribute{Flags: 0x50, Code: 0xFF, Value: make([]byte, 300)}
	assert.Equal(t, 304, rawAttributeLen(ra))
}

// TestPackRawAttributes_Empty verifies nil return for no attributes.
func TestPackRawAttributes_Empty(t *testing.T) {
	result := packRawAttributes(nil)
	assert.Nil(t, result)
}

// TestPackRawAttributes_Multiple verifies packing multiple attributes.
func TestPackRawAttributes_Multiple(t *testing.T) {
	attrs := []RawAttribute{
		{Flags: 0x40, Code: 0x01, Value: []byte{0x00}},
		{Flags: 0x40, Code: 0x02, Value: []byte{0x02, 0x01, 0x00, 0xFD, 0xE9}},
	}

	result := packRawAttributes(attrs)
	require.Len(t, result, 2)

	// First attribute: flags(0x40) + code(0x01) + len(0x01) + value(0x00).
	assert.Equal(t, []byte{0x40, 0x01, 0x01, 0x00}, result[0])

	// Second attribute: flags(0x40) + code(0x02) + len(0x05) + value.
	assert.Equal(t, byte(0x40), result[1][0])
	assert.Equal(t, byte(0x02), result[1][1])
	assert.Equal(t, byte(0x05), result[1][2])
}

// TestGroupRoutesByAttributes_SameAttributes verifies routes with same attrs are grouped.
func TestGroupRoutesByAttributes_SameAttributes(t *testing.T) {
	r1 := staticRoute("10.0.0.0/24")
	r2 := staticRoute("10.0.1.0/24")
	// Same next-hop, origin, etc. — should be grouped.

	groups := groupRoutesByAttributes([]StaticRoute{r1, r2})
	assert.Len(t, groups, 1, "same attributes should produce one group")
	assert.Len(t, groups[0], 2, "group should contain both routes")
}

// TestGroupRoutesByAttributes_DifferentMED verifies routes with different MED are separated.
func TestGroupRoutesByAttributes_DifferentMED(t *testing.T) {
	r1 := staticRoute("10.0.0.0/24")
	r1.MED = 100
	r2 := staticRoute("10.0.1.0/24")
	r2.MED = 200

	groups := groupRoutesByAttributes([]StaticRoute{r1, r2})
	assert.Len(t, groups, 2, "different MED should produce separate groups")
}

// TestGroupRoutesByAttributes_IPv6Separate verifies IPv6 routes are not grouped.
func TestGroupRoutesByAttributes_IPv6Separate(t *testing.T) {
	r1 := StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/48"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("::1")),
	}
	r2 := StaticRoute{
		Prefix:  netip.MustParsePrefix("2001:db8:2::/48"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("::1")),
	}

	groups := groupRoutesByAttributes([]StaticRoute{r1, r2})
	assert.Len(t, groups, 2, "IPv6 routes should not be grouped (each needs separate MP_REACH_NLRI)")
}
