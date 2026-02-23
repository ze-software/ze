// Design: docs/architecture/core-design.md — route reflector plugin

package bgp_rr

import (
	"sync"
	"sync/atomic"
	"time"
)

// workerKey identifies a per-source-peer worker goroutine.
type workerKey struct {
	sourcePeer string
}

// workItem represents a unit of work dispatched to a worker.
type workItem struct {
	msgID uint64
}

// poolConfig holds configuration for a workerPool.
type poolConfig struct {
	chanSize    int
	idleTimeout time.Duration

	// onItemDrop is called for each work item discarded during PeerDown or Stop
	// when overflow items remain. Callers use this to clean up associated state
	// (e.g., fwdCtx entries, cache refs). Nil means no cleanup.
	onItemDrop func(workItem)
}

// worker is a single long-lived goroutine processing items for one source-peer key.
type worker struct {
	ch      chan workItem
	done    chan struct{} // closed when goroutine exits
	closeCh chan struct{} // closed on PeerDown/Stop to signal drain goroutine

	// overflow is an unbounded FIFO buffer for items that arrive when the
	// channel is full. A temporary drain goroutine moves items from overflow
	// to the channel as space opens. Protected by overflowMu.
	overflowMu sync.Mutex
	overflow   []workItem
	draining   bool // drain goroutine is active

	// drainWg tracks the drain goroutine. PeerDown/Stop must Wait() on this
	// before closing w.ch to prevent "send on closed channel" panics.
	drainWg sync.WaitGroup
}

// overflowLen returns the number of items in the overflow buffer.
func (w *worker) overflowLen() int {
	w.overflowMu.Lock()
	n := len(w.overflow)
	w.overflowMu.Unlock()
	return n
}

// depth returns the total number of pending items: channel + overflow.
func (w *worker) depth() int {
	w.overflowMu.Lock()
	n := len(w.overflow)
	w.overflowMu.Unlock()
	return len(w.ch) + n
}

// workerPool manages per-source-peer worker goroutines.
// Workers are created lazily on first Dispatch and exit after idle timeout.
// Each key has exactly one worker goroutine — FIFO ordering is preserved per key.
//
// Concurrency constraint: Dispatch and PeerDown/Stop must not be called
// concurrently for the same sourcePeer. In the current design this is guaranteed
// because both are called from the OnEvent handler (single goroutine).
type workerPool struct {
	mu      sync.Mutex
	workers map[workerKey]*worker
	handler func(key workerKey, item workItem)
	cfg     poolConfig
	stopped bool

	// stopCh is closed by Stop() to signal shutdown.
	stopCh chan struct{}

	// backpressure tracks keys that have triggered backpressure warnings.
	// Used by BackpressureDetected (clear-on-read for dispatch polling).
	backpressure sync.Map // workerKey → bool

	// inBackpressure tracks keys currently in backpressure state.
	// Used by low-water check (cleared when channel drains below 25%).
	// Separate from backpressure because BackpressureDetected clears on read.
	inBackpressure sync.Map // workerKey → bool

	// onLowWater is called when a worker's channel drops below 25% after
	// being in backpressure. Used by dispatch to trigger resume RPCs.
	onLowWater func(key workerKey)

	// count tracks active workers for WorkerCount() without holding mu.
	count atomic.Int32
}

// newWorkerPool creates a new worker pool with the given handler and configuration.
func newWorkerPool(handler func(key workerKey, item workItem), cfg poolConfig) *workerPool {
	if cfg.chanSize <= 0 {
		cfg.chanSize = 1024
	}
	if cfg.idleTimeout <= 0 {
		cfg.idleTimeout = 5 * time.Second
	}
	return &workerPool{
		workers: make(map[workerKey]*worker),
		handler: handler,
		cfg:     cfg,
		stopCh:  make(chan struct{}),
	}
}

// Dispatch sends a work item to the worker for the given key.
// Creates the worker lazily if it doesn't exist.
// Never blocks: if the channel is full, the item goes to an overflow buffer
// and a drain goroutine feeds it into the channel as space opens.
// Returns true if the item was enqueued, false if the pool is stopped.
// Callers must clean up associated state (e.g., fwdCtx, cache entries) on false.
func (wp *workerPool) Dispatch(key workerKey, item workItem) bool {
	wp.mu.Lock()
	if wp.stopped {
		wp.mu.Unlock()
		return false
	}

	w, ok := wp.workers[key]
	if !ok {
		w = &worker{
			ch:      make(chan workItem, wp.cfg.chanSize),
			done:    make(chan struct{}),
			closeCh: make(chan struct{}),
		}
		wp.workers[key] = w
		wp.count.Add(1)
		go wp.runWorker(key, w)
	}
	wp.mu.Unlock()

	// Non-blocking enqueue. If overflow is non-empty, all new items must go
	// through overflow to preserve FIFO order. Otherwise, try channel first.
	w.overflowMu.Lock()
	if len(w.overflow) > 0 {
		// Overflow active — append to maintain FIFO.
		w.overflow = append(w.overflow, item)
		w.overflowMu.Unlock()
	} else {
		// Try non-blocking channel send.
		select {
		case w.ch <- item: // Channel has space — sent directly.
			w.overflowMu.Unlock()
			wp.checkBackpressure(key, w)
			return true
		default: // Channel full — start overflow.
			w.overflow = append(w.overflow, item)
			wp.ensureDraining(key, w)
			w.overflowMu.Unlock()
		}
	}

	wp.checkBackpressure(key, w)
	return true
}

// ensureDraining starts the drain goroutine if not already running.
// Must be called with w.overflowMu held.
func (wp *workerPool) ensureDraining(_ workerKey, w *worker) {
	if w.draining {
		return
	}
	w.draining = true
	w.drainWg.Add(1)
	go wp.drainLoop(w)
}

// drainLoop is a temporary goroutine that moves items from the overflow buffer
// into the channel as space opens. Exits when overflow is empty or closeCh fires.
func (wp *workerPool) drainLoop(w *worker) {
	defer w.drainWg.Done()

	for {
		w.overflowMu.Lock()
		if len(w.overflow) == 0 {
			w.draining = false
			w.overflowMu.Unlock()
			return
		}
		item := w.overflow[0]
		w.overflow = w.overflow[1:]
		w.overflowMu.Unlock()

		select {
		case w.ch <- item: // Item moved from overflow to channel.
		case <-w.closeCh: // Worker shutting down — drop remaining overflow.
			wp.dropItem(item)
			w.overflowMu.Lock()
			for _, remaining := range w.overflow {
				wp.dropItem(remaining)
			}
			w.overflow = nil
			w.draining = false
			w.overflowMu.Unlock()
			return
		}
	}
}

// dropItem calls the onItemDrop callback if configured.
func (wp *workerPool) dropItem(item workItem) {
	if wp.cfg.onItemDrop != nil {
		wp.cfg.onItemDrop(item)
	}
}

// checkBackpressure checks if the worker's depth has crossed the high-water mark.
// Log only on transition into backpressure (not while already in it).
func (wp *workerPool) checkBackpressure(key workerKey, w *worker) {
	if w.depth()*4 > cap(w.ch)*3 { // >75% of channel capacity
		if _, alreadyInBP := wp.inBackpressure.LoadOrStore(key, true); !alreadyInBP {
			logger().Warn("backpressure",
				"source-peer", key.sourcePeer,
				"queue-depth", w.depth(),
				"capacity", cap(w.ch),
			)
		}
		wp.backpressure.Store(key, true)
	}
}

// PeerDown closes the worker for the given source peer.
// Signals the drain goroutine to stop, waits for it, then closes the channel.
// The worker drains remaining channel items and exits.
func (wp *workerPool) PeerDown(sourcePeer string) {
	wp.mu.Lock()
	key := workerKey{sourcePeer: sourcePeer}
	w, ok := wp.workers[key]
	if ok {
		delete(wp.workers, key)
	}
	wp.mu.Unlock()

	if !ok {
		return
	}

	// Signal drain goroutine to stop, wait for it to exit, then close channel.
	close(w.closeCh)
	w.drainWg.Wait()
	close(w.ch)
	<-w.done
}

// Stop closes all workers and waits for them to drain.
// Closes stopCh first, then signals each worker's drain goroutine to stop.
func (wp *workerPool) Stop() {
	wp.mu.Lock()
	wp.stopped = true

	select {
	case <-wp.stopCh:
		// Already closed — Stop called twice (idempotent).
	default: // First Stop call — close to signal shutdown.
		close(wp.stopCh)
	}

	all := make([]*worker, 0, len(wp.workers))
	for key, w := range wp.workers {
		all = append(all, w)
		delete(wp.workers, key)
	}
	wp.mu.Unlock()

	// Signal all drain goroutines to stop.
	for _, w := range all {
		close(w.closeCh)
	}
	// Wait for all drain goroutines to exit before closing channels.
	for _, w := range all {
		w.drainWg.Wait()
	}
	for _, w := range all {
		close(w.ch)
	}
	for _, w := range all {
		<-w.done
	}
}

// WorkerCount returns the number of active workers.
func (wp *workerPool) WorkerCount() int {
	return int(wp.count.Load())
}

// BackpressureDetected returns true if the channel for the given key has
// triggered a backpressure warning (>75% full) since the last check.
// Clears the flag on read so the caller sees each backpressure event once.
func (wp *workerPool) BackpressureDetected(key workerKey) bool {
	_, ok := wp.backpressure.LoadAndDelete(key)
	return ok
}

// safeHandle calls the handler with panic recovery. If the handler panics,
// the panic is logged and the worker continues processing subsequent items.
// Without recovery, a panicking handler kills the worker goroutine while its
// entry stays in the pool map — subsequent dispatches send to a dead channel.
func (wp *workerPool) safeHandle(key workerKey, item workItem) {
	defer func() {
		if r := recover(); r != nil {
			logger().Error("worker handler panic",
				"source-peer", key.sourcePeer,
				"msgID", item.msgID,
				"panic", r,
			)
		}
	}()
	wp.handler(key, item)
}

// drainTimer drains a stopped timer's channel to prevent stale fires.
// Standard Go pattern: after timer.Stop() returns false, the channel may
// have a pending value that must be consumed before Reset.
func drainTimer(t *time.Timer) {
	select {
	case <-t.C:
	default: // Timer already drained or hadn't fired — safe to skip.
	}
}

// runWorker is the long-lived goroutine for one source-peer key.
// It reads items from the channel, calls the handler, and exits on idle timeout
// or channel close (PeerDown/Stop).
func (wp *workerPool) runWorker(key workerKey, w *worker) {
	defer func() {
		wp.count.Add(-1)
		close(w.done)
	}()

	idle := time.NewTimer(wp.cfg.idleTimeout)
	defer idle.Stop()

	for {
		select {
		case item, ok := <-w.ch:
			if !ok {
				// Channel closed (PeerDown or Stop) — exit.
				return
			}
			if !idle.Stop() {
				drainTimer(idle)
			}
			wp.safeHandle(key, item)

			// Low-water check: if total depth (channel + overflow) drained below
			// 25% of channel capacity and was in backpressure, fire onLowWater
			// callback to trigger resume. The inBackpressure flag is consumed
			// (deleted) so the callback fires once per high→low transition.
			if w.depth()*4 < cap(w.ch) { // <25% of channel capacity
				if _, wasBP := wp.inBackpressure.LoadAndDelete(key); wasBP && wp.onLowWater != nil {
					wp.onLowWater(key)
				}
			}

			idle.Reset(wp.cfg.idleTimeout)

		case <-idle.C:
			// Idle timeout — remove self from pool and exit.
			// Check channel AND overflow under lock: if either has items
			// or overflow is draining, we must not exit.
			wp.mu.Lock()
			if len(w.ch) > 0 || w.overflowLen() > 0 {
				wp.mu.Unlock()
				idle.Reset(wp.cfg.idleTimeout)
				continue
			}
			// Only delete if this worker is still the registered one
			// (PeerDown may have already removed it).
			if wp.workers[key] == w {
				delete(wp.workers, key)
			}
			wp.mu.Unlock()
			return
		}
	}
}
