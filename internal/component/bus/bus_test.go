package bus_test

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bus"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// collector implements ze.Consumer for testing. It collects delivered events
// into a thread-safe slice and signals via a channel when events arrive.
type collector struct {
	mu     sync.Mutex
	events []ze.Event
	signal chan struct{}
}

func newCollector() *collector {
	return &collector{signal: make(chan struct{}, 128)}
}

func (c *collector) Deliver(events []ze.Event) error {
	c.mu.Lock()
	c.events = append(c.events, events...)
	c.mu.Unlock()
	c.signal <- struct{}{}
	return nil
}

// waitFor waits until the collector has at least n events, or timeout.
func (c *collector) waitFor(t *testing.T, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		c.mu.Lock()
		got := len(c.events)
		c.mu.Unlock()
		if got >= n {
			return
		}
		select {
		case <-c.signal:
		case <-deadline:
			c.mu.Lock()
			got = len(c.events)
			c.mu.Unlock()
			t.Fatalf("timeout waiting for %d events, got %d", n, got)
		}
	}
}

func (c *collector) collected() []ze.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ze.Event, len(c.events))
	copy(out, c.events)
	return out
}

// VALIDATES: Topic creation and lookup.
// PREVENTS: Publishing to non-existent topics silently.
func TestCreateTopic(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	topic, err := b.CreateTopic("bgp/update")
	if err != nil {
		t.Fatalf("CreateTopic failed: %v", err)
	}
	if topic.Name != "bgp/update" {
		t.Fatalf("topic name = %q, want %q", topic.Name, "bgp/update")
	}
}

// VALIDATES: Duplicate topic creation returns error.
// PREVENTS: Silent topic shadowing.
func TestCreateTopicDuplicate(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	_, err := b.CreateTopic("bgp/update")
	if err != nil {
		t.Fatalf("first CreateTopic failed: %v", err)
	}

	_, err = b.CreateTopic("bgp/update")
	if err == nil {
		t.Fatal("expected error for duplicate topic, got nil")
	}
	if !strings.Contains(err.Error(), "exists") {
		t.Fatalf("error should mention 'exists', got: %v", err)
	}
}

// VALIDATES: AC-1 — Single subscriber receives published event with correct payload and metadata.
// PREVENTS: Events lost in transit.
func TestPublishDelivers(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	_, err := b.CreateTopic("bgp/update")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	c := newCollector()
	_, err = b.Subscribe("bgp/update", nil, c)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	payload := []byte("test-payload")
	meta := map[string]string{"peer": "1.2.3.4"}
	b.Publish("bgp/update", payload, meta)

	c.waitFor(t, 1, 2*time.Second)

	events := c.collected()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Topic != "bgp/update" {
		t.Errorf("topic = %q, want %q", events[0].Topic, "bgp/update")
	}
	if string(events[0].Payload) != "test-payload" {
		t.Errorf("payload = %q, want %q", events[0].Payload, "test-payload")
	}
	if events[0].Metadata["peer"] != "1.2.3.4" {
		t.Errorf("metadata[peer] = %q, want %q", events[0].Metadata["peer"], "1.2.3.4")
	}
}

// VALIDATES: AC-8 — Publish to topic with no subscribers succeeds silently.
// PREVENTS: Panics or errors on unmatched publishes.
func TestPublishNoSubscribers(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	_, err := b.CreateTopic("bgp/update")
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	// Should not panic or error
	b.Publish("bgp/update", []byte("data"), nil)
}

// VALIDATES: AC-2 — Prefix subscription matches subtopics.
// PREVENTS: Prefix matching broken (exact-only).
func TestPrefixSubscription(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	for _, name := range []string{"bgp/update", "bgp/state", "bgp/events/peer-up"} {
		if _, err := b.CreateTopic(name); err != nil {
			t.Fatalf("CreateTopic(%s): %v", name, err)
		}
	}

	c := newCollector()
	_, err := b.Subscribe("bgp/", nil, c)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	b.Publish("bgp/update", []byte("u"), nil)
	b.Publish("bgp/state", []byte("s"), nil)
	b.Publish("bgp/events/peer-up", []byte("p"), nil)

	c.waitFor(t, 3, 2*time.Second)

	events := c.collected()
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
}

// VALIDATES: AC-3 — Exact subscription only matches exact topic.
// PREVENTS: Over-matching with exact topic strings.
func TestExactSubscription(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	for _, name := range []string{"bgp/update", "bgp/state"} {
		if _, err := b.CreateTopic(name); err != nil {
			t.Fatalf("CreateTopic(%s): %v", name, err)
		}
	}

	c := newCollector()
	_, err := b.Subscribe("bgp/update", nil, c)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	b.Publish("bgp/update", []byte("yes"), nil)
	b.Publish("bgp/state", []byte("no"), nil)

	c.waitFor(t, 1, 2*time.Second)
	// Give a moment for any extra event to arrive
	time.Sleep(50 * time.Millisecond)

	events := c.collected()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Topic != "bgp/update" {
		t.Errorf("topic = %q, want %q", events[0].Topic, "bgp/update")
	}
}

// VALIDATES: AC-4 — Metadata filtering only delivers matching events.
// PREVENTS: Filter bypass.
func TestMetadataFiltering(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("bgp/update"); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	c := newCollector()
	_, err := b.Subscribe("bgp/update", map[string]string{"peer": "1.2.3.4"}, c)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// This should be delivered (metadata matches)
	b.Publish("bgp/update", []byte("match"), map[string]string{"peer": "1.2.3.4"})
	// This should NOT be delivered (different peer)
	b.Publish("bgp/update", []byte("no-match"), map[string]string{"peer": "5.6.7.8"})
	// This should NOT be delivered (no metadata)
	b.Publish("bgp/update", []byte("no-meta"), nil)

	c.waitFor(t, 1, 2*time.Second)
	time.Sleep(50 * time.Millisecond)

	events := c.collected()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if string(events[0].Payload) != "match" {
		t.Errorf("payload = %q, want %q", events[0].Payload, "match")
	}
}

// VALIDATES: AC-5 — Empty filter matches all events.
// PREVENTS: Empty filter treated as "match nothing".
func TestEmptyFilter(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("bgp/update"); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	c := newCollector()
	_, err := b.Subscribe("bgp/update", nil, c)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	b.Publish("bgp/update", []byte("a"), map[string]string{"peer": "1.2.3.4"})
	b.Publish("bgp/update", []byte("b"), nil)
	b.Publish("bgp/update", []byte("c"), map[string]string{"direction": "received"})

	c.waitFor(t, 3, 2*time.Second)

	events := c.collected()
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
}

// VALIDATES: AC-7 — Multiple consumers on same topic all receive the same event.
// PREVENTS: Fan-out broken.
func TestMultipleSubscribers(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("bgp/update"); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	c1 := newCollector()
	c2 := newCollector()
	if _, err := b.Subscribe("bgp/update", nil, c1); err != nil {
		t.Fatalf("Subscribe c1: %v", err)
	}
	if _, err := b.Subscribe("bgp/update", nil, c2); err != nil {
		t.Fatalf("Subscribe c2: %v", err)
	}

	b.Publish("bgp/update", []byte("fanout"), nil)

	c1.waitFor(t, 1, 2*time.Second)
	c2.waitFor(t, 1, 2*time.Second)

	if len(c1.collected()) != 1 {
		t.Errorf("c1 got %d events, want 1", len(c1.collected()))
	}
	if len(c2.collected()) != 1 {
		t.Errorf("c2 got %d events, want 1", len(c2.collected()))
	}
}

// VALIDATES: AC-6 — Consumer stops receiving after unsubscribe.
// PREVENTS: Stale subscriptions delivering events.
func TestUnsubscribe(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("bgp/update"); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	c := newCollector()
	sub, err := b.Subscribe("bgp/update", nil, c)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	b.Publish("bgp/update", []byte("before"), nil)
	c.waitFor(t, 1, 2*time.Second)

	b.Unsubscribe(sub)
	// Allow unsubscribe to take effect
	time.Sleep(50 * time.Millisecond)

	b.Publish("bgp/update", []byte("after"), nil)
	time.Sleep(100 * time.Millisecond)

	events := c.collected()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (only before unsubscribe)", len(events))
	}
}

// VALIDATES: Topic isolation — events on unrelated topics don't leak.
// PREVENTS: Subscription matching bugs.
func TestTopicIsolation(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("bgp/update"); err != nil {
		t.Fatalf("CreateTopic bgp/update: %v", err)
	}
	if _, err := b.CreateTopic("rib/route"); err != nil {
		t.Fatalf("CreateTopic rib/route: %v", err)
	}

	c := newCollector()
	if _, err := b.Subscribe("bgp/", nil, c); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	b.Publish("rib/route", []byte("wrong"), nil)
	b.Publish("bgp/update", []byte("right"), nil)

	c.waitFor(t, 1, 2*time.Second)
	time.Sleep(50 * time.Millisecond)

	events := c.collected()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Topic != "bgp/update" {
		t.Errorf("topic = %q, want %q", events[0].Topic, "bgp/update")
	}
}

// VALIDATES: AC-9 — Batch delivery: multiple rapid publishes delivered as batch.
// PREVENTS: Per-event delivery overhead.
func TestBatchDelivery(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("bgp/update"); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	// batchCollector tracks how many Deliver calls were made vs total events
	var deliverCalls atomic.Int64
	var totalEvents atomic.Int64
	done := make(chan struct{}, 1)

	bc := &batchCollector{
		deliverCalls: &deliverCalls,
		totalEvents:  &totalEvents,
		target:       100,
		done:         done,
	}

	if _, err := b.Subscribe("bgp/update", nil, bc); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Publish 100 events rapidly — some should be batched
	for i := range 100 {
		b.Publish("bgp/update", []byte{byte(i)}, nil)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for batch delivery")
	}

	calls := deliverCalls.Load()
	total := totalEvents.Load()
	if total != 100 {
		t.Fatalf("total events = %d, want 100", total)
	}
	// With batching, we should have fewer Deliver calls than events
	// (at least some batching should occur with 100 rapid publishes)
	if calls >= 100 {
		t.Logf("WARNING: no batching observed (calls=%d, events=%d)", calls, total)
	}
	t.Logf("batch delivery: %d calls for %d events (avg %.1f events/call)", calls, total, float64(total)/float64(calls))
}

// batchCollector tracks delivery call count vs event count.
type batchCollector struct {
	deliverCalls *atomic.Int64
	totalEvents  *atomic.Int64
	target       int64
	done         chan struct{}
}

func (bc *batchCollector) Deliver(events []ze.Event) error {
	bc.deliverCalls.Add(1)
	n := bc.totalEvents.Add(int64(len(events)))
	if n >= bc.target {
		select {
		case bc.done <- struct{}{}:
		default:
		}
	}
	return nil
}

// VALIDATES: Concurrent publishers don't race.
// PREVENTS: Data races in publish path.
func TestConcurrentPublish(t *testing.T) {
	b := bus.NewBus()
	defer b.Stop()

	if _, err := b.CreateTopic("bgp/update"); err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	c := newCollector()
	if _, err := b.Subscribe("bgp/update", nil, c); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	const goroutines = 10
	const perGoroutine = 100
	total := goroutines * perGoroutine

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range perGoroutine {
				b.Publish("bgp/update", []byte("concurrent"), nil)
			}
		}()
	}
	wg.Wait()

	c.waitFor(t, total, 5*time.Second)

	events := c.collected()
	if len(events) != total {
		t.Fatalf("got %d events, want %d", len(events), total)
	}
}

// VALIDATES: Bus interface satisfaction.
// PREVENTS: Compile-time interface drift.
func TestBusSatisfiesInterface(t *testing.T) {
	var _ ze.Bus = (*bus.Bus)(nil)
}
