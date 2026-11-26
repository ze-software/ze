// Design: docs/architecture/api/process-protocol.md -- bounded FIFO admission.
package process

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// deliveryAdmissionContext observes the Process lifetime operand of the real
// admission select. It remains observable when the caller cancellation arm is
// removed, and does not cancel either context or make queue space available.
type deliveryAdmissionContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (ctx *deliveryAdmissionContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.entered) })
	return ctx.Context.Done()
}

// A live caller cancels after real full-FIFO admission starts, while the prior
// queued item and the next useful delivery retain their order.
// MUTATION: remove only DeliverContext's caller ctx.Done select arm. Admission
// is still observed, but caller cancellation cannot release the full FIFO.
func TestDeliveryContextCancelRetainsFIFO(t *testing.T) {
	processCtx, stopProcess := context.WithCancel(t.Context())
	admission := &deliveryAdmissionContext{Context: processCtx, entered: make(chan struct{})}
	p := &Process{ctx: admission, eventQueue: newEventQueue(1)}
	require.Equal(t, enqueueAdmitted, p.eventQueue.enqueue(t.Context(), processCtx, nil, EventDelivery{Output: "previous"}, nil, nil))
	alreadyCanceled, cancelAlready := context.WithCancel(t.Context())
	cancelAlready()
	require.False(t, p.DeliverContext(alreadyCanceled, EventDelivery{Output: "already canceled"}))

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	wait, stopWait := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(stopWait)
	result := make(chan bool, 1)
	joined := false
	// The test MUST release the Process context and join this helper, including
	// when the caller cancellation arm is deliberately cut from production.
	t.Cleanup(func() {
		stopProcess()
		if joined {
			return
		}
		cleanup, stopCleanup := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCleanup()
		select {
		case <-result:
		case <-cleanup.Done():
			t.Error("delivery helper did not join after Process cancellation", cleanup.Err())
		}
	})
	go func() { result <- p.DeliverContext(ctx, EventDelivery{Output: "canceled"}) }()
	select {
	case <-admission.entered:
	case <-wait.Done():
		t.Fatal("live delivery did not enter FIFO admission", wait.Err())
	}
	// Select operands are evaluated only after the initial Err guard. The
	// queue is still full and both contexts are live, so admission must wait.
	require.NoError(t, ctx.Err())
	require.NoError(t, processCtx.Err())
	require.Equal(t, 1, p.eventQueue.Len())
	cancel()
	select {
	case delivered := <-result:
		joined = true
		require.False(t, delivered, "caller cancellation must refuse blocked admission")
	case <-wait.Done():
		t.Fatal("blocked FIFO admission did not return after caller cancellation", wait.Err())
	}
	prev, ok := p.eventQueue.pop()
	require.True(t, ok)
	require.Equal(t, "previous", prev.Output)
	require.True(t, p.DeliverContext(t.Context(), EventDelivery{Output: "next"}))
	next, ok := p.eventQueue.pop()
	require.True(t, ok)
	require.Equal(t, "next", next.Output)
}
