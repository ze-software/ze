// Design: docs/guide/bgp-policy.md -- read-only export kind simulation.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPolicyDryRunExportDefaultsToLearned proves an omitted simulated kind
// reports the stricter learned-route answer without changing the real peer.
func TestPolicyDryRunExportDefaultsToLearned(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("192.0.2.231"), 65000, 65000, 0x01010101)
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	r.peers[settings.PeerKey()] = peer
	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun(peer.sourceOwner, directionExport, "", dryRunUpdateBody(t), true, "")
	require.NoError(t, err)
	require.Equal(t, dryRunActionReject, result.Action)
	require.False(t, settings.ExportPolicy.Configured(), "a simulation must not install policy")
	require.Empty(t, peer.adjOut.routes, "a simulation must not send a route")
}
