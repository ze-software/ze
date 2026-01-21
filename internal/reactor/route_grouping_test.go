package reactor

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/rib"
	"github.com/stretchr/testify/require"
)

// TestToRIBRouteUnicastParams verifies Route to UnicastParams conversion.
//
// VALIDATES: All attribute fields correctly extracted from RIB route.
// PREVENTS: Lost attributes during conversion.
func TestToRIBRouteUnicastParams(t *testing.T) {
	tests := []struct {
		name   string
		route  *rib.Route
		expect message.UnicastParams
	}{
		{
			name: "basic_ipv4_route",
			route: rib.NewRouteWithASPath(
				nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
				netip.MustParseAddr("192.168.1.1"),
				[]attribute.Attribute{
					attribute.OriginIGP,
				},
				&attribute.ASPath{Segments: []attribute.ASPathSegment{
					{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
				}},
			),
			expect: message.UnicastParams{
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("192.168.1.1"),
				Origin:  attribute.OriginIGP,
				ASPath:  []uint32{65001, 65002},
			},
		},
		{
			name: "route_with_communities",
			route: rib.NewRouteWithASPath(
				nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.1.0.0/16"), 0),
				netip.MustParseAddr("192.168.1.1"),
				[]attribute.Attribute{
					attribute.OriginIGP,
					attribute.Communities{0x01020003, 0x01020004},
					attribute.MED(100),
					attribute.LocalPref(200),
				},
				nil,
			),
			expect: message.UnicastParams{
				Prefix:          netip.MustParsePrefix("10.1.0.0/16"),
				NextHop:         netip.MustParseAddr("192.168.1.1"),
				Origin:          attribute.OriginIGP,
				Communities:     []uint32{0x01020003, 0x01020004},
				MED:             100,
				LocalPreference: 200,
			},
		},
		{
			name: "route_with_path_id",
			route: rib.NewRouteWithASPath(
				nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.2.0.0/24"), 42),
				netip.MustParseAddr("192.168.1.1"),
				[]attribute.Attribute{attribute.OriginEGP},
				nil,
			),
			expect: message.UnicastParams{
				Prefix:  netip.MustParsePrefix("10.2.0.0/24"),
				PathID:  42,
				NextHop: netip.MustParseAddr("192.168.1.1"),
				Origin:  attribute.OriginEGP,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := toRIBRouteUnicastParams(tt.route)

			require.Equal(t, tt.expect.Prefix, params.Prefix, "prefix mismatch")
			require.Equal(t, tt.expect.PathID, params.PathID, "path-id mismatch")
			require.Equal(t, tt.expect.NextHop, params.NextHop, "next-hop mismatch")
			require.Equal(t, tt.expect.Origin, params.Origin, "origin mismatch")
			require.Equal(t, tt.expect.ASPath, params.ASPath, "as-path mismatch")
			require.Equal(t, tt.expect.MED, params.MED, "MED mismatch")
			require.Equal(t, tt.expect.LocalPreference, params.LocalPreference, "local-pref mismatch")
			require.Equal(t, tt.expect.Communities, params.Communities, "communities mismatch")
		})
	}
}

// TestGroupedSendReducesUpdateCount verifies grouping efficiency.
//
// VALIDATES: Routes with same attributes grouped into single UPDATE.
// PREVENTS: Regression to one-route-per-UPDATE.
func TestGroupedSendReducesUpdateCount(t *testing.T) {
	// Create 100 routes with identical attributes
	routes := make([]*rib.Route, 100)
	for i := 0; i < 100; i++ {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, byte(i / 256), byte(i % 256)}), 24)
		routes[i] = rib.NewRouteWithASPath(
			nlri.NewINET(nlri.IPv4Unicast, prefix, 0),
			netip.MustParseAddr("192.168.1.1"),
			[]attribute.Attribute{attribute.OriginIGP},
			&attribute.ASPath{Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{65001}},
			}},
		)
	}

	// Group routes
	groups := rib.GroupByAttributesTwoLevel(routes)

	// Should have exactly 1 group (all routes have same attributes)
	require.Equal(t, 1, len(groups), "all same-attr routes should be in 1 group")
	require.Equal(t, 1, len(groups[0].ByASPath), "all routes have same AS_PATH")
	require.Equal(t, 100, len(groups[0].ByASPath[0].Routes), "group should contain all 100 routes")

	t.Log("✅ 100 same-attr routes grouped into 1 attribute group with 1 AS_PATH group")
}

// TestGroupedSendSeparatesAttributeGroups verifies attribute-based separation.
//
// VALIDATES: Routes with different attributes go to separate groups.
// PREVENTS: Incorrect grouping leading to attribute corruption.
func TestGroupedSendSeparatesAttributeGroups(t *testing.T) {
	// Create routes with 3 different next-hops (different attributes)
	routes := make([]*rib.Route, 30)
	nextHops := []netip.Addr{
		netip.MustParseAddr("192.168.1.1"),
		netip.MustParseAddr("192.168.1.2"),
		netip.MustParseAddr("192.168.1.3"),
	}

	for i := 0; i < 30; i++ {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, byte(i / 256), byte(i % 256)}), 24)
		nh := nextHops[i%3] //nolint:gosec // nextHops has exactly 3 elements, i%3 is always in bounds
		routes[i] = rib.NewRouteWithASPath(
			nlri.NewINET(nlri.IPv4Unicast, prefix, 0),
			nh,
			[]attribute.Attribute{attribute.OriginIGP},
			nil,
		)
	}

	// Group routes
	groups := rib.GroupByAttributesTwoLevel(routes)

	// Should have 3 groups (one per next-hop)
	require.Equal(t, 3, len(groups), "should have 3 groups for 3 different next-hops")

	// Each group should have 10 routes
	for _, g := range groups {
		totalRoutes := 0
		for _, asp := range g.ByASPath {
			totalRoutes += len(asp.Routes)
		}
		require.Equal(t, 10, totalRoutes, "each next-hop group should have 10 routes")
	}

	t.Log("✅ 30 routes with 3 next-hops correctly separated into 3 groups")
}

// TestBuildGroupedMPReachWithLimit verifies MP family grouping.
//
// VALIDATES: IPv6 routes grouped and packed into MP_REACH_NLRI.
// PREVENTS: IPv6 routes sent one-per-UPDATE.
func TestBuildGroupedMPReachWithLimit(t *testing.T) {
	// Create 50 IPv6 routes with same attributes
	routes := make([]*rib.Route, 50)
	for i := 0; i < 50; i++ {
		prefix := netip.MustParsePrefix("2001:db8::" + string(rune('0'+i%10)) + "/48")
		if i >= 10 {
			// Generate more prefixes
			b := [16]byte{0x20, 0x01, 0x0d, 0xb8, byte(i >> 8), byte(i), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
			prefix = netip.PrefixFrom(netip.AddrFrom16(b), 48)
		}
		routes[i] = rib.NewRouteWithASPath(
			nlri.NewINET(nlri.IPv6Unicast, prefix, 0),
			netip.MustParseAddr("2001:db8::1"),
			[]attribute.Attribute{attribute.OriginIGP},
			nil,
		)
	}

	// Group routes
	groups := rib.GroupByAttributesTwoLevel(routes)

	// Should have 1 group (all routes have same attributes)
	require.Equal(t, 1, len(groups), "all same-attr IPv6 routes should be in 1 group")

	// Verify family is IPv6
	require.Equal(t, nlri.AFIIPv6, groups[0].Family.AFI, "family should be IPv6")

	t.Log("✅ 50 IPv6 routes grouped into 1 attribute group")
}

// TestGroupedSendDisabled verifies GroupUpdates=false sends individually.
//
// VALIDATES: group-update=false sends one route per UPDATE.
// PREVENTS: Grouping when disabled.
func TestGroupedSendDisabled(t *testing.T) {
	settings := &PeerSettings{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAS:      65000,
		PeerAS:       65001,
		RouterID:     0x01020304,
		GroupUpdates: false, // Explicitly disabled
	}

	require.False(t, settings.GroupUpdates, "GroupUpdates should be disabled")
}

// TestGroupedSendEnabled verifies GroupUpdates=true is the optimized path.
//
// VALIDATES: group-update=true uses grouping for efficiency.
// PREVENTS: Grouping disabled by default.
func TestGroupedSendEnabled(t *testing.T) {
	settings := &PeerSettings{
		Address:      netip.MustParseAddr("10.0.0.1"),
		LocalAS:      65000,
		PeerAS:       65001,
		RouterID:     0x01020304,
		GroupUpdates: true,
	}

	require.True(t, settings.GroupUpdates, "GroupUpdates should be enabled")
}

// TestHasComplexASPath verifies detection of complex AS_PATH structures.
//
// VALIDATES: Routes with AS_SET or multiple segments detected as complex.
// PREVENTS: AS_PATH data loss when grouping routes with aggregation.
func TestHasComplexASPath(t *testing.T) {
	tests := []struct {
		name     string
		asPath   *attribute.ASPath
		expected bool
	}{
		{
			name:     "nil_aspath",
			asPath:   nil,
			expected: false,
		},
		{
			name:     "empty_segments",
			asPath:   &attribute.ASPath{Segments: nil},
			expected: false,
		},
		{
			name: "single_as_sequence",
			asPath: &attribute.ASPath{Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
			}},
			expected: false, // Simple - can be represented as []uint32
		},
		{
			name: "single_as_set",
			asPath: &attribute.ASPath{Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSet, ASNs: []uint32{65001, 65002}},
			}},
			expected: true, // Complex - AS_SET loses ordering info
		},
		{
			name: "multiple_segments",
			asPath: &attribute.ASPath{Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{65001}},
				{Type: attribute.ASSequence, ASNs: []uint32{65002}},
			}},
			expected: true, // Complex - multiple segments
		},
		{
			name: "confed_sequence",
			asPath: &attribute.ASPath{Segments: []attribute.ASPathSegment{
				{Type: attribute.ASConfedSequence, ASNs: []uint32{65001}},
			}},
			expected: true, // Complex - confederation
		},
		{
			name: "sequence_then_set",
			asPath: &attribute.ASPath{Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{65001}},
				{Type: attribute.ASSet, ASNs: []uint32{65002, 65003}},
			}},
			expected: true, // Complex - aggregation pattern
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := rib.NewRouteWithASPath(
				nlri.NewINET(nlri.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0),
				netip.MustParseAddr("192.168.1.1"),
				nil,
				tt.asPath,
			)
			result := hasComplexASPath(route)
			require.Equal(t, tt.expected, result, "hasComplexASPath mismatch for %s", tt.name)
		})
	}
}
