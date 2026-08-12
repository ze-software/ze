package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// The RFC 7947 control-community gate on the GENERAL forward rail
// (reactorAPIAdapter.forwardUpdateCore, reactor_api_forward.go). Its route-server
// twin is reactorForwardRS (forward_rs.go), whose tests live in
// forward_rs_community_test.go and whose clients these tests reuse: the rails must
// answer the same question the same way, so they are asked with the same peers.

// communityForward runs one UPDATE through the general rail toward every client and
// returns what each one was asked to write. A client absent from the map was written
// nothing at all.
//
// It is wkForwardParts (forward_wellknown_test.go) with ONE difference, and the
// difference is why this is a second harness rather than an argument added to that
// one: globalLocalAS is the route server's own AS, which forwardUpdateCore reads as
// rsLocalAS, and RFC 7947's gate is inert while it is zero. wkForwardParts is inside
// an RFC-tagged file, so changing it needs the owner's approval
// (.claude/hooks/pretool-writeedit.py).
func communityForward(t testing.TB, payload []byte, clients ...*Peer) map[netip.Addr]wkParts {
	t.Helper()

	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, ctxID)

	type delivery struct {
		addr  netip.Addr
		parts wkParts
	}
	delivered := make(chan delivery, 8)
	pool := newFwdPool(func(k fwdKey, items []fwdItem) {
		delivered <- delivery{addr: k.peerAddr.Addr(), parts: wkItemParts(t, items)}
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	peerMap := make(map[netip.AddrPort]*Peer, len(clients))
	for _, c := range clients {
		key := fwdKey{peerAddr: c.Settings().PeerKey()}
		pool.registerOutgoingPool(key, 4096)
		peerMap[key.peerAddr] = c
	}

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           peerMap,
		fwdPool:         pool,
	}
	adapter := &reactorAPIAdapter{r: r}

	// The source is EXTERNAL, which is the shape a route server sees, and it also
	// keeps RFC 4456 out of the way: an internal source plus an internal destination,
	// neither a reflector client, is suppressed before RFC 7947 is asked.
	//
	// Reaching NO destination is a legitimate outcome here -- it is what a fully
	// suppressed UPDATE looks like -- so the error is asserted by the one test that
	// is about the error (TestForwardRefusedClientWithWithdrawalIsNotCountedSuppressed).
	_ = adapter.forwardUpdateCore(update, id, clients, forwardSourceInfo{
		resolved: true, isIBGP: false, globalLocalAS: 65000,
	})

	got := make(map[netip.Addr]wkParts, len(clients))
	for range clients {
		select {
		case d := <-delivered:
			got[d.addr] = d.parts
		case <-time.After(500 * time.Millisecond):
			return got
		}
	}
	return got
}

// TestForwardWithdrawsFromClientRefusedByControlCommunity is the general-rail
// sibling of TestForwardRSWithdrawsFromClientRefusedByControlCommunity.
//
// VALIDATES: a route-server client an RS control community excludes still receives
// the withdrawal half of a mixed UPDATE on the general rail.
// PREVENTS: the client keeping a prefix ze can no longer take back until the session
// resets. wireu.ParseCommunityPolicy reads RSBlackhole, WhitelistASNs and
// BlacklistASNs off the ANNOUNCED route's communities, so ShouldForwardTo is a
// decision about ONE ROUTE. Refusing the whole message applies that decision to
// withdrawn routes, which carry no attribute and were tagged by nobody.
func TestForwardWithdrawsFromClientRefusedByControlCommunity(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	refused := rsCommunityClient(t, rsRefusedClientAddr, rsRefusedClientAS, ctx, ctxID)
	allowed := rsCommunityClient(t, rsAllowedClientAddr, rsAllowedClientAS, ctx, ctxID)

	got := communityForward(t, wkMixedPayload(rsBlacklist(rsRefusedClientAS)), refused, allowed)

	parts, reached := got[netip.MustParseAddr(rsRefusedClientAddr)]
	require.True(t, reached, "the withdrawal must reach the excluded client")
	assert.Equal(t, wkWithdrawnPrefix, parts.withdrawn,
		"the route being taken back was never tagged, so the exclusion does not cover it")
	assert.NotContains(t, string(parts.nlri), string(wkAnnouncedPrefix),
		"the announcement the control community excludes must not reach that client")

	// The control. The same UPDATE reaches the client the community does NOT exclude,
	// whole. Without it, "the excluded client got only the withdrawal" CAN pass with
	// the announcement lost for some other reason.
	both, reached := got[netip.MustParseAddr(rsAllowedClientAddr)]
	require.True(t, reached)
	assert.Equal(t, wkWithdrawnPrefix, both.withdrawn)
	assert.Equal(t, wkAnnouncedPrefix, both.nlri,
		"a client the control community does not name receives both halves")
}

// VALIDATES: an UPDATE that only announces still reaches nobody the control community
// excludes, so the withdrawal path adds no route rather than weakening the gate.
// PREVENTS: reading the test above as "the general rail now sends something to every
// excluded client". A pure announcement has no withdrawal half, so the excluded client
// is written nothing at all, and suppressedCount counts it. That silence is also what
// makes the assertion above evidence, rather than a message arriving for its own
// reasons.
func TestForwardSendsNothingToRefusedClientWithoutWithdrawal(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	refused := rsCommunityClient(t, rsRefusedClientAddr, rsRefusedClientAS, ctx, ctxID)
	allowed := rsCommunityClient(t, rsAllowedClientAddr, rsAllowedClientAS, ctx, ctxID)

	got := communityForward(t, wkTestPayload(rsBlacklist(rsRefusedClientAS)), refused, allowed)

	assert.NotContains(t, got, netip.MustParseAddr(rsRefusedClientAddr),
		"an announcement-only UPDATE leaves an excluded client nothing to write")

	// The control: the same UPDATE, the same rail, the client the community does not
	// name. A silence above is only evidence once this one is not silent.
	both, reached := got[netip.MustParseAddr(rsAllowedClientAddr)]
	require.True(t, reached)
	assert.Equal(t, wkAnnouncedPrefix, both.nlri)
	assert.Empty(t, both.withdrawn)
}

// VALIDATES: a client the control community refuses, which the withdrawal half still
// reaches, is NOT counted in suppressedCount -- so "nothing was dispatched" cannot be
// reported as errAllDestinationsSuppressed when the dispatch itself failed.
// PREVENTS: the counter lying about a destination this UPDATE reached. suppressedCount
// means "skipped by a POLICY decision", and errAllDestinationsSuppressed exists so a
// caller can tell that outcome from a drop. Counting a reached client makes
// suppressedCount == len(matchingPeers) while the wire was never written, which is the
// exact conflation the sentinel's own contract forbids: a read-buffer exhaustion or a
// stopped pool would then read as a clean policy suppression.
//
// The forward pool is stopped, so TryDispatch and DispatchOverflow both refuse and
// dispatchedCount stays 0 (reactor_api_forward.go, "pool stopped").
func TestForwardRefusedClientWithWithdrawalIsNotCountedSuppressed(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, wkMixedPayload(rsBlacklist(rsRefusedClientAS)), ctxID)

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {},
		fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	refused := rsCommunityClient(t, rsRefusedClientAddr, rsRefusedClientAS, ctx, ctxID)
	pool.registerOutgoingPool(fwdKey{peerAddr: refused.Settings().PeerKey()}, 4096)
	pool.Stop()

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           map[netip.AddrPort]*Peer{refused.Settings().PeerKey(): refused},
		fwdPool:         pool,
	}
	adapter := &reactorAPIAdapter{r: r}

	got := adapter.forwardUpdateCore(update, id, []*Peer{refused}, forwardSourceInfo{
		resolved: true, isIBGP: false, globalLocalAS: 65000,
	})

	require.Error(t, got, "a stopped pool wrote nothing, so this UPDATE reached nobody")
	assert.NotErrorIs(t, got, errAllDestinationsSuppressed,
		"the withdrawal was built for this client, so the drop is a dispatch failure and not a policy suppression")

	// The control: the same client, the same rail, an UPDATE with no withdrawal half.
	// THAT one is a policy suppression, so the sentinel is the right answer and the
	// assertion above cannot pass by way of the sentinel never being returned here.
	announceOnly, announceID := newLeakTestUpdate(t, cache, wkTestPayload(rsBlacklist(rsRefusedClientAS)), ctxID)
	got = adapter.forwardUpdateCore(announceOnly, announceID, []*Peer{refused}, forwardSourceInfo{
		resolved: true, isIBGP: false, globalLocalAS: 65000,
	})
	assert.ErrorIs(t, got, errAllDestinationsSuppressed,
		"a client the community refuses, with nothing to withdraw, IS suppressed by policy")
}

// TestForwardRailsAgreeOnControlCommunityRefusal is the invariant both files state
// in prose and neither used to test: the general rail and the route-server rail MUST
// stay behaviorally identical, because which one runs is a deployment's choice of
// rs-fast-path and not a policy an operator asked for.
//
// VALIDATES: for one UPDATE and one set of clients, the two rails deliver the same
// halves to the same clients.
// PREVENTS: the state this fix closed, where one rail sent a refused client its
// withdrawals and the other dropped them. Fixing one rail alone is worse than fixing
// neither: the behavior then depends on the rail, so an operator cannot read either
// file and know what their daemon does.
func TestForwardRailsAgreeOnControlCommunityRefusal(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	cases := []struct {
		name    string
		payload []byte
	}{
		// The mixed shape is where the two rails disagreed. The announcement-only
		// shape is the control: agreeing on "nobody refused is written anything" is
		// what stops the row above from passing because both rails deliver
		// everything to everybody.
		{"mixed announcement and withdrawal", wkMixedPayload(rsBlacklist(rsRefusedClientAS))},
		{"announcement only", wkTestPayload(rsBlacklist(rsRefusedClientAS))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			general := communityForward(t, tc.payload,
				rsCommunityClient(t, rsRefusedClientAddr, rsRefusedClientAS, ctx, ctxID),
				rsCommunityClient(t, rsAllowedClientAddr, rsAllowedClientAS, ctx, ctxID))
			routeServer := rsCommunityForward(t, tc.payload,
				rsCommunityClient(t, rsRefusedClientAddr, rsRefusedClientAS, ctx, ctxID),
				rsCommunityClient(t, rsAllowedClientAddr, rsAllowedClientAS, ctx, ctxID))

			assert.Equal(t, routeServer, general,
				"the two forward rails must deliver the same halves to the same clients")
			// Neither map being empty is what makes the equality evidence: two rails
			// that both forward nothing agree about nothing.
			assert.NotEmpty(t, general)
		})
	}
}
