package reactor

import (
	"sync"
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

	pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
	pool.Dispatch(fwdKey{peerAddr: "2.2.2.2"}, fwdItem{})
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

	select {
	case size := <-batchSizes:
		// New worker processed exactly 1 item — no stale data from old buffer.
		assert.Equal(t, 1, size, "restarted worker should see only the new item")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for restarted worker to process item")
	}
	assert.Equal(t, 1, pool.WorkerCount())
}

// TestFwdPool_TryDispatch verifies TryDispatch is non-blocking: succeeds when
// channel has space, returns false immediately when channel is full.
//
// VALIDATES: AC-2, AC-3
// PREVENTS: TryDispatch blocking like Dispatch when channel is full.
func TestFwdPool_TryDispatch(t *testing.T) {
	blocker := make(chan struct{})
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})
	defer pool.Stop()

	key := fwdKey{peerAddr: "1.1.1.1"}

	// First dispatch enters handler (blocks on blocker)
	ok := pool.TryDispatch(key, fwdItem{})
	require.True(t, ok, "TryDispatch should succeed on empty channel")

	time.Sleep(10 * time.Millisecond) // Let handler start blocking

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

	key := fwdKey{peerAddr: "1.1.1.1"}

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

	key := fwdKey{peerAddr: "1.1.1.1"}

	// Create worker by dispatching one item (blocks in handler)
	pool.Dispatch(key, fwdItem{})
	time.Sleep(10 * time.Millisecond)

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
	var doneCalled atomic.Int32

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second})

	key := fwdKey{peerAddr: "1.1.1.1"}

	// Create worker (blocks in handler)
	pool.Dispatch(key, fwdItem{})
	time.Sleep(10 * time.Millisecond)

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
	var congestedPeers []string
	var resumedPeers []string
	var mu sync.Mutex

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second})
	pool.onCongested = func(peerAddr string) {
		mu.Lock()
		congestedPeers = append(congestedPeers, peerAddr)
		mu.Unlock()
	}
	pool.onResumed = func(peerAddr string) {
		mu.Lock()
		resumedPeers = append(resumedPeers, peerAddr)
		mu.Unlock()
	}
	defer pool.Stop()

	key := fwdKey{peerAddr: "10.0.0.1"}

	// Fill: 1 in handler + 4 in channel = full
	pool.Dispatch(key, fwdItem{})
	time.Sleep(10 * time.Millisecond)
	for range 4 {
		pool.Dispatch(key, fwdItem{})
	}

	// TryDispatch fails -> congested callback should fire
	ok := pool.TryDispatch(key, fwdItem{})
	assert.False(t, ok)

	mu.Lock()
	assert.Equal(t, []string{"10.0.0.1"}, congestedPeers, "onCongested should fire once")
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
	assert.Equal(t, []string{"10.0.0.1"}, resumedPeers)
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

	key := fwdKey{peerAddr: "1.1.1.1"}

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

// TestFwdPool_OverflowUsesPool verifies overflow items acquire tokens from
// the global overflow pool and return them after processing.
//
// VALIDATES: AC-1 (overflow stored in pool, never dropped)
// PREVENTS: Overflow bypassing the bounded pool system.
func TestFwdPool_OverflowUsesPool(t *testing.T) {
	blocker := make(chan struct{})
	var processed atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		<-blocker
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second, overflowPoolSize: 10})
	defer pool.Stop()

	key := fwdKey{peerAddr: "1.1.1.1"}

	// Create worker (blocks in handler).
	pool.Dispatch(key, fwdItem{})
	time.Sleep(10 * time.Millisecond)

	// Add 5 overflow items -- pool should track them.
	// peer must be non-nil so items are not treated as sentinels (which skip pool acquire).
	dummyPeer := &Peer{}
	for range 5 {
		pool.DispatchOverflow(key, fwdItem{done: func() {}, peer: dummyPeer})
	}

	// Pool should have 5 tokens acquired (10 total - 5 used = 5 available).
	assert.Equal(t, 5, pool.overflowPool.available(),
		"pool should have 5 tokens acquired")

	// Unblock -- all items processed, tokens returned.
	close(blocker)

	require.Eventually(t, func() bool {
		return pool.overflowPool.available() == 10
	}, time.Second, time.Millisecond, "all pool tokens should be returned after processing")

	require.Eventually(t, func() bool {
		return processed.Load() >= 5
	}, time.Second, time.Millisecond, "all overflow items should be processed")
}

// TestFwdPool_PeerDisconnectReturnsSlots verifies Stop returns all pool
// tokens and calls done() for overflow items still queued at shutdown.
//
// VALIDATES: AC-7 (peer disconnect returns pool slots, done() called)
// PREVENTS: Pool token leaks on peer disconnect or reactor shutdown.
func TestFwdPool_PeerDisconnectReturnsSlots(t *testing.T) {
	blocker := make(chan struct{})
	var doneCalled atomic.Int32

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second, overflowPoolSize: 10})

	key := fwdKey{peerAddr: "1.1.1.1"}

	// Create worker (blocks in handler).
	pool.Dispatch(key, fwdItem{})
	time.Sleep(10 * time.Millisecond)

	// Add overflow items with done callbacks.
	// peer must be non-nil so items acquire pool tokens (nil peer = sentinel, skips pool).
	dummyPeer := &Peer{}
	for range 4 {
		pool.DispatchOverflow(key, fwdItem{
			done: func() { doneCalled.Add(1) },
			peer: dummyPeer,
		})
	}

	// Verify tokens were acquired.
	assert.Equal(t, 6, pool.overflowPool.available(),
		"4 tokens should be acquired (10 - 4 = 6 available)")

	// Stop simulates shutdown -- should return all tokens and call done().
	close(blocker)
	pool.Stop()

	assert.GreaterOrEqual(t, int(doneCalled.Load()), 4,
		"done must be called for all overflow items")
	assert.Equal(t, 10, pool.overflowPool.available(),
		"all pool tokens must be returned after Stop")
}

// TestFwdPool_PoolExhausted verifies that when the overflow pool is exhausted,
// items are still accepted (unbounded fallback) and never dropped.
// Pool tokens are returned only for the items that acquired them.
//
// VALIDATES: AC-11 (memory bounded by pool size, graceful fallback)
// PREVENTS: Panic or route drop when pool is exhausted.
func TestFwdPool_PoolExhausted(t *testing.T) {
	blocker := make(chan struct{})
	var processed atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		<-blocker
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second, overflowPoolSize: 5})
	defer pool.Stop()

	key := fwdKey{peerAddr: "1.1.1.1"}

	// Create worker (blocks in handler).
	pool.Dispatch(key, fwdItem{})
	time.Sleep(10 * time.Millisecond)

	// Add 8 overflow items -- first 5 use pool, last 3 are fallback.
	// peer must be non-nil so items acquire pool tokens (nil peer = sentinel, skips pool).
	dummyPeer := &Peer{}
	for range 8 {
		ok := pool.DispatchOverflow(key, fwdItem{done: func() {}, peer: dummyPeer})
		require.True(t, ok, "DispatchOverflow must never reject items")
	}

	// Pool fully exhausted.
	assert.Equal(t, 0, pool.overflowPool.available(),
		"pool should be fully exhausted")

	// Unblock and verify all 8 items processed (none dropped).
	close(blocker)

	require.Eventually(t, func() bool {
		return processed.Load() >= 8
	}, time.Second, time.Millisecond, "all items including fallback must be processed")

	// Pool tokens for the 5 pooled items should be returned.
	require.Eventually(t, func() bool {
		return pool.overflowPool.available() == 5
	}, time.Second, time.Millisecond, "pooled tokens should be returned after processing")
}

// TestFwdOverflowPool_DoubleRelease verifies the release() default branch
// detects and handles a double-release (more releases than acquires).
//
// VALIDATES: release() error detection branch
// PREVENTS: Silent token count corruption from lifecycle bugs.
func TestFwdOverflowPool_DoubleRelease(t *testing.T) {
	p := newFwdOverflowPool(3)
	assert.Equal(t, 3, p.available())

	// Acquire one token.
	ok := p.acquire()
	require.True(t, ok)
	assert.Equal(t, 2, p.available())

	// Release it -- back to 3.
	p.release()
	assert.Equal(t, 3, p.available())

	// Double-release -- pool should NOT grow beyond size.
	// The release() default branch logs an error and discards the extra token.
	p.release()
	assert.Equal(t, 3, p.available(), "pool must not exceed its size on double-release")
}

// TestFwdOverflowPool_ConcurrentAcquireRelease verifies token safety
// under concurrent access from multiple goroutines.
//
// VALIDATES: channel-based semaphore goroutine safety
// PREVENTS: Race conditions in acquire/release under concurrent load.
func TestFwdOverflowPool_ConcurrentAcquireRelease(t *testing.T) {
	const poolSize = 100
	const goroutines = 50
	const iterations = 100

	p := newFwdOverflowPool(poolSize)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				if p.acquire() {
					// Simulate work.
					p.release()
				}
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, poolSize, p.available(),
		"all tokens must be returned after concurrent acquire/release")
}

// TestFwdPool_DrainOverflowDirectProcessReleasesTokens verifies pool tokens
// are returned when drainOverflow processes items directly (processDirect path)
// instead of enqueuing them to the channel.
//
// VALIDATES: drainOverflow processDirect path releases pool tokens
// PREVENTS: Token leaks when overflow items exceed channel capacity during drain.
func TestFwdPool_DrainOverflowDirectProcessReleasesTokens(t *testing.T) {
	var processed atomic.Int32

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		processed.Add(int32(len(items)))
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second, overflowPoolSize: 20})
	defer pool.Stop()

	key := fwdKey{peerAddr: "1.1.1.1"}

	// Add many overflow items (more than channel capacity of 2).
	// When drainOverflow runs, some enqueue to channel, rest are processed directly.
	// peer must be non-nil so items acquire pool tokens (nil peer = sentinel, skips pool).
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

	// All 10 pool tokens should be returned (regardless of channel vs direct path).
	require.Eventually(t, func() bool {
		return pool.overflowPool.available() == 20
	}, time.Second, time.Millisecond, "all pool tokens must be returned")
}

// TestFwdPool_DispatchOverflowAfterStopWithPool verifies DispatchOverflow
// on a stopped pool with overflow pool enabled: returns false, calls done(),
// and does NOT consume a pool token.
//
// VALIDATES: DispatchOverflow stopped-pool path with pool enabled
// PREVENTS: Pool token leaks when dispatching to a stopped pool.
func TestFwdPool_DispatchOverflowAfterStopWithPool(t *testing.T) {
	var doneCalled atomic.Int32

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
	}, fwdPoolConfig{chanSize: 4, idleTimeout: time.Second, overflowPoolSize: 10})
	pool.Stop()

	ok := pool.DispatchOverflow(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{
		done: func() { doneCalled.Add(1) },
	})

	assert.False(t, ok, "DispatchOverflow must return false on stopped pool")
	assert.Equal(t, int32(1), doneCalled.Load(), "done must be called on stopped pool")
	assert.Equal(t, 10, pool.overflowPool.available(),
		"no pool tokens should be consumed on stopped pool")
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
	pool.Dispatch(fwdKey{peerAddr: "10.0.0.1"}, fwdItem{})
	pool.Dispatch(fwdKey{peerAddr: "10.0.0.2"}, fwdItem{})
	time.Sleep(10 * time.Millisecond)

	// Add overflow items to first peer only.
	for range 3 {
		pool.DispatchOverflow(fwdKey{peerAddr: "10.0.0.1"}, fwdItem{done: func() {}})
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
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {
		<-blocker
	}, fwdPoolConfig{chanSize: 2, idleTimeout: time.Second, overflowPoolSize: 10})
	defer pool.Stop()

	// No overflow items yet -- ratio should be 0.
	assert.InDelta(t, 0.0, pool.PoolUsedRatio(), 0.001, "empty pool should have 0.0 ratio")

	// Create worker and add overflow items.
	pool.Dispatch(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{})
	time.Sleep(10 * time.Millisecond)
	dummyPeer := &Peer{}
	for range 4 {
		pool.DispatchOverflow(fwdKey{peerAddr: "1.1.1.1"}, fwdItem{done: func() {}, peer: dummyPeer})
	}

	// 4 of 10 tokens used = 0.4 ratio.
	assert.InDelta(t, 0.4, pool.PoolUsedRatio(), 0.001, "4/10 tokens used should be 0.4")

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
		pool.RecordForwarded("10.0.0.1")
	}
	for range 2 {
		pool.RecordOverflowed("10.0.0.1")
	}

	// Source B: 0 forwarded, 5 overflowed = 100% overflow ratio.
	for range 5 {
		pool.RecordOverflowed("10.0.0.2")
	}

	// Source C: 10 forwarded, 0 overflowed = 0% overflow ratio.
	for range 10 {
		pool.RecordForwarded("10.0.0.3")
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
				pool.RecordForwarded("10.0.0.1")
				pool.RecordOverflowed("10.0.0.1")
			}
		}()
	}
	wg.Wait()

	ratios := pool.SourceOverflowRatios()
	// Each goroutine does equal forwarded + overflowed, so ratio should be 0.5.
	assert.InDelta(t, 0.5, ratios["10.0.0.1"], 0.001,
		"equal forwarded and overflowed should give 0.5 ratio")
}
