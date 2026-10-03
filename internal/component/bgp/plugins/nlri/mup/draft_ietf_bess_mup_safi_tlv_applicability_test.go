package mup

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.5-1 positive -- ST2-only session, interwork endpoint, and source TLVs do not replace the mandatory ST1 fields exposed by the NLRI decoder.
func TestMUPST1IgnoresST2OnlyTLVs(t *testing.T) {
	wire := t1stNLRI(t, t1stBody+"20C0000201"+
		"01050000567807"+"0204C6336401"+"0304CB007101")
	decoded, err := DecodeNLRIHex("ipv4/mup", hex.EncodeToString(wire), false)
	require.NoError(t, err)
	fields, ok := decoded.(map[string]any)
	require.True(t, ok, "decoded NLRI is %T", decoded)
	require.Equal(t, "12345", fields["teid"])
	require.Equal(t, "9", fields["qfi"])
	require.Equal(t, "10.0.0.1", fields["endpoint_ip"])
	require.Equal(t, "192.0.2.1", fields["source_ip"])
}

// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.5-1 negative -- an ST2-only source TLV cannot make an ST1 route malformed merely because its value has no address interpretation; only its outer framing applies to ST1.
func TestMUPST1DoesNotInterpretInapplicableAddressTLV(t *testing.T) {
	wire := t1stNLRI(t, t1stBody+"20C0000201"+"0301FF")
	decoded, err := DecodeNLRIHex("ipv4/mup", hex.EncodeToString(wire), false)
	require.NoError(t, err)
	fields, ok := decoded.(map[string]any)
	require.True(t, ok, "decoded NLRI is %T", decoded)
	require.Equal(t, "192.0.2.1", fields["source_ip"])
	require.Equal(t, "10.0.0.1", fields["endpoint_ip"])
	require.Equal(t, "12345", fields["teid"])
}
