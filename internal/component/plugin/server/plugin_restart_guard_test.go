package server

import (
	"context"
	"errors"
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
