package bgp_rr

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWorkerPool_LazyCreation verifies workers are created on first work item.
//
// VALIDATES: No goroutine spawned until first Dispatch for a key (AC-11).
// PREVENTS: Eagerly allocating goroutines for keys that may never receive traffic.
func TestWorkerPool_LazyCreation(t *testing.T) {
	var count atomic.Int32
	handler := func(key workerKey, item workItem) {
		count.Add(1)
	}

	wp := newWorkerPool(handler, testPoolConfig())
	defer wp.Stop()

	// Before dispatch: no workers.
	if wp.WorkerCount() != 0 {
		t.Fatalf("expected 0 workers before dispatch, got %d", wp.WorkerCount())
	}

	// First dispatch creates the worker.
	wp.Dispatch(workerKey{sourcePeer: "10.0.0.1"}, workItem{msgID: 1})

	// Wait for processing.
	waitForCount(&count, 1, t)

	if wp.WorkerCount() != 1 {
		t.Errorf("expected 1 worker after first dispatch, got %d", wp.WorkerCount())
	}
}

// TestWorkerPool_IdleCooldown verifies workers exit after idle timeout.
//
// VALIDATES: Worker exits after idle timeout; recreated on next traffic (AC-12).
// PREVENTS: Goroutine accumulation from transient traffic patterns.
func TestWorkerPool_IdleCooldown(t *testing.T) {
	var count atomic.Int32
	handler := func(key workerKey, item workItem) {
		count.Add(1)
	}

	cfg := testPoolConfig()
	cfg.idleTimeout = 50 * time.Millisecond // Short for testing.

	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	if wp.WorkerCount() != 1 {
		t.Fatalf("expected 1 worker, got %d", wp.WorkerCount())
	}

	// Wait for idle timeout + margin.
	time.Sleep(150 * time.Millisecond)

	if wp.WorkerCount() != 0 {
		t.Errorf("expected 0 workers after idle timeout, got %d", wp.WorkerCount())
	}

	// New dispatch recreates the worker.
	wp.Dispatch(key, workItem{msgID: 2})
	waitForCount(&count, 2, t)

	if wp.WorkerCount() != 1 {
		t.Errorf("expected 1 worker after re-dispatch, got %d", wp.WorkerCount())
	}
}

// TestWorkerPool_PeerDown verifies the worker for a peer drains and exits on PeerDown.
//
// VALIDATES: Source peer going down drains its worker; other peers unaffected (AC-13).
// PREVENTS: Goroutine leak when source peer disconnects.
func TestWorkerPool_PeerDown(t *testing.T) {
	var count atomic.Int32
	handler := func(key workerKey, item workItem) {
		count.Add(1)
	}

	wp := newWorkerPool(handler, testPoolConfig())
	defer wp.Stop()

	// Create workers for 3 source peers.
	for _, p := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		wp.Dispatch(workerKey{sourcePeer: p}, workItem{msgID: 1})
	}
	waitForCount(&count, 3, t)

	if wp.WorkerCount() != 3 {
		t.Fatalf("expected 3 workers, got %d", wp.WorkerCount())
	}

	// Peer down for 10.0.0.1: only that worker should drain and exit.
	wp.PeerDown("10.0.0.1")

	// Wait for one worker to exit.
	deadline := time.After(2 * time.Second)
	for wp.WorkerCount() != 2 {
		select {
		case <-deadline:
			t.Fatalf("expected 2 workers after PeerDown, got %d", wp.WorkerCount())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// TestWorkerPool_BackpressureWarning verifies log warning when channel approaches capacity.
//
// VALIDATES: Warning logged when channel > 75% full (AC-14).
// PREVENTS: Silent queue overflow without operator notification.
func TestWorkerPool_BackpressureWarning(t *testing.T) {
	// Handler that blocks until signaled — creates backpressure.
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(key workerKey, item workItem) {
		if count.Add(1) == 1 {
			<-block // First item blocks.
		}
	}

	cfg := testPoolConfig()
	cfg.chanSize = 8 // Small channel for testing.

	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// Send first item — worker starts and blocks on it.
	wp.Dispatch(key, workItem{msgID: 1})
	// Wait for worker to pick up the first item.
	deadline := time.After(2 * time.Second)
	for count.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("worker did not pick up first item")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	// Fill channel to 75%+ capacity. Worker is blocked on item 1, so items
	// 2-8 queue up (7 items in buffer of 8). Backpressure check (len*4 > cap*3)
	// triggers when len > 6 for cap=8, i.e., after item 8 is sent (len=7).
	// We stop before filling completely to avoid blocking (Dispatch blocks on full).
	for i := 2; i <= 8; i++ {
		wp.Dispatch(key, workItem{msgID: uint64(i)})
	}

	// Check that backpressure was detected.
	if !wp.BackpressureDetected(key) {
		t.Error("expected backpressure detection for key with >75% full channel")
	}

	// Unblock to clean up.
	close(block)
}

// TestWorkerPool_ParallelProcessing verifies multiple workers run concurrently.
//
// VALIDATES: 6 source peers = 6 workers process in parallel (AC-15).
// PREVENTS: Global serialization across independent source-peer workers.
func TestWorkerPool_ParallelProcessing(t *testing.T) {
	var active atomic.Int32
	var maxActive atomic.Int32
	var done sync.WaitGroup

	handler := func(key workerKey, item workItem) {
		cur := active.Add(1)
		// Track max concurrent workers.
		for {
			old := maxActive.Load()
			if cur <= old || maxActive.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond) // Simulate work.
		active.Add(-1)
		done.Done()
	}

	wp := newWorkerPool(handler, testPoolConfig())
	defer wp.Stop()

	// 6 source peers = 6 workers processing in parallel.
	keys := []workerKey{
		{sourcePeer: "10.0.0.1"},
		{sourcePeer: "10.0.0.2"},
		{sourcePeer: "10.0.0.3"},
		{sourcePeer: "10.0.0.4"},
		{sourcePeer: "10.0.0.5"},
		{sourcePeer: "10.0.0.6"},
	}

	done.Add(len(keys))
	for _, k := range keys {
		wp.Dispatch(k, workItem{msgID: 1})
	}

	done.Wait()

	if got := maxActive.Load(); got < 3 {
		t.Errorf("expected at least 3 concurrent workers, max was %d", got)
	}
}

// TestWorkerPool_FIFOWithinKey verifies commands for same key arrive in send order.
//
// VALIDATES: 100 items dispatched to same key are processed in FIFO order (AC-8).
// PREVENTS: Out-of-order processing within a single source-peer worker.
func TestWorkerPool_FIFOWithinKey(t *testing.T) {
	var mu sync.Mutex
	var order []uint64

	handler := func(key workerKey, item workItem) {
		mu.Lock()
		order = append(order, item.msgID)
		mu.Unlock()
	}

	wp := newWorkerPool(handler, testPoolConfig())
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}
	const n = 100
	for i := uint64(1); i <= n; i++ {
		wp.Dispatch(key, workItem{msgID: i})
	}

	// Wait for all items to be processed.
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		got := len(order)
		mu.Unlock()
		if got == n {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timeout: processed %d/%d items", len(order), n)
			mu.Unlock()
		default:
			time.Sleep(time.Millisecond)
		}
	}

	// Verify strict ordering.
	mu.Lock()
	defer mu.Unlock()
	for i := 1; i < len(order); i++ {
		if order[i] <= order[i-1] {
			t.Errorf("FIFO violation: order[%d]=%d <= order[%d]=%d", i, order[i], i-1, order[i-1])
		}
	}
}

// TestWorkerPool_NoSendOnClosedChannel verifies Dispatch after PeerDown doesn't panic.
//
// VALIDATES: Dispatch to a peer whose workers were closed doesn't crash (AC-5 safety).
// PREVENTS: Panic from sending on closed channel after PeerDown.
func TestWorkerPool_NoSendOnClosedChannel(t *testing.T) {
	handler := func(key workerKey, item workItem) {}

	wp := newWorkerPool(handler, testPoolConfig())
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}
	wp.Dispatch(key, workItem{msgID: 1})

	// Wait for worker to be created.
	deadline := time.After(2 * time.Second)
	for wp.WorkerCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("worker not created")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	// Close all workers for this peer.
	wp.PeerDown("10.0.0.1")

	// Wait for workers to exit.
	deadline = time.After(2 * time.Second)
	for wp.WorkerCount() != 0 {
		select {
		case <-deadline:
			t.Fatal("worker did not exit")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	// Dispatch after PeerDown — must not panic, should lazily recreate worker.
	wp.Dispatch(key, workItem{msgID: 2})

	deadline = time.After(2 * time.Second)
	for wp.WorkerCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("worker not recreated after PeerDown")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

// TestWorkerPool_StopDrains verifies Stop() drains all workers.
//
// VALIDATES: All workers exit cleanly on Stop().
// PREVENTS: Goroutine leak on plugin shutdown.
func TestWorkerPool_StopDrains(t *testing.T) {
	var count atomic.Int32
	handler := func(key workerKey, item workItem) {
		count.Add(1)
	}

	wp := newWorkerPool(handler, testPoolConfig())

	// Create workers for multiple source peers.
	for _, p := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5"} {
		wp.Dispatch(workerKey{sourcePeer: p}, workItem{msgID: 1})
	}

	// Wait for all items to process.
	waitForCount(&count, 5, t)

	wp.Stop()

	if wp.WorkerCount() != 0 {
		t.Errorf("expected 0 workers after Stop(), got %d", wp.WorkerCount())
	}
}

// TestWorkerPool_HandlerPanicRecovery verifies a panicking handler doesn't kill the worker.
//
// VALIDATES: Worker recovers from handler panic and processes subsequent items.
// PREVENTS: Dead worker goroutine with live channel entry in pool map (black hole).
func TestWorkerPool_HandlerPanicRecovery(t *testing.T) {
	var mu sync.Mutex
	var processed []uint64

	handler := func(key workerKey, item workItem) {
		if item.msgID == 1 {
			panic("simulated handler panic")
		}
		mu.Lock()
		processed = append(processed, item.msgID)
		mu.Unlock()
	}

	wp := newWorkerPool(handler, testPoolConfig())
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First dispatch: handler panics.
	wp.Dispatch(key, workItem{msgID: 1})

	// Give worker time to recover.
	time.Sleep(50 * time.Millisecond)

	// Second dispatch: should succeed (worker recovered or recreated).
	wp.Dispatch(key, workItem{msgID: 2})

	// Wait for second item to be processed.
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		got := len(processed)
		mu.Unlock()
		if got >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout: second item not processed after handler panic")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 1 || processed[0] != 2 {
		t.Errorf("expected [2], got %v", processed)
	}
}

// TestWorkerPool_BackpressureClearedAfterRead verifies BackpressureDetected resets after read.
//
// VALIDATES: BackpressureDetected returns false on second call (cleared after first read).
// PREVENTS: Permanent backpressure flag that never resets.
func TestWorkerPool_BackpressureClearedAfterRead(t *testing.T) {
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(key workerKey, item workItem) {
		if count.Add(1) == 1 {
			<-block
		}
	}

	cfg := testPoolConfig()
	cfg.chanSize = 8

	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First item blocks the worker.
	wp.Dispatch(key, workItem{msgID: 1})
	deadline := time.After(2 * time.Second)
	for count.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("worker did not pick up first item")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	// Fill to >75% (7 items in buffer of 8).
	for i := 2; i <= 8; i++ {
		wp.Dispatch(key, workItem{msgID: uint64(i)})
	}

	// First read: should be true.
	if !wp.BackpressureDetected(key) {
		t.Fatal("expected backpressure on first read")
	}

	// Second read: should be false (cleared).
	if wp.BackpressureDetected(key) {
		t.Error("expected backpressure cleared after first read")
	}

	close(block)
}

// TestWorkerPoolLowWater verifies low-water callback fires when channel drains below 25%.
//
// VALIDATES: AC-2 — low-water callback fires when channel drains below 25% after backpressure.
// PREVENTS: Resume signal never sent after pause.
func TestWorkerPoolLowWater(t *testing.T) {
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block // First item blocks.
		}
	}

	var lowWaterCalls atomic.Int32
	cfg := testPoolConfig()
	cfg.chanSize = 8

	wp := newWorkerPool(handler, cfg)
	wp.onLowWater = func(key workerKey) {
		lowWaterCalls.Add(1)
	}
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First item blocks the worker.
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Fill to >75% (7 items in buffer of 8) to trigger backpressure.
	for i := 2; i <= 8; i++ {
		wp.Dispatch(key, workItem{msgID: uint64(i)})
	}

	if !wp.BackpressureDetected(key) {
		t.Fatal("expected backpressure detection")
	}

	// Unblock worker — it drains the channel. When <25% full, low-water fires.
	close(block)

	// Wait for all items to process and low-water to fire.
	waitForCount(&count, 8, t)

	deadline := time.After(2 * time.Second)
	for lowWaterCalls.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("timeout: low-water callback never fired")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

// TestWorkerPoolHighLowCycle verifies high-water→pause, low-water→resume with no duplicate signals.
//
// VALIDATES: AC-1, AC-2 — high-water triggers once, low-water triggers once, no duplicates.
// PREVENTS: Flooding pause/resume RPCs from rapid channel oscillation.
func TestWorkerPoolHighLowCycle(t *testing.T) {
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block
		}
	}

	var lowWaterCalls atomic.Int32
	cfg := testPoolConfig()
	cfg.chanSize = 8

	wp := newWorkerPool(handler, cfg)
	wp.onLowWater = func(key workerKey) {
		lowWaterCalls.Add(1)
	}
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First item blocks the worker.
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Fill to >75% to trigger backpressure.
	for i := 2; i <= 8; i++ {
		wp.Dispatch(key, workItem{msgID: uint64(i)})
	}

	// First read: backpressure detected.
	if !wp.BackpressureDetected(key) {
		t.Fatal("expected backpressure on first read")
	}

	// Second read: cleared (no duplicate).
	if wp.BackpressureDetected(key) {
		t.Error("expected backpressure cleared after first read")
	}

	// Unblock worker to drain.
	close(block)
	waitForCount(&count, 8, t)

	// Low-water should fire exactly once.
	deadline := time.After(2 * time.Second)
	for lowWaterCalls.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("timeout: low-water callback never fired")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	// Verify only one low-water signal.
	time.Sleep(50 * time.Millisecond) // Brief pause for any extra callbacks.
	if got := lowWaterCalls.Load(); got != 1 {
		t.Errorf("expected 1 low-water callback, got %d", got)
	}
}

// TestWorkerPoolCustomChanSize verifies custom channel size is respected.
//
// VALIDATES: AC-15 — custom chanSize respected by worker creation.
// PREVENTS: Ignoring chanSize config.
func TestWorkerPoolCustomChanSize(t *testing.T) {
	handler := func(_ workerKey, _ workItem) {}

	cfg := poolConfig{chanSize: 512, idleTimeout: 5 * time.Second}
	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	if wp.cfg.chanSize != 512 {
		t.Errorf("expected chanSize 512, got %d", wp.cfg.chanSize)
	}
}

// TestPoolChanSizeDefault verifies zero/negative chanSize uses default 64.
//
// VALIDATES: AC-17, AC-18 — zero/negative uses default 64.
// PREVENTS: Panic or zero-size channel from bad config.
func TestPoolChanSizeDefault(t *testing.T) {
	handler := func(_ workerKey, _ workItem) {}

	tests := []struct {
		name string
		size int
	}{
		{name: "zero", size: 0},
		{name: "negative", size: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := poolConfig{chanSize: tt.size, idleTimeout: 5 * time.Second}
			wp := newWorkerPool(handler, cfg)
			defer wp.Stop()

			if wp.cfg.chanSize != 64 {
				t.Errorf("expected default chanSize 64, got %d", wp.cfg.chanSize)
			}
		})
	}
}

// --- Test helpers ---

func testPoolConfig() poolConfig {
	return poolConfig{
		chanSize:    64,
		idleTimeout: 5 * time.Second,
	}
}

func waitForCount(count *atomic.Int32, target int32, t *testing.T) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for count.Load() < target {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for count=%d, got %d", target, count.Load())
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
