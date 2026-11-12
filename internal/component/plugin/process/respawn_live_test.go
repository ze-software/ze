// Design: docs/architecture/api/process-protocol.md -- multi-process coordination and respawn
// Related: manager.go -- Respawn, StartMore
// Related: metric_hooks.go -- the metrics callbacks Respawn takes back
// Related: process.go -- StopJoinBound, WaitRuntimeCleanupContext

package process

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/metrics"
)

// TestRespawnOfALiveProcessRacesNoMetricCallback restarts a plugin while
// events are delivered to it and its stage changes.
//
// VALIDATES: Respawn takes the metrics callbacks back from a LIVE process
// (request plugin restart restarts a running plugin) through the same atomic
// pointer Deliver, the delivery loop and SetStage read (metric_hooks.go).
// PREVENTS: Respawn writing the four callback fields with no lock while
// Deliver read them under eventMu, SetStage under stageMu and noteBatch under
// none: a data race between those readers and the replacement of the set,
// which the race detector reports here. The swap is atomic, so a reader sees
// the whole set or none; one that loaded the set before the clear may still
// make one last call through it (clearMetricHooks), and this test does not
// claim otherwise.
// METHOD: run under -race. Two workers call Deliver and SetStage on the
// process in a loop, each from its own goroutine, and signal once they have
// called through the callbacks; Respawn then replaces the process while they
// still run. Each worker ends when the stop it meets ends the process.
func TestRespawnOfALiveProcessRacesNoMetricCallback(t *testing.T) {
	const name = "test-respawn-live-metrics"
	registerRunEngine(t, name, func(conn net.Conn) int {
		drainUntilClosed(conn)
		return 0
	})
	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	pm.SetMetricsRegistry(metrics.NewPrometheusRegistry())
	require.NoError(t, pm.Start())
	t.Cleanup(pm.Stop)

	previous := pm.GetProcess(name)
	require.NotNil(t, previous)
	require.NotNil(t, previous.metricHooks.Load(), "the manager wired no metrics callbacks")

	started := make(chan struct{}, 2)
	var workers sync.WaitGroup
	workers.Go(func() {
		signaled := false
		// Each delivery carries a result channel, so the failed batch (the
		// process has no connection) is answered to it rather than logged.
		for previous.Deliver(EventDelivery{Output: "event", Result: make(chan EventResult, 1)}) {
			if !signaled {
				signaled = true
				started <- struct{}{}
			}
		}
	})
	workers.Go(func() {
		done := previous.Done()
		signaled := false
		for {
			select {
			case <-done:
				return
			default:
			}
			previous.SetStage(plugin.StageInit)
			if !signaled {
				signaled = true
				started <- struct{}{}
			}
		}
	})
	<-started
	<-started

	replacement, err := pm.Respawn(name)
	require.NoError(t, err)
	require.NotSame(t, previous, replacement)
	require.Nil(t, previous.metricHooks.Load(), "the replaced process kept its metrics callbacks")
	require.NotNil(t, replacement.metricHooks.Load(), "the replacement got no metrics callbacks")
	workers.Wait()
}

// TestReaddedPluginRespawnsWithItsNewConfig removes a plugin, adds it back
// with a different config, and restarts it, twice.
//
// VALIDATES: StartMore replaces a config whose name the manager holds, so
// ConfiguredNames lists the name once and Respawn starts the config it was
// added back with, each time it is added back. A reload's removal takes the
// process out of its slot and leaves the name configured, so every re-add
// meets a held config.
// PREVENTS: a re-added plugin holding two configs, with Respawn taking the
// first match, the stale one.
func TestReaddedPluginRespawnsWithItsNewConfig(t *testing.T) {
	const name = "test-readd-new-config"
	registerRunEngine(t, name, func(conn net.Conn) int {
		drainUntilClosed(conn)
		return 0
	})
	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	require.NoError(t, pm.Start())
	t.Cleanup(pm.Stop)

	removed := pm.GetProcess(name)
	require.NotNil(t, removed)
	pm.RemoveProcess(name)
	removed.Stop()
	joinCtx, joinCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer joinCancel()
	require.NoError(t, removed.Wait(joinCtx))

	require.NoError(t, pm.StartMore([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "text"}}))
	require.Equal(t, []string{name}, pm.ConfiguredNames(), "a re-added name is listed twice")
	respawned, err := pm.Respawn(name)
	require.NoError(t, err)
	require.Equal(t, "text", respawned.Config().Encoder, "Respawn started a stale config")

	// Added back a second time, after the same removal. The running process
	// leaves its slot first, as a reload's removal takes it: StartMore over a
	// live slot would orphan it.
	pm.RemoveProcess(name)
	respawned.Stop()
	require.NoError(t, respawned.Wait(joinCtx))
	require.NoError(t, pm.StartMore([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}}))
	require.Equal(t, []string{name}, pm.ConfiguredNames(), "a re-added name is listed twice")
	respawned, err = pm.Respawn(name)
	require.NoError(t, err)
	require.Equal(t, "json", respawned.Config().Encoder, "Respawn started a stale config")
}

// TestWaitRuntimeCleanupContextAnswersBothOutcomes waits for a runtime
// cleanup that never runs, then for one that has run.
//
// VALIDATES: WaitRuntimeCleanupContext answers the context's error when the
// context ends first, and nil once the cleanup has run, even with the
// context already ended; an ended context still answers "not run" until the
// cleanup has run.
// PREVENTS: a restart waiting forever, under the config transaction lock, for
// the runtime cleanup of a process whose cleanup never runs.
func TestWaitRuntimeCleanupContextAnswersBothOutcomes(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "test-cleanup-wait"})

	expired, expiredCancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer expiredCancel()
	require.ErrorIs(t, proc.WaitRuntimeCleanupContext(expired), context.DeadlineExceeded)
	require.ErrorIs(t, proc.WaitRuntimeCleanupContext(expired), context.DeadlineExceeded,
		"an ended context answered the cleanup as run before it ran")

	// RunRuntimeCleanupOnce returns only after the cleanup ran and its done
	// channel closed, so the waits below need no synchronization.
	proc.RunRuntimeCleanupOnce(func() {})
	require.NoError(t, proc.WaitRuntimeCleanupContext(t.Context()))
	require.NoError(t, proc.WaitRuntimeCleanupContext(expired),
		"a cleanup that has run is answered as not run because the context had also ended")
}

// TestStopJoinBound reads the bound a caller waits for a stopped process.
//
// VALIDATES: StopJoinBound is the process's exit grace plus respawnJoinMargin:
// the margin alone for a plugin in the daemon, pluginExitGrace more for a
// forked plugin where a stop gives that grace (Linux).
// PREVENTS: a caller outside the package inventing its own bound for the end
// of a process, which drifts from the join Respawn performs.
func TestStopJoinBound(t *testing.T) {
	internal := NewProcess(plugin.PluginConfig{Name: "test-bound-internal", Internal: true})
	require.Equal(t, respawnJoinMargin, internal.StopJoinBound())

	forked := NewProcess(plugin.PluginConfig{Name: "test-bound-forked"})
	want := respawnJoinMargin
	if stopGivesExitGrace {
		want += pluginExitGrace
	}
	require.Equal(t, want, forked.StopJoinBound())
}
