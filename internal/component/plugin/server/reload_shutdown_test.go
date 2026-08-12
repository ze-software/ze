// Related: reload.go -- txLock, stopTransaction
// Related: server.go -- Server.Stop
// Related: ../../config/transaction/orchestrator.go -- ErrShutdown

package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
)

// gatedReactor blocks inside GetConfigTree, which reloadConfig calls after it
// has taken the transaction lock and published its cancel handle. It puts the
// server in the state Stop finds when a reload is in flight, and it needs no
// live plugin process.
type gatedReactor struct {
	*mockReactor
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *gatedReactor) GetConfigTree() map[string]any {
	g.once.Do(func() { close(g.entered) })
	<-g.release
	return map[string]any{"bgp": map[string]any{"asn": 1}}
}

// TestStopWaitsForInFlightReload proves Stop does not close the plugin
// connections while a config transaction still owns them.
//
// VALIDATES: shutdown is ordered after the in-flight transaction.
// PREVENTS: the transaction reading its own connections' closure as a wave of
// crashed plugins, electing a rollback, and restarting plugins mid-exit.
func TestStopWaitsForInFlightReload(t *testing.T) {
	r := &gatedReactor{
		mockReactor: &mockReactor{},
		entered:     make(chan struct{}),
		release:     make(chan struct{}),
	}
	s, err := NewServer(&ServerConfig{}, r)
	require.NoError(t, err)
	require.NoError(t, s.Start())

	reloadDone := make(chan error, 1)
	go func() {
		reloadDone <- s.ReloadConfig(context.Background(), map[string]any{"bgp": map[string]any{"asn": 2}})
	}()
	<-r.entered

	stopReturned := make(chan struct{})
	go func() {
		s.Stop()
		close(stopReturned)
	}()

	select {
	case <-stopReturned:
		t.Fatal("Stop returned while a config transaction was still in flight")
	case <-time.After(150 * time.Millisecond):
	}

	close(r.release)

	select {
	case <-reloadDone:
	case <-time.After(5 * time.Second):
		t.Fatal("reload did not finish")
	}
	select {
	case <-stopReturned:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after the transaction unwound")
	}
}

// TestStopCancelsTransactionWithShutdownCause proves Stop names the reason.
// The cause is what the orchestrator reads to skip its rollback, so a bare
// cancellation would leave the rollback-and-restart path running.
//
// VALIDATES: Stop cancels with transaction.ErrShutdown.
// PREVENTS: shutdown looking like an ordinary cancellation to the orchestrator.
func TestStopCancelsTransactionWithShutdownCause(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, &mockReactor{})
	require.NoError(t, err)
	require.True(t, s.txLock.tryAcquire(), "transaction lock must be free")

	causeCh := make(chan error, 1)
	s.txLock.setCancel(func(cause error) { causeCh <- cause })

	stopReturned := make(chan struct{})
	go func() {
		s.Stop()
		close(stopReturned)
	}()

	select {
	case cause := <-causeCh:
		if !errors.Is(cause, transaction.ErrShutdown) {
			t.Fatalf("cancel cause = %v, want %v", cause, transaction.ErrShutdown)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not cancel the in-flight transaction")
	}

	s.txLock.release()

	select {
	case <-stopReturned:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after the transaction released the lock")
	}
}

// TestStopTransactionGivesUpAfterGrace bounds the wait. A shutdown that hangs
// forever on a stuck reload is worse than the log noise the wait removes.
//
// VALIDATES: stopTransaction returns once the grace expires.
// PREVENTS: an unbounded shutdown.
func TestStopTransactionGivesUpAfterGrace(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, &mockReactor{})
	require.NoError(t, err)
	require.True(t, s.txLock.tryAcquire(), "transaction lock must be free")
	s.txLock.setCancel(func(error) {}) // a transaction that ignores cancellation

	start := time.Now()
	s.stopTransaction(50 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("stopTransaction blocked for %v on a stuck transaction", elapsed)
	}
}
