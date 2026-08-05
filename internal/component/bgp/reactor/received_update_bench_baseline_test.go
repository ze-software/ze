package reactor

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// errBenchSlotNotPrimed says the benchmark fixture was not primed. The
// pre-change code never reached this state, because it generated on a miss.
// The sentinel gets its own identity. A production error borrowed here would
// name the wrong cause in the failure line.
var errBenchSlotNotPrimed = errors.New("bench: ebgp slot not primed")

// ebgpWireMutexHit reproduces the PRE-CHANGE cache-hit path of EBGPWire as it
// stood at b5ad2cabe^. It takes ebgpMu with a deferred unlock, reads the cached
// variant, and returns it. Only the field read differs, because the old fields
// are gone. That read is not what the benchmark measures. The mutex is.
//
// It exists so the "before" number stays measurable after the old code is gone.
// A speedup claim whose baseline lives only in a deleted spec cannot be re-run.
// Keep it beside BenchmarkEBGPWireCacheHitParallel and compare the two.
func (u *ReceivedUpdate) ebgpWireMutexHit(dstASN4 bool) (*wireu.WireUpdate, error) {
	u.ebgpMu.Lock()
	defer u.ebgpMu.Unlock()

	if dstASN4 {
		if s := u.ebgpSlotASN4.Load(); s != nil {
			return s.wire, nil
		}
	} else {
		if s := u.ebgpSlotASN2.Load(); s != nil {
			return s.wire, nil
		}
	}
	return nil, errBenchSlotNotPrimed
}

// BenchmarkEBGPWireCacheHitParallelMutexBaseline is the before-change comparator
// for BenchmarkEBGPWireCacheHitParallel. Same fixture, same parallel shape.
func BenchmarkEBGPWireCacheHitParallelMutexBaseline(b *testing.B) {
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
			wire, err := update.ebgpWireMutexHit(true)
			if err != nil {
				b.Fatalf("mutex cache hit: %v", err)
			}
			if wire == nil {
				b.Fatal("mutex cache hit returned nil")
			}
		}
	})
}
