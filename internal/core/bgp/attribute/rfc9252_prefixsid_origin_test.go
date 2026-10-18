// Design: docs/architecture/api/update-syntax.md -- SRv6 Prefix-SID origination
// RFC 9252 Sections 5, 6 and 7 -- see rfc/short/rfc9252.md.
package attribute

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestCheckSRv6TranspositionForFamily checks the route-family half of the SID
// Structure rule, which the encoder cannot judge because it sees no route.
//
// VALIDATES: a structure declaring a transposition is accepted with an IPv4 VPN,
// IPv6 VPN or EVPN route, whose NLRI has a label field, and refused with an IPv6
// unicast, IPv4 unicast or IPv6 labeled unicast route, whether the length or the
// offset is the non-zero value. A zero transposition is accepted on every family.
// PREVENTS: ze advertising transposition offset and length on a global IPv6
// service route, where the receiver has no label field to rebuild the SID from.
//
// RFC requirement: RFC9252-3.2.1-2 positive -- a SID Structure whose transposition offset and length are 0 is accepted with a route of a family where the Transposition Scheme is not applicable (IPv4 and IPv6 unicast, IPv6 labeled unicast), and a non-zero transposition is accepted with IPv4 VPN, IPv6 VPN and EVPN routes
// RFC requirement: RFC9252-3.2.1-2 negative -- a SID Structure with a non-zero transposition length, or a non-zero transposition offset, is refused with a route of a family where the Transposition Scheme is not applicable (IPv6 unicast, IPv4 unicast, IPv6 labeled unicast), so it never reaches the wire.
func TestCheckSRv6TranspositionForFamily(t *testing.T) {
	transposed, err := EncodePrefixSIDSRv6("l3-service 2001:db8:: 0x13 [64,24,16,0,16,64]")
	if err != nil {
		t.Fatalf("encode transposed structure: %v", err)
	}
	offsetOnly := append([]byte(nil), transposed...)
	offsetOnly[len(offsetOnly)-2] = 0 // transposition length 0, offset 64 kept
	untransposed, err := EncodePrefixSIDSRv6("l3-service 2001:db8::1 0x13 [64,24,16,0,0,0]")
	if err != nil {
		t.Fatalf("encode untransposed structure: %v", err)
	}

	labeled := []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
		{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN},
	}
	unlabeled := []family.Family{
		family.IPv6Unicast,
		family.IPv4Unicast,
		{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel},
	}
	for _, fam := range labeled {
		if err := CheckSRv6TranspositionForFamily(transposed, fam); err != nil {
			t.Fatalf("%s: transposition refused on a family with a label field: %v", fam, err)
		}
	}
	for _, fam := range unlabeled {
		if err := CheckSRv6TranspositionForFamily(untransposed, fam); err != nil {
			t.Fatalf("%s: zero transposition refused: %v", fam, err)
		}
		for name, value := range map[string][]byte{"length": transposed, "offset": offsetOnly} {
			err := CheckSRv6TranspositionForFamily(value, fam)
			if err == nil {
				t.Fatalf("%s: non-zero transposition %s accepted on a family with no label field", fam, name)
			}
			if !strings.Contains(err.Error(), fam.String()) || !strings.Contains(err.Error(), "[64,24,16,0,0,0]") {
				t.Fatalf("%s: error %q does not name the family and the corrected structure", fam, err)
			}
		}
	}
}

// TestCheckSRv6TranspositionLabelWidth checks the label-width half of the SID
// Structure rule on the families where the Transposition Scheme applies.
//
// VALIDATES: a Transposition Length equal to the label field width (20 bits on
// IPv4 and IPv6 VPN, 24 bits on EVPN) is accepted, and one bit wider is refused
// with an error naming the family, the width and the corrected structure. The
// Function Length is 32 so the separate FL bound never decides the case.
// PREVENTS: ze originating a structure whose transposed bits do not fit the label
// field, which its own receive side then treats as carrying no valid SID.
//
// RFC requirement: RFC9252-4.1-1 positive -- on IPv4 VPN and IPv6 VPN routes a Transposition Length of 20, the VPN label width, is accepted at origination.
// RFC requirement: RFC9252-4.1-1 negative -- on IPv4 VPN and IPv6 VPN routes a Transposition Length of 21 is refused at origination, so it never reaches the wire.
// RFC requirement: RFC9252-6.2-1 positive -- on EVPN routes a Transposition Length of 24, the EVPN label width, is accepted at origination.
// RFC requirement: RFC9252-6.2-1 negative -- on EVPN routes a Transposition Length of 25 is refused at origination, so it never reaches the wire.
func TestCheckSRv6TranspositionLabelWidth(t *testing.T) {
	for _, tc := range []struct {
		fam   family.Family
		width int
	}{
		{family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, 20},
		{family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}, 20},
		{family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, 24},
	} {
		structure := func(length int) []byte {
			text := "l3-service 2001:db8:: 0x13 [64,24,32,0," + strconv.Itoa(length) + ",88]"
			encoded, err := EncodePrefixSIDSRv6(text)
			if err != nil {
				t.Fatalf("encode %q: %v", text, err)
			}
			return encoded
		}
		if err := CheckSRv6TranspositionForFamily(structure(tc.width), tc.fam); err != nil {
			t.Fatalf("%s: transposition length %d refused: %v", tc.fam, tc.width, err)
		}
		err := CheckSRv6TranspositionForFamily(structure(tc.width+1), tc.fam)
		if err == nil {
			t.Fatalf("%s: transposition length %d accepted beyond the %d-bit label field", tc.fam, tc.width+1, tc.width)
		}
		fixed := "[64,24,32,0," + strconv.Itoa(tc.width) + ",88]"
		if !strings.Contains(err.Error(), tc.fam.String()) || !strings.Contains(err.Error(), fixed) {
			t.Fatalf("%s: error %q does not name the family and the corrected structure %s", tc.fam, err, fixed)
		}
	}
}
