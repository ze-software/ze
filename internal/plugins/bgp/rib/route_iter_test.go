package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// TestRouteAttrIterator verifies Route exposes attribute iterator.
//
// VALIDATES: Route.AttrIterator() returns iterator over wireBytes.
// PREVENTS: Inability to iterate attributes without parsed slice.
func TestRouteAttrIterator(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Build wire bytes for ORIGIN + MED
	wireBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED 100
	}

	route := NewRouteWithWireCache(inet, nextHop, nil, nil, wireBytes, 1)

	iter := route.AttrIterator()
	require.NotNil(t, iter, "AttrIterator must not return nil when wireBytes present")

	// First: ORIGIN
	typeCode, _, value, ok := iter.Next()
	require.True(t, ok, "expected first attribute")
	assert.Equal(t, attribute.AttrOrigin, typeCode)
	assert.Equal(t, 1, len(value))

	// Second: MED
	typeCode, _, value, ok = iter.Next()
	require.True(t, ok, "expected second attribute")
	assert.Equal(t, attribute.AttrMED, typeCode)
	assert.Equal(t, 4, len(value))

	// No more
	_, _, _, ok = iter.Next()
	assert.False(t, ok)
}

// TestRouteAttrIteratorNoWireBytes verifies nil when no wireBytes.
//
// VALIDATES: AttrIterator returns nil when route has no wire cache.
// PREVENTS: Panic on routes without wire cache.
func TestRouteAttrIteratorNoWireBytes(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Route without wire cache
	route := NewRoute(inet, nextHop, nil)

	iter := route.AttrIterator()
	assert.Nil(t, iter, "AttrIterator must return nil when no wireBytes")
}

// TestRouteASPathIterator verifies Route exposes AS-PATH iterator.
//
// VALIDATES: Route.ASPathIterator() returns iterator over AS-PATH.
// PREVENTS: Inability to iterate AS-PATH without parsed struct.
func TestRouteASPathIterator(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Build wire bytes with AS_PATH attribute
	// AS_PATH: AS_SEQUENCE [65001, 65002]
	asPathValue := []byte{
		0x02, 0x02, // AS_SEQUENCE, 2 ASNs
		0x00, 0x00, 0xFD, 0xE9, // 65001
		0x00, 0x00, 0xFD, 0xEA, // 65002
	}
	wireBytes := make([]byte, 3+len(asPathValue))
	wireBytes[0] = 0x40 // flags: transitive
	wireBytes[1] = 0x02 // type: AS_PATH
	wireBytes[2] = byte(len(asPathValue))
	copy(wireBytes[3:], asPathValue)

	route := NewRouteWithWireCache(inet, nextHop, nil, nil, wireBytes, 1)

	iter := route.ASPathIterator(true) // asn4=true
	require.NotNil(t, iter, "ASPathIterator must not return nil")

	// First segment: AS_SEQUENCE
	segType, asns, ok := iter.Next()
	require.True(t, ok, "expected first segment")
	assert.Equal(t, attribute.ASSequence, segType)

	// Iterate ASNs
	asnIter := attribute.NewASNIterator(asns, true)
	asn, ok := asnIter.Next()
	require.True(t, ok)
	assert.Equal(t, uint32(65001), asn)

	asn, ok = asnIter.Next()
	require.True(t, ok)
	assert.Equal(t, uint32(65002), asn)

	_, ok = asnIter.Next()
	assert.False(t, ok)
}

// TestRouteASPathIteratorNoASPath verifies nil when no AS-PATH.
//
// VALIDATES: ASPathIterator returns nil when route has no AS-PATH in wireBytes.
// PREVENTS: Panic on routes without AS-PATH.
func TestRouteASPathIteratorNoASPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	// Wire bytes with only ORIGIN, no AS_PATH
	wireBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
	}

	route := NewRouteWithWireCache(inet, nextHop, nil, nil, wireBytes, 1)

	iter := route.ASPathIterator(true)
	assert.Nil(t, iter, "ASPathIterator must return nil when no AS_PATH")
}

// TestRouteZeroCopy verifies iterator uses same buffer as wireBytes.
//
// VALIDATES: Iterator data is view into original wireBytes (no copy).
// PREVENTS: Hidden allocations defeating zero-copy goal.
func TestRouteZeroCopy(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	wireBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64, // MED 100
	}

	route := NewRouteWithWireCache(inet, nextHop, nil, nil, wireBytes, 1)

	iter := route.AttrIterator()
	require.NotNil(t, iter)

	_, _, value, ok := iter.Next()
	require.True(t, ok)

	// Verify value is a view into original wireBytes
	// Value should be at offset 3 (after flags+type+len), length 1
	assert.Equal(t, byte(0x00), value[0], "value should be IGP (0)")

	// Modify original wireBytes and verify iterator slice reflects change (zero-copy proof)
	// Since value is a slice into wireBytes, modifying wireBytes[3] should affect value[0]
	route.WireBytes()[3] = 0x02 // Change ORIGIN to INCOMPLETE
	assert.Equal(t, byte(0x02), value[0], "iterator should see modified value (zero-copy)")
}
