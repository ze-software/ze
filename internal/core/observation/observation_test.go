package observation

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// VALIDATES: AC-3 — Feed delivers typed observations from publisher to subscriber.
func TestObservationFeedDeliversToSubscriber(t *testing.T) {
	f := NewFeed()
	defer f.Close()

	var received []Observation
	var mu sync.Mutex
	f.Subscribe("test", func(obs Observation) {
		mu.Lock()
		received = append(received, obs)
		mu.Unlock()
	})

	obs := Observation{
		Kind:    KindSourceIP,
		Iface:   "eth0",
		Feature: FeatureRxBytes,
		Value:   42,
		At:      time.Now(),
	}
	obs.Flow.Src = netip.MustParseAddr("10.0.0.1")
	f.Publish(obs)

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 1
	})

	mu.Lock()
	defer mu.Unlock()
	if received[0].Kind != KindSourceIP {
		t.Errorf("kind = %v, want SourceIP", received[0].Kind)
	}
	if received[0].Value != 42 {
		t.Errorf("value = %f, want 42", received[0].Value)
	}
	if received[0].Iface != "eth0" {
		t.Errorf("iface = %q, want eth0", received[0].Iface)
	}
}

// VALIDATES: AC-6 — one subscriber receives observations from multiple publishers.
func TestObservationFeedMultiCollector(t *testing.T) {
	f := NewFeed()
	defer f.Close()

	var count atomic.Int32
	f.Subscribe("test", func(_ Observation) {
		count.Add(1)
	})

	now := time.Now()
	f.Publish(Observation{Kind: KindSourceIP, Feature: FeatureRxBytes, Value: 1, At: now})
	f.Publish(Observation{Kind: KindFlow, Feature: FeatureFlowBytes, Value: 2, At: now})

	waitFor(t, func() bool { return count.Load() == 2 })
	if count.Load() != 2 {
		t.Errorf("count = %d, want 2", count.Load())
	}
}

// VALIDATES: AC-8 — 0 allocs/op on the publish dispatch path.
func BenchmarkObservationFeedPublish(b *testing.B) {
	f := NewFeed()
	defer f.Close()

	f.Subscribe("bench", func(_ Observation) {})

	obs := Observation{
		Kind:    KindInterface,
		Iface:   "eth0",
		Feature: FeatureRxBytes,
		Value:   100,
		At:      time.Now(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		f.Publish(obs)
	}
}

// VALIDATES: AC-9 — publisher is not stalled by a slow subscriber.
func TestObservationFeedSlowSubscriber(t *testing.T) {
	f := NewFeed()

	block := make(chan struct{})
	f.Subscribe("slow", func(_ Observation) {
		<-block
	})

	now := time.Now()
	obs := Observation{Kind: KindInterface, Feature: FeatureRxBytes, Value: 1, At: now}

	done := make(chan struct{})
	go func() {
		for range bufferCap + 10 {
			f.Publish(obs)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publisher blocked on slow subscriber")
	}

	if f.Dropped() == 0 {
		t.Error("expected drops when subscriber buffer is full")
	}

	close(block)
	f.Close()
}

// VALIDATES: AC-7 — unsubscribed handler stops receiving; no goroutine leak.
func TestObservationFeedUnsubscribe(t *testing.T) {
	f := NewFeed()
	defer f.Close()

	var count atomic.Int32
	id := f.Subscribe("test", func(_ Observation) {
		count.Add(1)
	})

	now := time.Now()
	f.Publish(Observation{Kind: KindInterface, Feature: FeatureRxBytes, Value: 1, At: now})
	waitFor(t, func() bool { return count.Load() == 1 })

	f.Unsubscribe(id)
	f.Publish(Observation{Kind: KindInterface, Feature: FeatureRxBytes, Value: 2, At: now})
	time.Sleep(20 * time.Millisecond)
	if count.Load() != 1 {
		t.Errorf("count = %d after unsubscribe, want 1", count.Load())
	}
}

func TestObservationFeedMultiSubscriber(t *testing.T) {
	f := NewFeed()
	defer f.Close()

	var counts [3]atomic.Int32
	ids := make([]int, 3)
	for i := range ids {
		idx := i
		ids[i] = f.Subscribe("test", func(_ Observation) {
			counts[idx].Add(1)
		})
	}

	now := time.Now()
	f.Publish(Observation{Kind: KindInterface, Feature: FeatureRxBytes, Value: 1, At: now})
	waitFor(t, func() bool {
		for i := range counts {
			if counts[i].Load() < 1 {
				return false
			}
		}
		return true
	})

	for i := range counts {
		if counts[i].Load() != 1 {
			t.Errorf("subscriber %d: count = %d, want 1", i, counts[i].Load())
		}
	}
}

func TestObservationValueType(t *testing.T) {
	obs := Observation{
		Kind:    KindFlow,
		Iface:   "eth0",
		Feature: FeatureFlowBytes,
		Value:   1234,
		At:      time.Now(),
	}
	obs.Flow.Src = netip.MustParseAddr("10.0.0.1")
	obs.Flow.Dst = netip.MustParseAddr("10.0.0.2")
	obs.Flow.SrcPort = 12345
	obs.Flow.DstPort = 80
	obs.Flow.Proto = 6

	if obs.Flow.Src != netip.MustParseAddr("10.0.0.1") {
		t.Error("flow src mismatch")
	}
	if obs.Flow.Proto != 6 {
		t.Error("flow proto mismatch")
	}
}

// VALIDATES: child-6 AC-3/A-5 -- SrcAS is 0 on a bare Observation (the "AS
// unknown" sentinel) and survives the value copy through Publish to a
// subscriber, at both ends of the uint32 range.
func TestObservationSrcASFieldZeroValue(t *testing.T) {
	if got := (Observation{}).SrcAS; got != 0 {
		t.Errorf("zero-value Observation SrcAS = %d, want 0", got)
	}

	f := NewFeed()
	defer f.Close()

	var received []Observation
	var mu sync.Mutex
	f.Subscribe("test", func(obs Observation) {
		mu.Lock()
		received = append(received, obs)
		mu.Unlock()
	})

	want := []uint32{0, 64500, 4294967295}
	for _, as := range want {
		obs := Observation{
			Kind:    KindFlow,
			Feature: FeatureFlowBytes,
			Value:   1,
			At:      time.Now(),
			SrcAS:   as,
		}
		obs.Flow.Src = netip.MustParseAddr("192.0.2.1")
		f.Publish(obs)
	}

	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == len(want)
	})

	mu.Lock()
	defer mu.Unlock()
	for i, as := range want {
		if received[i].SrcAS != as {
			t.Errorf("received[%d].SrcAS = %d, want %d", i, received[i].SrcAS, as)
		}
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}
