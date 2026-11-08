package server

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// insertionListener installs a counting listener on the filter insertion
// change seam, records whether each call ran with s's transaction lock held,
// and restores the seam in t.Cleanup.
func insertionListener(t *testing.T, s *Server) (calls, unlocked *atomic.Int64) {
	t.Helper()
	calls, unlocked = &atomic.Int64{}, &atomic.Int64{}
	registry.SetFilterInsertionsChanged(func() {
		calls.Add(1)
		if s.txLock.tryAcquire() {
			unlocked.Add(1)
			s.txLock.release()
		}
	})
	t.Cleanup(func() { registry.SetFilterInsertionsChanged(nil) })
	return calls, unlocked
}

// startedServer answers a test server whose startup is over.
func startedServer() *Server {
	s := sessionReadyTestServer()
	s.startupDone = make(chan struct{})
	close(s.startupDone)
	return s
}

// TestInsertionChangeIsSerializedWithTransactions checks the re-insertion
// trigger of spec-filter-wire-6-stage-filters: a change of the declared
// insertions rebuilds the peers once, never while a config transaction could
// swap the running tree under the rebuild, and never during startup.
// Method: announce with a transaction holding the lock (deferred to its
// release), with none (run at once under the lock), and before startup ends.
func TestInsertionChangeIsSerializedWithTransactions(t *testing.T) {
	s := startedServer()
	calls, unlocked := insertionListener(t, s)

	require.True(t, s.txLock.tryAcquire())
	s.announceInsertionsChanged()
	s.announceInsertionsChanged()
	assert.Equal(t, int64(0), calls.Load(), "a change met inside a transaction waits for its end")
	s.txLock.release()
	assert.Equal(t, int64(1), calls.Load(), "the holder's release announces the deferred change once")

	s.announceInsertionsChanged()
	assert.Equal(t, int64(2), calls.Load(), "with no transaction the change is announced at once")
	assert.Equal(t, int64(0), unlocked.Load(), "every rebuild runs holding the transaction lock")
	assert.True(t, s.txLock.tryAcquire(), "the announce leaves the lock free")
	s.txLock.release()

	early := sessionReadyTestServer()
	early.startupDone = make(chan struct{})
	early.announceInsertionsChanged()
	assert.Equal(t, int64(2), calls.Load(), "during startup the post-startup rebuild places the insertions")

	s.announceInsertionChange([]rpc.FilterInsertion{thirdPartyInsertion}, []rpc.FilterInsertion{thirdPartyInsertion})
	assert.Equal(t, int64(2), calls.Load(), "an unchanged declaration rebuilds nothing")
	s.announceInsertionChange([]rpc.FilterInsertion{thirdPartyInsertion}, nil)
	assert.Equal(t, int64(3), calls.Load(), "a changed declaration rebuilds the peers")
}

// TestRestartWithAChangedDeclarationRebuildsThePeers checks that a respawn
// whose Stage 1 declares other insertions than the process it replaced
// announces the change, and one declaring the same says nothing.
// Method: the first generation is given an insertion, the replacement
// declares none; a second restart declares none again.
func TestRestartWithAChangedDeclarationRebuildsThePeers(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const claimant = "insertion-restart-claimant"
	const standDown = "insertion-restart-plugin"
	registerRestartPlugins(t, claimant, standDown, "show insertion restart")

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: claimant, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: standDown, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	s.dispatcher.Registry().Freeze()
	s.signalStartupComplete()
	calls, _ := insertionListener(t, s)

	pm := s.procManager.Load()
	require.NotNil(t, pm)
	pm.GetProcess(standDown).Registration().FilterInsertions = []rpc.FilterInsertion{thirdPartyInsertion}

	require.NoError(t, s.restartPlugin(standDown))
	assert.Equal(t, int64(1), calls.Load(), "the replacement declares no insertion, so its member leaves the peers")

	require.NoError(t, s.restartPlugin(standDown))
	assert.Equal(t, int64(1), calls.Load(), "an unchanged declaration rebuilds nothing")
}

// TestReloadStoppingAnInsertingPluginRebuildsThePeers checks the defect the
// re-insertion work found: a reload that removes a plugin's config stops it
// after the peers were re-parsed with it still loaded, so its members stayed.
// The stop now announces the withdrawal, and a stop of a plugin that
// declared nothing does not.
// Method: two plugins on two config roots, one given an insertion; remove
// each root in turn.
func TestReloadStoppingAnInsertingPluginRebuildsThePeers(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	register := func(name, root string) {
		require.NoError(t, registry.Register(registry.Registration{
			Name: name, Description: "insertion stop test plugin",
			ConfigRoots: []string{root},
			CLIHandler:  func([]string) int { return 0 },
			RunEngine: func(conn net.Conn) int {
				p := sdk.NewWithConn(name, conn)
				if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
					return 1
				}
				return 0
			},
		}))
	}
	register("insertion-stop-inserter", "insertionstopa")
	register("insertion-stop-plain", "insertionstopb")

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: "insertion-stop-inserter", Internal: true, Encoder: plugin.EncodingJSON},
		{Name: "insertion-stop-plain", Internal: true, Encoder: plugin.EncodingJSON},
	}))
	s.signalStartupComplete()
	calls, unlocked := insertionListener(t, s)
	pm := s.procManager.Load()
	require.NotNil(t, pm)
	pm.GetProcess("insertion-stop-inserter").Registration().FilterInsertions = []rpc.FilterInsertion{thirdPartyInsertion}

	require.NoError(t, s.autoStopForRemovedConfigPaths([]string{"insertionstopb"}, nil))
	assert.Equal(t, int64(0), calls.Load(), "a plugin that declared nothing changes no filter list")

	require.True(t, s.txLock.tryAcquire(), "the stop runs inside a reload")
	require.NoError(t, s.autoStopForRemovedConfigPaths([]string{"insertionstopa"}, nil))
	assert.Equal(t, int64(0), calls.Load(), "the rebuild waits for the reload's running tree")
	s.txLock.release()
	assert.Equal(t, int64(1), calls.Load(), "the stopped plugin's members leave the peers")
	assert.Equal(t, int64(0), unlocked.Load())
	s.wg.Wait()
}
