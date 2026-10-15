package server

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// registerCountedRestartPlugin registers a plugin that completes the startup
// handshake on each run up to okRuns, and exits at once on every later run,
// which fails the handshake of that run. okRuns of zero means every run
// completes it.
func registerCountedRestartPlugin(t *testing.T, name string, okRuns int64) {
	t.Helper()
	runs := &atomic.Int64{}
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "restart test plugin that counts its runs",
		RunEngine: func(conn net.Conn) int {
			run := runs.Add(1)
			if okRuns > 0 && run > okRuns {
				return 1
			}
			p := sdk.NewWithConn(name, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
}

// TestRestartPluginSaysWhenTheRespawnWasRefused verifies that every respawn
// refusal ProcessManager.Respawn returns before it stops the old process
// reaches the orchestrator as transaction.ErrRestartRefused, and that the old
// process is then still the one running.
//
// VALIDATES: review round 4 ISSUE-1. The Rollback line says "it still runs
// without part of its config" for a refused restart, and "it is not running"
// only for a restart that stopped the old process and failed after that.
// PREVENTS: a reload answer that tells the operator a plugin is down when the
// respawn limit refused its restart and the old process keeps serving.
func TestRestartPluginSaysWhenTheRespawnWasRefused(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "restart-refused"
	registerCountedRestartPlugin(t, name, 0)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	pm := s.procManager.Load()
	require.NotNil(t, pm)

	// refusedLeavesItRunning restarts name, expects the refusal want, and
	// checks the process that ran before the call still runs.
	refusedLeavesItRunning := func(want error) {
		t.Helper()
		before := pm.GetProcess(name)
		require.NotNil(t, before)
		err := s.restartPlugin(name)
		require.ErrorIs(t, err, want)
		require.ErrorIs(t, err, transaction.ErrRestartRefused)
		assert.Same(t, before, pm.GetProcess(name), "a refused restart must leave the old process in place")
		assert.True(t, before.Running(), "a refused restart must leave the old process running")
	}

	err := s.restartPlugin("restart-no-such-plugin")
	require.ErrorIs(t, err, process.ErrProcessNotFound)
	require.ErrorIs(t, err, transaction.ErrRestartRefused)

	for range process.RespawnLimit {
		require.NoError(t, s.restartPlugin(name))
	}
	refusedLeavesItRunning(process.ErrRespawnLimitExceeded)
	refusedLeavesItRunning(process.ErrProcessDisabled)

	pm.RefuseRespawns()
	refusedLeavesItRunning(process.ErrDaemonStopping)
}

// TestRestartPluginFailedHandshakeIsNotARefusal verifies that a restart whose
// respawn stopped the old process and whose replacement then failed the
// handshake is not reported as a refusal: nothing of the plugin runs.
//
// VALIDATES: review round 4 ISSUE-1, the other class. The Rollback line keeps
// "so it is not running" for this failure.
// PREVENTS: ErrRestartRefused wrapped around every restart error, which would
// tell the operator a stopped plugin still runs.
func TestRestartPluginFailedHandshakeIsNotARefusal(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "restart-handshake-fails"
	registerCountedRestartPlugin(t, name, 1)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	err := s.restartPlugin(name)
	require.Error(t, err)
	assert.NotErrorIs(t, err, transaction.ErrRestartRefused)
}
