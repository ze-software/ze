// VALIDATES: SR-Policy NLRI type (SAFI 73, RFC 9830) parse, encode, round-trip.
// PREVENTS: Wrong distinguisher/color/endpoint extraction, wrong wire length, missing family registration.

package srpolicy

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

func TestSRPolicyFamilyRegistered(t *testing.T) {
	f4, ok := family.LookupFamily("ipv4/sr-policy")
	require.True(t, ok, "ipv4/sr-policy must be registered")
	assert.Equal(t, family.AFIIPv4, f4.AFI)
	assert.Equal(t, family.SAFISRPolicy, f4.SAFI)

	f6, ok := family.LookupFamily("ipv6/sr-policy")
	require.True(t, ok, "ipv6/sr-policy must be registered")
	assert.Equal(t, family.AFIIPv6, f6.AFI)
	assert.Equal(t, family.SAFISRPolicy, f6.SAFI)
}

func TestSRPolicyParseIPv4(t *testing.T) {
	body := make([]byte, 12)
	binary.BigEndian.PutUint32(body[0:4], 42)  // distinguisher
	binary.BigEndian.PutUint32(body[4:8], 100) // color
	ep4 := netip.MustParseAddr("10.0.0.1").As4()
	copy(body[8:12], ep4[:]) // endpoint

	sp, err := Parse(family.AFIIPv4, body)
	require.NoError(t, err)
	assert.Equal(t, uint32(42), sp.Distinguisher())
	assert.Equal(t, uint32(100), sp.Color())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), sp.Endpoint())
}

func TestSRPolicyParseIPv6(t *testing.T) {
	body := make([]byte, 24)
	binary.BigEndian.PutUint32(body[0:4], 1)   // distinguisher
	binary.BigEndian.PutUint32(body[4:8], 200) // color
	ep := netip.MustParseAddr("2001:db8::1")
	a16 := ep.As16()
	copy(body[8:24], a16[:])

	sp, err := Parse(family.AFIIPv6, body)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), sp.Distinguisher())
	assert.Equal(t, uint32(200), sp.Color())
	assert.Equal(t, ep, sp.Endpoint())
}

func TestSRPolicyRoundTrip(t *testing.T) {
	tests := []struct {
		name          string
		afi           family.AFI
		distinguisher uint32
		color         uint32
		endpoint      netip.Addr
	}{
		{"ipv4_zero", family.AFIIPv4, 0, 0, netip.MustParseAddr("0.0.0.0")},
		{"ipv4_typical", family.AFIIPv4, 0, 100, netip.MustParseAddr("10.0.0.1")},
		{"ipv4_max", family.AFIIPv4, 0xFFFFFFFF, 0xFFFFFFFF, netip.MustParseAddr("255.255.255.255")},
		{"ipv6_typical", family.AFIIPv6, 1, 200, netip.MustParseAddr("2001:db8::1")},
		{"ipv6_any", family.AFIIPv6, 0, 100, netip.MustParseAddr("::")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := New(tt.afi, tt.distinguisher, tt.color, tt.endpoint)

			buf := make([]byte, sp.Len())
			n := sp.WriteTo(buf, 0)
			assert.Equal(t, sp.Len(), n)

			// Parse back from body (skip 1-byte length prefix)
			parsed, err := Parse(tt.afi, buf[1:])
			require.NoError(t, err)
			assert.Equal(t, tt.distinguisher, parsed.Distinguisher())
			assert.Equal(t, tt.color, parsed.Color())
			assert.Equal(t, tt.endpoint, parsed.Endpoint())
		})
	}
}

func TestSRPolicyFamily(t *testing.T) {
	sp4 := New(family.AFIIPv4, 0, 100, netip.MustParseAddr("10.0.0.1"))
	assert.Equal(t, family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}, sp4.Family())

	sp6 := New(family.AFIIPv6, 0, 100, netip.MustParseAddr("2001:db8::1"))
	assert.Equal(t, family.Family{AFI: family.AFIIPv6, SAFI: family.SAFISRPolicy}, sp6.Family())
}

func TestSRPolicyLen(t *testing.T) {
	sp4 := New(family.AFIIPv4, 0, 100, netip.MustParseAddr("10.0.0.1"))
	assert.Equal(t, 13, sp4.Len()) // 1 (length prefix) + 12 (body)

	sp6 := New(family.AFIIPv6, 0, 100, netip.MustParseAddr("2001:db8::1"))
	assert.Equal(t, 25, sp6.Len()) // 1 (length prefix) + 24 (body)
}

func TestSRPolicyParseTruncated(t *testing.T) {
	_, err := Parse(family.AFIIPv4, make([]byte, 11)) // need 12
	assert.ErrorIs(t, err, ErrSRPolicyTruncated)

	_, err = Parse(family.AFIIPv6, make([]byte, 23)) // need 24
	assert.ErrorIs(t, err, ErrSRPolicyTruncated)
}

func TestSRPolicyString(t *testing.T) {
	sp := New(family.AFIIPv4, 0, 100, netip.MustParseAddr("10.0.0.1"))
	s := sp.String()
	assert.Contains(t, s, "sr-policy")
	assert.Contains(t, s, "10.0.0.1")
}

func TestSRPolicySupportsAddPath(t *testing.T) {
	sp := New(family.AFIIPv4, 0, 100, netip.MustParseAddr("10.0.0.1"))
	assert.False(t, sp.SupportsAddPath())
	assert.False(t, sp.HasPathID())
	assert.Equal(t, uint32(0), sp.PathID())
}

func TestSRPolicyAppendJSON(t *testing.T) {
	sp := New(family.AFIIPv4, 0, 100, netip.MustParseAddr("10.0.0.1"))
	got := string(sp.AppendJSON(nil))
	assert.JSONEq(t, `{"color":100,"distinguisher":0,"endpoint":"10.0.0.1"}`, got)
}

func TestSRPolicyDecodeNLRIHex(t *testing.T) {
	result, err := DecodeNLRIHex("ipv4/sr-policy", "00000000000000640A000001")
	require.NoError(t, err)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint32(0), m["distinguisher"])
	assert.Equal(t, uint32(100), m["color"])
	assert.Equal(t, "10.0.0.1", m["endpoint"])
}

func TestSRPolicyDecodeNLRIHexIPv6(t *testing.T) {
	result, err := DecodeNLRIHex("ipv6/sr-policy", "0000002A000000C820010DB8000000000000000000000001")
	require.NoError(t, err)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint32(42), m["distinguisher"])
	assert.Equal(t, uint32(200), m["color"])
	assert.Equal(t, "2001:db8::1", m["endpoint"])
}
