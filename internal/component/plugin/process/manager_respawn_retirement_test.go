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
