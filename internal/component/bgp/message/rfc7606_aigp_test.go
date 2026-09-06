package message

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// AIGP flag handling on the receive path (RFC 7311 Section 3.2), enforced inside the
// RFC 7606 attribute walk because the action Section 3.2 asks for IS RFC 7606's
// "attribute discard".
//
// Both tests build the SAME UPDATE and change one bit: the AIGP flags byte. Everything
// else -- the mandatory attributes, the AIGP TLV, the metric -- is identical, so the only
// thing either test can be reacting to is the transitive bit.

// aigpAttrPrefix is the well-formed part of an AIGP attribute: type code 26, length 11,
// and one type-1 TLV (RFC 7311 Section 3) carrying the metric 1234. The flags byte in
// front of it is what each test below supplies.
const aigpAttrPrefix = "1a0b" + "01000b" + "00000000000004d2"

// aigpUpdateAttrs builds a valid IPv4 unicast UPDATE's path attributes with one AIGP
// attribute carrying the given flags byte.
func aigpUpdateAttrs(t *testing.T, aigpFlags string) []byte {
	t.Helper()
	const mandatory = "400101" + "00" + // ORIGIN = IGP
		"400200" + // AS_PATH, empty
		"400304" + "c0000201" // NEXT_HOP = 192.0.2.1
	attrs, err := hex.DecodeString(mandatory + aigpFlags + aigpAttrPrefix)
	require.NoError(t, err)
	return attrs
}

// requireMandatoryAttrsIntact checks that the three well-known mandatory attributes came
// through a discard untouched, which is what "the UPDATE continues to be processed" means
// on the wire.
func requireMandatoryAttrsIntact(t *testing.T, attrs []byte) {
	t.Helper()
	for _, one := range []struct {
		code  attribute.AttributeCode
		value string
	}{
		{attribute.AttrOrigin, "00"},
		{attribute.AttrASPath, ""},
		{attribute.AttrNextHop, "c0000201"},
	} {
		_, _, value, found := attribute.AttrFind(attrs, one.code)
		require.True(t, found, "attribute %d must survive the AIGP discard", one.code)
		require.Equal(t, one.value, hex.EncodeToString(value),
			"attribute %d value must be untouched by the AIGP discard", one.code)
	}
}

// TestRFC7606AIGPTransitiveIsDiscarded drives the RFC 7606 attribute walk with an AIGP
// attribute whose flags byte is 0xC0, and follows the verdict through ApplyAttrDiscard to
// the bytes a peer would be handed.
//
// VALIDATES: the walk answers attribute-discard naming code 26 with the malformed reason,
// the AIGP codepoint is gone from the rebuilt attributes, an ATTR_TOMBSTONE records which
// attribute went and why, and ORIGIN, AS_PATH and NEXT_HOP are byte-identical.
// PREVENTS: ze accepting and re-advertising the malformed AIGP form, which is what it did
// until this check existed, and prevents the fix being taken too far into a
// treat-as-withdraw or a session reset that would cost the peer its routes.
//
// RFC requirement: RFC7311-3.2-4 negative -- an AIGP attribute received with the transitive
// bit set is malformed: validateAttributeFlags answers RFC7606ActionAttributeDiscard on
// attribute code 26 with DiscardReasonMalformedValue, ApplyAttrDiscard removes the AIGP
// codepoint and leaves an ATTR_TOMBSTONE naming code 26 and that reason, and the ORIGIN,
// AS_PATH and NEXT_HOP attributes of the same UPDATE are unchanged.
func TestRFC7606AIGPTransitiveIsDiscarded(t *testing.T) {
	attrs := aigpUpdateAttrs(t, "c0") // optional AND transitive: the malformed form

	result := ValidateUpdateRFC7606(attrs, true /*hasNLRI*/, false /*isIBGP*/, true /*asn4*/)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action,
		"RFC 7311 Section 3.2 asks for attribute discard, not treat-as-withdraw or session reset")
	require.Equal(t, uint8(attribute.AttrAIGP), result.AttrCode)
	require.Equal(t, []DiscardEntry{{Code: uint8(attribute.AttrAIGP), Reason: DiscardReasonMalformedValue}},
		result.DiscardEntries)
	require.Contains(t, result.Description, "RFC 7311 Section 3.2")

	out, _ := ApplyAttrDiscard(attrs, result.DiscardEntries)

	_, _, _, found := attribute.AttrFind(out, attribute.AttrAIGP)
	require.False(t, found, "the malformed AIGP must not survive to be passed along")

	require.Equal(t, []DiscardEntry{{Code: uint8(attribute.AttrAIGP), Reason: DiscardReasonMalformedValue}},
		ExtractUpstreamAttrDiscard(out),
		"the discard must be readable as 'AIGP was discarded', not as 'no AIGP was present'")

	requireMandatoryAttrsIntact(t, out)
}

// TestRFC7606AIGPNonTransitiveIsKept is the same UPDATE with the transitive bit clear,
// which is the form RFC 7311 Section 3 defines and the form (*attribute.AIGP).Flags emits.
//
// VALIDATES: a conformant AIGP produces no error action at all and its bytes are left
// exactly as the peer sent them.
// PREVENTS: the check above being written on the codepoint rather than on the bit, which
// would discard every AIGP ze receives and take the attribute out of service.
//
// RFC requirement: RFC7311-3.2-4 positive -- an AIGP attribute received with the transitive
// bit clear (flags 0x80) is not malformed: the walk answers RFC7606ActionNone, records no
// discard entry, and the AIGP attribute keeps its flags byte and its metric TLV verbatim.
func TestRFC7606AIGPNonTransitiveIsKept(t *testing.T) {
	attrs := aigpUpdateAttrs(t, "80") // optional, non-transitive: the conformant form

	result := ValidateUpdateRFC7606(attrs, true /*hasNLRI*/, false /*isIBGP*/, true /*asn4*/)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"a non-transitive AIGP is well-formed and must not be discarded")
	require.Empty(t, result.DiscardEntries)

	_, flags, value, found := attribute.AttrFind(attrs, attribute.AttrAIGP)
	require.True(t, found, "a conformant AIGP must survive the walk")
	require.Equal(t, attribute.FlagOptional, flags)
	require.Equal(t, "01000b00000000000004d2", hex.EncodeToString(value),
		"the metric TLV must be untouched")

	requireMandatoryAttrsIntact(t, attrs)
}
