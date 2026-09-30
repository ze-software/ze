package packet

import (
	"errors"
	"testing"
)

// RFC requirement: RFC5880-6.8.6-2 negative -- with the A bit set, a Length of
// 25, one below the A=1 minimum of 26 and one above the A=0 minimum of 24, is
// discarded: ParseControl returns ErrLengthTooSmall for a 25-byte packet whose
// Length field says 25.
//
// VALIDATES: the A=1 floor sits exactly at 26, so the boundary is proven from
// both sides together with TestRFC5880LengthMinimumAccepted (26 accepted).
// PREVENTS: a floor of MandatoryLen+1 for A=1, which accepts a 25-byte
// authenticated packet and still passes the 24-refused and 26-accepted cases.
func TestRFC5880AuthenticatedLengthOneBelowMinimumDiscarded(t *testing.T) {
	authed := rfc5880Good()
	authed.Auth = true
	authed.Length = MandatoryLen + 1
	buf := make([]byte, MandatoryLen+1)
	authed.WriteTo(buf, 0)
	if buf[3] != MandatoryLen+1 {
		t.Fatalf("fixture: Length octet = %d, want %d", buf[3], MandatoryLen+1)
	}
	if _, _, err := ParseControl(buf); !errors.Is(err, ErrLengthTooSmall) {
		t.Fatalf("A=1 length 25: got err %v, want ErrLengthTooSmall", err)
	}
}
