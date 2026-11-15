package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// failureWait bounds the waits in these tests. A restart is a process stop, a
// spawn and a five-stage handshake over an in-memory pipe, so it lands in
// milliseconds; the bound covers a loaded build machine.
const failureWait = 10 * time.Second

// failureSettle is how long a negative test watches for something that must NOT
// happen. It is a wait for an absence, so it can only be a duration: the event
// it is watching for would land inside it many times over.
const failureSettle = 2 * time.Second

// crashOnSignal ends a plugin engine's context when the test says the plugin
// crashed. One goroutine for one plugin generation, ended by the close it waits
// for (ai/rules/goroutine-lifecycle.md).
func crashOnSignal(crash <-chan struct{}, cancel context.CancelFunc) {
	<-crash
	cancel()
}

// registerCrashingPlugin registers a plugin that declares policy and whose FIRST
// generation ends its own engine when crash closes. Later generations keep
// running, so a restart settles instead of looping. It returns the counter of
// engine starts, which is how many times the plugin has been spawned.
func registerCrashingPlugin(t *testing.T, name, commandName string, policy rpc.FailurePolicy, crash <-chan struct{}) *atomic.Int64 {
	t.Helper()
	starts := &atomic.Int64{}

	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "failure-policy test plugin that ends its engine on demand",
		RunEngine: func(conn net.Conn) int {
			generation := starts.Add(1)
			p := sdk.NewWithConn(name, conn)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if generation == 1 {
				go crashOnSignal(crash, cancel)
			}
			if err := p.Run(ctx, &sdk.Registration{
				FailurePolicy: policy,
				Commands:      []sdk.CommandDecl{{Name: commandName}},
			}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	return starts
}

// watchDaemonStop records whether the daemon was asked to stop. It watches both
// halves of the route stopDaemonForPlugin takes: the shutdown function a daemon
// wires, and the shutdown-requested channel Server.Wait reads.
// It MUST be called before runPluginPhase: that phase starts one supervisor
// goroutine per running plugin, and each of them reads the shutdown function.
func watchDaemonStop(s *Server) (signaled, stopped func() bool) {
	var called atomic.Bool
	s.SetShutdownFunc(func() { called.Store(true) })
	return called.Load, func() bool {
		select {
		case <-s.shutdownRequested:
			return true
		default:
			return false
		}
	}
}

// TestPluginThatExitsIsStartedAgainWhenItDeclaredRestart verifies that a plugin
// whose process ends on its own is started again, with no operator action and no
// config reload, and that the replacement completes the startup handshake.
//
// VALIDATES: AC-1 -- a plugin that declares restart and exits is running again,
// and its command resolves to the replacement, so an operator reaches it exactly
// as before.
// PREVENTS: the state this spec found. ProcessManager.Respawn had one caller, a
// config-reload rollback, so nothing in the daemon watched a plugin process
// exit. A crashed plugin stayed dead for the life of the daemon whatever anybody
// wrote, and the leaf that promised otherwise reached no code at all.
func TestPluginThatExitsIsStartedAgainWhenItDeclaredRestart(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "failure-restart"
	const commandName = "show failure restart"

	crash := make(chan struct{})
	starts := registerCrashingPlugin(t, name, commandName, rpc.FailureRestart, crash)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	require.Equal(t, int64(1), starts.Load(), "startup must spawn the plugin once")

	// Startup is over: the command registry is frozen, exactly as
	// signalStartupComplete leaves it before any crash can happen.
	s.dispatcher.Registry().Freeze()

	pm := s.procManager.Load()
	require.NotNil(t, pm)
	first := pm.GetProcess(name)
	require.NotNil(t, first)

	close(crash)

	require.Eventually(t, func() bool { return starts.Load() == 2 }, failureWait, 10*time.Millisecond,
		"the plugin exited and was never started again")
	require.Eventually(t, func() bool { return pm.GetProcess(name) != first }, failureWait, 10*time.Millisecond,
		"the manager still holds the exited process")

	cmd := s.dispatcher.Registry().Lookup(commandName)
	require.NotNil(t, cmd, "the replacement's command must be resolvable again")
	assert.Same(t, pm.GetProcess(name), cmd.Process,
		"the command must resolve to the replacement process")
}

// TestPluginThatExitsStaysDownWhenItDeclaredNothing verifies the other polarity:
// a plugin that said nothing about its own failure is left stopped.
//
// VALIDATES: AC-2 -- an undeclared policy reads as ignore, so the plugin stays
// down and the daemon keeps running.
// PREVENTS: a watcher that restarts every plugin it sees exit, which would turn
// a one-shot plugin into a loop and read silence as consent. It is also what
// makes the positive test above discriminate: without it, a restart-everything
// watcher would pass that one.
func TestPluginThatExitsStaysDownWhenItDeclaredNothing(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "failure-silent"
	const commandName = "show failure silent"

	crash := make(chan struct{})
	starts := registerCrashingPlugin(t, name, commandName, rpc.FailureUnspecified, crash)

	s, _ := newLifecycleStartupServer(t)
	_, stopped := watchDaemonStop(s)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	require.Equal(t, int64(1), starts.Load(), "startup must spawn the plugin once")

	s.dispatcher.Registry().Freeze()

	pm := s.procManager.Load()
	require.NotNil(t, pm)
	first := pm.GetProcess(name)
	require.NotNil(t, first)

	close(crash)

	assert.Never(t, func() bool { return starts.Load() != 1 }, failureSettle, 50*time.Millisecond,
		"the plugin was started again although it declared nothing")
	assert.Same(t, first, pm.GetProcess(name), "the manager must keep the stopped process")
	assert.False(t, stopped(), "an ignored failure must not stop the daemon")
	assert.Nil(t, s.StopFailure(), "an ignored failure must not make the daemon exit 1")
}

// TestPluginThatExitsStopsTheDaemonWhenItDeclaredFatal verifies the third
// outcome: a plugin whose author said its failure ends ze does end ze.
//
// VALIDATES: AC-3 -- a fatal plugin's exit signals the shutdown Server.Wait
// reads and calls the daemon's own shutdown function, and StopFailure then
// answers the operator error naming the plugin, which makes the daemon exit 1.
// PREVENTS: fatal being implemented as a log line. The daemon keeps running
// until something ends the wait in waitLoop (cmd/ze/hub/main.go), so a policy
// that only logs leaves the router up with the plugin its operator said it could
// not run without. Also a fatal stop exiting 0, which a service manager
// (systemd Restart=on-failure) and an operator's script read as a clean stop.
func TestPluginThatExitsStopsTheDaemonWhenItDeclaredFatal(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "failure-fatal"
	const commandName = "show failure fatal"

	crash := make(chan struct{})
	starts := registerCrashingPlugin(t, name, commandName, rpc.FailureFatal, crash)

	s, _ := newLifecycleStartupServer(t)
	signaled, stopped := watchDaemonStop(s)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	require.Equal(t, int64(1), starts.Load(), "startup must spawn the plugin once")

	s.dispatcher.Registry().Freeze()
	require.Nil(t, s.StopFailure(), "a running daemon reported a failed stop before the plugin exited")

	close(crash)

	require.Eventually(t, stopped, failureWait, 10*time.Millisecond,
		"a fatal plugin's exit did not signal the daemon shutdown Server.Wait reads")
	assert.True(t, signaled(), "a fatal plugin's exit must reach the daemon's own shutdown function")
	// The daemon's exit status reads this (runYANGConfig, cmd/ze/hub/main.go):
	// false here is a fatal stop exiting 0, read as a clean shutdown.
	failure := s.StopFailure()
	require.NotNil(t, failure, "a fatal plugin's stop must make the daemon exit 1")
	assert.Contains(t, failure.Problem(), name, "the stop failure must name the plugin")
	assert.Contains(t, failure.Error(), "the plugin declares that its failure stops ze", "the stop failure must carry the reason")
	assert.Equal(t, int64(1), starts.Load(), "a fatal plugin must not be started again")
}

// TestFatalPluginExitDuringAStopIsNotItsCause verifies that a fatal plugin
// exiting once the daemon's stop is under way does not turn that stop into a
// failure. Method: the stop is begun by each route that marks it (an accepted
// stop request, and the drain), then the plugin exits; the daemon's shutdown
// function must never be called again, StopFailure must stay nil and no
// last-stop record may be written.
//
// VALIDATES: an operator's clean stop exits 0 and `ze doctor` names no fatal
// stop for it, even when a fatal plugin exits during the drain or teardown.
// PREVENTS: a plugin that exits on its own after `request shutdown` or during
// the drain being recorded as the cause of the stop, so the clean stop exits 1
// and the next `ze doctor` names a fatal stop that never happened.
func TestFatalPluginExitDuringAStopIsNotItsCause(t *testing.T) {
	tests := []struct {
		name  string
		begin func(*Server)
	}{
		{"after an accepted stop request", func(s *Server) { s.SignalShutdownRequested() }},
		{"after the drain began", func(s *Server) { s.BeginDrain() }},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snap) })
			useLastStopStore(t)

			name := fmt.Sprintf("failure-fatal-stopping-%d", i)
			commandName := fmt.Sprintf("show failure fatal stopping %d", i)

			crash := make(chan struct{})
			starts := registerCrashingPlugin(t, name, commandName, rpc.FailureFatal, crash)

			s, _ := newLifecycleStartupServer(t)
			signaled, _ := watchDaemonStop(s)
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
				{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
			}))
			require.Equal(t, int64(1), starts.Load(), "startup must spawn the plugin once")
			s.dispatcher.Registry().Freeze()

			tt.begin(s)
			close(crash)

			// The crash must have been handled: applyFailurePolicy raises the
			// crash report just before it acts on the policy. Without this the
			// absences below would also pass for a crash nobody processed.
			require.Eventually(t, func() bool { return crashReported(name) }, failureWait, 10*time.Millisecond,
				"the plugin's exit was never handled by its failure policy")
			assert.Never(t, signaled, failureSettle, 50*time.Millisecond,
				"a fatal plugin that exited during the stop was read as its cause")
			assert.Nil(t, s.StopFailure(), "a clean stop must not exit 1 because a plugin exited during it")
			_, found, err := ReadLastStop()
			require.NoError(t, err)
			assert.False(t, found, "a clean stop must leave no fatal-stop record for ze doctor")
		})
	}
}

// crashReported reports whether the report bus holds the crash report
// applyFailurePolicy raises for plugin name (ProcessManager.ReportCrash).
func crashReported(name string) bool {
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorPluginCrashed && issue.Subject == name {
			return true
		}
	}
	return false
}

// TestPluginStopFailureNamesTheFixOfItsCause verifies the error ze prints at
// exit after a plugin stopped it: each cause has its own why and fix, and a
// refused record does not send the operator to a doctor that cannot answer.
//
// VALIDATES: the fatal policy points at ze doctor, or at the log when the
// record was refused; the respawn disagreement names the config change the
// startup refusal names; every form names the plugin and keeps the reason.
// PREVENTS: one wording for every stop, which told the operator of a respawn
// disagreement that the plugin forbade something, and sent them to ze doctor
// when no record existed.
func TestPluginStopFailureNamesTheFixOfItsCause(t *testing.T) {
	const name = "p1"
	tests := []struct {
		name     string
		cause    pluginStopCause
		recorded bool
		why      string
		fix      string
	}{
		{"fatal, recorded", pluginStopFatalPolicy, true, "failure policy says ze must not keep running without it", "ze doctor"},
		{"fatal, record refused", pluginStopFatalPolicy, false, "ze doctor cannot name it", "read the ze log lines that name p1"},
		{"respawn disagreement", pluginStopRespawnDisagreement, true, "asks to restart a plugin that declares it cannot be restarted", respawnDisagreementFix()},
		{"respawn disagreement, record refused", pluginStopRespawnDisagreement, false, "asks to restart a plugin that declares it cannot be restarted", respawnDisagreementFix()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := pluginStopFailure(name, tt.cause, "the evidence", tt.recorded)
			assert.Contains(t, failure.Problem(), name, "the error must name the plugin")
			assert.Contains(t, failure.Why(), tt.why)
			assert.Equal(t, tt.fix, failure.Fix())
			assert.Contains(t, failure.Error(), "the evidence", "the error must keep the reason as its evidence")
		})
	}
}

// TestRespawnFalseKeepsAPluginThatCanRestartDown verifies that a block declining
// the respawn of a plugin willing to be restarted is honored.
//
// VALIDATES: AC-8 -- `respawn false` asks for less than the declaration permits,
// which is inside the constraint, so the plugin is left stopped.
// PREVENTS: the leaf being inert in one of its two values. A `respawn false`
// that changed nothing would be exactly the defect this spec exists to end,
// moved from `true` to `false`.
func TestRespawnFalseKeepsAPluginThatCanRestartDown(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "failure-declined"
	const commandName = "show failure declined"

	crash := make(chan struct{})
	starts := registerCrashingPlugin(t, name, commandName, rpc.FailureRestart, crash)

	s, _ := newLifecycleStartupServer(t)
	_, stopped := watchDaemonStop(s)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON, Respawn: plugin.RespawnDeclined},
	}))
	require.Equal(t, int64(1), starts.Load(), "startup must spawn the plugin once")

	s.dispatcher.Registry().Freeze()

	close(crash)

	assert.Never(t, func() bool { return starts.Load() != 1 }, failureSettle, 50*time.Millisecond,
		"the plugin was started again although the block declined the respawn")
	assert.False(t, stopped(), "declining a respawn must not stop the daemon")
}

// TestRespawnRequestAgainstAPluginThatCannotRestartStopsTheDaemon verifies the
// owner's rule: "if a plugin declare that it can not restart then we can not
// start" (2026-09-06).
//
// VALIDATES: AC-7 -- the daemon stops, and the refusal names the plugin, the
// policy it declared and the leaf the configuration wrote.
// PREVENTS: ze picking a side in silence. Honoring the leaf would restart a
// plugin whose author forbade it; honoring the declaration would leave the
// operator's written instruction with no effect. Both are the silent wrong
// answer, and an operator who is told neither loses the afternoon to it.
func TestRespawnRequestAgainstAPluginThatCannotRestartStopsTheDaemon(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "failure-disagree"
	const commandName = "show failure disagree"

	crash := make(chan struct{})
	starts := registerCrashingPlugin(t, name, commandName, rpc.FailureIgnore, crash)
	t.Cleanup(func() { close(crash) })

	s, _ := newLifecycleStartupServer(t)
	signaled, stopped := watchDaemonStop(s)

	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON, Respawn: plugin.RespawnAsked},
	})

	require.Error(t, err, "a plugin that cannot meet the configured respawn must not start")
	assert.ErrorIs(t, err, errRespawnDisagreement)
	assert.Contains(t, err.Error(), name, "the refusal must name the plugin")
	assert.Contains(t, err.Error(), "ignore", "the refusal must name the policy the plugin declared")
	assert.Contains(t, err.Error(), "respawn", "the refusal must name the leaf the configuration wrote")

	assert.True(t, stopped(), "the disagreement must stop the daemon")
	assert.True(t, signaled(), "the disagreement must reach the daemon's own shutdown function")
	failure := s.StopFailure()
	require.NotNil(t, failure, "a daemon that refused to start must exit 1")
	assert.Equal(t, respawnDisagreementFix(), failure.Fix(), "the stop must name the config change, as the startup refusal does")
	assert.Equal(t, int64(1), starts.Load(), "the refused plugin must not be spawned twice")
}

// TestStartupFailureOfAFatalPluginStopsTheDaemon verifies that fatal binds at
// startup too, from the moment the plugin has declared it.
//
// VALIDATES: AC-4 -- a plugin that declares fatal in Stage 1 and then fails
// Stage 2 stops ze rather than leaving it running degraded.
// PREVENTS: fatal meaning less than it says at the moment it matters most. A
// failed plugin phase logs and returns (runPluginStartup), so without this the
// daemon came up without the plugin its operator was told it could not run
// without.
func TestStartupFailureOfAFatalPluginStopsTheDaemon(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "failure-fatal-startup"
	configureFailed := errors.New("this plugin refuses its configuration")

	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "declares fatal in stage 1 and then fails stage 2",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			p.OnConfigure(func([]sdk.ConfigSection) error { return configureFailed })
			if err := p.Run(context.Background(), &sdk.Registration{
				FailurePolicy: rpc.FailureFatal,
			}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	s, _ := newLifecycleStartupServer(t)
	signaled, stopped := watchDaemonStop(s)

	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	})

	require.Error(t, err, "the plugin refused its configuration, so the phase must fail")
	assert.True(t, stopped(), "a fatal plugin that fails startup must stop the daemon")
	assert.True(t, signaled(), "the stop must reach the daemon's own shutdown function")
	assert.NotNil(t, s.StopFailure(), "a fatal plugin that fails startup must make the daemon exit 1")
}

// TestStartupFailureOfASilentPluginLeavesTheDaemonRunning is the polarity that
// makes the test above discriminate: a plugin that declared nothing and failed
// startup leaves ze running, which is what ze did before this spec.
//
// VALIDATES: AC-2 -- silence is read as ignore at startup as well as at runtime.
// PREVENTS: a startup path that stops the daemon for every failed plugin, which
// would pass the fatal test above while breaking every deployment whose optional
// plugin cannot start.
func TestStartupFailureOfASilentPluginLeavesTheDaemonRunning(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	const name = "failure-silent-startup"
	configureFailed := errors.New("this plugin refuses its configuration")

	require.NoError(t, registry.Register(registry.Registration{
		Name:        name,
		Description: "declares nothing and then fails stage 2",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			p.OnConfigure(func([]sdk.ConfigSection) error { return configureFailed })
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))

	s, _ := newLifecycleStartupServer(t)
	_, stopped := watchDaemonStop(s)

	err := s.runPluginPhase([]plugin.PluginConfig{
		{Name: name, Internal: true, Encoder: plugin.EncodingJSON},
	})

	require.Error(t, err, "the plugin refused its configuration, so the phase must fail")
	assert.False(t, stopped(), "a plugin that declared nothing must not stop the daemon")
	assert.Nil(t, s.StopFailure(), "a plugin that declared nothing must not make the daemon exit 1")
}

// newFailurePolicyProcess builds a process carrying one declaration and one
// config request, which is the pair pluginFailurePolicy reads. It runs nothing:
// the resolution is a pure read of the two.
func newFailurePolicyProcess(t *testing.T, declared rpc.FailurePolicy, asked plugin.RespawnRequest) *process.Process {
	t.Helper()
	proc := process.NewProcess(plugin.PluginConfig{
		Name:     "failure-policy-resolution",
		Internal: true,
		Encoder:  plugin.EncodingJSON,
		Respawn:  asked,
	})
	proc.SetRegistration(&plugin.PluginRegistration{FailurePolicy: declared})
	return proc
}

// TestPluginFailurePolicyReadsAPluginThatNeverDeclaredAsIgnore covers the
// process that failed before Stage 1. NewProcess gives every process an EMPTY
// registration, and SetRegistration fills it at Stage 1, so such a plugin is not
// distinguishable by a nil: the unspecified sentinel is what carries the case.
//
// VALIDATES: AC-2 -- a plugin ze was never told about is not restarted.
// PREVENTS: the opposite reading, in which an unknown plugin is restarted
// because nothing said not to.
func TestPluginFailurePolicyReadsAPluginThatNeverDeclaredAsIgnore(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "never-declared", Internal: true})
	require.Equal(t, rpc.FailureUnspecified, proc.Registration().FailurePolicy,
		"a process that never reached stage 1 must carry no declared policy")
	assert.Equal(t, rpc.FailureIgnore, pluginFailurePolicy(proc))
}

// TestPluginFailurePolicyReadsAnUndeclaredPolicyAsIgnore covers the one function
// that turns a declaration into an outcome, over the four inputs it has to tell
// apart. The tests above prove the outcomes reach the daemon; this one proves
// the resolution itself, including the pairing no daemon-level test can reach
// because it is refused at startup.
//
// VALIDATES: AC-2, AC-8 -- silence resolves to ignore, and a declined respawn
// takes a restart away from a plugin that would have had one.
func TestPluginFailurePolicyReadsAnUndeclaredPolicyAsIgnore(t *testing.T) {
	cases := []struct {
		name     string
		declared rpc.FailurePolicy
		asked    plugin.RespawnRequest
		want     rpc.FailurePolicy
	}{
		{"silence is ignore", rpc.FailureUnspecified, plugin.RespawnUnstated, rpc.FailureIgnore},
		{"silence stays ignore even when a respawn is declined", rpc.FailureUnspecified, plugin.RespawnDeclined, rpc.FailureIgnore},
		{"restart stands when the block says nothing", rpc.FailureRestart, plugin.RespawnUnstated, rpc.FailureRestart},
		{"restart stands when the block asks for one", rpc.FailureRestart, plugin.RespawnAsked, rpc.FailureRestart},
		{"a declined respawn leaves a restarting plugin down", rpc.FailureRestart, plugin.RespawnDeclined, rpc.FailureIgnore},
		{"ignore stands", rpc.FailureIgnore, plugin.RespawnUnstated, rpc.FailureIgnore},
		{"fatal stands", rpc.FailureFatal, plugin.RespawnUnstated, rpc.FailureFatal},
		{"a declined respawn does not soften fatal", rpc.FailureFatal, plugin.RespawnDeclined, rpc.FailureFatal},
		{"unknown typed policy stays unrecognized", rpc.FailurePolicy(255), plugin.RespawnUnstated, rpc.FailurePolicy(255)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proc := newFailurePolicyProcess(t, tc.declared, tc.asked)
			assert.Equal(t, tc.want, pluginFailurePolicy(proc))
		})
	}
}

// TestRefuseRespawnDisagreementRefusesOnlyTheAskedPairings covers the guard's
// whole input space, including the pairings it must let through.
//
// VALIDATES: AC-7 -- only `respawn true` against a plugin that must not be
// restarted is refused.
// PREVENTS: a guard that refuses `respawn false`, which would stop the daemon
// for an ExaBGP configuration that turned respawning off.
func TestRefuseRespawnDisagreementRefusesOnlyTheAskedPairings(t *testing.T) {
	cases := []struct {
		name     string
		asked    plugin.RespawnRequest
		declared rpc.FailurePolicy
		refused  bool
	}{
		{"asked and the plugin restarts", plugin.RespawnAsked, rpc.FailureRestart, false},
		{"asked and the plugin ignores", plugin.RespawnAsked, rpc.FailureIgnore, true},
		{"asked and the plugin is fatal", plugin.RespawnAsked, rpc.FailureFatal, true},
		{"asked and the plugin declared nothing", plugin.RespawnAsked, rpc.FailureUnspecified, true},
		{"unstated against a fatal plugin", plugin.RespawnUnstated, rpc.FailureFatal, false},
		{"declined against a fatal plugin", plugin.RespawnDeclined, rpc.FailureFatal, false},
		{"declined against a restarting plugin", plugin.RespawnDeclined, rpc.FailureRestart, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := refuseRespawnDisagreement("p", tc.asked, tc.declared)
			if !tc.refused {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, errRespawnDisagreement)
		})
	}
}
