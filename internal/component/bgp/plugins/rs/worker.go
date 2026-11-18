// Design: docs/architecture/core-design.md — route server plugin
// Overview: server.go — RouteServer dispatch, batch accumulation, async forward

package rs

import (
	"context"
	"errors"
	"net/netip"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// workerKey names one captured physical source connection. Its zero value is
// invalid. Callers MUST construct it from the original delivered event; neither
// an address nor a replay receipt can substitute for either numeric identity.
// The address is immutable display/legacy selector data, never the identity.
type workerKey struct {
	owner        uint64
	connectionID uint64
	address      netip.Addr
}

func newWorkerKey(address string, owner, connectionID uint64) (workerKey, error) {
	if owner == 0 {
		return workerKey{}, errors.New("route-server source worker has no original peer owner")
	}
	if connectionID == 0 {
		return workerKey{}, errors.New("route-server source worker has no original physical connection")
	}
	peer, err := netip.ParseAddr(address)
	if err != nil {
		return workerKey{}, err
	}
	return workerKey{owner: owner, connectionID: connectionID, address: peer}, nil
}

// workItem represents a unit of work dispatched to a worker.
// Carries forward context directly to avoid sync.Map indirection.
// For DirectBridge delivery: msg is set (raw wire data).
// For fork-mode delivery: textPayload is set (deferred text parsing by worker).
type workItem struct {
	msgID             uint64
	sourcePolicyEpoch uint64
	sourceReceipt     uint64
	sourceOwner       uint64
	sourcePeer        string
	msg               *bgptypes.RawMessage
	validation        ribevents.ValidationRoute
	textPayload       string
	// The delivery receipt is owned by this item on both transports. It is
	// independent of the borrowed message wrapper and publisher's slice.
	reactorForwarded bool
	fastPathSkipped  []rpc.ForwardTarget
	// barrier, when set, makes the item a settle marker: the worker flushes
	// its batch and closes it, so every item queued before it was handled
	// (settleSource). It carries no UPDATE.
	barrier chan struct{}
}

// poolConfig holds configuration for a workerPool.
type poolConfig struct {
	chanSize           int
	idleTimeout        time.Duration
	bpReminderInterval time.Duration

	// onDrained completes an outstanding handled batch when both queues empty,
	// including final retirement after its overflow producer joins. The callback
	// MUST finish before onClose; already completed batches earn no repeat call.
	onDrained func(key workerKey)

	// onIdle is called on the worker goroutine when the idle timer fires,
	// before the worker decides whether to exit. The worker is still the
	// registered one, so per-worker state is not shared yet. Nil means none.
	onIdle func(key workerKey)

	// onClose is called on the worker goroutine as it exits on PeerDown or
	// Stop, after its last item. PeerDown keeps the closing worker registered
	// until then, so per-worker state is not shared yet. Nil means none.
	// onClose MUST run after the final owed onDrained callback has completed.
	onClose func(key workerKey)
}

// worker is a single long-lived goroutine processing items for one source-peer key.
type worker struct {
	stopOnce   sync.Once // all shutdown callers share the closeCh signal
	retireOnce sync.Once // concurrent physical/terminal callers close ch once
	ch         chan workItem
	done       chan struct{} // closed when goroutine exits
	closeCh    chan struct{} // closed on PeerDown/Stop to signal drain goroutine

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
// Dispatch holds mu through enqueue, so validation callbacks may race with
// PeerDown/Stop without sending to a closed or idle-expired worker.
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
	// Used by flow control low-water check (cleared when channel drains below 10%).
	// Separate from backpressure because BackpressureDetected clears on read.
	inBackpressure sync.Map // workerKey → bool

	// bpLastLog tracks the last time a backpressure WARN was emitted per key.
	// Used as a rate-limiter: logs only when absent (first event) or when
	// bpReminderInterval has elapsed (periodic reminder). Cleared on
	// PeerDown/idle exit so a reconnecting peer gets a fresh WARN.
	bpLastLog sync.Map // workerKey → time.Time

	// onLowWater is called when a worker's channel drops below 10% after
	// being in backpressure. Used by dispatch to trigger resume RPCs.
	// Wide hysteresis band (full→10%) minimizes pause/resume cycles.
	onLowWater func(key workerKey)

	// count tracks active workers for WorkerCount() without holding mu.
	count atomic.Int32
}

// newWorkerPool creates a new worker pool with the given handler and configuration.
func newWorkerPool(handler func(key workerKey, item workItem), cfg poolConfig) *workerPool {
	if cfg.chanSize <= 0 {
		cfg.chanSize = 4096
	}
	if cfg.idleTimeout <= 0 {
		cfg.idleTimeout = 5 * time.Second
	}
	if cfg.bpReminderInterval <= 0 {
		cfg.bpReminderInterval = 10 * time.Second
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
// Callers must clean up associated state (e.g., cache entries) on false.
func (wp *workerPool) Dispatch(key workerKey, item workItem) bool {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return wp.dispatchLocked(key, item)
}

// dispatchLocked enqueues into the exact original key while mu is held.
// Dispatch and settleOwner MUST hold mu throughout this call.
func (wp *workerPool) dispatchLocked(key workerKey, item workItem) bool {
	if wp.stopped {
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
	select {
	case <-w.closeCh:
		return false
	default:
		// The registered worker is still accepting this source's events.
	}

	// Non-blocking enqueue. If overflow is non-empty or the drain goroutine
	// has an in-flight item, all new items must go through overflow to
	// preserve FIFO order. Otherwise, try channel first.
	w.overflowMu.Lock()
	if len(w.overflow) > 0 || w.draining {
		// Overflow or drain in flight — append to maintain FIFO.
		w.overflow = append(w.overflow, item)
		w.overflowMu.Unlock()
	} else {
		// Try non-blocking channel send. The closeCh case prevents a
		// send-on-closed-channel panic when Stop/PeerDown is closing
		// the worker concurrently.
		select {
		case w.ch <- item: // Channel has space — sent directly.
			w.overflowMu.Unlock()
			wp.checkBackpressure(key, w)
			return true
		case <-w.closeCh: // Shutting down — bail out.
			w.overflowMu.Unlock()
			return false
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
// into the channel as space opens. Retirement seals admission and MUST join
// this producer after it sends every admitted item, before closing the channel.
func (wp *workerPool) drainLoop(w *worker) {
	defer w.drainWg.Done()
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			logger().Error("drain loop panic recovered",
				"panic", r,
				"stack", string(buf[:n]),
			)
			// Clear draining flag so peer shutdown isn't blocked.
			w.overflowMu.Lock()
			w.draining = false
			w.overflowMu.Unlock()
		}
	}()

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

		w.ch <- item // The captured retirement joins every admitted overflow item.
	}
}

// checkBackpressure checks if the worker's depth has reached channel capacity.
// Triggers when depth >= cap (channel full, overflow active). No headroom is
// reserved because the overflow buffer handles any in-flight items during
// pause latency — nothing is ever dropped.
//
// Logging is rate-limited by bpLastLog: WARN fires on first detection (absent)
// or when bpReminderInterval has elapsed (periodic reminder during sustained
// backpressure). This prevents log spam during pause/resume oscillation.
func (wp *workerPool) checkBackpressure(key workerKey, w *worker) {
	if w.depth() >= cap(w.ch) { // channel full
		wp.inBackpressure.Store(key, true)
		wp.backpressure.Store(key, true)

		var shouldLog bool
		if lastRaw, ok := wp.bpLastLog.Load(key); ok {
			last, _ := lastRaw.(time.Time)
			shouldLog = time.Since(last) >= wp.cfg.bpReminderInterval
		} else {
			shouldLog = true // First backpressure event for this key.
		}
		if shouldLog {
			logger().Warn("backpressure",
				"source-peer", key.address, "source-owner", key.owner, "connection-id", key.connectionID,
				"queue-depth", w.depth(),
				"capacity", cap(w.ch),
			)
			wp.bpLastLog.Store(key, time.Now())
		}
	}
}

// workerRetirement owns one already-sealed original worker. Its zero value is
// invalid; captureRetirementLocked is its only producer. Keeping the reference
// makes a concurrent pool stop independent of a later map lookup.
type workerRetirement struct {
	key      workerKey
	original *worker
}

// captureRetirementLocked seals the actual mapped worker under mu. Callers
// MUST hold mu and MUST finish the returned retirement outside that lock.
// The worker MUST remain registered until its real handler and close callback
// finish, so a late same-key dispatch cannot create a competing incarnation.
func (wp *workerPool) captureRetirementLocked(key workerKey, original *worker) workerRetirement {
	original.stopOnce.Do(func() { close(original.closeCh) })
	wp.inBackpressure.Delete(key)
	wp.backpressure.Delete(key)
	wp.bpLastLog.Delete(key)
	return workerRetirement{key: key, original: original}
}

// finishRetirement MUST receive the captured original reference, with its
// admission already sealed, and MUST be called without mu held. Concurrent
// callers join the same drain, handler and onClose; only the matching mapped
// incarnation can be removed or have its pressure state cleared afterward.
func (wp *workerPool) finishRetirement(retirement workerRetirement) {
	retirement.original.retireOnce.Do(func() {
		retirement.original.drainWg.Wait()
		close(retirement.original.ch)
	})
	<-retirement.original.done
	wp.mu.Lock()
	if wp.workers[retirement.key] == retirement.original {
		delete(wp.workers, retirement.key)
		wp.inBackpressure.Delete(retirement.key)
		wp.backpressure.Delete(retirement.key)
		wp.bpLastLog.Delete(retirement.key)
	}
	wp.mu.Unlock()
}

// PeerDown joins only the worker for the captured physical source key.
// Callers MUST use its original delivered key, never the current peer address,
// and MUST call without route-server locks held. Absence means no mapped work
// remains; sealed workers stay mapped until every original obligation finishes.
func (wp *workerPool) PeerDown(key workerKey) {
	wp.mu.Lock()
	original := wp.workers[key]
	if original == nil {
		wp.mu.Unlock()
		return
	}
	retirement := wp.captureRetirementLocked(key, original)
	wp.mu.Unlock()
	wp.finishRetirement(retirement)
}

// settleOwner appends one marker to every already-admitted physical worker
// of the original owner and waits for those markers. The snapshot and enqueue
// share mu, so an idle exit cannot make a replacement lane from a stale key.
// No address lookup or new worker is involved. Callers MUST finish upstream
// source admission and MUST call without route-server locks held. The callback
// MUST flush the lane's forward batch before closing its marker.
func (wp *workerPool) settleOwner(ctx context.Context, owner uint64) error {
	if owner == 0 {
		return errWorkerOwnerMissing
	}
	wp.mu.Lock()
	stopped := wp.stopped
	pending := make([]chan struct{}, 0, len(wp.workers))
	for key, original := range wp.workers {
		if key.owner != owner {
			continue
		}
		select {
		case <-original.closeCh:
			// Retirement has sealed admission. Its actual completion also joins the
			// overflow producer, final batch flush and close callback.
			pending = append(pending, original.done)
		default:
			marker := make(chan struct{})
			if !wp.dispatchLocked(key, workItem{barrier: marker}) {
				wp.mu.Unlock()
				return errWorkerPoolClosing
			}
			pending = append(pending, marker)
		}
	}
	wp.mu.Unlock()
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if stopped {
		return errWorkerPoolClosing
	}
	return nil
}

// endOwner joins the current physical workers of an actual terminal Peer
// removal. The event caller MUST have ended that original owner's admission
// before calling. A transport DOWN MUST instead call PeerDown with its one
// captured physical key, and MUST NOT end the original owner's successor.
func (wp *workerPool) endOwner(owner uint64) error {
	if owner == 0 {
		return errWorkerOwnerMissing
	}
	wp.mu.Lock()
	retirements := make([]workerRetirement, 0, len(wp.workers))
	for key, original := range wp.workers {
		if key.owner != owner {
			continue
		}
		retirements = append(retirements, wp.captureRetirementLocked(key, original))
	}
	wp.mu.Unlock()
	for _, retirement := range retirements {
		wp.finishRetirement(retirement)
	}
	return nil
}

var errWorkerOwnerMissing = errors.New("route-server worker operation has no original peer owner")
var errWorkerPoolClosing = errors.New("route-server source worker is closing or the pool is stopping")

// Stop seals pool admission and every mapped worker before joining any of
// them. It MUST finish each captured retirement without mu held. Repeated or
// concurrent Stop calls join the same registered obligations; no caller sees
// pending work disappear merely because another shutdown began.
func (wp *workerPool) Stop() {
	wp.mu.Lock()
	wp.stopped = true
	select {
	case <-wp.stopCh:
		// An earlier Stop already sealed the pool; its workers still join below.
	default:
		close(wp.stopCh)
	}
	retirements := make([]workerRetirement, 0, len(wp.workers))
	for key, original := range wp.workers {
		retirements = append(retirements, wp.captureRetirementLocked(key, original))
	}
	wp.mu.Unlock()
	for _, retirement := range retirements {
		wp.finishRetirement(retirement)
	}
}

// setChanSize updates the channel capacity for newly created workers.
// Existing workers keep their current channel size until they exit (idle timeout)
// and are recreated. Safe for concurrent use with Dispatch.
func (wp *workerPool) setChanSize(size int) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	if size > 0 {
		wp.cfg.chanSize = size
	}
}

// WorkerCount returns the number of active workers.
func (wp *workerPool) WorkerCount() int {
	return int(wp.count.Load())
}

// backpressureDetected returns true if the channel for the given key has
// triggered a backpressure warning (channel full) since the last check.
// Clears the flag on read so the caller sees each backpressure event once.
func (wp *workerPool) backpressureDetected(key workerKey) bool {
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
				"source-peer", key.address, "source-owner", key.owner, "connection-id", key.connectionID,
				"msgID", item.msgID,
				"panic", r,
			)
		}
	}()
	wp.handler(key, item)
}

// closeWorker runs the onClose callback of a worker exiting on PeerDown or Stop.
func (wp *workerPool) closeWorker(key workerKey) {
	if wp.cfg.onClose != nil {
		wp.cfg.onClose(key)
	}
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

	// Only this worker goroutine owns the outstanding handled-batch fact.
	// Every handling path MUST owe it; every flush MUST clear it. Retirement
	// MUST finish that obligation before onClose and the deferred done signal.
	drainOwed := false
	flush := func() {
		if !drainOwed {
			return
		}
		drainOwed = false
		if wp.cfg.onDrained != nil {
			wp.cfg.onDrained(key)
		}
	}
	finish := func() {
		flush()
		wp.closeWorker(key)
	}

	for {
		select {
		case item, ok := <-w.ch:
			if !ok {
				// Both close exits MUST settle the same final batch.
				finish()
				return
			}
			if !idle.Stop() {
				drainTimer(idle)
			}
			wp.safeHandle(key, item)
			drainOwed = true

			// Flow control low-water: if depth dropped below 10%, fire resume.
			// Wide hysteresis band (full / 10% low) minimizes pause/resume
			// cycles. The inBackpressure flag is consumed (deleted) so the
			// callback fires once per high→low transition.
			if w.depth()*10 < cap(w.ch) { // <10% of channel capacity
				if _, wasBP := wp.inBackpressure.LoadAndDelete(key); wasBP {
					if wp.onLowWater != nil {
						wp.onLowWater(key)
					}
				}
			}

			// Channel drain check: flush partial batches when no items pending.
			if len(w.ch) == 0 {
				if w.overflowLen() == 0 {
					flush()
				}
			}

			idle.Reset(wp.cfg.idleTimeout)

		case <-w.closeCh:
			// The captured retirement MUST join the overflow producer before
			// closing ch. A momentarily empty channel is not completion: its
			// producer may still own an admitted in-flight item.
			for item := range w.ch {
				wp.safeHandle(key, item)
				drainOwed = true
			}
			finish()
			return

		case <-idle.C:
			if wp.cfg.onIdle != nil {
				wp.cfg.onIdle(key)
			}
			// Idle timeout — remove self from pool and exit.
			// Check channel AND overflow under lock: if either has items
			// or overflow is draining, we must not exit.
			wp.mu.Lock()
			// A retirement may have sealed this worker while onIdle was out
			// of the lock. It MUST reach the ordinary close path and finish
			// onClose before its exact registration can be replaced.
			select {
			case <-w.closeCh:
				wp.mu.Unlock()
				continue
			default:
				// Admission is still open; only a genuinely idle worker exits.
			}
			w.overflowMu.Lock()
			pending := len(w.overflow) > 0 || w.draining
			w.overflowMu.Unlock()
			if len(w.ch) > 0 || pending {
				wp.mu.Unlock()
				idle.Reset(wp.cfg.idleTimeout)
				continue
			}
			// A sealed worker never reaches this idle deletion. Remove only
			// this still-open incarnation, before any replacement can enter.
			if wp.workers[key] == w {
				wp.inBackpressure.Delete(key)
				wp.backpressure.Delete(key)
				wp.bpLastLog.Delete(key)
				delete(wp.workers, key)
			}
			wp.mu.Unlock()
			return
		}
	}
}
