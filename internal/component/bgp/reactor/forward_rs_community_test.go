package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// rsCommunityClient builds an established RS CLIENT in peerAS, which is the fact
// RFC 7947's control-community gate branches on (PeerSettings.RSClient, read as
// forwardFacts.rsClient). makeRSPeer sets RSFastPath instead, so it selects the
// rail without ever reaching that gate.
func rsCommunityClient(t testing.TB, addr string, peerAS uint32, ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	t.Helper()
	peer := makeRSPeer(t, addr, peerAS, ctx, ctxID)
	peer.Settings().RSClient = true
	peer.refreshForwardFacts()
	return peer
}

// rsCommunityForward runs one UPDATE through the route-server rail toward every
// client and returns what each one was asked to write. A client absent from the
// map was written nothing at all.
func rsCommunityForward(t testing.TB, payload []byte, clients ...*Peer) map[netip.Addr]wkParts {
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

	srcAddr := netip.MustParseAddr("10.0.0.1")
	src := makeRSPeer(t, srcAddr.String(), 65001, ctx, ctxID)
	peerMap := map[netip.AddrPort]*Peer{src.Settings().PeerKey(): src}
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
	reactorForwardRS(r, update, id, srcAddr, src)

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

const (
	rsRefusedClientAddr = "10.0.0.4"
	rsAllowedClientAddr = "10.0.0.5"
	rsRefusedClientAS   = 65002
	rsAllowedClientAS   = 65003
)

// rsBlacklist is the ze route-server control community 0:<ASN>, which
// wireu.parseCommunityAttr reads into CommunityPolicy.BlacklistASNs and
// ShouldForwardTo then answers false for.
func rsBlacklist(asn uint32) attribute.Community { return attribute.Community(asn) }

// VALIDATES: a route-server client an RS control community excludes still receives
// the withdrawal half of a mixed UPDATE.
// PREVENTS: the client keeping a prefix ze can no longer take back until the session
// resets. wireu.CommunityPolicy is parsed from the ANNOUNCED route's communities:
// RSBlackhole, WhitelistASNs, BlacklistASNs. ShouldForwardTo is therefore a decision
// about one route. Refusing the whole message applies that decision to withdrawn
// routes which carry no attribute and were tagged by nobody. Same shape and same
// repair as RFC 1997 in ef5ed2a10 (TestForwardRSWithdrawsFromRefusedClient).
func TestForwardRSWithdrawsFromClientRefusedByControlCommunity(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	refused := rsCommunityClient(t, rsRefusedClientAddr, rsRefusedClientAS, ctx, ctxID)
	allowed := rsCommunityClient(t, rsAllowedClientAddr, rsAllowedClientAS, ctx, ctxID)

	got := rsCommunityForward(t, wkMixedPayload(rsBlacklist(rsRefusedClientAS)), refused, allowed)

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
// PREVENTS: reading the test above as "RFC 7947 now sends something to every excluded
// client". A pure announcement has no withdrawal half, so the excluded client is written
// nothing at all. That silence is also what makes the assertion above evidence, rather
// than a message arriving for its own reasons.
func TestForwardRSSendsNothingToRefusedClientWithoutWithdrawal(t *testing.T) {
	ctx := bgpctx.EncodingContextForASN4(true)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	refused := rsCommunityClient(t, rsRefusedClientAddr, rsRefusedClientAS, ctx, ctxID)
	allowed := rsCommunityClient(t, rsAllowedClientAddr, rsAllowedClientAS, ctx, ctxID)

	got := rsCommunityForward(t, wkTestPayload(rsBlacklist(rsRefusedClientAS)), refused, allowed)

	assert.NotContains(t, got, netip.MustParseAddr(rsRefusedClientAddr),
		"an announcement-only UPDATE leaves an excluded client nothing to write")

	// The control: the same UPDATE, the same rail, the client the community does not
	// name. A silence above is only evidence once this one is not silent.
	both, reached := got[netip.MustParseAddr(rsAllowedClientAddr)]
	require.True(t, reached)
	assert.Equal(t, wkAnnouncedPrefix, both.nlri)
	assert.Empty(t, both.withdrawn)
}
