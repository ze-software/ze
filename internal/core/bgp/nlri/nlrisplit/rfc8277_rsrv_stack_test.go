// Design: docs/architecture/wire/qualifiers.md -- label stack entries
// RFC: rfc/short/rfc8277.md -- RFC8277-2.2-3, Rsrv ignored on reception
// Related: rfc8277_test.go -- the same rule for a one-label NLRI

package nlrisplit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRFC8277RsrvIgnoredOnReceiveLabelStack extends the Rsrv reception rule to
// a stack of three labels, the Section 2.3 encoding, whose Rsrv paragraph repeats
// the Section 2.2 sentence: "This 3-bit field SHOULD be set to zero on
// transmission and MUST be ignored on reception."
//
// VALIDATES: ExtractLabels and SplitLabeled read the same labels 100, 200, 300,
// the same prefix and the same NLRI boundary whether every entry's Rsrv bits are
// 000 or 111. The NLRI is 10.0.0.0/8 followed by a second NLRI 11.0.0.0/8 with
// one label 400, so a stack walk that a set Rsrv bit shifted would misframe the
// second route.
// PREVENTS: a stack reader that treats a Rsrv bit as the S bit, or rejects a
// non-zero Rsrv, on any entry but the first.
//
// RFC requirement: RFC8277-2.2-3 positive -- a three-label stack with Rsrv 000 on every entry (50 000640 000c80 0012c1 0a) decodes to labels 100, 200, 300 over 10.0.0.0/8 and frames two NLRIs.
// RFC requirement: RFC8277-2.2-3 negative -- the same stack with Rsrv 111 on every entry (50 00064e 000c8e 0012cf 0a) is not rejected: the same labels, prefix and framing of both NLRIs come back.
func TestRFC8277RsrvIgnoredOnReceiveLabelStack(t *testing.T) {
	t.Parallel()

	// Second NLRI: 11.0.0.0/8 with label 400 (0x190): 20 001901 0b.
	second := []byte{0x20, 0x00, 0x19, 0x01, 0x0b}
	conformant := append([]byte{0x50, 0x00, 0x06, 0x40, 0x00, 0x0c, 0x80, 0x00, 0x12, 0xc1, 0x0a}, second...)
	rsrvSet := append([]byte{0x50, 0x00, 0x06, 0x4e, 0x00, 0x0c, 0x8e, 0x00, 0x12, 0xcf, 0x0a}, second...)

	for _, tc := range []struct {
		name string
		wire []byte
	}{{"rsrv 000", conformant}, {"rsrv 111", rsrvSet}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			labels, cidr, err := ExtractLabels(tc.wire[:11], false)
			require.NoError(t, err, "Rsrv is ignored on reception, never a reason to refuse")
			assert.Equal(t, []uint32{100, 200, 300}, labels)
			assert.Equal(t, []byte{8, 10}, cidr, "CIDR bytes are [prefix-bits][prefix]")

			framed, err := splitAll(t, SplitLabeled, tc.wire, false)
			require.NoError(t, err)
			require.Len(t, framed, 2)
			assert.Equal(t, tc.wire[:11], framed[0], "first NLRI ends after its prefix octet")
			assert.Equal(t, second, framed[1], "second NLRI starts where the first ends")
		})
	}
}
