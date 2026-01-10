package plugin

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
)

// TestRouteBuilderBasic verifies basic RouteBuilder usage.
//
// VALIDATES: RouteBuilder can build a simple route.
// PREVENTS: Basic route construction failures.
func TestRouteBuilderBasic(t *testing.T) {
	rb := NewRouteBuilder()
	rb.SetPrefix(netip.MustParsePrefix("10.0.0.0/24"))
	rb.SetNextHop(netip.MustParseAddr("192.168.1.1"))
	rb.SetFamily(nlri.IPv4Unicast)
	rb.Attrs().SetOrigin(0)

	route, err := rb.Build()
	require.NoError(t, err)
	require.NotNil(t, route)

	// Verify NLRI (type assert to INET to access Prefix)
	inet, ok := route.NLRI().(*nlri.INET)
	require.True(t, ok)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), inet.Prefix())

	// Verify next-hop
	assert.Equal(t, netip.MustParseAddr("192.168.1.1"), route.NextHop())
}

// TestRouteBuilderWithAttributes verifies attribute building.
//
// VALIDATES: RouteBuilder correctly builds attributes.
// PREVENTS: Attribute loss during route building.
func TestRouteBuilderWithAttributes(t *testing.T) {
	rb := NewRouteBuilder()
	rb.SetPrefix(netip.MustParsePrefix("10.0.0.0/24"))
	rb.SetNextHop(netip.MustParseAddr("192.168.1.1"))
	rb.SetFamily(nlri.IPv4Unicast)

	rb.Attrs().
		SetOrigin(0).
		SetLocalPref(200).
		SetMED(100).
		AddCommunity(65000, 100)

	route, err := rb.Build()
	require.NoError(t, err)

	// Check attributes were set
	attrs := route.Attributes()
	assert.NotEmpty(t, attrs)

	// Should have ORIGIN, MED, LOCAL_PREF, COMMUNITY
	var hasOrigin, hasMED, hasLocalPref, hasCommunity bool
	for _, attr := range attrs {
		switch attr.(type) {
		case attribute.Origin:
			hasOrigin = true
		case attribute.MED:
			hasMED = true
		case attribute.LocalPref:
			hasLocalPref = true
		case attribute.Communities:
			hasCommunity = true
		}
	}
	assert.True(t, hasOrigin, "should have ORIGIN")
	assert.True(t, hasMED, "should have MED")
	assert.True(t, hasLocalPref, "should have LOCAL_PREF")
	assert.True(t, hasCommunity, "should have COMMUNITY")
}

// TestRouteBuilderWithPathID verifies ADD-PATH support.
//
// VALIDATES: RouteBuilder handles path ID correctly.
// PREVENTS: Path ID loss in ADD-PATH scenarios.
func TestRouteBuilderWithPathID(t *testing.T) {
	rb := NewRouteBuilder()
	rb.SetPrefix(netip.MustParsePrefix("10.0.0.0/24"))
	rb.SetNextHop(netip.MustParseAddr("192.168.1.1"))
	rb.SetFamily(nlri.IPv4Unicast)
	rb.SetPathID(12345)

	route, err := rb.Build()
	require.NoError(t, err)

	// Check path ID in NLRI
	assert.Equal(t, uint32(12345), route.NLRI().PathID())
}

// TestRouteBuilderMissingPrefix verifies error on missing prefix.
//
// VALIDATES: RouteBuilder requires prefix.
// PREVENTS: Routes without destination.
func TestRouteBuilderMissingPrefix(t *testing.T) {
	rb := NewRouteBuilder()
	rb.SetNextHop(netip.MustParseAddr("192.168.1.1"))
	rb.SetFamily(nlri.IPv4Unicast)

	_, err := rb.Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prefix")
}

// TestRouteBuilderMissingNextHop verifies error on missing next-hop.
//
// VALIDATES: RouteBuilder requires next-hop.
// PREVENTS: Routes without forwarding info.
func TestRouteBuilderMissingNextHop(t *testing.T) {
	rb := NewRouteBuilder()
	rb.SetPrefix(netip.MustParsePrefix("10.0.0.0/24"))
	rb.SetFamily(nlri.IPv4Unicast)

	_, err := rb.Build()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "next-hop")
}

// TestRouteBuilderIPv6 verifies IPv6 route building.
//
// VALIDATES: RouteBuilder handles IPv6 correctly.
// PREVENTS: IPv6 routing failures.
func TestRouteBuilderIPv6(t *testing.T) {
	rb := NewRouteBuilder()
	rb.SetPrefix(netip.MustParsePrefix("2001:db8::/32"))
	rb.SetNextHop(netip.MustParseAddr("2001:db8::1"))
	rb.SetFamily(nlri.IPv6Unicast)

	route, err := rb.Build()
	require.NoError(t, err)
	inet, ok := route.NLRI().(*nlri.INET)
	require.True(t, ok)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), inet.Prefix())
}

// TestRouteBuilderChaining verifies method chaining.
//
// VALIDATES: RouteBuilder methods return self for chaining.
// PREVENTS: Verbose code.
func TestRouteBuilderChaining(t *testing.T) {
	route, err := NewRouteBuilder().
		SetPrefix(netip.MustParsePrefix("10.0.0.0/24")).
		SetNextHop(netip.MustParseAddr("192.168.1.1")).
		SetFamily(nlri.IPv4Unicast).
		Build()

	require.NoError(t, err)
	require.NotNil(t, route)
}

// TestRouteBuilderReset verifies reset clears state.
//
// VALIDATES: Reset allows builder reuse.
// PREVENTS: State leakage between builds.
func TestRouteBuilderReset(t *testing.T) {
	rb := NewRouteBuilder()
	rb.SetPrefix(netip.MustParsePrefix("10.0.0.0/24"))
	rb.SetNextHop(netip.MustParseAddr("192.168.1.1"))
	rb.SetFamily(nlri.IPv4Unicast)
	rb.SetPathID(123)
	rb.Attrs().SetOrigin(1)

	rb.Reset()

	// Should fail - no prefix set after reset
	_, err := rb.Build()
	require.Error(t, err)
}
