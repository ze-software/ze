package flowspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 8955 Section 8 frames the VPNv4 Flow Specification NLRI as an 8-octet Route
// Distinguisher followed by the Flow Specification value, under one length field that
// counts both. These tests pin ParseFlowSpecVPN and FlowSpecVPN.WriteTo to that frame.

// vpnWireDest24 is one VPNv4 Flow Specification NLRI: length 13, RD 100:100 (type 0),
// then a destination-prefix component for 10.0.0.0/24 (type 1, length 24, 3 octets).
var vpnWireDest24 = []byte{
	0x0d,
	0x00, 0x00, 0x00, 0x64, 0x00, 0x00, 0x00, 0x64,
	0x01, 0x18, 0x0a, 0x00, 0x00,
}

// TestRFC8955VPNNLRIStructure pins the RD-then-value layout of Section 8.
//
// VALIDATES: RFC8955-8-1, the RD precedes the Flow Specification value.
// PREVENTS: reading the value as an RD, or writing the value before the RD.
//
// RFC requirement: RFC8955-8-1 positive -- the 8 octets after the length decode as the Route Distinguisher and the octets after them as the Flow Specification value (§8).
// RFC requirement: RFC8955-8-1 negative -- an NLRI too short to hold the fixed-length 8-octet Route Distinguisher is refused (§8).
func TestRFC8955VPNNLRIStructure(t *testing.T) {
	fsv, err := ParseFlowSpecVPN(IPv4FlowSpecVPN, vpnWireDest24)
	require.NoError(t, err)

	wantRD := RouteDistinguisher{Type: RDType0, Value: [6]byte{0x00, 0x64, 0x00, 0x00, 0x00, 0x64}}
	assert.Equal(t, wantRD, fsv.RD(), "the first 8 octets are the Route Distinguisher")

	components := fsv.Components()
	require.Len(t, components, 1, "the octets after the RD are the Flow Specification value")
	assert.Equal(t, FlowDestPrefix, components[0].Type())
	assert.Equal(t, []byte{0x01, 0x18, 0x0a, 0x00, 0x00}, components[0].Bytes())

	buf := make([]byte, 64)
	n := fsv.WriteTo(buf, 0)
	assert.Equal(t, vpnWireDest24, buf[:n], "encoding writes the RD before the value")

	// Length 5 claims a value that cannot hold an 8-octet RD: RD 5 octets, no value.
	short := []byte{0x05, 0x00, 0x00, 0x00, 0x64, 0x00}
	_, err = ParseFlowSpecVPN(IPv4FlowSpecVPN, short)
	assert.ErrorIs(t, err, ErrFlowSpecTruncated, "an NLRI shorter than the RD is refused")
}

// TestRFC8955VPNLengthCoversRD pins the length field to RD plus value.
//
// VALIDATES: RFC8955-8-2, the length counts the RD and the value.
// PREVENTS: a length field that omits the 8 RD octets.
//
// RFC requirement: RFC8955-8-2 positive -- the NLRI length field equals 8 octets of RD plus the Flow Specification value length, on parse and on encode (§8).
// RFC requirement: RFC8955-8-2 negative -- a length field that counts only the Flow Specification value and not the Route Distinguisher is refused (§8).
func TestRFC8955VPNLengthCoversRD(t *testing.T) {
	fsv, err := ParseFlowSpecVPN(IPv4FlowSpecVPN, vpnWireDest24)
	require.NoError(t, err)

	valueLen := len(vpnWireDest24) - 1 - 8
	assert.Equal(t, 8+valueLen, int(vpnWireDest24[0]), "the length field counts the RD and the value")
	assert.Equal(t, len(vpnWireDest24), fsv.Len(), "Len reports the length octet plus what it counts")

	buf := make([]byte, 64)
	n := fsv.WriteTo(buf, 0)
	require.Equal(t, len(vpnWireDest24), n)
	assert.Equal(t, byte(8+valueLen), buf[0], "the encoded length counts the RD and the value")

	// The same RD and value under a length that counts only the value (5): the parser
	// cannot find an RD inside 5 octets and refuses rather than reading the value as RD.
	valueOnlyLen := append([]byte{byte(valueLen)}, vpnWireDest24[1:]...)
	_, err = ParseFlowSpecVPN(IPv4FlowSpecVPN, valueOnlyLen)
	assert.ErrorIs(t, err, ErrFlowSpecTruncated, "a length that excludes the RD is refused")
}
