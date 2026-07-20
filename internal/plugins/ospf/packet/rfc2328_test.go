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
}
