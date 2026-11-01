// Design: docs/architecture/api/process-protocol.md -- Filter Insertion (third-party insertions apply, rebuild before peers start)
// Related: reload_tree.go -- RebuildRunningPeers

package reactor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRebuildRunningPeersReplacesHeldPeer checks that RebuildRunningPeers
// rebuilds the running configuration's peers and replaces a peer whose
// rebuilt settings differ, while the start barrier still holds it.
//
// Method: a borrow-mode reactor (the daemon's mode) builds peer1 from its
// running tree with a 90 s hold time, and the peer is held (StartPeers has
// not run). The tree reload then answers 30 s for the same tree, as the
// daemon's does once a third party's Stage 1 insertion places a member the
// first build lacked (spec-filter-wire-6-stage-filters A-12). After
// RebuildRunningPeers the reactor's peer MUST carry the rebuilt settings and
// MUST still be held, so the replaced peer opens no session before the
// barrier lifts.
//
// VALIDATES: Reactor.RebuildRunningPeers reconciles the unchanged running
// tree against the reload pipeline's current answer.
// PREVENTS: a rebuild that is a no-op, leaving every peer without the members
// a third-party plugin declared at Stage 1.
func TestRebuildRunningPeersReplacesHeldPeer(t *testing.T) {
	r := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false: borrow mode
	srv := newBorrowedPluginServer(t, r)
	r.SetPluginServer(srv)

	var stageOneDeclared atomic.Bool
	base := testTreeReload(t)
	r.SetTreeReloadFunc(func(tree map[string]any) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error) {
		peers, globals, groups, err := base(tree)
		if err != nil || !stageOneDeclared.Load() {
			return peers, globals, groups, err
		}
		for _, ps := range peers {
			ps.ReceiveHoldTime = 30 * time.Second
		}
		return peers, globals, groups, nil
	})

	require.NoError(t, r.StartWithContext(context.Background()))
	t.Cleanup(func() { stopAndWait(t, r) })

	running := configRoot(makeBGPTree(map[string]testPeer{
		"peer1": {remoteIP: "10.0.0.1", remoteAS: "65002", localAS: "65001", holdTime: "90"},
	}))
	adapter := &reactorAPIAdapter{r: r}
	require.NoError(t, adapter.ApplyConfigDiff(running))
	adapter.SetConfigTree(running)

	before := peerAt(t, r, "10.0.0.1")
	require.Equal(t, 90*time.Second, before.Settings().ReceiveHoldTime)
	require.False(t, peerStarted(before), "the peer must be held by the start barrier before the rebuild")

	stageOneDeclared.Store(true)
	require.NoError(t, r.RebuildRunningPeers())

	after := peerAt(t, r, "10.0.0.1")
	require.Equal(t, 30*time.Second, after.Settings().ReceiveHoldTime,
		"RebuildRunningPeers did not replace the peer whose rebuilt settings differ")
	require.False(t, peerStarted(after), "a peer replaced before StartPeers must stay held by the barrier")
	require.False(t, barrierLifted(r), "the rebuild must not lift the start barrier")
}
