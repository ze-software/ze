// VALIDATES: every field RFC 9830 says to set to zero on transmission and
// ignore on receipt is zero in the sub-TLVs Ze encodes.
// PREVENTS: an encoder that leaks a configured value or stale octets into a
// Flags, RESERVED or reserved-bit field.

package srpolicy

import (
	"bytes"
	"testing"
)

// TestRFC9830FieldsToIgnoreAreZeroOnTransmission proves the transmission half
// of every RFC 9830 sentence "MUST be set to zero on transmission and MUST be
// ignored on receipt": every Flags, RESERVED and reserved-bit field of every
// SR Policy sub-TLV Ze writes is zero, while the value fields beside it carry
// the all-ones configuration. The receipt half is
// internal/component/bgp/reactor/rfc9012_tunnel_encap_carry_test.go.
// Method: encode srpDirty (every configurable value at its maximum) through
// the config parser and the sub-TLV encoder, then read each field at its
// offset. The negatives read the neighboring value field in the same
// sub-TLV, so a zero is a written zero, not an untouched or all-zero buffer.
//
// RFC requirement: RFC9830-2.4.1-5 positive -- the Preference Flags octet is zero on transmission.
// RFC requirement: RFC9830-2.4.1-5 negative -- the preference beside it is 0xDEADBEEF, so the zero is written.
// RFC requirement: RFC9830-2.4.1-7 positive -- the Preference RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.1-7 negative -- the preference beside it is 0xDEADBEEF, so the zero is written.
// RFC requirement: RFC9830-2.4.2-8 positive -- the Binding SID RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.2-8 negative -- the label beside it is 0xFFFFF.
// RFC requirement: RFC9830-2.4.2-10 positive -- the Binding SID TC, S and TTL bits are zero on transmission.
// RFC requirement: RFC9830-2.4.2-10 negative -- every bit of the 20-bit label before them is set.
// RFC requirement: RFC9830-2.4.3-5 positive -- the SRv6 Binding SID Flags octet has no unassigned bit set on transmission.
// RFC requirement: RFC9830-2.4.3-5 negative -- the SID beside it is non-zero.
// RFC requirement: RFC9830-2.4.3-7 positive -- the SRv6 Binding SID RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.3-7 negative -- the SID beside it is non-zero.
// RFC requirement: RFC9830-2.4.4-5 positive -- the Segment List RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.4-5 negative -- the sub-TLVs after it are present and non-zero.
// RFC requirement: RFC9830-2.4.4.1-5 positive -- the Weight Flags octet is zero on transmission.
// RFC requirement: RFC9830-2.4.4.1-5 negative -- the weight beside it is 0xCAFEBABE.
// RFC requirement: RFC9830-2.4.4.1-7 positive -- the Weight RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.4.1-7 negative -- the weight beside it is 0xCAFEBABE.
// RFC requirement: RFC9830-2.4.4.2.1-3 positive -- the Type A RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.4.2.1-3 negative -- the label beside it is 0xFFFFF.
// RFC requirement: RFC9830-2.4.4.2.1-5 positive -- the Type A label stack entry S bit is zero on transmission.
// RFC requirement: RFC9830-2.4.4.2.1-5 negative -- every bit of the label before it is set.
// RFC requirement: RFC9830-2.4.4.2.2-3 positive -- the Type B RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.4.2.2-3 negative -- the SRv6 SID beside it is non-zero.
// RFC requirement: RFC9830-2.4.4.2.3-2 positive -- no unassigned Segment Flags bit is set on transmission, on Type A or Type B.
// RFC requirement: RFC9830-2.4.4.2.3-2 negative -- the Type B Flags octet does carry the assigned B-Flag, so the check is per bit.
// RFC requirement: RFC9830-2.4.4.2.4-3 positive -- the two Reserved octets of the SRv6 Endpoint Behavior and SID Structure are zero on transmission.
// RFC requirement: RFC9830-2.4.4.2.4-3 negative -- the endpoint behavior before them is 0xFFFF and the structure after them 32/16/16/64.
// RFC requirement: RFC9830-2.4.6-6 positive -- the Priority RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.6-6 negative -- the priority beside it is 255.
// RFC requirement: RFC9830-2.4.7-7 positive -- the Candidate Path Name RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.7-7 negative -- the name after it is the configured "primary".
// RFC requirement: RFC9830-2.4.8-7 positive -- the Policy Name RESERVED octet is zero on transmission.
// RFC requirement: RFC9830-2.4.8-7 negative -- the name after it is the configured "alpha".
func TestRFC9830FieldsToIgnoreAreZeroOnTransmission(t *testing.T) {
	t.Parallel()

	subs := srpSubs(t, srpDirty)
	zero := func(field string, got byte) {
		t.Helper()
		if got != 0 {
			t.Errorf("%s = %#02x on transmission, want 0", field, got)
		}
	}
	same := func(field string, got, want []byte) {
		t.Helper()
		if !bytes.Equal(got, want) {
			t.Errorf("%s = %x, want %x", field, got, want)
		}
	}
	label := func(v []byte) uint32 { return uint32(v[2])<<12 | uint32(v[3])<<4 | uint32(v[4])>>4 }

	pref := srpOne(t, subs, subTLVPreference)
	zero("Preference Flags", pref[0])
	zero("Preference RESERVED", pref[1])
	same("Preference", pref[2:6], []byte{0xDE, 0xAD, 0xBE, 0xEF})

	bsid := srpOne(t, subs, subTLVBindingSID)
	zero("Binding SID RESERVED", bsid[1])
	zero("Binding SID TC and S", bsid[4]&0x0F)
	zero("Binding SID TTL", bsid[5])
	if got := label(bsid); got != 0xFFFFF {
		t.Errorf("Binding SID label %#x, want 0xfffff", got)
	}

	srv6 := srpOne(t, subs, subTLVSRv6BindingSID)
	zero("SRv6 Binding SID Flags", srv6[0])
	zero("SRv6 Binding SID RESERVED", srv6[1])
	same("SRv6 Binding SID first SID octet", srv6[2:3], []byte{0xFC})

	prio := srpOne(t, subs, subTLVPriority)
	zero("Priority RESERVED", prio[1])
	same("Priority", prio[0:1], []byte{255})

	list := srpAll(subs, subTLVSegmentList)[0]
	zero("Segment List RESERVED", list[0])
	inner := srpSegSubs(t, list)
	if len(inner) != 3 {
		t.Fatalf("segment list holds %d sub-TLVs, want weight, type A, type B", len(inner))
	}

	weight := srpAll(inner, segSubTLVWeight)[0]
	zero("Weight Flags", weight[0])
	zero("Weight RESERVED", weight[1])
	same("Weight", weight[2:6], []byte{0xCA, 0xFE, 0xBA, 0xBE})

	typeA := srpAll(inner, segSubTLVTypeA)[0]
	zero("Type A Flags", typeA[0])
	zero("Type A RESERVED", typeA[1])
	zero("Type A S bit", typeA[4]&0x01)
	if got := label(typeA); got != 0xFFFFF {
		t.Errorf("Type A label %#x, want 0xfffff", got)
	}

	typeB := srpAll(inner, segSubTLVTypeBSID)[0]
	zero("Type B unassigned Flags bits", typeB[0]&^0x10)
	same("Type B Flags (B-Flag only)", typeB[0:1], []byte{0x10})
	zero("Type B RESERVED", typeB[1])
	same("Type B SID first octet", typeB[2:3], []byte{0xFC})
	same("SRv6 Endpoint Behavior", typeB[18:20], []byte{0xFF, 0xFF})
	same("SRv6 Endpoint Behavior Reserved", typeB[20:22], []byte{0, 0})
	same("SRv6 SID Structure", typeB[22:26], []byte{32, 16, 16, 64})

	for _, name := range []struct {
		field string
		styp  uint8
		want  string
	}{
		{"Candidate Path Name", subTLVCandidatePathNam, "primary"},
		{"Policy Name", subTLVPolicyName, "alpha"},
	} {
		value := srpOne(t, subs, name.styp)
		zero(name.field+" RESERVED", value[0])
		same(name.field, value[1:], []byte(name.want))
	}
}
