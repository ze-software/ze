// Design: docs/architecture/wire/capabilities.md -- Extended Message capability
// Related: docs/architecture/edge-cases/extended-message.md -- Extended Message negotiation
// RFC: rfc/short/rfc8654.md -- Extended Message capability code and length (Section 3)
//
// RFC 8654 Section 3: "The BGP Extended Message Capability is a new BGP
// capability [RFC5492] defined with Capability Code 6 and Capability Length
// 0." These tests read the octets ExtendedMessage.WriteTo puts on the wire and
// the capability Parse returns for received octets.
//
// VALIDATES: the Extended Message capability is written and read as code 6, length 0.
// PREVENTS: a code or length drift that a Code()-to-Code() round trip cannot see.

package capability

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRFC8654ExtendedMessageWireIsCode6Length0 reads the capability TLV that
// ExtendedMessage writes, at a non-zero offset so a write that ignores the
// offset is caught, and parses the canonical octets back.
//
// RFC requirement: RFC8654-3-2 positive -- ExtendedMessage.WriteTo writes Capability Code octet 0x06, Code() is 6, and Parse of the received TLV 06 00 returns exactly one *ExtendedMessage.
// RFC requirement: RFC8654-3-3 positive -- ExtendedMessage.WriteTo writes exactly the two octets 06 00 (Capability Length 0, no value) and returns 2 with Len() 2, and Parse of 06 00 returns one *ExtendedMessage with no error.
func TestRFC8654ExtendedMessageWireIsCode6Length0(t *testing.T) {
	t.Parallel()

	em := &ExtendedMessage{}
	require.Equal(t, Code(6), em.Code())
	require.Equal(t, 2, em.Len(), "header only: code and a zero length")

	buf := []byte{0xAA, 0xAA, 0xAA, 0xAA, 0xAA}
	written := em.WriteTo(buf, 1)
	require.Equal(t, 2, written)
	require.Equal(t, []byte{0xAA, 0x06, 0x00, 0xAA, 0xAA}, buf, "code 6, length 0, nothing else written")

	caps, err := Parse([]byte{0x06, 0x00})
	require.NoError(t, err)
	require.Len(t, caps, 1)
	_, ok := caps[0].(*ExtendedMessage)
	require.True(t, ok, "code 6 must parse as ExtendedMessage, got %T", caps[0])
}

// TestRFC8654ExtendedMessageNonZeroLengthRefused drives the received forms
// that break "Capability Length 0": a code-6 capability carrying one or more
// value octets. Parse refuses each with ErrInvalidLength rather than accepting
// it as Extended Message support.
//
// RFC requirement: RFC8654-3-3 negative -- a received code-6 capability with Capability Length 1 (06 01 00) or 4 (06 04 00 00 00 00) is refused by Parse with ErrInvalidLength, so no ExtendedMessage is accepted from a non-zero length.
func TestRFC8654ExtendedMessageNonZeroLengthRefused(t *testing.T) {
	t.Parallel()

	for _, data := range [][]byte{
		{0x06, 0x01, 0x00},
		{0x06, 0x04, 0x00, 0x00, 0x00, 0x00},
	} {
		caps, err := Parse(data)
		require.Error(t, err, "% x", data)
		require.True(t, errors.Is(err, ErrInvalidLength), "% x: got %v", data, err)
		require.Empty(t, caps, "% x", data)
	}
}
