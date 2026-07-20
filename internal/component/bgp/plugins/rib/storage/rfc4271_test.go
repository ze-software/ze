package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// findAttrFlags walks a path-attribute section and returns the flags octet of the first
// attribute with the given type code.
func findAttrFlags(t *testing.T, wire []byte, code byte) byte {
	t.Helper()
	pos := 0
	for pos+3 <= len(wire) {
		flags := wire[pos]
		typeCode := wire[pos+1]
		var length, hdr int
		if flags&0x10 != 0 {
			require.LessOrEqual(t, pos+4, len(wire))
			length = int(wire[pos+2])<<8 | int(wire[pos+3])
			hdr = 4
		} else {
			length = int(wire[pos+2])
			hdr = 3
		}
		if typeCode == code {
			return flags
		}
		pos += hdr + length
	}
	t.Fatalf("attribute %d not found in %x", code, wire)
	return 0
}

// TestRFC4271PartialBitClearedOnReadvertisedWellKnown verifies the readvertise encoder
// normalizes well-known and optional non-transitive flags, clearing the Partial bit.
//
// VALIDATES: An ORIGIN and a LOCAL_PREF received with the Partial bit set, and a MED
// received with the Partial bit set, are re-emitted with 0x40 / 0x40 / 0x80.
//
// PREVENTS: Propagating "partial" on attributes for which the RFC forbids it.
//
// RFC requirement: RFC4271-4.3-2 negative -- a non-conformant Partial bit on a well-known
// or optional non-transitive attribute does not survive readvertisement: ToWireBytes
// writes each pooled attribute with a fixed flags octet (0x40 well-known, 0x80 optional
// non-transitive), so the received 0x20 is dropped
// (internal/component/bgp/plugins/rib/storage/familyrib.go:785-797,799-813, appendAttrWire
// at :882-893).
func TestRFC4271PartialBitClearedOnReadvertisedWellKnown(t *testing.T) {
	raw := []byte{
		0x60, 0x01, 0x01, 0x00, // ORIGIN with Partial set (0x40|0x20)
		0x60, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64, // LOCAL_PREF with Partial set
		0xA0, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a, // MED with Partial set (0x80|0x20)
	}
	entry, err := ParseAttributes(raw, true)
	require.NoError(t, err)
	defer entry.Release()

	wire, err := entry.ToWireBytes()
	require.NoError(t, err)

	assert.Equal(t, byte(0x40), findAttrFlags(t, wire, byte(attribute.AttrOrigin)),
		"ORIGIN re-emitted well-known transitive, Partial clear")
	assert.Equal(t, byte(0x40), findAttrFlags(t, wire, byte(attribute.AttrLocalPref)),
		"LOCAL_PREF re-emitted well-known transitive, Partial clear")
	assert.Equal(t, byte(0x80), findAttrFlags(t, wire, byte(attribute.AttrMED)),
		"MED re-emitted optional non-transitive, Partial clear")
}

// TestRFC4271PartialBitPreservedOnUnknownTransitive verifies a Partial bit already set by
// an upstream AS is carried forward unchanged.
//
// VALIDATES: An unrecognized optional transitive attribute received with flags 0xE0
// (Optional|Transitive|Partial) is stored and re-emitted with the Partial bit still set.
//
// PREVENTS: Silently downgrading partial information to complete.
//
// RFC requirement: RFC4271-5-4 positive -- appendOtherAttr stores the received flags
// octet verbatim, including the Partial bit
// (internal/component/bgp/plugins/rib/storage/attrparse.go:254-260), and parseOtherAttrs
// rebuilds the wire header from that stored octet
// (internal/component/bgp/plugins/rib/storage/familyrib.go:845-869).
func TestRFC4271PartialBitPreservedOnUnknownTransitive(t *testing.T) {
	const unknownCode = 0xFA
	raw := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN
		0xE0, unknownCode, 0x03, 0x01, 0x02, 0x03, // unknown optional transitive, Partial set
	}
	entry, err := ParseAttributes(raw, true)
	require.NoError(t, err)
	defer entry.Release()

	wire, err := entry.ToWireBytes()
	require.NoError(t, err)

	flags := findAttrFlags(t, wire, unknownCode)
	assert.Equal(t, byte(0x20), flags&0x20, "Partial bit still set after readvertisement")
	assert.Equal(t, byte(0x80), flags&0x80, "Optional bit preserved")
	assert.Equal(t, byte(0x40), flags&0x40, "Transitive bit preserved")
}

// TestRFC4271PartialBitSurvivesLengthReframing verifies the one code path that rewrites
// the flags octet of a stored unknown attribute does not clear Partial.
//
// VALIDATES: An unknown optional transitive attribute longer than 255 octets, received
// with the Partial bit set and the Extended Length bit set, is re-emitted with the
// Extended Length bit recomputed and the Partial bit still set.
//
// PREVENTS: A flags rewrite in the length-reframing branch resetting Partial to 0.
//
// RFC requirement: RFC4271-5-4 negative -- the reframing branch that recomputes the
// Extended Length bit touches only 0x10 and never clears the Partial bit set by a previous
// AS (internal/component/bgp/plugins/rib/storage/familyrib.go:860-867).
func TestRFC4271PartialBitSurvivesLengthReframing(t *testing.T) {
	const unknownCode = 0xFB
	value := make([]byte, 300)
	for i := range value {
		value[i] = byte(i)
	}
	raw := []byte{0x40, 0x01, 0x01, 0x00}
	raw = append(raw, 0xF0, unknownCode, byte(len(value)>>8), byte(len(value))) // 0x80|0x40|0x20|0x10
	raw = append(raw, value...)

	entry, err := ParseAttributes(raw, true)
	require.NoError(t, err)
	defer entry.Release()

	wire, err := entry.ToWireBytes()
	require.NoError(t, err)

	flags := findAttrFlags(t, wire, unknownCode)
	assert.Equal(t, byte(0x10), flags&0x10, "extended length recomputed for a 300-octet value")
	assert.Equal(t, byte(0x20), flags&0x20, "Partial bit not reset to 0 by the reframing branch")
}

// TestRFC4271OverlappingRoutesBothInstalled verifies a less specific and a more specific
// route for overlapping address space are both kept and both selectable.
//
// VALIDATES: 10.0.0.0/8 and 10.0.0.0/24 coexist in the RIB and each resolves to its own
// entry.
//
// PREVENTS: A more specific announcement silently displacing its covering aggregate.
//
// RFC requirement: RFC4271-9.2-4 positive -- the RIB keys on the full NLRI, so a less
// specific and a more specific overlapping route are considered as two distinct routes
// rather than one (internal/component/bgp/plugins/rib/storage/familyrib.go:143-215).
// RFC requirement: RFC4271-9.2-5 positive -- when both overlapping routes are accepted
// both are installed, so a Loc-RIB mirror of the RIB carries both
// (internal/component/bgp/plugins/rib/storage/familyrib.go:196-207).
func TestRFC4271OverlappingRoutesBothInstalled(t *testing.T) {
	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	less := []byte{8, 10}
	more := []byte{24, 10, 0, 0}

	rib.Insert(concat(wireOriginIGP, wireLocalPref100), less, true)
	rib.Insert(concat(wireOriginIGP, wireMED100), more, true)

	assert.Equal(t, 2, rib.Len(), "both the covering and the more specific route are held")

	lessEntry, ok := rib.LookupEntry(less)
	require.True(t, ok, "10.0.0.0/8 present")
	moreEntry, ok := rib.LookupEntry(more)
	require.True(t, ok, "10.0.0.0/24 present")
	assert.True(t, lessEntry.GetBundle().HasLocalPref(), "covering route keeps its own attributes")
	assert.True(t, moreEntry.GetBundle().HasMED(), "more specific keeps its own attributes")
}

// TestRFC4271SamePrefixReplacesRatherThanAccumulates verifies the two-routes outcome is
// specific to overlapping (different) NLRI, not a RIB that never replaces.
//
// VALIDATES: A second announcement for the identical prefix replaces the first, leaving
// one entry with the newer attributes.
//
// PREVENTS: Reading "both overlapping routes installed" as "every announcement is kept".
//
// RFC requirement: RFC4271-9.2-4 negative -- overlapping routes are considered separately
// only because their NLRI differ: an identical NLRI replaces the older route in place
// rather than adding a second entry
// (internal/component/bgp/plugins/rib/storage/familyrib.go:196-207).
// RFC requirement: RFC4271-9.2-5 negative -- installing both is conditioned on the two
// routes covering different address space; the same prefix yields exactly one installed
// route (internal/component/bgp/plugins/rib/storage/familyrib.go:196-207).
// RFC requirement: RFC4271-9-2 positive -- a new route with NLRI identical to an existing
// route replaces the older route in the Adj-RIB-In
// (internal/component/bgp/plugins/rib/storage/familyrib.go:196-207).
func TestRFC4271SamePrefixReplacesRatherThanAccumulates(t *testing.T) {
	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	nlriBytes := []byte{24, 10, 0, 0}
	rib.Insert(concat(wireOriginIGP, wireLocalPref100), nlriBytes, true)
	require.Equal(t, 1, rib.Len())

	rib.Insert(concat(wireOriginIGP, wireMED100), nlriBytes, true)
	assert.Equal(t, 1, rib.Len(), "identical NLRI replaces, it does not accumulate")

	entry, ok := rib.LookupEntry(nlriBytes)
	require.True(t, ok)
	assert.True(t, entry.GetBundle().HasMED(), "the newer route's attributes are in place")
	assert.False(t, entry.GetBundle().HasLocalPref(), "the older route's attributes are gone")
}

// TestRFC4271WithdrawRemovesFromAdjRIBIn verifies a withdrawal removes the route.
//
// VALIDATES: Remove drops the entry and a later lookup misses.
//
// PREVENTS: A withdrawn prefix lingering in the Adj-RIB-In and staying selectable.
//
// RFC requirement: RFC4271-9-1 positive -- FamilyRIB.Remove deletes the withdrawn route
// from the Adj-RIB-In (internal/component/bgp/plugins/rib/storage/familyrib.go:317-356).
// RFC requirement: RFC4271-9-2 negative -- removal is keyed on the exact NLRI: withdrawing
// a different prefix leaves the stored route in place, so replacement and removal are not
// blanket operations (internal/component/bgp/plugins/rib/storage/familyrib.go:332-340).
// RFC requirement: RFC4271-9-1 negative -- the removal is not a blanket flush: only the
// withdrawn NLRI is dropped from the Adj-RIB-In and every other stored route survives, so
// a withdrawal for a prefix that was never announced removes nothing
// (internal/component/bgp/plugins/rib/storage/familyrib.go:317-356).
func TestRFC4271WithdrawRemovesFromAdjRIBIn(t *testing.T) {
	rib := NewFamilyRIB(family.IPv4Unicast, false)
	defer rib.Release()

	kept := []byte{24, 10, 0, 1}
	gone := []byte{24, 10, 0, 0}
	rib.Insert(concat(wireOriginIGP, wireLocalPref100), kept, true)
	rib.Insert(concat(wireOriginIGP, wireLocalPref100), gone, true)
	require.Equal(t, 2, rib.Len())

	rib.Remove(gone)
	_, ok := rib.LookupEntry(gone)
	assert.False(t, ok, "withdrawn route removed from the Adj-RIB-In")
	_, ok = rib.LookupEntry(kept)
	assert.True(t, ok, "an unrelated prefix is untouched by the withdrawal")
	assert.Equal(t, 1, rib.Len())
}
