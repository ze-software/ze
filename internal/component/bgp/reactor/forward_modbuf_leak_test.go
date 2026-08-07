// Tests for the Outgoing Peer Pool ("MOD buffer") release obligation on the
// body-build failure path of the two forward rails.
//
// Both rails acquire a per-destination pool buffer when a policy rebuild is
// needed (buildModifiedPayload / buildWithdrawalPayload), store its index on the
// fwdItem, and hand the item to the forward pool, which returns the buffer after
// the write. Between those two points sits buildFwdBody. When it reports !ok the
// loop drops the destination, so the item never reaches the pool and nothing else
// can give the buffer back.
//
// The sibling obligation on the shutdown path is
// forward_pool_stopped_release_test.go; the read-pool one is
// forward_readbuf_leak_test.go. Different pool, different path, same contract:
// a resource taken on the way in is returned on every way out.
package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unregisteredCtxID returns a ContextID the global registry does not hold.
//
// It is the failure lever these tests need. bgpctx.Registry hands IDs out from 1
// upwards and never releases one, so counting DOWN from the top of the uint16
// space finds an unused ID whatever else the test binary has registered.
// fwdUpdateForDestination (forward_body.go) rejects an unknown destination
// context, which is the "encoding update for forward" failure buildFwdBody logs
// before returning !ok.
func unregisteredCtxID(t testing.TB) bgpctx.ContextID {
	t.Helper()
	for id := bgpctx.ContextID(65535); id > 1; id-- {
		if bgpctx.Registry.Get(id) == nil {
			return id
		}
	}
	t.Fatal("the context registry is full: no unregistered ID to drive the failure with")
	return 0
}

// modBufTestPayload builds an UPDATE body carrying ORIGIN plus one IPv4 prefix.
//
// The NLRI matters: the rebuild's advertise gate (advertiseGate.advertises,
// forward_build.go) refuses to create an attribute on a body that advertises
// nothing, and a refused plan acquires no buffer. With a prefix present the
// next-hop Set below plans, the payload is rebuilt, and a pool buffer is taken.
func modBufTestPayload() []byte {
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	// AS_PATH (4-byte ASNs), one AS_SEQUENCE element, so the source context's
	// ASN4 setting is expressed on the wire.
	aspValue := []byte{2, 1, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(aspValue[2:], 65001)
	aspAttr := append([]byte{0x40, 0x02, byte(len(aspValue))}, aspValue...)

	attrs := append(append([]byte{}, origin...), aspAttr...)
	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24
	return buildUpdatePayload(attrs, nlri)
}

// makeNextHopSelfIBGPPeer builds an established IBGP destination that rewrites
// next-hop to its own address.
//
// IBGP plus next-hop-self is the smallest shape that reaches the acquire site:
// applyFactsNextHop records a NEXT_HOP Set (peer_forward_facts.go), so
// mods.HasModifications() is true and the rail calls buildModifiedPayload with
// this peer's Outgoing Peer Pool. IBGP also keeps the AS-path intent out of the
// edit set, so the rebuilt wire keeps the SOURCE context label and buildFwdBody
// is forced down the transcode branch, where sendCtxID decides success.
func makeNextHopSelfIBGPPeer(t testing.TB, addr string, ctx *bgpctx.EncodingContext, sendCtxID bgpctx.ContextID) *Peer {
	t.Helper()
	peerAddr := netip.MustParseAddr(addr)
	settings := &PeerSettings{
		Connection:    ConnectionBoth,
		Address:       peerAddr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65000, // IBGP
		RouterID:      0x01020300 | uint32(peerAddr.As4()[3]),
		LocalAddress:  netip.MustParseAddr("10.0.0.254"),
		NextHopMode:   NextHopSelf,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		ExtendedMessage: false,
	})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = sendCtxID
	peer.refreshForwardFacts()
	require.Equal(t, nhModeSelf4, peer.forwardFacts().nhMode,
		"precondition: the peer must rewrite next-hop, or no rebuild happens and no buffer is taken")
	require.False(t, peer.forwardFacts().isEBGP,
		"precondition: the peer must be IBGP, or the AS-path intent relabels the wire and buildFwdBody stops transcoding")
	return peer
}

// modBufFixture wires one reactor around one next-hop-self destination and
// registers that destination's Outgoing Peer Pool, which is the pool under test.
type modBufFixture struct {
	adapter *reactorAPIAdapter
	reactor *Reactor
	dst     *Peer
	pool    *fwdPool
	peerPl  *peerPool
	update  *ReceivedUpdate
	id      uint64
}

func newModBufFixture(t *testing.T, sendCtx *bgpctx.EncodingContext, sendCtxID bgpctx.ContextID, handler func(fwdKey, []fwdItem)) *modBufFixture {
	t.Helper()

	srcCtx := bgpctx.EncodingContextForASN4(true)
	srcCtxID, err := bgpctx.Registry.Register(srcCtx)
	require.NoError(t, err)

	cache := NewRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, modBufTestPayload(), srcCtxID)

	dst := makeNextHopSelfIBGPPeer(t, "10.0.0.2", sendCtx, sendCtxID)

	pool := newFwdPool(handler, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	key := fwdKey{peerAddr: dst.Settings().PeerKey()}
	pool.RegisterOutgoingPool(key, 4096)
	pp := pool.OutgoingPool(key)
	require.NotNil(t, pp, "setup: the destination must have an Outgoing Peer Pool, or the rebuild falls back to sync.Pool")

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           map[netip.AddrPort]*Peer{key.peerAddr: dst},
		fwdPool:         pool,
	}

	return &modBufFixture{
		adapter: &reactorAPIAdapter{r: r},
		reactor: r,
		dst:     dst,
		pool:    pool,
		peerPl:  pp,
		update:  update,
		id:      id,
	}
}

// TestForwardModBufTakenOnRebuild is the control for the two leak tests below.
//
// It proves the fixture actually reaches the acquire site, which the leak tests
// cannot show on their own: a pool that never lent a buffer also ends at its
// baseline, so "back to baseline" is only evidence once a loan is proven.
//
// VALIDATES: a next-hop-self IBGP destination with a registered Outgoing Peer
// Pool carries a pool buffer on its fwdItem (peerBufIdx > 0).
// PREVENTS: the leak tests below passing vacuously because no buffer was taken.
func TestForwardModBufTakenOnRebuild(t *testing.T) {
	// A REGISTERED 2-byte-ASN send context: buildFwdBody transcodes and succeeds,
	// so the item is dispatched and the handler can read what it carries.
	sendCtx := bgpctx.EncodingContextForASN4(false)
	sendCtxID, err := bgpctx.Registry.Register(sendCtx)
	require.NoError(t, err)

	seen := make(chan int, 4)
	f := newModBufFixture(t, sendCtx, sendCtxID, func(_ fwdKey, items []fwdItem) {
		for i := range items {
			seen <- items[i].peerBufIdx
		}
	})

	require.NoError(t, f.adapter.forwardUpdateCore(f.update, f.id, []*Peer{f.dst}, leakTestSource))

	select {
	case idx := <-seen:
		require.Positive(t, idx,
			"the rebuilt payload must sit in an Outgoing Peer Pool buffer, or the leak tests below prove nothing")
	case <-time.After(2 * time.Second):
		t.Fatal("the dispatched item never reached the batch handler")
	}
}

// TestForwardUpdateCoreReturnsModBufOnBodyFailure drives the general forward
// rail's body-build failure with a pool buffer already acquired.
//
// VALIDATES: forwardUpdateCore returns the destination's Outgoing Peer Pool
// buffer when buildFwdBody reports !ok, so the pool's free count is unchanged
// across the call.
// PREVENTS: the leak recorded 2026-07-21. The rebuild stored the buffer on the
// fwdItem and the `continue` at the buildFwdBody failure dropped that item, so
// nothing reached fwdPool.releaseItem and the buffer was gone for the life of the
// session. 64 buffers per destination, one lost per failing UPDATE.
func TestForwardUpdateCoreReturnsModBufOnBodyFailure(t *testing.T) {
	sendCtx := bgpctx.EncodingContextForASN4(false)
	f := newModBufFixture(t, sendCtx, unregisteredCtxID(t), func(_ fwdKey, _ []fwdItem) {})

	before := freeCount(f.peerPl)

	err := f.adapter.forwardUpdateCore(f.update, f.id, []*Peer{f.dst}, leakTestSource)
	require.ErrorIs(t, err, errNoEstablishedPeersToForwardTo,
		"the only destination must fail its body build, or this test is not on the failure path")

	assert.Equal(t, before, freeCount(f.peerPl),
		"the Outgoing Peer Pool buffer must come back when the body build fails")
}

// TestForwardRSReturnsModBufOnBodyFailure drives the same failure on the
// route-server rail, which carries its own copy of the loop.
//
// VALIDATES: reactorForwardRS returns the destination's Outgoing Peer Pool
// buffer when buildFwdBody reports !ok.
// PREVENTS: the same leak on the rail a route server actually runs. The two
// rails are separate code, so fixing one leaves the other leaking on whichever
// path the deployment selects.
func TestForwardRSReturnsModBufOnBodyFailure(t *testing.T) {
	sendCtx := bgpctx.EncodingContextForASN4(false)
	f := newModBufFixture(t, sendCtx, unregisteredCtxID(t), func(_ fwdKey, _ []fwdItem) {})

	srcAddr := netip.MustParseAddr("10.0.0.1")
	src := makeRSPeer(t, srcAddr.String(), 65001, bgpctx.EncodingContextForASN4(true), f.update.WireUpdate.SourceCtxID())
	f.reactor.peers[src.Settings().PeerKey()] = src

	before := freeCount(f.peerPl)

	_, delivered := reactorForwardRS(f.reactor, f.update, f.id, srcAddr, src)
	require.Zero(t, delivered,
		"the only destination must fail its body build, or this test is not on the failure path")

	assert.Equal(t, before, freeCount(f.peerPl),
		"the Outgoing Peer Pool buffer must come back when the body build fails")
}
