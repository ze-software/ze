// Design: docs/architecture/api/process-protocol.md -- bounded growable lossless FIFO event delivery queue
// Related: delivery.go -- per-process event delivery loop and barrier admission
// Related: process.go -- Process lifecycle and effectiveDeliveryQueueMax

package process

import (
	"context"
	"sync"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	// eventQueueInitialCap is the initial ring slot count for a per-process
	// event queue, and the capacity it shrinks back to once drained empty.
	eventQueueInitialCap = 64
	// eventQueueShrinkThreshold is the backing-slice capacity above which
	// draining the ring empty allocates a fresh eventQueueInitialCap ring.
	eventQueueShrinkThreshold = 256
	// eventDeliveryBatchMax caps how many events deliveryLoop drains per
	// deliver-batch RPC (CallBatchRPC splits oversized frames at 16 MiB).
	eventDeliveryBatchMax = 512
)

// enqueueOutcome classifies the result of eventQueue.enqueue.
type enqueueOutcome uint8

const (
	enqueueOutcomeUnspecified enqueueOutcome = iota
	enqueueAdmitted
	enqueueCallerCanceled
	enqueueProcessClosed
)

// eventQueueWaiter represents one producer blocked at the queue ceiling.
// ready has capacity 1 so the consumer or close path wakes it without blocking.
type eventQueueWaiter struct {
	ready    chan struct{}
	signaled bool
}

// eventQueue is a bounded, dynamically growable, lossless FIFO ring queue for
// per-process event delivery.
//
// Safe for concurrent use by multiple producers and one consumer.
type eventQueue struct {
	mu       sync.Mutex
	buf      []EventDelivery
	head     int
	count    int
	maxCap   int
	closed   bool
	notEmpty chan struct{}
	waiters  []*eventQueueWaiter
}

// newEventQueue returns an empty ring queue bounded at maxCap items.
func newEventQueue(maxCap int) *eventQueue {
	if maxCap < 1 {
		panic("BUG: plugin/process: eventQueue maxCap must be positive")
	}
	initCap := min(eventQueueInitialCap, maxCap)
	return &eventQueue{
		buf:      make([]EventDelivery, initCap),
		maxCap:   maxCap,
		notEmpty: make(chan struct{}, 1),
	}
}

// Len returns the number of queued items.
func (q *eventQueue) Len() int {
	q.mu.Lock()
	n := q.count
	q.mu.Unlock()
	return n
}

// Cap returns the current backing-ring slot capacity.
func (q *eventQueue) Cap() int {
	q.mu.Lock()
	n := len(q.buf)
	q.mu.Unlock()
	return n
}

// maxCapacity returns the maximum queue ceiling.
func (q *eventQueue) maxCapacity() int {
	q.mu.Lock()
	n := q.maxCap
	q.mu.Unlock()
	return n
}

// setMaxCapacity updates the queue ceiling without dropping buffered items.
// If space becomes available, it wakes the next FIFO producer waiter.
func (q *eventQueue) setMaxCapacity(maxCap int) {
	if maxCap < 1 {
		panic("BUG: plugin/process: eventQueue maxCap must be positive")
	}
	q.mu.Lock()
	q.maxCap = maxCap
	if !q.closed && q.count < q.maxCap {
		q.wakeNextWaiterLocked()
	}
	q.mu.Unlock()
}

// enqueue appends d to the queue in FIFO order, growing the backing ring by 2x
// up to maxCap as needed. When the queue is at its ceiling, enqueue blocks
// without spawning goroutines until space frees, the queue closes, or one of
// ctx, procCtx, or extraDone cancels.
//
// onDepth, when non-nil, is invoked while holding q.mu so concurrent producer
// and consumer gauge updates serialize in exact queue mutation order; callers
// MUST NOT re-enter eventQueue from onDepth.
func (q *eventQueue) enqueue(
	ctx, procCtx context.Context,
	extraDone <-chan struct{},
	d EventDelivery,
	onBackpressure func(),
	onDepth func(int),
) enqueueOutcome {
	if outcome, canceled := checkEnqueueCancel(ctx, procCtx, extraDone); canceled {
		return outcome
	}

	notedBackpressure := false
	q.mu.Lock()
	for {
		if q.closed {
			q.mu.Unlock()
			return enqueueProcessClosed
		}
		if q.count < q.maxCap && len(q.waiters) == 0 {
			q.pushLocked(d)
			if onDepth != nil {
				onDepth(q.count)
			}
			q.mu.Unlock()
			return enqueueAdmitted
		}

		w := &eventQueueWaiter{ready: make(chan struct{}, 1)}
		q.waiters = append(q.waiters, w)
		q.mu.Unlock()

		if !notedBackpressure {
			notedBackpressure = true
			if onBackpressure != nil {
				onBackpressure()
			}
		}

		wokeReady := waitEnqueueSlot(ctx, procCtx, extraDone, w.ready)

		q.mu.Lock()
		if !wokeReady {
			if w.signaled {
				if !q.closed && q.count < q.maxCap {
					q.wakeNextWaiterLocked()
				}
			} else {
				q.removeWaiterLocked(w)
			}
			q.mu.Unlock()
			if ctx.Err() != nil {
				return enqueueCallerCanceled
			}
			return enqueueProcessClosed
		}

		if q.closed {
			q.mu.Unlock()
			return enqueueProcessClosed
		}
		if outcome, canceled := checkEnqueueCancel(ctx, procCtx, extraDone); canceled {
			if q.count < q.maxCap {
				q.wakeNextWaiterLocked()
			}
			q.mu.Unlock()
			return outcome
		}
		if q.count < q.maxCap {
			q.pushLocked(d)
			if onDepth != nil {
				onDepth(q.count)
			}
			if q.count < q.maxCap {
				q.wakeNextWaiterLocked()
			}
			q.mu.Unlock()
			return enqueueAdmitted
		}
		// Queue is still at or above maxCap (for example after a concurrent
		// setMaxCapacity reduction or another producer claiming a single freed
		// slot); re-enqueue as a waiter on the next iteration.
	}
}

// drainBatch blocks until at least one item is available or the queue is closed
// and empty, then drains up to maxBatch items in FIFO order into buf.
// Returns the populated slice and true, or buf[:0] and false once closed and empty.
// onDepth, when non-nil, runs under q.mu (same contract as enqueue).
func (q *eventQueue) drainBatch(buf []EventDelivery, maxBatch int, onDepth func(int)) ([]EventDelivery, bool) {
	if maxBatch < 1 {
		panic("BUG: plugin/process: eventQueue drainBatch maxBatch must be positive")
	}
	for {
		q.mu.Lock()
		if q.count > 0 {
			n := min(q.count, maxBatch)
			buf = buf[:0]
			bufLen := len(q.buf)
			for range n {
				buf = append(buf, q.buf[q.head])
				q.buf[q.head] = EventDelivery{}
				q.head++
				if q.head == bufLen {
					q.head = 0
				}
			}
			q.count -= n
			q.shrinkIfEmptyLocked()
			if onDepth != nil {
				onDepth(q.count)
			}
			if q.count > 0 {
				q.signalNotEmptyLocked()
			}
			if !q.closed && q.count < q.maxCap {
				q.wakeNextWaiterLocked()
			}
			q.mu.Unlock()
			return buf, true
		}
		if q.closed {
			q.mu.Unlock()
			return buf[:0], false
		}
		q.mu.Unlock()
		<-q.notEmpty
	}
}

// pop removes and returns the oldest queued item without blocking.
func (q *eventQueue) pop() (EventDelivery, bool) {
	q.mu.Lock()
	if q.count == 0 {
		q.mu.Unlock()
		return EventDelivery{}, false
	}
	item := q.buf[q.head]
	q.buf[q.head] = EventDelivery{}
	q.head++
	if q.head == len(q.buf) {
		q.head = 0
	}
	q.count--
	q.shrinkIfEmptyLocked()
	if q.count > 0 {
		q.signalNotEmptyLocked()
	}
	if !q.closed && q.count < q.maxCap {
		q.wakeNextWaiterLocked()
	}
	q.mu.Unlock()
	return item, true
}

// Close marks the queue closed, wakes the consumer, and unblocks all waiting
// producers. Safe for concurrent use and idempotent.
func (q *eventQueue) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	q.signalNotEmptyLocked()
	for i, w := range q.waiters {
		w.signaled = true
		select {
		case w.ready <- struct{}{}:
		default:
		}
		q.waiters[i] = nil
	}
	q.waiters = nil
	q.mu.Unlock()
}

// cleanupRemaining drains any buffered items left in the queue, notifies
// waiting callers with ErrConnectionClosed (or calls OnFailure), and recycles
// pooled StructuredEvent pointers so abnormal deliveryLoop exit never leaks.
// onDepth, when non-nil, runs under q.mu (same contract as enqueue).
func (q *eventQueue) cleanupRemaining(procName string, onDepth func(int)) {
	q.mu.Lock()
	if q.count == 0 {
		q.mu.Unlock()
		return
	}
	remaining := make([]EventDelivery, q.count)
	bufLen := len(q.buf)
	for i := range remaining {
		remaining[i] = q.buf[q.head]
		q.buf[q.head] = EventDelivery{}
		q.head++
		if q.head == bufLen {
			q.head = 0
		}
	}
	q.count = 0
	q.shrinkIfEmptyLocked()
	if onDepth != nil {
		onDepth(0)
	}
	q.mu.Unlock()

	for _, req := range remaining {
		if req.Result != nil {
			req.Result <- EventResult{
				ProcName: procName,
				Err:      ErrConnectionClosed,
			}
		} else if req.OnFailure != nil {
			req.OnFailure()
		}
		if se, ok := req.Event.(*rpc.StructuredEvent); ok {
			rpc.PutStructuredEvent(se)
		}
	}
}

// pushLocked appends d to the ring, growing buf by 2x up to maxCap when full.
// Caller MUST hold q.mu and ensure q.count < q.maxCap.
func (q *eventQueue) pushLocked(d EventDelivery) {
	if q.count == len(q.buf) {
		newCap := min(len(q.buf)*2, q.maxCap)
		newBuf := make([]EventDelivery, newCap)
		tailLen := len(q.buf) - q.head
		if tailLen >= q.count {
			copy(newBuf, q.buf[q.head:q.head+q.count])
		} else {
			copy(newBuf, q.buf[q.head:])
			copy(newBuf[tailLen:], q.buf[:q.count-tailLen])
		}
		q.buf = newBuf
		q.head = 0
	}
	idx := (q.head + q.count) % len(q.buf)
	q.buf[idx] = d
	q.count++
	q.signalNotEmptyLocked()
}

// shrinkIfEmptyLocked resets head and releases an oversized burst backing ring
// once the queue drains empty. Caller MUST hold q.mu.
func (q *eventQueue) shrinkIfEmptyLocked() {
	if q.count != 0 {
		return
	}
	q.head = 0
	if len(q.buf) > eventQueueShrinkThreshold {
		q.buf = make([]EventDelivery, min(eventQueueInitialCap, q.maxCap))
	}
}

// signalNotEmptyLocked wakes the consumer if it is waiting on notEmpty.
// Caller MUST hold q.mu.
func (q *eventQueue) signalNotEmptyLocked() {
	select {
	case q.notEmpty <- struct{}{}:
	default:
	}
}

// wakeNextWaiterLocked pops and signals the oldest waiting producer.
// Caller MUST hold q.mu.
func (q *eventQueue) wakeNextWaiterLocked() {
	if len(q.waiters) == 0 {
		return
	}
	w := q.waiters[0]
	copy(q.waiters, q.waiters[1:])
	q.waiters[len(q.waiters)-1] = nil
	q.waiters = q.waiters[:len(q.waiters)-1]
	w.signaled = true
	select {
	case w.ready <- struct{}{}:
	default:
	}
}

// removeWaiterLocked removes target from the waiter queue if still present.
// Caller MUST hold q.mu.
func (q *eventQueue) removeWaiterLocked(target *eventQueueWaiter) {
	for i, w := range q.waiters {
		if w == target {
			copy(q.waiters[i:], q.waiters[i+1:])
			q.waiters[len(q.waiters)-1] = nil
			q.waiters = q.waiters[:len(q.waiters)-1]
			return
		}
	}
}

// checkEnqueueCancel checks whether any cancellation signal is already active.
func checkEnqueueCancel(ctx, procCtx context.Context, extraDone <-chan struct{}) (enqueueOutcome, bool) {
	if ctx.Err() != nil {
		return enqueueCallerCanceled, true
	}
	if procCtx != nil && procCtx.Err() != nil {
		return enqueueProcessClosed, true
	}
	if extraDone != nil {
		select {
		case <-extraDone:
			return enqueueProcessClosed, true
		default:
		}
	}
	return enqueueOutcomeUnspecified, false
}

// waitEnqueueSlot waits for either a queue slot signal or cancellation.
// Calling procCtx.Done() here keeps admission select evaluation observable
// after the initial Err guard.
func waitEnqueueSlot(ctx, procCtx context.Context, extraDone, ready <-chan struct{}) bool {
	var procDone <-chan struct{}
	if procCtx != nil {
		procDone = procCtx.Done()
	}
	select {
	case <-ready:
		return true
	case <-ctx.Done():
		return false
	case <-procDone:
		return false
	case <-extraDone:
		return false
	}
}
