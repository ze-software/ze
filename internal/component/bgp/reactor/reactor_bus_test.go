package reactor

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bus"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// VALIDATES: AC-2 — Bus publishes event matching registered prefix → handler called with event.
// PREVENTS: Reactor ignoring Bus events after subscription.
func TestReactorReceivesBusEvent(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("interface/addr/added"); err != nil {
		t.Fatal(err)
	}

	r := New(&Config{})
	r.SetBus(b)

	var received atomic.Value
	var wg sync.WaitGroup
	wg.Add(1)

	err := r.OnBusEvent("interface/", func(ev ze.Event) {
		received.Store(ev)
		wg.Done()
	})
	if err != nil {
		t.Fatal(err)
	}

	r.subscribeBus()
	defer r.unsubscribeBus()

	b.Publish("interface/addr/added", []byte(`{"address":"10.0.0.1"}`), map[string]string{
		"address": "10.0.0.1",
		"family":  "ipv4",
	})

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event delivery")
	}

	val := received.Load()
	if val == nil {
		t.Fatal("no event received")
	}
	ev, ok := val.(ze.Event)
	if !ok {
		t.Fatalf("unexpected type %T", val)
	}
	if ev.Topic != "interface/addr/added" {
		t.Errorf("topic = %q, want %q", ev.Topic, "interface/addr/added")
	}
	if ev.Metadata["address"] != "10.0.0.1" {
		t.Errorf("metadata[address] = %q, want %q", ev.Metadata["address"], "10.0.0.1")
	}
	if string(ev.Payload) != `{"address":"10.0.0.1"}` {
		t.Errorf("payload = %q, want %q", string(ev.Payload), `{"address":"10.0.0.1"}`)
	}
}

// VALIDATES: AC-3 — Bus publishes event NOT matching any prefix → no handler called.
// PREVENTS: Spurious handler invocations for unrelated topics.
func TestReactorNoMatchingHandler(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("bgp/update"); err != nil {
		t.Fatal(err)
	}

	r := New(&Config{})
	r.SetBus(b)

	var called atomic.Bool

	err := r.OnBusEvent("interface/", func(_ ze.Event) {
		called.Store(true)
	})
	if err != nil {
		t.Fatal(err)
	}

	r.subscribeBus()
	defer r.unsubscribeBus()

	// Publish to a topic that doesn't match "interface/" prefix.
	b.Publish("bgp/update", nil, nil)

	// Give delivery goroutine time to process.
	time.Sleep(100 * time.Millisecond)

	if called.Load() {
		t.Error("handler called for non-matching topic")
	}
}

// VALIDATES: AC-4 — Multiple handlers for overlapping prefixes → all called.
// PREVENTS: Only first matching handler being invoked.
func TestReactorMultipleHandlers(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("interface/addr/added"); err != nil {
		t.Fatal(err)
	}

	r := New(&Config{})
	r.SetBus(b)

	var count atomic.Int32

	// Two handlers with overlapping prefixes.
	if err := r.OnBusEvent("interface/", func(_ ze.Event) {
		count.Add(1)
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.OnBusEvent("interface/addr/", func(_ ze.Event) {
		count.Add(1)
	}); err != nil {
		t.Fatal(err)
	}

	r.subscribeBus()
	defer r.unsubscribeBus()

	b.Publish("interface/addr/added", nil, nil)

	// Both handlers are called synchronously within Deliver, which runs
	// in the Bus worker goroutine. Wait for delivery to complete.
	deadline := time.After(2 * time.Second)
	for count.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("timeout: handler count = %d, want 2", count.Load())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// VALIDATES: AC-5 — Reactor.Stop() unsubscribes, no further deliveries.
// PREVENTS: Events delivered after shutdown causing panics or stale state.
func TestReactorUnsubscribesOnStop(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("interface/addr/added"); err != nil {
		t.Fatal(err)
	}

	r := New(&Config{})
	r.SetBus(b)

	var count atomic.Int32

	if err := r.OnBusEvent("interface/", func(_ ze.Event) {
		count.Add(1)
	}); err != nil {
		t.Fatal(err)
	}

	r.subscribeBus()

	// Publish one event, wait for delivery.
	b.Publish("interface/addr/added", nil, nil)
	time.Sleep(100 * time.Millisecond)

	if count.Load() != 1 {
		t.Fatalf("expected 1 delivery before unsubscribe, got %d", count.Load())
	}

	// Unsubscribe.
	r.unsubscribeBus()

	// Publish again — should NOT be delivered.
	b.Publish("interface/addr/added", nil, nil)
	time.Sleep(100 * time.Millisecond)

	if count.Load() != 1 {
		t.Errorf("event delivered after unsubscribe: count = %d, want 1", count.Load())
	}
}

// VALIDATES: AC-6 — No handlers registered → no subscription, clean start/stop.
// PREVENTS: Nil pointer or unnecessary Bus subscription when no handlers exist.
func TestReactorNoHandlersNoop(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	r := New(&Config{})
	r.SetBus(b)

	// No OnBusEvent calls — subscribeBus should be a no-op.
	r.subscribeBus()
	r.unsubscribeBus()

	// The test passes if no panic occurs.
}

// VALIDATES: AC-7 — OnBusEvent after Start returns error.
// PREVENTS: Race condition from dynamic handler registration.
func TestReactorOnBusEventAfterStart(t *testing.T) {
	r := New(&Config{})

	// Mark as started.
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()

	err := r.OnBusEvent("interface/", func(_ ze.Event) {})
	if err == nil {
		t.Error("expected error registering handler after start, got nil")
	}

	// Reset for clean state.
	r.mu.Lock()
	r.running = false
	r.mu.Unlock()
}
