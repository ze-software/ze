// Design: docs/architecture/ospf/ospf-1-types.md -- OSPF checksum algorithm vectors and boundaries

package types

import "testing"

// VALIDATES: the fold repeats until no carry is left. The words 0xFFFF, 0xFFFF
// and 0x0001 sum to 0x1FFFF. The first fold gives 0xFFFF + 0x0001 = 0x10000,
// which itself carries, and the second fold gives 0x0001, so the checksum is
// ^0x0001 = 0xFFFE. A fold that stops after one addition keeps 0x0000 and
// answers 0xFFFF. The same vector through the two-segment form (the OSPF packet
// path) must agree, and the verifier must accept the data once 0xFFFE is placed.
//
// RFC requirement: RFC1071-1-6 positive -- a sum whose first fold carries again (0x1FFFF -> 0x10000 -> 0x0001) is folded until no high bits remain: internetChecksum and InternetChecksumPair both answer exactly 0xFFFE, where a single fold answers 0xFFFF, and internetChecksumValid accepts the data with 0xFFFE appended.
func TestRFC1071FoldRepeatsUntilNoCarry(t *testing.T) {
	data := []byte{0xff, 0xff, 0xff, 0xff, 0x00, 0x01}
	want := uint16(0xfffe)
	if got := internetChecksum(data); got != want {
		t.Fatalf("internetChecksum(% x) = %#04x, want %#04x (the second end-around carry was not added)", data, got, want)
	}
	if got := InternetChecksumPair(data[:2], data[2:]); got != want {
		t.Fatalf("InternetChecksumPair(% x | % x) = %#04x, want %#04x", data[:2], data[2:], got, want)
	}
	withChecksum := append(append([]byte(nil), data...), byte(want>>8), byte(want))
	if !internetChecksumValid(withChecksum) {
		t.Fatalf("internetChecksumValid rejected % x", withChecksum)
	}
}
