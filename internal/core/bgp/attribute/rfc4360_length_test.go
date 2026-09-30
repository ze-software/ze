// VALIDATES: an Extended Communities value is parsed only as whole 8-octet
// quantities (RFC 4360 Section 2), and any other length is refused.
// PREVENTS: a parser that truncates a trailing partial community instead of
// reporting the attribute length error.

package attribute

import (
	"errors"
	"testing"
)

// TestRFC4360ExtendedCommunitiesAreWholeEightOctetQuantities proves the
// Extended Communities value is a set of 8-octet quantities and nothing else.
// Method: ParseExtendedCommunities over 8 and 16 octets, which yield one and
// two communities holding the exact octets; then over 12 and 20 octets, whose
// length is above 8 but not a multiple of it, which must be refused rather
// than parsed with the remainder dropped.
//
// RFC requirement: RFC4360-x-1 positive -- 8 and 16 octets parse into one and two communities of exactly those 8-octet quantities, in order.
// RFC requirement: RFC4360-x-1 negative -- 12 and 20 octets (not whole 8-octet quantities) are refused with ErrInvalidLength, not truncated.
func TestRFC4360ExtendedCommunitiesAreWholeEightOctetQuantities(t *testing.T) {
	data := []byte{0x00, 0x02, 0xFD, 0xE8, 0, 0, 0, 1, 0x03, 0x0b, 0, 0, 0, 0, 0, 42}
	for _, n := range []int{8, 16} {
		comms, err := ParseExtendedCommunities(data[:n])
		if err != nil {
			t.Fatalf("%d octets refused: %v", n, err)
		}
		if len(comms) != n/8 {
			t.Fatalf("%d octets parsed as %d communities, want %d", n, len(comms), n/8)
		}
		for i := range comms {
			if comms[i] != ExtendedCommunity(data[i*8:i*8+8]) {
				t.Errorf("%d octets: community %d is %x, want %x", n, i, comms[i], data[i*8:i*8+8])
			}
		}
	}
	for _, n := range []int{12, 20} {
		value := make([]byte, n)
		copy(value, data)
		if _, err := ParseExtendedCommunities(value); !errors.Is(err, ErrInvalidLength) {
			t.Errorf("%d octets: err %v, want ErrInvalidLength", n, err)
		}
	}
}
