package reactor

import (
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	ok := pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
	require.True(t, ok)
	<-handled
	assert.Equal(t, 1, pool.WorkerCount())

	// Same peer reuses worker
	ok = pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
	require.True(t, ok)
	<-handled
	assert.Equal(t, 1, pool.WorkerCount())

	// Different peer creates second worker
	ok = pool.Dispatch(fwdKey{peerAddr: "2.2.2.2"}, fwdItem{})
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

	pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, pool.WorkerCount())

	// Wait for idle timeout + margin
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, pool.WorkerCount())
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

	pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
	pool.Dispatch(fwdKey{peerAddr: "2.2.2.2"}, fwdItem{})
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 2, pool.WorkerCount())

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

	ok := pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
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
		pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
	}

	for i := 1; i <= 5; i++ {
		select {
		case got := <-orderCh:
			assert.Equal(t, i, got)
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for item %d", i)
		}
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

	pool := newFwdPool(func(key fwdKey, _ []fwdItem) {
		if key.peerAddr == "slow" {
			<-slowCh
		} else {
			close(fastDone)
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	// Dispatch to slow peer first
	pool.Dispatch(fwdKey{peerAddr: "slow"}, fwdItem{})
	time.Sleep(10 * time.Millisecond) // Ensure slow worker is blocked

	// Dispatch to fast peer — should complete without waiting for slow
	pool.Dispatch(fwdKey{peerAddr: "fast"}, fwdItem{})

	select {
	case <-fastDone:
		// Fast peer completed — not blocked by slow peer
	case <-time.After(time.Second):
		t.Fatal("fast peer blocked by slow peer")
	}

	close(slowCh)
}

// TestFwdPool_BackpressureBehavior verifies Dispatch blocks when
// the per-peer channel is full and does not drop items.
//
// VALIDATES: AC-10 (backpressure on full channel)
// PREVENTS: Silent message drops under load.
func TestFwdPool_BackpressureBehavior(t *testing.T) {
	blocker := make(chan struct{})
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: "1.1.1.1"}

	// Fill: 1 item in handler + 2 in channel = 3 dispatches
	pool.Dispatch(key, fwdItem{})
	time.Sleep(10 * time.Millisecond)
	pool.Dispatch(key, fwdItem{})
	pool.Dispatch(key, fwdItem{})

	// Next dispatch should block
	dispatched := make(chan bool, 1)
	go func() {
		ok := pool.Dispatch(key, fwdItem{})
		dispatched <- ok
	}()

	select {
	case <-dispatched:
		t.Fatal("dispatch should have blocked on full channel")
	case <-time.After(100 * time.Millisecond):
		// Expected: dispatch is blocked
	}

	close(blocker) // Unblock processing

	select {
	case ok := <-dispatched:
		assert.True(t, ok)
	case <-time.After(time.Second):
		t.Fatal("dispatch should have unblocked after handler drained")
	}
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

	key := fwdKey{peerAddr: "1.1.1.1"}
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
func TestFwdPool_StopUnblocksDispatch(t *testing.T) {
	blocker := make(chan struct{})
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 1, idleTimeout: time.Second})

	key := fwdKey{peerAddr: "1.1.1.1"}
	pool.Dispatch(key, fwdItem{}) // In handler
	time.Sleep(10 * time.Millisecond)
	pool.Dispatch(key, fwdItem{}) // In channel

	// This dispatch should block
	result := make(chan bool, 1)
	go func() {
		ok := pool.Dispatch(key, fwdItem{})
		result <- ok
	}()

	time.Sleep(50 * time.Millisecond)

	// Stop should unblock the blocked dispatch
	close(blocker)
	pool.Stop()

	select {
	case ok := <-result:
		assert.False(t, ok)
	case <-time.After(time.Second):
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
			ok := pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
			done <- ok
		}()
		select {
		case ok := <-done:
			if !ok {
				t.Fatalf("dispatch %d rejected", i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("dispatch %d blocked — chanSize likely smaller than 128", i)
		}
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
	pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
	<-entered

	// Fill channel (4 items = chanSize) while handler is blocked.
	for range 4 {
		pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
	}

	// Next dispatch should block (channel full, handler blocked).
	dispatched := make(chan struct{})
	go func() {
		pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
		close(dispatched)
	}()

	select {
	case <-dispatched:
		t.Fatal("dispatch should be blocked but returned immediately")
	case <-time.After(100 * time.Millisecond):
		// Expected: dispatch is blocked.
	}

	// Unblock handler → dispatch should complete.
	unblocked.Store(1)
	close(block)
	select {
	case <-dispatched:
		// Dispatch completed after handler unblocked.
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch still blocked after handler unblocked")
	}
}

func TestFwdPool_DoneCalledOnSuccess(t *testing.T) {
	doneCh := make(chan struct{}, 5)

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		// Handler does nothing
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer pool.Stop()

	for range 3 {
		pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{
			done: func() { doneCh <- struct{}{} },
		})
	}

	for i := range 3 {
		select {
		case <-doneCh:
			// done called
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for done callback %d", i)
		}
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
	var calls atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		n := calls.Add(1)
		if n == 1 {
			// Signal that first handler call entered, then block
			// so remaining items queue in the channel.
			entered <- struct{}{}
			// Wait briefly for items to be dispatched
			time.Sleep(20 * time.Millisecond)
		}
		batchSizes <- len(items)
	}, fwdPoolConfig{chanSize: 10, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: "1.1.1.1"}

	// Dispatch first item — enters handler
	pool.Dispatch(key, fwdItem{})
	<-entered

	// Dispatch 5 more while first handler call is sleeping
	for range 5 {
		pool.Dispatch(key, fwdItem{})
	}

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
	pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})

	select {
	case size := <-batchSizes:
		assert.Equal(t, 1, size, "single item should produce batch of 1")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler")
	}
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

	key := fwdKey{peerAddr: "1.1.1.1"}
	doneFunc := func() { doneCalled.Add(1) }

	// Dispatch first item — enters handler and panics
	pool.Dispatch(key, fwdItem{done: doneFunc})
	<-entered

	// Wait for panic recovery
	time.Sleep(50 * time.Millisecond)

	// Dispatch 3 more while worker is ready again
	// These should form a batch
	pool.Dispatch(key, fwdItem{done: doneFunc})
	pool.Dispatch(key, fwdItem{done: doneFunc})
	pool.Dispatch(key, fwdItem{done: doneFunc})

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// All 4 done callbacks should have been called
	assert.Equal(t, int32(4), doneCalled.Load(), "done must be called for every item")
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
	buf = drainBatch(buf, first, ch)

	if len(buf) != 3 {
		t.Fatalf("expected 3 items, got %d", len(buf))
	}
	firstPtr := unsafe.SliceData(buf)

	// Second call: reuse existing buffer.
	ch <- fwdItem{}
	first2 := fwdItem{}
	buf = drainBatch(buf, first2, ch)

	if len(buf) != 2 {
		t.Fatalf("expected 2 items, got %d", len(buf))
	}
	secondPtr := unsafe.SliceData(buf)

	if firstPtr != secondPtr {
		t.Error("second call allocated a new backing array instead of reusing buffer")
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

	key := fwdKey{peerAddr: "1.1.1.1"}

	// Dispatch 3 items to force buffer growth.
	pool.Dispatch(key, fwdItem{})
	pool.Dispatch(key, fwdItem{})
	pool.Dispatch(key, fwdItem{})
	time.Sleep(20 * time.Millisecond)

	// Drain batch sizes from first worker.
	for len(batchSizes) > 0 {
		<-batchSizes
	}

	assert.Equal(t, 1, pool.WorkerCount())

	// Wait for idle timeout — worker exits.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, pool.WorkerCount())

	// Dispatch a single item — new worker is created with fresh (nil) buffer.
	pool.Dispatch(key, fwdItem{})

	select {
	case size := <-batchSizes:
		// New worker processed exactly 1 item — no stale data from old buffer.
		assert.Equal(t, 1, size, "restarted worker should see only the new item")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for restarted worker to process item")
	}
	assert.Equal(t, 1, pool.WorkerCount())
}
