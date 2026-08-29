// VALIDATES: RFC 2328 Section 12.1.7 -- the LS checksum is never zero: a zero value in the LS
// Checksum field is a checksum failure, not "no checksum computed".
// PREVENTS: an LSA whose checksum field was never backfilled being accepted as valid.
package packet

import (
	"testing"
)

// RFC requirement: RFC2328-12.1.7-1 negative -- an otherwise well-formed LSA whose LS Checksum
// field is zero is rejected: the calculation is not optional and zero is a failure, so
// VerifyLSAChecksum returns false (FletcherVerify zero guard, types/checksum.go:36-38, reached
// through VerifyLSAChecksum, checksum.go:47-56).
func TestRFC2328ZeroLSChecksumRejected(t *testing.T) {
	wire := encodeLSA(t, sampleRouterLSA(t))
	if !VerifyLSAChecksum(wire) {
		t.Fatalf("precondition: the encoded LSA must verify")
	}
	zeroed := append([]byte(nil), wire...)
	writeUint16(zeroed, lsaChecksumOff, 0)
	if VerifyLSAChecksum(zeroed) {
		t.Fatalf("an LSA carrying LS Checksum 0 was accepted; RFC 2328 sec 12.1.7 makes zero a failure")
	}

	// The case above rejects for two reasons at once: the checksum field is zero, and
	// zeroing it also broke the Fletcher sums. Only the second is needed to fail it, so it
	// says nothing about the zero rule. This one isolates the rule: the LS Checksum field is
	// zero AND the Fletcher sums over the covered region are both zero, so the sum check
	// accepts and only the zero rule can reject.
	summing := append([]byte(nil), wire...)
	writeUint16(summing, lsaChecksumOff, 0)
	length := int(readUint16(summing, lsaLengthOff))
	solveFletcherToZero(t, summing[2:length])

	if got := readUint16(summing, lsaChecksumOff); got != 0 {
		t.Fatalf("fixture: LS Checksum = %#04x, want 0", got)
	}
	if c0, c1 := fletcherSums(summing[2:length]); c0 != 0 || c1 != 0 {
		t.Fatalf("fixture: Fletcher sums = (%d, %d), want (0, 0) so only the zero rule can reject", c0, c1)
	}

	// RFC requirement: RFC2328-12.1.7-1 negative -- an LSA whose Fletcher sums are valid but
	// whose LS Checksum field is zero is still rejected: RFC 2328 sec 12.1.7 makes zero a
	// failure rather than "no checksum computed" (FletcherVerify zero guard,
	// types/checksum.go, reached through VerifyLSAChecksum, checksum.go).
	if VerifyLSAChecksum(summing) {
		t.Fatalf("an LSA with LS Checksum 0 and valid Fletcher sums was accepted; the zero rule is not enforced")
	}
}

// solveFletcherToZero rewrites the last two octets of covered so both Fletcher sums
// over it become zero, leaving every other octet as it was.
//
// A solution always exists: an octet at position i shifts c0 by its delta and c1 by
// that delta times (len-i), so the last two positions give the system
// [[1,1],[2,1]] over Z/255, whose determinant is -1. The search below is the boring
// way to solve it, and 65536 candidates over a 40-octet LSA is instant.
func solveFletcherToZero(t *testing.T, covered []byte) {
	t.Helper()
	if len(covered) < 2 {
		t.Fatalf("covered region is %d octets, need at least 2 to solve", len(covered))
	}
	tail := covered[len(covered)-2:]
	for first := range 256 {
		for second := range 256 {
			tail[0] = byte(first)
			tail[1] = byte(second)
			if c0, c1 := fletcherSums(covered); c0 == 0 && c1 == 0 {
				return
			}
		}
	}
	t.Fatal("no two trailing octets make both Fletcher sums zero, which the modulus arithmetic says is impossible")
}
