// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// Related: forward_rs.go -- reactorForwardRS and hasActiveFilter, the code under test
//
// The deactivated-filter half of the route-server fast path lives in its own
// file. forward_rs_test.go carries RFC 7947 tags, and adding a function to a
// tagged file reads to the commit gate as a change to the evidence behind a
// published compliance claim (internal/le/commit/rfcchange.go). This test
// asserts nothing about RFC 7947: it asserts that an `inactive:` ref applies no
// policy, which is a Ze config rule.

package reactor

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// TestReactorForwardRSDeactivatedExportFilterKeepsTheFastPath is the other half
// of TestReactorForwardRSFallback: a chain whose every ref is deactivated
// applies no policy, so the policy-agnostic rail stays correct for that peer.
//
// VALIDATES: `inactive:` means the filter never runs, on the fast path too.
// PREVENTS: an operator losing the RS fast path by recording a decision and
// switching it off. A declared RFC 9234 role now requires the peer's chains to
// NAME a leak filter (bgpconfig.validateLeakFilterObligations), and `inactive:`
// is how that decision is recorded, so every such peer would have left the fast
// path for a filter that never executes.
func TestReactorForwardRSDeactivatedExportFilterKeepsTheFastPath(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)
	wu.SetMessageID(51)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}

	cache := newRecentUpdateCache(100)
	cache.Add(update)
	cache.Activate(51, 1)

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	dst.settings.ExportFilters = frefs("inactive:bgp-rs:test-filter")
	dst.refreshForwardFacts()

	var dispatched []fwdItem
	var mu sync.Mutex
	done := make(chan struct{})

	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched = append(dispatched, items...)
		mu.Unlock()
		close(done)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	defer testPool.Stop()

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: testPool,
	}

	skipped, nDispatched := reactorForwardRS(r, update, 51, netip.MustParseAddr("10.0.0.1"), src)
	assert.Empty(t, skipped, "a deactivated ref applies no policy, so the peer is not skipped")
	if nDispatched != 1 {
		t.Fatalf("nDispatched = %d, want 1", nDispatched)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dispatch")
	}

	mu.Lock()
	require.Len(t, dispatched, 1)
	assert.Equal(t, dst, dispatched[0].peer)
	mu.Unlock()
}

// TestExportFilterForBodyDeactivatedChainAcceptsWithoutAPIServer holds the
// egress rail to the same rule as the fast path: a chain of only deactivated
// refs applies no policy, so there is no policy for a missing API server to
// fail closed on.
//
// VALIDATES: an all-`inactive:` export chain takes the legitimate-accept branch,
// the one an empty chain takes, rather than the r.api == nil fail-closed branch.
// PREVENTS: the two rails disagreeing about the same peer -- reactorForwardRS
// forwarding the route while exportFilterForBody suppresses it, which
// blackholes the operator who recorded a filter and switched it off.
func TestExportFilterForBodyDeactivatedChainAcceptsWithoutAPIServer(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	peer := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)
	peer.settings.ExportFilters = frefs("inactive:bgp-rs:test-filter")
	peer.refreshForwardFacts()

	r := &Reactor{}

	suppress, override := r.exportFilterForBody(peer, []byte{0, 0, 0, 0})
	assert.False(t, suppress, "a deactivated ref applies no policy, so nothing suppresses the route")
	assert.Nil(t, override, "no filter ran, so no filter rewrote the body")
}

// TestRunIngressPolicyChainDeactivatedChainAcceptsWithoutAPIServer is the
// import half of the pair above.
//
// VALIDATES: an all-`inactive:` import chain takes the legitimate-accept branch
// rather than the r.api == nil fail-closed branch that drops the route.
// PREVENTS: an operator's opt-out dropping every inbound route on a daemon that
// runs no API server, while the route-server rail forwards the same route for
// the same peer.
func TestRunIngressPolicyChainDeactivatedChainAcceptsWithoutAPIServer(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, _ := bgpctx.Registry.Register(ctx)

	peer := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	peer.settings.ImportFilters = frefs("inactive:bgp-rs:test-filter")

	payload := []byte{0, 0, 0, 0}
	wu := wireu.NewWireUpdate(payload, ctxID)

	r := &Reactor{}

	res := r.runIngressPolicyChain(peer, netip.MustParseAddr("10.0.0.1"), 65001, wu, payload)
	assert.True(t, res.accept, "a deactivated ref applies no policy, so the route is accepted")
	assert.False(t, res.teardown, "no filter ran, so no filter asked for a teardown")
}
