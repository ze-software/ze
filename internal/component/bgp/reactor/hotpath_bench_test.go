package reactor

import (
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// --- BufMux benchmarks ---

// BenchmarkBufMuxGetReturn measures the BufMux Get/Return cycle.
// This is the lock contention bottleneck for buffer pool access.
func BenchmarkBufMuxGetReturn(b *testing.B) {
	m := newBufMux(4096, 128)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h := m.Get()
		m.Return(h)
	}
}

// BenchmarkBufMuxGetReturnParallel measures BufMux Get/Return under contention
// from multiple goroutines competing for the same mutex.
func BenchmarkBufMuxGetReturnParallel(b *testing.B) {
	m := newBufMux(4096, 128)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h := m.Get()
			m.Return(h)
		}
	})
}

// --- RecentUpdateCache benchmarks ---

// benchPayload is a minimal valid UPDATE payload: WithdrawnLen(2)=0 + AttrLen(2)=0.
var benchPayload = []byte{0, 0, 0, 0}

// newBenchUpdate creates a ReceivedUpdate for benchmarks.
func newBenchUpdate(id uint64) *ReceivedUpdate {
	wu := wireu.NewWireUpdate(benchPayload, bgpctx.ContextID(1))
	wu.SetMessageID(id)
	return &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}
}

// BenchmarkCacheRetainRelease measures N sequential Retain+Release operations
// on the same cache entry. This simulates ForwardUpdate dispatching to N peers
// where each peer retains and releases the cached update.
func BenchmarkCacheRetainRelease(b *testing.B) {
	cache := NewRecentUpdateCache(0)
	cache.Start()
	defer cache.Stop()

	update := newBenchUpdate(1)
	cache.Add(update)
	cache.Activate(1, 1) // One plugin consumer to keep entry alive

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		cache.Retain(1)
		cache.Release(1)
	}
}

// BenchmarkCacheRetainReleaseParallel measures Retain+Release under parallel
// contention, simulating multiple ForwardUpdate calls from concurrent goroutines.
func BenchmarkCacheRetainReleaseParallel(b *testing.B) {
	cache := NewRecentUpdateCache(0)
	cache.Start()
	defer cache.Stop()

	update := newBenchUpdate(1)
	cache.Add(update)
	cache.Activate(1, 1) // One plugin consumer to keep entry alive

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Retain(1)
			cache.Release(1)
		}
	})
}

// BenchmarkCacheAddGetAck measures the full cache lifecycle for a single entry:
// Add, Get, Ack. Uses a unique message ID per iteration to avoid eviction conflicts.
func BenchmarkCacheAddGetAck(b *testing.B) {
	cache := NewRecentUpdateCache(0)
	cache.Start()
	defer cache.Stop()

	const consumer = "bench-plugin"
	cache.RegisterConsumer(consumer)

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		id := uint64(i + 1) // Unique IDs to avoid collision
		update := newBenchUpdate(id)
		cache.Add(update)
		cache.Activate(id, 1)

		got, ok := cache.Get(id)
		if !ok {
			b.Fatalf("cache.Get(%d) not found at iteration %d", id, i)
		}
		if got.WireUpdate.MessageID() != id {
			b.Fatalf("cache.Get(%d) returned wrong ID %d", id, got.WireUpdate.MessageID())
		}

		if err := cache.Ack(id, consumer); err != nil {
			b.Fatalf("unexpected ack error at iteration %d: %v", i, err)
		}
	}
}

// --- Forward pool benchmarks ---

// BenchmarkFwdPoolTryDispatch measures TryDispatch overhead with a no-op handler.
// Exercises: mutex lock/unlock, map lookup, non-blocking channel send.
func BenchmarkFwdPoolTryDispatch(b *testing.B) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {}, fwdPoolConfig{
		chanSize:    1024,
		idleTimeout: 10 * time.Second,
	})
	defer pool.Stop()

	key := fwdKey{peerAddr: "10.0.0.1"}

	// Warm up: ensure worker exists so we measure steady-state, not creation.
	pool.TryDispatch(key, fwdItem{})
	time.Sleep(10 * time.Millisecond) // Let worker drain

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pool.TryDispatch(key, fwdItem{})
	}
}

// BenchmarkFwdPoolTryDispatchParallel measures TryDispatch under contention
// from multiple goroutines dispatching to different destination peers.
func BenchmarkFwdPoolTryDispatchParallel(b *testing.B) {
	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {}, fwdPoolConfig{
		chanSize:    1024,
		idleTimeout: 10 * time.Second,
	})
	defer pool.Stop()

	// Pre-create keys for GOMAXPROCS goroutines to avoid map growth during benchmark.
	var keysMu sync.Mutex
	keys := make(map[int]fwdKey)
	nextID := 0

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets its own key (simulates different destination peers).
		keysMu.Lock()
		id := nextID
		nextID++
		keysMu.Unlock()

		key := fwdKey{peerAddr: fmt.Sprintf("10.0.%d.%d", id/256, id%256)}
		keysMu.Lock()
		keys[id] = key
		keysMu.Unlock()

		// Warm up worker.
		pool.TryDispatch(key, fwdItem{})
		time.Sleep(5 * time.Millisecond)

		for pb.Next() {
			pool.TryDispatch(key, fwdItem{})
		}
	})
}

// --- env.GetDuration benchmark ---

// BenchmarkEnvGetDuration measures the cost of env.GetDuration, which is called
// per-batch in fwdBatchHandler to read ze.fwd.write.deadline.
func BenchmarkEnvGetDuration(b *testing.B) {
	// env.Get checks the registration and cache, which is the hot path cost.
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		d := env.GetDuration("ze.fwd.write.deadline", 30*time.Second)
		if d <= 0 {
			b.Fatal("unexpected non-positive duration")
		}
	}
}

// --- publishBusNotification pattern benchmark ---

// BenchmarkPublishBusNotificationPattern measures the map allocation and
// String() conversion done per UPDATE in publishBusNotification calls.
// The reactor builds map[string]string metadata on every notification.
func BenchmarkPublishBusNotificationPattern(b *testing.B) {
	addr := netip.MustParseAddr("192.168.1.1")
	direction := "received"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		// This mirrors the exact allocation pattern in reactor_notify.go:
		// r.publishBusNotification("bgp/update", map[string]string{
		//     "peer": peerAddr.String(), "direction": direction,
		// })
		m := map[string]string{
			"peer":      addr.String(),
			"direction": direction,
		}
		if len(m) != 2 {
			b.Fatal("unexpected map size")
		}
	}
}

// --- Timer reset pattern benchmark ---

// BenchmarkTimerResetPattern measures the timer.Stop() + clock.AfterFunc() cycle
// that happens in resetSendHoldTimer on every successful write.
func BenchmarkTimerResetPattern(b *testing.B) {
	clk := clock.RealClock{}
	noop := func() {}
	timer := clk.AfterFunc(time.Hour, noop)
	defer timer.Stop()

	duration := 8 * time.Minute
	var mu sync.Mutex

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		mu.Lock()
		timer.Stop()
		timer = clk.AfterFunc(duration, noop)
		mu.Unlock()
	}
}
