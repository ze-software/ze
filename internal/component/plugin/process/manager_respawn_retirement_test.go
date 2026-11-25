// Design: docs/architecture/api/process-protocol.md -- exact restart ownership.
// Related: manager.go -- Respawn retires the outgoing generation before Stop.
package process

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/report"
)

// An intentionally retiring generation ceases to be current before its real
// engine cleanup begins, but stays physically owned until that cleanup joins.
// MUTATION: leave the outgoing process in pm.processes across old.Stop/Wait.
func TestRespawnWithdrawsRetiringGenerationBeforeCleanup(t *testing.T) {
	const name = "test-respawn-retiring-generation"
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var generations atomic.Int64
	registerRunEngine(t, name, func(conn net.Conn) int {
		generation := generations.Add(1)
		drainUntilClosed(conn)
		if generation == 1 {
			close(entered)
			<-release
		}
		return 0
	})
	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	old := pm.GetProcess(name)
	if old == nil {
		t.Fatal("initial generation was not published")
	}
	type result struct {
		proc *Process
		err  error
	}
	respawned, finished := make(chan result, 1), make(chan struct{})
	t.Cleanup(func() {
		unblock()
		pm.Stop()
		select {
		case <-finished:
		case <-time.After(pluginStopGrace + time.Second):
			t.Error("respawn did not finish during cleanup")
		}
		ctx, cancel := context.WithTimeout(context.Background(), processStopBudget)
		defer cancel()
		if err := old.Wait(ctx); err != nil {
			t.Errorf("join retired generation: %v", err)
		}
	})
	go func() { proc, err := pm.Respawn(name); respawned <- result{proc, err}; close(finished) }()
	select {
	case <-entered:
	case <-time.After(old.StopJoinBound() + time.Second):
		t.Fatal("respawn did not enter the outgoing engine's cleanup")
	}
	if current := pm.GetProcess(name); current != nil {
		t.Errorf("retiring generation still owns the current process slot: %p", current)
	}
	pm.mu.RLock()
	_, retained := pm.starting[old]
	pm.mu.RUnlock()
	if !retained {
		t.Error("unpublished outgoing generation has no retained physical owner")
	}
	select {
	case <-old.EngineDone():
		t.Error("held cleanup was reported complete")
	default:
	}
	unblock()
	select {
	case outcome := <-respawned:
		if outcome.err != nil {
			t.Fatalf("respawn after released cleanup: %v", outcome.err)
		}
		if outcome.proc == nil || outcome.proc == old || pm.GetProcess(name) != outcome.proc {
			t.Fatal("respawn did not publish its distinct started replacement")
		}
	case <-time.After(old.StopJoinBound() + time.Second):
		t.Fatal("respawn did not finish after cleanup was released")
	}
	pm.mu.RLock()
	_, retained = pm.starting[old]
	pm.mu.RUnlock()
	if retained {
		t.Error("joined outgoing generation remained retained")
	}
}

// A real outgoing engine deliberately outlasts Respawn's own join bound.
// Terminal shutdown must still join that exact retired owner before it closes
// the provider, even though a different process now occupies the same name.
// MUTATION: discard the outgoing owner after its respawn join times out.
func TestOrderedStopRetainsRespawnOutgoingGeneration(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	const provider = "test-respawn-retired-provider"
	const consumer = "test-respawn-retired-consumer"
	entered, release, fenced := make(chan struct{}), make(chan struct{}), make(chan struct{})
	providerStopped, stopped := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var oldRef atomic.Pointer[Process]
	var providerSawCompletedOld atomic.Bool
	var generations atomic.Int64
	registerRunEngine(t, provider, func(conn net.Conn) int {
		drainUntilClosed(conn)
		if old := oldRef.Load(); old != nil {
			select {
			case <-old.EngineDone():
				providerSawCompletedOld.Store(true)
			default:
			}
		}
		close(providerStopped)
		return 0
	})
	registerRunEngine(t, consumer, func(conn net.Conn) int {
		generation := generations.Add(1)
		drainUntilClosed(conn)
		if generation == 1 {
			close(entered)
			<-release
		}
		return 0
	})
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	pm := NewProcessManager([]plugin.PluginConfig{
		{Name: provider, Internal: true, Encoder: "json"},
		{Name: consumer, Internal: true, Encoder: "json"},
	})
	wrapped := &shutdownCloseListener{Listener: listener, onClose: func() {
		if pm.shutdown.Load() == nil {
			t.Error("acceptor closed before terminal admission was fenced")
		}
		close(fenced)
	}}
	acceptor := ipc.NewPluginAcceptor(wrapped, "a-plugin-secret-of-at-least-32-chars", nil)
	pm.SetAcceptor(acceptor)
	t.Cleanup(acceptor.Stop)
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	old := pm.GetProcess(consumer)
	if old == nil {
		t.Fatal("initial consumer was not published")
	}
	oldRef.Store(old)
	type result struct {
		proc *Process
		err  error
	}
	respawned, finished := make(chan result, 1), make(chan struct{})
	stopStarted := false
	t.Cleanup(func() {
		unblock()
		if stopStarted {
			select {
			case <-stopped:
			case <-time.After(pluginStopGrace + time.Second):
				t.Error("ordered stop did not finish during cleanup")
			}
		} else {
			pm.Stop()
		}
		select {
		case <-finished:
		case <-time.After(pluginStopGrace + time.Second):
			t.Error("respawn did not finish during cleanup")
		}
		ctx, cancel := context.WithTimeout(context.Background(), processStopBudget)
		defer cancel()
		if err := old.Wait(ctx); err != nil {
			t.Errorf("join retired generation: %v", err)
		}
	})
	go func() { proc, err := pm.Respawn(consumer); respawned <- result{proc, err}; close(finished) }()
	select {
	case <-entered:
	case <-time.After(old.StopJoinBound() + time.Second):
		t.Fatal("outgoing consumer did not enter cleanup")
	}
	var replacement *Process
	select {
	case outcome := <-respawned:
		if outcome.err != nil {
			t.Fatalf("respawn after its outgoing join bound: %v", outcome.err)
		}
		replacement = outcome.proc
	case <-time.After(old.StopJoinBound() + time.Second):
		t.Fatal("respawn did not observe its existing outgoing join bound")
	}
	if replacement == nil || replacement == old || pm.GetProcess(consumer) != replacement {
		t.Fatal("a distinct replacement was not published after the outgoing join bound")
	}
	select {
	case <-old.EngineDone():
		t.Fatal("held outgoing engine was reported complete")
	default:
	}
	stopStarted = true
	go func() {
		pm.StopInOrder(func([]plugin.PluginConfig) [][]string { return [][]string{{provider}, {consumer}} })
		close(stopped)
	}()
	select {
	case <-fenced:
	case <-time.After(time.Second):
		t.Fatal("ordered stop did not reach its real acceptor fence")
	}
	// This bounds the assertion that a deliberately held physical owner keeps
	// providers live. It is not a wait for engine completion; release below is.
	select {
	case <-providerStopped:
		t.Error("provider stopped while the retired outgoing cleanup was held")
	case <-stopped:
		t.Error("ordered stop returned while the retired outgoing cleanup was held")
	case <-time.After(time.Second):
	}
	unblock()
	select {
	case <-stopped:
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("ordered stop did not join released outgoing cleanup")
	}
	if !providerSawCompletedOld.Load() {
		t.Error("provider stopped before the actual retired engine completed")
	}
	if late := pm.CleanupLate(); len(late) != 0 {
		t.Errorf("joined retired owner reported late: %v", late)
	}
	if _, err := pm.Respawn(consumer); !errors.Is(err, ErrDaemonStopping) {
		t.Errorf("terminal manager admitted another replacement: %v", err)
	}
}

// TestRespawnDoesNotBlockReadersOrUnrelatedRespawn verifies that while one
// plugin's Respawn is blocked waiting on its outgoing process join, readers of
// ProcessManager and Respawn of an unrelated plugin name complete promptly,
// while a second Respawn of the same name remains serialized.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-3 --
// per-name respawning serialization and unlocked join/start let GetProcess,
// AllProcesses, ConfiguredNames, ConfiguredConfigs, and Respawn("other") answer
// well inside a slow plugin's join/connect-back window, while GetProcess("slow")
// immediately returns nil (!= old) for applyFailurePolicy's generation check.
// PREVENTS: a slow or stalled replacement blocking ProcessManager readers or
// unrelated plugin respawns behind a manager-wide lock.
func TestRespawnDoesNotBlockReadersOrUnrelatedRespawn(t *testing.T) {
	const (
		slowName  = "test-respawn-nonblocking-slow"
		otherName = "test-respawn-nonblocking-other"
	)
	origMargin := respawnJoinMargin
	respawnJoinMargin = 5 * time.Second
	t.Cleanup(func() { respawnJoinMargin = origMargin })

	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var slowGenerations atomic.Int64
	registerRunEngine(t, slowName, func(conn net.Conn) int {
		generation := slowGenerations.Add(1)
		drainUntilClosed(conn)
		if generation == 1 {
			close(entered)
			<-release
		}
		return 0
	})
	registerRunEngine(t, otherName, func(conn net.Conn) int {
		drainUntilClosed(conn)
		return 0
	})

	pm := NewProcessManager([]plugin.PluginConfig{
		{Name: slowName, Internal: true, Encoder: "json"},
		{Name: otherName, Internal: true, Encoder: "json"},
	})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	slowOld := pm.GetProcess(slowName)
	otherOld := pm.GetProcess(otherName)
	if slowOld == nil || otherOld == nil {
		t.Fatal("initial processes were not published")
	}

	type respawnOutcome struct {
		proc *Process
		err  error
	}
	slow1Ch := make(chan respawnOutcome, 1)
	slow2Ch := make(chan respawnOutcome, 1)
	var wg sync.WaitGroup
	t.Cleanup(func() {
		unblock()
		pm.Stop()
		wg.Wait()
		ctx, cancel := context.WithTimeout(context.Background(), processStopBudget)
		defer cancel()
		if err := slowOld.Wait(ctx); err != nil {
			t.Errorf("join retired slow generation: %v", err)
		}
	})

	wg.Go(func() {
		proc, err := pm.Respawn(slowName)
		slow1Ch <- respawnOutcome{proc: proc, err: err}
	})

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("slow respawn did not enter outgoing cleanup")
	}

	const promptBound = time.Second
	readersDone := make(chan struct{})
	wg.Go(func() {
		defer close(readersDone)
		if got := pm.GetProcess(slowName); got == slowOld || got != nil {
			t.Errorf("GetProcess(%q) during retiring join = %p, want nil (!= old %p)", slowName, got, slowOld)
		}
		if got := pm.GetProcess(otherName); got != otherOld {
			t.Errorf("GetProcess(%q) = %p, want %p", otherName, got, otherOld)
		}
		if procs := pm.AllProcesses(); len(procs) != 1 || procs[0] != otherOld {
			t.Errorf("AllProcesses() = %v, want only %q", procs, otherName)
		}
		if names := pm.ConfiguredNames(); len(names) != 2 {
			t.Errorf("ConfiguredNames() = %v, want 2 names", names)
		}
		cfgs := pm.ConfiguredConfigs()
		if len(cfgs) != 2 || cfgs[0].Name != otherName || cfgs[1].Name != slowName {
			t.Errorf("ConfiguredConfigs() = %+v, want sorted [%q %q]", cfgs, otherName, slowName)
		}
	})
	select {
	case <-readersDone:
	case <-time.After(promptBound):
		t.Fatal("ProcessManager readers blocked while slow Respawn waited on join")
	}

	otherCh := make(chan respawnOutcome, 1)
	wg.Go(func() {
		proc, err := pm.Respawn(otherName)
		otherCh <- respawnOutcome{proc: proc, err: err}
	})
	select {
	case outcome := <-otherCh:
		if outcome.err != nil {
			t.Fatalf("Respawn(%q) while %q was blocked: %v", otherName, slowName, outcome.err)
		}
		if outcome.proc == nil || outcome.proc == otherOld || pm.GetProcess(otherName) != outcome.proc {
			t.Fatal("unrelated Respawn did not publish a distinct replacement")
		}
	case <-time.After(promptBound):
		t.Fatalf("Respawn(%q) blocked behind %q's Respawn", otherName, slowName)
	}

	// Same-name Respawn MUST still serialize behind slow1 rather than racing it.
	wg.Go(func() {
		proc, err := pm.Respawn(slowName)
		slow2Ch <- respawnOutcome{proc: proc, err: err}
	})
	select {
	case outcome := <-slow1Ch:
		t.Fatalf("slow1 returned before release: %+v", outcome)
	default:
	}

	unblock()

	var slow1, slow2 respawnOutcome
	select {
	case slow1 = <-slow1Ch:
	case <-time.After(2 * time.Second):
		t.Fatal("first slow Respawn did not complete after release")
	}
	if slow1.err != nil {
		t.Fatalf("first slow Respawn: %v", slow1.err)
	}
	select {
	case slow2 = <-slow2Ch:
	case <-time.After(2 * time.Second):
		t.Fatal("second serialized slow Respawn did not complete after first finished")
	}
	if slow2.err != nil {
		t.Fatalf("second slow Respawn: %v", slow2.err)
	}
	if slow2.proc == nil || slow2.proc == slowOld || slow2.proc == slow1.proc {
		t.Fatalf("second slow Respawn did not publish a third distinct generation: old=%p slow1=%p slow2=%p",
			slowOld, slow1.proc, slow2.proc)
	}
	if pm.GetProcess(slowName) != slow2.proc {
		t.Fatalf("pm.GetProcess(%q) = %p, want %p", slowName, pm.GetProcess(slowName), slow2.proc)
	}
}

// TestRespawnInDaemonExitNoOverlap verifies that an in-daemon plugin engine
// whose exit path reads ProcessManager (GetProcess, AllProcesses,
// ConfiguredNames, ConfiguredConfigs) joins within StopJoinBound and raises no
// doctor-plugin-restart-overlap event.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-3 --
// releasing pm.mu before old.Stop/Wait lets an exiting in-daemon engine read
// ProcessManager without deadlocking its own join.
// PREVENTS: a false doctor-plugin-restart-overlap caused by an exiting engine
// blocking on pm.mu held by Respawn.
func TestRespawnInDaemonExitNoOverlap(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	const name = "test-respawn-indaemon-exit-reads-pm"
	var pmRef atomic.Pointer[ProcessManager]
	var oldRef atomic.Pointer[Process]
	var exitReadsCompleted atomic.Bool
	var sawWithdrawnSlot atomic.Bool
	var generations atomic.Int64

	registerRunEngine(t, name, func(conn net.Conn) int {
		generation := generations.Add(1)
		drainUntilClosed(conn)
		if generation == 1 {
			if mgr := pmRef.Load(); mgr != nil {
				if mgr.GetProcess(name) != oldRef.Load() {
					sawWithdrawnSlot.Store(true)
				}
				_ = mgr.AllProcesses()
				_ = mgr.ConfiguredNames()
				_ = mgr.ConfiguredConfigs()
				exitReadsCompleted.Store(true)
			}
		}
		return 0
	})

	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	pmRef.Store(pm)
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)

	old := pm.GetProcess(name)
	if old == nil {
		t.Fatal("initial process was not published")
	}
	oldRef.Store(old)

	replacement, err := pm.Respawn(name)
	if err != nil {
		t.Fatalf("Respawn: %v", err)
	}
	if replacement == nil || replacement == old {
		t.Fatalf("Respawn did not publish a distinct replacement: %p", replacement)
	}
	if !exitReadsCompleted.Load() {
		t.Fatal("exiting in-daemon engine did not finish its ProcessManager reads during join")
	}
	if !sawWithdrawnSlot.Load() {
		t.Fatal("exiting in-daemon engine still saw itself in pm.GetProcess during join")
	}
	if events := restartOverlapEvents(); len(events) != 0 {
		t.Fatalf("unexpected doctor-plugin-restart-overlap events: %+v", events)
	}
}
