package server

import (
	"context"
	"errors"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// restartAnswerBound is how long a test waits for a restart to answer. It is
// far past restartCleanupGrace, so a restart that answers inside it has not
// hung, and a restart that hangs fails the test rather than the run.
const restartAnswerBound = restartCleanupGrace + 10*time.Second

// keepCleanupHooks restores the process cleanup hooks the package registered
// once the test ends. Call it before the server starts, so the restore runs
// after the server's teardown, which runs the hooks.
func keepCleanupHooks(t *testing.T) {
	t.Helper()
	processCleanupMu.Lock()
	saved := slices.Clone(processCleanupHooks)
	processCleanupMu.Unlock()
	t.Cleanup(func() {
		processCleanupMu.Lock()
		processCleanupHooks = saved
		processCleanupMu.Unlock()
	})
}

// holdFirstCleanup makes the first process cleanup of name that runs after
// this call wait until release is called, and closes entered when it starts
// waiting. Every later cleanup of name passes. The test's cleanup releases it,
// so no goroutine stays blocked past the test.
//
// It stands for a process whose runtime cleanup cannot run because a call it
// made into the engine has not returned: cleanupProcess is the runtime
// cleanup WaitRuntimeCleanupContext waits for, and the hooks run inside it.
func holdFirstCleanup(t *testing.T, name string) (entered <-chan struct{}, release func()) {
	t.Helper()
	enteredCh := make(chan struct{})
	releaseCh := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(releaseCh) }) }
	t.Cleanup(release)
	var armed atomic.Bool
	armed.Store(true)
	RegisterProcessCleanup(func(processName string) {
		if processName != name {
			return
		}
		if !armed.CompareAndSwap(true, false) {
			return
		}
		close(enteredCh)
		<-releaseCh
	})
	return enteredCh, release
}

// requireTxLockFree takes and gives back the config transaction lock, failing
// the test when something still holds it.
func requireTxLockFree(t *testing.T, s *Server) {
	t.Helper()
	if !s.txLock.tryAcquire() {
		t.Fatal("the restart left the config transaction lock held")
	}
	s.txLock.release()
}

// TestRestartCommandRefusesItsCaller has a plugin ask, through the
// dispatcher, for its own restart.
//
// Goal: the restart of the plugin that sent the command is refused at once,
// before it takes the transaction lock or stops anything.
// Method: the line is dispatched as text with the command context the plugin
// dispatch paths build (Process and Sender name the plugin), on another
// goroutine whose answer is bounded; the lock is then taken to show it is free.
// PREVENTS: the restart waiting for the runtime cleanup of the process whose
// own in-flight dispatch is this command, so it never answers and holds the
// lock for good.
func TestRestartCommandRefusesItsCaller(t *testing.T) {
	d := restartLineDispatcher(t)
	const name = "restart-self"
	s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
	proc := s.procManager.Load().GetProcess(name)
	ctx := &CommandContext{Server: s, Process: proc, Sender: plugin.ProcessSender(name)}

	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	answered := make(chan restartOutcome, 1)
	wg.Go(func() {
		resp, err := d.Dispatch(ctx, "request plugin restart "+name)
		answered <- restartOutcome{resp: resp, err: err}
	})
	outcome := receiveWithin(t, answered, restartAnswerBound, "the restart a plugin asked for itself never answered")

	if outcome.err != nil {
		t.Fatalf("dispatch: %v", outcome.err)
	}
	if outcome.resp.Status == plugin.StatusDone {
		t.Fatalf("a plugin's restart of itself was accepted: %+v", outcome.resp)
	}
	if !strings.Contains(outcome.resp.Error, "cannot restart itself") {
		t.Fatalf("the refusal does not say the plugin asked for its own restart: %s", outcome.resp.Error)
	}
	if after := s.procManager.Load().GetProcess(name); after != proc {
		t.Fatal("the refused self-restart replaced the process")
	}
	requireTxLockFree(t, s)
}

// TestRestartCommandRefusesPendingRemoval restarts a running plugin whose
// removal a refused reload left pending.
//
// Goal: the command refuses a plugin a removal attempt still owns, and leaves
// its process and the transaction lock as they were.
// Method: the removal record a refused reload leaves behind (a Bye not yet
// answered, its recovery waiting) is put on the server for the running
// process; the handler runs on it.
// PREVENTS: a restart replacing the process under the removal attempt, whose
// recovery would then restore or retire a process that no longer serves.
func TestRestartCommandRefusesPendingRemoval(t *testing.T) {
	const name = "restart-removing"
	s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
	proc := s.procManager.Load().GetProcess(name)
	s.removalMu.Lock()
	if s.removals == nil {
		s.removals = make(map[*process.Process]*pluginRemoval)
	}
	s.removals[proc] = &pluginRemoval{done: make(chan struct{}), recovering: true}
	s.removalMu.Unlock()
	// Runs before the server's teardown, whose supervisor reads the record.
	t.Cleanup(func() {
		s.removalMu.Lock()
		delete(s.removals, proc)
		s.removalMu.Unlock()
	})

	resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args("name", name))
	if resp.Status == plugin.StatusDone {
		t.Fatalf("the restart of a plugin whose removal is pending was accepted: %+v", resp)
	}
	if !strings.Contains(resp.Error, "removal") {
		t.Fatalf("the refusal does not name the pending removal: %s", resp.Error)
	}
	if after := s.procManager.Load().GetProcess(name); after != proc {
		t.Fatal("the refused restart replaced the process")
	}
	requireTxLockFree(t, s)
}

// TestRestartCommandFailsWhenReplacementNeverHandshakes restarts a running
// plugin whose replacement connects and never sends Stage 1, and does not end
// on its closed connection either.
//
// Goal: the operator's restart answers as a failed restart once its handshake
// bound (one stage timeout per stage, restartHandshakeBound) has passed: the
// answer says the handshake did not finish in time, the transaction lock is
// free, the plugin is recorded stopped by a failed restart, and no
// replacement is left in its slot. The rollback's wait for the replacement,
// which does not end until the test lets it, is bounded as well.
// Method: the plugin's second process blocks before it runs the SDK, so the
// engine reads no Stage 1 and the stopped replacement's engine goroutine does
// not return. The plugin's stage timeout is set short in its config, which is
// what restartHandshakeBound reads, so the bound is a second. The command
// runs on another goroutine whose answer is bounded far below the default
// handshake bound; the replacement is let go when the test ends.
// PREVENTS: the restart's handshake waiting as long as the daemon runs with
// the config transaction lock held, and its rollback waiting the same way for
// a replacement that does not end.
func TestRestartCommandFailsWhenReplacementNeverHandshakes(t *testing.T) {
	const name = "restart-silent-replacement"
	const stage = 200 * time.Millisecond
	// Registered first, so it runs after the server's teardown, which ends a
	// handshake this test failed to bound.
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)

	hold := make(chan struct{})
	var holdOnce sync.Once
	releaseReplacement := func() { holdOnce.Do(func() { close(hold) }) }
	var starts atomic.Int32
	s := startRestartTargetConfig(t,
		plugin.PluginConfig{Name: name, Internal: true, Encoder: plugin.EncodingJSON, StageTimeout: stage},
		func(*sdk.Plugin) *sdk.Registration {
			if starts.Add(1) > 1 {
				<-hold
			}
			return &sdk.Registration{}
		})
	// Registered after the server's cleanups, so it runs before them.
	t.Cleanup(releaseReplacement)
	pm := s.procManager.Load()
	if bound := restartHandshakeBound(pm.GetProcess(name)); bound != restartHandshakeStages*stage {
		t.Fatalf("handshake bound %s, want %d stages of the plugin's %s stage timeout", bound, restartHandshakeStages, stage)
	}

	answered := make(chan restartOutcome, 1)
	wg.Go(func() {
		resp, err := handlePluginRestart(&CommandContext{Server: s}, commandtest.Args("name", name))
		answered <- restartOutcome{resp: resp, err: err}
	})
	outcome := receiveWithin(t, answered, restartAnswerBound, "the restart never answered while its replacement withheld Stage 1")

	if outcome.err != nil {
		t.Fatalf("handlePluginRestart: %v", outcome.err)
	}
	if outcome.resp.Status == plugin.StatusDone {
		t.Fatalf("the restart succeeded though the replacement never ran its handshake: %+v", outcome.resp)
	}
	if !strings.Contains(outcome.resp.Error, errRestartHandshakeLate.Error()) {
		t.Fatalf("the failure does not say the handshake did not finish in time: %s", outcome.resp.Error)
	}
	requireTxLockFree(t, s)
	stop, down := pm.Stopped(name)
	if !down || stop.Cause != process.StopRestartFailed {
		t.Fatalf("the plugin is not recorded stopped by a failed restart: %+v, down %v", stop, down)
	}
	if proc := pm.GetProcess(name); proc != nil {
		t.Fatalf("the replacement is left in the plugin's slot: %p, running %v", proc, proc.Running())
	}
}

// TestRestartCommandFailsWhenOldCleanupNeverEnds restarts a running plugin
// whose stopped process never finishes its runtime cleanup.
//
// Goal: the restart answers within its bound as a failed restart: the lock is
// free, the plugin is recorded stopped by a failed restart, and no
// replacement serves. While that cleanup is still owed a second restart and a
// startup phase naming the plugin are both refused before they start anything,
// and the doctor state says so; once it has run, the plugin restarts.
// Method: the first cleanup of the plugin's name is held, which is where a
// call into the engine that has not returned leaves it (cleanupProcess runs
// only after the in-flight dispatch ends). The command runs on another
// goroutine whose answer is bounded.
// PREVENTS: the restart waiting without bound with the transaction lock held,
// and a later restart starting a generation the late cleanup would strip of
// everything registered under its name.
func TestRestartCommandFailsWhenOldCleanupNeverEnds(t *testing.T) {
	const name = "restart-stuck-cleanup"
	keepCleanupHooks(t)
	s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
	pm := s.procManager.Load()
	old := pm.GetProcess(name)

	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	entered, release := holdFirstCleanup(t, name)

	answered := make(chan restartOutcome, 1)
	wg.Go(func() {
		resp, err := handlePluginRestart(&CommandContext{Server: s}, commandtest.Args("name", name))
		answered <- restartOutcome{resp: resp, err: err}
	})
	outcome := receiveWithin(t, answered, restartAnswerBound, "the restart never answered while the old cleanup was held")
	receiveWithin(t, entered, restartAnswerBound, "the stopped process's cleanup never started")

	if outcome.err != nil {
		t.Fatalf("handlePluginRestart: %v", outcome.err)
	}
	if outcome.resp.Status == plugin.StatusDone {
		t.Fatalf("the restart succeeded though the stopped process never ran its cleanup: %+v", outcome.resp)
	}
	if !strings.Contains(outcome.resp.Error, "or still be running") {
		t.Fatalf("the failure does not say the cleanup may be waiting on a call or still running: %s", outcome.resp.Error)
	}
	if strings.Contains(outcome.resp.Error, "has not returned") {
		t.Fatalf("the failure claims a call has not returned, which ze does not know: %s", outcome.resp.Error)
	}
	if !strings.Contains(outcome.resp.Error, "systemctl restart ze") {
		t.Fatalf("the failure does not name the daemon restart as the last resort: %s", outcome.resp.Error)
	}
	if !strings.HasSuffix(outcome.resp.Error, "fix: "+process.RestartCommand(name)) {
		t.Fatalf("the fix line is not the restart command alone: %s", outcome.resp.Error)
	}
	requireTxLockFree(t, s)
	stop, down := pm.Stopped(name)
	if !down || stop.Cause != process.StopRestartFailed {
		t.Fatalf("the plugin is not recorded stopped by a failed restart: %+v, down %v", stop, down)
	}
	if proc := pm.GetProcess(name); proc != nil {
		t.Fatalf("a replacement process is left serving: %p (old %p), running %v", proc, old, proc.Running())
	}

	err := s.restartPlugin(name)
	if !errors.Is(err, transaction.ErrRestartRefused) || !errors.Is(err, errReplacedCleanupLate) {
		t.Fatalf("a restart while the old cleanup is owed was not refused for it: %v", err)
	}
	if proc := pm.GetProcess(name); proc != nil {
		t.Fatal("the refused restart started a process")
	}
	// The other door a generation enters by: a reload's auto-load, or a
	// removal's recovery, starts the plugin through a startup phase.
	phaseErr := s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}})
	if !errors.Is(phaseErr, errReplacedCleanupLate) {
		t.Fatalf("a startup phase while the old cleanup is owed was not refused for it: %v", phaseErr)
	}
	if proc := pm.GetProcess(name); proc != nil {
		t.Fatal("the refused startup phase started a process")
	}
	if state := s.pluginDoctorState(name, true); !state.CleanupOwed {
		t.Fatalf("the doctor state does not say the cleanup is owed, so its row names a refused restart: %+v", state)
	}

	release()
	cleanupCtx, cleanupCancel := context.WithTimeout(t.Context(), restartAnswerBound)
	defer cleanupCancel()
	if err := old.WaitRuntimeCleanupContext(cleanupCtx); err != nil {
		t.Fatalf("the released cleanup never finished: %v", err)
	}
	if state := s.pluginDoctorState(name, true); state.CleanupOwed {
		t.Fatal("the doctor state still says the cleanup is owed after it ran")
	}
	if err := s.restartPlugin(name); err != nil {
		t.Fatalf("the restart after the old cleanup ran: %v", err)
	}
	if proc := pm.GetProcess(name); proc == nil || !proc.Running() {
		t.Fatal("the plugin does not run after the old cleanup ran and it was restarted")
	}
}

// TestRefusedRestartPreservesDisableAndRespawnCounters verifies that every
// pre-spawn refusal of `request plugin restart` leaves a respawn-disabled
// plugin's disable state, respawn counters, and StopRespawnLimit record intact,
// and that once unblocked an admitted restart lifts them atomically and brings
// the plugin back to running.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-7 --
// handlePluginRestart checks pluginRestartRefusal under txLock and defers
// re-enable into ProcessManager.RespawnReenabled after all pre-spawn refusals
// pass, while restartReplacementWithin preserves existing stop records on
// pre-spawn refusals (transaction.ErrRestartRefused).
// PREVENTS: a refused restart lifting the respawn-limit disable or resetting
// respawn counters, or overwriting StopRespawnLimit with StopRestartFailed.
func TestRefusedRestartPreservesDisableAndRespawnCounters(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))

	exhaustRespawnLimit := func(t *testing.T, s *Server, pm *process.ProcessManager, name string) process.PluginStop {
		t.Helper()
		for range process.RespawnLimit {
			if err := s.restartPlugin(name); err != nil {
				t.Fatalf("setup restart %s: %v", name, err)
			}
		}
		err := s.restartPlugin(name)
		if !errors.Is(err, process.ErrRespawnLimitExceeded) {
			t.Fatalf("expected ErrRespawnLimitExceeded after %d restarts, got %v", process.RespawnLimit, err)
		}
		if !errors.Is(err, transaction.ErrRestartRefused) {
			t.Fatalf("expected ErrRestartRefused on respawn limit, got %v", err)
		}
		if !pm.Disabled(name) {
			t.Fatalf("plugin %s was not disabled at respawn limit", name)
		}
		stop, down := pm.Stopped(name)
		if !down || stop.Cause != process.StopRespawnLimit {
			t.Fatalf("expected StopRespawnLimit preserved after hitting limit, got down=%v stop=%+v", down, stop)
		}
		return stop
	}

	assertStillDisabled := func(t *testing.T, pm *process.ProcessManager, name, wantDetail string) {
		t.Helper()
		if !pm.Disabled(name) {
			t.Fatalf("refused restart cleared disabled state for %s", name)
		}
		stop, down := pm.Stopped(name)
		if !down || stop.Cause != process.StopRespawnLimit {
			t.Fatalf("refused restart overwrote StopRespawnLimit: down=%v stop=%+v", down, stop)
		}
		if stop.Detail != wantDetail {
			t.Fatalf("stop detail changed from %q to %q", wantDetail, stop.Detail)
		}
	}

	t.Run("recoverable refusals keep disable until admitted restart", func(t *testing.T) {
		const name = "restart-refused-keeps-disable"
		s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
		pm := s.procManager.Load()
		initialStop := exhaustRespawnLimit(t, s, pm, name)

		// Unconfigured name refusal under txLock releases txLock cleanly and
		// leaves no stop record for the unknown name.
		const unknownName = "restart-refused-unknown"
		unknownResp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, unknownName))
		if unknownResp.Status == plugin.StatusDone {
			t.Fatalf("restart of unconfigured plugin was accepted: %+v", unknownResp)
		}
		if !strings.Contains(unknownResp.Error, "not a configured plugin") {
			t.Fatalf("unexpected refusal for unknown plugin: %s", unknownResp.Error)
		}
		requireTxLockFree(t, s)
		if _, down := pm.Stopped(unknownName); down {
			t.Fatal("refused restart of unconfigured plugin wrote a stop record")
		}

		// Refusal 1: restart controller busy.
		ctrl, err := s.restartControllers.tryAcquire(name)
		if err != nil {
			t.Fatalf("tryAcquire controller: %v", err)
		}
		resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, name))
		ctrl.release()
		if resp.Status == plugin.StatusDone {
			t.Fatalf("restart succeeded while controller was busy: %+v", resp)
		}
		if !strings.Contains(resp.Error, "another restart is still in progress") {
			t.Fatalf("controller-busy refusal had unexpected message: %s", resp.Error)
		}
		requireTxLockFree(t, s)
		assertStillDisabled(t, pm, name, initialStop.Detail)

		// Refusal 2: cleanup owed by a stopped predecessor.
		owedProc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true})
		s.lateCleanups.owe(owedProc)
		t.Cleanup(func() { owedProc.RunRuntimeCleanupOnce(func() {}) })
		resp = restartByCommand(t, &CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, name))
		if resp.Status == plugin.StatusDone {
			t.Fatalf("restart succeeded while cleanup was owed: %+v", resp)
		}
		requireTxLockFree(t, s)
		assertStillDisabled(t, pm, name, initialStop.Detail)
		owedProc.RunRuntimeCleanupOnce(func() {})

		// Refusal 3: pending removal.
		live := pm.GetProcess(name)
		s.removalMu.Lock()
		if s.removals == nil {
			s.removals = make(map[*process.Process]*pluginRemoval)
		}
		s.removals[live] = &pluginRemoval{done: make(chan struct{}), recovering: true}
		s.removalMu.Unlock()
		resp = restartByCommand(t, &CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, name))
		s.removalMu.Lock()
		delete(s.removals, live)
		s.removalMu.Unlock()
		if resp.Status == plugin.StatusDone {
			t.Fatalf("restart succeeded while removal was pending: %+v", resp)
		}
		if !strings.Contains(resp.Error, "removal") {
			t.Fatalf("pending-removal refusal had unexpected message: %s", resp.Error)
		}
		requireTxLockFree(t, s)
		assertStillDisabled(t, pm, name, initialStop.Detail)

		// Refusal 4: config transaction lock held.
		if !s.txLock.tryAcquire() {
			t.Fatal("txLock unexpectedly held before test")
		}
		resp = restartByCommand(t, &CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, name))
		s.txLock.release()
		if resp.Status == plugin.StatusDone {
			t.Fatalf("restart succeeded while txLock was held: %+v", resp)
		}
		if !strings.Contains(resp.Error, "config transaction is in flight") {
			t.Fatalf("txLock refusal had unexpected message: %s", resp.Error)
		}
		assertStillDisabled(t, pm, name, initialStop.Detail)

		// Non-operator Respawn still observes disabled state and counters intact.
		if _, err := pm.Respawn(name); !errors.Is(err, process.ErrProcessDisabled) {
			t.Fatalf("non-operator Respawn after refusals = %v, want ErrProcessDisabled", err)
		}

		// Once all blockers are cleared, an admitted operator restart lifts the
		// disable atomically and brings the plugin back to running.
		resp = restartByCommand(t, &CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, name))
		if resp.Status != plugin.StatusDone {
			t.Fatalf("admitted restart after clearing refusals failed: %+v", resp)
		}
		requireTxLockFree(t, s)
		if pm.Disabled(name) {
			t.Fatal("admitted restart did not clear disabled state")
		}
		if stop, down := pm.Stopped(name); down {
			t.Fatalf("admitted restart left stop record behind: %+v", stop)
		}
		if proc := pm.GetProcess(name); proc == nil || !proc.Running() {
			t.Fatal("plugin is not running after admitted restart")
		}
	})

	t.Run("daemon stopping refusal keeps disable and stop record", func(t *testing.T) {
		const name = "restart-refused-stopping"
		s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
		pm := s.procManager.Load()
		initialStop := exhaustRespawnLimit(t, s, pm, name)

		pm.RefuseRespawns()
		resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, name))
		if resp.Status == plugin.StatusDone {
			t.Fatalf("restart succeeded while daemon was stopping: %+v", resp)
		}
		requireTxLockFree(t, s)
		assertStillDisabled(t, pm, name, initialStop.Detail)
	})
}

// TestRestartRefusedDuringBootWindow verifies that `request plugin restart` is
// refused immediately while plugin startup is still in progress (before
// signalStartupComplete), leaves no owed cleanup or replaced process, and
// succeeds once startup completes.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-9 --
// handlePluginRestart refuses when !s.startupComplete() with an operator error
// naming ze show doctor and the restart command, and allows the restart once
// startup completes.
// PREVENTS: an SSH restart arriving right after bgp Stage 2 opens the SSH
// listener from racing Phase 1 Step (d) supervision or later startup phases.
func TestRestartRefusedDuringBootWindow(t *testing.T) {
	const name = "restart-boot-window"
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	if err := registry.Register(registry.Registration{
		Name:        name,
		Description: "boot-window restart refusal test plugin",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.stopParticipantFilterPushWorker()
			s.wg.Wait()
		}
	})
	s, _ = newLifecycleStartupServer(t)
	if err := s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}); err != nil {
		t.Fatalf("runPluginPhase: %v", err)
	}
	pm := s.procManager.Load()
	initial := pm.GetProcess(name)
	if initial == nil || !initial.Running() {
		t.Fatalf("plugin %s is not running after Phase 1", name)
	}

	// Boot is still in progress: signalStartupComplete has not run yet.
	resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, name))
	if resp.Status == plugin.StatusDone {
		t.Fatalf("restart during boot window was accepted: %+v", resp)
	}
	if !strings.Contains(resp.Error, "cannot be restarted while startup is in progress") {
		t.Fatalf("boot-window refusal missing expected problem text: %s", resp.Error)
	}
	if !strings.Contains(resp.Error, "ze show doctor") {
		t.Fatalf("boot-window refusal does not name ze show doctor: %s", resp.Error)
	}
	if !strings.Contains(resp.Error, process.RestartCommand(name)) {
		t.Fatalf("boot-window refusal does not name the restart command: %s", resp.Error)
	}
	if after := pm.GetProcess(name); after != initial {
		t.Fatalf("refused boot-window restart replaced the process: got %p, want %p", after, initial)
	}
	if s.lateCleanups.owed(name) {
		t.Fatal("refused boot-window restart left a cleanup-owed record")
	}
	requireTxLockFree(t, s)

	// Once startup completes, the restart succeeds cleanly.
	s.signalStartupComplete()
	resp = restartByCommand(t, &CommandContext{Server: s}, commandtest.Args(pluginRestartLeaf, name))
	if resp.Status != plugin.StatusDone {
		t.Fatalf("restart after startup completed failed: %+v", resp)
	}
	after := pm.GetProcess(name)
	if after == nil || after == initial || !after.Running() {
		t.Fatalf("plugin did not restart cleanly after startup completed: %p (was %p)", after, initial)
	}
	if s.lateCleanups.owed(name) {
		t.Fatal("completed post-boot restart left a cleanup-owed record")
	}
	requireTxLockFree(t, s)
}
