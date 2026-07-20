// VALIDATES: the RFC 9830 obligations ze carries on RECEIPT of an SR Policy Tunnel
// Encapsulation attribute -- every Flags, RESERVED, TC/S/TTL and unassigned-bit field of
// every SR Policy sub-TLV is ignored (it changes nothing ze processes) while being
// re-advertised octet for octet, the Preference sub-TLV is only read at its mandated
// 6-octet length, and no semantic judgement is passed on any sub-TLV field.
// PREVENTS: a receiver that reads a reserved octet as data, that normalizes the octets it
// ignores instead of propagating them, that reads a Preference at a guessed offset, and
// that rejects an SR Policy update because a field value looks wrong.

package attribute

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	srpSubTLVSRv6BindingSID uint8 = 20  // RFC 9830 Section 2.4.3.
	srpSubTLVENLP           uint8 = 14  // RFC 9830 Section 2.4.5 (ze implements none).
	srpSubTLVCPName         uint8 = 129 // RFC 9830 Section 2.4.7 (2-octet length).
	srpSubTLVPolicyName     uint8 = 130 // RFC 9830 Section 2.4.8 (2-octet length).

	srpSegWeight uint8 = 9  // RFC 9830 Section 2.4.4.1.
	srpSegTypeA  uint8 = 1  // RFC 9830 Section 2.4.4.2.1.
	srpSegTypeB  uint8 = 13 // RFC 9830 Section 2.4.4.2.2.
)

// srpLong builds a sub-TLV with the 2-octet length header (types 128-255).
func srpLong(styp uint8, value ...byte) []byte {
	hdr := []byte{styp, 0, 0}
	binary.BigEndian.PutUint16(hdr[1:3], uint16(len(value)))
	return append(hdr, value...)
}

// srpSID is a 16-octet SRv6 SID with no zero octets, so a reserved octet reading zero
// beside it is a written zero rather than a blank buffer.
func srpSID() []byte {
	sid := make([]byte, 16)
	for i := range sid {
		sid[i] = byte(0xF0 | i)
	}
	return sid
}

// srpBindingSID builds an SR-MPLS Binding SID sub-TLV (RFC 9830 Section 2.4.2):
// Flags(1) + RESERVED(1) + the 4-octet label stack entry.
func srpBindingSID(flags, reserved byte, entry ...byte) []byte {
	return teShort(SubTLVBindingSID, append([]byte{flags, reserved}, entry...)...)
}

// srpSRv6BindingSID builds an SRv6 Binding SID sub-TLV (RFC 9830 Section 2.4.3).
func srpSRv6BindingSID(flags, reserved byte) []byte {
	return teShort(srpSubTLVSRv6BindingSID, append([]byte{flags, reserved}, srpSID()...)...)
}

// srpSegmentList builds a Segment List sub-TLV (RFC 9830 Section 2.4.4): a RESERVED
// octet followed by the Weight and Segment sub-TLVs.
func srpSegmentList(reserved byte, subs ...[]byte) []byte {
	value := []byte{reserved}
	for _, s := range subs {
		value = append(value, s...)
	}
	return srpLong(SubTLVSegmentList, value...)
}

// srpWeight builds a Weight sub-TLV (RFC 9830 Section 2.4.4.1).
func srpWeight(flags, reserved byte) []byte {
	return teShort(srpSegWeight, flags, reserved, 0x00, 0x00, 0x00, 0x07)
}

// srpTypeA builds a Type A segment sub-TLV (RFC 9830 Section 2.4.4.2.1). The label
// stack entry carries label 24000 with the caller's TC/S/TTL octets.
func srpTypeA(flags, reserved, tcs byte) []byte {
	return teShort(srpSegTypeA, flags, reserved, 0x05, 0xDC, tcs, 0x00)
}

// srpTypeB builds a Type B segment sub-TLV with the SRv6 Endpoint Behavior and SID
// Structure (RFC 9830 Sections 2.4.4.2.2 and 2.4.4.2.4).
// The Flags octet carries the assigned B-Flag (bit 3), which Section 2.4.4.2.3 defines
// as "the SRv6 Endpoint Behavior and SID Structure is present".
func srpTypeB(reserved, ebReserved byte) []byte {
	value := append([]byte{0x10, reserved}, srpSID()...)
	value = append(value, 0xFF, 0xFF, ebReserved, ebReserved, 32, 16, 16, 64)
	return teShort(srpSegTypeB, value...)
}

// srpNamed builds a name sub-TLV (RFC 9830 Sections 2.4.7 and 2.4.8): a RESERVED octet
// followed by the symbolic name.
func srpNamed(styp uint8, reserved byte, name string) []byte {
	return srpLong(styp, append([]byte{reserved}, []byte(name)...)...)
}

// srpIgnoredCase is one field an SR Policy receiver must ignore: `dirty` sets it,
// `clean` leaves it zero, and neither may change what ze reads out of the TLV.
type srpIgnoredCase struct {
	name  string
	dirty []byte
	clean []byte
}

// TestRFC9830ReceivedFieldsAreIgnoredNotRead pins that every Flags, RESERVED and
// reserved-bit field of every SR Policy sub-TLV is ignored on receipt: the value ze
// processes out of the TLV is the same whether the field is zero or full of ones, and
// the received octets are re-advertised exactly as they arrived.
func TestRFC9830ReceivedFieldsAreIgnoredNotRead(t *testing.T) {
	t.Parallel()

	cases := []srpIgnoredCase{
		// RFC requirement: RFC9830-2.4.2-6 positive -- the unassigned bits of the Binding SID Flags field are ignored on receipt
		{"binding-sid flags", srpBindingSID(0xFF, 0x00, 0x05, 0xDC, 0x00, 0x00), srpBindingSID(0x00, 0x00, 0x05, 0xDC, 0x00, 0x00)},
		// RFC requirement: RFC9830-2.4.2-8 positive -- the Binding SID RESERVED octet is ignored on receipt
		{"binding-sid reserved", srpBindingSID(0x00, 0xFF, 0x05, 0xDC, 0x00, 0x00), srpBindingSID(0x00, 0x00, 0x05, 0xDC, 0x00, 0x00)},
		// RFC requirement: RFC9830-2.4.2-10 positive -- the TC, S and TTL bits of the Binding SID label stack entry are ignored on receipt
		{"binding-sid tc/s/ttl", srpBindingSID(0x00, 0x00, 0x05, 0xDC, 0x0F, 0xFF), srpBindingSID(0x00, 0x00, 0x05, 0xDC, 0x00, 0x00)},
		// RFC requirement: RFC9830-2.4.3-5 positive -- the unassigned bits of the SRv6 Binding SID Flags field are ignored on receipt
		{"srv6-binding-sid flags", srpSRv6BindingSID(0xFF, 0x00), srpSRv6BindingSID(0x00, 0x00)},
		// RFC requirement: RFC9830-2.4.3-7 positive -- the SRv6 Binding SID RESERVED octet is ignored on receipt
		{"srv6-binding-sid reserved", srpSRv6BindingSID(0x00, 0xFF), srpSRv6BindingSID(0x00, 0x00)},
		// RFC requirement: RFC9830-2.4.4-5 positive -- the Segment List RESERVED octet is ignored on receipt
		{"segment-list reserved",
			srpSegmentList(0xFF, srpWeight(0, 0), srpTypeA(0, 0, 0)),
			srpSegmentList(0x00, srpWeight(0, 0), srpTypeA(0, 0, 0))},
		// RFC requirement: RFC9830-2.4.4.1-5 positive -- the Weight Flags field is ignored on receipt
		{"weight flags",
			srpSegmentList(0x00, srpWeight(0xFF, 0), srpTypeA(0, 0, 0)),
			srpSegmentList(0x00, srpWeight(0x00, 0), srpTypeA(0, 0, 0))},
		// RFC requirement: RFC9830-2.4.4.1-7 positive -- the Weight RESERVED octet is ignored on receipt
		{"weight reserved",
			srpSegmentList(0x00, srpWeight(0, 0xFF), srpTypeA(0, 0, 0)),
			srpSegmentList(0x00, srpWeight(0, 0x00), srpTypeA(0, 0, 0))},
		// RFC requirement: RFC9830-2.4.4.2.1-3 positive -- the Type A segment RESERVED octet is ignored on receipt
		{"type-a reserved",
			srpSegmentList(0x00, srpTypeA(0, 0xFF, 0)),
			srpSegmentList(0x00, srpTypeA(0, 0x00, 0))},
		// RFC requirement: RFC9830-2.4.4.2.1-5 positive -- the S bit of a Type A label stack entry is ignored on reception
		{"type-a s bit",
			srpSegmentList(0x00, srpTypeA(0, 0, 0x01)),
			srpSegmentList(0x00, srpTypeA(0, 0, 0x00))},
		// RFC requirement: RFC9830-2.4.4.2.3-2 positive -- the unassigned bits of the Segment Flags field are ignored on receipt
		{"segment flags unassigned",
			srpSegmentList(0x00, srpTypeA(0x6F, 0, 0)),
			srpSegmentList(0x00, srpTypeA(0x00, 0, 0))},
		// RFC requirement: RFC9830-2.4.4.2.3-3 positive -- a B-Flag (bit 3) appearing on a Segment Type A is ignored
		{"type-a b flag",
			srpSegmentList(0x00, srpTypeA(0x10, 0, 0)),
			srpSegmentList(0x00, srpTypeA(0x00, 0, 0))},
		// RFC requirement: RFC9830-2.4.4.2.2-3 positive -- the Type B segment RESERVED octet is ignored on receipt
		{"type-b reserved",
			srpSegmentList(0x00, srpTypeB(0xFF, 0x00)),
			srpSegmentList(0x00, srpTypeB(0x00, 0x00))},
		// RFC requirement: RFC9830-2.4.4.2.4-3 positive -- the Reserved field of the SRv6 Endpoint Behavior and SID Structure is ignored on receipt
		{"endpoint-behavior reserved",
			srpSegmentList(0x00, srpTypeB(0x00, 0xFF)),
			srpSegmentList(0x00, srpTypeB(0x00, 0x00))},
		// RFC requirement: RFC9830-2.4.6-6 positive -- the Priority RESERVED octet is ignored on receipt
		{"priority reserved", teShort(SubTLVPriority, 9, 0xFF), teShort(SubTLVPriority, 9, 0x00)},
		// RFC requirement: RFC9830-2.4.7-7 positive -- the SR Policy Candidate Path Name RESERVED octet is ignored on receipt
		{"cp-name reserved", srpNamed(srpSubTLVCPName, 0xFF, "primary"), srpNamed(srpSubTLVCPName, 0x00, "primary")},
		// RFC requirement: RFC9830-2.4.8-7 positive -- the SR Policy Name RESERVED octet is ignored on receipt
		{"policy-name reserved", srpNamed(srpSubTLVPolicyName, 0xFF, "alpha"), srpNamed(srpSubTLVPolicyName, 0x00, "alpha")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dirty := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(tc.dirty, tePreference(4242))}
			clean := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(tc.clean, tePreference(4242))}

			got, ok := dirty.Preference()
			require.True(t, ok)
			want, ok := clean.Preference()
			require.True(t, ok)
			assert.Equal(t, want, got, "the field must not change what is processed")
			assert.Equal(t, uint32(4242), got)

			// The sub-TLV is ignored, not normalized: it goes back out as it came in.
			raw := teEncode(dirty)
			assert.Equal(t, raw, teRoundTrip(t, raw), "the received octets are re-advertised unchanged")
			assert.NotEqual(t, raw, teEncode(clean), "the dirty and clean forms really are different octets")

			// The same TLV with a different Preference yields a different value, so the
			// field above is ignored while the fields that carry meaning are not.
			other := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teCat(tc.dirty, tePreference(9999))}
			changed, ok := other.Preference()
			require.True(t, ok)
			assert.NotEqual(t, got, changed)
		})
	}

	// RFC requirement: RFC9830-2.4.2-6 negative -- an SR Policy TLV whose meaningful content differs IS processed differently, so "the Binding SID flags are ignored" is not a blanket "nothing is read"
	// RFC requirement: RFC9830-2.4.2-8 negative -- the same control applies to the Binding SID RESERVED octet
	// RFC requirement: RFC9830-2.4.2-10 negative -- and to the TC, S and TTL bits of its label stack entry
	// RFC requirement: RFC9830-2.4.3-5 negative -- and to the SRv6 Binding SID Flags field
	// RFC requirement: RFC9830-2.4.3-7 negative -- and to the SRv6 Binding SID RESERVED octet
	// RFC requirement: RFC9830-2.4.4-5 negative -- and to the Segment List RESERVED octet
	// RFC requirement: RFC9830-2.4.4.1-5 negative -- and to the Weight Flags field
	// RFC requirement: RFC9830-2.4.4.1-7 negative -- and to the Weight RESERVED octet
	// RFC requirement: RFC9830-2.4.4.2.1-3 negative -- and to the Type A segment RESERVED octet
	// RFC requirement: RFC9830-2.4.4.2.1-5 negative -- and to the S bit of a Type A label stack entry
	// RFC requirement: RFC9830-2.4.4.2.2-3 negative -- and to the Type B segment RESERVED octet
	// RFC requirement: RFC9830-2.4.4.2.3-2 negative -- and to the unassigned bits of the Segment Flags field
	// RFC requirement: RFC9830-2.4.4.2.3-3 negative -- and to a B-Flag carried on a Segment Type A
	// RFC requirement: RFC9830-2.4.4.2.4-3 negative -- and to the Reserved field of the SRv6 Endpoint Behavior and SID Structure
	// RFC requirement: RFC9830-2.4.6-6 negative -- and to the Priority RESERVED octet
	// RFC requirement: RFC9830-2.4.7-7 negative -- and to the SR Policy Candidate Path Name RESERVED octet
	// RFC requirement: RFC9830-2.4.8-7 negative -- and to the SR Policy Name RESERVED octet
	// Each sub-test above runs this control against its own case; here it is stated once
	// against a TLV whose only difference is the Preference.
	low := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(1)}
	high := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(2)}
	a, ok := low.Preference()
	require.True(t, ok)
	b, ok := high.Preference()
	require.True(t, ok)
	assert.NotEqual(t, a, b)
}

// TestRFC9830PreferenceFlagsAndReservedIgnored pins that the Preference sub-TLV's own
// Flags and RESERVED octets are ignored on receipt: the preference read out of a
// sub-TLV whose first two octets are full of ones is the one in its last four octets.
func TestRFC9830PreferenceFlagsAndReservedIgnored(t *testing.T) {
	t.Parallel()

	pref := func(flags, reserved byte, value uint32) TunnelTLV {
		v := make([]byte, preferenceValueLen)
		v[0] = flags
		v[1] = reserved
		binary.BigEndian.PutUint32(v[2:preferenceValueLen], value)
		return TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teShort(SubTLVPreference, v...)}
	}

	clean := pref(0x00, 0x00, 4242)
	dirtyFlags := pref(0xFF, 0x00, 4242)
	dirtyReserved := pref(0x00, 0xFF, 4242)

	want, ok := clean.Preference()
	require.True(t, ok)
	assert.Equal(t, uint32(4242), want)

	// RFC requirement: RFC9830-2.4.1-5 positive -- a Preference sub-TLV whose Flags octet is 0xFF yields the same preference as one whose Flags octet is zero
	got, ok := dirtyFlags.Preference()
	require.True(t, ok)
	assert.Equal(t, want, got)

	// RFC requirement: RFC9830-2.4.1-7 positive -- the same holds for the RESERVED octet
	got, ok = dirtyReserved.Preference()
	require.True(t, ok)
	assert.Equal(t, want, got)

	// RFC requirement: RFC9830-2.4.1-5 negative -- changing the four Preference octets DOES change the value read, so the Flags octet is ignored rather than the whole sub-TLV being disregarded
	// RFC requirement: RFC9830-2.4.1-7 negative -- the same control shows the RESERVED octet is ignored while the preference beside it is read
	dirtyBoth := pref(0xFF, 0xFF, 9999)
	other, ok := dirtyBoth.Preference()
	require.True(t, ok)
	assert.Equal(t, uint32(9999), other)
	assert.NotEqual(t, want, other)

	// RFC requirement: RFC9830-2.4.1-3 negative -- a Preference sub-TLV whose value is not the mandated 6 octets is not read at a guessed offset: a 4-octet and an 8-octet value both yield nothing
	short := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teShort(SubTLVPreference, 0x00, 0x00, 0x10, 0x92)}
	_, ok = short.Preference()
	assert.False(t, ok)
	long := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: teShort(SubTLVPreference, 0, 0, 0, 0, 0, 0, 0x10, 0x92)}
	_, ok = long.Preference()
	assert.False(t, ok)
}

// TestRFC9830EgressEndpointAndColorSubTLVsIgnored pins that the two RFC 9012 sub-TLVs
// the SR Policy SAFI does not use change nothing ze processes, and are still carried.
func TestRFC9830EgressEndpointAndColorSubTLVsIgnored(t *testing.T) {
	t.Parallel()

	// Tunnel Egress Endpoint (type 6) and Color (type 4) sub-TLVs, both well-formed.
	color := teShort(teSubTLVColor, 0x03, 0x0b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64)
	withThem := TunnelTLV{TunnelType: teTunnelTypeSRPolicy,
		Value: teCat(teEgressEndpoint(0x00), color, tePreference(4242))}
	without := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: tePreference(4242)}

	// RFC requirement: RFC9830-2.3-1 positive -- the Tunnel Egress Endpoint and Color sub-TLVs are ignored: the value ze processes out of the TLV is the same as if they were not there
	got, ok := withThem.Preference()
	require.True(t, ok)
	want, ok := without.Preference()
	require.True(t, ok)
	assert.Equal(t, want, got)

	// They are ignored, not removed: an SR Policy speaker may drop them, ze does not.
	raw := teEncode(withThem)
	assert.Equal(t, raw, teRoundTrip(t, raw))

	// RFC requirement: RFC9830-2.3-1 negative -- a sub-TLV that DOES have applicability to the SR Policy SAFI is not ignored: changing the Preference beside them changes what is processed
	other := TunnelTLV{TunnelType: teTunnelTypeSRPolicy,
		Value: teCat(teEgressEndpoint(0x00), color, tePreference(9999))}
	changed, ok := other.Preference()
	require.True(t, ok)
	assert.NotEqual(t, got, changed)
}

// TestRFC9830NoSemanticVerification pins that ze passes no judgement on the CONTENT of
// an SR Policy sub-TLV: values the SRPM would reject are carried and processed exactly
// like sensible ones.
func TestRFC9830NoSemanticVerification(t *testing.T) {
	t.Parallel()

	// Every field below is semantically wrong and syntactically fine: a weight of zero
	// (invalid per Section 2.4.4.1), a reserved MPLS label value (3), a SID structure
	// whose lengths total 255 (over the 128 limit of Section 2.4.4.2.4), an ENLP value
	// outside 1..4, and a segment list holding no segment at all.
	nonsense := teCat(
		tePreference(4242),
		srpBindingSID(0x00, 0x00, 0x00, 0x00, 0x30, 0x00), // label 3, a reserved value
		teShort(srpSubTLVENLP, 0x00, 0x00, 99),
		srpSegmentList(0x00,
			teShort(srpSegWeight, 0, 0, 0, 0, 0, 0), // weight 0
			teShort(srpSegTypeB, append(append([]byte{0x10, 0x00}, srpSID()...),
				0xFF, 0xFF, 0x00, 0x00, 64, 64, 64, 63)...), // lengths total 255
		),
		srpSegmentList(0x00), // no weight, no segment
	)
	tlv := TunnelTLV{TunnelType: teTunnelTypeSRPolicy, Value: nonsense}
	raw := teEncode(tlv)

	// RFC requirement: RFC9830-5-9 positive -- none of those field values makes the attribute unusable: it parses, every sub-TLV is enumerated, the Preference is still read, and the bytes are re-advertised unchanged
	te, err := ParseTunnelEncap(raw)
	require.NoError(t, err, "no semantic verification is performed on the sub-TLV fields")
	require.Len(t, te.TLVs, 1)
	stlvs, err := te.TLVs[0].SubTLVs()
	require.NoError(t, err)
	assert.Len(t, stlvs, 5, "preference, binding sid, enlp, two segment lists")
	pref, ok := te.TLVs[0].Preference()
	require.True(t, ok)
	assert.Equal(t, uint32(4242), pref)
	assert.Equal(t, raw, teRoundTrip(t, raw))

	// RFC requirement: RFC9830-5-9 negative -- an attribute whose FRAMING cannot be walked is still refused, so "no semantic verification" is not "no verification": the check that is skipped is on the field values, not on the structure
	_, err = ParseTunnelEncap([]byte{0x00, 0x0F, 0x00, 0x10, 0x01, 0x02})
	require.Error(t, err)
	partial, err := (&TunnelTLV{TunnelType: teTunnelTypeSRPolicy,
		Value: teShort(SubTLVPreference, 0x00, 0x00, 0x00)[:4]}).SubTLVs()
	require.Error(t, err, "a sub-TLV whose declared length overruns the TLV is refused")
	assert.Empty(t, partial)
}
