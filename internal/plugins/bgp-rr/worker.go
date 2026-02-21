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
}

// worker is a single long-lived goroutine processing items for one source-peer key.
type worker struct {
	ch      chan workItem
	done    chan struct{} // closed when goroutine exits
	pending atomic.Int32  // items about to be sent (between mu.Unlock and channel send)
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

	// stopCh is closed by Stop() to unblock any Dispatch goroutine
	// that is blocked on a full channel send.
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
		cfg.chanSize = 64
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
// Blocks if the channel is full (backpressure on the caller).
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
			ch:   make(chan workItem, wp.cfg.chanSize),
			done: make(chan struct{}),
		}
		wp.workers[key] = w
		wp.count.Add(1)
		go wp.runWorker(key, w)
	}

	// Increment pending BEFORE releasing the lock. The idle handler checks
	// pending under the same lock, so it will see pending > 0 and not exit
	// even if the channel is momentarily empty.
	w.pending.Add(1)
	wp.mu.Unlock()

	// Blocking send: every cached UPDATE must be forwarded or released
	// (CacheConsumer protocol). Dropping is not acceptable. If the channel
	// is full, this blocks until the worker drains one item. The stopCh
	// escape prevents deadlock during shutdown.
	select {
	case w.ch <- item:
		w.pending.Add(-1)
	case <-wp.stopCh:
		w.pending.Add(-1)
		return false
	}

	// Check backpressure after send (informational only).
	// Only log on transition into backpressure state (not while already in it).
	if len(w.ch)*4 > cap(w.ch)*3 { // >75% full
		if _, alreadySet := wp.backpressure.LoadOrStore(key, true); !alreadySet {
			logger().Warn("backpressure",
				"source-peer", key.sourcePeer,
				"queue-depth", len(w.ch),
				"capacity", cap(w.ch),
			)
		}
		wp.inBackpressure.Store(key, true)
	}

	return true
}

// PeerDown closes the worker for the given source peer.
// The worker drains remaining items and exits.
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

	close(w.ch)
	<-w.done
}

// Stop closes all workers and waits for them to drain.
// Closes stopCh first to unblock any Dispatch blocked on a full channel.
func (wp *workerPool) Stop() {
	wp.mu.Lock()
	wp.stopped = true

	// Unblock any Dispatch waiting on a full channel send.
	select {
	case <-wp.stopCh:
		// Already closed — Stop called twice (idempotent).
	default: // First Stop call — close to unblock pending sends.
		close(wp.stopCh)
	}

	all := make([]*worker, 0, len(wp.workers))
	for key, w := range wp.workers {
		all = append(all, w)
		delete(wp.workers, key)
	}
	wp.mu.Unlock()

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
				// Channel closed (PeerDown or Stop) — drain remaining items.
				return
			}
			if !idle.Stop() {
				drainTimer(idle)
			}
			wp.safeHandle(key, item)

			// Low-water check: if channel drained below 25% and was in backpressure,
			// fire onLowWater callback to trigger resume. The inBackpressure flag is
			// consumed (deleted) so the callback fires once per high→low transition.
			if len(w.ch)*4 < cap(w.ch) { // <25% full
				if _, wasBP := wp.inBackpressure.LoadAndDelete(key); wasBP && wp.onLowWater != nil {
					wp.onLowWater(key)
				}
			}

			idle.Reset(wp.cfg.idleTimeout)

		case <-idle.C:
			// Idle timeout — remove self from pool and exit.
			// Check channel AND pending counter under lock: Dispatch increments
			// pending under the same lock before sending, so if pending > 0, a
			// send is in flight and we must not exit.
			wp.mu.Lock()
			if len(w.ch) > 0 || w.pending.Load() > 0 {
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
