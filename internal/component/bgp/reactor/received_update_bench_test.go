package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// BenchmarkEBGPWireCacheHitParallel measures the cost of cache-hit reads of
// EBGPWire under parallel goroutine load. This is the hot path in RS fan-out:
// one UPDATE forwarded to N eBGP peers, each calling EBGPWire on the same
// ReceivedUpdate.
//
// Before the lock-free change: every hit takes ebgpMu (mutex lock/unlock pair).
// After: hit path is a single atomic pointer load.
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
