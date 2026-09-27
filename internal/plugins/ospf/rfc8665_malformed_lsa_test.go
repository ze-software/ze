// VALIDATES: RFC 8665 Section 9 at the reception seam: a Router Information LSA that
// carries an SR TLV or sub-TLV of invalid length is malformed, and none of its SR
// capabilities are applied.
// PREVENTS: srDecodeRemoteCapabilities skipping only the bad TLV and applying the rest of
// the LSA, so a half-parsed LSA sources an SRGB or an algorithm set.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

// srCapabilitiesApplied reports whether any SR capability was taken from the LSA.
func srCapabilitiesApplied(c srRemoteCapabilities) bool {
	return len(c.Algorithms) != 0 || !c.SRGB.Empty() || !c.SRLB.Empty() || c.HasSRMS
}

// TestRFC8665LengthInvalidTLVIgnoresLSA decodes one RI LSA body with well-formed SR TLVs,
// then the same body with one TLV whose length is invalid for its layout.
func TestRFC8665LengthInvalidTLVIgnoresLSA(t *testing.T) {
	srTestReset(t)
	algorithm := packet.RITLV{Type: sr.V4TypeSRAlgorithm, Value: sr.EncodeAlgorithmValue([]uint8{0})}
	srgb := packet.RITLV{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(sr.LabelRange{Base: 16000, Size: 100})}
	srlb := sr.EncodeRangeValue(sr.LabelRange{Base: 15000, Size: 10})
	srms := sr.EncodeSRMSValue(7)

	// RFC requirement: RFC8665-9-1 positive -- an RI LSA whose SR TLVs all carry a valid
	// length is applied: its algorithms, SRGB, SRLB and SRMS preference are all decoded.
	good := packet.EncodeRITLVs([]packet.RITLV{algorithm, srgb,
		{Type: sr.V4TypeSRLB, Value: srlb}, {Type: sr.V4TypeSRMS, Value: srms}})
	caps := srDecodeRemoteCapabilities(interfaceFamilyIPv4, good)
	if caps.SRGB.TotalSize() != 100 || caps.SRLB.TotalSize() != 10 || !caps.HasSRMS || !sr.HasAlgorithm(caps.Algorithms, 0) {
		t.Fatalf("well-formed RI LSA not applied: %+v", caps)
	}

	// RFC requirement: RFC8665-9-1 negative -- one SR TLV whose length is invalid (an SRLB
	// cut inside its SID/Label sub-TLV, an SRMS Preference TLV of 2 octets rather than 4)
	// makes the LSA malformed, and it MUST be ignored: no capability from it is applied,
	// including the well-formed TLVs beside the bad one.
	invalid := map[string]packet.RITLV{
		"srlb-truncated": {Type: sr.V4TypeSRLB, Value: srlb[:len(srlb)-2]},
		"srms-short":     {Type: sr.V4TypeSRMS, Value: srms[:2]},
	}
	for name, bad := range invalid {
		body := packet.EncodeRITLVs([]packet.RITLV{algorithm, srgb, bad})
		if caps := srDecodeRemoteCapabilities(interfaceFamilyIPv4, body); srCapabilitiesApplied(caps) {
			t.Fatalf("%s: RI LSA with a length-invalid SR TLV was applied: %+v", name, caps)
		}
	}
}
