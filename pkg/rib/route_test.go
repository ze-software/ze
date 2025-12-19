package rib

import (
	"net/netip"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/stretchr/testify/require"
)

// TestRouteCreation verifies that routes can be created with NLRI and attributes.
//
// VALIDATES: Basic route construction with NLRI, next-hop, and attributes.
//
// PREVENTS: Nil pointer panics when creating routes, incorrect field assignment.
func TestRouteCreation(t *testing.T) {
	// Create an INET NLRI
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)

	// Create attributes
	origin := attribute.OriginIGP
	attrs := []attribute.Attribute{origin}

	// Create next-hop
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Create route
	route := NewRoute(inet, nextHop, attrs)

	require.NotNil(t, route, "route must not be nil")
	require.Equal(t, inet, route.NLRI(), "NLRI must match")
	require.Equal(t, nextHop, route.NextHop(), "next-hop must match")
	require.Len(t, route.Attributes(), 1, "must have 1 attribute")
}

// TestRouteWithASPath verifies that routes store AS-PATH separately for deduplication.
//
// VALIDATES: AS-PATH is accessible as part of route identity (novel approach
// where AS-PATH is treated like ADD-PATH path-id for better attribute sharing).
//
// PREVENTS: Loss of AS-PATH data, inability to use AS-PATH for route indexing.
func TestRouteWithASPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)

	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}

	nextHop := netip.MustParseAddr("192.168.1.1")
	route := NewRouteWithASPath(inet, nextHop, nil, asPath)

	require.NotNil(t, route.ASPath(), "AS-PATH must not be nil")
	require.Len(t, route.ASPath().Segments, 1, "must have 1 segment")
	require.Equal(t, []uint32{65001, 65002}, route.ASPath().Segments[0].ASNs)
}

// TestRouteIndex verifies that route index includes NLRI and optionally AS-PATH.
//
// VALIDATES: Route indexing for RIB storage and lookup. Index must be stable
// and include NLRI wire format + AS-PATH hash when present.
//
// PREVENTS: Route collisions in RIB where different routes have same index,
// causing one to overwrite another.
func TestRouteIndex(t *testing.T) {
	prefix1 := netip.MustParsePrefix("10.0.0.0/24")
	prefix2 := netip.MustParsePrefix("10.0.1.0/24")
	inet1 := nlri.NewINET(nlri.IPv4Unicast, prefix1, 0)
	inet2 := nlri.NewINET(nlri.IPv4Unicast, prefix2, 0)

	nextHop := netip.MustParseAddr("192.168.1.1")

	route1 := NewRoute(inet1, nextHop, nil)
	route2 := NewRoute(inet2, nextHop, nil)

	// Different prefixes must have different indexes
	require.NotEqual(t, route1.Index(), route2.Index(),
		"different prefixes must have different indexes")

	// Same prefix must have same index
	route1b := NewRoute(inet1, nextHop, nil)
	require.Equal(t, route1.Index(), route1b.Index(),
		"same prefix must have same index")
}

// TestRouteIndexWithASPath verifies that AS-PATH affects route index.
//
// VALIDATES: Novel approach where AS-PATH is part of route identity (like
// ADD-PATH path-id). Same NLRI with different AS-PATH = different routes.
//
// PREVENTS: Route overwriting when same prefix arrives via different AS paths.
// This is critical for route diversity and BGP add-path scenarios.
func TestRouteIndexWithASPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	asPath1 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}
	asPath2 := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65002}},
		},
	}

	route1 := NewRouteWithASPath(inet, nextHop, nil, asPath1)
	route2 := NewRouteWithASPath(inet, nextHop, nil, asPath2)

	// Same NLRI but different AS-PATH must have different indexes
	require.NotEqual(t, route1.Index(), route2.Index(),
		"same NLRI with different AS-PATH must have different indexes")
}

// TestRouteRefCount verifies reference counting for route lifecycle management.
//
// VALIDATES: Routes can be reference counted for memory management,
// allowing multiple RIB entries to share the same route object.
//
// PREVENTS: Memory leaks where routes are never freed, or use-after-free
// where routes are freed while still referenced.
func TestRouteRefCount(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	route := NewRoute(inet, nextHop, nil)

	require.Equal(t, int32(1), route.RefCount(), "initial refcount must be 1")

	route.Acquire()
	require.Equal(t, int32(2), route.RefCount(), "refcount must be 2 after acquire")

	route.Release()
	require.Equal(t, int32(1), route.RefCount(), "refcount must be 1 after release")

	route.Release()
	require.Equal(t, int32(0), route.RefCount(), "refcount must be 0 after final release")
}
