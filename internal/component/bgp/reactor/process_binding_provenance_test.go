// Design: docs/architecture/api/architecture.md -- explicit peer attachments.
// Related: config.go -- binding parser; delivery_graph.go -- graph input producer.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestProcessBindingExplicitProvenance proves only parsed operator blocks
// request dissemination and both public readers preserve that distinction.
// Method: parse a real attachment, add existing derived and implicit grants,
// then inspect graph input and the reactor API's independent conversion.
func TestProcessBindingExplicitProvenance(t *testing.T) {
	address := netip.MustParseAddr("192.0.2.41")
	settings := &PeerSettings{Address: address}
	require.NoError(t, parseProcessBindingsFromTree(map[string]any{"attach": map[string]any{"process": map[string]any{
		"operator": map[string]any{"receive": "update-received state", "send": "update"},
	}}}, settings))
	require.NoError(t, EnsureProcessBinding(settings, "derived", "update state refresh", "update"))
	require.NoError(t, GrantImplicitReceive(settings, "implicit", "state"))
	assertProvenance := func(bindings []plugin.PeerProcessBinding) {
		t.Helper()
		require.Len(t, bindings, 3)
		for _, binding := range bindings {
			require.Equal(t, binding.PluginName == "operator", binding.Explicit, binding.PluginName)
		}
	}
	graph := deliveryPeersFromSettings([]*PeerSettings{settings})
	require.Len(t, graph, 1)
	assertProvenance(graph[0].Bindings)
	reactor := New(&Config{})
	reactor.peers[peerKeyFromAddrPort(address, DefaultBGPPort)] = NewPeer(settings)
	assertProvenance((&reactorAPIAdapter{r: reactor}).GetPeerProcessBindings(address))
}
