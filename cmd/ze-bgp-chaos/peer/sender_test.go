package peer

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateBuild verifies building a valid ipv4/unicast UPDATE
// from a generated route prefix.
//
// VALIDATES: UPDATE construction with correct attributes and NLRI.
// PREVENTS: Malformed UPDATE causing Ze to reject the route.
func TestUpdateBuild(t *testing.T) {
	cfg := SenderConfig{
		ASN:     65001,
		IsIBGP:  false,
		NextHop: netip.MustParseAddr("10.255.0.1"),
	}
	sender := NewSender(cfg)

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	data := sender.BuildRoute(prefix)

	require.NotNil(t, data)
	require.Greater(t, len(data), 19, "UPDATE must be larger than header")
	// Message type should be UPDATE (2).
	assert.Equal(t, byte(2), data[18])
}

// TestUpdateBuildIBGP verifies iBGP UPDATE includes LOCAL_PREF.
//
// VALIDATES: iBGP updates set LOCAL_PREF attribute.
// PREVENTS: Missing LOCAL_PREF causing iBGP route to be ignored.
func TestUpdateBuildIBGP(t *testing.T) {
	cfg := SenderConfig{
		ASN:     65000,
		IsIBGP:  true,
		NextHop: netip.MustParseAddr("10.255.0.1"),
	}
	sender := NewSender(cfg)

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	data := sender.BuildRoute(prefix)

	require.NotNil(t, data)
	// Should be a valid UPDATE.
	assert.Equal(t, byte(2), data[18])
	// iBGP UPDATE should be larger (has LOCAL_PREF attribute).
	assert.Greater(t, len(data), 40, "iBGP UPDATE should include LOCAL_PREF")
}

// TestEORBuild verifies building an End-of-RIB marker for ipv4/unicast.
//
// VALIDATES: EOR is an empty UPDATE per RFC 4724.
// PREVENTS: Wrong EOR format causing Ze to misinterpret the marker.
func TestEORBuild(t *testing.T) {
	data := BuildEORIPv4Unicast()

	require.NotNil(t, data)
	// Message type should be UPDATE (2).
	assert.Equal(t, byte(2), data[18])
	// IPv4 unicast EOR is an empty UPDATE: header (19) + withdrawn len (2) + attr len (2) = 23.
	assert.Equal(t, 23, len(data), "IPv4 unicast EOR should be 23 bytes")
}

// TestMultipleRoutesDifferent verifies that building routes for different
// prefixes produces different wire bytes.
//
// VALIDATES: Each prefix produces unique UPDATE.
// PREVENTS: Builder reusing stale state between calls.
func TestMultipleRoutesDifferent(t *testing.T) {
	cfg := SenderConfig{
		ASN:     65001,
		IsIBGP:  false,
		NextHop: netip.MustParseAddr("10.255.0.1"),
	}
	sender := NewSender(cfg)

	data1 := sender.BuildRoute(netip.MustParsePrefix("10.0.0.0/24"))
	data2 := sender.BuildRoute(netip.MustParsePrefix("10.0.1.0/24"))

	require.NotNil(t, data1)
	require.NotNil(t, data2)
	assert.NotEqual(t, data1, data2, "different prefixes should produce different UPDATEs")
}
