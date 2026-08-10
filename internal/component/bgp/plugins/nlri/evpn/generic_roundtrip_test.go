package evpn

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An EVPN route type ze does not recognize becomes an EVPNGeneric holding the opaque body.
// Ze relays such routes rather than discarding them (RFC 7606 Section 5.4 divergence,
// disclosed in docs/features/rfc-status.md), so the encoding side has to be exactly right:
// a relay that corrupts what it relays is worse than one that drops it.
//
// EVPNGeneric.WriteTo used to copy the body ALONE, omitting the [route-type][length]
// header that every sibling type writes. Two contracts broke:
//
//   - Len() promises len(body)+2, so plugin.go's `make([]byte, Len())` followed by
//     WriteTo left two trailing ZERO octets on the wire.
//   - appendRawField (json.go) drops scratch[:2] to strip the header, so the JSON "raw"
//     field lost the first two octets of the body instead.

// unknownTypeNLRI is one EVPN NLRI of route type 8, which ze does not parse:
// [type=8][length=6][6 opaque body octets].
var unknownTypeNLRI = []byte{0x08, 0x06, 0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}

// VALIDATES: an unrecognized EVPN route type survives parse then re-encode byte for byte.
// PREVENTS: a ze route reflector silently corrupting the routes it relays for types newer
// than its own codec -- which is the entire justification for relaying them at all.
func TestEVPNGenericRoundTripsByteForByte(t *testing.T) {
	parsed, rest, err := ParseEVPN(unknownTypeNLRI, false)
	require.NoError(t, err)
	require.Empty(t, rest)

	g, ok := parsed.(*eVPNGeneric)
	require.True(t, ok, "route type 8 must fall through to EVPNGeneric")
	assert.Equal(t, EVPNRouteType(8), g.RouteType())

	assert.Equal(t, unknownTypeNLRI, g.Bytes(),
		"Bytes must return the full wire encoding, header included, like every other type")

	buf := make([]byte, g.Len())
	n := g.WriteTo(buf, 0)
	assert.Equal(t, g.Len(), n,
		"WriteTo must write exactly Len() octets, or the caller ships trailing zeros")
	assert.Equal(t, unknownTypeNLRI, buf)
}

// VALIDATES: Len() agrees with what WriteTo actually writes, at a non-zero offset too.
// PREVENTS: the trailing-zero corruption. plugin.go sizes its buffer from Len() and never
// looks at WriteTo's return, so a short write is invisible there.
func TestEVPNGenericLenMatchesWriteTo(t *testing.T) {
	parsed, _, err := ParseEVPN(unknownTypeNLRI, false)
	require.NoError(t, err)

	const off = 5
	buf := make([]byte, off+parsed.Len()+4)
	for i := range buf {
		buf[i] = 0xAA // poison, so a short write is visible as surviving 0xAA
	}
	n := parsed.WriteTo(buf, off)

	require.Equal(t, parsed.Len(), n)
	assert.Equal(t, unknownTypeNLRI, buf[off:off+n])
	assert.Equal(t, byte(0xAA), buf[off+n], "WriteTo must not run past what it reported")
}

// VALIDATES: an empty body is encoded as a well-formed zero-length NLRI.
// PREVENTS: an off-by-one in the header write when there is nothing after it.
func TestEVPNGenericEmptyBody(t *testing.T) {
	nlri := []byte{0x09, 0x00} // type 9, length 0
	parsed, rest, err := ParseEVPN(nlri, false)
	require.NoError(t, err)
	require.Empty(t, rest)
	require.Equal(t, 2, parsed.Len())
	assert.Equal(t, nlri, parsed.Bytes())
}
