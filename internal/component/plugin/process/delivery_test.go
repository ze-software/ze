package process

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestProcessDrainBatchReusesBuffer verifies that drainBatch reuses the
// caller-provided buffer across calls — no new backing array allocation on the second call.
//
// VALIDATES: AC-3 from spec-alloc-1-batch-pooling.md
// PREVENTS: Per-burst slice allocations in per-process delivery goroutine.
func TestProcessDrainBatchReusesBuffer(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "test-drain-reuse"})
	proc.ctx, proc.cancel = context.WithCancel(context.Background())
	defer proc.cancel()

	proc.eventQueue = newEventQueue(4)
	for _, out := range []string{"first", "a", "b"} {
		if got := proc.eventQueue.enqueue(context.Background(), proc.ctx, nil, EventDelivery{Output: out}, nil, nil); got != enqueueAdmitted {
			t.Fatalf("enqueue %q: %v", out, got)
		}
	}

	// First call: buffer grows from nil.
	var buf []EventDelivery
	var ok bool
	buf, ok = proc.eventQueue.drainBatch(buf, eventDeliveryBatchMax, nil)
	if !ok {
		t.Fatal("expected first drainBatch to succeed")
	}

	if len(buf) != 3 {
		t.Fatalf("expected 3 items, got %d", len(buf))
	}
	firstPtr := unsafe.SliceData(buf)

	// Second call: reuse existing buffer.
	for _, out := range []string{"second", "c"} {
		if got := proc.eventQueue.enqueue(context.Background(), proc.ctx, nil, EventDelivery{Output: out}, nil, nil); got != enqueueAdmitted {
			t.Fatalf("enqueue %q: %v", out, got)
		}
	}
	buf, ok = proc.eventQueue.drainBatch(buf, eventDeliveryBatchMax, nil)
	if !ok {
		t.Fatal("expected second drainBatch to succeed")
	}

	if len(buf) != 2 {
		t.Fatalf("expected 2 items, got %d", len(buf))
	}
	secondPtr := unsafe.SliceData(buf)

	if firstPtr != secondPtr {
		t.Error("second call allocated a new backing array instead of reusing buffer")
	}
}

// TestDeliverBatchReusesEventsSlice verifies that deliverBatch reuses the
// caller-provided eventsBuf across calls — no new backing array allocation.
//
// VALIDATES: AC-4 from spec-alloc-1-batch-pooling.md
// PREVENTS: Per-batch string slice allocations in delivery pipeline.
func TestDeliverBatchReusesEventsSlice(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "test-events-reuse"})
	proc.ctx, proc.cancel = context.WithCancel(context.Background())
	defer proc.cancel()

	// No bridge or conn set — deliverBatch will error with "connection closed",
	// but the eventsBuf slice is still constructed and returned.
	batch1 := []EventDelivery{
		{Output: "event-1"},
		{Output: "event-2"},
		{Output: "event-3"},
	}

	// First call: eventsBuf grows from nil.
	var eventsBuf []string
	eventsBuf = proc.deliverBatch(batch1, eventsBuf, defaultDeliveryTimeout)

	if len(eventsBuf) != 3 {
		t.Fatalf("expected 3 events, got %d", len(eventsBuf))
	}
	firstPtr := unsafe.SliceData(eventsBuf)

	// Second call: reuse existing eventsBuf.
	batch2 := []EventDelivery{
		{Output: "event-a"},
		{Output: "event-b"},
	}
	eventsBuf = proc.deliverBatch(batch2, eventsBuf, defaultDeliveryTimeout)

	if len(eventsBuf) != 2 {
		t.Fatalf("expected 2 events, got %d", len(eventsBuf))
	}
	secondPtr := unsafe.SliceData(eventsBuf)

	if firstPtr != secondPtr {
		t.Error("second call allocated a new backing array instead of reusing eventsBuf")
	}
}

// TestSafeBridgeCallRecoversPanic verifies that safeBridgeCall catches panics
// from DirectBridge handlers and returns them as errors.
//
// VALIDATES: H1 — DirectBridge panic does not crash delivery loop.
// PREVENTS: Internal plugin panic propagating to engine event loop.
func TestSafeBridgeCallRecoversPanic(t *testing.T) {
	err := safeBridgeCall(func() error {
		panic("plugin handler exploded")
	})
	if err == nil {
		t.Fatal("expected error from panicking bridge call, got nil")
		return
	}
	if !strings.Contains(err.Error(), "plugin panic") {
		t.Errorf("error should mention 'plugin panic', got: %v", err)
	}
}

// TestSafeBridgeCallPassesError verifies that safeBridgeCall passes through
// normal errors without interference.
//
// VALIDATES: H1 — normal errors unaffected by panic recovery.
// PREVENTS: Panic recovery swallowing legitimate errors.
func TestSafeBridgeCallPassesError(t *testing.T) {
	want := errors.New("normal failure")
	got := safeBridgeCall(func() error {
		return want
	})
	if !errors.Is(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestSafeBridgeCallSuccess verifies that safeBridgeCall returns nil on success.
func TestSafeBridgeCallSuccess(t *testing.T) {
	err := safeBridgeCall(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestDeliverBatchAnswersBarrierAndSendsNothing verifies that a Barrier item is
// answered like any other delivery and reaches no plugin, and that a batch of
// barriers alone sends nothing at all.
//
// VALIDATES: the barrier `request quiesce` waits on (quiesce.go,
// plugin-event-delivery). The channel is FIFO and every Result in a processed
// batch is answered, so an answered barrier means the items ahead of it were
// delivered.
// PREVENTS: a barrier reaching a plugin as an event. An empty delivery is a
// bare newline to a text consumer and a call to a bridge consumer, so a barrier
// that was carried would be indistinguishable from a real one that lost its
// payload.
func TestDeliverBatchAnswersBarrierAndSendsNothing(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "test-barrier"})
	proc.ctx, proc.cancel = context.WithCancel(context.Background())
	defer proc.cancel()

	var mu sync.Mutex
	var delivered []string
	bridge := rpc.NewDirectBridge()
	bridge.SetDeliverEvents(func(events []string) error {
		mu.Lock()
		defer mu.Unlock()
		delivered = append(delivered, events...)
		return nil
	})
	bridge.SetReady()
	proc.bridge = bridge

	real1 := make(chan EventResult, 1)
	barrier := make(chan EventResult, 1)
	batch := []EventDelivery{
		{Output: "route-one", Result: real1},
		{Barrier: true, Result: barrier},
	}
	proc.deliverBatch(batch, nil, defaultDeliveryTimeout)

	mu.Lock()
	seen := append([]string(nil), delivered...)
	mu.Unlock()
	if len(seen) != 1 || seen[0] != "route-one" {
		t.Errorf("the plugin received %v, want only the real event", seen)
	}
	if len(real1) != 1 {
		t.Error("the real event was not answered")
	}
	if len(barrier) != 1 {
		t.Error("the barrier was not answered, so a drain waiting on it would hang")
	}

	// A batch of barriers alone must not reach the plugin at all.
	mu.Lock()
	delivered = delivered[:0]
	mu.Unlock()
	only := make(chan EventResult, 1)
	proc.deliverBatch([]EventDelivery{{Barrier: true, Result: only}}, nil, defaultDeliveryTimeout)
	mu.Lock()
	after := append([]string(nil), delivered...)
	mu.Unlock()
	if len(after) != 0 {
		t.Errorf("a barrier-only batch delivered %v, want nothing", after)
	}
	if len(only) != 1 {
		t.Error("the barrier-only batch was not answered")
	}
}

// TestDrainEventsReturnsWhenTheRailIsEmpty verifies that DrainEvents answers
// only after the events queued before it have been delivered, and that it
// answers at once for a process whose rail is closed.
//
// VALIDATES: the inbound half of `request quiesce`.
// PREVENTS: a barrier that returns while a command is still in the delivery
// queue, which is what let a caller read state the command had not reached.
func TestDrainEventsReturnsWhenTheRailIsEmpty(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: "test-drain"})
	proc.ctx, proc.cancel = context.WithCancel(context.Background())
	defer proc.cancel()

	var mu sync.Mutex
	var delivered []string
	bridge := rpc.NewDirectBridge()
	bridge.SetDeliverEvents(func(events []string) error {
		mu.Lock()
		defer mu.Unlock()
		delivered = append(delivered, events...)
		return nil
	})
	bridge.SetReady()
	proc.bridge = bridge
	proc.eventQueue = newEventQueue(plugin.DefaultDeliveryQueueMax)
	go proc.deliveryLoop()

	for i := range 8 {
		proc.Deliver(EventDelivery{Output: "event-" + string(rune('a'+i))})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := proc.DrainEvents(ctx); err != nil {
		t.Fatalf("DrainEvents: %v", err)
	}
	mu.Lock()
	count := len(delivered)
	mu.Unlock()
	if count != 8 {
		t.Errorf("drain returned with %d of 8 events delivered", count)
	}

	// A closed rail owes nothing: a stopped plugin must not fail the drain.
	proc.stopEventChan()
	if err := proc.DrainEvents(ctx); err != nil {
		t.Errorf("a closed rail reported a drain failure: %v", err)
	}
}

// TestProcessCacheConsumerDefaultQueueBound verifies the role-based default
// delivery queue ceiling (65536 for non-cache-consumer endpoints, 256 for
// CacheConsumer plugins) and explicit DeliveryQueueMax precedence.
//
// VALIDATES: AC-3 of spec-bus-observer-delivery-decoupling.
// PREVENTS: Cache-consumer forwarding plugins losing tight backpressure or
// explicit operator delivery-queue-max being overwritten by SetCacheConsumer.
func TestProcessCacheConsumerDefaultQueueBound(t *testing.T) {
	stopAndWait := func(t *testing.T, p *Process) {
		t.Helper()
		t.Cleanup(func() {
			p.Stop()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.Wait(ctx); err != nil {
				t.Errorf("wait %s: %v", p.Name(), err)
			}
		})
	}

	t.Run("ordinary-default", func(t *testing.T) {
		proc := NewProcess(plugin.PluginConfig{Name: "ordinary"})
		proc.StartDelivery(t.Context())
		stopAndWait(t, proc)

		if got := proc.eventQueue.maxCapacity(); got != plugin.DefaultDeliveryQueueMax {
			t.Fatalf("ordinary default maxCapacity = %d, want %d", got, plugin.DefaultDeliveryQueueMax)
		}
	})

	t.Run("cache-consumer-before-start", func(t *testing.T) {
		proc := NewProcess(plugin.PluginConfig{Name: "cc-before"})
		proc.SetCacheConsumer(true)
		proc.StartDelivery(t.Context())
		stopAndWait(t, proc)

		if got := proc.eventQueue.maxCapacity(); got != plugin.DefaultCacheConsumerQueueMax {
			t.Fatalf("cache-consumer pre-start maxCapacity = %d, want %d", got, plugin.DefaultCacheConsumerQueueMax)
		}
	})

	t.Run("cache-consumer-after-start", func(t *testing.T) {
		proc := NewProcess(plugin.PluginConfig{Name: "cc-after"})
		proc.StartDelivery(t.Context())
		stopAndWait(t, proc)

		if got := proc.eventQueue.maxCapacity(); got != plugin.DefaultDeliveryQueueMax {
			t.Fatalf("initial maxCapacity = %d, want %d", got, plugin.DefaultDeliveryQueueMax)
		}
		proc.SetCacheConsumer(true)
		if got := proc.eventQueue.maxCapacity(); got != plugin.DefaultCacheConsumerQueueMax {
			t.Fatalf("after SetCacheConsumer(true) maxCapacity = %d, want %d", got, plugin.DefaultCacheConsumerQueueMax)
		}
		proc.SetCacheConsumer(false)
		if got := proc.eventQueue.maxCapacity(); got != plugin.DefaultDeliveryQueueMax {
			t.Fatalf("after SetCacheConsumer(false) maxCapacity = %d, want %d", got, plugin.DefaultDeliveryQueueMax)
		}
	})

	t.Run("explicit-override-wins", func(t *testing.T) {
		proc := NewProcess(plugin.PluginConfig{Name: "cc-override", DeliveryQueueMax: 1024})
		proc.SetCacheConsumer(true)
		proc.StartDelivery(t.Context())
		stopAndWait(t, proc)

		if got := proc.eventQueue.maxCapacity(); got != 1024 {
			t.Fatalf("pre-start override maxCapacity = %d, want 1024", got)
		}
		proc.SetCacheConsumer(false)
		if got := proc.eventQueue.maxCapacity(); got != 1024 {
			t.Fatalf("after toggle override maxCapacity = %d, want 1024", got)
		}
		proc.SetCacheConsumer(true)
		if got := proc.eventQueue.maxCapacity(); got != 1024 {
			t.Fatalf("after re-enable override maxCapacity = %d, want 1024", got)
		}
	})
}

// TestProcessBurstNonBlockingFIFO512Batch verifies that a non-cache-consumer
// Process absorbs a 16,000-event burst while its consumer is paused without
// blocking Deliver, then delivers every event in exact FIFO order in batches
// capped at 512 and shrinks the ring back to 64 once drained.
//
// VALIDATES: AC-1 and AC-4 of spec-bus-observer-delivery-decoupling.
// PREVENTS: Producer stalls during full-table bursts, dropped or reordered
// events, batches exceeding 512, or retained burst memory after drain.
func TestProcessBurstNonBlockingFIFO512Batch(t *testing.T) {
	const burstSize = 16000

	proc := NewProcess(plugin.PluginConfig{Name: "test-burst-fifo"})
	entered := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	var mu sync.Mutex
	delivered := make([]string, 0, burstSize+1)
	var maxBatchSeen int

	bridge := rpc.NewDirectBridge()
	bridge.SetDeliverEvents(func(events []string) error {
		first.Do(func() {
			close(entered)
			<-release
		})
		mu.Lock()
		if len(events) > maxBatchSeen {
			maxBatchSeen = len(events)
		}
		delivered = append(delivered, events...)
		mu.Unlock()
		return nil
	})
	bridge.SetReady()
	proc.bridge = bridge
	proc.StartDelivery(t.Context())
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proc.Wait(ctx); err != nil {
			t.Errorf("wait: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// Pause deliveryLoop inside DeliverEvents on the first event.
	if !proc.Deliver(EventDelivery{Output: "trigger"}) {
		t.Fatal("trigger event not admitted")
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("consumer did not enter DeliverEvents")
	}

	for i := range burstSize {
		if !proc.Deliver(EventDelivery{Output: seqLabel("burst-", i)}) {
			t.Fatalf("burst event %d was not admitted", i)
		}
	}
	if got := proc.eventQueue.Len(); got != burstSize {
		t.Fatalf("queued depth = %d, want %d", got, burstSize)
	}
	if got := proc.eventQueue.Cap(); got < 16384 {
		t.Fatalf("grown cap = %d, want >= 16384", got)
	}

	close(release)
	if err := proc.DrainEvents(ctx); err != nil {
		t.Fatalf("DrainEvents: %v", err)
	}

	mu.Lock()
	gotDelivered := append([]string(nil), delivered...)
	gotMaxBatch := maxBatchSeen
	mu.Unlock()

	if len(gotDelivered) != burstSize+1 {
		t.Fatalf("delivered %d events, want %d", len(gotDelivered), burstSize+1)
	}
	if gotDelivered[0] != "trigger" {
		t.Fatalf("first delivered event = %q, want trigger", gotDelivered[0])
	}
	for i := range burstSize {
		want := seqLabel("burst-", i)
		if gotDelivered[i+1] != want {
			t.Fatalf("event %d = %q, want %q", i, gotDelivered[i+1], want)
		}
	}
	if gotMaxBatch > eventDeliveryBatchMax {
		t.Fatalf("max batch size = %d, exceeds cap %d", gotMaxBatch, eventDeliveryBatchMax)
	}
	if gotMaxBatch != eventDeliveryBatchMax {
		t.Fatalf("max batch size = %d, want %d during 16k burst drain", gotMaxBatch, eventDeliveryBatchMax)
	}
	if got := proc.eventQueue.Len(); got != 0 {
		t.Fatalf("post-drain len = %d, want 0", got)
	}
	if got := proc.eventQueue.Cap(); got != eventQueueInitialCap {
		t.Fatalf("post-drain cap = %d, want %d", got, eventQueueInitialCap)
	}
}
