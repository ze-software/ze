// Design: docs/architecture/core-design.md -- Forwarding Path (section 9): per-UPDATE in-process hot path throughput.
// Related: forward_update_test.go -- functional tests of the fast path.

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
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

	cache := newRecentUpdateCache(b.N + 100)
	defer cache.Stop()
	cache.RegisterConsumer("rs")
	cache.setConsumerUnordered("rs")

	payload := []byte{0, 0, 0, 0}
	for i := range b.N {
		id := uint64(i + 1) //nolint:gosec // bench loop
		wu := wireu.NewWireUpdate(payload, ctxID)
		wu.SetMessageID(id)
		cache.Add(&ReceivedUpdate{
			WireUpdate:   wu,
			SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
			ReceivedAt:   time.Now(),
		})
		cache.Activate(id, 1)
	}

	// Established source peer: the forward rail refuses an UPDATE whose source is
	// not one (errForwardNoSource), so without this the benchmark would measure
	// the refusal path instead of the forward it is named after.
	src := makeForwardSourcePeer(b, ctx, ctxID)

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
	peer.refreshForwardFacts()

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {}, fwdPoolConfig{chanSize: 4096, idleTimeout: time.Second})
	b.Cleanup(pool.Stop)

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			settings.PeerKey():       peer,
		},
		fwdPool: pool,
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

// BenchmarkForwardDirect_Batch measures the batch-resolve optimization:
// 8 update IDs dispatched to 32 peers in a single ForwardUpdatesDirect call.
// The peer-map walk and source-info resolve happen once per batch, not per ID.
func BenchmarkForwardDirect_Batch(b *testing.B) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	const batchSize = 8
	const peerCount = 32

	cache := newRecentUpdateCache(b.N*batchSize + 100)
	defer cache.Stop()
	cache.RegisterConsumer("rs")
	cache.setConsumerUnordered("rs")

	payload := []byte{0, 0, 0, 0}
	for i := range b.N * batchSize {
		id := uint64(i + 1) //nolint:gosec // bench loop
		wu := wireu.NewWireUpdate(payload, ctxID)
		wu.SetMessageID(id)
		cache.Add(&ReceivedUpdate{
			WireUpdate:   wu,
			SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
			ReceivedAt:   time.Now(),
		})
		cache.Activate(id, 1)
	}

	// See BenchmarkForwardDirect: without an established source peer every id
	// would take the errForwardNoSource refusal instead of the forward path.
	src := makeForwardSourcePeer(b, ctx, ctxID)

	peers := make(map[netip.AddrPort]*Peer, peerCount+1)
	peers[src.Settings().PeerKey()] = src
	dests := make([]netip.AddrPort, 0, peerCount)
	for i := range peerCount {
		addr := netip.AddrFrom4([4]byte{10, 0, 1, byte(i + 1)})
		s := &PeerSettings{
			Connection: ConnectionBoth,
			Address:    addr,
			LocalAS:    65000,
			PeerAS:     65000,
			RouterID:   uint32(0x01020300 + i + 1),
		}
		p := NewPeer(s)
		p.state.Store(int32(PeerStateEstablished))
		p.negotiated.Store(&NegotiatedCapabilities{
			families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
			ExtendedMessage: false,
		})
		p.sendCtx.Store(ctx)
		p.sendCtxID = ctxID
		p.refreshForwardFacts()
		peers[s.PeerKey()] = p
		dests = append(dests, netip.AddrPortFrom(addr, 0))
	}

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {}, fwdPoolConfig{chanSize: 4096, idleTimeout: time.Second})
	b.Cleanup(pool.Stop)

	r := &Reactor{

		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peers,
		fwdPool:         pool,
	}
	adapter := &reactorAPIAdapter{r: r}

	ids := make([]uint64, batchSize)

	b.ResetTimer()
	for i := range b.N {
		for j := range batchSize {
			ids[j] = uint64(i*batchSize + j + 1) //nolint:gosec // bench loop
		}
		_ = adapter.ForwardUpdatesDirect(ids, dests, "rs")
	}
}
