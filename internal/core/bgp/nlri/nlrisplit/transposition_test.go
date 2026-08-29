// RFC: rfc/short/rfc9252.md -- Sections 5.1 and 5.2, the VPN label field; Section 6, EVPN
// Related: transposition.go -- TranspositionLabel, the function under test

package nlrisplit

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

var (
	famVPNv4 = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	famVPNv6 = family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}
	famEVPN  = family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}
)

// vpnNLRI builds a VPNv4 NLRI for 10.0.0.0/8 carrying the given 20-bit label.
func vpnNLRI(label uint32, addPath bool) []byte {
	var out []byte
	if addPath {
		out = append(out, 0, 0, 0, 7) // path identifier
	}
	// Length in bits, the RFC 8277 label, the Route Distinguisher, the prefix.
	out = append(out,
		24+64+8,
		byte(label>>12), byte(label>>4), byte(label<<4)|0x01,
		0, 0, 0, 0, 0, 0, 0, 0,
		0x0a)
	return out
}

// TestTranspositionLabelVPN checks the label value read out of a VPN NLRI.
//
// VALIDATES: RFC 9252 Sections 5.1 and 5.2 -- the VPN label field is encoded
// as RFC 8277 specifies, "with the 20-bit Label Value set to the whole or a
// portion of the Function part of the SRv6 SID".
// PREVENTS: reading the three-octet field as a raw 24-bit value, which would
// shift every transposed bit four places and reconstruct a different SID.
func TestTranspositionLabelVPN(t *testing.T) {
	const label = 0xE5A70
	for _, fam := range []family.Family{famVPNv4, famVPNv6} {
		for _, addPath := range []bool{false, true} {
			got, ok := TranspositionLabel(fam, vpnNLRI(label, addPath), addPath)
			if !ok {
				t.Fatalf("%v addPath=%v: no label read", fam, addPath)
			}
			if got != label {
				t.Errorf("%v addPath=%v: label = %#x, want %#x", fam, addPath, got, label)
			}
		}
	}
}

// TestTranspositionLabelBoundaryValues pins both ends of the 20-bit field.
//
// VALIDATES: RFC 8277 Section 2.1 -- the Label Value is the high-order 20 bits
// of the three-octet field, so the widest value it can hold is 0xFFFFF.
// PREVENTS: an off-by-one in the shift that would drop the top or bottom bit.
func TestTranspositionLabelBoundaryValues(t *testing.T) {
	for _, label := range []uint32{0, 1, 0xFFFFF} {
		got, ok := TranspositionLabel(famVPNv4, vpnNLRI(label, false), false)
		if !ok || got != label {
			t.Errorf("label %#x: got %#x ok=%v", label, got, ok)
		}
	}
}

// TestTranspositionLabelTruncated checks a NLRI too short to hold a label.
//
// VALIDATES: a malformed NLRI yields no label rather than bytes read past the
// end of the slice.
// PREVENTS: a panic, and a fabricated label reconstructing a wrong SID.
func TestTranspositionLabelTruncated(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3} {
		if _, ok := TranspositionLabel(famVPNv4, vpnNLRI(0xABCDE, false)[:n], false); ok {
			t.Errorf("len %d: label read from a truncated NLRI", n)
		}
	}
}

// TestTranspositionLabelUnreadableFamilies checks the families whose label
// field ze does not read.
//
// VALIDATES: RFC 9252 Section 6 -- EVPN transposes into label fields that sit
// at a different place in each route type, and for Route Type 3 outside the
// NLRI entirely, so no single NLRI offset answers for EVPN.
// PREVENTS: reading arbitrary NLRI bytes as an EVPN label and reconstructing a
// SID from them, which is worse than reporting none.
func TestTranspositionLabelUnreadableFamilies(t *testing.T) {
	for _, fam := range []family.Family{
		famEVPN,
		family.IPv4Unicast,
		family.IPv6Unicast,
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
	} {
		if _, ok := TranspositionLabel(fam, vpnNLRI(0xABCDE, false), false); ok {
			t.Errorf("%v: reported a transposition label", fam)
		}
	}
}
