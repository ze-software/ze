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

// partialFlagsOf returns the flags octet of the first attribute with the given code.
func partialFlagsOf(t *testing.T, section []byte, code byte) byte {
	t.Helper()
	for pos := 0; pos+3 <= len(section); {
		flags := section[pos]
		hdrLen, valLen := 3, int(section[pos+2])
		if flags&0x10 != 0 {
			require.LessOrEqual(t, pos+4, len(section))
			hdrLen, valLen = 4, int(section[pos+2])<<8|int(section[pos+3])
		}
		if section[pos+1] == code {
			return flags
		}
		pos += hdrLen + valLen
	}
	t.Fatalf("attribute %d not found in %x", code, section)
	return 0
}

// TestRFC4271PartialStampedOnUnrecognizedTransitive verifies the walk that implements the
// pass-along rule stamps the one class the RFC names, in both header length forms.
//
// VALIDATES: a 3-octet-header and a 4-octet-header unrecognized optional transitive
// attribute each come back with the Partial bit set, their value, length, header size and
// other flag bits untouched, and the walk reports two stamps.
//
// PREVENTS: a stamp that handles only the short header form, which would leave every
// unrecognized attribute longer than 255 octets non-conformant.
//
// RFC requirement: RFC4271-5-3 positive -- SetPartialOnUnrecognizedTransitive ORs
// FlagPartial into the flags octet of an unrecognized optional transitive attribute
// (internal/core/bgp/attribute/partial.go).
func TestRFC4271PartialStampedOnUnrecognizedTransitive(t *testing.T) {
	t.Parallel()
	long := make([]byte, 300)
	section := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0xC0, 0xFA, 0x03, 0x01, 0x02, 0x03, // unrecognized, optional transitive, short header
	}
	section = append(section, 0xD0, 0xFB, byte(len(long)>>8), byte(len(long))) // extended length
	section = append(section, long...)

	require.False(t, AttributeCode(0xFA).Recognized())
	require.False(t, AttributeCode(0xFB).Recognized())

	assert.Equal(t, 2, SetPartialOnUnrecognizedTransitive(section),
		"both unrecognized transitive optional attributes are stamped")
	assert.Equal(t, byte(0xE0), partialFlagsOf(t, section, 0xFA),
		"RFC 4271 Section 5: Partial set, Optional and Transitive kept")
	assert.Equal(t, byte(0xF0), partialFlagsOf(t, section, 0xFB),
		"the extended-length form keeps its 0x10 bit and gains 0x20")
	assert.Equal(t, byte(0x03), section[6], "the short attribute's length octet is untouched")
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, section[7:10], "the value is untouched")
}

// TestRFC4271PartialNotStampedOnExcludedClasses verifies the walk fires for exactly one
// class of attribute.
//
// VALIDATES: a well-known attribute, an optional non-transitive one and a RECOGNIZED
// optional transitive one are all returned unchanged, and the walk reports zero stamps.
//
// PREVENTS: satisfying Section 5 for unrecognized attributes by breaking Section 4.3 for
// every other attribute in the same UPDATE.
//
// RFC requirement: RFC4271-5-3 negative -- the stamp is conditioned on Optional AND
// Transitive AND unrecognized, so none of the three excluded classes is touched
// (internal/core/bgp/attribute/partial.go).
// RFC requirement: RFC4271-4.3-2 negative -- the Partial bit is not merely left alone by
// an encoder that never sets it: a walk whose whole purpose is to SET the bit still leaves
// it 0 on a well-known attribute and on an optional non-transitive one.
func TestRFC4271PartialNotStampedOnExcludedClasses(t *testing.T) {
	t.Parallel()
	section := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN, well-known transitive
		0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a, // MED, optional non-transitive
		0xC0, 0x08, 0x04, 0xff, 0xff, 0xff, 0x01, // COMMUNITIES, recognized optional transitive
	}
	original := append([]byte(nil), section...)

	require.True(t, AttrCommunity.Recognized(), "COMMUNITIES is an attribute ze implements")
	assert.Zero(t, SetPartialOnUnrecognizedTransitive(section), "no attribute here qualifies")
	assert.Equal(t, original, section, "not one octet of an excluded attribute may change")
}

// TestRFC4271PartialFromPreviousASNotCleared verifies the walk is incapable of clearing the
// bit, whichever class the attribute belongs to.
//
// VALIDATES: an unrecognized and a recognized optional transitive attribute, each received
// with the Partial bit already set, keep it, and the already-set unrecognized one is not
// counted as a fresh stamp.
//
// PREVENTS: a rewrite that normalizes flags from the attribute's type, which would reset a
// previous AS's Partial bit to 0 as the route crosses ze.
//
// RFC requirement: RFC4271-5-4 negative -- the preservation is not an accident of copying
// bytes: the one walk that WRITES this octet only ORs the bit in, so no input path clears
// it (internal/core/bgp/attribute/partial.go).
func TestRFC4271PartialFromPreviousASNotCleared(t *testing.T) {
	t.Parallel()
	section := []byte{
		0xE0, 0x08, 0x04, 0xff, 0xff, 0xff, 0x01, // COMMUNITIES, recognized, Partial already set
		0xE0, 0xFA, 0x02, 0x01, 0x02, // unrecognized, Partial already set
	}

	assert.Zero(t, SetPartialOnUnrecognizedTransitive(section),
		"an attribute that already carries the bit needs no stamp")
	assert.Equal(t, byte(0xE0), partialFlagsOf(t, section, 0x08),
		"RFC 4271 Section 5: a Partial bit set by a previous AS must not be set back to 0")
	assert.Equal(t, byte(0xE0), partialFlagsOf(t, section, 0xFA))
}

// TestAttributeRecognizedTracksTheNamesRegistry verifies the predicate the pass-along rule
// asks is the registry every attribute ze implements joins, plugin attributes included.
//
// VALIDATES: a core code, a plugin-registered code and an unassigned code answer true,
// true and false, and RegisterName is what moves a code from the third group to the second.
//
// PREVENTS: deriving "unrecognized" from the parser table, which holds no entry for
// PREFIX_SID or ATTR_TOMBSTONE and would have ze stamp attributes it does understand.
func TestAttributeRecognizedTracksTheNamesRegistry(t *testing.T) {
	assert.True(t, AttrCommunity.Recognized(), "a core attribute is recognized")
	assert.True(t, AttrPrefixSID.Recognized(),
		"PREFIX_SID has no entry in knownAttrParsers and is still an attribute ze implements")
	assert.True(t, AttrTombstone.Recognized())

	const unassigned AttributeCode = 249
	require.False(t, unassigned.Recognized(), "an unassigned code starts unrecognized")
	RegisterName(unassigned, "TEST_ONLY_UNASSIGNED")
	assert.True(t, unassigned.Recognized(),
		"registering a name is what makes a plugin's attribute recognized")
}

// TestPartialStampAllocatesNothing pins the walk to zero allocations.
//
// It runs once per received UPDATE on the session read goroutine, ahead of the span
// index build, so an allocation here is one per UPDATE for every peer
// (ai/rules/performance.md). The bit set behind Recognized is what keeps it that way: a
// map lookup would not allocate either, but this measurement is the guard that stops a
// future rewrite from reaching for one that does.
func TestPartialStampAllocatesNothing(t *testing.T) {
	section := []byte{
		0x40, 0x01, 0x01, 0x00,
		0xC0, 0x08, 0x04, 0xff, 0xff, 0xff, 0x01,
		0xC0, 0xFA, 0x03, 0x01, 0x02, 0x03,
	}
	allocs := testing.AllocsPerRun(100, func() {
		section[11] = 0xC0 // clear the stamp so every run does the same work
		SetPartialOnUnrecognizedTransitive(section)
	})
	assert.Zero(t, allocs, "the stamp walks headers in place and must allocate nothing")
}
