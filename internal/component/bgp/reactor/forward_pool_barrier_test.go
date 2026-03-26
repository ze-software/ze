package reactor

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFwdPool_Barrier_DrainsAll verifies that Barrier blocks until all workers
// have processed their queued items (channel + overflow).
//
// VALIDATES: AC-1 — Barrier waits until all workers have drained
// PREVENTS: Barrier returning before items are processed.
func TestFwdPool_Barrier_DrainsAll(t *testing.T) {
	var processed atomic.Int32
	var handlerGate sync.WaitGroup
	handlerGate.Add(1) // block handler until we release

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		handlerGate.Wait() // block until test releases
		for range items {
			processed.Add(1)
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Dispatch two items — handler is blocked so they queue up
	pool.Dispatch(key, fwdItem{})
	pool.Dispatch(key, fwdItem{})

	// Start barrier in goroutine
	barrierDone := make(chan error, 1)
	go func() {
		barrierDone <- pool.Barrier(context.Background())
	}()

	// Barrier should not complete yet (handler is blocked)
	select {
	case <-barrierDone:
		t.Fatal("barrier returned before handler processed items")
	case <-time.After(50 * time.Millisecond):
		// expected — barrier is still waiting
	}

	// Release the handler
	handlerGate.Done()

	// Barrier should complete
	select {
	case err := <-barrierDone:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("barrier did not complete after handler released")
	}

	// All items processed before barrier returned
	assert.GreaterOrEqual(t, processed.Load(), int32(2))
}

// TestFwdPool_Barrier_NoWorkers verifies that Barrier returns immediately
// when no workers exist.
//
// VALIDATES: AC-3 — Barrier returns immediately with no workers
// PREVENTS: Barrier hanging on empty pool.
func TestFwdPool_Barrier_NoWorkers(t *testing.T) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	defer pool.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := pool.Barrier(ctx)
	assert.NoError(t, err)
}

// TestFwdPool_Barrier_Filtered verifies that Barrier with a filter only
// waits for matching workers, returning while non-matching workers still have items.
//
// VALIDATES: AC-2 — Filtered barrier only waits for targeted peer
// PREVENTS: Barrier blocking on unrelated peers.
func TestFwdPool_Barrier_Filtered(t *testing.T) {
	var gate sync.WaitGroup
	gate.Add(1) // block handler for peer2

	pool := newFwdPool(func(key fwdKey, _ []fwdItem) {
		if key.peerAddr == netip.MustParseAddrPort("2.2.2.2:179") {
			gate.Wait() // block peer2's handler
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	defer func() {
		gate.Done() // release peer2 handler before stopping pool
		pool.Stop()
	}()

	// Dispatch to both peers
	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("2.2.2.2:179")}, fwdItem{})

	// Wait for peer1's item to be processed
	require.Eventually(t, func() bool {
		return pool.WorkerCount() >= 2
	}, time.Second, time.Millisecond)

	// Barrier for peer1 only — should complete even though peer2 is blocked
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pool.BarrierPeer(ctx, netip.MustParseAddrPort("1.1.1.1:179"))
	assert.NoError(t, err)
}

// TestFwdPool_Barrier_ContextCancel verifies that Barrier respects context
// cancellation and returns the context error.
//
// VALIDATES: AC-4 — Context cancellation returns context error
// PREVENTS: Barrier blocking forever on stuck workers.
func TestFwdPool_Barrier_ContextCancel(t *testing.T) {
	var gate sync.WaitGroup
	gate.Add(1) // block handler forever

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		gate.Wait()
	}, fwdPoolConfig{chanSize: 8, idleTimeout: 5 * time.Second})
	defer func() {
		gate.Done()
		pool.Stop()
	}()

	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := pool.Barrier(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestFwdPool_Barrier_StoppedPool verifies that Barrier returns immediately
// on a stopped pool.
//
// VALIDATES: AC-3 variant — stopped pool returns without blocking
// PREVENTS: Barrier hanging during shutdown.
func TestFwdPool_Barrier_StoppedPool(t *testing.T) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 8, idleTimeout: 5 * time.Second})

	pool.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := pool.Barrier(ctx)
	assert.NoError(t, err)
}

// TestFwdPool_Barrier_WithOverflow verifies that Barrier waits for overflow
// buffer items to be drained, not just the channel.
//
// VALIDATES: AC-1 — Barrier drains overflow buffer too
// PREVENTS: Barrier returning while overflow items are pending.
func TestFwdPool_Barrier_WithOverflow(t *testing.T) {
	var processed atomic.Int32
	var handlerGate sync.WaitGroup
	handlerGate.Add(1)

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		handlerGate.Wait()
		for range items {
			processed.Add(1)
		}
	}, fwdPoolConfig{chanSize: 2, idleTimeout: 5 * time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Fill the channel (size=2)
	pool.TryDispatch(key, fwdItem{})
	pool.TryDispatch(key, fwdItem{})

	// These go to overflow
	pool.DispatchOverflow(key, fwdItem{})
	pool.DispatchOverflow(key, fwdItem{})

	// Start barrier
	barrierDone := make(chan error, 1)
	go func() {
		barrierDone <- pool.Barrier(context.Background())
	}()

	// Barrier should not complete yet
	select {
	case <-barrierDone:
		t.Fatal("barrier returned before overflow items processed")
	case <-time.After(50 * time.Millisecond):
	}

	// Release handler
	handlerGate.Done()

	// Barrier should complete after all items (channel + overflow) processed
	select {
	case err := <-barrierDone:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("barrier did not complete after handler released")
	}

	assert.GreaterOrEqual(t, processed.Load(), int32(4))
}
