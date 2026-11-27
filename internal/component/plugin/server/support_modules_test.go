// VALIDATES: Daemon-backed support modules (event-ring, plugins, reload-history)
// register via internal/core/supportreg on Server.StartWithContext, produce
// bounded JSON-serializable snapshots, honor SinceTime, and unregister cleanly
// on Server.Stop so repeated start/stop cycles never panic with duplicates.
// PREVENTS: plugin/server importing support, duplicate supportreg panics across
// server restarts, or unbounded/non-JSON payloads in support archives.

package server

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/supportreg"
)

func TestSupportModulesLifecycleAndPayloads(t *testing.T) {
	s1, _ := newLifecycleStartupServer(t)
	require.NoError(t, s1.StartWithContext(t.Context()))

	// All 3 daemon modules are registered on StartWithContext.
	names := supportreg.Names()
	assert.True(t, slices.Contains(names, supportModuleEventRing), "event-ring must be registered")
	assert.True(t, slices.Contains(names, supportModulePlugins), "plugins must be registered")
	assert.True(t, slices.Contains(names, supportModuleReloadHistory), "reload-history must be registered")

	snap := supportreg.Snapshot()
	require.Contains(t, snap, supportModuleEventRing)
	require.Contains(t, snap, supportModulePlugins)
	require.Contains(t, snap, supportModuleReloadHistory)

	// 1. event-ring: verify cap at 200 and SinceTime filtering.
	for i := range supportEventRingMaxEntries + 25 {
		s1.eventRing.AppendWithMeta("bgp", "update-"+strconv.Itoa(i), "engine", "192.0.2.1", "receive")
	}
	evPayload, err := snap[supportModuleEventRing](&supportreg.CollectOptions{})
	require.NoError(t, err)
	evBytes, err := json.Marshal(evPayload)
	require.NoError(t, err)
	var evDecoded supportEventRingPayload
	require.NoError(t, json.Unmarshal(evBytes, &evDecoded))
	assert.True(t, evDecoded.Available)
	assert.Equal(t, supportEventRingMaxEntries, evDecoded.Count)
	assert.Equal(t, supportEventRingMaxEntries+25, evDecoded.TotalCount)
	assert.True(t, evDecoded.Truncated)
	assert.Len(t, evDecoded.Entries, supportEventRingMaxEntries)

	futureOpts := &supportreg.CollectOptions{SinceTime: time.Now().Add(time.Hour)}
	evFuture, err := snap[supportModuleEventRing](futureOpts)
	require.NoError(t, err)
	evFutureBytes, err := json.Marshal(evFuture)
	require.NoError(t, err)
	var evFutureDecoded supportEventRingPayload
	require.NoError(t, json.Unmarshal(evFutureBytes, &evFutureDecoded))
	assert.True(t, evFutureDecoded.Available)
	assert.Equal(t, 0, evFutureDecoded.Count)
	assert.NotNil(t, evFutureDecoded.Entries)
	assert.Empty(t, evFutureDecoded.Entries)

	// 2. plugins: verify internal, external with PID, and stopped/errored plugins.
	pm := process.NewProcessManager([]plugin.PluginConfig{
		{Name: "alpha-internal", Internal: true},
		{Name: "beta-external", Internal: false},
	})
	intProc := process.NewProcess(plugin.PluginConfig{Name: "alpha-internal", Internal: true})
	markProcessRunning(t, intProc)
	intProc.SetStage(plugin.StageRunning)
	require.NoError(t, pm.AddProcess("alpha-internal", intProc))

	extProc := process.NewProcess(plugin.PluginConfig{Name: "beta-external", Internal: false})
	markProcessRunning(t, extProc)
	extProc.SetStage(plugin.StageRunning)
	setProcessExternalForTest(t, extProc, 4242)
	extProc.SetStartupError(errors.New("transient init warn"))
	require.NoError(t, pm.AddProcess("beta-external", extProc))
	s1.procManager.Store(pm)

	plugPayload, err := snap[supportModulePlugins](&supportreg.CollectOptions{})
	require.NoError(t, err)
	plugBytes, err := json.Marshal(plugPayload)
	require.NoError(t, err)
	var plugDecoded supportPluginsPayload
	require.NoError(t, json.Unmarshal(plugBytes, &plugDecoded))
	assert.True(t, plugDecoded.Available)
	assert.Equal(t, 2, plugDecoded.Count)
	assert.Equal(t, 2, plugDecoded.TotalCount)
	assert.Equal(t, 2, plugDecoded.Running)
	require.Len(t, plugDecoded.Plugins, 2)

	assert.Equal(t, "alpha-internal", plugDecoded.Plugins[0].Name)
	assert.Equal(t, processModeInternal, plugDecoded.Plugins[0].Mode)
	assert.True(t, plugDecoded.Plugins[0].Running)
	assert.Equal(t, "healthy", plugDecoded.Plugins[0].Health)
	assert.Equal(t, 0, plugDecoded.Plugins[0].PID)

	assert.Equal(t, "beta-external", plugDecoded.Plugins[1].Name)
	assert.Equal(t, processModeExternal, plugDecoded.Plugins[1].Mode)
	assert.True(t, plugDecoded.Plugins[1].Running)
	assert.Equal(t, 4242, plugDecoded.Plugins[1].PID)
	assert.Equal(t, "degraded", plugDecoded.Plugins[1].Health)
	assert.Equal(t, "transient init warn", plugDecoded.Plugins[1].LastError)

	// 3. reload-history: verify recorded reloads and SinceTime filtering.
	oldTime := time.Now().UTC().Add(-2 * time.Hour)
	recentTime := time.Now().UTC()
	s1.MarkReloadProcessedWithRecord(ReloadRecord{
		OpID:       "op-old",
		Trigger:    "sighup",
		Outcome:    ReloadOutcomeApplied,
		StartedAt:  oldTime,
		FinishedAt: oldTime,
		DurationMS: 3,
	})
	s1.MarkReloadProcessedWithRecord(ReloadRecord{
		OpID:       "op-recent",
		Trigger:    "commit",
		Outcome:    ReloadOutcomeFailed,
		StartedAt:  recentTime,
		FinishedAt: recentTime,
		DurationMS: 9,
		Error:      "verify failed",
	})

	relPayload, err := snap[supportModuleReloadHistory](&supportreg.CollectOptions{})
	require.NoError(t, err)
	relBytes, err := json.Marshal(relPayload)
	require.NoError(t, err)
	var relDecoded supportReloadHistoryPayload
	require.NoError(t, json.Unmarshal(relBytes, &relDecoded))
	assert.True(t, relDecoded.Available)
	assert.Equal(t, uint64(2), relDecoded.Generation)
	assert.Equal(t, ReloadOutcomeFailed, relDecoded.LastOutcome)
	assert.Equal(t, "op-recent", relDecoded.LastOperationID)
	assert.Equal(t, 2, relDecoded.Count)
	require.Len(t, relDecoded.Entries, 2)
	assert.Equal(t, "op-recent", relDecoded.Entries[0].OpID)

	relSincePayload, err := snap[supportModuleReloadHistory](&supportreg.CollectOptions{
		SinceTime: time.Now().UTC().Add(-30 * time.Minute),
	})
	require.NoError(t, err)
	relSinceBytes, err := json.Marshal(relSincePayload)
	require.NoError(t, err)
	var relSinceDecoded supportReloadHistoryPayload
	require.NoError(t, json.Unmarshal(relSinceBytes, &relSinceDecoded))
	assert.Equal(t, 1, relSinceDecoded.Count)
	require.Len(t, relSinceDecoded.Entries, 1)
	assert.Equal(t, "op-recent", relSinceDecoded.Entries[0].OpID)

	// Stop unregisters all 3 modules cleanly (and is idempotent).
	s1.Stop()
	s1.Stop()
	afterStop := supportreg.Names()
	assert.False(t, slices.Contains(afterStop, supportModuleEventRing), "event-ring must be unregistered after Stop")
	assert.False(t, slices.Contains(afterStop, supportModulePlugins), "plugins must be unregistered after Stop")
	assert.False(t, slices.Contains(afterStop, supportModuleReloadHistory), "reload-history must be unregistered after Stop")

	// Second server start/stop cycle succeeds without duplicate-name panic.
	s2, _ := newLifecycleStartupServer(t)
	require.NoError(t, s2.StartWithContext(t.Context()))
	assert.True(t, slices.Contains(supportreg.Names(), supportModuleEventRing))
	assert.True(t, slices.Contains(supportreg.Names(), supportModulePlugins))
	assert.True(t, slices.Contains(supportreg.Names(), supportModuleReloadHistory))
	s2.Stop()
	assert.False(t, slices.Contains(supportreg.Names(), supportModuleEventRing))
	assert.False(t, slices.Contains(supportreg.Names(), supportModulePlugins))
	assert.False(t, slices.Contains(supportreg.Names(), supportModuleReloadHistory))
}

func TestSupportModulesUnavailableFallbacks(t *testing.T) {
	t.Parallel()

	s := &Server{}

	ev, err := s.collectSupportEventRing(nil)
	require.NoError(t, err)
	evMap, ok := ev.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, evMap["available"])
	assert.Equal(t, "event bus not active", evMap["reason"])

	plugs, err := s.collectSupportPlugins(nil)
	require.NoError(t, err)
	plugMap, ok := plugs.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, plugMap["available"])
	assert.NotEmpty(t, plugMap["reason"])
}
