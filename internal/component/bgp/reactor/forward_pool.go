// Design: docs/architecture/core-design.md — per-peer forward worker pool
// Design: docs/architecture/buffer-architecture.md -- zero-copy, copy-on-modify (Incoming/Outgoing Peer Pools, Global Shared Pool)
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
// A channel item aliases the source cache entry's bytes zero-copy. An overflow
// item does not: it can outlive the entry, so dispatchOverflow copies its bodies
// into the overflow handle it already holds (ownOverflowBodies, forward_body.go).
//
// Weight tracking sizes per-peer channel capacity proportional to the peer's
// NLRI volume. Congestion control uses two thresholds (warn/critical) on the
// shared buffer pool usage ratio to pause slow peers before memory exhaustion.

package reactor

import (
	"bytes"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
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
	rawBodies     [][]byte          // Zero-copy or split pieces: SendRawUpdateBody per entry
	updates       []*message.Update // Re-encode path: SendUpdate per entry
	peer          *Peer             // Target peer for all operations
	done          func()            // Called after all ops complete (Release cache entry)
	peerBufIdx    int               // 1-based index into per-peer pool; 0 = not from per-peer pool
	peerPoolRef   *peerPool         // Pool to return buffer to (avoids map lookup + lock)
	overflowBuf   BufHandle         // Overflow MixedBufMux handle holding this item's copied bodies (ownOverflowBodies); nil Buf = not from overflow
	meta          map[string]any    // Route metadata from ReceivedUpdate; set on sent events
	sourcePeerStr string            // Source peer address string for ribOut stale-scoping
	supersedeKey  uint64            // FNV-1a hash of raw body for route superseding (AC-23); 0 = no superseding
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

	// Select the destination from the first REAL item: sentinel items (barrier
	// done-callbacks, overflow wake-ups) carry a nil peer and no data, and a
	// batch may interleave them with real items. Returning on items[0] alone
	// would skip the writes of every real item batched behind a sentinel
	// (their done() would still fire in safeBatchHandle -- silent route loss).
	var peer *Peer
	for i := range items {
		if items[i].peer != nil {
			peer = items[i].peer
			break
		}
	}
	if peer == nil {
		// Sentinel-only batch — nothing to write. done() is called by
		// safeBatchHandle regardless.
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
		session.sentMeta = nil         // Clear route metadata on all exit paths.
		session.sentSourcePeerStr = "" // Clear source peer string on all exit paths.
		// Clear write deadline (zero value = no deadline).
		_ = conn.SetWriteDeadline(time.Time{})
	}()

	// Bucket merge: group items with identical post-egress attrs into fewer
	// outbound UPDATEs with packed NLRIs. Reduces per-message header overhead
	// and TCP write syscall count.
	nc := peer.negotiated.Load()
	extMsg := nc != nil && nc.ExtendedMessage
	items = fwdBucketMerge(items, fwdBucketMaxBodySize(extMsg))

	for i := range items {
		session.sentMeta = items[i].meta                   // Route metadata for sent event callbacks.
		session.sentSourcePeerStr = items[i].sourcePeerStr // Source peer for ribOut stale-scoping.
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
			// Pre-filtered: forwardUpdateCore already ran this peer's export chain
			// (and only then the EBGP prepend). See writeUpdatePreFiltered.
			if err := session.writeUpdatePreFiltered(update); err != nil {
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
	ch        chan fwdItem
	done      chan struct{} // closed when goroutine exits
	pending   atomic.Int32  // items about to be sent (between mu.Unlock and channel send)
	batchBuf  []fwdItem     // reusable drain buffer — owned by runWorker goroutine
	addrLabel string        // cached key.peerAddr.Addr().String(), computed once at creation

	// Overflow buffer for non-blocking dispatch (TryDispatch fallback).
	// Protected by overflowMu. Items are moved to the channel by the worker
	// goroutine after processing each batch.
	overflowMu sync.Mutex
	overflow   []fwdItem

	// overflowPending counts items dispatched to overflow whose bytes have NOT
	// yet been written to the destination. That is wider than len(w.overflow)
	// on purpose: it also covers the items a drain has snapshotted out of the
	// slice, the items sitting in w.ch, and the batch fwdBatchHandler is
	// writing right now. While nonzero, TryDispatch refuses new items and the
	// route-server rail refuses its direct write, so both route the newer item
	// through dispatchOverflow behind the pending ones.
	//
	// Every narrower count leaves an inversion window. len(w.overflow) alone
	// misses the drain snapshot. Decrementing as each item enters w.ch misses
	// the gap between that enqueue and the worker taking session.writeMu, and
	// a direct write that wins that race overtakes the very items an ordering
	// hold has just released.
	//
	// Mutated only under overflowMu: dispatchOverflow adds one per appended
	// item, and the worker re-derives the count from len(w.overflow) once a
	// batch completes with an empty channel (runWorker), which is the first
	// moment every item that left the slice is provably written.
	//
	// It is a POINTER because the destination peer holds the same counter
	// (Peer.fwdOverflowPending): the route-server rail reads it there with two
	// atomic loads instead of taking fp.mu.RLock and hashing a fwdKey per
	// destination per UPDATE. A handle into this struct would work equally well
	// for the read and would pin the whole worker after its goroutine exits,
	// batchBuf included -- up to batchLimit fwdItems per peer that ever
	// overflowed. Eight bytes on their own pin eight bytes. newWorker is the
	// only place a fwdWorker is built, and it allocates this.
	overflowPending *atomic.Int64

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
	mu      sync.RWMutex
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
	overflowed atomic.Int64 // fell through to dispatchOverflow
	addrLabel  string       // cached addr.String(), computed once at creation
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

// setOverflowMux sets the shared overflow MixedBufMux for the pool.
// When set, overflow dispatch acquires buffer handles from this mux.
// Must be called before concurrent use.
func (fp *fwdPool) setOverflowMux(m *MixedBufMux) {
	fp.overflowMux = m
}

// registerOutgoingPool creates an Outgoing Peer Pool for the given destination peer.
// bufSize is the negotiated message size (4K standard, 64K ExtMsg).
// Called at session establishment. Safe for concurrent use.
func (fp *fwdPool) registerOutgoingPool(key fwdKey, bufSize int) {
	fp.mu.Lock()
	fp.outgoingPools[key] = newPeerPool(bufSize)
	fp.mu.Unlock()
}

// unregisterOutgoingPool removes the Outgoing Peer Pool for the given destination peer.
// Called at session teardown. Safe for concurrent use.
func (fp *fwdPool) unregisterOutgoingPool(key fwdKey) {
	fp.mu.Lock()
	delete(fp.outgoingPools, key)
	fp.mu.Unlock()
}

// outgoingPool returns the Outgoing Peer Pool for the given key, or nil.
// Used by ForwardUpdate to pass to buildModifiedPayload for copy-on-modify.
func (fp *fwdPool) outgoingPool(key fwdKey) *peerPool {
	fp.mu.RLock()
	pp := fp.outgoingPools[key]
	fp.mu.RUnlock()
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
	// Fast path: RLock when the worker already exists.
	// pending.Add(1) must happen under RLock so the idle handler (which takes
	// Lock) always sees pending > 0 while a Dispatch is in flight.
	fp.mu.RLock()
	if fp.stopped {
		fp.mu.RUnlock()
		return false
	}

	fp.dispatchWG.Add(1)
	w, ok := fp.workers[key]
	if ok {
		w.pending.Add(1)
	}
	fp.mu.RUnlock()

	if !ok {
		// Slow path: exclusive lock to create worker.
		fp.dispatchWG.Done()
		fp.mu.Lock()
		if fp.stopped {
			fp.mu.Unlock()
			return false
		}
		fp.dispatchWG.Add(1)
		w, ok = fp.workers[key]
		if !ok {
			w = fp.newWorker(key)
		}
		w.pending.Add(1)
		fp.mu.Unlock()
	}

	defer fp.dispatchWG.Done()

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
// On false, the caller should use dispatchOverflow as a fallback.
//
// If the send fails because the channel is full:
//   - Sets the worker's congested flag (if not already set)
//   - Fires onCongested callback on false->true transition
func (fp *fwdPool) TryDispatch(key fwdKey, item fwdItem) bool {
	// Fast path: RLock for the common case where the worker already exists.
	// The non-blocking send is performed under RLock to prevent the idle
	// handler (which takes Lock) from deleting the worker between lookup
	// and send. This is safe because the send never blocks.
	fp.mu.RLock()
	if fp.stopped {
		fp.mu.RUnlock()
		return false
	}

	fp.dispatchWG.Add(1)
	w, ok := fp.workers[key]

	if !ok {
		fp.mu.RUnlock()
		// Slow path: worker doesn't exist, need exclusive lock to create.
		fp.dispatchWG.Done()
		fp.mu.Lock()
		if fp.stopped {
			fp.mu.Unlock()
			return false
		}
		fp.dispatchWG.Add(1)
		w, ok = fp.workers[key]
		if !ok {
			w = fp.newWorker(key)
		}
		w.pending.Add(1)
		fp.mu.Unlock()

		// The re-check may have found an existing worker with pending
		// overflow items; refuse so FIFO is preserved (see below).
		if w.overflowPending.Load() > 0 {
			w.pending.Add(-1)
			fp.dispatchWG.Done()
			return false
		}

		select {
		case w.ch <- item:
			w.pending.Add(-1)
			fp.dispatchWG.Done()
			return true
		default:
			w.pending.Add(-1)
			fp.dispatchWG.Done()
			return false
		}
	}

	// FIFO gate: while overflow items are pending (queued, snapshotted by an
	// in-flight drain, on the channel, or in the batch being written), a direct
	// channel send would let this newer item overtake them. Refuse so the
	// caller routes it through dispatchOverflow behind the pending items. A
	// send behind items already ON the channel would keep FIFO on its own, so
	// this refusal is wider than this rail strictly needs; the count is one
	// condition serving both rails, and the route-server rail's direct write
	// needs every one of those states (Peer.forwardOverflowPending, which reads
	// this same counter through the destination). The congested flag
	// is not touched here: it was already set when the overflow episode began,
	// and clearing is the worker's job once overflow fully drains.
	if w.overflowPending.Load() > 0 {
		fp.mu.RUnlock()
		fp.dispatchWG.Done()
		return false
	}

	// Fast path: non-blocking send under RLock.
	select {
	case w.ch <- item:
		fp.mu.RUnlock()
		fp.dispatchWG.Done()
		return true
	default: // channel full
		fp.mu.RUnlock()
		fp.dispatchWG.Done()

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

// dispatchOverflow adds an item to the per-worker overflow buffer.
// Creates the worker lazily if it doesn't exist. The worker goroutine
// drains overflow items after each batch from the channel.
//
// The overflow buffer is unbounded. Routes are critical data and must
// never be dropped. Memory growth from a slow peer is preferable to
// silent routing inconsistency.
//
// Returns true if the item was buffered, false if the pool is stopped
// (in which case done() is called immediately to prevent cache leaks).
func (fp *fwdPool) dispatchOverflow(key fwdKey, item fwdItem) bool {
	// Fast path: RLock when the worker already exists.
	// pending.Add(1) under RLock prevents the idle handler from deleting
	// the worker between lookup and the overflow append below. RLock is
	// released before buffer acquisition (which takes its own RLock).
	fp.mu.RLock()
	if fp.stopped {
		fp.mu.RUnlock()
		// done() releases the recent-update CACHE reference. Returning the
		// item's pooled buffers is a separate obligation, and the caller has
		// already acquired them: the two forward rails construct the fwdItem
		// with peerBufIdx and peerPoolRef set before dispatching. Neither
		// TryDispatch nor the caller releases on a false return, so without
		// this the Outgoing Peer Pool buffer is never handed back.
		fp.releaseItem(&item)
		if item.done != nil {
			item.done()
		}
		return false
	}

	fp.dispatchWG.Add(1)
	w, ok := fp.workers[key]
	if ok {
		w.pending.Add(1)
	}
	fp.mu.RUnlock()

	if !ok {
		// Slow path: exclusive lock to create worker.
		fp.dispatchWG.Done()
		fp.mu.Lock()
		if fp.stopped {
			fp.mu.Unlock()
			// Same obligation as the fast-path stopped branch above: the item
			// still holds the caller's pooled buffer here.
			fp.releaseItem(&item)
			if item.done != nil {
				item.done()
			}
			return false
		}
		fp.dispatchWG.Add(1)
		w, ok = fp.workers[key]
		if !ok {
			w = fp.newWorker(key)
		}
		w.pending.Add(1)
		fp.mu.Unlock()
	}

	defer fp.dispatchWG.Done()
	defer w.pending.Add(-1)

	// Check buffer denial (AC-2): if the congestion controller says this
	// destination peer is the worst offender, skip pool acquisition. The item
	// still goes to unbounded overflow (routes never dropped), but the denial
	// signal feeds into teardown decisions.
	denied := fp.congestion.shouldDeny(w.addrLabel)

	// Acquire overflow MixedBufMux handle if available and not denied.
	// Skip for sentinel items (peer == nil) — they carry no route data
	// and should not consume pool capacity meant for real updates.
	if fp.overflowMux != nil && item.peer != nil && !denied {
		// Determine buffer size from the destination peer's Outgoing Peer Pool.
		// Default to 4K (standard) if no pool is registered.
		bufSize := message.MaxMsgLen
		fp.mu.RLock()
		if pp := fp.outgoingPools[key]; pp != nil {
			bufSize = pp.bufSize
		}
		fp.mu.RUnlock()

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

	// The item is about to sit in an unbounded queue for as long as this
	// destination stays behind, so it must stop aliasing the source entry's
	// buffers before it gets there (ownOverflowBodies).
	ownOverflowBodies(&item)

	w.overflowMu.Lock()

	// Publish the count to the destination BEFORE the count can move. The
	// route-server rail reads it there (Peer.forwardOverflowPending), so a
	// reader that finds no handle is ordered before this store and therefore
	// before the Add below: its answer describes a moment when the item had not
	// arrived, which is the answer a pool lookup gives in the same window.
	// Sentinels carry no peer and no bytes, so they publish nothing.
	if item.peer != nil {
		item.peer.fwdOverflowPending.Store(w.overflowPending)
	}

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
	w.overflowPending.Add(1)

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

	// Wake an idle worker. The worker only drains overflow after receiving a
	// channel item, and the TryDispatch FIFO gate routes items here even when
	// the channel has room -- so an item appended just after the worker's
	// final drain would otherwise sit in overflow with the worker blocked on
	// an empty channel. A non-blocking nil-peer sentinel closes that wedge:
	// it carries no data (fwdBatchHandler skips nil-peer items) and only
	// triggers the next drain cycle. Safe against Stop: we are inside the
	// dispatchWG window, so the channel cannot close under the send.
	if len(w.ch) == 0 {
		select {
		case w.ch <- fwdItem{}:
		default: // channel gained an item meanwhile -- that item wakes the worker
		}
	}
	return true
}

// newWorker creates a new fwdWorker, registers it in the pool, and starts its goroutine.
// Caller must hold fp.mu.
func (fp *fwdPool) newWorker(key fwdKey) *fwdWorker {
	w := &fwdWorker{
		ch:              make(chan fwdItem, fp.cfg.chanSize),
		done:            make(chan struct{}),
		addrLabel:       key.peerAddr.Addr().String(),
		overflowPending: new(atomic.Int64),
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
		kw.w.overflowPending.Store(0)
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

// overflowDepths returns a snapshot of per-destination-peer overflow depth.
// Each entry maps peer address string (IP-only, no port) to the number of
// items currently in its overflow buffer. IP-only format matches the key
// format used by weightTracker (peerAddrLabel) and Prometheus labels.
// Called by the metrics update loop; must not block.
func (fp *fwdPool) overflowDepths() map[string]int {
	fp.mu.RLock()
	result := make(map[string]int, len(fp.workers))
	for _, w := range fp.workers {
		w.overflowMu.Lock()
		result[w.addrLabel] += len(w.overflow)
		w.overflowMu.Unlock()
	}
	fp.mu.RUnlock()
	return result
}

// poolUsedRatio returns the fraction of overflow pool capacity in use (0.0 to 1.0).
// Reads from MixedBufMux stats (usedBytes/budgetBytes).
// Returns 0.0 if no overflow mux is configured. Called by the metrics update loop.
func (fp *fwdPool) poolUsedRatio() float64 {
	if fp.overflowMux != nil {
		return fp.overflowMux.usedRatio()
	}
	return 0.0
}

// recordForwarded increments the forwarded counter for a source peer.
// Called from ForwardUpdate when TryDispatch succeeds.
func (fp *fwdPool) recordForwarded(sourcePeer netip.Addr) {
	fp.getSourceStats(sourcePeer).forwarded.Add(1)
}

// recordOverflowed increments the overflowed counter for a source peer.
// Called from ForwardUpdate when dispatchOverflow is used.
func (fp *fwdPool) recordOverflowed(sourcePeer netip.Addr) {
	fp.getSourceStats(sourcePeer).overflowed.Add(1)
}

// getSourceStats returns the stats for a source peer, creating if needed.
func (fp *fwdPool) getSourceStats(sourcePeer netip.Addr) *fwdSourceStats {
	fp.srcStatsMu.Lock()
	s, ok := fp.srcStats[sourcePeer]
	if !ok {
		s = &fwdSourceStats{addrLabel: sourcePeer.String()}
		fp.srcStats[sourcePeer] = s
	}
	fp.srcStatsMu.Unlock()
	return s
}

// sourceOverflowRatios returns per-source-peer overflow ratio: overflowed/(forwarded+overflowed).
// Returns 0.0 for peers with no overflow. Called by the metrics update loop.
// Keys are string-form addresses for display/metrics consumption.
func (fp *fwdPool) sourceOverflowRatios() map[string]float64 {
	fp.srcStatsMu.Lock()
	result := make(map[string]float64, len(fp.srcStats))
	for _, s := range fp.srcStats {
		fwd := s.forwarded.Load()
		ovf := s.overflowed.Load()
		total := fwd + ovf
		if total == 0 {
			result[s.addrLabel] = 0.0
		} else {
			result[s.addrLabel] = float64(ovf) / float64(total)
		}
	}
	fp.srcStatsMu.Unlock()
	return result
}

// removeSourceStats deletes the source stats entry for a peer.
// Called on peer disconnect to prevent unbounded srcStats growth.
func (fp *fwdPool) removeSourceStats(sourcePeer netip.Addr) {
	fp.srcStatsMu.Lock()
	delete(fp.srcStats, sourcePeer)
	fp.srcStatsMu.Unlock()
}

// safeBatchHandle calls the handler with panic recovery, then calls done() for
// every item in the batch. The done callbacks (Release) are guaranteed to run
// even if the handler panics. Without this guarantee, a panicking handler would
// leak cache entries.
//
// The batch is handed to the handler in the order it was queued. It used to be
// partitioned withdrawals-first for convergence, copied from RustBGPd, where
// that is safe because its PendingTx deduplicates by PREFIX so one prefix never
// appears in both groups (docs/architecture/congestion-industry.md). Ze
// deduplicates by BYTES instead -- fwdSupersedeKey hashes the whole body and
// fwdBodiesEqual compares it -- and an announce and a withdraw of one prefix are
// never byte-identical, so the partition inverted exactly that pair and left the
// peer holding a prefix that had been withdrawn. It also bought nothing here:
// fwdBatchHandler writes the whole batch under one writeMu and flushes once, so
// the partition moved bytes inside a single write rather than sending a
// withdrawal any sooner.
func (fp *fwdPool) safeBatchHandle(key fwdKey, items []fwdItem) {
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
	h := uint64(fnvOffset64)
	for _, body := range rawBodies {
		for _, b := range body {
			h ^= uint64(b)
			h *= fnvPrime64
		}
	}
	return h
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

// runWorker is the long-lived goroutine for one destination peer.
// It reads items from the channel using drain-batch (one blocking receive +
// non-blocking drain of available items), calls the batch handler, and exits
// on idle timeout or channel close (Stop).
//
// After processing each batch from the channel, the worker drains overflow
// items (added by dispatchOverflow) into the channel or processes them directly.
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
			fp.congestion.checkTeardown(key.peerAddr)

			// Drain overflow items into the channel after processing.
			fp.drainOverflow(key, w)

			// Check congestion: clear if channel dropped below low-water mark.
			// Single lock acquisition to atomically decide whether to fire onResumed.
			// overflowPending must also be zero: a nonzero count means older
			// items are still queued behind overflow (possibly in a drain
			// snapshot) and the episode is not over.
			if len(w.ch) <= lowWater {
				var fireResumed bool
				w.overflowMu.Lock()
				// An empty channel here is the first moment every item that
				// ever left w.overflow is provably WRITTEN: the batch above is
				// done, and this goroutine is the only one that takes from
				// w.ch. So what the destination is still owed is exactly what
				// is still queued. Re-deriving the count here, rather than
				// decrementing it as each item entered the channel, is what
				// closes the enqueue-to-writeMu window against the route-server
				// rail's direct write (Peer.forwardOverflowPending).
				if len(w.ch) == 0 {
					w.overflowPending.Store(int64(len(w.overflow)))
				}
				if w.congested && len(w.overflow) == 0 && w.overflowPending.Load() == 0 {
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
			// pending under RLock before sending, so Lock here blocks until any
			// in-flight RLock releases. If pending > 0, a send is in flight.
			fp.mu.Lock()
			if len(w.ch) > 0 || w.pending.Load() > 0 {
				fp.mu.Unlock()
				idle.Reset(fp.cfg.idleTimeout)
				continue
			}
			// Also check overflow — don't idle-exit with pending overflow items.
			w.overflowMu.Lock()
			hasOverflow := len(w.overflow) > 0 || w.overflowPending.Load() > 0
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

// drainOverflow moves items from the overflow buffer into the channel.
// Called by the worker goroutine after each batch.
//
// FIFO invariant: items that cannot fit in the channel are pushed back to
// the FRONT of w.overflow so the next drainOverflow cycle (after the worker
// has drained a batch from the channel) picks them up in their original
// order. Running leftover items directly via safeBatchHandle would violate
// FIFO, because items already in the channel -- sitting behind the
// Barrier sentinel, for example -- would still be unread when the "direct"
// batch completes. That broke TestFwdPool_Barrier_WithOverflow: Barrier's
// sentinel fell through to the direct path, its done() fired, and Barrier
// returned while real items were still in the channel.
//
// Only the pool-stopped path still runs the remaining items directly:
// Stop() is about to close w.ch, so no further worker iterations will run.
// Preserving FIFO there is moot because the items are about to be drained
// as part of shutdown anyway.
//
// Track with dispatchWG so Stop() waits for us before closing w.ch.
// Must check stopped under mu before Add(1) -- if Stop() already called
// Wait(), adding to a zero-counter WaitGroup is a race.
func (fp *fwdPool) drainOverflow(key fwdKey, w *fwdWorker) {
	w.overflowMu.Lock()
	if len(w.overflow) == 0 {
		w.overflowMu.Unlock()
		return
	}

	// Ordering hold: the destination is inside its initial route sync, so the
	// dispatch gates parked these items here to sit BEHIND the route operations
	// its opQueue still holds (reactor_api_forward.go, forward_rs.go). Draining
	// now would put a forwarded withdraw on the wire ahead of the queued
	// announce of the same prefix and leave the peer holding a route that was
	// withdrawn. sendInitialRoutes clears the flag and wakes this worker
	// (Peer.wakeForwardOverflow), which is when the items go out.
	if overflowHeld(w.overflow) {
		w.overflowMu.Unlock()
		return
	}

	// Take all overflow items under the lock, then release.
	items := w.overflow
	w.overflow = nil
	w.overflowMu.Unlock()

	fp.mu.RLock()
	if fp.stopped {
		fp.mu.RUnlock()
		fp.safeBatchHandle(key, items)
		// Written, so the destination is owed only what arrived in w.overflow
		// while the batch ran. Same re-derivation runWorker makes, for the same
		// reason: the count may not drop before the bytes are out.
		w.overflowMu.Lock()
		w.overflowPending.Store(int64(len(w.overflow)))
		w.overflowMu.Unlock()
		return
	}
	fp.dispatchWG.Add(1)
	fp.mu.RUnlock()
	defer fp.dispatchWG.Done()

	for i := range items {
		// Honor a pending Stop without enqueueing further items. Remaining
		// items are pushed back to the overflow queue so Stop() picks them
		// up in its final drain loop.
		select {
		case <-fp.stopCh:
			fp.requeueOverflow(w, items[i:])
			return
		default: // not stopping -- fall through to the enqueue attempt below
		}
		select {
		case w.ch <- items[i]:
			// Enqueued successfully -- the worker loop will process it in FIFO
			// order. The item stays counted in overflowPending until it has
			// been WRITTEN: it is on the channel, not on the wire, and the
			// route-server rail would overtake it here with a direct write.
			// runWorker re-derives the count once a batch completes with an
			// empty channel.
		default: // channel full -- push remaining back to overflow to preserve FIFO
			// Running them directly here would break FIFO relative to the
			// items already in the channel. The next drainOverflow cycle
			// (after the worker drains a batch) picks them up in order.
			fp.requeueOverflow(w, items[i:])
			return
		}
	}
}

// overflowHeld reports whether this worker's overflow must stay parked because
// its destination peer is still inside its initial route sync.
//
// The predicate is Peer.forwardOrderHold, the same one the two dispatch gates
// park on (peer.go). Gate and hold must be one condition: a gate wider than its
// hold parks items nothing will release, and a hold wider than its gate releases
// items the gate never parked.
//
// The decision belongs to the first REAL item: sentinels (barrier
// done-callbacks, wake-ups) carry a nil peer, and fwdKey IS the destination, so
// every real item in one worker's overflow names the same peer. A worker holding
// nothing but sentinels holds nothing back.
//
// This runs with w.overflowMu held, and forwardOrderHold reads two atomics and
// takes no peer lock. Keep it that way: a predicate that took p.mu here would
// put p.mu under w.overflowMu, and the peer's own goroutine takes them the other
// way round.
func overflowHeld(items []fwdItem) bool {
	for i := range items {
		p := items[i].peer
		if p == nil {
			continue
		}
		return p.forwardOrderHold()
	}
	return false
}

// wakeOverflow nudges the worker for key so it re-evaluates the ordering hold in
// drainOverflow. Called when a destination's initial route sync ends
// (Peer.wakeForwardOverflow): the worker drains overflow only after a channel
// item, and a held worker has an empty channel by construction, so without this
// the parked items wait for the next forward to that peer.
//
// Same non-blocking nil-peer sentinel dispatchOverflow uses for the same reason.
// A full channel needs no wake: the worker has items to process and reaches
// drainOverflow on its own.
func (fp *fwdPool) wakeOverflow(key fwdKey) {
	fp.mu.RLock()
	if fp.stopped {
		fp.mu.RUnlock()
		return
	}
	// Inside the dispatchWG window the channel cannot close under the send:
	// Stop() sets stopped under the write lock, then waits.
	fp.dispatchWG.Add(1)
	w, ok := fp.workers[key]
	fp.mu.RUnlock()
	defer fp.dispatchWG.Done()

	if !ok || len(w.ch) > 0 {
		return
	}
	select {
	case w.ch <- fwdItem{}:
	default: // channel gained an item meanwhile -- that item wakes the worker
	}
}

// requeueOverflow prepends items to the front of w.overflow so they are
// picked up in their original FIFO order on the next drainOverflow cycle.
// Any items concurrently appended to w.overflow by dispatchOverflow (after
// this drain cycle took its snapshot) are kept in their relative order
// after the re-queued items.
func (fp *fwdPool) requeueOverflow(w *fwdWorker, leftover []fwdItem) {
	w.overflowMu.Lock()
	if len(w.overflow) == 0 {
		w.overflow = leftover
	} else {
		merged := make([]fwdItem, 0, len(leftover)+len(w.overflow))
		merged = append(merged, leftover...)
		merged = append(merged, w.overflow...)
		w.overflow = merged
	}
	w.overflowMu.Unlock()
}

// fwdDrainTimer drains a stopped timer's channel to prevent stale fires.
func fwdDrainTimer(t clock.Timer) {
	select {
	case <-t.C():
	default: // Timer already drained or hadn't fired — safe to skip.
	}
}
