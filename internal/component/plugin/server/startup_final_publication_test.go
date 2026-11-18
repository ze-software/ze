// Design: docs/architecture/api/process-protocol.md -- terminal startup completion.
// Related: startup.go -- finishStartupCompletion; last_stop.go -- clearLastStop.
package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// startupSettlementStore pauses the actual last-stop deletion before the
// underlying private store is changed. Every other operation still reaches
// that real store. Its owner MUST release the gate and join startup before
// unregistering or closing the borrowed store.
type startupSettlementStore struct {
	statestore.Storage
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (store *startupSettlementStore) RemoveKey(key string) error {
	if key == zefs.KeyPluginLastStop.Pattern {
		store.once.Do(func() { close(store.entered) })
		<-store.release
	}
	return store.Storage.RemoveKey(key)
}

// TestStartupPublicationSettlesBeforeLaterStop proves an admitted successful
// publication keeps Stop outside its final settlement, rather than merely
// noticing Stop after a readiness callback. Start reaches the real durable
// deletion of an earlier stop record and remains held there while Stop tries
// to run. Releasing that deletion must publish success before the later Stop,
// and both operations must then join normally.
// MUTATION: remove only lifecycleMu.Lock and its deferred Unlock from
// finishStartupCompletion, retaining every terminal check and settlement.
func TestStartupPublicationSettlesBeforeLaterStop(t *testing.T) {
	useLastStopStore(t)
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	if err := recordLastStopIn(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), LastStopEntry{
		Reason: LastStopCleanupLate,
		Plugin: "previous-engine",
		Detail: "previous run cleanup remained unfinished",
	}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := ReadLastStop(); err != nil || !found {
		t.Fatalf("seed previous stop: found %v, err %v", found, err)
	}
	underlying := statestore.Store()
	release := make(chan struct{})
	gate := &startupSettlementStore{
		Storage: underlying,
		entered: make(chan struct{}),
		release: release,
	}
	statestore.SetStore(gate)
	// Registered before server cleanup, so the latter joins both operations
	// before SetStore waits for any remaining borrowed store operation.
	t.Cleanup(func() { statestore.SetStore(underlying) })
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	started := make(chan error, 1)
	startDone, stopDone := make(chan struct{}), make(chan struct{})
	stopLaunched := false
	t.Cleanup(func() {
		// Fatal must never leave the real store call or Stop behind its gate.
		unblock()
		s.Stop()
		select {
		case <-startDone:
		case <-time.After(failureWait):
			t.Error("startup did not join after releasing its settlement")
		}
		if stopLaunched {
			select {
			case <-stopDone:
			case <-time.After(failureWait):
				t.Error("Stop did not join after releasing startup settlement")
			}
		}
	})
	go func() {
		started <- s.Start()
		close(startDone)
	}()
	select {
	case <-gate.entered:
	case <-time.After(failureWait):
		t.Fatal("actual startup did not reach last-stop settlement")
	}
	if !s.Running() {
		t.Fatal("server was not live when startup entered settlement")
	}
	stopAttempted := make(chan struct{})
	stopLaunched = true
	go func() {
		close(stopAttempted)
		s.Stop()
		close(stopDone)
	}()
	<-stopAttempted
	// The store gate, not this timer, controls settlement. This single bounded
	// interval observes that Stop cannot complete or cancel dispatch while that
	// concrete publication remains held. It never polls for a worker to finish.
	held := time.NewTimer(time.Second)
	defer held.Stop()
	select {
	case <-stopDone:
		t.Error("Stop completed while successful startup settlement was still held")
	case <-s.Context().Done():
		t.Error("Stop canceled dispatch while successful startup settlement was still held")
	case <-held.C:
	}
	unblock()
	select {
	case err := <-started:
		if err != nil {
			t.Fatalf("accepted empty startup returned %v", err)
		}
	case <-time.After(failureWait):
		t.Fatal("Start did not return after its actual settlement was released")
	}
	ctx, cancel := context.WithTimeout(context.Background(), failureWait)
	defer cancel()
	if err := s.WaitForStartupComplete(ctx); err != nil {
		t.Fatalf("startup admitted before the later Stop = %v, want historical success", err)
	}
	select {
	case <-stopDone:
	case <-ctx.Done():
		t.Fatal("later Stop did not finish after startup publication")
	}
	if s.Running() || s.Context().Err() == nil {
		t.Fatal("later Stop did not end the live server lifetime")
	}
	if _, found, err := ReadLastStop(); err != nil || found {
		t.Fatalf("completed startup settlement: stale record found %v, err %v", found, err)
	}
	if err := s.WaitForStartupComplete(ctx); err != nil {
		t.Fatalf("later Stop changed historical startup success: %v", err)
	}
}
