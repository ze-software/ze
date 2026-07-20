// RFC: rfc/short/rfc8669.md — BGP Prefix-SID attribute (code 40)
// Overview: rfc7606.go — validatePrefixSIDAttr walks the attribute's TLVs
// Related: attr_discard.go — ApplyAttrDiscard removes a discarded attribute from the wire
//
// RFC 8669 §6 gives the Prefix-SID attribute "attribute discard" error handling: a TLV
// whose length runs past the attribute, or trailing bytes after the last TLV, mean the
// attribute is ignored and not re-advertised. §3 requires unknown TLVs to be ignored,
// and §3.1/§3.2 require the Label-Index Reserved/Flags and the Originator SRGB Flags
// fields to be ignored on reception. These tests drive the real receive-side validator.

package message

import (
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
)

var (
	rfc8669Origin  = []byte{0x40, 0x01, 0x01, 0x00}                   // ORIGIN = IGP
	rfc8669ASPath  = []byte{0x40, 0x02, 0x00}                         // AS_PATH (empty, valid per RFC 7606 §4)
	rfc8669NextHop = []byte{0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01} // NEXT_HOP = 192.0.2.1
)

// rfc8669Attr wraps a Prefix-SID attribute value in an optional-transitive
// (flags 0xC0) attribute header for code 40.
func rfc8669Attr(value []byte) []byte {
	out := []byte{0xC0, 40, byte(len(value))}
	return append(out, value...)
}

// rfc8669LabelIndexTLV builds a Label-Index TLV (type 1, length 7) with caller-chosen
// Reserved and Flags octets so reception-side "MUST be ignored" rules can be driven with
// both conforming (zero) and non-conforming (non-zero) values.
func rfc8669LabelIndexTLV(reserved byte, flags uint16, index uint32) []byte {
	return []byte{
		1, 0, 7,
		reserved,
		byte(flags >> 8), byte(flags),
		byte(index >> 24), byte(index >> 16), byte(index >> 8), byte(index),
	}
}

// rfc8669SRGBTLV builds an Originator SRGB TLV (type 3) with one base/range entry and a
// caller-chosen Flags field.
func rfc8669SRGBTLV(flags uint16, base, count uint32) []byte {
	return []byte{
		3, 0, 8,
		byte(flags >> 8), byte(flags),
		byte(base >> 16), byte(base >> 8), byte(base),
		byte(count >> 16), byte(count >> 8), byte(count),
	}
}

// rfc8669Update concatenates the well-known mandatory attributes with a Prefix-SID
// attribute carrying the supplied TLVs.
func rfc8669Update(tlvs ...[]byte) []byte {
	var value []byte
	for _, t := range tlvs {
		value = append(value, t...)
	}
	out := append([]byte{}, rfc8669Origin...)
	out = append(out, rfc8669ASPath...)
	out = append(out, rfc8669NextHop...)
	return append(out, rfc8669Attr(value)...)
}

// TestRFC8669UnknownTLVIsIgnored feeds a Prefix-SID attribute whose only TLV carries an
// unallocated type code.
//
// VALIDATES: RFC 8669 §3 — an unknown TLV is ignored: it neither invalidates the
// attribute nor triggers any RFC 7606 action, so the attribute survives on the wire and
// is propagated unmodified (validatePrefixSIDAttr inspects only types 5 and 6, and no
// receive path rewrites the attribute value).
// PREVENTS: a validator that rejects the extension mechanism RFC 9252 relies on, which
// would blackhole every future Prefix-SID TLV type.
//
// RFC requirement: RFC8669-3-1 positive -- a Prefix-SID attribute whose TLV type is unallocated is accepted unchanged; the unknown TLV is ignored and its bytes are left intact for propagation.
func TestRFC8669UnknownTLVIsIgnored(t *testing.T) {
	unknown := []byte{99, 0, 4, 0xde, 0xad, 0xbe, 0xef}
	pathAttrs := rfc8669Update(unknown)
	before := append([]byte{}, pathAttrs...)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"an unknown Prefix-SID TLV must be ignored, not treated as an error: %s", result.Description)
	require.Equal(t, before, pathAttrs,
		"the unknown TLV must be propagated unmodified: validation must not rewrite the attribute")
}

// TestRFC8669LabelIndexReservedAndFlagsZeroAccepted is the conforming half of the
// §3.1 reception rules.
//
// RFC requirement: RFC8669-3.1-4 positive -- a Label-Index TLV with a zero Reserved octet is accepted.
// RFC requirement: RFC8669-3.1-6 positive -- a Label-Index TLV with a zero Flags field is accepted.
func TestRFC8669LabelIndexReservedAndFlagsZeroAccepted(t *testing.T) {
	pathAttrs := rfc8669Update(rfc8669LabelIndexTLV(0, 0, 777))

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"a conforming Label-Index TLV must be accepted: %s", result.Description)
}

// TestRFC8669LabelIndexNonZeroReservedIgnored sets the Reserved octet a conforming sender
// would clear.
//
// VALIDATES: RFC 8669 §3.1 — the Reserved octet is ignored on reception: a non-zero value
// changes nothing about how the attribute is handled.
// PREVENTS: a receiver that rejects an UPDATE from a sender which sets bits ze does not
// know about, the exact interop failure "ignored on receipt" exists to avoid.
//
// RFC requirement: RFC8669-3.1-4 negative -- a Label-Index TLV whose Reserved octet is non-zero is still accepted, proving the field is ignored rather than validated.
func TestRFC8669LabelIndexNonZeroReservedIgnored(t *testing.T) {
	pathAttrs := rfc8669Update(rfc8669LabelIndexTLV(0xFF, 0, 777))
	before := append([]byte{}, pathAttrs...)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"a non-zero Label-Index Reserved octet must be ignored, never an error: %s", result.Description)
	require.Equal(t, before, pathAttrs, "an ignored field must not be rewritten on the wire")
}

// TestRFC8669LabelIndexNonZeroFlagsIgnored is the Flags analog of the Reserved case.
//
// RFC requirement: RFC8669-3.1-6 negative -- a Label-Index TLV whose Flags field is non-zero is still accepted, proving the field is ignored rather than validated.
func TestRFC8669LabelIndexNonZeroFlagsIgnored(t *testing.T) {
	pathAttrs := rfc8669Update(rfc8669LabelIndexTLV(0, 0xFFFF, 777))
	before := append([]byte{}, pathAttrs...)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"non-zero Label-Index Flags must be ignored, never an error: %s", result.Description)
	require.Equal(t, before, pathAttrs, "an ignored field must not be rewritten on the wire")
}

// TestRFC8669SRGBFlagsZeroAccepted is the conforming half of the §3.2 reception rule.
//
// RFC requirement: RFC8669-3.2-2 positive -- an Originator SRGB TLV with a zero Flags field is accepted.
func TestRFC8669SRGBFlagsZeroAccepted(t *testing.T) {
	pathAttrs := rfc8669Update(rfc8669LabelIndexTLV(0, 0, 300), rfc8669SRGBTLV(0, 800000, 4096))

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"a conforming Originator SRGB TLV must be accepted: %s", result.Description)
}

// TestRFC8669SRGBNonZeroFlagsIgnored sets the SRGB Flags field a conforming sender clears.
//
// RFC requirement: RFC8669-3.2-2 negative -- an Originator SRGB TLV whose Flags field is non-zero is still accepted, proving the field is ignored rather than validated.
func TestRFC8669SRGBNonZeroFlagsIgnored(t *testing.T) {
	pathAttrs := rfc8669Update(rfc8669LabelIndexTLV(0, 0, 300), rfc8669SRGBTLV(0xFFFF, 800000, 4096))

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"non-zero Originator SRGB Flags must be ignored, never an error: %s", result.Description)
}

// TestRFC8669SRGBUnchangedThroughReceiveValidation compares the SRGB TLV bytes before and
// after the receive-side validator has walked the attribute.
//
// VALIDATES: RFC 8669 §3.2 — the Originator SRGB TLV is not changed while the UPDATE is
// processed. validatePrefixSIDAttr only reads TLV headers, and the sole modification any
// receive path makes to attribute 40 is the whole-attribute ATTR_DISCARD overwrite that
// applies when the attribute is malformed or policy-discarded, never a value edit.
// PREVENTS: a receiver that normalizes or re-bases another speaker's SRGB, which would
// hand downstream speakers a label range their originator never advertised.
//
// RFC requirement: RFC8669-3.2-3 positive -- a well-formed Originator SRGB TLV traverses receive validation byte-for-byte unchanged.
func TestRFC8669SRGBUnchangedThroughReceiveValidation(t *testing.T) {
	srgb := rfc8669SRGBTLV(0, 1000000, 5000)
	pathAttrs := rfc8669Update(rfc8669LabelIndexTLV(0, 0, 300), srgb)
	before := append([]byte{}, pathAttrs...)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action, "the UPDATE is well formed: %s", result.Description)
	require.Equal(t, before, pathAttrs, "the Originator SRGB TLV must not be changed during processing")

	_, _, value, found := attribute.AttrFind(pathAttrs, attribute.AttrPrefixSID)
	require.True(t, found, "the Prefix-SID attribute must still be present")
	require.Equal(t, srgb, value[10:], "the SRGB TLV bytes must be identical to those received")
}

// TestRFC8669WellFormedAttributeAccepted is the conforming half of the §6 error handling.
//
// RFC requirement: RFC8669-6-1 positive -- a Prefix-SID attribute whose TLVs exactly fill the attribute is accepted and kept on the wire.
func TestRFC8669WellFormedAttributeAccepted(t *testing.T) {
	pathAttrs := rfc8669Update(rfc8669LabelIndexTLV(0, 0, 777), rfc8669SRGBTLV(0, 800000, 4096))

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"a well-formed Prefix-SID attribute must not be discarded: %s", result.Description)

	_, _, _, found := attribute.AttrFind(pathAttrs, attribute.AttrPrefixSID)
	require.True(t, found, "an accepted Prefix-SID attribute stays available for propagation")
}

// TestRFC8669MalformedAttributeDiscardedAndNotAdvertised drives a TLV whose declared
// length runs past the end of the attribute.
//
// VALIDATES: RFC 8669 §6 — a malformed Prefix-SID attribute is ignored (attribute discard,
// never a session reset or a withdraw of the prefix) AND is removed from the path
// attributes so it is not advertised to other BGP peers.
// PREVENTS: silently propagating an attribute ze itself could not parse, and the
// over-reaction of tearing the session down for an optional-transitive attribute.
//
// RFC requirement: RFC8669-6-1 negative -- a Prefix-SID TLV length that exceeds the attribute bounds yields attribute discard and the attribute is stripped from the path attributes rather than re-advertised.
func TestRFC8669MalformedAttributeDiscardedAndNotAdvertised(t *testing.T) {
	// Label-Index TLV claiming length 200 inside a 10-octet attribute value.
	overrun := []byte{1, 0, 200, 0, 0, 0, 0, 0, 0, 42}
	pathAttrs := rfc8669Update(overrun)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action,
		"a TLV length past the attribute bound is attribute-discard, not withdraw or reset: %s",
		result.Description)
	require.Equal(t, uint8(40), result.AttrCode)
	require.Contains(t, result.Description, "RFC 8669 Section 6")

	newAttrs, _ := ApplyAttrDiscard(pathAttrs, result.DiscardEntries)
	_, _, _, found := attribute.AttrFind(newAttrs, attribute.AttrPrefixSID)
	require.False(t, found,
		"a discarded Prefix-SID attribute must not remain in the attributes advertised onward")
}

// TestRFC8669TrailingBytesDiscarded covers the other §6 malformed shape: a complete TLV
// followed by bytes that cannot form a TLV header.
//
// RFC requirement: RFC8669-6-1 negative -- trailing bytes after the last complete TLV make the attribute malformed and it is discarded rather than partially honored.
func TestRFC8669TrailingBytesDiscarded(t *testing.T) {
	trailing := append(rfc8669LabelIndexTLV(0, 0, 777), 0x01)
	pathAttrs := rfc8669Update(trailing)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action,
		"trailing bytes make the attribute malformed: %s", result.Description)
	require.Equal(t, uint8(40), result.AttrCode)
	require.Contains(t, result.Description, "trailing bytes")
}

// TestRFC8669DuplicateAttributeFirstOccurrenceWins puts a VALID Prefix-SID first and a
// MALFORMED copy second. Only the copy that is kept is validated
// (rfc7606.go:283 skips an already-seen code), so the outcome separates first-wins from
// last-wins: first-wins reports no error, last-wins reports attribute discard.
//
// VALIDATES: RFC 8669 §6 — when the attribute appears more than once, occurrences after
// the first are discarded unexamined.
// PREVENTS: a peer overriding a valid Prefix-SID by appending a second copy.
//
// RFC requirement: RFC8669-6-2 positive -- with a valid first occurrence and a malformed duplicate, the duplicate is discarded unexamined and the UPDATE is accepted.
func TestRFC8669DuplicateAttributeFirstOccurrenceWins(t *testing.T) {
	valid := rfc8669Attr(rfc8669LabelIndexTLV(0, 0, 777))
	malformed := rfc8669Attr([]byte{1, 0, 200, 0, 0, 0, 0, 0, 0, 42})

	pathAttrs := append([]byte{}, rfc8669Origin...)
	pathAttrs = append(pathAttrs, rfc8669ASPath...)
	pathAttrs = append(pathAttrs, rfc8669NextHop...)
	pathAttrs = append(pathAttrs, valid...)
	pathAttrs = append(pathAttrs, malformed...)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"the first Prefix-SID is valid and the duplicate must be discarded unexamined; "+
			"an error here would mean the LAST occurrence was kept: %s", result.Description)
}

// TestRFC8669DuplicateAttributeCannotRepairFirst is the converse: a MALFORMED first copy
// followed by a VALID duplicate.
//
// RFC requirement: RFC8669-6-2 negative -- a later duplicate cannot repair or replace a malformed first occurrence, which is still discarded.
func TestRFC8669DuplicateAttributeCannotRepairFirst(t *testing.T) {
	malformed := rfc8669Attr([]byte{1, 0, 200, 0, 0, 0, 0, 0, 0, 42})
	valid := rfc8669Attr(rfc8669LabelIndexTLV(0, 0, 777))

	pathAttrs := append([]byte{}, rfc8669Origin...)
	pathAttrs = append(pathAttrs, rfc8669ASPath...)
	pathAttrs = append(pathAttrs, rfc8669NextHop...)
	pathAttrs = append(pathAttrs, malformed...)
	pathAttrs = append(pathAttrs, valid...)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action,
		"the malformed FIRST occurrence is what gets validated; no error here would mean "+
			"the valid LAST occurrence had replaced it: %s", result.Description)
	require.Equal(t, uint8(40), result.AttrCode)
}
