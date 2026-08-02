package reactor

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
		// The bodies alias the read-pool buffer: forward_body.go appends
		// peerWire.Payload() zero-copy, and done() drops the last retain, so
		// evictLocked hands that buffer back (recent_cache.go). Copy before
		// releasing. Keeping the alias would let a later assertion read whatever
		// the pool handed out next, which fails a test whose code is correct.
		mu.Lock()
		for i := range items {
			item := items[i]
			bodies := make([][]byte, len(item.rawBodies))
			for j, body := range item.rawBodies {
				bodies[j] = append([]byte(nil), body...)
			}
			item.rawBodies = bodies
			dispatched = append(dispatched, item)
		}
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
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		clock:           clock.RealClock{},
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

// relayBodyASPath returns the AS_PATH ASNs carried by a forwarded UPDATE body.
// The body shape is RFC 4271 Section 4.3: withdrawn-routes length, withdrawn
// routes, total-path-attribute length, attributes, NLRI.
//
// It reads the AS_PATH the destination peer actually receives, which is the only
// thing that can distinguish "the route server left the path alone" from "the
// route server prepended its AS". Asserting a non-empty body cannot.
func relayBodyASPath(t *testing.T, body []byte) []uint32 {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 4, "an UPDATE body carries both length fields")
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	attrLenOff := 2 + withdrawnLen
	require.LessOrEqual(t, attrLenOff+2, len(body), "attribute-length field is inside the body")
	attrLen := int(binary.BigEndian.Uint16(body[attrLenOff : attrLenOff+2]))
	attrOff := attrLenOff + 2
	require.LessOrEqual(t, attrOff+attrLen, len(body), "attribute block is inside the body")

	path := forwardBodyFindASPath(t, body[attrOff:attrOff+attrLen], true)
	require.NotNil(t, path, "the forwarded body must carry an AS_PATH")
	require.Len(t, path.Segments, 1, "the fixture route carries one AS_SEQUENCE")
	return path.Segments[0].ASNs
}

// storedRouteASPath returns the AS_PATH as it was RECEIVED, decoded from the
// stored attribute bytes. It is the "before" half of the transform assertions.
func storedRouteASPath(t *testing.T) []uint32 {
	t.Helper()
	attrs := mustHex(t, storedIPv4Route("10.0.0.1").AttrHex)
	path := forwardBodyFindASPath(t, attrs, true)
	require.NotNil(t, path, "the stored route must carry an AS_PATH")
	require.Len(t, path.Segments, 1)
	return path.Segments[0].ASNs
}

// relayDispatchedBody runs a one-route replay to 10.0.0.2 and returns the wire
// body the destination peer received.
func relayDispatchedBody(t *testing.T, api *reactorAPIAdapter, dispatched *[]fwdItem, mu *sync.Mutex, done chan struct{}) []byte {
	t.Helper()
	require.NoError(t, api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")}))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("relayed route never reached the forward pool")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, *dispatched, 1, "exactly one destination dispatch")
	item := (*dispatched)[0]
	require.Equal(t, netip.MustParseAddr("10.0.0.2"), item.peer.Settings().Address)
	require.NotEmpty(t, item.rawBodies, "the relay must produce wire bodies")
	return item.rawBodies[0]
}

// TestRelayStoredRouteRSClientPreservesASPath proves a stored route replayed to
// an RS-client peer reaches it with the AS_PATH it arrived with: the route
// server does not insert itself.
//
// This pins the transform on the RELAY rail. The existing x-1 positive
// (TestReactorForwardRSTransparent) drives reactorForwardRS, whose prepend gate
// is a SEPARATE copy in forward_rs.go. The gate this test drives lives in
// forwardUpdateCore (reactor_api_forward.go), which is what BOTH
// RelayStoredRoute (peer-up replay) and ForwardUpdate reach. Before this test
// neither polarity of x-1 was asserted on that gate at all, so reverting it
// there broke a route server's peer-up replay with every test still green.
//
// RFC requirement: RFC7947-x-1 positive -- the route server SHOULD NOT prepend its own AS to
// AS_PATH for an RS client; a route replayed to an RS client through RelayStoredRoute carries
// the AS_PATH exactly as received, and the route server's own AS 65000 is absent from it.
// VALIDATES: RFC 7947 Section 2.2.2.1 through the stored-route replay entry point.
// PREVENTS: a peer-up replay silently prepending the route server's AS, which makes every
// client peering look like transit through the IXP rather than direct eBGP between clients.
func TestRelayStoredRouteRSClientPreservesASPath(t *testing.T) {
	api, _, dispatched, mu, done := relayFixture(t)

	// The RS-client leaf is the ONLY thing that selects the transparent path:
	// facts.rsClient comes from PeerSettings.RSClient alone
	// (peer_forward_facts.go), and its YANG default is false.
	dst := api.r.peers[netip.MustParseAddrPort("10.0.0.2:179")]
	require.NotNil(t, dst, "fixture must expose the destination peer")
	dst.settings.RSClient = true
	dst.refreshForwardFacts()
	require.True(t, dst.forwardFacts().rsClient, "precondition: destination is an RS client")
	require.True(t, dst.forwardFacts().isEBGP, "precondition: destination is eBGP, so the prepend gate is live")

	before := storedRouteASPath(t)
	after := relayBodyASPath(t, relayDispatchedBody(t, api, dispatched, mu, done))

	assert.Equal(t, before, after,
		"RFC 7947 S2.2.2.1: an RS client must receive the AS_PATH unchanged")
	assert.NotContains(t, after, uint32(65000),
		"the route server's own AS must not appear in the AS_PATH it relays")
}

// TestRelayStoredRoutePlainEBGPPrependsLocalAS is the confining half: the
// no-prepend behavior above is specific to RS clients, not a blanket disable of
// AS_PATH prepending on the relay rail.
//
// Without this, a mutation that never prepends (gate forced false) would leave
// the positive test above green and nothing else on this rail would notice --
// the relay would violate RFC 4271 Section 5.1.2 for every ordinary eBGP peer.
//
// RFC requirement: RFC7947-x-1 negative -- the "SHOULD NOT prepend own AS" transparency is
// confined to RS clients: a plain (non-RS-client) eBGP destination replayed through
// RelayStoredRoute DOES get the local AS 65000 prepended to the leading AS_SEQUENCE.
// VALIDATES: RFC 4271 Section 5.1.2 still governs the relay rail for ordinary eBGP peers.
// PREVENTS: suppressing the prepend for every peer instead of just RS clients, which would
// make Ze advertise routes it relays as though they were directly adjacent.
func TestRelayStoredRoutePlainEBGPPrependsLocalAS(t *testing.T) {
	api, _, dispatched, mu, done := relayFixture(t)

	// relayFixture leaves RSClient at its default (false), so this destination is
	// an ordinary eBGP peer.
	dst := api.r.peers[netip.MustParseAddrPort("10.0.0.2:179")]
	require.NotNil(t, dst, "fixture must expose the destination peer")
	require.False(t, dst.forwardFacts().rsClient, "precondition: destination is NOT an RS client")
	require.True(t, dst.forwardFacts().isEBGP, "precondition: destination is eBGP")

	before := storedRouteASPath(t)
	after := relayBodyASPath(t, relayDispatchedBody(t, api, dispatched, mu, done))

	assert.Equal(t, append([]uint32{65000}, before...), after,
		"RFC 4271 S5.1.2: an ordinary eBGP peer receives the local AS at the head of AS_PATH")
}

// TestRelayStoredRouteCountsDispatchFailureAsIncomplete verifies a route that
// reached no destination because dispatch FAILED is reported, not counted as a
// successful relay.
//
// VALIDATES: the completeness guard distinguishes "egress policy suppressed this
// route" from "we failed to send it". forwardUpdateCore reaches one zero-dispatch
// exit for both (reactor_api_forward.go:719-726), so counting that exit as
// handled would make a read-pool exhaustion or a wire-build failure
// indistinguishable from a policy decision.
// PREVENTS: a silently dropped route on a peer-up replay under load being
// reported as relayed, which is exactly the load-dependent class this spec exists
// to fix, and would leave `relayed < eligible` unable to ever fire.
func TestRelayStoredRouteCountsDispatchFailureAsIncomplete(t *testing.T) {
	api, cache, dispatched, mu, _ := relayFixture(t)

	// Drop the destination's forwarding facts. forwardUpdateCore skips a peer
	// whose facts are nil (reactor_api_forward.go:472-475) -- the state a peer is
	// in when its session tore down mid-forward (peer.go clearEncodingContexts
	// stores nil). Nothing is dispatched, and the failure must NOT read as a
	// suppression.
	//
	// This drives ONE of the failure branches. The pool-exhaustion and
	// body-build branches named in the spec are not reachable from a unit
	// fixture; that coverage gap is homed in
	// plan/spec-fixit-stored-route-relay-hardening.md.
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
	// (reactor_api_forward.go:491-497), so the destination is suppressed.
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

// TestRelayStoredRouteCountsFailedEgressStepAsIncomplete verifies a route
// dropped because an egress STEP could not run is reported, not counted as a
// policy suppression.
//
// VALIDATES: `accept == false` out of the ordered egress pass is overloaded --
// a filter-plugin IPC error under the default fail-closed on-error policy
// (filter_chain.go policyFilterFunc), an unparseable filter response, a missing
// API server (filter_ordered.go), and a filter panic (safeEgressFilter) all
// produce it alongside a genuine policy reject. Only the genuine reject may
// count as suppression.
// PREVENTS: the exact fail-open this spec exists to close, re-entered through a
// different door -- a forked export-filter plugin timing out under load would
// drop every route of a peer-up replay while RelayStoredRoute reported the
// replay complete.
func TestRelayStoredRouteCountsFailedEgressStepAsIncomplete(t *testing.T) {
	api, cache, dispatched, mu, _ := relayFixture(t)

	// One in-process egress step that panics. safeEgressFilter recovers it and
	// suppresses fail-closed, reporting panicked=true: a step that COULD NOT RUN,
	// not a step that decided.
	api.r.orderedEgressSteps = []orderedEgressStep{{
		name: "panicking-test-filter",
		inproc: func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
			panic("simulated egress filter failure")
		},
	}}

	err := api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")})
	require.Error(t, err, "a route dropped by a FAILED egress step must not report success")
	assert.NotErrorIs(t, err, errAllDestinationsSuppressed,
		"a step that could not run is not an egress-policy suppression")

	mu.Lock()
	assert.Empty(t, *dispatched, "nothing was dispatched")
	mu.Unlock()
	require.Eventually(t, func() bool { return cache.Len() == 0 }, 2*time.Second, 10*time.Millisecond,
		"the failed relay must still release its pooled buffer")
}

// TestRelayStoredRouteCountsFailedPolicyChainAsIncomplete verifies the OTHER
// half of the failed-step distinction: the export policy chain.
//
// The panic test above drives the in-process branch. This one drives the policy
// chain, whose "could not run" state is an export policy configured while the
// filter engine (the API server) is absent -- runEgressPolicyChainASN4 calls that
// a guard MISS, not an accept. Without it, three of the four producers the fix
// names had no test that would fail if their half were reverted.
//
// VALIDATES: a route dropped because the export chain could not be evaluated is
// reported as a drop, not as an egress-policy suppression.
// PREVENTS: reverting `failed: true` on the nil-API guard (filter_ordered.go) or
// the `failed` plumbing into the step result, either of which would let a replay
// whose policy engine was missing report itself complete having sent nothing.
func TestRelayStoredRouteCountsFailedPolicyChainAsIncomplete(t *testing.T) {
	api, cache, dispatched, mu, _ := relayFixture(t)

	// Export policy configured on the destination, but no API server: the chain
	// cannot be evaluated. r.api is nil in this fixture by construction.
	require.Nil(t, api.r.api, "fixture must have no API server for this case")
	dst := api.r.peers[netip.MustParseAddrPort("10.0.0.2:179")]
	require.NotNil(t, dst, "fixture must expose the destination peer")
	dst.settings.ExportFilters = []filterapi.FilterRef{{Name: "someplugin:scrub"}}
	dst.refreshForwardFacts()

	api.r.orderedEgressSteps = []orderedEgressStep{{name: "peer-chain", policyChain: true}}

	err := api.RelayStoredRoute(netip.MustParseAddr("10.0.0.2"),
		[]rpc.StoredRoute{storedIPv4Route("10.0.0.1")})
	require.Error(t, err, "a route dropped by an unevaluatable export chain must not report success")
	assert.NotErrorIs(t, err, errAllDestinationsSuppressed,
		"a chain that could not run is not an egress-policy suppression")

	mu.Lock()
	assert.Empty(t, *dispatched, "nothing was dispatched")
	mu.Unlock()
	require.Eventually(t, func() bool { return cache.Len() == 0 }, 2*time.Second, 10*time.Millisecond,
		"the failed relay must still release its pooled buffer")
}

// TestRelayStoredRouteFailsClosedWithoutSource verifies a route whose source peer
// is gone is NOT relayed.
//
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
