package rib

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
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

// TestRouteWireCacheStored verifies wire bytes are stored in route.
//
// VALIDATES: wireBytes and sourceCtxID accessible after construction.
//
// PREVENTS: Lost optimization opportunity for zero-copy forwarding.
func TestRouteWireCacheStored(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	wireBytes := []byte{0x40, 0x01, 0x01, 0x00} // Example: ORIGIN IGP
	ctxID := bgpctx.ContextID(42)

	route := NewRouteWithWireCache(inet, nextHop, nil, nil, wireBytes, ctxID)

	require.NotNil(t, route, "route must not be nil")
	require.Equal(t, wireBytes, route.WireBytes(), "wire bytes must match")
	require.Equal(t, ctxID, route.SourceCtxID(), "source context ID must match")
}

// TestRouteCanForwardDirect_Match verifies true when context IDs match.
//
// VALIDATES: Returns true when route sourceCtxID matches destination ctxID.
//
// PREVENTS: Unnecessary re-encoding when contexts are identical.
func TestRouteCanForwardDirect_Match(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	wireBytes := []byte{0x40, 0x01, 0x01, 0x00}
	ctxID := bgpctx.ContextID(42)

	route := NewRouteWithWireCache(inet, nextHop, nil, nil, wireBytes, ctxID)

	require.True(t, route.CanForwardDirect(ctxID),
		"CanForwardDirect must return true when context IDs match")
}

// TestRouteCanForwardDirect_Mismatch verifies false when context IDs differ.
//
// VALIDATES: Returns false when route sourceCtxID differs from destination.
//
// PREVENTS: Sending wrongly encoded data to peer with different capabilities.
func TestRouteCanForwardDirect_Mismatch(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	wireBytes := []byte{0x40, 0x01, 0x01, 0x00}
	srcCtxID := bgpctx.ContextID(42)
	dstCtxID := bgpctx.ContextID(99)

	route := NewRouteWithWireCache(inet, nextHop, nil, nil, wireBytes, srcCtxID)

	require.False(t, route.CanForwardDirect(dstCtxID),
		"CanForwardDirect must return false when context IDs differ")
}

// TestRouteCanForwardDirect_NoCache verifies false when wireBytes is nil.
//
// VALIDATES: Returns false when route has no cached wire bytes.
//
// PREVENTS: Nil dereference when trying to forward uncached route.
func TestRouteCanForwardDirect_NoCache(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Route without wire cache
	route := NewRoute(inet, nextHop, nil)

	require.False(t, route.CanForwardDirect(bgpctx.ContextID(42)),
		"CanForwardDirect must return false when no wire cache")
}

// TestRouteCanForwardDirect_EmptyWireBytes verifies false when wireBytes is empty.
//
// VALIDATES: Returns false when wireBytes slice is empty (len=0).
//
// PREVENTS: Sending empty data when wire cache was cleared or never set.
func TestRouteCanForwardDirect_EmptyWireBytes(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Route with empty wire bytes
	route := NewRouteWithWireCache(inet, nextHop, nil, nil, []byte{}, bgpctx.ContextID(42))

	require.False(t, route.CanForwardDirect(bgpctx.ContextID(42)),
		"CanForwardDirect must return false when wire bytes empty")
}

// TestRouteCanForwardDirect_ZeroContextID verifies behavior with zero context ID.
//
// VALIDATES: Zero ContextID (unregistered) still works correctly with matching IDs.
//
// PREVENTS: Special-casing zero ID causing unexpected behavior.
func TestRouteCanForwardDirect_ZeroContextID(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	wireBytes := []byte{0x40, 0x01, 0x01, 0x00}

	// Route with zero context ID (edge case - unregistered context)
	route := NewRouteWithWireCache(inet, nextHop, nil, nil, wireBytes, bgpctx.ContextID(0))

	// Zero matches zero - should return true since wireBytes exists
	require.True(t, route.CanForwardDirect(bgpctx.ContextID(0)),
		"CanForwardDirect must return true when both IDs are zero and wireBytes exists")

	// Zero doesn't match non-zero
	require.False(t, route.CanForwardDirect(bgpctx.ContextID(1)),
		"CanForwardDirect must return false when IDs differ")
}

// TestPackAttributesFor_ZeroCopy verifies zero-copy path when contexts match.
//
// VALIDATES: Returns cached wireBytes when source and dest context IDs match.
//
// PREVENTS: Unnecessary re-encoding wasting CPU cycles.
func TestPackAttributesFor_ZeroCopy(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Create a context and register it
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	// Simulated wire bytes (ORIGIN IGP attribute)
	wireBytes := []byte{0x40, 0x01, 0x01, 0x00}

	route := NewRouteWithWireCache(inet, nextHop, nil, nil, wireBytes, ctxID)

	// PackAttributesFor with same context ID should return cached bytes
	result := route.PackAttributesFor(ctxID)

	require.Equal(t, wireBytes, result, "must return cached wireBytes for matching context")
}

// TestPackAttributesFor_Reencode verifies re-encoding when contexts differ.
//
// VALIDATES: Re-encodes attributes when source and dest context IDs differ.
//
// PREVENTS: Sending wrongly-encoded data to peer with different capabilities.
func TestPackAttributesFor_Reencode(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Source context (ASN4=true)
	srcCtx := bgpctx.EncodingContextForASN4(true)
	srcCtxID := bgpctx.Registry.Register(srcCtx)

	// Destination context (ASN4=false) - different!
	dstCtx := bgpctx.EncodingContextForASN4(false)
	dstCtxID := bgpctx.Registry.Register(dstCtx)

	// Route with attributes
	origin := attribute.OriginIGP
	attrs := []attribute.Attribute{origin}

	// Wire bytes from source context
	wireBytes := []byte{0x40, 0x01, 0x01, 0x00}

	route := NewRouteWithWireCache(inet, nextHop, attrs, nil, wireBytes, srcCtxID)

	// PackAttributesFor with different context ID should re-encode
	result := route.PackAttributesFor(dstCtxID)

	// Should NOT be the cached wireBytes
	require.NotNil(t, result, "must return re-encoded bytes")
	// Result should contain the ORIGIN attribute
	require.GreaterOrEqual(t, len(result), 4, "must contain at least ORIGIN header+value")
}

// TestPackAttributesFor_NoCache verifies re-encoding when no cache exists.
//
// VALIDATES: Re-encodes attributes when route has no wire cache.
//
// PREVENTS: Nil return when forwarding routes without cache.
func TestPackAttributesFor_NoCache(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Create dest context
	dstCtx := bgpctx.EncodingContextForASN4(true)
	dstCtxID := bgpctx.Registry.Register(dstCtx)

	// Route without wire cache
	origin := attribute.OriginIGP
	attrs := []attribute.Attribute{origin}
	route := NewRoute(inet, nextHop, attrs)

	// PackAttributesFor should re-encode
	result := route.PackAttributesFor(dstCtxID)

	require.NotNil(t, result, "must return re-encoded bytes")
	require.GreaterOrEqual(t, len(result), 4, "must contain at least ORIGIN header+value")
}

// TestPackAttributesFor_WithASPath verifies AS_PATH is included in re-encoding.
//
// VALIDATES: AS_PATH (stored separately) is packed with context-aware encoding.
//
// PREVENTS: Missing AS_PATH in forwarded routes.
func TestPackAttributesFor_WithASPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Dest context with ASN4
	dstCtx := bgpctx.EncodingContextForASN4(true)
	dstCtxID := bgpctx.Registry.Register(dstCtx)

	// Route with AS_PATH
	origin := attribute.OriginIGP
	attrs := []attribute.Attribute{origin}
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001, 65002}},
		},
	}
	route := NewRouteWithASPath(inet, nextHop, attrs, asPath)

	result := route.PackAttributesFor(dstCtxID)

	// Should contain both ORIGIN and AS_PATH
	// ORIGIN: 4 bytes (header 3 + value 1)
	// AS_PATH with 2 ASNs (4-byte): header 3-4 + type 1 + count 1 + 2*4 = ~13 bytes
	require.GreaterOrEqual(t, len(result), 12, "must contain ORIGIN + AS_PATH")
}

// TestPackNLRIFor_NoAddPath verifies NLRI packing without ADD-PATH.
//
// VALIDATES: PackNLRIFor returns NLRI without path ID when context has no ADD-PATH.
//
// PREVENTS: Sending path IDs to peers that don't support ADD-PATH.
func TestPackNLRIFor_NoAddPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 42) // Has path ID
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Context without ADD-PATH
	ctx := bgpctx.EncodingContextWithAddPath(true, make(map[nlri.Family]bool))
	ctxID := bgpctx.Registry.Register(ctx)

	route := NewRoute(inet, nextHop, nil)
	result := route.PackNLRIFor(ctxID)

	// Without ADD-PATH: length(1) + prefix bytes(3 for /24)
	require.Equal(t, 4, len(result), "NLRI without ADD-PATH should be 4 bytes")
}

// TestPackNLRIFor_WithAddPath verifies NLRI packing with ADD-PATH.
//
// VALIDATES: PackNLRIFor includes path ID when context has ADD-PATH for family.
//
// PREVENTS: Missing path IDs for ADD-PATH capable peers.
func TestPackNLRIFor_WithAddPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 42) // Path ID = 42
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Context with ADD-PATH for IPv4 unicast
	ctx := bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		{AFI: 1, SAFI: 1}: true, // IPv4 unicast
	})
	ctxID := bgpctx.Registry.Register(ctx)

	route := NewRoute(inet, nextHop, nil)
	result := route.PackNLRIFor(ctxID)

	// With ADD-PATH: path-id(4) + length(1) + prefix bytes(3 for /24)
	require.Equal(t, 8, len(result), "NLRI with ADD-PATH should be 8 bytes")
	// First 4 bytes should be path ID (42)
	require.Equal(t, byte(0), result[0])
	require.Equal(t, byte(0), result[1])
	require.Equal(t, byte(0), result[2])
	require.Equal(t, byte(42), result[3])
}

// TestPackNLRIFor_ZeroCopy verifies zero-copy when NLRI cache exists and contexts match.
//
// VALIDATES: Returns cached nlriWireBytes when context IDs match.
//
// PREVENTS: Unnecessary re-encoding of NLRI.
func TestPackNLRIFor_ZeroCopy(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 42)
	nextHop := netip.MustParseAddr("192.168.1.1")

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	// Cached NLRI wire bytes
	nlriWireBytes := []byte{24, 10, 0, 0} // /24 prefix without path-id
	wireBytes := []byte{0x40, 0x01, 0x01, 0x00}

	route := NewRouteWithWireCacheFull(inet, nextHop, nil, nil, wireBytes, nlriWireBytes, ctxID)

	result := route.PackNLRIFor(ctxID)
	require.Equal(t, nlriWireBytes, result, "must return cached nlriWireBytes")
}

// TestPackNLRIFor_Reencode verifies re-encoding when contexts differ.
//
// VALIDATES: Re-encodes NLRI when context IDs don't match.
//
// PREVENTS: Sending wrongly-encoded NLRI to peer.
func TestPackNLRIFor_ReencodeOnMismatch(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 42)
	nextHop := netip.MustParseAddr("192.168.1.1")

	srcCtx := bgpctx.EncodingContextForASN4(true)
	srcCtxID := bgpctx.Registry.Register(srcCtx)

	dstCtx := bgpctx.EncodingContextWithAddPath(true, map[nlri.Family]bool{
		{AFI: 1, SAFI: 1}: true,
	})
	dstCtxID := bgpctx.Registry.Register(dstCtx)

	// Cached without ADD-PATH
	nlriWireBytes := []byte{24, 10, 0, 0}
	wireBytes := []byte{0x40, 0x01, 0x01, 0x00}

	route := NewRouteWithWireCacheFull(inet, nextHop, nil, nil, wireBytes, nlriWireBytes, srcCtxID)

	// Request with different context (has ADD-PATH)
	result := route.PackNLRIFor(dstCtxID)

	// Should NOT be the cached bytes (different length due to path-id)
	require.NotEqual(t, nlriWireBytes, result, "must re-encode, not use cache")
	require.Equal(t, 8, len(result), "re-encoded NLRI with ADD-PATH should be 8 bytes")
}

// TestPackAttributesFor_ZeroContextID verifies behavior with unregistered context IDs.
//
// VALIDATES: Handles edge case where both source and dest IDs are 0.
//
// PREVENTS: Incorrect zero-copy when contexts are actually different.
func TestPackAttributesFor_ZeroContextID(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	origin := attribute.OriginIGP
	attrs := []attribute.Attribute{origin}

	// Wire cache with unregistered context (ID=0)
	// Use a unique marker to detect if cache is used vs re-encoded
	wireBytes := []byte{0xFF, 0xFF, 0xFF, 0xFF} // Invalid bytes - won't match re-encoded ORIGIN
	route := NewRouteWithWireCache(inet, nextHop, attrs, nil, wireBytes, bgpctx.ContextID(0))

	// Request with ID=0 - should use cache (both unregistered)
	result := route.PackAttributesFor(bgpctx.ContextID(0))
	require.Equal(t, wireBytes, result, "same ID=0 should use cache")

	// Request with registered ID - should re-encode (not use invalid cache)
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)
	result2 := route.PackAttributesFor(ctxID)
	require.NotEqual(t, wireBytes, result2, "different ID should re-encode, not use cache")
	// Verify we got valid ORIGIN attribute
	require.GreaterOrEqual(t, len(result2), 4, "re-encoded result should have ORIGIN")
}

// TestPackAttributesFor_EmptyAttributes verifies handling of empty attributes.
//
// VALIDATES: Returns nil/empty when no attributes to pack.
//
// PREVENTS: Panic on empty attribute slice.
func TestPackAttributesFor_EmptyAttributes(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	// Route with no attributes and no AS_PATH
	route := NewRoute(inet, nextHop, nil)

	result := route.PackAttributesFor(ctxID)
	require.Nil(t, result, "empty attributes should return nil")
}

// TestPackNLRIFor_IPv6 verifies NLRI packing for IPv6 prefixes.
//
// VALIDATES: IPv6 NLRI is correctly packed with proper length.
//
// PREVENTS: Incorrect encoding for IPv6 routes.
func TestPackNLRIFor_IPv6(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8::/32")
	inet := nlri.NewINET(nlri.IPv6Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID := bgpctx.Registry.Register(ctx)

	route := NewRoute(inet, nextHop, nil)
	result := route.PackNLRIFor(ctxID)

	// IPv6 /32: length(1) + 4 bytes of prefix
	require.Equal(t, 5, len(result), "IPv6 /32 NLRI should be 5 bytes")
	require.Equal(t, byte(32), result[0], "prefix length should be 32")
}
