// Design: docs/architecture/core-design.md — BGP reactor event loop

package reactor

import (
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// fwdLogger returns the lazy logger for forward pool warnings.
var fwdLogger = slogutil.LazyLogger("bgp.reactor.forward")

// fwdKey identifies a per-destination-peer forward worker.
type fwdKey struct {
	peerAddr string
}

// fwdItem is a unit of work dispatched to a forward worker.
// Pre-computed send operations for one destination peer from one ForwardUpdate call.
// The worker executes rawBodies (SendRawUpdateBody) then updates (SendUpdate).
type fwdItem struct {
	rawBodies [][]byte          // Zero-copy or split pieces: SendRawUpdateBody per entry
	updates   []*message.Update // Re-encode path: SendUpdate per entry
	peer      *Peer             // Target peer for all operations
	done      func()            // Called after all ops complete (Release cache entry)
}

// fwdBatchHandler executes pre-computed send operations for a batch of fwdItems.
// Acquires the session write lock once, writes all messages to bufWriter, flushes once.
// On first write error, remaining items in the batch are skipped.
// Errors are logged but not propagated — TCP failures trigger FSM disconnect independently.
func fwdBatchHandler(_ fwdKey, items []fwdItem) {
	if len(items) == 0 {
		return
	}

	peer := items[0].peer

	peer.mu.RLock()
	session := peer.session
	peer.mu.RUnlock()

	if session == nil {
		return
	}

	session.mu.RLock()
	state := session.fsm.State()
	conn := session.conn
	session.mu.RUnlock()

	if state != fsm.StateEstablished || conn == nil {
		return
	}

	session.writeMu.Lock()
	defer session.writeMu.Unlock()

	for _, item := range items {
		for _, body := range item.rawBodies {
			if err := session.writeRawUpdateBody(body); err != nil {
				fwdLogger().Warn("forward batch write failed",
					"peer", peer.Settings().Address,
					"err", err,
				)
				return
			}
		}
		for _, update := range item.updates {
			if err := session.writeUpdate(update); err != nil {
				fwdLogger().Warn("forward batch write failed",
					"peer", peer.Settings().Address,
					"err", err,
				)
				return
			}
		}
	}

	if err := session.flushWrites(); err != nil {
		fwdLogger().Warn("forward batch flush failed",
			"peer", peer.Settings().Address,
			"err", err,
		)
	}
}

// fwdPoolConfig holds configuration for a fwdPool.
type fwdPoolConfig struct {
	chanSize    int
	idleTimeout time.Duration
}

// fwdWorker is a single long-lived goroutine processing items for one destination peer.
type fwdWorker struct {
	ch       chan fwdItem
	done     chan struct{} // closed when goroutine exits
	pending  atomic.Int32  // items about to be sent (between mu.Unlock and channel send)
	batchBuf []fwdItem     // reusable drain buffer — owned by runWorker goroutine
}

// fwdPool manages per-destination-peer worker goroutines for async UPDATE forwarding.
// Workers are created lazily on first Dispatch and exit after idle timeout.
// Each key has exactly one worker goroutine — FIFO ordering is preserved per key.
//
// Unlike bgp-rs/workerPool (single-goroutine caller), fwdPool supports concurrent
// Dispatch and Stop from different goroutines (RPC workers vs reactor shutdown).
type fwdPool struct {
	mu      sync.Mutex
	workers map[fwdKey]*fwdWorker
	handler func(key fwdKey, items []fwdItem)
	cfg     fwdPoolConfig
	clock   clock.Clock
	stopped bool

	// stopCh is closed by Stop() to unblock any Dispatch goroutine
	// that is blocked on a full channel send.
	stopCh chan struct{}

	// dispatchWG tracks in-flight Dispatch calls. Stop waits for all
	// Dispatches to exit the select before closing worker channels.
	// This prevents the race between w.ch<-item and close(w.ch).
	dispatchWG sync.WaitGroup

	// count tracks active workers for WorkerCount() without holding mu.
	count atomic.Int32
}

// newFwdPool creates a new forward pool with the given handler and configuration.
func newFwdPool(handler func(fwdKey, []fwdItem), cfg fwdPoolConfig) *fwdPool {
	if cfg.chanSize <= 0 {
		cfg.chanSize = 64
	}
	if cfg.idleTimeout <= 0 {
		cfg.idleTimeout = 5 * time.Second
	}
	return &fwdPool{
		workers: make(map[fwdKey]*fwdWorker),
		handler: handler,
		cfg:     cfg,
		clock:   clock.RealClock{},
		stopCh:  make(chan struct{}),
	}
}

// SetClock sets the clock used for worker idle timers.
// Must be called before any Dispatch.
func (fp *fwdPool) SetClock(c clock.Clock) {
	fp.clock = c
}

// Dispatch sends a work item to the worker for the given key.
// Creates the worker lazily if it doesn't exist.
// Blocks if the channel is full (backpressure on the caller).
// Returns true if the item was enqueued, false if the pool is stopped.
// Callers must clean up associated state (e.g., cache Release) on false.
func (fp *fwdPool) Dispatch(key fwdKey, item fwdItem) bool {
	fp.mu.Lock()
	if fp.stopped {
		fp.mu.Unlock()
		return false
	}

	// Track this Dispatch so Stop can wait for all in-flight sends to
	// exit the select before closing worker channels.
	fp.dispatchWG.Add(1)
	defer fp.dispatchWG.Done()

	w, ok := fp.workers[key]
	if !ok {
		w = &fwdWorker{
			ch:   make(chan fwdItem, fp.cfg.chanSize),
			done: make(chan struct{}),
		}
		fp.workers[key] = w
		fp.count.Add(1)
		go fp.runWorker(key, w)
	}

	// Increment pending BEFORE releasing the lock. The idle handler checks
	// pending under the same lock, so it will see pending > 0 and not exit
	// even if the channel is momentarily empty.
	w.pending.Add(1)
	fp.mu.Unlock()

	// Blocking send: every cached UPDATE must be forwarded or released
	// (CacheConsumer protocol). Dropping is not acceptable. If the channel
	// is full, this blocks until the worker drains one item. The stopCh
	// escape prevents deadlock during shutdown.
	select {
	case w.ch <- item:
		w.pending.Add(-1)
	case <-fp.stopCh:
		w.pending.Add(-1)
		return false
	}

	return true
}

// Stop closes all workers and waits for them to drain.
// Closes stopCh first to unblock any Dispatch blocked on a full channel,
// then waits for all in-flight Dispatches to exit before closing channels.
func (fp *fwdPool) Stop() {
	fp.mu.Lock()
	fp.stopped = true

	// Unblock any Dispatch waiting on a full channel send.
	select {
	case <-fp.stopCh: // Already closed — Stop called twice (idempotent).
	default: // First Stop call — close to unblock pending sends.
		close(fp.stopCh)
	}
	fp.mu.Unlock()

	// Wait for all in-flight Dispatches to exit the select.
	// After this returns, no Dispatch goroutine is touching any w.ch,
	// so it's safe to close channels without racing on send-to-closed.
	fp.dispatchWG.Wait()

	fp.mu.Lock()
	all := make([]*fwdWorker, 0, len(fp.workers))
	for key, w := range fp.workers {
		all = append(all, w)
		delete(fp.workers, key)
	}
	fp.mu.Unlock()

	for _, w := range all {
		close(w.ch)
	}
	for _, w := range all {
		<-w.done
	}
}

// WorkerCount returns the number of active workers.
func (fp *fwdPool) WorkerCount() int {
	return int(fp.count.Load())
}

// safeBatchHandle calls the handler with panic recovery, then calls done() for
// every item in the batch. The done callbacks (Release) are guaranteed to run
// even if the handler panics. Without this guarantee, a panicking handler would
// leak cache entries.
func (fp *fwdPool) safeBatchHandle(key fwdKey, items []fwdItem) {
	defer func() {
		for i := range items {
			if items[i].done != nil {
				items[i].done()
			}
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			fwdLogger().Error("forward worker panic",
				"peer", key.peerAddr,
				"panic", r,
			)
		}
	}()
	fp.handler(key, items)
}

// drainBatch collects available items from the channel without blocking.
// Returns a batch starting with firstItem, followed by any immediately available items.
// buf is a reusable slice from the caller — reset to [:0] and returned for reuse.
func drainBatch(buf []fwdItem, firstItem fwdItem, ch <-chan fwdItem) []fwdItem {
	buf = append(buf[:0], firstItem)
	for {
		select {
		case extra, ok := <-ch:
			if !ok {
				return buf
			}
			buf = append(buf, extra)
		default: // non-blocking: no more items ready
			return buf
		}
	}
}

// runWorker is the long-lived goroutine for one destination peer.
// It reads items from the channel using drain-batch (one blocking receive +
// non-blocking drain of available items), calls the batch handler, and exits
// on idle timeout or channel close (Stop).
func (fp *fwdPool) runWorker(key fwdKey, w *fwdWorker) {
	defer func() {
		fp.count.Add(-1)
		close(w.done)
	}()

	idle := fp.clock.NewTimer(fp.cfg.idleTimeout)
	defer idle.Stop()

	for {
		select {
		case item, ok := <-w.ch:
			if !ok {
				// Channel closed (Stop) — exit.
				return
			}
			if !idle.Stop() {
				fwdDrainTimer(idle)
			}
			w.batchBuf = drainBatch(w.batchBuf, item, w.ch)
			fp.safeBatchHandle(key, w.batchBuf)
			idle.Reset(fp.cfg.idleTimeout)

		case <-idle.C():
			// Idle timeout — remove self from pool and exit.
			// Check channel AND pending counter under lock: Dispatch increments
			// pending under the same lock before sending, so if pending > 0, a
			// send is in flight and we must not exit.
			fp.mu.Lock()
			if len(w.ch) > 0 || w.pending.Load() > 0 {
				fp.mu.Unlock()
				idle.Reset(fp.cfg.idleTimeout)
				continue
			}
			// Only delete if this worker is still the registered one.
			if fp.workers[key] == w {
				delete(fp.workers, key)
			}
			fp.mu.Unlock()
			return
		}
	}
}

// fwdDrainTimer drains a stopped timer's channel to prevent stale fires.
func fwdDrainTimer(t clock.Timer) {
	select {
	case <-t.C():
	default: // Timer already drained or hadn't fired — safe to skip.
	}
}
