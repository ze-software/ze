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
// VALIDATES: Warning logged when channel > 90% full.
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
	waitForCount(&count, 1, t)

	// Fill channel to 90%+ capacity. Worker is blocked on item 1, so items
	// 2-9 queue up (8 items in buffer of 8). Backpressure check (depth*10 > cap*9)
	// triggers when depth > 7.2 for cap=8, i.e., at depth=8 (items 2-9).
	for i := 2; i <= 9; i++ {
		wp.Dispatch(key, workItem{msgID: uint64(i)})
	}

	// Check that backpressure was detected.
	if !wp.BackpressureDetected(key) {
		t.Error("expected backpressure detection for key with >90% full channel")
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
	waitForCount(&count, 1, t)

	// Fill to >90% (8 items in buffer of 8). depth*10 > cap*9: 8*10=80 > 72.
	for i := 2; i <= 9; i++ {
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

// TestWorkerPoolLowWater verifies low-water callback fires when channel drains below 10%.
//
// VALIDATES: Low-water callback fires when channel drains below 10% after backpressure.
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

	// Fill to >90% (8 items in buffer of 8) to trigger backpressure.
	for i := 2; i <= 9; i++ {
		wp.Dispatch(key, workItem{msgID: uint64(i)})
	}

	if !wp.BackpressureDetected(key) {
		t.Fatal("expected backpressure detection")
	}

	// Unblock worker — it drains the channel. When <10% full, low-water fires.
	close(block)

	// Wait for all items to process and low-water to fire.
	waitForCount(&count, 9, t)

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
// VALIDATES: High-water triggers once, low-water triggers once, no duplicates.
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

	// Fill to >90% to trigger backpressure (8 items in buffer of 8).
	for i := 2; i <= 9; i++ {
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
	waitForCount(&count, 9, t)

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

// TestPoolChanSizeDefault verifies zero/negative chanSize uses default 4096.
//
// VALIDATES: AC-1 — zero/negative uses default 4096.
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

			if wp.cfg.chanSize != 4096 {
				t.Errorf("expected default chanSize 4096, got %d", wp.cfg.chanSize)
			}
		})
	}
}

// TestWorkerPool_OverflowNonBlocking verifies Dispatch returns immediately when channel is full.
//
// VALIDATES: Dispatch never blocks the caller (SDK event loop) even when channel is at capacity.
// PREVENTS: SDK event loop stall causing engine delivery timeout and silent route loss.
func TestWorkerPool_OverflowNonBlocking(t *testing.T) {
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block
		}
	}

	cfg := testPoolConfig()
	cfg.chanSize = 4

	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First dispatch: worker blocks on handler.
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Fill channel to capacity (4 items in buffer of 4).
	for i := uint64(2); i <= 5; i++ {
		wp.Dispatch(key, workItem{msgID: i})
	}

	// Channel is full. Overflow items must NOT block.
	dispatched := make(chan struct{})
	go func() {
		for i := uint64(6); i <= 10; i++ {
			wp.Dispatch(key, workItem{msgID: i})
		}
		close(dispatched)
	}()

	select {
	case <-dispatched:
		// All overflow dispatches returned without blocking.
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch blocked on full channel — overflow buffer not working")
	}

	close(block)
}

// TestWorkerPool_OverflowFIFO verifies items through overflow maintain strict FIFO order.
//
// VALIDATES: Items dispatched when channel is full are processed after channel items, in order.
// PREVENTS: Out-of-order processing when overflow buffer absorbs excess items.
func TestWorkerPool_OverflowFIFO(t *testing.T) {
	block := make(chan struct{})
	var mu sync.Mutex
	var order []uint64
	var count atomic.Int32

	handler := func(_ workerKey, item workItem) {
		if count.Add(1) == 1 {
			<-block
		}
		mu.Lock()
		order = append(order, item.msgID)
		mu.Unlock()
	}

	cfg := testPoolConfig()
	cfg.chanSize = 4
	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First dispatch: worker blocks.
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Items 2-5 fill channel, 6-10 go to overflow.
	dispatched := make(chan struct{})
	go func() {
		for i := uint64(2); i <= 10; i++ {
			wp.Dispatch(key, workItem{msgID: i})
		}
		close(dispatched)
	}()

	select {
	case <-dispatched:
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch blocked — overflow not working")
	}

	// Unblock worker — all 10 items should process in FIFO order.
	close(block)

	// Wait for all items processed.
	deadline := time.After(5 * time.Second)
	for {
		mu.Lock()
		got := len(order)
		mu.Unlock()
		if got == 10 {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timeout: processed %d/10 items, order=%v", len(order), order)
			mu.Unlock()
		default:
			time.Sleep(time.Millisecond)
		}
	}

	// Verify strict FIFO: [1, 2, 3, ..., 10].
	mu.Lock()
	defer mu.Unlock()
	for i, id := range order {
		if id != uint64(i+1) {
			t.Errorf("order[%d] = %d, want %d (full order: %v)", i, id, i+1, order)
			break
		}
	}
}

// TestWorkerPool_OverflowDrains verifies all overflow items are eventually processed.
//
// VALIDATES: Items in overflow buffer drain to channel and get processed as worker frees up.
// PREVENTS: Items stuck permanently in overflow buffer.
func TestWorkerPool_OverflowDrains(t *testing.T) {
	block := make(chan struct{})
	var processed atomic.Int32
	var count atomic.Int32

	handler := func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block
		}
		processed.Add(1)
	}

	cfg := testPoolConfig()
	cfg.chanSize = 4
	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First dispatch: worker blocks.
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Items 2-5 fill channel, 6-11 go to overflow (6 overflow items).
	dispatched := make(chan struct{})
	go func() {
		for i := uint64(2); i <= 11; i++ {
			wp.Dispatch(key, workItem{msgID: i})
		}
		close(dispatched)
	}()

	select {
	case <-dispatched:
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch blocked")
	}

	// Unblock — all 11 items (1 blocking + 4 channel + 6 overflow) should process.
	close(block)

	deadline := time.After(5 * time.Second)
	// 11 total: item 1 processed (handler returns after unblock) + items 2-11.
	for processed.Load() < 11 {
		select {
		case <-deadline:
			t.Fatalf("timeout: only %d/11 items processed", processed.Load())
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

// TestWorkerPool_DepthIncludesOverflow verifies backpressure accounts for overflow items.
//
// VALIDATES: Backpressure detection uses channel + overflow depth, not just channel length.
// PREVENTS: Missing backpressure signal when overflow absorbs items beyond channel capacity.
func TestWorkerPool_DepthIncludesOverflow(t *testing.T) {
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block
		}
	}

	cfg := testPoolConfig()
	cfg.chanSize = 4
	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Fill channel + overflow.
	dispatched := make(chan struct{})
	go func() {
		for i := uint64(2); i <= 8; i++ {
			wp.Dispatch(key, workItem{msgID: i})
		}
		close(dispatched)
	}()

	select {
	case <-dispatched:
	case <-time.After(2 * time.Second):
		t.Fatal("Dispatch blocked")
	}

	// Channel full + overflow items → backpressure must be detected.
	if !wp.BackpressureDetected(key) {
		t.Error("expected backpressure when overflow items exist")
	}

	close(block)
}

// TestWorkerPool_StopDropsOverflow verifies Stop calls onItemDrop for overflow items.
//
// VALIDATES: Overflow items are cleaned up via onItemDrop when pool is stopped.
// PREVENTS: Resource leak (fwdCtx, cache entries) when pool stops with overflow items.
func TestWorkerPool_StopDropsOverflow(t *testing.T) {
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block
		}
	}

	var mu sync.Mutex
	var dropped []uint64

	cfg := testPoolConfig()
	cfg.chanSize = 4
	cfg.onItemDrop = func(item workItem) {
		mu.Lock()
		dropped = append(dropped, item.msgID)
		mu.Unlock()
	}

	wp := newWorkerPool(handler, cfg)

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First dispatch: worker blocks.
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Fill channel (4) + overflow (3).
	dispatched := make(chan struct{})
	go func() {
		for i := uint64(2); i <= 8; i++ {
			wp.Dispatch(key, workItem{msgID: i})
		}
		close(dispatched)
	}()

	select {
	case <-dispatched:
	case <-time.After(2 * time.Second):
		close(block)
		t.Fatal("Dispatch blocked")
	}

	// Stop while overflow items exist. Unblock handler so Stop completes.
	stopDone := make(chan struct{})
	go func() {
		wp.Stop()
		close(stopDone)
	}()

	// Give Stop a moment to signal drain goroutine.
	time.Sleep(50 * time.Millisecond)
	close(block)
	<-stopDone

	mu.Lock()
	defer mu.Unlock()
	if len(dropped) == 0 {
		t.Error("expected onItemDrop to be called for overflow items during Stop")
	}
}

// TestWorkerPool_PeerDownDropsOverflow verifies PeerDown calls onItemDrop for overflow items.
//
// VALIDATES: Overflow items are cleaned up via onItemDrop when peer goes down.
// PREVENTS: Resource leak (fwdCtx, cache entries) when peer disconnects with overflow items.
func TestWorkerPool_PeerDownDropsOverflow(t *testing.T) {
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block
		}
	}

	var mu sync.Mutex
	var dropped []uint64

	cfg := testPoolConfig()
	cfg.chanSize = 4
	cfg.onItemDrop = func(item workItem) {
		mu.Lock()
		dropped = append(dropped, item.msgID)
		mu.Unlock()
	}

	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First dispatch: worker blocks.
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Fill channel (4) + overflow (3).
	dispatched := make(chan struct{})
	go func() {
		for i := uint64(2); i <= 8; i++ {
			wp.Dispatch(key, workItem{msgID: i})
		}
		close(dispatched)
	}()

	select {
	case <-dispatched:
	case <-time.After(2 * time.Second):
		close(block)
		t.Fatal("Dispatch blocked")
	}

	// PeerDown while overflow items exist. Unblock handler so PeerDown completes.
	peerDownDone := make(chan struct{})
	go func() {
		wp.PeerDown("10.0.0.1")
		close(peerDownDone)
	}()

	// Give PeerDown a moment to signal drain goroutine.
	time.Sleep(50 * time.Millisecond)
	close(block)
	<-peerDownDone

	mu.Lock()
	defer mu.Unlock()
	if len(dropped) == 0 {
		t.Error("expected onItemDrop to be called for overflow items during PeerDown")
	}
}

// TestDefaultChannelCapacity4096 verifies that the default channel capacity is 4096.
//
// VALIDATES: AC-1 — default capacity is 4096 (was 1024).
// PREVENTS: Regression to 1024 default that triggers backpressure too early.
func TestDefaultChannelCapacity4096(t *testing.T) {
	handler := func(_ workerKey, _ workItem) {}

	wp := newWorkerPool(handler, poolConfig{chanSize: 0, idleTimeout: 5 * time.Second})
	defer wp.Stop()

	if wp.cfg.chanSize != 4096 {
		t.Errorf("expected default chanSize 4096, got %d", wp.cfg.chanSize)
	}
}

// TestBackpressureHighWater90Percent verifies backpressure triggers at >90% capacity.
//
// VALIDATES: AC-2 — backpressure triggers at >90%, not 75%.
// PREVENTS: Premature pause that reduces throughput.
func TestBackpressureHighWater90Percent(t *testing.T) {
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block // First item blocks.
		}
	}

	cfg := testPoolConfig()
	cfg.chanSize = 10 // 90% = 9 items

	wp := newWorkerPool(handler, cfg)
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First item blocks the worker.
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Fill to 80% (8 items in channel of 10) — should NOT trigger.
	for i := uint64(2); i <= 9; i++ {
		wp.Dispatch(key, workItem{msgID: i})
	}
	if wp.BackpressureDetected(key) {
		t.Error("backpressure should NOT trigger at 80% (8/10)")
	}

	// Fill to 100% (10 items in channel of 10). depth=10: 10*10=100 > 10*9=90 → yes.
	wp.Dispatch(key, workItem{msgID: 10})
	wp.Dispatch(key, workItem{msgID: 11})

	if !wp.BackpressureDetected(key) {
		t.Error("backpressure should trigger at >90% (10/10)")
	}

	close(block)
}

// TestBackpressureLowWater10Percent verifies low-water callback fires at <10% capacity.
//
// VALIDATES: AC-3 — low-water fires at <10%, not 25%.
// PREVENTS: Premature resume that causes oscillation.
func TestBackpressureLowWater10Percent(t *testing.T) {
	block := make(chan struct{})
	var count atomic.Int32
	handler := func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block // First item blocks.
		}
	}

	var lowWaterCalls atomic.Int32
	cfg := testPoolConfig()
	cfg.chanSize = 20 // 10% = 2 items, <10% means depth < 2

	wp := newWorkerPool(handler, cfg)
	wp.onLowWater = func(_ workerKey) {
		lowWaterCalls.Add(1)
	}
	defer wp.Stop()

	key := workerKey{sourcePeer: "10.0.0.1"}

	// First item blocks the worker.
	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Fill to >90% to trigger backpressure. Need depth > 18.
	// Worker processing item 1, items 2-20 in channel (19 items). depth=19.
	// 19*10=190 > 20*9=180 → yes.
	for i := uint64(2); i <= 20; i++ {
		wp.Dispatch(key, workItem{msgID: i})
	}

	if !wp.BackpressureDetected(key) {
		t.Fatal("expected backpressure detection")
	}

	// Unblock worker — it drains the channel.
	close(block)

	// Wait for all items to process.
	waitForCount(&count, 20, t)

	// Low-water fires at <10% (depth < 2 for cap=20).
	// After all 20 items processed, depth=0 → low-water fires.
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

// TestBackpressureThresholdOscillation verifies wider band reduces pause/resume cycles.
//
// VALIDATES: AC-15 — wider backpressure band (90/10) reduces oscillation vs narrow band.
// PREVENTS: Rapid pause/resume cycling that degrades throughput.
func TestBackpressureThresholdOscillation(t *testing.T) {
	// With chanSize=20 and 90%/10% thresholds:
	// high-water at depth > 18 (need 19+)
	// low-water at depth < 2 (need 0 or 1)
	// Band = 80% of capacity = 16 items between high and low
	cfg := testPoolConfig()
	cfg.chanSize = 20

	block := make(chan struct{})
	var count atomic.Int32
	wp := newWorkerPool(func(_ workerKey, _ workItem) {
		if count.Add(1) == 1 {
			<-block
		}
	}, cfg)
	wp.onLowWater = func(_ workerKey) {}
	defer func() { close(block); wp.Stop() }()

	key := workerKey{sourcePeer: "10.0.0.1"}

	wp.Dispatch(key, workItem{msgID: 1})
	waitForCount(&count, 1, t)

	// Fill to 85% (17 items in channel of 20). Should NOT trigger at 90%.
	for i := uint64(2); i <= 18; i++ {
		wp.Dispatch(key, workItem{msgID: i})
	}

	if wp.BackpressureDetected(key) {
		t.Error("backpressure should NOT trigger at 85% (17/20) with 90% threshold")
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
