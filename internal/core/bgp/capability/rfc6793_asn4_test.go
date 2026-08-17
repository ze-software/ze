// RFC: rfc/short/rfc6793.md — four-octet AS number capability (code 65)

package capability

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRFC6793ASN4CapabilityCarriesSpeakerAS drives the capability encoder
// (ASN4.WriteTo, capability.go) and the decoder (parseASN4) for the four-octet
// AS number capability.
//
// RFC requirement: RFC6793-4.1-2 positive -- the capability is code 65 with Capability Length
// 4 and its Capability Value field carries the speaker's own four-octet AS number, which the
// decoder recovers unchanged (including an AS above 65535 that the OPEN header cannot hold).
func TestRFC6793ASN4CapabilityCarriesSpeakerAS(t *testing.T) {
	t.Parallel()
	for _, asn := range []uint32{1, 65535, 65536, 4200000001, 4294967295} {
		c := &ASN4{ASN: asn}
		require.Equal(t, CodeASN4, c.Code())
		require.Equal(t, Code(65), c.Code())

		buf := make([]byte, 16)
		n := c.WriteTo(buf, 0)
		require.Equal(t, 6, n, "2 octets of header + 4 octets of value")
		require.Equal(t, c.Len(), n)
		assert.Equal(t, byte(65), buf[0], "Capability Code")
		assert.Equal(t, byte(4), buf[1], "Capability Length")
		assert.Equal(t, asn, binary.BigEndian.Uint32(buf[2:6]), "Capability Value is the AS")

		decoded, err := parseASN4(buf[2:6])
		require.NoError(t, err)
		assert.Equal(t, asn, decoded.ASN)
	}
}

// TestRFC6793ASN4CapabilityValueMustBeFourOctets proves the four-octet Capability
// Value is required rather than optional: parseASN4 rejects a shorter value
// instead of zero-extending it into a plausible-looking AS number.
//
// RFC requirement: RFC6793-4.1-2 negative -- a capability whose value is shorter than the
// four octets needed to carry an AS number is rejected with ErrShortRead, so a truncated
// advertisement never yields a fabricated peer AS.
func TestRFC6793ASN4CapabilityValueMustBeFourOctets(t *testing.T) {
	t.Parallel()
	for _, short := range [][]byte{nil, {}, {0x00}, {0x00, 0x01}, {0x00, 0x01, 0x02}} {
		_, err := parseASN4(short)
		require.Error(t, err, "value of %d octets must be rejected", len(short))
		assert.True(t, errors.Is(err, ErrShortRead))
	}
}
