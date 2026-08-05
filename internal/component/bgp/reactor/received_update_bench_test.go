package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// BenchmarkEBGPWireCacheHitParallel measures the cost of cache-hit reads of
// EBGPWire under parallel goroutine load: many goroutines calling EBGPWire on
// one ReceivedUpdate.
//
// Before the lock-free change: every hit takes ebgpMu (mutex lock/unlock pair).
// After: hit path is a single atomic pointer load.
// BenchmarkEBGPWireCacheHitParallelMutexBaseline is the before comparator.
//
// EBGPWire has no production caller since the AS-path fold (e2037e598), so this
// measures a path a running daemon does not take. It stays until the cache is
// deleted (plan/spec-wire-edit-3-deferred-ac9-dead-code.md).
func BenchmarkEBGPWireCacheHitParallel(b *testing.B) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := testUpdatePayloadWithASPath([]uint32{64512, 64513})
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(nextMsgID())

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		poolBuf:      BufHandle{ID: noPoolBufID, Buf: make([]byte, 4096)},
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	if _, err := update.EBGPWire(65000, true, true); err != nil {
		b.Fatalf("prime EBGPWire: %v", err)
	}
	// Nothing evicts this fixture, so the benchmark owns the primed handle and
	// returns it itself. Left borrowed, it shifts the shared bufMuxStd counter
	// that the pool-accounting tests in this package read.
	defer func() {
		if s := update.ebgpSlotASN4.Load(); s != nil {
			ReturnReadBuffer(s.handle)
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wire, err := update.EBGPWire(65000, true, true)
			if err != nil {
				b.Fatalf("EBGPWire cache hit: %v", err)
			}
			if wire == nil {
				b.Fatal("EBGPWire returned nil on cache hit")
			}
		}
	})
}
