package bgpconfig

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC9252-3.2.1-4 positive -- local route configuration advertises zero in the transposed SID bits while preserving adjacent bits.
func TestSRv6OriginTranspositionKeepsOnlyUntransposedBits(t *testing.T) {
	attrs, err := ParseRouteAttributes(&StaticRouteConfig{
		PrefixSID: "l3-service 2001:db8:1:2:8000:: 0x13 [64,0,32,0,15,65]",
	})
	require.NoError(t, err)
	require.Len(t, attrs.PrefixSID, 37)
	require.Equal(t, netip.MustParseAddr("2001:db8:1:2:8000::").AsSlice(), attrs.PrefixSID[8:24])
	require.Equal(t, []byte{1, 0, 6, 64, 0, 32, 0, 15, 65}, attrs.PrefixSID[28:])
}

// RFC requirement: RFC9252-3.2.1-4 negative -- local configuration refuses a SID with a set bit in the transposed range instead of advertising conflicting SID and label bits.
func TestSRv6OriginRefusesNonzeroTransposedBits(t *testing.T) {
	attrs, err := ParseRouteAttributes(&StaticRouteConfig{
		PrefixSID: "l3-service 2001:db8:1:2:8001:: 0x13 [64,0,32,0,15,65]",
	})
	require.Error(t, err)
	require.Nil(t, attrs)
}

// RFC requirement: RFC9252-3.2.1-6 positive -- an End.DT4 route advertises a zero Argument Length.
func TestSRv6OriginWithoutArguments(t *testing.T) {
	attrs, err := ParseRouteAttributes(&StaticRouteConfig{
		PrefixSID: "l3-service 2001:db8:1:2:: 0x13 [64,0,32,0,0,0]",
	})
	require.NoError(t, err)
	require.Len(t, attrs.PrefixSID, 37)
	require.Equal(t, []byte{0, 0x13}, attrs.PrefixSID[25:27])
	require.Equal(t, byte(0), attrs.PrefixSID[34])
}

// RFC requirement: RFC9252-3.2.1-6 negative -- a nonzero Argument Length cannot be advertised for End.DT4, which has no argument.
func TestSRv6OriginRefusesArgumentsForDT4(t *testing.T) {
	attrs, err := ParseRouteAttributes(&StaticRouteConfig{
		PrefixSID: "l3-service 2001:db8:1:2:: 0x13 [64,0,16,16,0,0]",
	})
	require.Error(t, err)
	require.Nil(t, attrs)
}

func TestSRv6OriginDT2MAllowsESIArgument(t *testing.T) {
	attrs, err := ParseRouteAttributes(&StaticRouteConfig{
		PrefixSID: "l2-service 2001:db8:1:2:3:4:: 0x18 [64,0,16,16,0,0]",
	})
	require.NoError(t, err)
	require.Len(t, attrs.PrefixSID, 37)
	require.Equal(t, byte(6), attrs.PrefixSID[0])
	require.Equal(t, []byte{0, 0x18}, attrs.PrefixSID[25:27])
	require.Equal(t, byte(16), attrs.PrefixSID[34])
}
