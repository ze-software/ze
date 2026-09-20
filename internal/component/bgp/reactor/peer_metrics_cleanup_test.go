// Related: reactor_peers.go — doRemovePeer, the cleanup under test
// Related: peer_stats.go — msgTypeNames, the set it deletes by

package reactor

import (
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRemovedPeerLeavesNoMessageCounters drives every message-counter emitter
// on one peer, removes the peer, and asserts that not one series survives.
//
// VALIDATES: doRemovePeer deletes every `type` label value the emitters can
// stamp on ze_peer_messages_received_total and _sent_total.
// PREVENTS: the cleanup naming its own copy of that vocabulary. It did until
// 2026-09-20, and one member had drifted: the emitters write `refresh` for a
// ROUTE-REFRESH and the cleanup deleted `route_refresh`, a series nothing ever
// created. A removed peer therefore kept its two refresh counters forever,
// under a `peer` label an operator can recreate with different config, so the
// next peer at that address inherited a stranger's totals.
//
// The test drives the PRODUCERS rather than writing the label values itself.
// A list of expected labels here would be a third copy of the vocabulary, and
// a third copy is what the defect was.
func TestRemovedPeerLeavesNoMessageCounters(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	registry := newSpyRegistry()
	reactor := New(&Config{ConfigPath: configPath, ListenAddr: "127.0.0.1:0", Standalone: true})
	reactor.SetReloadFunc(simpleReloadFunc)
	reactor.SetMetricsRegistry(registry)
	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	addr := netip.MustParseAddr("192.0.2.77")
	settings := NewPeerSettings(addr, 65000, 65001, 0x01020304)
	require.NoError(t, reactor.AddPeer(settings))

	reactor.mu.RLock()
	peer := reactor.peers[settings.PeerKey()]
	reactor.mu.RUnlock()
	require.NotNil(t, peer, "peer must be stored under its own address:port key")
	label := peer.peerAddrLabel()

	// Every producer that stamps a `type` label, both directions.
	peer.incrUpdatesReceived()
	peer.incrUpdatesSent()
	peer.incrKeepalivesReceived()
	peer.incrKeepalivesSent()
	peer.incrEORReceived()
	peer.incrEORSent()
	peer.incrOpensReceived()
	peer.incrOpensSent()
	peer.incrNotificationReceived(0, 0)
	peer.incrNotificationSent(0, 0)
	peer.incrRefreshReceived()
	peer.incrRefreshSent()

	for _, name := range []string{"ze_peer_messages_received_total", "ze_peer_messages_sent_total"} {
		vec := registry.counterVec(name)
		require.NotNil(t, vec, "%s must be registered", name)
		require.NotEmpty(t, seriesForPeer(vec, label), "%s must hold series for the peer before removal", name)
	}

	require.NoError(t, reactor.RemovePeer(addr))

	for _, name := range []string{"ze_peer_messages_received_total", "ze_peer_messages_sent_total"} {
		vec := registry.counterVec(name)
		if left := seriesForPeer(vec, label); len(left) != 0 {
			t.Errorf("%s kept %d series for a removed peer: %v", name, len(left), left)
		}
	}
}

// seriesForPeer answers the `type` label values the vec still holds for one
// peer.
func seriesForPeer(vec *spyCounterVec, peerLabel string) []string {
	types := make([]string, 0, len(msgTypeNames))
	for _, labels := range vec.labelSets() {
		if len(labels) == 2 && labels[0] == peerLabel {
			types = append(types, labels[1])
		}
	}
	slices.Sort(types)
	return types
}
