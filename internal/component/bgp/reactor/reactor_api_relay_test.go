package reactor

import (
	"encoding/hex"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// storedIPv4Route is the stored form of the source frame in
// test/plugin/remove-private-as-replace-peer.ci: ORIGIN, AS_PATH
// [64496 64512 64497], NEXT_HOP 1.1.1.1, NLRI 10.0.0.0/24.
func storedIPv4Route(sourcePeer string) rpc.StoredRoute {
	return rpc.StoredRoute{
		SourcePeer: sourcePeer,
		Family:     "ipv4/unicast",
		AttrHex:    "4001010040020E02030000FBF00000FC000000FBF140030401010101",
		NextHopHex: "01010101",
		NLRIHex:    "180A0000",
	}
}

// relayFixture builds a reactor with an established source and destination peer
// plus a capturing forward pool, and returns the dispatch observation channel.
func relayFixture(t *testing.T) (*reactorAPIAdapter, *RecentUpdateCache, *[]fwdItem, *sync.Mutex, chan struct{}) {
	t.Helper()
	return relayFixtureAS(t, 65001, 65002)
}

// relayFixtureAS is relayFixture with the peer AS numbers chosen by the caller,
// so a test can build the IBGP-to-IBGP pair whose RFC 4456 reflection rules make
// forwardUpdateCore suppress the destination.
func relayFixtureAS(t *testing.T, srcAS, dstAS uint32) (*reactorAPIAdapter, *RecentUpdateCache, *[]fwdItem, *sync.Mutex, chan struct{}) {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	src := makeRSPeer(t, "10.0.0.1", srcAS, ctx, ctxID)
	src.recvCtxID = ctxID
	dst := makeRSPeer(t, "10.0.0.2", dstAS, ctx, ctxID)

	var dispatched []fwdItem
	var mu sync.Mutex
	done := make(chan struct{}, 8)

	pool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		for i := range items {
			if items[i].done != nil {
				items[i].done()
			}
			done <- struct{}{}
		}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	cache := NewRecentUpdateCache(100)
	r := &Reactor{
		recentUpdates: cache,
		clock:         clock.RealClock{},
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: pool,
	}
	return &reactorAPIAdapter{r: r}, cache, &dispatched, &mu, done
}

// TestRelayStoredRouteForwardsThroughForwardRail verifies a stored route is
// relayed through forwardUpdateCore and that its cache entry is fully released
// afterwards.
//
// VALIDATES: spec AC-1/C-3 -- the replay reaches the forward pool (the same rail
// a live forward uses), and the reconstruction buffer is returned exactly once
// via cache eviction rather than leaked or double-returned.
// PREVENTS: a relayed route pinning a read-pool buffer for the process lifetime,
// which under a large peer-up replay would exhaust the pool.
func TestRelayStoredRouteForwardsThroughForwardRail(t *testing.T) {
	api, cache, dispatched, mu, done := relayFixture(t)

	require.Equal(t, 0, cache.Len(), "cache starts empty")

	err := api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")})
	require.NoError(t, err, "relay of a well-formed stored route must succeed")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("relayed route never reached the forward pool")
	}

	mu.Lock()
	require.Len(t, *dispatched, 1, "exactly one destination dispatch")
	item := (*dispatched)[0]
	mu.Unlock()

	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), item.peer.Settings().Address)
	assert.NotEmpty(t, item.rawBodies, "the relay must produce wire bodies")

	// Eviction is what returns the pooled reconstruction buffer. Once the pool
	// worker's done() and the build-time release have both run, the entry must be
	// gone -- a lingering entry is a pinned buffer.
	require.Eventually(t, func() bool { return cache.Len() == 0 }, 2*time.Second, 10*time.Millisecond,
		"relay cache entry must be evicted, returning its pooled buffer")
}

// TestRelayStoredRouteFailsClosedWithoutSource verifies a route whose source peer
// is gone is NOT relayed.
//
// TestRelayStoredRouteCountsDispatchFailureAsIncomplete verifies a route that
// reached no destination because dispatch FAILED is reported, not counted as a
// successful relay.
//
// VALIDATES: the completeness guard distinguishes "egress policy suppressed this
// route" from "we failed to send it". forwardUpdateCore returns one error for
// both when nothing was dispatched (reactor_api_forward.go:689-691), so counting
// that error as handled would make a read-pool exhaustion or a wire-build failure
// indistinguishable from a policy decision.
// PREVENTS: a silently dropped route on a peer-up replay under load being
// reported as relayed, which is exactly the load-dependent class this spec exists
// to fix, and would leave `relayed < eligible` unable to ever fire.
func TestRelayStoredRouteCountsDispatchFailureAsIncomplete(t *testing.T) {
	api, cache, dispatched, mu, _ := relayFixture(t)

	// Drop the destination's forwarding facts. forwardUpdateCore skips a peer
	// whose facts are nil (reactor_api_forward.go:456-459) -- the state a peer is
	// in when its session tore down mid-forward. Nothing is dispatched, and the
	// failure must NOT read as a suppression.
	dst := api.r.peers[netip.MustParseAddrPort("10.0.0.2:179")]
	require.NotNil(t, dst, "fixture must expose the destination peer")
	dst.fwdFacts.Store(nil)

	err := api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")})
	require.Error(t, err, "a route that reached no destination through failure must not report success")
	assert.NotErrorIs(t, err, errAllDestinationsSuppressed,
		"a dispatch failure must not be classified as an egress-policy suppression")

	mu.Lock()
	assert.Empty(t, *dispatched, "nothing was dispatched")
	mu.Unlock()
	require.Eventually(t, func() bool { return cache.Len() == 0 }, 2*time.Second, 10*time.Millisecond,
		"the failed relay must still release its pooled buffer")
}

// TestRelayStoredRouteTreatsEgressSuppressionAsHandled verifies a route the
// destination's policy legitimately suppresses is NOT reported as a failure.
//
// VALIDATES: spec Review Gate R2-1 -- with a single destination, a correct
// suppression leaves nothing dispatched, and failing the whole replay for it
// would deliver strictly fewer routes than before the completeness guard existed.
// PREVENTS: one policy-suppressed route failing an entire peer-up replay and
// making bgp-rs skip its delta-convergence loop.
func TestRelayStoredRouteTreatsEgressSuppressionAsHandled(t *testing.T) {
	// Both peers in the local AS: RFC 4456 forbids reflecting an IBGP-learned
	// route to another IBGP peer when neither side is a route-reflector client
	// (reactor_api_forward.go:474-479), so the destination is suppressed.
	api, cache, dispatched, mu, _ := relayFixtureAS(t, 65000, 65000)

	err := api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")})
	require.NoError(t, err, "a correctly suppressed route is handled, not a failed relay")

	mu.Lock()
	assert.Empty(t, *dispatched, "a suppressed route reaches no peer")
	mu.Unlock()
	require.Eventually(t, func() bool { return cache.Len() == 0 }, 2*time.Second, 10*time.Millisecond,
		"the suppressed relay must release its pooled buffer")
}

// VALIDATES: spec C-4 -- without the source peer the egress transform a live
// forward would apply cannot be reproduced, so the relay must refuse.
// PREVENTS: relaying under a zero-valued source, which would apply the wrong
// AS_PATH prepend and skip the RFC 9234 role step -- sending a WRONG route rather
// than none.
func TestRelayStoredRouteFailsClosedWithoutSource(t *testing.T) {
	api, cache, dispatched, mu, _ := relayFixture(t)

	err := api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.9.9.9")}) // never an established peer
	require.ErrorIs(t, err, errRelayNoSource, "an unknown source must fail closed")

	mu.Lock()
	assert.Empty(t, *dispatched, "nothing may be dispatched for an unknown source")
	mu.Unlock()
	assert.Equal(t, 0, cache.Len(), "a refused relay must not leave a cache entry")
}

// TestRelayStoredRouteRejectsMalformedInput verifies each reconstruction failure
// mode is refused, named, and leaves no cache entry behind.
//
// VALIDATES: spec S-1/S-2/S-5 -- attacker-shaped or corrupt stored bytes are
// rejected whole.
// PREVENTS: a partially-decoded route reaching a peer session, and a failed build
// leaking the buffer it had already taken from the pool.
func TestRelayStoredRouteRejectsMalformedInput(t *testing.T) {
	cases := []struct {
		name string
		want error
		mut  func(*rpc.StoredRoute)
	}{
		{"unknown family", errRelayFamily, func(r *rpc.StoredRoute) { r.Family = "ipv9/unicast" }},
		{"attr hex not hex", errRelayHex, func(r *rpc.StoredRoute) { r.AttrHex = "zzzz" }},
		{"nlri missing", errRelayHex, func(r *rpc.StoredRoute) { r.NLRIHex = "" }},
		{"next-hop missing", errRelayHex, func(r *rpc.StoredRoute) { r.NextHopHex = "" }},
		{"attr block truncated", errRelayAttrs, func(r *rpc.StoredRoute) { r.AttrHex = "4001" }},
		// RFC 4271 Section 5.1.3 fixes legacy NEXT_HOP at 4 octets. An RFC 5549
		// route stores a 16-byte next hop; emitting it as type-3 would be an
		// attribute-length error at the peer.
		{"ipv4 next-hop not 4 bytes", errRelayNextHopLen, func(r *rpc.StoredRoute) {
			r.AttrHex = "4001010040020602010000FBF1" // ORIGIN + AS_PATH, no type-3
			r.NextHopHex = "20010db8000000000000000000000001"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api, cache, dispatched, mu, _ := relayFixture(t)
			route := storedIPv4Route("10.0.0.1")
			tc.mut(&route)

			err := api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"), []rpc.StoredRoute{route})
			require.ErrorIs(t, err, tc.want)

			mu.Lock()
			assert.Empty(t, *dispatched, "a malformed route must not be dispatched")
			mu.Unlock()
			assert.Equal(t, 0, cache.Len(), "a failed build must leave no cache entry")
		})
	}
}

// TestRelayStoredRouteSkipsSourceEqualsDestination verifies the engine refuses to
// echo a route back to the peer that sent it, even when the caller asks.
//
// VALIDATES: RFC 4271 split-horizon expectations -- a route is never advertised
// back to its source.
// PREVENTS: trusting a plugin-supplied route list to have filtered the source,
// which would bounce a route straight back at the peer that announced it.
func TestRelayStoredRouteSkipsSourceEqualsDestination(t *testing.T) {
	api, cache, dispatched, mu, _ := relayFixture(t)

	err := api.RelayStoredRoute(netip.MustParseAddr("10.0.0.1"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")})
	require.NoError(t, err, "skipping the source is not an error")

	mu.Lock()
	assert.Empty(t, *dispatched, "a route must never be relayed back to its source")
	mu.Unlock()
	assert.Equal(t, 0, cache.Len())
}

// TestRelayStoredRouteEmptyIsNoOp verifies an empty replay is a success.
//
// VALIDATES: a peer-up replay with nothing stored is normal, not an error.
// PREVENTS: adj-rib-in logging a failure on every peer-up before any route has
// been learned.
func TestRelayStoredRouteEmptyIsNoOp(t *testing.T) {
	api, cache, _, _, _ := relayFixture(t)
	require.NoError(t, api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"), nil))
	assert.Equal(t, 0, cache.Len())
}

// TestRelayPayloadSizeBoundary verifies the size guard at the exact UPDATE limit
// and one byte past it.
//
// VALIDATES: ai/rules/tdd.md boundary testing -- last valid, first invalid.
// PREVENTS: a stored route one byte over the 16-bit attribute-length limit being
// silently truncated onto the wire instead of refused.
func TestRelayPayloadSizeBoundary(t *testing.T) {
	nextHop := mustHex(t, "01010101")
	nlri := mustHex(t, "180A0000")

	// An attribute block sized so the total body lands exactly on, then one past,
	// the maximum. Body = 4 (lengths) + attrLen + len(nlri).
	fixed := 4 + len(nlri)
	for _, tc := range []struct {
		name    string
		attrLen int
		wantOK  bool
	}{
		{"largest encodable", maxUpdateBodyLen - fixed, true},
		{"one byte over", maxUpdateBodyLen - fixed + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// One synthetic extended-length attribute of the required size.
			valueLen := tc.attrLen - attrHdrExtLen
			block := make([]byte, tc.attrLen)
			block[0] = byte(0x80 | 0x10) // optional + extended length
			block[1] = 0xFE              // an unknown-but-well-formed attribute code
			block[2] = byte(valueLen >> 8)
			block[3] = byte(valueLen)

			spans, ok := scanAttrBlock(nil, block)
			require.True(t, ok, "the synthetic block must be well formed")

			_, ok = relayPayloadLen(spans, nextHop, nlri, family.IPv4Unicast, false)
			assert.Equal(t, tc.wantOK, ok, "size guard verdict at the boundary")
		})
	}

	// Sanity: the fixture hex used across these tests really is the .ci frame.
	assert.Equal(t, "180a0000", hex.EncodeToString(nlri))
}

// TestRelayStoredRouteRefusesAddPathSource verifies a route whose source session
// negotiated ADD-PATH is refused rather than relayed.
//
// VALIDATES: the stored NLRI does not record whether it carries an RFC 7911
// path-id (the structured ingest strips it, the legacy ingest prepends it), so it
// cannot be emitted under a context that declares add-path.
// PREVENTS: the worst outcome this change could produce -- a destination sharing
// the source's context receives the wire verbatim, parses the first four NLRI
// bytes as a path-id, and resets the session. A peer-up replay would tear down
// the peer it is replaying to.
func TestRelayStoredRouteRefusesAddPathSource(t *testing.T) {
	api, cache, dispatched, mu, _ := relayFixture(t)

	// Re-register the source under a context that negotiated ADD-PATH.
	addPathCtx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
	addPathCtxID, err := bgpctx.Registry.Register(addPathCtx)
	require.NoError(t, err)

	api.r.mu.RLock()
	src, found := api.r.findPeerByAddr(netip.MustParseAddr("10.0.0.1"))
	api.r.mu.RUnlock()
	require.True(t, found)
	src.recvCtxID = addPathCtxID

	relayErr := api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")})
	require.ErrorIs(t, relayErr, errRelayAddPath, "an add-path source must be refused")

	mu.Lock()
	assert.Empty(t, *dispatched, "nothing may reach the wire for an add-path source")
	mu.Unlock()
	assert.Equal(t, 0, cache.Len(), "a refused relay must leave no cache entry")
}
