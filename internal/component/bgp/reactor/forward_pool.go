// Design: docs/architecture/core-design.md — per-peer forward worker pool
// Overview: reactor.go — BGP reactor event loop and peer management
// Detail: forward_pool_weight.go — burst fraction, buffer demand calculation
// Detail: forward_pool_weight_tracker.go — per-peer weight tracking and pool budget
// Related: reactor_api_forward.go — UPDATE forwarding dispatches to forward pool
// Related: reactor_metrics.go — metrics loop polls overflow depth, pool ratio, source stats
// Related: bufmux.go — block-backed buffer multiplexer (shared buffer pools)

package reactor

import (
	"net/netip"
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
// Uses netip.AddrPort to distinguish peers on different ports sharing the same IP.
type fwdKey struct {
	peerAddr netip.AddrPort
}

// fwdItem is a unit of work dispatched to a forward worker.
// Pre-computed send operations for one destination peer from one ForwardUpdate call.
// The worker executes rawBodies (SendRawUpdateBody) then updates (SendUpdate).
type fwdItem struct {
	rawBodies [][]byte          // Zero-copy or split pieces: SendRawUpdateBody per entry
	updates   []*message.Update // Re-encode path: SendUpdate per entry
	peer      *Peer             // Target peer for all operations
	done      func()            // Called after all ops complete (Release cache entry)
	pooled    bool              // Holds overflow pool token; MUST release after processing
	meta      map[string]any    // Route metadata from ReceivedUpdate; set on sent events
}

// fwdWriteDeadlineDefault is the default TCP write deadline for forward pool
// batch writes (30 seconds). Overridable via env var ze.fwd.write.deadline.
const fwdWriteDeadlineDefault = 30 * time.Second

// fwdWriteDeadlineNs holds the resolved write deadline in nanoseconds,
// cached at package init to avoid per-batch env.GetDuration overhead on
// the hot path. Overridden by initFwdWriteDeadline() at reactor startup.
// Stored via atomic.Int64 for safe concurrent access.
var fwdWriteDeadlineNs atomic.Int64 //nolint:gochecknoglobals // hot-path cache

func init() {
	fwdWriteDeadlineNs.Store(int64(fwdWriteDeadlineDefault))
}

// initFwdWriteDeadline reads ze.fwd.write.deadline from env and caches it.
// Called once from reactor startup, before any forward pool dispatch.
func initFwdWriteDeadline() {
	d := env.GetDuration("ze.fwd.write.deadline", fwdWriteDeadlineDefault)
	if d <= 0 {
		d = fwdWriteDeadlineDefault
	}
	fwdWriteDeadlineNs.Store(int64(d))
}

// fwdWriteDeadline returns the cached write deadline for forward pool batches.
func fwdWriteDeadline() time.Duration {
	return time.Duration(fwdWriteDeadlineNs.Load())
}

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
	// Write deadline is cached in fwdPoolConfig at startup to avoid per-batch
	// env.GetDuration overhead (hot path: called thousands of times/sec).
	if err := conn.SetWriteDeadline(session.clock.Now().Add(fwdWriteDeadline())); err != nil {
		fwdLogger().Warn("forward set write deadline failed",
			"peer", peer.Settings().Address,
			"err", err,
		)
		return
	}
	defer func() {
		session.sentMeta = nil // Clear route metadata on all exit paths.
		// Clear write deadline (zero value = no deadline).
		_ = conn.SetWriteDeadline(time.Time{})
	}()

	for _, item := range items {
		session.sentMeta = item.meta // Route metadata for sent event callbacks.
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
		return
	}

	// Successful batch write -- reset RFC 9687 Send Hold Timer.
	session.resetSendHoldTimer()
}

// fwdOverflowPoolMaxSize caps the overflow pool to prevent unbounded
// allocation from misconfigured ze.fwd.pool.size values.
const fwdOverflowPoolMaxSize = 10_000_000

// fwdOverflowPool bounds the number of overflow items across all workers.
// Sized at startup via ze.fwd.pool.size. When exhausted, DispatchOverflow
// falls back to unbounded append and logs a warning. Layer 3/4 handle
// escalation (read throttling, teardown).
type fwdOverflowPool struct {
	tokens    chan struct{}
	size      int
	exhausted atomic.Int64 // count of failed acquire() calls (for rate-limited logging)
}

func newFwdOverflowPool(size int) *fwdOverflowPool {
	p := &fwdOverflowPool{
		tokens: make(chan struct{}, size),
		size:   size,
	}
	for range size {
		p.tokens <- struct{}{}
	}
	return p
}

// acquire attempts to take a pool token (non-blocking).
// Returns true if a token was acquired, false if the pool is exhausted.
// Caller MUST call release() exactly once after processing the item.
func (p *fwdOverflowPool) acquire() bool {
	select {
	case <-p.tokens:
		return true
	default: // pool exhausted — non-blocking, caller handles fallback
		return false
	}
}

// release returns a pool token. MUST be called exactly once per successful acquire().
// Logs an error on double-release (more releases than acquires), which indicates
// a bug in token lifecycle management.
func (p *fwdOverflowPool) release() {
	select {
	case p.tokens <- struct{}{}:
	default: // bug: more releases than acquires — pool was never this large
		fwdLogger().Error("overflow pool double-release")
	}
}

// available returns the number of free tokens. This is a point-in-time snapshot
// and may be stale by the time the caller acts on it. Use for diagnostics only.
func (p *fwdOverflowPool) available() int { return len(p.tokens) }

// fwdPoolConfig holds configuration for a fwdPool.
type fwdPoolConfig struct {
	chanSize         int
	idleTimeout      time.Duration
	overflowPoolSize int // 0 = unbounded (legacy); >0 = bounded overflow pool
	batchLimit       int // 0 = unlimited; >0 = max items per drain-batch (AC-24)
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
	onCongested func(peerAddr netip.AddrPort) // Called on false->true transition
	onResumed   func(peerAddr netip.AddrPort) // Called on true->false transition

	// overflowPool bounds overflow items across all workers (nil = unbounded).
	overflowPool *fwdOverflowPool

	// congestion is the two-threshold enforcement controller (Phase 5).
	// Nil when congestion control is not configured.
	congestion *congestionController

	// Per-source-peer dispatch counters for AC-16 overflow ratio.
	// Key: source peer address. Updated atomically in ForwardUpdate path.
	srcStatsMu sync.Mutex
	srcStats   map[netip.Addr]*fwdSourceStats
}

// fwdSourceStats tracks forwarded vs overflowed counts for one source peer.
// Used to compute the overflow ratio (AC-16).
type fwdSourceStats struct {
	forwarded  atomic.Int64 // successfully dispatched via TryDispatch
	overflowed atomic.Int64 // fell through to DispatchOverflow
}

// newFwdPool creates a new forward pool with the given handler and configuration.
// Caller MUST call Stop when done to drain workers and release resources.
func newFwdPool(handler func(fwdKey, []fwdItem), cfg fwdPoolConfig) *fwdPool {
	if cfg.chanSize <= 0 {
		cfg.chanSize = 64
	}
	if cfg.idleTimeout <= 0 {
		cfg.idleTimeout = 5 * time.Second
	}
	fp := &fwdPool{
		workers:  make(map[fwdKey]*fwdWorker),
		handler:  handler,
		cfg:      cfg,
		clock:    clock.RealClock{},
		stopCh:   make(chan struct{}),
		srcStats: make(map[netip.Addr]*fwdSourceStats),
	}
	if cfg.overflowPoolSize > 0 {
		poolSize := cfg.overflowPoolSize
		if poolSize > fwdOverflowPoolMaxSize {
			fwdLogger().Warn("overflow pool size capped at maximum",
				"configured", poolSize,
				"max", fwdOverflowPoolMaxSize,
			)
			poolSize = fwdOverflowPoolMaxSize
		}
		fp.overflowPool = newFwdOverflowPool(poolSize)
	}
	return fp
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

	// Check buffer denial (AC-2): if the congestion controller says this
	// destination peer is the worst offender, skip pool acquisition. The item
	// still goes to unbounded overflow (routes never dropped), but the denial
	// signal feeds into teardown decisions.
	denied := fp.congestion.ShouldDeny(key.peerAddr.String())

	// Acquire overflow pool token if pool exists and not denied.
	// Skip for sentinel items (peer == nil) — they carry no route data
	// and should not consume pool capacity meant for real updates.
	if fp.overflowPool != nil && item.peer != nil && !denied {
		if fp.overflowPool.acquire() {
			item.pooled = true
		} else {
			// Pool exhausted — fall back to unbounded append.
			// Layer 3 (read throttling) and Layer 4 (teardown) handle escalation.
			// Rate-limited: log at thresholds to avoid flooding under sustained pressure.
			if n := fp.overflowPool.exhausted.Add(1); n == 1 || n == 100 || n == 10000 || n == 100000 {
				fwdLogger().Warn("overflow pool exhausted, unbounded fallback",
					"peer", key.peerAddr,
					"pool_size", fp.overflowPool.size,
					"exhausted_total", n,
				)
			}
		}
	}

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

	// Fire done callbacks and release pool tokens for all overflow items
	// before closing channels. Workers won't process these since we're
	// about to close their channels.
	for _, w := range all {
		w.overflowMu.Lock()
		for _, item := range w.overflow {
			if item.done != nil {
				item.done()
			}
			if item.pooled && fp.overflowPool != nil {
				fp.overflowPool.release()
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

// OverflowDepths returns a snapshot of per-destination-peer overflow depth.
// Each entry maps peer address string to the number of items currently in its overflow buffer.
// Called by the metrics update loop; must not block.
func (fp *fwdPool) OverflowDepths() map[string]int {
	fp.mu.Lock()
	result := make(map[string]int, len(fp.workers))
	for key, w := range fp.workers {
		w.overflowMu.Lock()
		result[key.peerAddr.String()] = len(w.overflow)
		w.overflowMu.Unlock()
	}
	fp.mu.Unlock()
	return result
}

// PoolUsedRatio returns the fraction of overflow pool tokens in use (0.0 to 1.0).
// Returns 0.0 if no pool is configured. Called by the metrics update loop.
func (fp *fwdPool) PoolUsedRatio() float64 {
	if fp.overflowPool == nil {
		return 0.0
	}
	avail := fp.overflowPool.available()
	total := fp.overflowPool.size
	if total == 0 {
		return 0.0
	}
	return 1.0 - float64(avail)/float64(total)
}

// RecordForwarded increments the forwarded counter for a source peer.
// Called from ForwardUpdate when TryDispatch succeeds.
func (fp *fwdPool) RecordForwarded(sourcePeer netip.Addr) {
	fp.getSourceStats(sourcePeer).forwarded.Add(1)
}

// RecordOverflowed increments the overflowed counter for a source peer.
// Called from ForwardUpdate when DispatchOverflow is used.
func (fp *fwdPool) RecordOverflowed(sourcePeer netip.Addr) {
	fp.getSourceStats(sourcePeer).overflowed.Add(1)
}

// getSourceStats returns the stats for a source peer, creating if needed.
func (fp *fwdPool) getSourceStats(sourcePeer netip.Addr) *fwdSourceStats {
	fp.srcStatsMu.Lock()
	s, ok := fp.srcStats[sourcePeer]
	if !ok {
		s = &fwdSourceStats{}
		fp.srcStats[sourcePeer] = s
	}
	fp.srcStatsMu.Unlock()
	return s
}

// SourceOverflowRatios returns per-source-peer overflow ratio: overflowed/(forwarded+overflowed).
// Returns 0.0 for peers with no overflow. Called by the metrics update loop.
// Keys are string-form addresses for display/metrics consumption.
func (fp *fwdPool) SourceOverflowRatios() map[string]float64 {
	fp.srcStatsMu.Lock()
	result := make(map[string]float64, len(fp.srcStats))
	for peer, s := range fp.srcStats {
		fwd := s.forwarded.Load()
		ovf := s.overflowed.Load()
		total := fwd + ovf
		if total == 0 {
			result[peer.String()] = 0.0
		} else {
			result[peer.String()] = float64(ovf) / float64(total)
		}
	}
	fp.srcStatsMu.Unlock()
	return result
}

// RemoveSourceStats deletes the source stats entry for a peer.
// Called on peer disconnect to prevent unbounded srcStats growth.
func (fp *fwdPool) RemoveSourceStats(sourcePeer netip.Addr) {
	fp.srcStatsMu.Lock()
	delete(fp.srcStats, sourcePeer)
	fp.srcStatsMu.Unlock()
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
			if items[i].pooled && fp.overflowPool != nil { // nil check is defensive — pooled is only set when pool exists
				fp.overflowPool.release()
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
// limit caps the total batch size (0 = unlimited). Remaining items stay in the
// channel for the next drain cycle (AC-24: TX budget).
func drainBatch(buf []fwdItem, firstItem fwdItem, ch <-chan fwdItem, limit int) []fwdItem {
	buf = append(buf[:0], firstItem)
	for {
		if limit > 0 && len(buf) >= limit {
			return buf
		}
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
			w.batchBuf = drainBatch(w.batchBuf, item, w.ch, fp.cfg.batchLimit)
			fp.safeBatchHandle(key, w.batchBuf)

			// Check congestion teardown after each batch (AC-4).
			// This is cheap (atomic reads + map lookup) and only fires when
			// the pool is critically full and this peer is the worst offender.
			fp.congestion.CheckTeardown(key.peerAddr)

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
	// If channel is full or pool is stopping, process remaining directly.
	// The stopCh check prevents send-on-closed-channel panic: Stop closes
	// stopCh (step 1) well before closing w.ch (step 5), so if stopCh is
	// closed, w.ch may be about to close — fall through to processDirect.
	var remaining []fwdItem
	for i, item := range items {
		select {
		case <-fp.stopCh: // pool stopping — don't risk send on closing channel
			remaining = items[i:]
			goto processDirect
		default: // not stopping — safe to attempt enqueue
		}
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
