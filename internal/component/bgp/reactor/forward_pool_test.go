package reactor

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// TestFwdPool_LazyCreation verifies workers are created on first Dispatch
// and reused for subsequent dispatches to the same peer.
//
// VALIDATES: AC-1 (workers created per destination peer)
// PREVENTS: Eager worker creation wasting goroutines for idle peers.
func TestFwdPool_LazyCreation(t *testing.T) {
	handled := make(chan struct{}, 10)
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		handled <- struct{}{}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	assert.Equal(t, 0, pool.WorkerCount())

	// First dispatch creates worker
	ok := pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	require.True(t, ok)
	<-handled
	assert.Equal(t, 1, pool.WorkerCount())

	// Same peer reuses worker
	ok = pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	require.True(t, ok)
	<-handled
	assert.Equal(t, 1, pool.WorkerCount())

	// Different peer creates second worker
	ok = pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("2.2.2.2:179")}, fwdItem{})
	require.True(t, ok)
	<-handled
	assert.Equal(t, 2, pool.WorkerCount())
}

// TestFwdPool_IdleTimeout verifies workers exit after idle period
// and WorkerCount decrements.
//
// VALIDATES: AC-6 (worker idle timeout)
// PREVENTS: Goroutine leaks from workers that never exit.
func TestFwdPool_IdleTimeout(t *testing.T) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 8, idleTimeout: 50 * time.Millisecond})
	defer pool.Stop()

	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	require.Eventually(t, func() bool {
		return pool.WorkerCount() == 1
	}, time.Second, time.Millisecond, "worker should be spawned")

	// Wait for idle timeout
	require.Eventually(t, func() bool {
		return pool.WorkerCount() == 0
	}, time.Second, time.Millisecond, "worker should exit after idle timeout")
}

// TestFwdPool_Stop verifies all workers drain and exit on Stop.
//
// VALIDATES: AC-7 (pool shutdown)
// PREVENTS: Goroutine leaks on reactor shutdown.
func TestFwdPool_Stop(t *testing.T) {
	blocker := make(chan struct{})
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})

	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("2.2.2.2:179")}, fwdItem{})
	require.Eventually(t, func() bool {
		return pool.WorkerCount() == 2
	}, time.Second, time.Millisecond, "both workers should be spawned")

	close(blocker) // Unblock handlers
	pool.Stop()
	assert.Equal(t, 0, pool.WorkerCount())
}

// TestFwdPool_DispatchAfterStop verifies Dispatch returns false
// when pool is already stopped. Caller is responsible for cleanup.
//
// VALIDATES: AC-8 (dispatch to stopped pool)
// PREVENTS: Sends to closed channels or blocked dispatches after shutdown.
func TestFwdPool_DispatchAfterStop(t *testing.T) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	pool.Stop()

	ok := pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	assert.False(t, ok)
}

// TestFwdPool_FIFOPerPeer verifies items dispatched to the same peer
// are processed in dispatch order.
//
// VALIDATES: AC-9 (FIFO ordering per peer)
// PREVENTS: Out-of-order UPDATE delivery to a single peer.
func TestFwdPool_FIFOPerPeer(t *testing.T) {
	orderCh := make(chan int, 10)
	var counter atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for range items {
			orderCh <- int(counter.Add(1))
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	for range 5 {
		pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	}

	for i := 1; i <= 5; i++ {
		var got int
		require.Eventually(t, func() bool {
			select {
			case got = <-orderCh:
				return true
			default:
				return false
			}
		}, 2*time.Second, time.Millisecond, "timeout waiting for item %d", i)
		assert.Equal(t, i, got)
	}
}

// TestFwdPool_ParallelPeers verifies a slow peer does not block
// delivery to a fast peer.
//
// VALIDATES: AC-1 (slow peer independence)
// PREVENTS: Head-of-line blocking across destination peers.
func TestFwdPool_ParallelPeers(t *testing.T) {
	slowCh := make(chan struct{})
	fastDone := make(chan struct{})
	slowStarted := make(chan struct{}, 1)

	slowAddr := netip.MustParseAddrPort("10.0.0.1:179")
	fastAddr := netip.MustParseAddrPort("10.0.0.2:179")

	pool := newFwdPool(func(key fwdKey, _ []fwdItem) {
		if key.peerAddr == slowAddr {
			select {
			case slowStarted <- struct{}{}:
			default:
			}
			<-slowCh
		} else {
			close(fastDone)
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	// Dispatch to slow peer first
	pool.Dispatch(fwdKey{peerAddr: slowAddr}, fwdItem{})
	<-slowStarted // Wait for slow worker to be blocking

	// Dispatch to fast peer — should complete without waiting for slow
	pool.Dispatch(fwdKey{peerAddr: fastAddr}, fwdItem{})

	require.Eventually(t, func() bool {
		select {
		case <-fastDone:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "fast peer blocked by slow peer")

	close(slowCh)
}

// TestFwdPool_BackpressureBehavior verifies Dispatch blocks when
// the per-peer channel is full and does not drop items.
//
// VALIDATES: AC-10 (backpressure on full channel)
// PREVENTS: Silent message drops under load.
func TestFwdPool_BackpressureBehavior(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Fill: 1 item in handler + 2 in channel = 3 dispatches
	pool.Dispatch(key, fwdItem{})
	<-handlerStarted
	pool.Dispatch(key, fwdItem{})
	pool.Dispatch(key, fwdItem{})

	// Next dispatch should block
	dispatched := make(chan bool, 1)
	go func() {
		ok := pool.Dispatch(key, fwdItem{})
		dispatched <- ok
	}()

	require.Never(t, func() bool {
		select {
		case <-dispatched:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond, "dispatch should have blocked on full channel")

	close(blocker) // Unblock processing

	var ok2 bool
	require.Eventually(t, func() bool {
		select {
		case ok2 = <-dispatched:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "dispatch should have unblocked after handler drained")
	assert.True(t, ok2)
}

// TestFwdPool_HandlerError verifies panics in the handler don't kill the worker
// and the done callback is still called (for Release).
//
// VALIDATES: AC-11 (error in handler; Release still called)
// PREVENTS: Goroutine death on handler panic; cache entry leaks.
func TestFwdPool_HandlerError(t *testing.T) {
	var handled atomic.Int32
	var doneCalled atomic.Int32

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		n := handled.Add(1)
		if n == 1 {
			panic("test panic")
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}
	doneFunc := func() { doneCalled.Add(1) }

	// First dispatch panics
	pool.Dispatch(key, fwdItem{done: doneFunc})

	// done callback should still be called despite panic (goroutine needs time under race detector)
	require.Eventually(t, func() bool { return doneCalled.Load() >= 1 },
		time.Second, 10*time.Millisecond, "done callback should be called after panic")

	// Second dispatch should still work (worker survived panic)
	pool.Dispatch(key, fwdItem{done: doneFunc})

	require.Eventually(t, func() bool { return handled.Load() >= 2 },
		time.Second, 10*time.Millisecond, "worker should survive panic and handle second dispatch")
	require.Eventually(t, func() bool { return doneCalled.Load() >= 2 },
		time.Second, 10*time.Millisecond, "done callback should be called for both dispatches")
	assert.Equal(t, 1, pool.WorkerCount())
}

// TestFwdPool_StopUnblocksDispatch verifies Stop unblocks a Dispatch
// that is blocked on a full channel.
//
// VALIDATES: AC-7 (stop unblocks blocked dispatch via stopCh)
// PREVENTS: Deadlock during reactor shutdown when workers are backed up.
//
// The invariant is "the blocked Dispatch returns after Stop is called" --
// NOT "Dispatch returns false specifically." When Stop closes stopCh, the
// blocked Dispatch's select has TWO ready cases: (a) stopCh closed, return
// false, or (b) the worker has drained item2 and w.ch now has a free slot,
// send item3, return true. Go's select picks a ready case at random. Both
// outcomes are valid for AC-7; which one wins is scheduling-dependent.
// The previous `assert.False(t, stopOk)` asserted case (a) specifically,
// which made the test flake under parallel -race load whenever the worker
// drained item2 just before Stop closed stopCh.
func TestFwdPool_StopUnblocksDispatch(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 1, idleTimeout: time.Second})

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}
	pool.Dispatch(key, fwdItem{}) // In handler
	<-handlerStarted
	pool.Dispatch(key, fwdItem{}) // In channel

	// This dispatch should block on the full worker channel.
	go func() {
		_ = pool.Dispatch(key, fwdItem{})
	}()

	require.Eventually(t, func() bool {
		pool.mu.Lock()
		w := pool.workers[key]
		pending := w != nil && w.pending.Load() == 1
		pool.mu.Unlock()
		return pending
	}, time.Second, time.Millisecond, "dispatch should be blocked on full channel")

	// Stop should unblock the blocked dispatch. Whether Dispatch returns
	// true (worker drained, channel accepted item) or false (stopCh
	// unblocked first) is scheduling-dependent and both are valid
	// outcomes for AC-7. The key invariant is that the Dispatch does
	// return (no deadlock).
	close(blocker)
	stopDone := make(chan struct{})
	go func() {
		pool.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop should have unblocked blocked dispatch")
	}
}

// TestFwdPool_DoneCalledOnSuccess verifies the done callback is called
// after successful handler execution.
//
// VALIDATES: AC-5 (Release called after worker completes)
// PREVENTS: Cache entry leaks from missing Release calls.
// TestFwdPoolCustomChanSize verifies custom channel size is respected.
//
// VALIDATES: AC-16 — custom chanSize respected by worker creation.
// PREVENTS: Ignoring configuration and always using default 64.
func TestFwdPoolCustomChanSize(t *testing.T) {
	block := make(chan struct{})
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-block
	}, fwdPoolConfig{chanSize: 128, idleTimeout: time.Second})
	defer func() { close(block); pool.Stop() }()

	// Dispatch more than 64 items (default) without blocking — proves chanSize=128.
	for i := range 100 {
		done := make(chan bool, 1)
		go func() {
			ok := pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
			done <- ok
		}()
		var ok bool
		require.Eventually(t, func() bool {
			select {
			case ok = <-done:
				return true
			default:
				return false
			}
		}, 2*time.Second, time.Millisecond, "dispatch %d blocked — chanSize likely smaller than 128", i)
		require.True(t, ok, "dispatch %d rejected", i)
	}
}

// TestForwardPoolBackpressurePropagation verifies that a blocked destination
// causes upstream backpressure through the blocking Dispatch call.
//
// VALIDATES: AC-7 — blocked destination causes upstream backpressure chain.
// PREVENTS: Silent drops when destination peer is slow.
func TestForwardPoolBackpressurePropagation(t *testing.T) {
	var unblocked atomic.Int32
	block := make(chan struct{})
	entered := make(chan struct{}, 1)
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case entered <- struct{}{}: // Signal first entry only.
		default: // already signaled
		}
		<-block // Block all processing.
	}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})
	defer func() {
		if unblocked.CompareAndSwap(0, 1) {
			close(block)
		}
		pool.Stop()
	}()

	// Dispatch 1 item and wait for handler to start blocking.
	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	<-entered

	// Fill channel (4 items = chanSize) while handler is blocked.
	for range 4 {
		pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	}

	// Next dispatch should block (channel full, handler blocked).
	dispatched := make(chan struct{})
	go func() {
		pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
		close(dispatched)
	}()

	require.Never(t, func() bool {
		select {
		case <-dispatched:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond, "dispatch should be blocked but returned immediately")

	// Unblock handler → dispatch should complete.
	unblocked.Store(1)
	close(block)
	require.Eventually(t, func() bool {
		select {
		case <-dispatched:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "dispatch still blocked after handler unblocked")
}

func TestFwdPool_DoneCalledOnSuccess(t *testing.T) {
	doneCh := make(chan struct{}, 5)

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		// Handler does nothing
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	for range 3 {
		pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{
			done: func() { doneCh <- struct{}{} },
		})
	}

	for i := range 3 {
		require.Eventually(t, func() bool {
			select {
			case <-doneCh:
				return true
			default:
				return false
			}
		}, 2*time.Second, time.Millisecond, "timeout waiting for done callback %d", i)
	}
}

// TestFwdWorkerDrainBatch verifies the worker drain-batches multiple items
// from the channel into a single handler call.
//
// VALIDATES: AC-7 (multiple items drained and written in single handler call)
// PREVENTS: One-at-a-time processing when multiple items are queued.
func TestFwdWorkerDrainBatch(t *testing.T) {
	batchSizes := make(chan int, 10)
	entered := make(chan struct{}, 1)
	itemsQueued := make(chan struct{})
	var calls atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		n := calls.Add(1)
		if n == 1 {
			// Signal that first handler call entered, then block
			// so remaining items queue in the channel.
			entered <- struct{}{}
			// Wait for test to finish dispatching items
			<-itemsQueued
		}
		batchSizes <- len(items)
	}, fwdPoolConfig{chanSize: 10, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Dispatch first item — enters handler
	pool.Dispatch(key, fwdItem{})
	<-entered

	// Dispatch 5 more while first handler call is blocked
	for range 5 {
		pool.Dispatch(key, fwdItem{})
	}

	// Unblock first handler call now that items are queued
	close(itemsQueued)

	// First handler call returns batch of 1
	size1 := <-batchSizes
	assert.Equal(t, 1, size1, "first batch should be 1 item")

	// Second handler call should drain-batch all 5 queued items
	size2 := <-batchSizes
	assert.Equal(t, 5, size2, "second batch should drain all 5 queued items")
}

// TestFwdWorkerBatchSingleItem verifies a single queued item works
// identically to the non-batch path (batch of 1).
//
// VALIDATES: AC-12 (single item behavior identical)
// PREVENTS: Edge case in drain-batch breaking single-item processing.
func TestFwdWorkerBatchSingleItem(t *testing.T) {
	batchSizes := make(chan int, 10)

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		batchSizes <- len(items)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	// Dispatch single item
	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})

	var size int
	require.Eventually(t, func() bool {
		select {
		case size = <-batchSizes:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "timeout waiting for handler")
	assert.Equal(t, 1, size, "single item should produce batch of 1")
}

// TestFwdWorkerBatchAllDoneCalled verifies done() is called for every
// item in a batch, even when the handler panics mid-batch.
//
// VALIDATES: AC-8/AC-9 (done called for all items on error)
// PREVENTS: Cache entry leaks when batch processing fails.
func TestFwdWorkerBatchAllDoneCalled(t *testing.T) {
	var doneCalled atomic.Int32
	entered := make(chan struct{}, 1)

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case entered <- struct{}{}:
		default: // already signaled
		}
		panic("test batch panic")
	}, fwdPoolConfig{chanSize: 10, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}
	doneFunc := func() { doneCalled.Add(1) }

	// Dispatch first item — enters handler and panics
	pool.Dispatch(key, fwdItem{done: doneFunc})
	<-entered

	// Wait for panic recovery (done callback called for panicked item)
	require.Eventually(t, func() bool { return doneCalled.Load() >= 1 },
		time.Second, time.Millisecond, "done should be called after panic recovery")

	// Dispatch 3 more while worker is ready again
	// These should form a batch
	pool.Dispatch(key, fwdItem{done: doneFunc})
	pool.Dispatch(key, fwdItem{done: doneFunc})
	pool.Dispatch(key, fwdItem{done: doneFunc})

	// All 4 done callbacks should have been called
	require.Eventually(t, func() bool {
		return doneCalled.Load() == 4
	}, time.Second, time.Millisecond, "done must be called for every item")
}

// TestFwdDrainBatchReusesBuffer verifies that drainBatch reuses the
// caller-provided buffer across calls — no new backing array allocation on the second call.
//
// VALIDATES: AC-2 from spec-alloc-1-batch-pooling.md
// PREVENTS: Per-burst slice allocations in forward pool worker goroutine.
func TestFwdDrainBatchReusesBuffer(t *testing.T) {
	ch := make(chan fwdItem, 4)
	ch <- fwdItem{}
	ch <- fwdItem{}

	first := fwdItem{}

	// First call: buffer grows from nil.
	var buf []fwdItem
	buf = drainBatch(buf, first, ch, 0)

	if len(buf) != 3 {
		t.Fatalf("expected 3 items, got %d", len(buf))
	}
	firstPtr := unsafe.SliceData(buf)

	// Second call: reuse existing buffer.
	ch <- fwdItem{}
	first2 := fwdItem{}
	buf = drainBatch(buf, first2, ch, 0)

	if len(buf) != 2 {
		t.Fatalf("expected 2 items, got %d", len(buf))
	}
	secondPtr := unsafe.SliceData(buf)

	if firstPtr != secondPtr {
		t.Error("second call allocated a new backing array instead of reusing buffer")
	}
}

// TestFwdDrainBatchLimit verifies that drainBatch stops at the configured limit.
// Remaining items stay in the channel for the next cycle.
//
// VALIDATES: AC-24 — TX budget caps messages per batch.
// PREVENTS: One peer's convergence burst monopolizing writeMu hold time.
func TestFwdDrainBatchLimit(t *testing.T) {
	ch := make(chan fwdItem, 10)
	// Queue 5 extra items (6 total with firstItem).
	for range 5 {
		ch <- fwdItem{}
	}

	first := fwdItem{}

	// Limit=1: returns only firstItem, no channel drain.
	var buf []fwdItem
	buf = drainBatch(buf, first, ch, 1)
	if len(buf) != 1 {
		t.Fatalf("limit=1: got %d items, want 1", len(buf))
	}
	if len(ch) != 5 {
		t.Errorf("limit=1: channel should still have 5, got %d", len(ch))
	}

	// Limit=3: should get exactly 3 items from channel (firstItem re-created).
	first = fwdItem{}
	buf = drainBatch(buf, first, ch, 3)
	if len(buf) != 3 {
		t.Fatalf("limit=3: got %d items, want 3", len(buf))
	}

	// 3 items should remain in the channel (5 - 2 drained by limit=3).
	if len(ch) != 3 {
		t.Errorf("remaining in channel: %d, want 3", len(ch))
	}

	// Limit=0 (unlimited): drains all remaining.
	first2 := <-ch
	buf = drainBatch(buf, first2, ch, 0)
	if len(buf) != 3 {
		t.Errorf("limit=0: got %d items, want 3", len(buf))
	}
	if len(ch) != 0 {
		t.Errorf("channel should be empty, got %d", len(ch))
	}
}

// TestFwdWorkerIdleRestartFreshBuffer verifies that when a forward pool worker
// exits on idle timeout and is re-created for a new dispatch, it starts with
// a fresh buffer (no stale data from previous incarnation).
//
// VALIDATES: AC-6 from spec-alloc-1-batch-pooling.md
// PREVENTS: Stale data leaking across worker incarnations.
func TestFwdWorkerIdleRestartFreshBuffer(t *testing.T) {
	batchSizes := make(chan int, 10)

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		batchSizes <- len(items)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: 50 * time.Millisecond})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Dispatch 3 items to force buffer growth.
	pool.Dispatch(key, fwdItem{})
	pool.Dispatch(key, fwdItem{})
	pool.Dispatch(key, fwdItem{})

	// Wait for worker to process items and report batch size.
	require.Eventually(t, func() bool {
		return len(batchSizes) > 0
	}, time.Second, time.Millisecond, "worker should process first batch")

	// Drain batch sizes from first worker.
	for len(batchSizes) > 0 {
		<-batchSizes
	}

	// Wait for idle timeout — worker exits.
	require.Eventually(t, func() bool {
		return pool.WorkerCount() == 0
	}, time.Second, time.Millisecond, "worker should exit after idle timeout")

	// Dispatch a single item — new worker is created with fresh (nil) buffer.
	pool.Dispatch(key, fwdItem{})

	var restartSize int
	require.Eventually(t, func() bool {
		select {
		case restartSize = <-batchSizes:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "timeout waiting for restarted worker to process item")
	assert.Equal(t, 1, restartSize, "restarted worker should see only the new item")
	assert.Equal(t, 1, pool.WorkerCount())
}

// TestFwdPool_TryDispatch verifies TryDispatch is non-blocking: succeeds when
// channel has space, returns false immediately when channel is full.
//
// VALIDATES: AC-2, AC-3
// PREVENTS: TryDispatch blocking like Dispatch when channel is full.
func TestFwdPool_TryDispatch(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// First dispatch enters handler (blocks on blocker)
	ok := pool.TryDispatch(key, fwdItem{})
	require.True(t, ok, "TryDispatch should succeed on empty channel")

	<-handlerStarted // Wait for handler to start blocking

	// Fill channel (2 items = chanSize)
	ok = pool.TryDispatch(key, fwdItem{})
	require.True(t, ok, "TryDispatch should succeed when channel has space")
	ok = pool.TryDispatch(key, fwdItem{})
	require.True(t, ok, "TryDispatch should succeed when channel has space")

	// Channel full -- TryDispatch should return false immediately
	ok = pool.TryDispatch(key, fwdItem{})
	assert.False(t, ok, "TryDispatch should return false on full channel")

	close(blocker)
}

// TestFwdPool_DispatchOverflow verifies overflow items are processed after
// channel items drain.
//
// VALIDATES: AC-4, AC-6
// PREVENTS: Overflow items silently lost.
func TestFwdPool_DispatchOverflow(t *testing.T) {
	var processed atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Dispatch some items via overflow
	pool.DispatchOverflow(key, fwdItem{done: func() {}})
	pool.DispatchOverflow(key, fwdItem{done: func() {}})

	// Dispatch a normal item to trigger worker creation + processing
	pool.Dispatch(key, fwdItem{})

	// Wait for processing -- all items including overflow should be processed
	require.Eventually(t, func() bool {
		return processed.Load() >= 3
	}, time.Second, time.Millisecond, "all items including overflow should be processed")
}

// TestFwdPool_OverflowNeverDrops verifies the overflow buffer grows without
// dropping items. Routes are critical data and must never be silently lost.
//
// VALIDATES: AC-5
// PREVENTS: Silent route loss from overflow cap.
func TestFwdPool_OverflowNeverDrops(t *testing.T) {
	blocker := make(chan struct{})
	var unblocked atomic.Bool
	var processed atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		if !unblocked.Load() {
			<-blocker
		}
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Create worker by dispatching one item (blocks in handler)
	pool.Dispatch(key, fwdItem{})
	require.Eventually(t, func() bool {
		return pool.WorkerCount() == 1
	}, time.Second, time.Millisecond, "worker should be spawned")

	// Fill overflow with many items -- none should be dropped
	const overflowCount = 500
	for range overflowCount {
		pool.DispatchOverflow(key, fwdItem{done: func() {}})
	}

	// Unblock handler -- all items should eventually be processed
	unblocked.Store(true)
	close(blocker)

	require.Eventually(t, func() bool {
		return processed.Load() >= overflowCount
	}, 2*time.Second, 10*time.Millisecond, "all overflow items must be processed, none dropped")
}

// TestFwdPool_StopFiresOverflowDone verifies Stop fires done callbacks
// for all items remaining in overflow buffers.
//
// VALIDATES: AC-7
// PREVENTS: Cache entry leaks from overflow items on shutdown.
func TestFwdPool_StopFiresOverflowDone(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)
	var doneCalled atomic.Int32

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Create worker (blocks in handler)
	pool.Dispatch(key, fwdItem{})
	<-handlerStarted

	// Add overflow items
	for range 3 {
		pool.DispatchOverflow(key, fwdItem{
			done: func() { doneCalled.Add(1) },
		})
	}

	// Stop should fire done for overflow items
	close(blocker)
	pool.Stop()

	assert.GreaterOrEqual(t, int(doneCalled.Load()), 3, "done must be called for all overflow items on Stop")
}

// TestFwdPool_CongestionCallbacks verifies onCongested fires on first TryDispatch
// failure and onResumed fires when worker drains below low-water mark.
//
// VALIDATES: AC-8, AC-9, AC-14
// PREVENTS: Missing congestion state transitions, callback storms.
func TestFwdPool_CongestionCallbacks(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)
	var congestedPeers []string
	var resumedPeers []string
	var mu sync.Mutex

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})
	pool.onCongested = func(peerAddr netip.AddrPort) {
		mu.Lock()
		congestedPeers = append(congestedPeers, peerAddr.String())
		mu.Unlock()
	}
	pool.onResumed = func(peerAddr netip.AddrPort) {
		mu.Lock()
		resumedPeers = append(resumedPeers, peerAddr.String())
		mu.Unlock()
	}
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}

	// Fill: 1 in handler + 4 in channel = full
	pool.Dispatch(key, fwdItem{})
	<-handlerStarted
	for range 4 {
		pool.Dispatch(key, fwdItem{})
	}

	// TryDispatch fails -> congested callback should fire
	ok := pool.TryDispatch(key, fwdItem{})
	assert.False(t, ok)

	mu.Lock()
	assert.Equal(t, []string{"10.0.0.1:179"}, congestedPeers, "onCongested should fire once")
	mu.Unlock()

	// Second failure should NOT fire again (already congested)
	ok = pool.TryDispatch(key, fwdItem{})
	assert.False(t, ok)

	mu.Lock()
	assert.Equal(t, 1, len(congestedPeers), "onCongested should not fire twice")
	mu.Unlock()

	// Unblock handler -- worker drains, should fire onResumed
	close(blocker)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(resumedPeers) == 1
	}, 2*time.Second, 10*time.Millisecond, "onResumed should fire after drain")

	mu.Lock()
	assert.Equal(t, []string{"10.0.0.1:179"}, resumedPeers)
	mu.Unlock()
}

// TestFwdPool_DrainOverflowDirectProcess verifies drainOverflow's processDirect
// path: when overflow items cannot be enqueued (channel full), they are processed
// directly via safeBatchHandle.
//
// VALIDATES: drainOverflow goto processDirect path
// PREVENTS: Overflow items silently lost when channel refills during drain.
func TestFwdPool_DrainOverflowDirectProcess(t *testing.T) {
	var processed atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Add many overflow items (more than channel capacity).
	// When drainOverflow runs, some will enqueue, rest will be processed directly.
	for range 10 {
		pool.DispatchOverflow(key, fwdItem{done: func() {}})
	}

	// Trigger worker creation + processing by dispatching a normal item.
	pool.Dispatch(key, fwdItem{})

	// All 11 items (10 overflow + 1 normal) should be processed.
	require.Eventually(t, func() bool {
		return processed.Load() >= 11
	}, 2*time.Second, time.Millisecond, "all items including overflow direct-processed should complete")
}

// TestFwdPool_OverflowUsesPool verifies overflow items acquire handles from
// the shared MixedBufMux and return them after processing.
//
// VALIDATES: AC-1 (overflow stored in pool, never dropped)
// PREVENTS: Overflow bypassing the bounded pool system.
func TestFwdPool_OverflowUsesPool(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)
	var processed atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	mux := newMixedBufMux()
	mux.SetByteBudget(1024 * 1024) // 1MB budget
	pool.SetOverflowMux(mux)
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Create worker (blocks in handler).
	pool.Dispatch(key, fwdItem{})
	<-handlerStarted

	// Add 5 overflow items -- mux should track them.
	// peer must be non-nil so items are not treated as sentinels (which skip pool acquire).
	dummyPeer := &Peer{}
	for range 5 {
		pool.DispatchOverflow(key, fwdItem{done: func() {}, peer: dummyPeer})
	}

	// Mux should have buffers in use.
	_, inUse := mux.Stats()
	assert.Greater(t, inUse, int64(0), "mux should have buffers in use")

	// Unblock -- all items processed, buffers returned.
	close(blocker)

	require.Eventually(t, func() bool {
		_, used := mux.Stats()
		return used == 0
	}, time.Second, time.Millisecond, "all mux buffers should be returned after processing")

	require.Eventually(t, func() bool {
		return processed.Load() >= 5
	}, time.Second, time.Millisecond, "all overflow items should be processed")
}

// TestFwdPool_PeerDisconnectReturnsSlots verifies Stop returns all overflow
// buffers and calls done() for overflow items still queued at shutdown.
//
// VALIDATES: AC-7 (peer disconnect returns pool slots, done() called)
// PREVENTS: Buffer leaks on peer disconnect or reactor shutdown.
func TestFwdPool_PeerDisconnectReturnsSlots(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)
	var doneCalled atomic.Int32

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	mux := newMixedBufMux()
	mux.SetByteBudget(1024 * 1024)
	pool.SetOverflowMux(mux)

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Create worker (blocks in handler).
	pool.Dispatch(key, fwdItem{})
	<-handlerStarted

	// Add overflow items with done callbacks.
	// peer must be non-nil so items acquire overflow handles (nil peer = sentinel, skips acquire).
	dummyPeer := &Peer{}
	for range 4 {
		pool.DispatchOverflow(key, fwdItem{
			done: func() { doneCalled.Add(1) },
			peer: dummyPeer,
		})
	}

	// Verify buffers were acquired.
	_, inUse := mux.Stats()
	assert.Greater(t, inUse, int64(0), "mux should have buffers in use")

	// Stop simulates shutdown -- should return all buffers and call done().
	close(blocker)
	pool.Stop()

	assert.GreaterOrEqual(t, int(doneCalled.Load()), 4,
		"done must be called for all overflow items")
	_, inUseAfter := mux.Stats()
	assert.Equal(t, int64(0), inUseAfter,
		"all mux buffers must be returned after Stop")
}

// TestFwdPool_PoolExhausted verifies that when the overflow mux budget is
// exhausted, items are still accepted (unbounded fallback) and never dropped.
//
// VALIDATES: AC-11 (memory bounded by pool size, graceful fallback)
// PREVENTS: Panic or route drop when pool is exhausted.
func TestFwdPool_PoolExhausted(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)
	var processed atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	// Tiny budget: only room for ~1 block of 4K buffers (16 buffers).
	mux := newMixedBufMux()
	mux.SetByteBudget(int64(message.MaxMsgLen) * 16)
	pool.SetOverflowMux(mux)
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Create worker (blocks in handler).
	pool.Dispatch(key, fwdItem{})
	<-handlerStarted

	// Add more overflow items than the budget allows.
	// Items beyond budget get nil Buf (exhausted) but are still accepted.
	dummyPeer := &Peer{}
	for range 20 {
		ok := pool.DispatchOverflow(key, fwdItem{done: func() {}, peer: dummyPeer})
		require.True(t, ok, "DispatchOverflow must never reject items")
	}

	// Unblock and verify all 20 items processed (none dropped).
	close(blocker)

	require.Eventually(t, func() bool {
		return processed.Load() >= 20
	}, time.Second, time.Millisecond, "all items including fallback must be processed")

	// All buffers that were acquired should be returned.
	require.Eventually(t, func() bool {
		_, used := mux.Stats()
		return used == 0
	}, time.Second, time.Millisecond, "all mux buffers should be returned after processing")
}

// TestFwdPool_DrainOverflowDirectProcessReleasesTokens verifies overflow
// buffers are returned when drainOverflow processes items directly
// (processDirect path) instead of enqueuing them to the channel.
//
// VALIDATES: drainOverflow processDirect path releases overflow buffers
// PREVENTS: Buffer leaks when overflow items exceed channel capacity during drain.
func TestFwdPool_DrainOverflowDirectProcessReleasesTokens(t *testing.T) {
	var processed atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	mux := newMixedBufMux()
	mux.SetByteBudget(1024 * 1024)
	pool.SetOverflowMux(mux)
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}

	// Add many overflow items (more than channel capacity of 2).
	// When drainOverflow runs, some enqueue to channel, rest are processed directly.
	// peer must be non-nil so items acquire overflow handles (nil peer = sentinel, skips acquire).
	dummyPeer := &Peer{}
	for range 10 {
		pool.DispatchOverflow(key, fwdItem{done: func() {}, peer: dummyPeer})
	}

	// Trigger worker creation + processing.
	pool.Dispatch(key, fwdItem{})

	// All 11 items should be processed.
	require.Eventually(t, func() bool {
		return processed.Load() >= 11
	}, 2*time.Second, time.Millisecond, "all items should be processed")

	// All overflow buffers should be returned (regardless of channel vs direct path).
	require.Eventually(t, func() bool {
		_, used := mux.Stats()
		return used == 0
	}, time.Second, time.Millisecond, "all mux buffers must be returned")
}

// TestFwdPool_DispatchOverflowAfterStopWithMux verifies DispatchOverflow
// on a stopped pool with overflow mux enabled: returns false, calls done(),
// and does NOT consume a mux buffer.
//
// VALIDATES: DispatchOverflow stopped-pool path with mux enabled
// PREVENTS: Buffer leaks when dispatching to a stopped pool.
func TestFwdPool_DispatchOverflowAfterStopWithMux(t *testing.T) {
	var doneCalled atomic.Int32

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})
	mux := newMixedBufMux()
	mux.SetByteBudget(1024 * 1024)
	pool.SetOverflowMux(mux)
	pool.Stop()

	ok := pool.DispatchOverflow(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{
		done: func() { doneCalled.Add(1) },
	})

	assert.False(t, ok, "DispatchOverflow must return false on stopped pool")
	assert.Equal(t, int32(1), doneCalled.Load(), "done must be called on stopped pool")
	_, inUse := mux.Stats()
	assert.Equal(t, int64(0), inUse,
		"no mux buffers should be consumed on stopped pool")
}

// TestFwdPool_OverflowDepths verifies OverflowDepths returns per-peer
// overflow item counts as a snapshot.
//
// VALIDATES: AC-17 (overflow depth visible per destination peer)
// PREVENTS: Missing or inaccurate overflow depth metrics.
func TestFwdPool_OverflowDepths(t *testing.T) {
	blocker := make(chan struct{})
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	// Create workers (block in handler).
	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}, fwdItem{})
	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.2:179")}, fwdItem{})
	require.Eventually(t, func() bool {
		return pool.WorkerCount() == 2
	}, time.Second, time.Millisecond, "both workers should be spawned")

	// Add overflow items to first peer only.
	for range 3 {
		pool.DispatchOverflow(fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}, fwdItem{done: func() {}})
	}

	depths := pool.OverflowDepths()
	assert.Equal(t, 3, depths["10.0.0.1"], "peer 10.0.0.1 should have 3 overflow items")
	assert.Equal(t, 0, depths["10.0.0.2"], "peer 10.0.0.2 should have 0 overflow items")

	// Unblock and verify depths return to 0 after drain.
	close(blocker)

	require.Eventually(t, func() bool {
		d := pool.OverflowDepths()
		return d["10.0.0.1"] == 0
	}, time.Second, time.Millisecond, "overflow depth should return to 0 after drain")
}

// TestFwdPool_PoolUsedRatio verifies PoolUsedRatio returns correct utilization.
//
// VALIDATES: AC-18 (pool utilization visible as ratio)
// PREVENTS: Incorrect pool utilization metric.
func TestFwdPool_PoolUsedRatio(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	mux := newMixedBufMux()
	mux.SetByteBudget(1024 * 1024) // 1MB budget
	pool.SetOverflowMux(mux)
	defer pool.Stop()

	// No overflow items yet -- ratio should be 0.
	assert.InDelta(t, 0.0, pool.PoolUsedRatio(), 0.001, "empty pool should have 0.0 ratio")

	// Create worker and add overflow items.
	pool.Dispatch(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{})
	<-handlerStarted
	dummyPeer := &Peer{}
	for range 4 {
		pool.DispatchOverflow(fwdKey{peerAddr: netip.MustParseAddrPort("1.1.1.1:179")}, fwdItem{done: func() {}, peer: dummyPeer})
	}

	// With items in use, ratio should be > 0.
	assert.Greater(t, pool.PoolUsedRatio(), 0.0, "ratio should be > 0 with items in use")

	close(blocker)
}

// TestFwdPool_PoolUsedRatioNoPool verifies PoolUsedRatio returns 0 when
// no overflow pool is configured.
//
// VALIDATES: AC-18 (graceful no-pool path)
// PREVENTS: Panic or wrong value when pool is nil.
func TestFwdPool_PoolUsedRatioNoPool(t *testing.T) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	assert.InDelta(t, 0.0, pool.PoolUsedRatio(), 0.001, "no pool should return 0.0")
}

// TestFwdPool_SourceOverflowRatios verifies per-source overflow ratio tracking.
//
// VALIDATES: AC-16 (per-source overflow ratio)
// PREVENTS: Wrong ratio calculation or missing source peer tracking.
func TestFwdPool_SourceOverflowRatios(t *testing.T) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	// Source A: 8 forwarded, 2 overflowed = 20% overflow ratio.
	for range 8 {
		pool.RecordForwarded(netip.MustParseAddr("10.0.0.1"))
	}
	for range 2 {
		pool.RecordOverflowed(netip.MustParseAddr("10.0.0.1"))
	}

	// Source B: 0 forwarded, 5 overflowed = 100% overflow ratio.
	for range 5 {
		pool.RecordOverflowed(netip.MustParseAddr("10.0.0.2"))
	}

	// Source C: 10 forwarded, 0 overflowed = 0% overflow ratio.
	for range 10 {
		pool.RecordForwarded(netip.MustParseAddr("10.0.0.3"))
	}

	ratios := pool.SourceOverflowRatios()
	assert.InDelta(t, 0.2, ratios["10.0.0.1"], 0.001, "source A: 2/10 = 0.2")
	assert.InDelta(t, 1.0, ratios["10.0.0.2"], 0.001, "source B: 5/5 = 1.0")
	assert.InDelta(t, 0.0, ratios["10.0.0.3"], 0.001, "source C: 0/10 = 0.0")
}

// TestFwdPool_SourceOverflowRatiosConcurrent verifies concurrent
// RecordForwarded/RecordOverflowed calls don't race.
//
// VALIDATES: AC-16 (concurrent safety of source stats)
// PREVENTS: Race conditions in source peer counter updates.
func TestFwdPool_SourceOverflowRatiosConcurrent(t *testing.T) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	var wg sync.WaitGroup
	const goroutines = 20
	const iterations = 100
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				pool.RecordForwarded(netip.MustParseAddr("10.0.0.1"))
				pool.RecordOverflowed(netip.MustParseAddr("10.0.0.1"))
			}
		}()
	}
	wg.Wait()

	ratios := pool.SourceOverflowRatios()
	// Each goroutine does equal forwarded + overflowed, so ratio should be 0.5.
	assert.InDelta(t, 0.5, ratios["10.0.0.1"], 0.001,
		"equal forwarded and overflowed should give 0.5 ratio")
}

// --- Per-peer pool tests (fwd-auto-sizing Phase 3) ---

func TestPeerPool4K(t *testing.T) {
	// Per-peer pool: 64 slots for standard (4K) peer.
	pp := newPeerPool(4096)
	assert.Equal(t, 64, pp.size())
	assert.Equal(t, 64, pp.available())
	assert.Equal(t, 4096, pp.bufSize)
}

func TestPeerPool64K(t *testing.T) {
	// Per-peer pool: 64 slots for ExtMsg (64K) peer.
	pp := newPeerPool(65535)
	assert.Equal(t, 64, pp.size())
	assert.Equal(t, 65535, pp.bufSize)
}

func TestPeerPoolExhausted(t *testing.T) {
	// Pool full after 64 Get() calls, next returns (nil, 0).
	pp := newPeerPool(4096)
	idxs := make([]int, 0, 64)
	for range 64 {
		buf, idx := pp.Get()
		require.NotNil(t, buf, "Get should succeed within capacity")
		require.Greater(t, idx, 0, "index should be 1-based")
		assert.Len(t, buf, 4096, "buffer should be 4096 bytes")
		idxs = append(idxs, idx)
	}
	assert.Equal(t, 0, pp.available())
	buf, idx := pp.Get()
	assert.Nil(t, buf, "Get should return nil when pool is exhausted")
	assert.Equal(t, 0, idx, "exhausted pool returns sentinel 0")

	// Return all buffers.
	for _, i := range idxs {
		pp.Return(i)
	}
}

func TestPeerPoolReturn(t *testing.T) {
	// Return frees buffer, subsequent Get succeeds.
	pp := newPeerPool(4096)
	idxs := make([]int, 0, 64)
	for range 64 {
		_, idx := pp.Get()
		idxs = append(idxs, idx)
	}
	_, idx := pp.Get()
	assert.Equal(t, 0, idx)
	pp.Return(idxs[0])
	assert.Equal(t, 1, pp.available())
	buf, idx := pp.Get()
	assert.NotNil(t, buf, "Get should succeed after Return")
	assert.Greater(t, idx, 0)
	assert.Len(t, buf, 4096)
}

func TestPeerPoolConcurrent(t *testing.T) {
	// Concurrent Get/Return does not corrupt the pool.
	pp := newPeerPool(4096)
	var wg sync.WaitGroup
	const goroutines = 10
	const iterations = 100
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				if _, idx := pp.Get(); idx > 0 {
					pp.Return(idx)
				}
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 64, pp.available(), "all buffers should be returned")
}

// --- Overflow MixedBufMux integration tests (fwd-auto-sizing Phase 4) ---

func TestFwdPool_PoolUsedRatioMixedBufMux(t *testing.T) {
	// PoolUsedRatio = totalBlocks / maxBlocks (memory pressure, not bytes handed out).
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	// Budget = 32 blocks. First Get4K grows a chunk of 16 blocks -> ratio = 16/32 = 0.5.
	mux := newMixedBufMux()
	mux.SetByteBudget(32 * overflowBlockSize)
	pool.SetOverflowMux(mux)

	// No allocations: ratio should be 0.
	assert.Equal(t, 0.0, pool.PoolUsedRatio())

	// Get4K grows one chunk (16 blocks) -> totalBlocks=16, maxBlocks=32 -> ratio=0.5.
	h := mux.Get4K()
	ratio := pool.PoolUsedRatio()
	assert.InDelta(t, 0.5, ratio, 0.01, "one chunk of two allocated")

	mux.Return(h)
}

func TestFwdPool_OverflowExhaustedRejectsDispatch(t *testing.T) {
	// When overflow MixedBufMux is exhausted, DispatchOverflow still accepts
	// (routes never dropped) but without a pooled buffer.
	handled := make(chan struct{}, 100)
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		for range items {
			handled <- struct{}{}
		}
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	mux := newMixedBufMux()
	mux.SetByteBudget(1) // Tiny budget -- will exhaust immediately.
	pool.SetOverflowMux(mux)

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}

	// DispatchOverflow should still return true (routes never dropped).
	ok := pool.DispatchOverflow(key, fwdItem{})
	assert.True(t, ok, "DispatchOverflow must not drop routes even when exhausted")
}

// --- Two-tier dispatch integration tests (deep-review findings 1-5, 10) ---

func TestFwdPool_TryDispatchWithPeerPool(t *testing.T) {
	// Finding #1: TryDispatch acquires/releases per-peer pool slot through full lifecycle.
	var processed atomic.Int32
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 256, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}
	pool.RegisterOutgoingPool(key, 4096)

	pool.mu.Lock()
	pp := pool.outgoingPools[key]
	pool.mu.Unlock()

	// Dispatch 10 items -- each should acquire a peer pool slot.
	for range 10 {
		ok := pool.TryDispatch(key, fwdItem{})
		require.True(t, ok, "TryDispatch should succeed with available peer pool slots")
	}

	// Wait for processing.
	require.Eventually(t, func() bool {
		return processed.Load() >= 10
	}, time.Second, time.Millisecond)

	// All slots should be returned after processing.
	require.Eventually(t, func() bool {
		return pp.available() == 64
	}, time.Second, time.Millisecond, "all peer pool slots should be returned after processing")
}

func TestFwdPool_TryDispatchChannelFull(t *testing.T) {
	// When the worker channel is full, TryDispatch returns false and fires congested callback.
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)

	const chanSize = 8
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: chanSize, idleTimeout: time.Second})
	defer func() {
		close(blocker)
		pool.Stop()
	}()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}

	var congestedCount atomic.Int32
	pool.onCongested = func(_ netip.AddrPort) {
		congestedCount.Add(1)
	}

	// First dispatch starts the worker, which blocks.
	pool.TryDispatch(key, fwdItem{})
	<-handlerStarted

	// Fill the channel (handler is blocked, items queue up).
	for i := range chanSize {
		ok := pool.TryDispatch(key, fwdItem{})
		require.True(t, ok, "TryDispatch #%d should succeed (channel not full)", i+2)
	}

	// Next dispatch should fail because channel is full.
	ok := pool.TryDispatch(key, fwdItem{})
	assert.False(t, ok, "TryDispatch should fail when channel is full")
	assert.Equal(t, int32(1), congestedCount.Load(), "congested callback should fire once")
}

func TestFwdPool_DispatchOverflowGet64K(t *testing.T) {
	// Finding #10: DispatchOverflow uses Get64K() for ExtMsg peers.
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	mux := newMixedBufMux()
	mux.SetByteBudget(1024 * 1024)
	pool.SetOverflowMux(mux)
	defer func() {
		close(blocker)
		pool.Stop()
	}()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}
	pool.RegisterOutgoingPool(key, 65535) // ExtMsg buffer size

	// Block the worker so overflow items stay queued.
	pool.Dispatch(key, fwdItem{})
	<-handlerStarted

	dummyPeer := &Peer{}
	pool.DispatchOverflow(key, fwdItem{peer: dummyPeer, done: func() {}})

	// Check that the mux allocated a whole block (64K) for the ExtMsg peer.
	_, usedBytes := mux.Stats()
	assert.Equal(t, int64(overflowBlockSize), usedBytes, "should allocate one active block for ExtMsg peer")
}

func TestFwdPool_SupersedeReleasesOverflowBuf(t *testing.T) {
	// Finding #4: Superseding releases old item's overflowBuf.
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	mux := newMixedBufMux()
	mux.SetByteBudget(1024 * 1024)
	pool.SetOverflowMux(mux)
	defer func() {
		close(blocker)
		pool.Stop()
	}()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}
	dummyPeer := &Peer{}

	// Block the worker so overflow items accumulate.
	pool.Dispatch(key, fwdItem{})
	<-handlerStarted

	// Dispatch two overflow items with same supersede key and body.
	body := []byte{0x00, 0x00, 0x00, 0x15, 0x40, 0x01}
	pool.DispatchOverflow(key, fwdItem{
		peer:         dummyPeer,
		rawBodies:    [][]byte{body},
		supersedeKey: 12345,
		done:         func() {},
	})

	_, used1 := mux.Stats()
	assert.Greater(t, used1, int64(0), "first overflow item should hold a buffer")

	// Second item with same key supersedes the first -- old buffer returned.
	pool.DispatchOverflow(key, fwdItem{
		peer:         dummyPeer,
		rawBodies:    [][]byte{body},
		supersedeKey: 12345,
		done:         func() {},
	})

	// After supersede: should still have exactly 1 buffer in use (the new item's).
	_, used2 := mux.Stats()
	assert.Equal(t, used1, used2, "supersede should return old buffer and acquire new, net same")
}

func TestFwdPool_RegisterUnregisterOutgoingPool(t *testing.T) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}

	pool.RegisterOutgoingPool(key, 4096)
	pool.mu.Lock()
	pp := pool.outgoingPools[key]
	pool.mu.Unlock()
	require.NotNil(t, pp)
	assert.Equal(t, 64, pp.size())
	assert.Equal(t, 4096, pp.bufSize)

	pool.UnregisterOutgoingPool(key)
	pool.mu.Lock()
	pp = pool.outgoingPools[key]
	pool.mu.Unlock()
	assert.Nil(t, pp)
}

// TestFwdPool_DenialThroughDispatchOverflow verifies that the congestion
// controller's ShouldDeny integrates with DispatchOverflow: when the pool
// is above the denial threshold and the peer is the worst offender, the
// item is still accepted (routes never dropped) but no overflow buffer is
// acquired. This is the actual backpressure signal -- not dispatch rejection.
//
// VALIDATES: AC-12 (backpressure on pool exhaustion via denial, not rejection)
// PREVENTS: Confusion between "dispatch rejected" (spec wording) and actual
// behavior (accepted without buffer, congestion controller escalates).
func TestFwdPool_DenialThroughDispatchOverflow(t *testing.T) {
	blocker := make(chan struct{})
	handlerStarted := make(chan struct{}, 1)

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		select {
		case handlerStarted <- struct{}{}:
		default:
		}
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	mux := newMixedBufMux()
	mux.SetByteBudget(1024 * 1024) // Large budget -- not the exhaustion path.
	pool.SetOverflowMux(mux)
	defer func() {
		close(blocker)
		pool.Stop()
	}()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}
	pool.RegisterOutgoingPool(key, message.MaxMsgLen)

	// Wire a congestion controller that always denies this peer.
	wt := newWeightTracker(nil)
	wt.AddPeer("10.0.0.1", 10000, 1)

	var deniedCount atomic.Int64
	pool.congestion = newCongestionController(congestionConfig{
		gracePeriod:    5 * time.Second,
		poolUsedRatio:  func() float64 { return 0.90 }, // Above 80% threshold
		overflowDepths: func() map[string]int { return map[string]int{"10.0.0.1": 500} },
		weights:        wt,
		onDenied:       func() { deniedCount.Add(1) },
	})

	// Block the worker so overflow items stay queued.
	pool.Dispatch(key, fwdItem{})
	<-handlerStarted

	dummyPeer := &Peer{}
	ok := pool.DispatchOverflow(key, fwdItem{peer: dummyPeer, done: func() {}})
	assert.True(t, ok, "DispatchOverflow must accept (routes never dropped)")
	assert.Equal(t, int64(1), deniedCount.Load(), "denial callback should fire")

	// The item was accepted but no overflow buffer acquired (denied).
	// Verify mux has zero bytes in use -- denial skipped buffer acquisition.
	_, usedBytes := mux.Stats()
	assert.Equal(t, int64(0), usedBytes, "denied peer should not consume overflow buffer")
}

// TestFwdPool_UnregisterWithInFlightItems verifies that items dispatched
// before UnregisterOutgoingPool still release their peerPoolRef correctly.
// The orphaned peerPool (deleted from the map) must still accept Return()
// calls via the direct peerPoolRef pointer on the fwdItem.
//
// VALIDATES: AC-13 (session teardown destroys pool without leaking in-flight items)
// PREVENTS: Use-after-free or leak when peer disconnects while items are queued.
func TestFwdPool_UnregisterWithInFlightItems(t *testing.T) {
	var processed atomic.Int32
	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 256, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}
	pool.RegisterOutgoingPool(key, message.MaxMsgLen)

	// Grab the pool reference before unregister.
	pool.mu.Lock()
	pp := pool.outgoingPools[key]
	pool.mu.Unlock()
	require.NotNil(t, pp)

	// Acquire a buffer slot (simulating what TryDispatch does).
	buf, idx := pp.Get()
	require.NotNil(t, buf, "should get a buffer")
	require.Greater(t, idx, 0, "index should be 1-based")
	assert.Equal(t, 63, pp.available(), "one buffer should be out on loan")

	// Unregister the pool (simulating session teardown).
	pool.UnregisterOutgoingPool(key)

	// The pool is removed from the map, but the peerPoolRef is still valid.
	pool.mu.Lock()
	gone := pool.outgoingPools[key]
	pool.mu.Unlock()
	assert.Nil(t, gone, "pool should be removed from map")

	// Return the buffer via the direct reference -- must not panic.
	pp.Return(idx)
	assert.Equal(t, 64, pp.available(), "all buffers should be returned to orphaned pool")
}

// TestFwdPool_ReregisterExtMsg verifies that re-registering a peer with a
// different buffer size (4K -> 64K for ExtMsg negotiation) replaces the pool.
// Items in flight with the old pool's peerPoolRef release correctly via
// the direct pointer (atomic counter on orphaned struct, GC'd after refs gone).
//
// VALIDATES: AC-2 (ExtMsg peers use 64K buffers after renegotiation)
// PREVENTS: Buffer size mismatch after ExtMsg capability negotiation.
func TestFwdPool_ReregisterExtMsg(t *testing.T) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: netip.MustParseAddrPort("10.0.0.1:179")}

	// Register with standard 4K.
	pool.RegisterOutgoingPool(key, message.MaxMsgLen)
	pool.mu.Lock()
	pp4K := pool.outgoingPools[key]
	pool.mu.Unlock()
	require.Equal(t, message.MaxMsgLen, pp4K.bufSize)

	// Acquire a buffer from the 4K pool (in-flight item).
	buf4K, idx4K := pp4K.Get()
	require.NotNil(t, buf4K)
	assert.Equal(t, message.MaxMsgLen, len(buf4K))

	// Re-register with ExtMsg 64K (simulating capability negotiation).
	pool.RegisterOutgoingPool(key, message.ExtMsgLen)
	pool.mu.Lock()
	pp64K := pool.outgoingPools[key]
	pool.mu.Unlock()
	require.Equal(t, message.ExtMsgLen, pp64K.bufSize)

	// Old pool is orphaned but the in-flight item's peerPoolRef still works.
	pp4K.Return(idx4K)
	assert.Equal(t, 64, pp4K.available(), "orphaned 4K pool should accept return")

	// New pool is fully available.
	assert.Equal(t, 64, pp64K.available(), "new 64K pool should be fully available")

	// New pool returns 64K buffers.
	buf64K, idx64K := pp64K.Get()
	require.NotNil(t, buf64K)
	assert.Equal(t, message.ExtMsgLen, len(buf64K))
	pp64K.Return(idx64K)
}
