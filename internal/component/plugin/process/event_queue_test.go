// Design: docs/architecture/api/process-protocol.md -- bounded growable lossless FIFO event delivery queue
// Related: event_queue.go

package process

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const testEventQueueProcName = "test-proc"

func seqLabel(prefix string, n int) string {
	var tb textbuf.Buffer
	tb.Str(prefix).Int(int64(n))
	return tb.String()
}

// TestEventQueueGrowDrainShrinkFIFO verifies non-blocking burst growth up to
// 16,000 items, FIFO batch draining capped at 512, shrink-on-empty back to 64,
// and exact FIFO preservation across wrapped-ring growth.
//
// VALIDATES: AC-1 and AC-4 of spec-bus-observer-delivery-decoupling.md.
// PREVENTS: Burst drops, reordering during ring growth, oversized batches, or
// permanent retention of peak burst memory after drain.
func TestEventQueueGrowDrainShrinkFIFO(t *testing.T) {
	const burstSize = 16000

	q := newEventQueue(plugin.DefaultDeliveryQueueMax)
	require.Equal(t, eventQueueInitialCap, q.Cap())
	require.Equal(t, plugin.DefaultDeliveryQueueMax, q.MaxCap())

	var backpressures atomic.Int64
	lastDepth := -1
	onBackpressure := func() { backpressures.Add(1) }
	onDepth := func(depth int) { lastDepth = depth }

	for i := range burstSize {
		got := q.enqueue(t.Context(), t.Context(), nil, EventDelivery{Output: seqLabel("evt-", i)}, onBackpressure, onDepth)
		require.Equal(t, enqueueAdmitted, got)
	}
	require.Zero(t, backpressures.Load())
	require.Equal(t, burstSize, q.Len())
	require.Equal(t, burstSize, lastDepth)
	require.GreaterOrEqual(t, q.Cap(), 16384)

	var buf []EventDelivery
	seen := 0
	maxBatchSeen := 0
	batches := 0
	for seen < burstSize {
		var ok bool
		buf, ok = q.drainBatch(buf, eventDeliveryBatchMax, onDepth)
		require.True(t, ok)
		require.NotEmpty(t, buf)
		require.LessOrEqual(t, len(buf), eventDeliveryBatchMax)
		if len(buf) > maxBatchSeen {
			maxBatchSeen = len(buf)
		}
		batches++
		for _, item := range buf {
			require.Equal(t, seqLabel("evt-", seen), item.Output)
			seen++
		}
	}
	require.Equal(t, burstSize, seen)
	require.Equal(t, eventDeliveryBatchMax, maxBatchSeen)
	require.GreaterOrEqual(t, batches, burstSize/eventDeliveryBatchMax)
	require.Zero(t, q.Len())
	require.Equal(t, eventQueueInitialCap, q.Cap())
	require.Zero(t, lastDepth)

	// Wrap-around growth: advance head past zero, then force ring expansion
	// across the wrap boundary and verify exact FIFO order.
	wrapped := newEventQueue(256)
	for i := range 50 {
		require.Equal(t, enqueueAdmitted, wrapped.enqueue(t.Context(), t.Context(), nil, EventDelivery{Output: seqLabel("wrap-", i)}, nil, nil))
	}
	for i := range 30 {
		item, ok := wrapped.pop()
		require.True(t, ok)
		require.Equal(t, seqLabel("wrap-", i), item.Output)
	}
	for i := 50; i < 150; i++ {
		require.Equal(t, enqueueAdmitted, wrapped.enqueue(t.Context(), t.Context(), nil, EventDelivery{Output: seqLabel("wrap-", i)}, nil, nil))
	}
	require.Equal(t, 120, wrapped.Len())
	require.GreaterOrEqual(t, wrapped.Cap(), 128)
	for i := 30; i < 150; i++ {
		item, ok := wrapped.pop()
		require.True(t, ok)
		require.Equal(t, seqLabel("wrap-", i), item.Output)
	}
	require.Zero(t, wrapped.Len())
}

// TestEventQueueBackpressureAndCancel verifies lossless backpressure at the
// queue ceiling, prompt cancellation on caller ctx.Done() and procCtx.Done()
// without corrupting FIFO order, and race-free concurrent producer/consumer flow.
//
// VALIDATES: AC-2 of spec-bus-observer-delivery-decoupling.md.
// PREVENTS: Event drops at ceiling, hung waiters after cancellation, or lost
// wakeups under concurrent contention.
func TestEventQueueBackpressureAndCancel(t *testing.T) {
	const ceiling = 64

	q := newEventQueue(ceiling)
	for i := range ceiling {
		require.Equal(t, enqueueAdmitted, q.enqueue(t.Context(), t.Context(), nil, EventDelivery{Output: seqLabel("full-", i)}, nil, nil))
	}

	// Part 1: Caller context cancellation while blocked at ceiling.
	callerCtx, cancelCaller := context.WithCancel(t.Context())
	bpHit := make(chan struct{}, 1)
	outcomeCh := make(chan enqueueOutcome, 1)
	go func() {
		outcomeCh <- q.enqueue(callerCtx, t.Context(), nil, EventDelivery{Output: "canceled"}, func() {
			select {
			case bpHit <- struct{}{}:
			default:
			}
		}, nil)
	}()

	waitCtx, stopWait := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopWait()
	select {
	case <-bpHit:
	case <-waitCtx.Done():
		t.Fatal("blocked producer did not report backpressure")
	}
	cancelCaller()
	select {
	case got := <-outcomeCh:
		require.Equal(t, enqueueCallerCanceled, got)
	case <-waitCtx.Done():
		t.Fatal("blocked producer did not return after caller cancellation")
	}

	// Prior 64 items remain intact in exact FIFO order; popping one frees a slot.
	first, ok := q.pop()
	require.True(t, ok)
	require.Equal(t, "full-0", first.Output)
	require.Equal(t, enqueueAdmitted, q.enqueue(t.Context(), t.Context(), nil, EventDelivery{Output: "after-free"}, nil, nil))

	for i := 1; i < ceiling; i++ {
		item, popped := q.pop()
		require.True(t, popped)
		require.Equal(t, seqLabel("full-", i), item.Output)
	}
	last, ok := q.pop()
	require.True(t, ok)
	require.Equal(t, "after-free", last.Output)
	require.Zero(t, q.Len())

	// Part 2: Process context cancellation while blocked at ceiling.
	for i := range ceiling {
		require.Equal(t, enqueueAdmitted, q.enqueue(t.Context(), t.Context(), nil, EventDelivery{Output: seqLabel("proc-", i)}, nil, nil))
	}
	procCtx, cancelProc := context.WithCancel(t.Context())
	bpProcHit := make(chan struct{}, 1)
	procOutcomeCh := make(chan enqueueOutcome, 1)
	go func() {
		procOutcomeCh <- q.enqueue(t.Context(), procCtx, nil, EventDelivery{Output: "proc-canceled"}, func() {
			select {
			case bpProcHit <- struct{}{}:
			default:
			}
		}, nil)
	}()
	select {
	case <-bpProcHit:
	case <-waitCtx.Done():
		t.Fatal("procCtx-blocked producer did not report backpressure")
	}
	cancelProc()
	select {
	case got := <-procOutcomeCh:
		require.Equal(t, enqueueProcessClosed, got)
	case <-waitCtx.Done():
		t.Fatal("procCtx-blocked producer did not return after procCtx cancellation")
	}
	q.CleanupRemaining(testEventQueueProcName, nil)

	// Part 3: Concurrent producers, canceling waiters, and single consumer under -race.
	stressQ := newEventQueue(ceiling)
	const producers = 4
	const perProducer = 250
	const totalExpected = producers * perProducer

	var received atomic.Int64
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		var buf []EventDelivery
		for {
			var drained bool
			buf, drained = stressQ.drainBatch(buf, 32, nil)
			if !drained {
				return
			}
			received.Add(int64(len(buf)))
		}
	}()

	var wg sync.WaitGroup
	for p := range producers {
		wg.Go(func() {
			for i := range perProducer {
				got := stressQ.enqueue(t.Context(), t.Context(), nil, EventDelivery{Output: seqLabel("s-", p*perProducer+i)}, nil, nil)
				if got != enqueueAdmitted {
					t.Errorf("producer %d item %d: got %v, want admitted", p, i, got)
					return
				}
			}
		})
	}
	// Extra callers that cancel quickly to exercise baton passing under race.
	for range 2 {
		wg.Go(func() {
			for range 20 {
				cctx, ccancel := context.WithCancel(t.Context())
				ccancel()
				got := stressQ.enqueue(cctx, t.Context(), nil, EventDelivery{Output: "cancel-fast"}, nil, nil)
				if got != enqueueCallerCanceled {
					t.Errorf("pre-canceled enqueue got %v, want caller canceled", got)
					return
				}
			}
		})
	}

	wg.Wait()
	stressQ.Close()
	select {
	case <-consumerDone:
	case <-waitCtx.Done():
		t.Fatal("stress consumer did not exit after Close")
	}
	require.Equal(t, int64(totalExpected), received.Load())
	require.Zero(t, stressQ.Len())
}

// TestEventQueueCloseCleansUpPooledEvents verifies that closing a queue wakes
// blocked producers with enqueueProcessClosed and CleanupRemaining delivers
// ErrConnectionClosed, invokes OnFailure, resets depth to zero, and recycles
// pooled StructuredEvent pointers.
//
// VALIDATES: R-3 of spec-bus-observer-delivery-decoupling.md.
// PREVENTS: Leaked StructuredEvent pool entries or hung Result/producer waiters
// when a process stops with buffered events.
func TestEventQueueCloseCleansUpPooledEvents(t *testing.T) {
	q := newEventQueue(3)

	resCh := make(chan EventResult, 1)
	var onFailureCalled atomic.Bool

	se1 := rpc.GetStructuredEvent()
	se1.PeerAddress = "192.0.2.1"
	se2 := rpc.GetStructuredEvent()
	se2.PeerAddress = "192.0.2.2"

	require.Equal(t, enqueueAdmitted, q.enqueue(t.Context(), t.Context(), nil, EventDelivery{
		Event:  se1,
		Result: resCh,
	}, nil, nil))
	require.Equal(t, enqueueAdmitted, q.enqueue(t.Context(), t.Context(), nil, EventDelivery{
		Event:     se2,
		OnFailure: func() { onFailureCalled.Store(true) },
	}, nil, nil))
	require.Equal(t, enqueueAdmitted, q.enqueue(t.Context(), t.Context(), nil, EventDelivery{
		Output: "plain",
	}, nil, nil))

	bpHit := make(chan struct{}, 1)
	blockedOutcome := make(chan enqueueOutcome, 1)
	go func() {
		blockedOutcome <- q.enqueue(t.Context(), t.Context(), nil, EventDelivery{Output: "waiter"}, func() {
			select {
			case bpHit <- struct{}{}:
			default:
			}
		}, nil)
	}()

	waitCtx, stopWait := context.WithTimeout(t.Context(), 5*time.Second)
	defer stopWait()
	select {
	case <-bpHit:
	case <-waitCtx.Done():
		t.Fatal("waiter did not block at queue ceiling")
	}

	q.Close()
	select {
	case got := <-blockedOutcome:
		require.Equal(t, enqueueProcessClosed, got)
	case <-waitCtx.Done():
		t.Fatal("blocked waiter did not wake on Close")
	}

	depthSeen := -1
	q.CleanupRemaining(testEventQueueProcName, func(depth int) { depthSeen = depth })

	require.Zero(t, depthSeen)
	require.Zero(t, q.Len())
	require.True(t, onFailureCalled.Load())

	select {
	case res := <-resCh:
		require.Equal(t, testEventQueueProcName, res.ProcName)
		require.ErrorIs(t, res.Err, ErrConnectionClosed)
	default:
		t.Fatal("Result channel did not receive ErrConnectionClosed")
	}

	buf, ok := q.drainBatch(nil, eventDeliveryBatchMax, nil)
	require.False(t, ok)
	require.Empty(t, buf)
}
