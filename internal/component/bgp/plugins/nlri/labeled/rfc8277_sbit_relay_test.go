// Design: docs/architecture/wire/qualifiers.md -- Which Writer to Call, WriteLabelStack
// RFC: rfc/short/rfc8277.md -- Sections 2.2 and 2.3, the S bit on transmission
// Overview: types.go -- LabeledUnicast.WriteTo
// Related: rfc8277_test.go -- the S bit of stacks built from label values

package labeled

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC8277RelayedStackSBitRewritten hands LabeledUnicast.WriteTo label stack
// ENTRIES whose S bits are wrong, the input a relayed stack reaches it with, and
// checks the octets it transmits.
//
// VALIDATES: RFC 8277 Section 2.2 ("S: This 1-bit field MUST be set to one on
// transmission") and Section 2.3 ("In all labels except the last ... the S bit
// MUST be 0. In the last label, the S bit MUST be 1."). WriteTo owns the S bit:
// a lone entry given with S = 0 leaves with S = 1; in a three-entry stack given
// with S = 1, S = 1, S = 0, the first two leave with S = 0 and the last with
// S = 1. The label and traffic-class bits of each entry are written unchanged.
// Each case compares the whole NLRI against literal octets.
// PREVENTS: an encoder that copies the S bit of the entry it was handed, which
// would transmit a stack the receiver ends early or reads past.
//
// RFC requirement: RFC8277-2.2-1 negative -- a lone entry 0x000640 (S = 0) for 10.0.0.0/8 is transmitted as 20 000641 0a, S set to one.
// RFC requirement: RFC8277-2.3-1 negative -- entries 0x000641, 0x000c81, 0x0012c0 (S = 1 on the first two) are transmitted with S = 0 on the first two: 50 000640 000c80 0012c1 0a.
// RFC requirement: RFC8277-2.3-2 negative -- the same stack, whose last entry was given with S = 0, is transmitted with S = 1 on the last entry (0012c1).
func TestRFC8277RelayedStackSBitRewritten(t *testing.T) {
	t.Parallel()

	prefix := netip.MustParsePrefix("10.0.0.0/8")
	cases := []struct {
		name    string
		entries []uint32
		want    string
	}{
		{"lone entry without S", []uint32{0x000640}, "20" + "000641" + "0a"},
		{"stack with S misplaced", []uint32{0x000641, 0x000c81, 0x0012c0}, "50" + "000640" + "000c80" + "0012c1" + "0a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			route := &LabeledUnicast{
				family: Family{AFI: family.AFIIPv4, SAFI: SAFIMPLSLabel},
				prefix: prefix,
				labels: tc.entries,
			}
			buf := make([]byte, route.Len())
			written := route.WriteTo(buf, 0)
			if got := hex.EncodeToString(buf[:written]); got != tc.want {
				t.Errorf("NLRI = %s, want %s", got, tc.want)
			}
		})
	}
}
