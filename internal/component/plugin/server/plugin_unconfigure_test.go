package server

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// removalSettleBound is how long a test waits for a refused removal's
// recovery to settle. The recovery waits for nothing the test holds, so it
// settles in milliseconds; the bound only turns a hang into a failure.
const removalSettleBound = 10 * time.Second

// startRemovableTarget starts one internal plugin that owns the config root
// root, through the real startup phase, and answers the server and the
// phase's error. setup gives each process the plugin starts its callbacks.
// The test's cleanups restore the registry and wait for every goroutine the
// server started, after its teardown.
func startRemovableTarget(t *testing.T, name, root string, setup func(p *sdk.Plugin)) (*Server, error) {
	t.Helper()
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "plugin a reload removes",
		ConfigRoots: []string{root},
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			setup(p)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	// Registered before the server's own cleanup, so it runs after it.
	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
	})
	s, _ = newLifecycleStartupServer(t)
	s.SetShutdownFunc(func() {})
	err := s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}})
	s.dispatcher.Registry().Freeze()
	return s, err
}

// removeRootByReload runs the removal a reload runs for a config root that
// left the config: collect its owners, then stop them.
func removeRootByReload(s *Server, root string) error {
	return s.stopCollectedProcesses(s.collectProcessesForRemovedConfigPaths([]string{root}, nil))
}

// TestRefusedRemovalKeepsPluginConfigured removes a plugin that refuses its
// removal.
//
// Goal: a removal the plugin refused leaves it configured and offered, and
// once the removal's recovery has settled the restart command restarts it.
// Method: the plugin's first Bye answers an error, so stopCollectedProcesses
// fails on it; the test waits for the recovery to leave s.removals and for the
// transaction lock it took to be free, then restarts through the handler.
// PREVENTS: a refused removal unconfiguring a plugin that still runs and
// still serves, which hides it from the doctor and the restart command.
func TestRefusedRemovalKeepsPluginConfigured(t *testing.T) {
	const name, root = "unconfigure-refused", "unconfigure-refused-root"
	var refused atomic.Bool
	s, err := startRemovableTarget(t, name, root, func(p *sdk.Plugin) {
		p.OnBye(func(string) error {
			if refused.CompareAndSwap(false, true) {
				return errors.New("the test plugin refuses its removal")
			}
			return nil
		})
	})
	require.NoError(t, err)
	pm := s.procManager.Load()
	before := pm.GetProcess(name)

	require.Error(t, removeRootByReload(s, root), "a refused removal was answered as done")

	require.Contains(t, pm.ConfiguredNames(), name, "the refused removal unconfigured the plugin")
	offered := false
	for _, candidate := range completePluginRestart(s) {
		offered = offered || candidate.Value == name
	}
	require.True(t, offered, "completion does not offer the plugin whose removal was refused")
	snapshot, err := s.doctorSnapshot()
	require.NoError(t, err)
	listed := false
	for i := range snapshot.Plugins {
		listed = listed || (snapshot.Plugins[i].Name == name && snapshot.Plugins[i].Configured)
	}
	require.True(t, listed, "the doctor snapshot does not list the plugin as configured")

	require.Eventually(t, func() bool { return !s.removalPendingFor(name) }, removalSettleBound, 10*time.Millisecond,
		"the refused removal's recovery never settled")
	// The recovery forgets the removal before it gives the lock back.
	lockCtx, cancel := context.WithTimeout(t.Context(), removalSettleBound)
	defer cancel()
	require.NoError(t, s.txLock.acquire(lockCtx))
	s.txLock.release()

	resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args("name", name))
	if resp.Status != plugin.StatusDone {
		t.Fatalf("the plugin whose removal was refused cannot be restarted: %+v", resp)
	}
	if after := pm.GetProcess(name); after == nil || after == before || !after.Running() {
		t.Fatalf("the restart did not replace the process: %p, was %p", after, before)
	}
}
