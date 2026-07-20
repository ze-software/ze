// VALIDATES: the RFC 9012 Tunnel Encapsulation attribute obligations ze's attribute
// layer actually carries -- an unrecognized tunnel type or sub-TLV is neither an error
// nor a reason to drop bytes, a malformed or meaningless sub-TLV is skipped while being
// propagated, duplicate single-instance sub-TLVs keep their first occurrence, and every
// reserved octet is re-advertised exactly as received.
// PREVENTS: a receiver that silently normalizes or strips the parts of the attribute it
// does not understand (which would break the tunnels of every speaker downstream), a
// malformed sub-TLV being read at a guessed offset, and a genuinely malformed attribute
// being waved through as if it were merely unrecognized.

package attribute

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	teTunnelTypeSRPolicy   uint16 = 15     // RFC 9830: SR Policy CP.
	teTunnelTypeVXLAN      uint16 = 8      // RFC 9012 Section 3.2.1.
	teTunnelTypeUnassigned uint16 = 0xFFFE // Not in the IANA tunnel type registry.

	teSubTLVEncapsulation  uint8 = 1   // RFC 9012 Section 3.2.
	teSubTLVColor          uint8 = 4   // RFC 9012 Section 3.4.2.
	teSubTLVEgressEndpoint uint8 = 6   // RFC 9012 Section 3.1.
	teSubTLVEmbeddedLabel  uint8 = 9   // RFC 9012 Section 3.5.
	teSubTLVUnknownShort   uint8 = 60  // Unassigned, 1-octet length.
	teSubTLVUnknownLong    uint8 = 200 // Unassigned, 2-octet length.
)

// teShort builds a sub-TLV with the 1-octet length header (types 0-127).
func teShort(styp uint8, value ...byte) []byte {
	return append([]byte{styp, byte(len(value))}, value...)
}

// teLong builds an unassigned sub-TLV with the 2-octet length header (types 128-255).
func teLong(value ...byte) []byte {
	hdr := []byte{teSubTLVUnknownLong, 0, 0}
	binary.BigEndian.PutUint16(hdr[1:3], uint16(len(value)))
	return append(hdr, value...)
}

// tePreference builds a conformant Preference sub-TLV (RFC 9830 Section 2.4.1):
// value = Flags(1) + RESERVED(1) + Preference(4), so Length is 6.
func tePreference(pref uint32) []byte {
	value := make([]byte, preferenceValueLen)
	binary.BigEndian.PutUint32(value[2:preferenceValueLen], pref)
	return teShort(SubTLVPreference, value...)
}

// teEgressEndpoint builds a Tunnel Egress Endpoint sub-TLV (RFC 9012 Section 3.1):
// Reserved(4) + Address Family(2) + Address(4 for IPv4).
func teEgressEndpoint(reserved byte) []byte {
	return teShort(teSubTLVEgressEndpoint,
		reserved, reserved, reserved, reserved,
		0x00, 0x01,
		10, 0, 0, 1)
}

// teVXLANEncap builds a VXLAN Encapsulation sub-TLV (RFC 9012 Section 3.2.1):
// Flags(1) + VN-ID(3) + MAC(6) + Reserved(2), 12 octets of value.
func teVXLANEncap(flags byte) []byte {
	return teShort(teSubTLVEncapsulation,
		flags,
		0x00, 0x00, 0x64,
		0x02, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x00)
}

// teCat concatenates sub-TLVs into a Tunnel TLV value.
func teCat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// teEncode renders an attribute value from tunnel TLVs, exactly as it would be
// re-advertised.
func teEncode(tlvs ...TunnelTLV) []byte {
	te := &TunnelEncap{TLVs: tlvs}
	buf := make([]byte, te.Len())
	te.WriteTo(buf, 0)
	return buf
}

// teRoundTrip parses an attribute value and re-encodes it, returning the bytes a
// downstream speaker would see.
func teRoundTrip(t *testing.T, raw []byte) []byte {
	t.Helper()
	te, err := ParseTunnelEncap(raw)
	require.NoError(t, err)
	buf := make([]byte, te.Len())
	te.WriteTo(buf, 0)
	return buf
}

// TestRFC9012UnrecognizedTunnelTypeIsCarried pins that a tunnel type ze does not know
// is parsed alongside the ones it does and re-advertised byte for byte.
func TestRFC9012UnrecognizedTunnelTypeIsCarried(t *testing.T) {
	t.Parallel()

	known := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(100)}
	unknown := TunnelTLV{
		TunnelType: teTunnelTypeUnassigned,
		Value:      teCat(teShort(teSubTLVUnknownShort, 0xDE, 0xAD), teLong(0xBE, 0xEF)),
	}
	raw := teEncode(known, unknown)

	// RFC requirement: RFC9012-13-3 positive -- an attribute holding a TLV whose tunnel type is not in the registry parses without error, and the recognized TLV beside it is still usable
	te, err := ParseTunnelEncap(raw)
	require.NoError(t, err, "an unrecognized tunnel type is not a malformed attribute")
	require.Len(t, te.TLVs, 2)
	assert.Equal(t, teTunnelTypeUnassigned, te.TLVs[1].TunnelType)
	pref, ok := te.TLVs[0].Preference()
	assert.True(t, ok)
	assert.Equal(t, uint32(100), pref, "the recognized TLV is unaffected by its unrecognized neighbor")

	// RFC requirement: RFC9012-13-5 positive -- re-encoding the parsed attribute reproduces the unrecognized TLV byte for byte, so it remains in the attribute when the route is propagated
	assert.Equal(t, raw, teRoundTrip(t, raw), "the unrecognized TLV survives propagation unchanged")
}

// TestRFC9012UnrecognizedTLVRemovalIsObservable is the counterpart: dropping the TLV ze
// does not understand produces different bytes, so "it remains in the attribute" is a
// property the test above can actually fail on.
func TestRFC9012UnrecognizedTLVRemovalIsObservable(t *testing.T) {
	t.Parallel()

	known := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(100)}
	unknown := TunnelTLV{TunnelType: teTunnelTypeUnassigned, Value: teShort(teSubTLVUnknownShort, 0xDE, 0xAD)}
	raw := teEncode(known, unknown)

	// RFC requirement: RFC9012-13-5 negative -- an attribute that keeps only the recognized TLV does not reproduce the received bytes and is shorter, so a speaker that dropped the unrecognized TLV would be caught
	stripped := teEncode(known)
	assert.NotEqual(t, raw, stripped)
	assert.Less(t, len(stripped), len(raw))
}

// TestRFC9012MalformedAttributeIsRejected pins the other side of "unrecognized is not
// malformed": framing that cannot be walked IS refused, so acceptance of unknown types
// and of duplicate or meaningless sub-TLVs is not blanket acceptance of anything.
func TestRFC9012MalformedAttributeIsRejected(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC9012-13-3 negative -- a TLV whose declared length runs past the end of the attribute is refused, so an attribute is not called well-formed merely because its tunnel type is unknown
	overrun := []byte{0xFF, 0xFE, 0x00, 0x10, 0x01, 0x02}
	_, err := ParseTunnelEncap(overrun)
	require.Error(t, err)

	// RFC requirement: RFC9012-13-18 negative -- a TLV header cut short is refused; only the sub-TLV cases the RFC names (unrecognized, meaningless, duplicate) are exempt from being malformed
	// RFC requirement: RFC9830-2.4-2 negative -- genuinely broken framing IS malformed, so "a duplicate single-instance sub-TLV is not malformed" is a carve-out for that case rather than blanket acceptance
	_, err = ParseTunnelEncap([]byte{0x00, 0x0F, 0x00})
	require.Error(t, err)

	// RFC requirement: RFC9012-13-8 negative -- a stray octet after the final TLV cannot start another TLV and is refused, so a well-formed parse means the TLV sequence really was complete
	trailing := append(teEncode(TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(10)}), 0x07)
	_, err = ParseTunnelEncap(trailing)
	require.Error(t, err)
}

// TestRFC9012AllSubTLVsPropagate pins that every sub-TLV of a TLV -- recognized or not,
// short header or long -- is enumerated and re-advertised.
func TestRFC9012AllSubTLVsPropagate(t *testing.T) {
	t.Parallel()

	value := teCat(
		tePreference(300),
		teShort(teSubTLVUnknownShort, 0x11, 0x22),
		teLong(0x33, 0x44, 0x55),
	)
	tlv := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: value}
	raw := teEncode(tlv)

	// RFC requirement: RFC9012-13-9 positive -- all three sub-TLVs are enumerated and the re-encoded attribute is byte-identical, so every sub-TLV is propagated with the route
	// RFC requirement: RFC9830-4.2.3-6 positive -- the TLV here is Tunnel Type 15, and the SR Policy information it carries is re-advertised octet for octet: propagation alters nothing
	stlvs, err := tlv.SubTLVs()
	require.NoError(t, err)
	require.Len(t, stlvs, 3)
	assert.Equal(t, raw, teRoundTrip(t, raw))

	// RFC requirement: RFC9012-13-11 positive -- both unrecognized sub-TLVs (one of each header width) are still present after the round trip, with their values intact
	assert.Equal(t, teSubTLVUnknownShort, stlvs[1].Type)
	assert.Equal(t, []byte{0x11, 0x22}, stlvs[1].Value)
	assert.Equal(t, teSubTLVUnknownLong, stlvs[2].Type)
	assert.Equal(t, []byte{0x33, 0x44, 0x55}, stlvs[2].Value)

	dropped := teEncode(TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(300)})
	// RFC requirement: RFC9012-13-9 negative -- an attribute carrying only the recognized sub-TLV differs from the received bytes, so silently dropping sub-TLVs on propagation is detectable
	// RFC requirement: RFC9830-4.2.3-6 negative -- altering the SR Policy TLV on the way out produces different, shorter bytes, so "unaltered" is a property this comparison can fail on
	assert.NotEqual(t, raw, dropped)
	// RFC requirement: RFC9012-13-11 negative -- the same comparison shows the unrecognized sub-TLVs are what makes up the difference: without them the attribute is shorter
	assert.Less(t, len(dropped), len(raw))
}

// TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing pins that a TLV is processed as
// if the sub-TLVs ze does not recognize were not there.
func TestRFC9012UnrecognizedSubTLVDoesNotDisturbProcessing(t *testing.T) {
	t.Parallel()

	surrounded := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(
		teLong(0x01, 0x02, 0x03),
		tePreference(4242),
		teShort(teSubTLVUnknownShort, 0x09),
	)}
	alone := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(4242)}

	// RFC requirement: RFC9012-13-10 positive -- the Preference read from a TLV wrapped in unrecognized sub-TLVs is the same as from a TLV holding the Preference alone, so the TLV is processed as if they were absent
	got, ok := surrounded.Preference()
	require.True(t, ok)
	want, ok := alone.Preference()
	require.True(t, ok)
	assert.Equal(t, want, got)
	assert.Equal(t, uint32(4242), got)

	// RFC requirement: RFC9012-13-10 negative -- a TLV whose sub-TLVs are all unrecognized yields nothing to process; an unrecognized sub-TLV is never mistaken for a recognized one
	onlyUnknown := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(
		teShort(teSubTLVUnknownShort, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00),
		teLong(0x00, 0x00, 0x00, 0x00, 0x00, 0x00),
	)}
	_, ok = onlyUnknown.Preference()
	assert.False(t, ok)
}

// TestRFC9012MalformedSubTLVTreatedAsUnrecognized pins that a sub-TLV whose value is not
// the length its type mandates is skipped -- not read at a guessed offset, not fatal to
// the TLV, and not removed from it.
func TestRFC9012MalformedSubTLVTreatedAsUnrecognized(t *testing.T) {
	t.Parallel()

	malformed := teShort(SubTLVPreference, 0x00, 0x00, 0x00, 0x63) // 4-octet value; RFC 9830 mandates 6.
	tlv := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(malformed, tePreference(777))}
	raw := teEncode(tlv)

	// RFC requirement: RFC9012-13-12 positive -- a Preference sub-TLV with a value of the wrong length is skipped exactly as an unrecognized type would be: the TLV still parses, the next well-formed Preference is used, and the malformed sub-TLV is still there after the round trip
	pref, ok := tlv.Preference()
	require.True(t, ok)
	assert.Equal(t, uint32(777), pref)
	stlvs, err := tlv.SubTLVs()
	require.NoError(t, err)
	require.Len(t, stlvs, 2)
	assert.Equal(t, raw, teRoundTrip(t, raw))

	// RFC requirement: RFC9012-13-12 negative -- the same sub-TLV type at its mandated length IS used, so the skip above is caused by the malformation and not by ignoring the type outright
	wellFormed := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(555)}
	pref, ok = wellFormed.Preference()
	require.True(t, ok)
	assert.Equal(t, uint32(555), pref)
}

// TestRFC9012DuplicateSingleInstanceSubTLVs pins the "first one wins, and the TLV is
// still fine" rule for a sub-TLV that may appear only once.
func TestRFC9012DuplicateSingleInstanceSubTLVs(t *testing.T) {
	t.Parallel()

	dup := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(tePreference(100), tePreference(200))}
	raw := teEncode(dup)

	// RFC requirement: RFC9012-13-7 positive -- with two Preference sub-TLVs the first is the one used and the second is disregarded
	// RFC requirement: RFC9830-2.4-1 positive -- the Preference sub-TLV is single-instance, so only the first instance is used and the later one is ignored
	pref, ok := dup.Preference()
	require.True(t, ok)
	assert.Equal(t, uint32(100), pref)

	// RFC requirement: RFC9012-13-8 positive -- the duplicate does not make the TLV malformed: it parses, both occurrences are enumerated, and both are re-advertised
	// RFC requirement: RFC9830-2.4-2 positive -- the ignored duplicate instance of a single-instance sub-TLV is not considered malformed: the attribute parses and is propagated with both instances intact
	te, err := ParseTunnelEncap(raw)
	require.NoError(t, err)
	stlvs, err := te.TLVs[0].SubTLVs()
	require.NoError(t, err)
	require.Len(t, stlvs, 2)
	assert.Equal(t, raw, teRoundTrip(t, raw))

	// RFC requirement: RFC9012-13-7 negative -- when only the second value is present it is the one returned, so "the first occurrence" is genuinely positional and not a fixed answer
	// RFC requirement: RFC9830-2.4-1 negative -- the later instance is not discarded on principle: standing alone it IS the instance used, so "only the first" is positional rather than a fixed preference for one value
	single := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(200)}
	pref, ok = single.Preference()
	require.True(t, ok)
	assert.Equal(t, uint32(200), pref)
}

// TestRFC9012MeaninglessSubTLVIgnoredNotRemoved pins the treatment of a sub-TLV that is
// well-formed but says nothing about the tunnel type carrying it.
func TestRFC9012MeaninglessSubTLVIgnoredNotRemoved(t *testing.T) {
	t.Parallel()

	// A VXLAN Encapsulation sub-TLV inside an SR Policy TLV: well-formed, meaningless here.
	meaningless := teVXLANEncap(0xC0)
	tlv := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(meaningless, tePreference(900))}
	raw := teEncode(tlv)

	// RFC requirement: RFC9012-13-18 positive -- a sub-TLV meaningless for the tunnel type carrying it does not make the TLV malformed: the attribute parses and both sub-TLVs are enumerated
	te, err := ParseTunnelEncap(raw)
	require.NoError(t, err)
	stlvs, err := te.TLVs[0].SubTLVs()
	require.NoError(t, err)
	require.Len(t, stlvs, 2)

	// RFC requirement: RFC9012-13-16 positive -- the meaningless sub-TLV is disregarded: the value processed out of the TLV is the same as if it were not present at all
	// RFC requirement: RFC9830-2.3-3 positive -- a VXLAN Encapsulation sub-TLV has no defined applicability to the SR Policy SAFI, and inside a Tunnel Type 15 TLV it is ignored: what ze processes is what it would process without it
	withMeaningless, ok := tlv.Preference()
	require.True(t, ok)
	bare := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(900)}
	without, ok := bare.Preference()
	require.True(t, ok)
	assert.Equal(t, without, withMeaningless)

	// RFC requirement: RFC9012-13-16 negative -- a sub-TLV that IS meaningful for this tunnel type is not disregarded: changing the Preference changes what is processed, so "disregarded" is not "everything is ignored"
	// RFC requirement: RFC9830-2.3-3 negative -- the Preference sub-TLV, which the SR Policy SAFI does define, is not ignored, so the rule is scoped to the sub-TLVs without applicability
	other := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(meaningless, tePreference(901))}
	changed, ok := other.Preference()
	require.True(t, ok)
	assert.NotEqual(t, withMeaningless, changed)

	// RFC requirement: RFC9012-13-19 positive -- the disregarded sub-TLV is still in the attribute after the round trip, so it is not stripped before the route is distributed
	assert.Equal(t, raw, teRoundTrip(t, raw))

	// RFC requirement: RFC9012-13-19 negative -- an attribute built without the meaningless sub-TLV differs from the received bytes, so removing it on distribution is detectable
	assert.NotEqual(t, raw, teEncode(TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(900)}))
}

// TestRFC9012ReservedOctetsPropagateUnchanged pins that reserved octets are carried, not
// normalized: ze re-advertises what it received even where the RFC says the originator
// should have sent zero.
func TestRFC9012ReservedOctetsPropagateUnchanged(t *testing.T) {
	t.Parallel()

	dirty := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teEgressEndpoint(0xA5)}
	rawDirty := teEncode(dirty)
	clean := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teEgressEndpoint(0x00)}

	// RFC requirement: RFC9012-3.1-3 positive -- a Tunnel Egress Endpoint sub-TLV whose Reserved subfield is non-zero is re-advertised with those octets unchanged
	assert.Equal(t, rawDirty, teRoundTrip(t, rawDirty))

	// RFC requirement: RFC9012-3.1-3 negative -- zeroing the Reserved subfield produces different bytes, so a speaker that normalized it instead of propagating it unchanged would be caught
	assert.NotEqual(t, rawDirty, teEncode(clean))

	// The V and M bits are 1 and the six R bits are set: an originator must not do this,
	// but an intermediate router must pass it on untouched.
	rBits := TunnelTLV{TunnelType: teTunnelTypeVXLAN, Value: teVXLANEncap(0xFF)}
	rawRBits := teEncode(rBits)

	// RFC requirement: RFC9012-3.2.1-2 positive -- the reserved R bits of a VXLAN Encapsulation sub-TLV are propagated without modification by the re-encode path
	assert.Equal(t, rawRBits, teRoundTrip(t, rawRBits))

	// RFC requirement: RFC9012-3.2.1-2 negative -- clearing the R bits while keeping V and M produces different bytes, so masking them off on propagation is detectable
	assert.NotEqual(t, rawRBits, teEncode(TunnelTLV{TunnelType: teTunnelTypeVXLAN, Value: teVXLANEncap(0xC0)}))
}

// TestRFC9012EmbeddedLabelHandlingNotStripped pins that the Embedded Label Handling
// sub-TLV stays in a TLV it means nothing for, rather than being cleaned up on the way
// out.
func TestRFC9012EmbeddedLabelHandlingNotStripped(t *testing.T) {
	t.Parallel()

	// Tunnel type 15 has no virtual network identifier, so this sub-TLV is ignored here.
	elh := teShort(teSubTLVEmbeddedLabel, 0x02)
	tlv := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(elh, tePreference(120))}
	raw := teEncode(tlv)

	// RFC requirement: RFC9012-3.5-3 positive -- the ignored Embedded Label Handling sub-TLV is still enumerated and the re-encoded attribute is byte-identical, so it is not stripped from the TLV before the route is propagated
	stlvs, err := tlv.SubTLVs()
	require.NoError(t, err)
	require.Len(t, stlvs, 2)
	assert.Equal(t, teSubTLVEmbeddedLabel, stlvs[0].Type)
	assert.Equal(t, []byte{0x02}, stlvs[0].Value)
	assert.Equal(t, raw, teRoundTrip(t, raw))

	// RFC requirement: RFC9012-3.5-3 negative -- the same TLV without the sub-TLV encodes to different, shorter bytes, so stripping it on propagation cannot pass unnoticed
	stripped := teEncode(TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(120)})
	assert.NotEqual(t, raw, stripped)
	assert.Less(t, len(stripped), len(raw))
}

// TestRFC9012ColorExtendedCommunityUnchangedOnPropagation pins that the Color Extended
// Community ze carries is re-advertised with the value it arrived with. ze attaches no
// meaning to the type, which is exactly why the eight octets must survive verbatim.
func TestRFC9012ColorExtendedCommunityUnchangedOnPropagation(t *testing.T) {
	t.Parallel()

	// Type 0x03, Sub-Type 0x0b: Color Extended Community, Flags(2) + Color(4).
	color100 := []byte{0x03, 0x0b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64}
	color200 := []byte{0x03, 0x0b, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC8}

	reencode := func(raw []byte) []byte {
		ecs, err := ParseExtendedCommunities(raw)
		require.NoError(t, err)
		buf := make([]byte, ecs.Len())
		ecs.WriteTo(buf, 0)
		return buf
	}

	// RFC requirement: RFC9012-4.3-2 positive -- a Color Extended Community re-encodes to the exact octets it was parsed from, so propagating the route does not change its value
	assert.Equal(t, color100, reencode(color100))

	// RFC requirement: RFC9012-4.3-2 negative -- a different color re-encodes to its own octets and not to the first one's, so the value is carried through rather than synthesized
	assert.Equal(t, color200, reencode(color200))
	assert.NotEqual(t, reencode(color100), reencode(color200))
}

// TestRFC9012ColorSubTLVUnrecognizedShapeIsCarried is not a compliance claim about the
// Color sub-TLV (ze decodes none); it pins that a Color sub-TLV of the wrong shape does
// not disturb the TLV around it, which is what the rest of this file relies on.
func TestRFC9012ColorSubTLVWrongShapeIsHarmless(t *testing.T) {
	t.Parallel()

	badColor := teShort(teSubTLVColor, 0x03, 0x0b, 0x00) // Length 3, not the mandated 8.
	tlv := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(badColor, tePreference(11))}
	raw := teEncode(tlv)

	pref, ok := tlv.Preference()
	require.True(t, ok)
	assert.Equal(t, uint32(11), pref)
	assert.Equal(t, raw, teRoundTrip(t, raw))
}
