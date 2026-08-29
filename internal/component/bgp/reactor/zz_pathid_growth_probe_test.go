package reactor

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The RFC 7911 identifier table measured at FULL-SIZE UPDATE scale, and the
// keying rule that bounds it.
//
// These two tests began as measurement probes for the Review Gate BLOCKER of
// plan/spec-rfc7911-generate-own-path-id.md, written before the release point
// was decided. They logged what they measured and asserted almost nothing. The
// release contract is settled now, so each one asserts the fact it used to
// print, and the logs stay because the numbers are what sized the fix.
//
// The companion file is forward_path_id_churn_test.go, which drives one path per
// UPDATE across many cycles. These two take the other two axes: one UPDATE
// carrying hundreds of withdrawn identifiers, and one received identifier
// meeting several prefixes.

const (
	// probeSourceFramed and probeSourceUnframed are this file's two ingress
	// peers. The table is a package global shared with every other test in this
	// binary, so each count below is read for one source id no other test uses.
	probeSourceFramed    source.SourceID = 4301
	probeSourceUnframed  source.SourceID = 4302
	probeSourceReencoded source.SourceID = 4303
)

// fwdPathIDTableSize counts every identifier the table holds, under both keys.
// A count that read bySource alone would report zero for exactly the population
// that can grow, which is the source that frames Path Identifiers.
func fwdPathIDTableSize() (entries, used int) {
	fwdPathIDs.mu.RLock()
	defer fwdPathIDs.mu.RUnlock()
	for _, perSource := range fwdPathIDs.bySource {
		entries += len(perSource)
	}
	for _, perSource := range fwdPathIDs.byPath {
		entries += len(perSource)
	}
	return entries, len(fwdPathIDs.used)
}

// probeWithdrawOnlyBody builds an UPDATE body whose only content is withdrawn routes,
// ADD-PATH framed, one per identifier in ids, all for the SAME prefix.
func probeWithdrawOnlyBody(ids []uint32, prefix netip.Prefix) []byte {
	var withdrawn []byte
	for _, id := range ids {
		var idBytes [4]byte
		binary.BigEndian.PutUint32(idBytes[:], id)
		withdrawn = append(withdrawn, idBytes[:]...)
		withdrawn = append(withdrawn, nlri.NewINET(family.IPv4Unicast, prefix, 0).Bytes()...)
	}
	body := make([]byte, 0, 4+len(withdrawn))
	var hdr [2]byte
	binary.BigEndian.PutUint16(hdr[:], uint16(len(withdrawn)))
	body = append(body, hdr[:]...)
	body = append(body, withdrawn...)
	body = append(body, 0, 0) // total path attribute length
	return body
}

// TestWithdrawOnlyUpdateFreesEveryIdentifierItBuys is the leak at the scale a
// peer can reach in one message.
//
// VALIDATES: AC-4 at full-size-UPDATE scale. One withdraw-only UPDATE naming 200
// identifiers ze never advertised buys 200 table entries while it is being
// relayed, and holds none of them once the cache evicts it.
// PREVENTS: the memory this spec's Review Gate opened on. The relay consults no
// RIB, so it rewrites a withdrawn section exactly as it rewrites an announced
// one, and every distinct identifier in it used to buy a permanent entry. The
// entry is 30 to 40 octets for a 5-octet withdraw NLRI, so a peer that sends
// these at line rate grows the daemon faster than it sends.
// RFC requirement: RFC7911-2-2 positive -- "the Path Identifier MUST be assigned
// in such a way that the BGP speaker is able to use the (Prefix, Path
// Identifier) to uniquely identify a path advertised to a neighbor". The
// obligation runs over the pairs ze currently advertises, so a value ze
// advertises no path under is free, and holding it forever buys nothing.
func TestWithdrawOnlyUpdateFreesEveryIdentifierItBuys(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	require.True(t, ctx.AddPath(family.IPv4Unicast))

	src := makeRSPeer(t, "10.0.0.1", 65001, ctx, ctxID)
	dst := makeRSPeer(t, "10.0.0.2", 65002, ctx, ctxID)

	const withdrawn = 200
	ids := make([]uint32, 0, withdrawn)
	for i := range withdrawn {
		ids = append(ids, uint32(1_000_000+i))
	}
	body := probeWithdrawOnlyBody(ids, netip.MustParsePrefix("10.9.0.0/24"))
	t.Logf("withdraw-only UPDATE: %d octets on the wire for %d withdrawn NLRIs", message.HeaderLen+len(body), withdrawn)

	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetSourceID(probeSourceFramed)
	wu.SetMessageID(4242)

	update := &ReceivedUpdate{
		WireUpdate:   wu,
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		ReceivedAt:   time.Now(),
	}
	cache := newRecentUpdateCache(100)
	t.Cleanup(cache.Stop)
	const consumer = "pathid-withdraw-only-probe"
	cache.RegisterConsumer(consumer)
	cache.Add(update)
	cache.Activate(4242, 1)
	t.Cleanup(func() { fwdPathIDs.releaseSource(probeSourceFramed) })

	// The handler counts and signals, and releases nothing: runWorker calls
	// item.done() itself once the handler returns (forward_pool.go), so a
	// handler that also called it would drive the retain count negative and
	// evict the entry under the measurement below.
	var mu sync.Mutex
	var dispatched int
	done := make(chan struct{}, 4)
	testPool := newFwdPool(func(_ fwdKey, items []fwdItem) {
		mu.Lock()
		dispatched += len(items)
		mu.Unlock()
		for range items {
			done <- struct{}{}
		}
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

	beforeEntries, beforeUsed := fwdPathIDTableSize()
	_, delivered := reactorForwardRS(r, update, 4242, netip.MustParseAddr("10.0.0.1"), src)
	require.Equal(t, 1, delivered, "the withdraw must reach the destination client")

	// Measured before anything can evict: reactorForwardRS builds every
	// destination body before it returns, and the entry it retained still has
	// this test's one cache consumer outstanding.
	relayedEntries, relayedUsed := fwdPathIDTableSize()
	t.Logf("fwdPathIDs entries %d -> %d (+%d), used %d -> %d (+%d)",
		beforeEntries, relayedEntries, relayedEntries-beforeEntries,
		beforeUsed, relayedUsed, relayedUsed-beforeUsed)
	t.Logf("octets of wire per entry: %.1f", float64(message.HeaderLen+len(body))/float64(max(relayedEntries-beforeEntries, 1)))

	// EXACT, on both sides. "Fewer than 200 survive" passes on a leak that is
	// merely slower, and "some were bought" passes on a relay that never ran.
	require.Equal(t, withdrawn, relayedEntries-beforeEntries,
		"a withdraw-only UPDATE must buy one entry per identifier it names, which is what makes the release the thing worth testing")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for dispatch")
	}
	require.NoError(t, cache.Ack(4242, consumer))

	// Eventually, not immediately: the ack drops this test's consumer, and the
	// worker drops the retain it took at dispatch. Eviction follows whichever
	// lands second, so the entry can still be in the cache for the microsecond
	// between the two.
	require.Eventually(t, func() bool { return !cache.Contains(4242) }, 2*time.Second, 5*time.Millisecond,
		"the entry must be evicted, because eviction is where the release runs")

	afterEntries, afterUsed := fwdPathIDTableSize()
	assert.Equal(t, beforeEntries, afterEntries,
		"the withdrawn identifiers outlived the UPDATE that carried them, so a peer grows this table from the socket")
	assert.Equal(t, beforeUsed, afterUsed,
		"the values did not return to the pool, so the counter must step over paths that no longer exist")
}

// TestPathIDKeyFollowsWhatTheSourceFramed is the keying rule the bound rests on.
//
// VALIDATES: AC-2, AC-4 -- how many prefixes one received identifier covers is
// decided by what the SOURCE framed. A source that frames identifiers names a
// path by (identifier, prefix), so two prefixes under one received value leave
// under two identifiers and each is freed by its own withdraw. A source that
// frames none names a path by its prefix alone and sends every one under
// identifier 0, so one identifier covers all of them and no withdraw may free
// it.
// PREVENTS: both halves of getting the key wrong. Key the framing source on the
// received identifier alone and one entry covers every prefix sent under it, so
// no withdraw can free that entry without renumbering the prefixes still
// advertised -- the reason "release on the relayed withdraw" was wrong before
// the key changed. Key the non-framing source per prefix instead and a
// full-table client costs one entry per route to bound a population that cannot
// grow.
// RFC requirement: RFC7911-2-2 positive -- "the Path Identifier MUST be assigned
// in such a way that the BGP speaker is able to use the (Prefix, Path
// Identifier) to uniquely identify a path advertised to a neighbor". Both
// keyings satisfy it, because uniqueness is asked of the PAIR: one identifier
// shared across prefixes still names one path per prefix.
func TestPathIDKeyFollowsWhatTheSourceFramed(t *testing.T) {
	destCtx, destCtxID := registerForwardBodyTestContext(t, true, true)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	// One received identifier for every prefix, which is what a source that
	// frames none sends and what a framing source is free to send too.
	const received = 0

	// emit forwards one prefix and returns the identifier the destination reads.
	emit := func(t *testing.T, srcCtxID bgpctx.ContextID, src source.SourceID, framed bool, prefix string) uint32 {
		t.Helper()
		wire := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0).Bytes()
		if framed {
			var idBytes [4]byte
			binary.BigEndian.PutUint32(idBytes[:], received)
			wire = append(idBytes[:], wire...)
		}
		body := buildRawUpdateBody(nil, forwardBodyBaseAttrs(t, 65001), [][]byte{wire})
		update := wireu.NewWireUpdate(body, srcCtxID)
		update.SetSourceID(src)
		result, ok := buildFwdBody(update, message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.10"), &fwdParseCache{})
		require.True(t, ok, "the UPDATE must forward")
		defer ReturnReadBuffer(result.transcodeBuf)
		return forwardedPathID(t, result)
	}

	t.Run("source frames identifiers", func(t *testing.T) {
		t.Cleanup(func() { fwdPathIDs.releaseSource(probeSourceFramed) })
		before, _ := fwdPathIDTableSize()

		first := emit(t, destCtxID, probeSourceFramed, true, "10.1.0.0/24")
		second := emit(t, destCtxID, probeSourceFramed, true, "10.2.0.0/24")
		after, _ := fwdPathIDTableSize()
		t.Logf("two prefixes under received identifier %d left as %d and %d", received, first, second)

		assert.NotEqual(t, first, second,
			"one identifier covers both prefixes, so withdrawing either one renumbers the other and strands it at the destination")
		assert.Equal(t, 2, after-before,
			"the two paths must hold one entry each, which is what lets a withdraw free exactly the path it withdraws")
	})

	t.Run("source frames identifiers, re-encode rail", func(t *testing.T) {
		// Same ADD-PATH answer for IPv4, a different context. buildFwdBody takes
		// its raw same-context branch only when the two context ids are equal, so
		// a destination that also reads IPv6 identifiers puts this forward on the
		// re-encode rail (fwdReencodeNLRIs) with both sides framing IPv4. That
		// branch has its own call into the generator and no other test reaches
		// it: keying it on the source alone survives every case above.
		srcCtx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true})
		srcCtxID, err := bgpctx.Registry.Register(srcCtx)
		require.NoError(t, err)
		wideCtx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true})
		wideCtxID, err := bgpctx.Registry.Register(wideCtx)
		require.NoError(t, err)
		require.NotEqual(t, srcCtxID, wideCtxID, "guard: the fixture must put the forward on the re-encode rail")
		require.True(t, wideCtx.AddPath(family.IPv4Unicast), "guard: the destination must still read IPv4 identifiers")

		widePeer := forwardBodyTestPeer(wideCtx, wideCtxID)
		reencode := func(prefix string) uint32 {
			t.Helper()
			var idBytes [4]byte
			binary.BigEndian.PutUint32(idBytes[:], received)
			wire := append(idBytes[:], nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0).Bytes()...)
			body := buildRawUpdateBody(nil, forwardBodyBaseAttrs(t, 65001), [][]byte{wire})
			update := wireu.NewWireUpdate(body, srcCtxID)
			update.SetSourceID(probeSourceReencoded)
			result, ok := buildFwdBody(update, message.MaxMsgLen, wideCtxID, widePeer, netip.MustParseAddr("192.0.2.11"), &fwdParseCache{})
			require.True(t, ok, "the UPDATE must forward")
			defer ReturnReadBuffer(result.transcodeBuf)
			require.Empty(t, result.rawBodies, "guard: a raw body means the forward took the same-context rail, not the re-encode one")
			return forwardedPathID(t, result)
		}

		t.Cleanup(func() { fwdPathIDs.releaseSource(probeSourceReencoded) })
		before, _ := fwdPathIDTableSize()

		first := reencode("10.5.0.0/24")
		second := reencode("10.6.0.0/24")
		after, _ := fwdPathIDTableSize()
		t.Logf("two prefixes re-encoded under received identifier %d left as %d and %d", received, first, second)

		assert.NotEqual(t, first, second,
			"the re-encode rail keyed both prefixes on the source alone, so withdrawing either one renumbers the other")
		assert.Equal(t, 2, after-before,
			"the two paths must hold one entry each on this rail too")
	})

	t.Run("source frames none", func(t *testing.T) {
		_, srcCtxID := registerForwardBodyTestContext(t, true, false)
		t.Cleanup(func() { fwdPathIDs.releaseSource(probeSourceUnframed) })
		before, _ := fwdPathIDTableSize()

		first := emit(t, srcCtxID, probeSourceUnframed, false, "10.3.0.0/24")
		second := emit(t, srcCtxID, probeSourceUnframed, false, "10.4.0.0/24")
		after, _ := fwdPathIDTableSize()
		t.Logf("two prefixes from a source that framed nothing left as %d and %d", first, second)

		assert.Equal(t, first, second,
			"such a source paid an identifier per prefix, so a full table costs one entry per route to bound a set that cannot grow")
		assert.Equal(t, 1, after-before,
			"one entry serves this source for its whole session, and nothing but peer removal ends it")
	})
}
