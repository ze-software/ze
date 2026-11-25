package server

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// restartExclusiveRole is the claim token the restart tests move between two
// plugins. Its spelling is arbitrary; the engine treats a claim as opaque.
const restartExclusiveRole = "restart-test-peer-up-replay"

// restartProbe counts what a restarted plugin must go through again.
type restartProbe struct {
	configures atomic.Int64 // Stage 2 configure callbacks received
	claimSeen  atomic.Int64 // configure callbacks that saw the role claimed
	commands   atomic.Int64 // commands executed
}

// registerRestartPlugins registers a claimant that declares the exclusive role
// and a plugin that stands down for it, and returns the probe the stand-down
// plugin writes on every handshake it completes.
func registerRestartPlugins(t *testing.T, claimant, standDown, commandName string) *restartProbe {
	t.Helper()
	probe := &restartProbe{}

	require.NoError(t, registry.Register(registry.Registration{
		Name:        claimant,
		Description: "restart test claimant",
		Claims:      []string{restartExclusiveRole},
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(claimant, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	require.NoError(t, registry.Register(registry.Registration{
		Name:        standDown,
		Description: "restart test stand-down plugin",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(standDown, conn)
			p.OnConfigure(func([]sdk.ConfigSection) error {
				probe.configures.Add(1)
				if p.ClaimActive(restartExclusiveRole) {
					probe.claimSeen.Add(1)
				}
				return nil
			})
			p.OnExecuteCommand(func(_, _ string, _ []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
				probe.commands.Add(1)
				return "done", map[string]any{"served": true}, nil
			})
			err := p.Run(context.Background(), &sdk.Registration{
				Commands: []sdk.CommandDecl{{Name: commandName}},
			})
			if err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	return probe
}

// TestRestartPluginReRunsTheStartupHandshake verifies that restarting a plugin
// brings the replacement process through the 5-stage handshake, so it is
// configured, is told which exclusive roles other plugins hold, and has its
// commands registered against itself.
//
// VALIDATES: AC-5 -- a plugin that stood its default behavior down for another
// plugin's claim is told about that claim again after a restart, before it can
// receive any event.
// PREVENTS: ProcessManager.Respawn spawning a replacement the engine never
// speaks to. Before this, Respawn called StartWithContext and nothing else, so
// the replacement held no registration, no delivered config, no subscriptions,
// no commands and no claim set: it resumed its own default behavior, which is
// the duplicate announce the claim exists to prevent.
func TestRestartPluginReRunsTheStartupHandshake(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const claimant = "restart-claimant"
	const standDown = "restart-stand-down"
	const commandName = "show restart standdown"

	probe := registerRestartPlugins(t, claimant, standDown, commandName)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: claimant, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: standDown, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	require.Equal(t, int64(1), probe.configures.Load(), "startup must configure the stand-down plugin once")
	require.Equal(t, int64(1), probe.claimSeen.Load(), "startup must tell it the role is claimed")

	// Startup is over: the command registry is frozen, exactly as
	// signalStartupComplete leaves it before any reload can run.
	s.dispatcher.Registry().Freeze()

	pm := s.procManager.Load()
	require.NotNil(t, pm)
	before := pm.GetProcess(standDown)
	require.NotNil(t, before)

	require.NoError(t, s.restartPlugin(standDown))

	after := pm.GetProcess(standDown)
	require.NotNil(t, after)
	assert.NotSame(t, before, after, "restart must replace the process")

	assert.Equal(t, int64(2), probe.configures.Load(),
		"the replacement must be configured again")
	assert.Equal(t, int64(2), probe.claimSeen.Load(),
		"the replacement must be told the role is still claimed")

	cmd := s.dispatcher.Registry().Lookup(commandName)
	require.NotNil(t, cmd, "the replacement's command must be resolvable after the restart")
	assert.Same(t, after, cmd.Process, "the command must resolve to the replacement process")

	assert.NotNil(t, s.registry.LookupCommand(commandName),
		"the plugin registry row must be rebuilt for the replacement")
}

// TestRestartPluginRestartsAPluginThatDeclaredNothing verifies that a rollback
// restart is carried out for a plugin that made no declaration about its own
// failure.
//
// VALIDATES: AC-11 -- the config-reload rollback restart succeeds.
// PREVENTS: the state this spec found. ProcessManager.Respawn refused every
// plugin whose PluginConfig.Respawn was false, and no producer ever set that
// field, so the rollback path could not restart anything: an ack reporting a
// BROKEN plugin was answered with "respawn not enabled for plugin" every time.
//
// A rollback restart is an ORDER, not a policy question. The orchestrator has
// already been told the plugin is broken, and it is repairing a config
// transaction rather than reacting to a failure, so it does not consult the
// plugin's failure policy.
func TestRestartPluginRestartsAPluginThatDeclaredNothing(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const claimant = "restart-noenable-claimant"
	const standDown = "restart-noenable-stand-down"
	const commandName = "show restart noenable"

	probe := registerRestartPlugins(t, claimant, standDown, commandName)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: claimant, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: standDown, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	s.dispatcher.Registry().Freeze()

	pm := s.procManager.Load()
	require.NotNil(t, pm)
	before := pm.GetProcess(standDown)
	require.NotNil(t, before)

	require.NoError(t, s.restartPlugin(standDown))

	after := pm.GetProcess(standDown)
	require.NotNil(t, after)
	assert.NotSame(t, before, after, "the rollback restart must replace the process")
	assert.Equal(t, int64(2), probe.configures.Load(), "the replacement must be configured again")

	cmd := s.dispatcher.Registry().Lookup(commandName)
	require.NotNil(t, cmd, "the replacement's command must be resolvable")
	assert.Same(t, after, cmd.Process, "the command must resolve to the replacement process")
}

// TestRestartBeforeStartupCompletesLeavesTheFanOutToDeliverOnce verifies that a
// plugin restarted while startup phases are still running receives the
// post-startup callback exactly once, from the fan-out that owes it.
//
// VALIDATES: AC-13 -- a restarted plugin runs its OnAllPluginsReady handler, and
// runs it once. The handler is where a plugin takes an exclusive role over from
// another one, and those handlers are written for a single call
// (sendPostStartupToNames, poststartup.go).
// PREVENTS: a second delivery. A plugin's own exit reaches restartHandshake
// through applyFailurePolicy, whose supervisors start at the end of each
// runPluginPhase, so a plugin that exits between two phases is restarted BEFORE
// signalStartupComplete. Delivering there and again from sendPostStartupToAll
// runs the handler twice, and the first of the two runs before the registries
// are frozen, which is the state the callback's contract promises the handler.
func TestRestartBeforeStartupCompletesLeavesTheFanOutToDeliverOnce(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "restart-early-ready"
	ready := &atomic.Int64{}

	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "restart test plugin that counts its post-startup callbacks",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			p.OnAllPluginsReady(func() error {
				ready.Add(1)
				return nil
			})
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	}))

	// No signalStartupComplete yet: this is the window a later phase would still
	// be running in.
	require.NoError(t, s.restartPlugin(name))
	require.Never(t, func() bool { return ready.Load() > 0 }, failureSettle, 10*time.Millisecond,
		"a restart before startup completed must leave the callback to the fan-out")

	s.signalStartupComplete()
	require.Eventually(t, func() bool { return ready.Load() == 1 }, failureWait, 10*time.Millisecond,
		"the fan-out must deliver the callback to the replacement")
	assert.Never(t, func() bool { return ready.Load() > 1 }, failureSettle, 10*time.Millisecond,
		"the replacement must not run its OnAllPluginsReady handler twice")
}

// TestUnbarrieredHandshakeDoesNotClobberTierCoordinator verifies that running
// an unbarriered startup handshake concurrently with an active tier barrier
// neither nils s.coordinator nor disturbs the tier's StartupCoordinator.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-4, AC-9 --
// runUnbarrieredStartupHandshake passes coord=nil explicitly instead of
// mutating s.coordinator.
// PREVENTS: an ad-hoc session or restart nil-and-restoring s.coordinator while
// a startup tier barrier is active, which let tier plugins bypass their barrier.
func TestUnbarrieredHandshakeDoesNotClobberTierCoordinator(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	s, _ := newLifecycleStartupServer(t)
	ctx, cancel := context.WithTimeout(t.Context(), failureWait)
	var wg sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	tierCoord := plugin.NewStartupCoordinator(2)
	s.coordinatorMu.Lock()
	s.coordinator = tierCoord
	s.coordinatorMu.Unlock()

	tier0 := process.NewProcess(plugin.PluginConfig{Name: "tier-member-0", Internal: true})
	tier0.SetIndex(0)
	tier0.SetStage(plugin.StageRegistration)
	tier0.SetRegistration(&plugin.PluginRegistration{})

	tier1 := process.NewProcess(plugin.PluginConfig{Name: "tier-member-1", Internal: true})
	tier1.SetIndex(1)
	tier1.SetStage(plugin.StageRegistration)
	tier1.SetRegistration(&plugin.PluginRegistration{})

	entered := make(chan struct{})
	observed := &startupWaitObservedContext{Context: ctx, entered: entered}
	tier0Result := make(chan bool, 1)
	wg.Go(func() {
		sink := &engineStartupSink{s: s, proc: tier0, coord: tierCoord, startup: observed}
		tier0Result <- sink.transition(plugin.StageRegistration, plugin.StageConfig)
	})

	select {
	case <-entered:
	case ok := <-tier0Result:
		t.Fatalf("tier member 0 did not park at the tier barrier: %v", ok)
	case <-ctx.Done():
		t.Fatalf("tier member 0 never entered the tier barrier: %v", ctx.Err())
	}

	// Run a real 5-stage unbarriered handshake while tier0 waits at tierCoord.
	engineEnd, pluginEnd := net.Pipe()
	adhoc := process.NewProcess(plugin.PluginConfig{Name: "unbarriered-concurrent", Internal: true, Encoder: plugin.EncodingJSON})
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	adhoc.SetConn(plugipc.NewMuxPluginConn(engineMux))
	adhoc.SetRunning(true)

	adhocCtx, adhocCancel := context.WithCancel(ctx)
	t.Cleanup(func() {
		adhocCancel()
		_ = engineMux.Close() //nolint:errcheck // test cleanup
		_ = pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	wg.Go(func() {
		p := sdk.NewWithConn("unbarriered-concurrent", pluginEnd)
		_ = p.Run(adhocCtx, &sdk.Registration{}) //nolint:errcheck // test cancels adhocCtx on cleanup
	})

	s.runUnbarrieredStartupHandshake(ctx, adhoc)
	require.Equal(t, plugin.StageRunning, adhoc.Stage(), "unbarriered handshake must reach running without waiting on the tier")

	s.coordinatorMu.Lock()
	gotCoord := s.coordinator
	s.coordinatorMu.Unlock()
	assert.Same(t, tierCoord, gotCoord, "unbarriered handshake must not clear or alter s.coordinator")

	select {
	case ok := <-tier0Result:
		t.Fatalf("tier member 0 left the barrier before member 1 completed: %v", ok)
	default:
	}

	sink1 := &engineStartupSink{s: s, proc: tier1, coord: tierCoord, startup: ctx}
	require.True(t, sink1.transition(plugin.StageRegistration, plugin.StageConfig),
		"tier member 1 must complete registration on the intact coordinator")
	require.True(t, receiveWithin(t, tier0Result, failureWait, "tier member 0 did not release after member 1 completed"),
		"tier member 0 must advance cleanly to StageConfig")
	assert.Equal(t, plugin.StageConfig, tier0.Stage())
	assert.Equal(t, plugin.StageConfig, tier1.Stage())
}

// TestLateCleanupRecordTracksMultipleProcessesPerName verifies that
// lateCleanupRecord retains every stopped process per plugin name and clears
// only once every recorded process has finished its runtime cleanup.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-4 --
// lateCleanupRecord tracks []*process.Process per name with pointer dedup.
// PREVENTS: a newer stopped process finishing cleanup early from clearing an
// earlier predecessor whose cleanup is still owed.
func TestLateCleanupRecordTracksMultipleProcessesPerName(t *testing.T) {
	const name = "multi-cleanup-plugin"
	var record lateCleanupRecord

	record.owe(nil)
	assert.False(t, record.owed(name))

	p1 := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true})
	p2 := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true})

	record.owe(p1)
	record.owe(p1)
	record.owe(p2)
	assert.True(t, record.owed(name), "both processes owe cleanup")

	// Finish the newer process first; p1 still owes cleanup.
	p2.RunRuntimeCleanupOnce(func() {})
	assert.True(t, record.owed(name), "owed must stay true while earlier process p1 still owes cleanup")

	// Finish the earlier process; now the name clears.
	p1.RunRuntimeCleanupOnce(func() {})
	assert.False(t, record.owed(name), "owed must clear once every recorded process has finished cleanup")
}

// TestLateCleanupClearRefreshesStoppedBusWarning verifies that when a restart
// fails because the stopped process's cleanup outlives restartCleanupGrace,
// the doctor-plugin-stopped report-bus warning carries the shared cleanup-owed
// refusal and daemon-restart last resort with a bare restart command Fix, and
// that completing the cleanup automatically refreshes the bus warning to drop
// the cleanup-owed text while keeping the warning active.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-6 --
// restartReplacementWithin records CleanupOwed on PluginStop, cleanupProcess
// invokes lateCleanupOwed on completion to refresh the bus warning with zero
// new goroutines, and direct lateCleanupOwed probes also refresh when cleanup
// finishes outside cleanupProcess.
// PREVENTS: the report bus continuing to tell operators restarts are refused
// after the stopped predecessor's cleanup has already completed.
func TestLateCleanupClearRefreshesStoppedBusWarning(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	keepCleanupHooks(t)
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))

	findWarning := func(t *testing.T, name string) report.Issue {
		t.Helper()
		for _, w := range report.Warnings() {
			if w.Code == diagnostic.CodeDoctorPluginStopped && w.Subject == name {
				return w
			}
		}
		t.Fatalf("no active %s warning on report bus for %s", diagnostic.CodeDoctorPluginStopped, name)
		return report.Issue{}
	}

	const name = "restart-bus-refresh"
	s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
	pm := s.procManager.Load()
	old := pm.GetProcess(name)

	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	entered, release := holdFirstCleanup(t, name)

	answered := make(chan restartOutcome, 1)
	wg.Go(func() {
		resp, err := handlePluginRestart(&CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, name))
		answered <- restartOutcome{resp: resp, err: err}
	})
	outcome := receiveWithin(t, answered, restartAnswerBound, "restart never answered while old cleanup was held")
	receiveWithin(t, entered, restartAnswerBound, "stopped process cleanup never started")
	require.NoError(t, outcome.err)
	require.NotEqual(t, plugin.StatusDone, outcome.resp.Status)

	stop, down := pm.Stopped(name)
	require.True(t, down, "plugin must be recorded down after cleanup-owed restart failure")
	require.True(t, stop.CleanupOwed, "stop record must carry CleanupOwed while old cleanup is held")

	const refusalPhrase = "refuses to start the plugin"
	owedWarn := findWarning(t, name)
	assert.Contains(t, owedWarn.Message, refusalPhrase)
	assert.Contains(t, owedWarn.Message, process.DaemonRestartLastResort())
	assert.Equal(t, process.RestartCommand(name), owedWarn.Fix)

	// Releasing old's cleanup finishes cleanupProcess, which calls
	// lateCleanupOwed(name) directly without waiting for a doctor read.
	release()
	cleanupCtx, cleanupCancel := context.WithTimeout(t.Context(), restartAnswerBound)
	defer cleanupCancel()
	require.NoError(t, old.WaitRuntimeCleanupContext(cleanupCtx))

	require.Eventually(t, func() bool {
		st, ok := pm.Stopped(name)
		return ok && !st.CleanupOwed
	}, failureWait, 10*time.Millisecond, "cleanupProcess must clear CleanupOwed automatically once cleanup finishes")

	refreshedWarn := findWarning(t, name)
	assert.NotContains(t, refreshedWarn.Message, refusalPhrase,
		"refreshed bus warning must drop cleanup-owed refusal")
	assert.NotContains(t, refreshedWarn.Message, process.DaemonRestartLastResort(),
		"refreshed bus warning must drop daemon-restart last resort")
	assert.Equal(t, process.RestartCommand(name), refreshedWarn.Fix)

	// Also verify direct lateCleanupOwed refresh when a process finishes
	// cleanup outside cleanupProcess and when a preserved stop record hits a
	// cleanup-owed pre-spawn refusal.
	owedProc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true})
	s.lateCleanups.owe(owedProc)
	t.Cleanup(func() { owedProc.RunRuntimeCleanupOnce(func() {}) })

	err := s.restartPlugin(name)
	require.Error(t, err)
	stopAfterRefusal, downAfterRefusal := pm.Stopped(name)
	require.True(t, downAfterRefusal)
	assert.True(t, stopAfterRefusal.CleanupOwed, "pre-spawn cleanup-owed refusal must mark CleanupOwed on preserved stop record")
	assert.Contains(t, findWarning(t, name).Message, refusalPhrase)

	owedProc.RunRuntimeCleanupOnce(func() {})
	assert.False(t, s.lateCleanupOwed(name), "lateCleanupOwed must observe completion")
	stopCleared, downCleared := pm.Stopped(name)
	require.True(t, downCleared)
	assert.False(t, stopCleared.CleanupOwed, "lateCleanupOwed must clear CleanupOwed on transition")
	assert.NotContains(t, findWarning(t, name).Message, refusalPhrase,
		"bus warning must refresh when lateCleanupOwed observes completion")
}
