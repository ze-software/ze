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

	proc.eventChan = make(chan EventDelivery, 4)
	proc.eventChan <- EventDelivery{Output: "a"}
	proc.eventChan <- EventDelivery{Output: "b"}

	first := EventDelivery{Output: "first"}

	// First call: buffer grows from nil.
	var buf []EventDelivery
	buf = proc.drainBatch(buf, first)

	if len(buf) != 3 {
		t.Fatalf("expected 3 items, got %d", len(buf))
	}
	firstPtr := unsafe.SliceData(buf)

	// Second call: reuse existing buffer.
	proc.eventChan <- EventDelivery{Output: "c"}
	first2 := EventDelivery{Output: "second"}
	buf = proc.drainBatch(buf, first2)

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
// PREVENTS: a barrier that returns while a command is still in the 64-deep
// channel, which is what let a caller read state the command had not reached.
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
	proc.eventChan = make(chan EventDelivery, eventDeliveryCapacity)
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
