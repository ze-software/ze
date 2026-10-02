package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// rfc8092Attrs returns ORIGIN IGP, an empty AS_PATH, NEXT_HOP 192.0.2.1 and a
// LARGE_COMMUNITY attribute (flags 0xc0, code 32) carrying value.
func rfc8092Attrs(value []byte) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
		0xc0, 0x20, byte(len(value)),
	}
	return append(attrs, value...)
}

// TestRFC8092LargeCommunityAnyGlobalAdministratorIsWellFormed proves that the
// Global Administrator value never makes a LARGE_COMMUNITY malformed.
//
// Method: one well-formed 12-octet value per Global Administrator, covering
// the reserved ASNs 0, 23456 (AS_TRANS), 65535 and 4294967295, the
// documentation ASN 64496, the private ASN 4200000000 and the unallocated ASN
// 1000000, each with Local Data Parts 1 and 2. The validator must return
// RFC7606ActionNone for every one.
//
// RFC requirement: RFC8092-6-1 positive -- ValidateUpdateRFC7606 returns
// RFC7606ActionNone for a LARGE_COMMUNITY whose Global Administrator is 0,
// 23456, 64496, 65535, 1000000, 4200000000 or 4294967295.
func TestRFC8092LargeCommunityAnyGlobalAdministratorIsWellFormed(t *testing.T) {
	administrators := map[string][]byte{
		"reserved 0":          {0x00, 0x00, 0x00, 0x00},
		"AS_TRANS 23456":      {0x00, 0x00, 0x5b, 0xa0},
		"documentation 64496": {0x00, 0x00, 0xfb, 0xf0},
		"reserved 65535":      {0x00, 0x00, 0xff, 0xff},
		"unallocated 1000000": {0x00, 0x0f, 0x42, 0x40},
		"private 4200000000":  {0xfa, 0x56, 0xea, 0x00},
		"reserved 4294967295": {0xff, 0xff, 0xff, 0xff},
	}
	for name, administrator := range administrators {
		t.Run(name, func(t *testing.T) {
			value := append(append([]byte(nil), administrator...),
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x02)
			result := ValidateUpdateRFC7606(rfc8092Attrs(value), true, false, false)
			require.Equal(t, RFC7606ActionNone, result.Action,
				"Global Administrator % x must not make the attribute malformed", administrator)
		})
	}
}

// TestRFC8092LargeCommunityLengthNotANonZeroMultipleOf12IsMalformed proves
// both halves of "not a non-zero multiple of 12": the zero length and a length
// that 12 does not divide.
//
// Method: LARGE_COMMUNITY values of 0, 1, 11, 13, 23 and 25 octets. Each must
// draw RFC7606ActionTreatAsWithdraw naming attribute code 32.
//
// RFC requirement: RFC8092-6-2 positive -- a LARGE_COMMUNITY of length 0, 1,
// 11, 13, 23 or 25 is malformed: ValidateUpdateRFC7606 returns
// RFC7606ActionTreatAsWithdraw with AttrCode 32.
func TestRFC8092LargeCommunityLengthNotANonZeroMultipleOf12IsMalformed(t *testing.T) {
	for _, length := range []int{0, 1, 11, 13, 23, 25} {
		value := make([]byte, length)
		result := ValidateUpdateRFC7606(rfc8092Attrs(value), true, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action, "length %d", length)
		require.Equal(t, uint8(32), result.AttrCode, "length %d", length)
	}
}

// TestRFC8092LargeCommunityNonZeroMultipleOf12IsNotMalformed is the
// counterpart: lengths 12, 24 and 36 are well formed.
//
// RFC requirement: RFC8092-6-2 negative -- a LARGE_COMMUNITY of length 12, 24
// or 36 is not malformed: ValidateUpdateRFC7606 returns RFC7606ActionNone.
func TestRFC8092LargeCommunityNonZeroMultipleOf12IsNotMalformed(t *testing.T) {
	for _, length := range []int{12, 24, 36} {
		value := make([]byte, length)
		for i := range value {
			value[i] = byte(i + 1)
		}
		result := ValidateUpdateRFC7606(rfc8092Attrs(value), true, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action, "length %d", length)
	}
}
