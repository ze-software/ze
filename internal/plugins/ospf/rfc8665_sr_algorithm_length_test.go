// VALIDATES: RFC 8665 Sections 3.1, 9 and 10 at the reception seam: a Router Information
// LSA whose SR-Algorithm TLV carries no algorithm octet is detected as malformed, and none
// of its SR capabilities are applied.
// PREVENTS: a zero-length SR-Algorithm TLV decoding as an empty algorithm list, so the LSA
// still sources the SRGB beside it and the originator is taken as SR capable with no
// algorithm at all.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

// RFC requirement: RFC8665-10-1 negative -- a malformed SR-Algorithm TLV (Length 0, so not
// even Algorithm 0) is detected: the RI LSA carrying it applies no SR capability, neither
// the well-formed SRGB beside it nor a second, well-formed SR-Algorithm TLV after it.
func TestRFC8665ZeroLengthSRAlgorithmIgnoresLSA(t *testing.T) {
	// Goal: the zero-length SR-Algorithm TLV is caught, not read as an empty list. Method:
	// one control body with the TLV holding Algorithm 0 is applied, then the same body with
	// a zero-length SR-Algorithm TLV in its place, and with that TLV ahead of a well-formed
	// one, applies nothing.
	srTestReset(t)
	srgb := packet.RITLV{Type: sr.V4TypeSRGB, Value: sr.EncodeRangeValue(sr.LabelRange{Base: 16000, Size: 100})}
	wellFormed := packet.RITLV{Type: sr.V4TypeSRAlgorithm, Value: []byte{0}}
	empty := packet.RITLV{Type: sr.V4TypeSRAlgorithm, Value: []byte{}}

	control := srDecodeRemoteCapabilities(interfaceFamilyIPv4, packet.EncodeRITLVs([]packet.RITLV{wellFormed, srgb}))
	if control.SRGB.TotalSize() != 100 || len(control.Algorithms) != 1 || control.Algorithms[0] != 0 {
		t.Fatalf("control RI LSA with SR-Algorithm {0} not applied: %+v", control)
	}

	bodies := map[string][]packet.RITLV{
		"zero-length only":         {empty, srgb},
		"zero-length then {0}":     {empty, wellFormed, srgb},
		"srgb then zero-length":    {srgb, empty},
		"zero-length after {0, 1}": {{Type: sr.V4TypeSRAlgorithm, Value: []byte{0, 1}}, empty, srgb},
	}
	for name, tlvs := range bodies {
		caps := srDecodeRemoteCapabilities(interfaceFamilyIPv4, packet.EncodeRITLVs(tlvs))
		if srCapabilitiesApplied(caps) {
			t.Fatalf("%s: RI LSA with a zero-length SR-Algorithm TLV was applied: %+v", name, caps)
		}
	}
}
