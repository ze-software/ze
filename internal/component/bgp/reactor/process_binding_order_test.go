package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// A peer that attaches two processes MUST parse to the same PeerSettings on
// every read of the same config. The reload diff compares the running peer's
// settings against the re-parsed file with reflect.DeepEqual
// (peerSettingsSwapPlan, peer_settings_apply.go), so an attach list whose order
// followed Go's map iteration read as a changed peer on half the reloads and
// restarted an Established session that the operator had not touched
// (test/exabgp-compat/api/api-reload.ci: "connection 1: EOF" right after
// "received SIGHUP").
func TestProcessBindingsParseInOneOrder(t *testing.T) {
	tree := map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "127.0.0.1"},
			"local":  map[string]any{"ip": "127.0.0.1", "accept": "false"},
		},
		"session": map[string]any{"asn": map[string]any{"local": "1", "remote": "1"}},
		"attach": map[string]any{"process": map[string]any{
			"bgp-rib":       map[string]any{"receive": "update state refresh", "send": "update"},
			"exabgp-bridge": map[string]any{"receive": "*", "send": "*"},
		}},
	}
	ip := netip.MustParseAddr("127.0.0.1")

	first, err := parsePeerSettings("peer-1", tree, ip, 1, 1, 0x01020304)
	require.NoError(t, err)
	require.Len(t, first.ProcessBindings, 2)

	// Fifty parses: the map-order defect restarts the peer on about half of
	// them, so one unchanged result here would be a 2^-50 accident.
	for i := range 50 {
		again, err := parsePeerSettings("peer-1", tree, ip, 1, 1, 0x01020304)
		require.NoError(t, err)
		require.Empty(t, peerSettingsRestartReason(first, again, nil),
			"parse %d of an unchanged config must not restart the peer", i)
	}
	require.Equal(t, "bgp-rib", first.ProcessBindings[0].PluginName, "attach blocks are ordered by process name")
}
