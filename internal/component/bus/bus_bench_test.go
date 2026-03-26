package bus_test

import (
	"fmt"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bus"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// drainConsumer implements ze.Consumer and drains events as fast as possible.
// It never blocks, making it suitable for benchmarks that measure publish overhead.
type drainConsumer struct{}

func (drainConsumer) Deliver([]ze.Event) error { return nil }

// BenchmarkBusPublishNoSubscribers measures the fast path: publish with zero
// subscribers. This is the floor cost of Publish (RLock + scan empty slice).
func BenchmarkBusPublishNoSubscribers(b *testing.B) {
	bu := bus.NewBus()
	defer bu.Stop()

	if _, err := bu.CreateTopic("bgp/update"); err != nil {
		b.Fatalf("CreateTopic: %v", err)
	}

	payload := []byte("bench-payload")
	meta := map[string]string{"peer": "192.0.2.1", "direction": "received"}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		bu.Publish("bgp/update", payload, meta)
	}
}

// BenchmarkBusPublishOneSubscriber measures publish with a single subscriber,
// the common case for UPDATE forwarding to the RIB plugin.
func BenchmarkBusPublishOneSubscriber(b *testing.B) {
	bu := bus.NewBus()
	defer bu.Stop()

	if _, err := bu.CreateTopic("bgp/update"); err != nil {
		b.Fatalf("CreateTopic: %v", err)
	}
	if _, err := bu.Subscribe("bgp/update", nil, drainConsumer{}); err != nil {
		b.Fatalf("Subscribe: %v", err)
	}

	payload := []byte("bench-payload")
	meta := map[string]string{"peer": "192.0.2.1", "direction": "received"}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		bu.Publish("bgp/update", payload, meta)
	}
}

// BenchmarkBusPublishTenSubscribers measures fan-out cost with ten subscribers,
// simulating multiple plugins (RIB, route-server, graceful-restart, etc.)
// all consuming UPDATE events.
func BenchmarkBusPublishTenSubscribers(b *testing.B) {
	bu := bus.NewBus()
	defer bu.Stop()

	if _, err := bu.CreateTopic("bgp/update"); err != nil {
		b.Fatalf("CreateTopic: %v", err)
	}
	for range 10 {
		if _, err := bu.Subscribe("bgp/update", nil, drainConsumer{}); err != nil {
			b.Fatalf("Subscribe: %v", err)
		}
	}

	payload := []byte("bench-payload")
	meta := map[string]string{"peer": "192.0.2.1", "direction": "received"}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		bu.Publish("bgp/update", payload, meta)
	}
}

// BenchmarkBusPublishWithFilter measures publish when subscribers have metadata
// filters, which is the path used by peer-specific subscriptions.
func BenchmarkBusPublishWithFilter(b *testing.B) {
	bu := bus.NewBus()
	defer bu.Stop()

	if _, err := bu.CreateTopic("bgp/update"); err != nil {
		b.Fatalf("CreateTopic: %v", err)
	}
	// One matching filter, one non-matching filter.
	if _, err := bu.Subscribe("bgp/update", map[string]string{"peer": "192.0.2.1"}, drainConsumer{}); err != nil {
		b.Fatalf("Subscribe: %v", err)
	}
	if _, err := bu.Subscribe("bgp/update", map[string]string{"peer": "10.0.0.1"}, drainConsumer{}); err != nil {
		b.Fatalf("Subscribe: %v", err)
	}

	payload := []byte("bench-payload")
	meta := map[string]string{"peer": "192.0.2.1", "direction": "received"}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		bu.Publish("bgp/update", payload, meta)
	}
}

// BenchmarkBusPublishParallel measures contention when multiple goroutines
// publish concurrently, simulating multiple BGP peers sending UPDATEs.
func BenchmarkBusPublishParallel(b *testing.B) {
	bu := bus.NewBus()
	defer bu.Stop()

	if _, err := bu.CreateTopic("bgp/update"); err != nil {
		b.Fatalf("CreateTopic: %v", err)
	}
	if _, err := bu.Subscribe("bgp/update", nil, drainConsumer{}); err != nil {
		b.Fatalf("Subscribe: %v", err)
	}

	payload := []byte("bench-payload")
	meta := map[string]string{"peer": "192.0.2.1", "direction": "received"}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bu.Publish("bgp/update", payload, meta)
		}
	})
}

// BenchmarkMapAllocationPattern benchmarks the per-UPDATE metadata map
// allocation used in reactor_notify.go:
//
//	map[string]string{"peer": addr, "direction": dir}
//
// This map is allocated on every Publish call in the reactor. The benchmark
// measures the cost of this allocation pattern at various sizes to inform
// whether pooling or a fixed-field struct would be worthwhile.
func BenchmarkMapAllocationPattern(b *testing.B) {
	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "2-field-peer-direction",
			fn: func() {
				m := map[string]string{"peer": "192.0.2.1", "direction": "received"}
				_ = m
			},
		},
		{
			name: "3-field-peer-state-reason",
			fn: func() {
				m := map[string]string{"peer": "192.0.2.1", "state": "down", "reason": "hold-timer-expired"}
				_ = m
			},
		},
		{
			name: "1-field-peer-only",
			fn: func() {
				m := map[string]string{"peer": "192.0.2.1"}
				_ = m
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				tt.fn()
			}
		})
	}
}

// BenchmarkMapAllocationPooled benchmarks a sync.Pool-based approach for
// the metadata map, as a comparison point against the allocation pattern.
func BenchmarkMapAllocationPooled(b *testing.B) {
	pool := sync.Pool{
		New: func() any {
			return make(map[string]string, 2)
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		m, _ := pool.Get().(map[string]string)
		m["peer"] = "192.0.2.1"
		m["direction"] = "received"
		// Clear before returning to pool.
		delete(m, "peer")
		delete(m, "direction")
		pool.Put(m)
	}
}

// BenchmarkPublishEndToEnd measures the full per-UPDATE cost: allocate a
// metadata map, publish to the bus with one subscriber. This is the
// realistic hot path in reactor_notify.go.
func BenchmarkPublishEndToEnd(b *testing.B) {
	bu := bus.NewBus()
	defer bu.Stop()

	if _, err := bu.CreateTopic("bgp/update"); err != nil {
		b.Fatalf("CreateTopic: %v", err)
	}
	if _, err := bu.Subscribe("bgp/update", nil, drainConsumer{}); err != nil {
		b.Fatalf("Subscribe: %v", err)
	}

	payload := []byte("bench-payload")
	addr := "192.0.2.1"
	dir := "received"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		meta := map[string]string{"peer": addr, "direction": dir}
		bu.Publish("bgp/update", payload, meta)
	}
}

// BenchmarkPublishSubscriberScaling shows how publish cost scales with
// subscriber count, useful for capacity planning.
func BenchmarkPublishSubscriberScaling(b *testing.B) {
	for _, n := range []int{0, 1, 2, 5, 10, 20, 50} {
		b.Run(fmt.Sprintf("subscribers-%d", n), func(b *testing.B) {
			bu := bus.NewBus()
			defer bu.Stop()

			if _, err := bu.CreateTopic("bgp/update"); err != nil {
				b.Fatalf("CreateTopic: %v", err)
			}
			for range n {
				if _, err := bu.Subscribe("bgp/update", nil, drainConsumer{}); err != nil {
					b.Fatalf("Subscribe: %v", err)
				}
			}

			payload := []byte("bench-payload")
			meta := map[string]string{"peer": "192.0.2.1", "direction": "received"}

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				bu.Publish("bgp/update", payload, meta)
			}
		})
	}
}
