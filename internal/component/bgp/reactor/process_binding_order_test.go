// Design: docs/architecture/api/architecture.md -- deterministic peer bus parsing across reloads
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestProcessBindingsParseInOneOrder verifies that a peer with a multi-endpoint
// bus filter block parses identically on every read so reload diffing never
// restarts an untouched Established session due to Go map iteration order.
//
// VALIDATES: repeated parses of an unchanged bus block produce zero restart
// reason in peerSettingsRestartReason.
// PREVENTS: nondeterministic map iteration order restarting Established peers
// on reload.
func TestProcessBindingsParseInOneOrder(t *testing.T) {
	tree := map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "127.0.0.1"},
			"local":  map[string]any{"ip": "127.0.0.1", "accept": "false"},
		},
		"session": map[string]any{"asn": map[string]any{"local": "1", "remote": "1"}},
		"bus": map[string]any{
			"send": map[string]any{
				"allow": []string{"bgp:update", "bgp:state", "bgp:refresh"},
				"to":    []string{"/bus/endpoint/exabgp-bridge", "/bus/endpoint/bgp-rib"},
			},
			"receive": map[string]any{
				"allow": []string{"bgp:update"},
				"from":  []string{"/bus/endpoint/exabgp-bridge", "/bus/endpoint/bgp-rib"},
			},
		},
	}
	ip := netip.MustParseAddr("127.0.0.1")

	first, err := parsePeerSettings("peer-1", tree, ip, 1, 1, 0x01020304)
	require.NoError(t, err)
	require.False(t, first.Bus.IsEmpty())

	// Fifty parses: map-order sensitivity would restart the peer on roughly
	// half of them, so 50 consecutive empty restart reasons proves stability.
	for i := range 50 {
		again, err := parsePeerSettings("peer-1", tree, ip, 1, 1, 0x01020304)
		require.NoError(t, err)
		require.Empty(t, peerSettingsRestartReason(first, again, nil),
			"parse %d of an unchanged config must not restart the peer", i)
	}
}
