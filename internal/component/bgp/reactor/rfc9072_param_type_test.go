package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// rfc9072ParamTypes writes an OPEN carrying the given Optional Parameters and
// returns the type octet of every parameter in it, walking the framing the
// OPEN announces: classic one-octet Parameter Lengths, or, behind the 255/255
// envelope, two-octet ones.
func rfc9072ParamTypes(t *testing.T, params []byte, extended bool) (types []byte, envelope bool) {
	t.Helper()
	open := &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0xC0000201,
		OptionalParams: params, ExtendedParams: extended}
	wire := message.PackTo(open, nil)
	body := wire[message.HeaderLen:]

	off, end, lengthOctets := 10, 10+int(body[9]), 1
	if body[9] == 0xFF && body[10] == 0xFF {
		envelope = true
		off, end, lengthOctets = 13, 13+int(binary.BigEndian.Uint16(body[11:13])), 2
	}
	require.Equal(t, len(body), end, "the parameters fill the OPEN")
	for off < end {
		types = append(types, body[off])
		paramLen := int(body[off+1])
		if lengthOctets == 2 {
			paramLen = int(binary.BigEndian.Uint16(body[off+1 : off+3]))
		}
		off += 1 + lengthOctets + paramLen
	}
	require.Equal(t, end, off, "the parameter framing ends where the OPEN does")
	return types, envelope
}

// TestRFC9072ParameterType255OnlyAsTheExtendedMarker proves that Ze uses
// Optional Parameter type 255 only as the RFC 9072 envelope marker.
//
// VALIDATES: the Optional Parameters buildOptionalParams produces, below and
// above 255 octets, are written by the OPEN encoder as parameters of type 2
// only; 255 appears only as the Non-Ext OP Type of the extended envelope, and
// only when the parameters exceed 255 octets.
// PREVENTS: 255 used as a real parameter type, and the marker emitted on an
// OPEN whose parameters fit the classic encoding.
//
// Method: the capabilities carry 0xFF octets in their values (AS 4294967295,
// AFI 65535, SAFI 255), so a builder that took any value octet as a type would
// be caught; the OPEN is written and every parameter type read back from the
// framing the OPEN announces.
//
// RFC requirement: RFC9072-3-2 positive -- above 255 parameter octets the OPEN carries the
// 255/255 envelope and every parameter behind it is type 2.
// RFC requirement: RFC9072-3-2 negative -- with capability values full of 0xFF octets, no
// parameter is ever type 255, and an OPEN whose parameters fit 255 octets carries no 255
// marker at all.
func TestRFC9072ParameterType255OnlyAsTheExtendedMarker(t *testing.T) {
	small := []capability.Capability{
		&capability.ASN4{ASN: 0xFFFFFFFF},
		&capability.Multiprotocol{AFI: capability.AFI(0xFFFF), SAFI: capability.SAFI(0xFF)},
	}
	params, extended := buildOptionalParams(small)
	types, envelope := rfc9072ParamTypes(t, params, extended)
	require.False(t, envelope, "parameters of 255 octets or fewer take the classic encoding")
	require.Equal(t, []byte{2}, types)

	var large []capability.Capability
	for range 50 {
		large = append(large, &capability.Multiprotocol{AFI: capability.AFI(0xFFFF), SAFI: capability.SAFI(0xFF)})
	}
	large = append(large, &capability.ASN4{ASN: 0xFFFFFFFF})
	params, extended = buildOptionalParams(large)
	require.Greater(t, len(params), 255)
	types, envelope = rfc9072ParamTypes(t, params, extended)
	require.True(t, envelope, "parameters above 255 octets take the extended envelope")
	for _, paramType := range types {
		require.Equal(t, byte(2), paramType)
	}
}
