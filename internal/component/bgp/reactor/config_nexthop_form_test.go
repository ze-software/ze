package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// peerTreeWith builds the smallest config tree parsePeerFromTree accepts, with
// the local IP leaf set.
func peerTreeWith(localIP string) map[string]any {
	return map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "2001:db8::2"},
			"local":  map[string]any{"ip": localIP},
		},
		"session": map[string]any{"asn": map[string]any{"remote": "65001"}},
	}
}

// TestValidatePeerGlobalNextHop drives the RFC 2545 Section 3 next-hop form
// guard directly. Explicit next-hop values in export policies are validated by
// filter_modify using the same attribute.ValidateGlobalNextHop check.
//
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop."
//
// VALIDATES: ValidatePeerGlobalNextHop refuses link-local IPv6 addresses, names
// the peer and leaf, and accepts global IPv6 and IPv4 addresses.
// PREVENTS: a link-local address occupying the global next-hop slot.
func TestValidatePeerGlobalNextHop(t *testing.T) {
	err := ValidatePeerGlobalNextHop("peer1", "next-hop", netip.MustParseAddr("fe80::cafe"))
	require.Error(t, err)
	assert.ErrorIs(t, err, attribute.ErrLinkLocalNextHop)
	assert.Contains(t, err.Error(), "peer1")
	assert.Contains(t, err.Error(), "next-hop")

	for _, raw := range []string{"2001:db8::ffff", "192.0.2.1"} {
		t.Run("accept_"+raw, func(t *testing.T) {
			require.NoError(t, ValidatePeerGlobalNextHop("peer1", "next-hop", netip.MustParseAddr(raw)))
		})
	}
}

// TestParsePeerFromTreeAcceptsLocalIP verifies applyLocalAddress accepts global
// IPv6, loopback, IPv4, and "auto" local transport endpoints.
//
// VALIDATES: applyLocalAddress parses valid local IP values without error.
// PREVENTS: transport parsing rejecting valid local endpoint forms.
func TestParsePeerFromTreeAcceptsLocalIP(t *testing.T) {
	for _, localIP := range []string{"2001:db8::1", "::1", "192.0.2.1", "auto"} {
		t.Run("local ip "+localIP, func(t *testing.T) {
			ps, err := parsePeerFromTree("peer1", peerTreeWith(localIP), 65000, 0x0a000001)
			require.NoError(t, err)
			require.NotNil(t, ps)
		})
	}
}
