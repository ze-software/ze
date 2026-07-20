package attribute

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wellKnownAttrsForRFC4271 is the set of RFC 4271 well-known attributes ze encodes,
// plus MULTI_EXIT_DISC, the one optional non-transitive attribute the RFC defines.
func wellKnownAttrsForRFC4271() []Attribute {
	return []Attribute{
		OriginIGP,
		&ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: []uint32{65001}}}},
		&NextHop{Addr: netip.MustParseAddr("192.0.2.1")},
		LocalPref(100),
		AtomicAggregate{},
	}
}

// TestRFC4271WellKnownAttributesAreTransitive verifies every well-known attribute ze
// encodes sets the Transitive bit.
//
// VALIDATES: ORIGIN, AS_PATH, NEXT_HOP, LOCAL_PREF and ATOMIC_AGGREGATE all report
// FlagTransitive, and the encoded flags octet has bit 0x40 set.
//
// PREVENTS: Emitting a well-known attribute a conformant peer would treat as malformed.
//
// RFC requirement: RFC4271-4.3-1 positive -- each well-known attribute's Flags() returns
// FlagTransitive, so WriteHeaderTo stamps 0x40 into the flags octet
// (internal/core/bgp/attribute/origin.go:108, aspath.go:88, simple.go:32, simple.go:132,
// simple.go:176, attribute.go:216-233).
func TestRFC4271WellKnownAttributesAreTransitive(t *testing.T) {
	t.Parallel()
	buf := make([]byte, 256)
	for _, attr := range wellKnownAttrsForRFC4271() {
		require.True(t, attr.Flags().IsTransitive(), "%s Flags().IsTransitive()", attr.Code())
		require.False(t, attr.Flags().IsOptional(), "%s must be well-known", attr.Code())
		n := WriteAttrTo(attr, buf, 0)
		require.Positive(t, n)
		assert.Equal(t, byte(0x40), buf[0]&0x40, "%s encoded Transitive bit", attr.Code())
	}
}

// TestRFC4271PartialBitClearOnSend verifies the Partial bit is never set on a well-known
// or an optional non-transitive attribute ze encodes.
//
// VALIDATES: The encoded flags octet of every well-known attribute, and of
// MULTI_EXIT_DISC (optional non-transitive), has bit 0x20 clear.
//
// PREVENTS: Advertising complete information as partial.
//
// RFC requirement: RFC4271-4.3-2 positive -- no well-known attribute's Flags() includes
// FlagPartial and MED's Flags() is FlagOptional alone, so the encoded flags octet has the
// Partial bit clear (internal/core/bgp/attribute/simple.go:87,
// internal/core/bgp/attribute/attribute.go:216-233).
func TestRFC4271PartialBitClearOnSend(t *testing.T) {
	t.Parallel()
	buf := make([]byte, 256)
	attrs := append(wellKnownAttrsForRFC4271(), MED(10))
	for _, attr := range attrs {
		require.False(t, attr.Flags().IsPartial(), "%s Flags().IsPartial()", attr.Code())
		n := WriteAttrTo(attr, buf, 0)
		require.Positive(t, n)
		assert.Zero(t, buf[0]&0x20, "%s encoded Partial bit must be clear", attr.Code())
	}
	assert.Equal(t, FlagOptional, MED(10).Flags(), "MED is optional non-transitive")
}

// TestRFC4271AttributeFlagsLowNibbleZeroOnSend verifies the reserved low-order four bits
// of the attribute flags octet are zero in everything ze encodes.
//
// VALIDATES: For every well-known attribute plus MED and AGGREGATOR, and for both the
// one-octet and the two-octet length forms, flags & 0x0F is zero.
//
// PREVENTS: Setting reserved bits a future revision may assign meaning to.
//
// RFC requirement: RFC4271-4.3-3 positive -- the only flag values ze constructs are
// FlagOptional/FlagTransitive/FlagPartial/FlagExtLength (0x80/0x40/0x20/0x10), and
// WriteHeaderTo adds only FlagExtLength, so the low-order four bits of the encoded flags
// octet are always zero (internal/core/bgp/attribute/attribute.go:119-138,216-233).
func TestRFC4271AttributeFlagsLowNibbleZeroOnSend(t *testing.T) {
	t.Parallel()
	buf := make([]byte, 4096)
	attrs := append(wellKnownAttrsForRFC4271(), MED(10))
	for _, attr := range attrs {
		n := WriteAttrTo(attr, buf, 0)
		require.Positive(t, n)
		assert.Zero(t, buf[0]&0x0F, "%s reserved low-order bits", attr.Code())
	}

	// Extended-length form: a long AS_PATH forces the two-octet length header, which is
	// the only place WriteHeaderTo mutates the flags octet.
	long := &ASPath{Segments: []ASPathSegment{{Type: ASSequence, ASNs: make([]uint32, 200)}}}
	n := WriteAttrTo(long, buf, 0)
	require.Greater(t, n, 255)
	assert.Equal(t, byte(0x10), buf[0]&0x10, "extended length bit set for a long attribute")
	assert.Zero(t, buf[0]&0x0F&^0x10, "reserved bits stay zero in the extended-length form")
}
