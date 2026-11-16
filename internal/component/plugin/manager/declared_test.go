package plugin

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parent "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// startedManagerHolding answers a manager past StartAll whose record of the
// plugins started before the full parse is declared. No process exists: the
// tests below judge which configs SpawnMore still forks, which needs none.
func startedManagerHolding(t *testing.T, declared map[string]error) *Manager {
	t.Helper()

	manager := NewManager()
	require.NoError(t, manager.StartAll(context.Background(), nil, nil))
	manager.declared = declared
	return manager
}

// TestSpawnMoreTakesADeclaredPluginOnce proves a plugin started before the
// full parse is never forked a second time: the phase that names it gets no
// config left to fork for it, and the record is spent, so a later start of
// the same name (a reload, a restart) forks as it always did.
func TestSpawnMoreTakesADeclaredPluginOnce(t *testing.T) {
	manager := startedManagerHolding(t, map[string]error{"acme-monitor": nil})
	held := parent.PluginConfig{Name: "acme-monitor", Run: "acme-monitor-plugin"}
	other := parent.PluginConfig{Name: "bgp-rib", Internal: true}

	remaining, err := manager.takeDeclared([]parent.PluginConfig{held, other})

	require.NoError(t, err)
	assert.Equal(t, []parent.PluginConfig{other}, remaining, "the held plugin is not forked again")
	assert.Empty(t, manager.declared, "the record is taken once")

	remaining, err = manager.takeDeclared([]parent.PluginConfig{held})

	require.NoError(t, err)
	assert.Equal(t, []parent.PluginConfig{held}, remaining, "a later start of the same name forks")
}

// TestSpawnMoreOfOnlyHeldPluginsForksNothing proves the phase of a held
// plugin succeeds with nothing to fork: SpawnMore answers nil without a
// process manager to start anything in.
func TestSpawnMoreOfOnlyHeldPluginsForksNothing(t *testing.T) {
	manager := startedManagerHolding(t, map[string]error{"acme-monitor": nil})

	err := manager.SpawnMore([]parent.PluginConfig{{Name: "acme-monitor", Run: "acme-monitor-plugin"}})

	require.NoError(t, err)
	assert.Nil(t, manager.GetProcessManager(), "nothing was forked for a plugin that is already held")
}

// TestSpawnMoreRefusesAPluginThatDidNotDeclare proves a plugin that failed
// before the full parse is not tried a second time: the phase that names it
// is refused with the reason kept from that attempt, and forks nothing.
func TestSpawnMoreRefusesAPluginThatDidNotDeclare(t *testing.T) {
	cause := errors.New(`external plugin "silent" connected and sent no declaration within 5s, so it was stopped`)
	manager := startedManagerHolding(t, map[string]error{"silent": cause, "acme-monitor": nil})

	err := manager.SpawnMore([]parent.PluginConfig{
		{Name: "silent", Run: "silent-plugin"},
		{Name: "acme-monitor", Run: "acme-monitor-plugin"},
	})

	require.ErrorIs(t, err, cause)
	assert.Empty(t, manager.declared, "both records are spent: the phase did not run")
	assert.Nil(t, manager.GetProcessManager(), "nothing was forked")
}

// TestStartDeclaredAfterStartAllIsRefused proves the early start cannot run
// once the manager is in its ordinary phases, where SpawnMore owns the forks.
func TestStartDeclaredAfterStartAllIsRefused(t *testing.T) {
	manager := startedManagerHolding(t, nil)

	failures, err := manager.StartDeclared(context.Background(), []parent.PluginConfig{{Name: "acme-monitor", Run: "acme-monitor-plugin"}})

	require.ErrorIs(t, err, errAlreadyStarted)
	assert.Nil(t, failures)
}

// TestStartDeclaredWithNoPluginRecordsNothing proves a boot that declares no
// external plugin leaves the manager as it was: no listener, no process
// manager, no record.
func TestStartDeclaredWithNoPluginRecordsNothing(t *testing.T) {
	manager := NewManager()

	failures, err := manager.StartDeclared(context.Background(), nil)

	require.NoError(t, err)
	assert.Empty(t, failures)
	assert.Nil(t, manager.declared)
	assert.Nil(t, manager.GetProcessManager())
}

// TestStartDeclaredWithoutAuthorityForksNothing proves a manager that cannot
// build the listener a plugin connects back to answers that error and records
// nothing, so the startup phase reports the plugin as it always did.
func TestStartDeclaredWithoutAuthorityForksNothing(t *testing.T) {
	manager := NewManager()

	failures, err := manager.StartDeclared(context.Background(), []parent.PluginConfig{{Name: "acme-monitor", Run: "acme-monitor-plugin"}})

	require.Error(t, err, "no certificate authority was set, so no listener can be built")
	assert.Nil(t, failures)
	assert.Nil(t, manager.declared)
}

// TestSpawnMoreRecordsTheRefusedPluginsDown proves a phase refused for a
// plugin that sent no declaration leaves a stop record for every plugin it
// leaves down, which is what the doctor-plugin-stopped check reads: the
// plugin that did not declare carries its kept reason, and the held plugin
// stopped with it carries the phase refusal. The manager is given the process
// manager StartDeclared would have created; nothing is forked.
func TestSpawnMoreRecordsTheRefusedPluginsDown(t *testing.T) {
	cause := errors.New(`external plugin "silent" connected and sent no declaration within 5s, so it was stopped`)
	manager := startedManagerHolding(t, map[string]error{"silent": cause, "acme-monitor": nil})
	pm := process.NewProcessManager(nil)
	manager.procManager = pm
	// A stop record raises a warning on the process-wide report bus, and a
	// removal clears both.
	t.Cleanup(func() {
		pm.RemoveProcess("silent")
		pm.RemoveProcess("acme-monitor")
	})

	err := manager.SpawnMore([]parent.PluginConfig{
		{Name: "silent", Run: "silent-plugin"},
		{Name: "acme-monitor", Run: "acme-monitor-plugin"},
	})

	require.ErrorIs(t, err, cause)
	stop, down := pm.Stopped("silent")
	require.True(t, down, "the plugin that did not declare is recorded down")
	assert.Equal(t, process.StopHandshakeFailed, stop.Cause)
	assert.Equal(t, cause.Error(), stop.Detail, "the record carries the reason kept from the attempt")

	stop, down = pm.Stopped("acme-monitor")
	require.True(t, down, "the held plugin stopped with the refused phase is recorded down")
	assert.Equal(t, process.StopHandshakeFailed, stop.Cause)
	assert.Equal(t, declaredPhaseRefusedDetail, stop.Detail)
}

// TestFailDeclaredRaisesAStopThatOutlivedItsBound proves a held plugin that
// has not ended when its stop bound expires is raised on the report bus as
// doctor-plugin-stop-unfinished, subject the plugin, before the manager
// removes it. The held plugin runs in the daemon so the test forks nothing:
// its engine ignores its closed connection until the test releases it, and
// the bound of a plugin in the daemon (1s) is the time the test takes.
func TestFailDeclaredRaisesAStopThatOutlivedItsBound(t *testing.T) {
	const name = "test-held-stop-unfinished"
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	stuck := make(chan struct{})
	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "test plugin whose engine outlives its stop",
		RunEngine: func(net.Conn) int {
			<-stuck
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	pm := process.NewProcessManager(nil)
	failures := pm.StartEach(t.Context(), []parent.PluginConfig{{Name: name, Internal: true, Encoder: parent.EncodingJSON}})
	require.Empty(t, failures)
	proc := pm.GetProcess(name)
	require.NotNil(t, proc)
	// The engine is released and joined before the registry is restored.
	t.Cleanup(func() {
		close(stuck)
		joinCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, proc.Wait(joinCtx))
	})
	manager := NewManager()
	manager.procManager = pm
	manager.declared = map[string]error{name: nil}

	manager.FailDeclared(context.Background(), name, errors.New("sent no declaration"))

	var events []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorPluginStopUnfinished {
			events = append(events, issue)
		}
	}
	require.Len(t, events, 1, "the expired stop bound is raised once")
	assert.Equal(t, name, events[0].Subject)
	assert.Contains(t, events[0].Message, "had not returned 1s after ze stopped it")
	assert.Nil(t, pm.GetProcess(name), "the manager removed the process after the raise")
}
