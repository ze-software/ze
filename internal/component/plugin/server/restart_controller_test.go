// Design: docs/architecture/api/process-protocol.md -- exact replacement ownership.

package server

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The failure-policy controller owns its replacement through Configure and
// runtime admission. An operator cannot replace it mid-handshake, leaving the
// first controller's rollback to unregister the newer generation's commands.
func TestRestartControllerRetainsAutomaticReplacement(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "restart-controller-owner"
	const commandName = "show restart controller"
	crash, configuring, release, ready := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var starts atomic.Int64
	var releaseOnce, crashOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	crashInitial := func() { crashOnce.Do(func() { close(crash) }) }
	s, _ := newLifecycleStartupServer(t)
	t.Cleanup(func() {
		unblock()
		crashInitial()
		s.cancel()
		if pm := s.procManager.Load(); pm != nil {
			pm.Stop()
		}
		s.wg.Wait()
	})
	registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
		generation := starts.Add(1)
		p := sdk.NewWithConn(name, conn)
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()
		if generation == 1 {
			go crashOnSignal(crash, cancel)
		}
		p.OnConfigure(func([]sdk.ConfigSection) error {
			if generation == 2 {
				close(configuring)
				select {
				case <-release:
				case <-s.ctx.Done():
					return s.ctx.Err()
				}
			}
			return nil
		})
		p.OnAllPluginsReady(func() error {
			if generation == 2 {
				close(ready)
			}
			return nil
		})
		p.OnExecuteCommand(func(_, _ string, _ []string, _ string) (string, any, error) {
			return "done", map[string]any{"generation": generation}, nil
		})
		if err := p.Run(ctx, &sdk.Registration{FailurePolicy: rpc.FailureRestart, Commands: []sdk.CommandDecl{{Name: commandName}}}); err != nil {
			return 1
		}
		return 0
	})
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}))
	s.signalStartupComplete()
	pm := s.procManager.Load()
	initial := pm.GetProcess(name)
	require.NotNil(t, initial)
	crashInitial()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	select {
	case <-configuring:
	case <-ctx.Done():
		t.Fatal("automatic replacement did not reach its real Configure callback")
	}
	replacement := pm.GetProcess(name)
	require.NotNil(t, replacement)
	require.NotSame(t, initial, replacement)
	s.restartControllers.mu.Lock()
	controller := s.restartControllers.active[name]
	s.restartControllers.mu.Unlock()
	require.NotNil(t, controller, "automatic replacement has no active controller")
	response, err := handlePluginRestart(&CommandContext{Server: s}, commandtest.Args("name", name))
	require.NoError(t, err)
	if response.Status == plugin.StatusDone {
		t.Error("operator restarted the automatic controller's pending replacement")
	}
	if current := pm.GetProcess(name); current != replacement {
		t.Errorf("overlapping restart changed the active replacement: got %p, want %p", current, replacement)
	}
	if starts.Load() != 2 {
		t.Errorf("overlapping restart spawned %d generations, want 2", starts.Load())
	}
	unblock()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("admitted automatic replacement could not complete startup")
	}
	require.Eventually(t, func() bool { return replacement.Stage() == plugin.StageRunning }, 5*time.Second, time.Millisecond,
		"automatic replacement did not enter runtime")
	command := s.dispatcher.Registry().Lookup(commandName)
	require.NotNil(t, command, "a stale controller removed the admitted replacement's command")
	require.Same(t, replacement, command.Process)
	require.Same(t, replacement, pm.GetProcess(name))
	requireTxLockFree(t, s)
	// A completed controller releases its reservation: a later healthy operator
	// restart still reaches runtime and owns the same command.
	select {
	case <-controller.done:
	case <-ctx.Done():
		t.Fatal("automatic controller did not release its completed replacement")
	}
	response, err = handlePluginRestart(&CommandContext{Server: s}, commandtest.Args("name", name))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, response.Status, response.Error)
	later := pm.GetProcess(name)
	require.NotNil(t, later)
	require.NotSame(t, replacement, later)
	require.Equal(t, plugin.StageRunning, later.Stage())
	command = s.dispatcher.Registry().Lookup(commandName)
	require.NotNil(t, command)
	require.Same(t, later, command.Process)
}

// A failure controller can pass its first manager check before another
// controller finishes a replacement. Resuming at the real restart boundary
// must compare the outgoing process again before replacing anything by name.
func TestRestartControllerIgnoresSupersededFailure(t *testing.T) {
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
	const name = "restart-controller-stale"
	const commandName = "show restart stale"
	var starts atomic.Int64
	registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
		starts.Add(1)
		p := sdk.NewWithConn(name, conn)
		if err := p.Run(context.Background(), &sdk.Registration{FailurePolicy: rpc.FailureRestart, Commands: []sdk.CommandDecl{{Name: commandName}}}); err != nil {
			return 1
		}
		return 0
	})
	s, _ := newLifecycleStartupServer(t)
	t.Cleanup(func() {
		s.cancel()
		if pm := s.procManager.Load(); pm != nil {
			pm.Stop()
		}
		s.wg.Wait()
	})
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}))
	s.dispatcher.Registry().Freeze()
	pm := s.procManager.Load()
	outgoing := pm.GetProcess(name)
	require.NotNil(t, outgoing)
	require.NoError(t, s.restartPlugin(name))
	replacement := pm.GetProcess(name)
	require.NotNil(t, replacement)
	require.NotSame(t, outgoing, replacement)
	require.Equal(t, plugin.StageRunning, replacement.Stage())
	// Resume a controller that captured outgoing before the completed restart.
	require.NoError(t, s.restartPluginAfterExit(outgoing))
	if current := pm.GetProcess(name); current != replacement {
		t.Fatalf("stale failure controller replaced the admitted current generation: got %p, want %p", current, replacement)
	}
	require.Equal(t, int64(2), starts.Load(), "stale failure controller spawned another generation")
	command := s.dispatcher.Registry().Lookup(commandName)
	require.NotNil(t, command)
	require.Same(t, replacement, command.Process)
}
