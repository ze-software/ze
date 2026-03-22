// Design: docs/architecture/core-design.md — per-peer forward worker pool
// Overview: reactor.go — BGP reactor event loop and peer management
// Related: reactor_api_forward.go — UPDATE forwarding dispatches to forward pool

package reactor

import (
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
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

// fwdWriteDeadlineDefault is the default TCP write deadline for forward pool
// batch writes (30 seconds). Overridable via env var ze.fwd.write.deadline.
const fwdWriteDeadlineDefault = 30 * time.Second

// fwdBatchHandler executes pre-computed send operations for a batch of fwdItems.
// Acquires the session write lock once, writes all messages to bufWriter, flushes once.
// On first write error, remaining items in the batch are skipped.
// Errors are logged but not propagated — TCP failures trigger FSM disconnect independently.
//
// Sets a write deadline on the TCP connection before writing to prevent a stuck
// peer from blocking the worker goroutine indefinitely. The deadline is cleared
// after the batch write+flush completes (or fails).
func fwdBatchHandler(_ fwdKey, items []fwdItem) {
	if len(items) == 0 {
		return
	}

	peer := items[0].peer
	if peer == nil {
		// Sentinel item (barrier) — no data to write. done() is called
		// by safeBatchHandle regardless.
		return
	}

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

	// Set write deadline AFTER acquiring writeMu so the full deadline budget
	// is available for TCP writes (not consumed by mutex contention).
	// Cleared in defer after write+flush completes.
	writeDeadline := env.GetDuration("ze.fwd.write.deadline", fwdWriteDeadlineDefault)
	if writeDeadline <= 0 {
		writeDeadline = fwdWriteDeadlineDefault
	}
	if err := conn.SetWriteDeadline(session.clock.Now().Add(writeDeadline)); err != nil {
		fwdLogger().Warn("forward set write deadline failed",
			"peer", peer.Settings().Address,
			"err", err,
		)
		return
	}
	defer func() {
		// Clear write deadline (zero value = no deadline).
		_ = conn.SetWriteDeadline(time.Time{})
	}()

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

	// Overflow buffer for non-blocking dispatch (TryDispatch fallback).
	// Protected by overflowMu. Items are moved to the channel by the worker
	// goroutine after processing each batch.
	overflowMu sync.Mutex
	overflow   []fwdItem

	// congested tracks whether this worker's channel is full.
	// Set on TryDispatch failure, cleared when channel drains below low-water.
	// Transitions fire pool-level onCongested/onResumed callbacks.
	congested bool
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

	// Congestion callbacks — fire on transitions only (not every item).
	// Called from the TryDispatch caller goroutine (onCongested) or the
	// worker goroutine (onResumed). Must not block.
	onCongested func(peerAddr string) // Called on false->true transition
	onResumed   func(peerAddr string) // Called on true->false transition
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
		w = fp.newWorker(key)
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

// TryDispatch attempts a non-blocking send to the worker for the given key.
// Creates the worker lazily if it doesn't exist.
// Returns true if the item was enqueued, false if the channel is full or pool is stopped.
// On false, the caller should use DispatchOverflow as a fallback.
//
// If the send fails because the channel is full:
//   - Sets the worker's congested flag (if not already set)
//   - Fires onCongested callback on false->true transition
func (fp *fwdPool) TryDispatch(key fwdKey, item fwdItem) bool {
	fp.mu.Lock()
	if fp.stopped {
		fp.mu.Unlock()
		return false
	}

	// Track this TryDispatch so Stop can wait for all in-flight sends
	// to exit before closing worker channels (prevents send-on-closed panic).
	fp.dispatchWG.Add(1)
	defer fp.dispatchWG.Done()

	w, ok := fp.workers[key]
	if !ok {
		w = fp.newWorker(key)
	}
	fp.mu.Unlock()

	// Non-blocking send.
	select {
	case w.ch <- item:
		return true
	default: // channel full — non-blocking fallback
		// Mark congested and fire callback on transition.
		w.overflowMu.Lock()
		wasCongested := w.congested
		w.congested = true
		w.overflowMu.Unlock()

		if !wasCongested && fp.onCongested != nil {
			fp.onCongested(key.peerAddr)
		}
		return false
	}
}

// DispatchOverflow adds an item to the per-worker overflow buffer.
// Creates the worker lazily if it doesn't exist. The worker goroutine
// drains overflow items after each batch from the channel.
//
// The overflow buffer is unbounded. Routes are critical data and must
// never be dropped. Memory growth from a slow peer is preferable to
// silent routing inconsistency.
//
// Returns true if the item was buffered, false if the pool is stopped
// (in which case done() is called immediately to prevent cache leaks).
func (fp *fwdPool) DispatchOverflow(key fwdKey, item fwdItem) bool {
	fp.mu.Lock()
	if fp.stopped {
		fp.mu.Unlock()
		// Pool stopped — call done immediately to prevent cache leaks.
		if item.done != nil {
			item.done()
		}
		return false
	}

	// Track this DispatchOverflow so Stop can wait for all in-flight
	// overflow appends before draining and closing channels.
	fp.dispatchWG.Add(1)
	defer fp.dispatchWG.Done()

	w, ok := fp.workers[key]
	if !ok {
		w = fp.newWorker(key)
	}
	fp.mu.Unlock()

	w.overflowMu.Lock()
	w.overflow = append(w.overflow, item)

	// Log when overflow grows large (potential slow peer), but never drop.
	// Routes are critical data — dropping causes silent routing inconsistency
	// with no automatic recovery. Memory pressure from a slow peer is
	// preferable to missing routes.
	if n := len(w.overflow); n == 1000 || n == 10000 || n == 100000 {
		fwdLogger().Warn("overflow buffer growing",
			"peer", key.peerAddr,
			"queued", n,
		)
	}
	w.overflowMu.Unlock()
	return true
}

// newWorker creates a new fwdWorker, registers it in the pool, and starts its goroutine.
// Caller must hold fp.mu.
func (fp *fwdPool) newWorker(key fwdKey) *fwdWorker {
	w := &fwdWorker{
		ch:   make(chan fwdItem, fp.cfg.chanSize),
		done: make(chan struct{}),
	}
	fp.workers[key] = w
	fp.count.Add(1)
	go fp.runWorker(key, w)
	return w
}

// Stop closes all workers and waits for them to drain.
// Closes stopCh first to unblock any Dispatch blocked on a full channel,
// then waits for all in-flight Dispatches to exit before closing channels.
// Fires done callbacks for all remaining overflow items to prevent cache leaks.
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

	// Fire done callbacks for all overflow items before closing channels.
	// Workers won't process these since we're about to close their channels.
	for _, w := range all {
		w.overflowMu.Lock()
		for _, item := range w.overflow {
			if item.done != nil {
				item.done()
			}
		}
		w.overflow = nil
		w.overflowMu.Unlock()
	}

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
//
// After processing each batch from the channel, the worker drains overflow
// items (added by DispatchOverflow) into the channel or processes them directly.
// Checks congestion state: clears congested flag when channel occupancy drops
// below low-water mark (25% of channel capacity).
func (fp *fwdPool) runWorker(key fwdKey, w *fwdWorker) {
	defer func() {
		fp.count.Add(-1)
		close(w.done)
	}()

	idle := fp.clock.NewTimer(fp.cfg.idleTimeout)
	defer idle.Stop()

	lowWater := fp.cfg.chanSize / 4 // 25% of channel capacity

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

			// Drain overflow items into the channel after processing.
			fp.drainOverflow(key, w)

			// Check congestion: clear if channel dropped below low-water mark.
			// Single lock acquisition to atomically decide whether to fire onResumed.
			if len(w.ch) <= lowWater {
				var fireResumed bool
				w.overflowMu.Lock()
				if w.congested && len(w.overflow) == 0 {
					w.congested = false
					fireResumed = true
				}
				w.overflowMu.Unlock()

				if fireResumed && fp.onResumed != nil {
					fp.onResumed(key.peerAddr)
				}
			}

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
			// Also check overflow — don't idle-exit with pending overflow items.
			w.overflowMu.Lock()
			hasOverflow := len(w.overflow) > 0
			w.overflowMu.Unlock()
			if hasOverflow {
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

// drainOverflow moves items from the overflow buffer into the channel or
// processes them directly. Called by the worker goroutine after each batch.
func (fp *fwdPool) drainOverflow(key fwdKey, w *fwdWorker) {
	w.overflowMu.Lock()
	if len(w.overflow) == 0 {
		w.overflowMu.Unlock()
		return
	}

	// Take all overflow items under the lock, then release.
	items := w.overflow
	w.overflow = nil
	w.overflowMu.Unlock()

	// Try to enqueue overflow items into the channel.
	// If channel is full, process remaining items directly as a batch.
	var remaining []fwdItem
	for i, item := range items {
		select {
		case w.ch <- item:
			// Enqueued successfully — worker loop will process it.
		default: // channel full — process rest directly
			remaining = items[i:]
			goto processDirect
		}
	}
	return

processDirect:
	fp.safeBatchHandle(key, remaining)
}

// fwdDrainTimer drains a stopped timer's channel to prevent stale fires.
func fwdDrainTimer(t clock.Timer) {
	select {
	case <-t.C():
	default: // Timer already drained or hadn't fired — safe to skip.
	}
}
