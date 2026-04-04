// Design: docs/architecture/core-design.md — per-peer forward worker pool
// Design: .claude/rules/design-principles.md — zero-copy, copy-on-modify (Incoming/Outgoing Peer Pools, Global Shared Pool)
// Overview: reactor.go — BGP reactor event loop and peer management
// Detail: forward_pool_weight.go — burst fraction, buffer demand calculation
// Detail: forward_pool_weight_tracker.go — per-peer weight tracking and pool budget
// Detail: forward_pool_congestion.go — two-threshold congestion enforcement
// Related: reactor_api_forward.go — UPDATE forwarding dispatches to forward pool
// Related: reactor_metrics.go — metrics loop polls overflow depth, pool ratio, source stats
// Related: bufmux.go — block-backed buffer multiplexer (shared buffer pools)
//
// Algorithm overview:
//
// Each destination peer gets a worker goroutine + bounded channel. Incoming
// UPDATEs are dispatched to the destination's channel (TryDispatch). If the
// channel is full, items spill into a shared overflow pool. Workers drain
// their channel in batches, writing wire bytes directly to the peer's TCP
// bufio.Writer, then flushing once per batch.
//
// Weight tracking sizes per-peer channel capacity proportional to the peer's
// NLRI volume. Congestion control uses two thresholds (warn/critical) on the
// shared buffer pool usage ratio to pause slow peers before memory exhaustion.

package reactor

import (
	"bytes"
	"hash/fnv"
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
	rawBodies    [][]byte          // Zero-copy or split pieces: SendRawUpdateBody per entry
	updates      []*message.Update // Re-encode path: SendUpdate per entry
	peer         *Peer             // Target peer for all operations
	done         func()            // Called after all ops complete (Release cache entry)
	peerBufIdx   int               // 1-based index into per-peer pool; 0 = not from per-peer pool
	peerPoolRef  *peerPool         // Pool to return buffer to (avoids map lookup + lock)
	overflowBuf  BufHandle         // Holds overflow MixedBufMux handle; nil Buf = not from overflow
	meta         map[string]any    // Route metadata from ReceivedUpdate; set on sent events
	supersedeKey uint64            // FNV-1a hash of raw body for route superseding (AC-23); 0 = no superseding
	withdrawal   bool              // True if this item contains only withdrawals (AC-25)
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

	for i := range items {
		session.sentMeta = items[i].meta // Route metadata for sent event callbacks.
		for _, body := range items[i].rawBodies {
			if err := session.writeRawUpdateBody(body); err != nil {
				fwdLogger().Warn("forward batch write failed",
					"peer", peer.Settings().Address,
					"err", err,
				)
				return
			}
		}
		for _, update := range items[i].updates {
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

// peerPoolSize is the number of buffers in each per-peer pool.
// Matches the proven per-worker channel capacity for micro-burst absorption.
const peerPoolSize = 64

// peerPool is a pre-allocated buffer pool used as an Outgoing Peer Pool.
// Created at session establishment with the negotiated message size
// (4K standard, 64K for RFC 8654 Extended Message).
//
// Buffers are acquired by buildModifiedPayload when egress filters need
// to modify the payload for this destination peer (copy-on-modify).
// When exhausted, modification falls back to sync.Pool.
//
// Pre-allocates one contiguous backing array at init, sliced into
// peerPoolSize buffers. Mutex-protected index stack for O(1) Get/Return.
// GC scans one pointer (backing slice) instead of 64 (one per buffer).
// Same type for all peers -- buffer size is set at initialization.
//
// Indices are stored as idx+1 (1-based) so that the zero value of
// fwdItem.peerBufIdx means "no buffer" without requiring explicit -1
// initialization.
type peerPool struct {
	mu      sync.Mutex
	backing []byte              // single contiguous allocation
	free    [peerPoolSize]uint8 // stack of free buffer indices (1-based); free[:top] are available
	top     int                 // number of free buffers (0 = exhausted)
	lent    [peerPoolSize]bool  // true = buffer is out on loan (double-return guard)
	bufSize int                 // negotiated buffer size (message.MaxMsgLen or message.ExtMsgLen)
}

// newPeerPool creates a per-peer pool with the given buffer size.
// Pre-allocates peerPoolSize buffers of bufSize bytes in one allocation.
// RFC 8654: Extended Message peers use 64K buffers, standard peers use 4K.
func newPeerPool(bufSize int) *peerPool {
	pp := &peerPool{
		backing: make([]byte, peerPoolSize*bufSize),
		top:     peerPoolSize,
		bufSize: bufSize,
	}
	for i := range peerPoolSize {
		pp.free[i] = uint8(i + 1) // 1-based
	}
	return pp
}

// Get returns a buffer and its 1-based index from the pool (non-blocking).
// Returns (nil, 0) if the pool is exhausted.
// Caller MUST call Return(idx) exactly once after processing.
func (pp *peerPool) Get() ([]byte, int) {
	pp.mu.Lock()
	if pp.top == 0 {
		pp.mu.Unlock()
		return nil, 0
	}
	pp.top--
	idx := int(pp.free[pp.top]) // 1-based
	pp.lent[idx-1] = true       // mark as out on loan
	off := (idx - 1) * pp.bufSize
	pp.mu.Unlock()
	return pp.backing[off : off+pp.bufSize], idx
}

// Return puts a buffer back into the pool by its 1-based index.
// Caller MUST NOT use the buffer after returning it.
func (pp *peerPool) Return(idx int) {
	pp.mu.Lock()
	if idx < 1 || idx > peerPoolSize {
		pp.mu.Unlock()
		fwdLogger().Error("peer pool return: index out of range", "idx", idx)
		return
	}
	if !pp.lent[idx-1] {
		pp.mu.Unlock()
		fwdLogger().Error("peer pool double return", "idx", idx)
		return
	}
	pp.lent[idx-1] = false
	pp.free[pp.top] = uint8(idx)
	pp.top++
	pp.mu.Unlock()
}

// available returns the number of free buffers.
func (pp *peerPool) available() int {
	pp.mu.Lock()
	n := pp.top
	pp.mu.Unlock()
	return n
}

// size returns the pool capacity.
func (pp *peerPool) size() int {
	return peerPoolSize
}

// fwdPoolConfig holds configuration for a fwdPool.
type fwdPoolConfig struct {
	chanSize    int
	idleTimeout time.Duration
	batchLimit  int // 0 = unlimited; >0 = max items per drain-batch (AC-24)
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

	// overflowMux is the shared mixed-size overflow pool (fwd-auto-sizing).
	// 64K blocks subdivisible to 16 x 4K slices, byte-budgeted.
	overflowMux *MixedBufMux

	// outgoingPools tracks Outgoing Peer Pools for egress modification.
	// Used by buildModifiedPayload when egress filters need to modify the
	// payload for a destination peer (copy-on-modify). 64 pre-allocated
	// buffers at the peer's negotiated message size. Created at peer
	// registration, destroyed on session teardown. Protected by mu.
	outgoingPools map[fwdKey]*peerPool

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
		workers:       make(map[fwdKey]*fwdWorker),
		handler:       handler,
		cfg:           cfg,
		clock:         clock.RealClock{},
		stopCh:        make(chan struct{}),
		srcStats:      make(map[netip.Addr]*fwdSourceStats),
		outgoingPools: make(map[fwdKey]*peerPool),
	}
	return fp
}

// SetClock sets the clock used for worker idle timers.
// Must be called before any Dispatch.
func (fp *fwdPool) SetClock(c clock.Clock) {
	fp.clock = c
}

// SetOverflowMux sets the shared overflow MixedBufMux for the pool.
// When set, overflow dispatch acquires buffer handles from this mux.
// Must be called before concurrent use.
func (fp *fwdPool) SetOverflowMux(m *MixedBufMux) {
	fp.overflowMux = m
}

// RegisterOutgoingPool creates an Outgoing Peer Pool for the given destination peer.
// bufSize is the negotiated message size (4K standard, 64K ExtMsg).
// Called at session establishment. Safe for concurrent use.
func (fp *fwdPool) RegisterOutgoingPool(key fwdKey, bufSize int) {
	fp.mu.Lock()
	fp.outgoingPools[key] = newPeerPool(bufSize)
	fp.mu.Unlock()
}

// UnregisterOutgoingPool removes the Outgoing Peer Pool for the given destination peer.
// Called at session teardown. Safe for concurrent use.
func (fp *fwdPool) UnregisterOutgoingPool(key fwdKey) {
	fp.mu.Lock()
	delete(fp.outgoingPools, key)
	fp.mu.Unlock()
}

// OutgoingPool returns the Outgoing Peer Pool for the given key, or nil.
// Used by ForwardUpdate to pass to buildModifiedPayload for copy-on-modify.
func (fp *fwdPool) OutgoingPool(key fwdKey) *peerPool {
	fp.mu.Lock()
	pp := fp.outgoingPools[key]
	fp.mu.Unlock()
	return pp
}

// releaseItem returns all pool resources held by an fwdItem.
// Handles Outgoing Peer Pool buffers and Global Shared Pool handles.
// Called from safeBatchHandle and Stop cleanup.
func (fp *fwdPool) releaseItem(item *fwdItem) {
	if item.peerBufIdx > 0 && item.peerPoolRef != nil {
		item.peerPoolRef.Return(item.peerBufIdx)
		item.peerBufIdx = 0
		item.peerPoolRef = nil
	}
	if item.overflowBuf.Buf != nil && fp.overflowMux != nil {
		fp.overflowMux.Return(item.overflowBuf)
		item.overflowBuf = BufHandle{}
	}
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

	// Non-blocking send. Per-peer pool buffers are not acquired here --
	// they are taken by buildModifiedPayload only when egress filters
	// need to modify the payload (copy-on-modify). The channel capacity
	// provides the concurrency gate.
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
	denied := fp.congestion.ShouldDeny(key.peerAddr.Addr().String())

	// Acquire overflow MixedBufMux handle if available and not denied.
	// Skip for sentinel items (peer == nil) — they carry no route data
	// and should not consume pool capacity meant for real updates.
	if fp.overflowMux != nil && item.peer != nil && !denied {
		// Determine buffer size from the destination peer's Outgoing Peer Pool.
		// Default to 4K (standard) if no pool is registered.
		bufSize := message.MaxMsgLen
		fp.mu.Lock()
		if pp := fp.outgoingPools[key]; pp != nil {
			bufSize = pp.bufSize
		}
		fp.mu.Unlock()

		var h BufHandle
		if bufSize >= message.ExtMsgLen {
			h = fp.overflowMux.Get64K()
		} else {
			h = fp.overflowMux.Get4K()
		}
		if h.Buf != nil {
			item.overflowBuf = h
		}
		// If h.Buf == nil, pool exhausted. Proceed without — routes never dropped.
		// Layer 3 (read throttling) and Layer 4 (teardown) handle escalation.
	}

	w.overflowMu.Lock()

	// Route superseding (AC-23): if a pending item has the same content hash,
	// replace it instead of appending. This bounds queue growth to unique
	// UPDATE content rather than total update count. O(n) scan is acceptable
	// because overflow is the slow path and items are bounded by the pool.
	if item.supersedeKey != 0 {
		for i := range w.overflow {
			if w.overflow[i].supersedeKey != item.supersedeKey {
				continue
			}
			// Verify content match (guard against FNV hash collision).
			if !fwdBodiesEqual(w.overflow[i].rawBodies, item.rawBodies) {
				continue
			}
			// Supersede: release old item's resources, replace with new.
			old := w.overflow[i]
			if old.done != nil {
				old.done()
			}
			if old.peerBufIdx > 0 && old.peerPoolRef != nil {
				old.peerPoolRef.Return(old.peerBufIdx)
			}
			if old.overflowBuf.Buf != nil && fp.overflowMux != nil {
				fp.overflowMux.Return(old.overflowBuf)
			}
			w.overflow[i] = item
			w.overflowMu.Unlock()
			return true
		}
	}

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
	type keyWorker struct {
		key fwdKey
		w   *fwdWorker
	}
	all := make([]keyWorker, 0, len(fp.workers))
	for key, w := range fp.workers {
		all = append(all, keyWorker{key: key, w: w})
		delete(fp.workers, key)
	}
	fp.mu.Unlock()

	// Fire done callbacks and release pool tokens for all overflow items
	// before closing channels. Workers won't process these since we're
	// about to close their channels.
	for _, kw := range all {
		kw.w.overflowMu.Lock()
		for i := range kw.w.overflow {
			if kw.w.overflow[i].done != nil {
				kw.w.overflow[i].done()
			}
			fp.releaseItem(&kw.w.overflow[i])
		}
		kw.w.overflow = nil
		kw.w.overflowMu.Unlock()
	}

	for _, kw := range all {
		close(kw.w.ch)
	}
	for _, kw := range all {
		<-kw.w.done
	}
}

// WorkerCount returns the number of active workers.
func (fp *fwdPool) WorkerCount() int {
	return int(fp.count.Load())
}

// OverflowDepths returns a snapshot of per-destination-peer overflow depth.
// Each entry maps peer address string (IP-only, no port) to the number of
// items currently in its overflow buffer. IP-only format matches the key
// format used by weightTracker (peerAddrLabel) and Prometheus labels.
// Called by the metrics update loop; must not block.
func (fp *fwdPool) OverflowDepths() map[string]int {
	fp.mu.Lock()
	result := make(map[string]int, len(fp.workers))
	for key, w := range fp.workers {
		w.overflowMu.Lock()
		result[key.peerAddr.Addr().String()] += len(w.overflow)
		w.overflowMu.Unlock()
	}
	fp.mu.Unlock()
	return result
}

// PoolUsedRatio returns the fraction of overflow pool capacity in use (0.0 to 1.0).
// Reads from MixedBufMux stats (usedBytes/budgetBytes).
// Returns 0.0 if no overflow mux is configured. Called by the metrics update loop.
func (fp *fwdPool) PoolUsedRatio() float64 {
	if fp.overflowMux != nil {
		return fp.overflowMux.UsedRatio()
	}
	return 0.0
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
//
// Reorders the batch so withdrawals are sent before announcements (AC-25).
// This is safe because route superseding (AC-23) ensures no prefix appears
// in both a withdrawal and announcement within the same overflow batch.
func (fp *fwdPool) safeBatchHandle(key fwdKey, items []fwdItem) {
	items = fwdReorderWithdrawalsFirst(items)
	defer func() {
		for i := range items {
			if items[i].done != nil {
				items[i].done()
			}
			fp.releaseItem(&items[i])
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

// fwdSupersedeKey computes an FNV-1a hash of the raw body bytes for route
// superseding (AC-23). Two fwdItems with the same key carry identical wire
// content and can safely supersede each other in the overflow queue.
// Returns 0 if no raw bodies (re-encode path items are not superseded).
func fwdSupersedeKey(rawBodies [][]byte) uint64 {
	if len(rawBodies) == 0 {
		return 0
	}
	h := fnv.New64a()
	for _, body := range rawBodies {
		h.Write(body) //nolint:errcheck // fnv.Write never returns error
	}
	return h.Sum64()
}

// fwdBodiesEqual compares two rawBodies slices for byte-level equality.
// Used as a guard against FNV hash collisions during superseding (finding 6).
// Only called on the rare hash-match path -- negligible performance impact.
func fwdBodiesEqual(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

// fwdIsWithdrawal returns true if the fwdItem contains only withdrawals
// (no announcements). Handles both IPv4 withdrawals (legacy Withdrawn Routes
// field) and non-IPv4 withdrawals (MP_UNREACH_NLRI attribute code 15).
// Used by AC-25 withdrawal priority.
//
// Classification rules:
//   - Legacy NLRI present (IPv4 announce) -> announcement
//   - MP_REACH_NLRI (attr code 14) present -> announcement
//   - Legacy Withdrawn Routes present -> withdrawal (if no announcements)
//   - MP_UNREACH_NLRI (attr code 15) present -> withdrawal (if no announcements)
//   - Truncated or malformed body -> not classified (skipped)
func fwdIsWithdrawal(item *fwdItem) bool {
	hasWithdrawal := false

	// Check parsed updates (re-encode path).
	for _, u := range item.updates {
		if len(u.NLRI) > 0 {
			return false // Has IPv4 announcements.
		}
		if len(u.WithdrawnRoutes) > 0 {
			hasWithdrawal = true
		}
		// Check PathAttributes for MP_REACH_NLRI (14) vs MP_UNREACH_NLRI (15).
		if fwdAttrsHaveReach(u.PathAttributes) {
			return false // Has MP_REACH = announcement.
		}
		if fwdAttrsHaveUnreach(u.PathAttributes) {
			hasWithdrawal = true
		}
	}

	// Check raw bodies (zero-copy path).
	// UPDATE body format: [2B withdrawn_len][withdrawn][2B attr_len][attrs][nlri]
	for _, body := range item.rawBodies {
		if len(body) < 4 {
			continue // Truncated, skip (don't classify malformed data).
		}
		wdLen := int(body[0])<<8 | int(body[1])
		if wdLen > 0 {
			hasWithdrawal = true
		}

		off := 2 + wdLen
		if off+2 > len(body) {
			continue // Truncated after withdrawn routes.
		}
		attrLen := int(body[off])<<8 | int(body[off+1])
		attrStart := off + 2
		attrEnd := attrStart + attrLen

		// Check for legacy NLRI after attributes.
		if attrEnd < len(body) {
			return false // Has NLRI section = announcement.
		}

		// Check attributes for MP_REACH (14) / MP_UNREACH (15).
		if attrEnd <= len(body) {
			attrs := body[attrStart:attrEnd]
			if fwdAttrsHaveReach(attrs) {
				return false // MP_REACH = announcement.
			}
			if fwdAttrsHaveUnreach(attrs) {
				hasWithdrawal = true
			}
		}
	}

	return hasWithdrawal
}

// fwdAttrsHaveReach scans path attributes for MP_REACH_NLRI (code 14).
// Uses minimal parsing: reads flags + type code, skips by length.
func fwdAttrsHaveReach(attrs []byte) bool {
	return fwdAttrsScanCode(attrs, 14)
}

// fwdAttrsHaveUnreach scans path attributes for MP_UNREACH_NLRI (code 15).
func fwdAttrsHaveUnreach(attrs []byte) bool {
	return fwdAttrsScanCode(attrs, 15)
}

// fwdAttrsScanCode scans path attributes for a specific attribute code.
// Attribute format: [1B flags][1B code][1-2B length][data].
// Extended length (flag bit 4 set) uses 2-byte length.
func fwdAttrsScanCode(attrs []byte, code byte) bool {
	off := 0
	for off+2 < len(attrs) {
		flags := attrs[off]
		attrCode := attrs[off+1]
		off += 2

		var attrLen int
		if flags&0x10 != 0 { // Extended length.
			if off+2 > len(attrs) {
				return false
			}
			attrLen = int(attrs[off])<<8 | int(attrs[off+1])
			off += 2
		} else {
			if off >= len(attrs) {
				return false
			}
			attrLen = int(attrs[off])
			off++
		}

		if attrCode == code {
			return true
		}
		off += attrLen
	}
	return false
}

// fwdReorderWithdrawalsFirst reorders a batch so withdrawals come before
// announcements (AC-25). This is safe because AC-23 route superseding
// ensures no prefix appears in both a withdrawal and announcement within
// the same batch. Reordering is stable within each group (withdrawal order
// and announcement order are preserved).
func fwdReorderWithdrawalsFirst(batch []fwdItem) []fwdItem {
	// Count withdrawals to avoid allocation when none exist.
	wdCount := 0
	for i := range batch {
		if batch[i].withdrawal {
			wdCount++
		}
	}
	if wdCount == 0 || wdCount == len(batch) {
		return batch // Nothing to reorder.
	}
	// Stable partition: withdrawals first, then announcements.
	// Uses a single pass with two write positions.
	result := make([]fwdItem, len(batch))
	wi, ai := 0, wdCount
	for i := range batch {
		if batch[i].withdrawal {
			result[wi] = batch[i]
			wi++
		} else {
			result[ai] = batch[i]
			ai++
		}
	}
	copy(batch, result)
	return batch
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
	//
	// Track with dispatchWG so Stop() waits for us before closing w.ch.
	// Must check stopped under mu before Add(1) -- if Stop() already called
	// Wait(), adding to a zero-counter WaitGroup is a race.
	fp.mu.Lock()
	if fp.stopped {
		fp.mu.Unlock()
		fp.safeBatchHandle(key, items)
		return
	}
	fp.dispatchWG.Add(1)
	fp.mu.Unlock()

	var remaining []fwdItem
	for i := range items {
		select {
		case <-fp.stopCh: // pool stopping — Stop() will close w.ch after dispatchWG
			remaining = items[i:]
			goto processDirect
		default: // not stopping — safe to attempt enqueue
		}
		select {
		case w.ch <- items[i]:
			// Enqueued successfully — worker loop will process it.
		default: // channel full — process rest directly
			remaining = items[i:]
			goto processDirect
		}
	}
	fp.dispatchWG.Done()
	return

processDirect:
	fp.dispatchWG.Done()
	fp.safeBatchHandle(key, remaining)
}

// fwdDrainTimer drains a stopped timer's channel to prevent stale fires.
func fwdDrainTimer(t clock.Timer) {
	select {
	case <-t.C():
	default: // Timer already drained or hadn't fired — safe to skip.
	}
}
