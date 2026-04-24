// Design: rs-fastpath-3 AC-9 -- per-UPDATE in-process hot path throughput.
// Related: forward_update_test.go -- functional tests of the fast path.

package reactor

import (
	"net/netip"
	"testing"
	"time"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// BenchmarkForwardDirect measures the per-UPDATE cost of the rs-fastpath-3
// ForwardUpdatesDirect path: cache lookup + destination resolve + shared
// per-destination loop + dispatch to the forward pool.
//
// AC-9 target: >= 500k UPDATE/s/core on the in-process hot path against the
// Phase 1 profile baseline. Not a CI gate; run on demand:
//
//	go test -run=^$ -bench=BenchmarkForwardDirect ./internal/component/bgp/reactor/...
func BenchmarkForwardDirect(b *testing.B) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	cache := NewRecentUpdateCache(b.N + 100)
	defer cache.Stop()
	cache.RegisterConsumer("rs")
	cache.SetConsumerUnordered("rs")

	payload := []byte{0, 0, 0, 0}
	for i := range b.N {
		id := uint64(i + 1) //nolint:gosec // bench loop
		wu := wireu.NewWireUpdate(payload, ctxID)
		wu.SetMessageID(id)
		cache.Add(&ReceivedUpdate{
			WireUpdate:   wu,
			SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
			ReceivedAt:   time.Now(),
		})
		cache.Activate(id, 1)
	}

	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65000,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {}, fwdPoolConfig{chanSize: 4096, idleTimeout: time.Second})
	b.Cleanup(pool.Stop)

	r := &Reactor{
		recentUpdates: cache,
		peers:         map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
		fwdPool:       pool,
	}
	adapter := &reactorAPIAdapter{r: r}

	dests := []netip.AddrPort{netip.AddrPortFrom(settings.Address, 0)}
	ids := make([]uint64, 1)

	b.ResetTimer()
	for i := range b.N {
		ids[0] = uint64(i + 1) //nolint:gosec // bench loop
		_ = adapter.ForwardUpdatesDirect(ids, dests, "rs")
	}
}
