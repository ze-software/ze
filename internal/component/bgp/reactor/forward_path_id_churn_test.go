// Design: docs/architecture/bgp/structural-forwarding.md -- one egress transform, both rails
// RFC: rfc/short/rfc7911.md — Section 2, a re-advertised route carries the speaker's own Path Identifier
// Related: forward_path_id.go -- fwdPathIDTable and fwdReleaseWithdrawnPathIDs
// Related: forward_rs.go -- reactorForwardRS, the relay these tests drive
package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"
)

// The identifier table's bound, driven from the relay rather than from the
// table.
//
// The generator's own tests (forward_path_id_gen_test.go) reach fwdPathIDTable
// through buildFwdBody, which assigns and never frees: the free runs when the
// recent-update cache evicts the UPDATE that carried the withdraw, and only the
// relay puts an UPDATE in that cache. So these tests drive reactorForwardRS with
// a published cache entry and ack it, which is the sequence reactor_notify.go
// runs for every UPDATE a route-server client sends.
//
// The client they model is the one that costs memory: it negotiated ADD-PATH, so
// it frames a Path Identifier of its own and a new path arrives under a new
// value. A client that negotiated none sends every path under identifier 0 and
// costs one entry for its whole session, whatever it churns.

const (
	// fwdChurnSource is this file's ingress peer. The table is a package global
	// shared with every other test in this binary, so the counts below are read
	// for one source id that no other test uses.
	fwdChurnSource source.SourceID = 4201

	// fwdChurnConsumer is the cache consumer whose ack evicts an entry, which is
	// where the identifiers of a withdrawn path are freed.
	fwdChurnConsumer = "forward-path-id-churn-test"

	fwdChurnSourceAS = 65001
)

// fwdPathEntries counts the identifiers the table currently holds for this
// file's ingress peer. An EXACT count is the only useful assertion here: a bound
// stated as "fewer than N" passes on a leak that is merely slower.
func fwdPathEntries() int {
	fwdPathIDs.mu.RLock()
	defer fwdPathIDs.mu.RUnlock()
	return len(fwdPathIDs.byPath[fwdChurnSource])
}

// fwdChurnRail wires one source peer and one ADD-PATH destination behind the
// route-server rail, with the destination out of its initial sync so a
// dispatched item takes the direct write and this test stays deterministic.
func fwdChurnRail(t *testing.T) (*Reactor, *Peer, *Peer, *recordingConn, bgpctx.ContextID) {
	t.Helper()
	return fwdChurnRailWith(t, false)
}

// fwdChurnRailWith is fwdChurnRail with the destination made a route-reflector
// client of an IBGP source, which is one shape that puts the forward on the
// rebuild branch: RFC 4456 has it set ORIGINATOR_ID and prepend CLUSTER_LIST,
// so the destination reads bytes ze built rather than the ones the source sent.
func fwdChurnRailWith(t *testing.T, reflected bool) (*Reactor, *Peer, *Peer, *recordingConn, bgpctx.ContextID) {
	t.Helper()

	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	require.True(t, ctx.AddPath(family.IPv4Unicast), "guard: the fixture must negotiate ADD-PATH")

	cache := newRecentUpdateCache(100)
	t.Cleanup(cache.Stop)
	cache.RegisterConsumer(fwdChurnConsumer)

	src := makeForwardSourcePeer(t, ctx, ctxID)
	dst, conn := newSyncOrderDest(t, ctx, ctxID)
	dst.sendingInitialRoutes.Store(0)
	if reflected {
		src.settings.PeerAS = src.settings.LocalAS
		src.refreshForwardFacts()
		dst.settings.RouteReflectorClient = true
		dst.refreshForwardFacts()
	}

	pool := newFwdPool(fwdBatchHandler, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers: map[netip.AddrPort]*Peer{
			src.Settings().PeerKey(): src,
			dst.Settings().PeerKey(): dst,
		},
		fwdPool: pool,
	}
	dst.SetReactor(r)

	t.Cleanup(func() { fwdPathIDs.releaseSource(fwdChurnSource) })
	require.Equal(t, 0, fwdPathEntries(), "guard: the table holds nothing for this source yet")

	return r, src, dst, conn, ctxID
}

// fwdChurnRelay publishes one UPDATE as a session read does, relays it, and acks
// it. The ack is what evicts the cache entry, and eviction is where a withdrawn
// path's identifier is freed.
func fwdChurnRelay(t *testing.T, r *Reactor, src *Peer, ctxID bgpctx.ContextID, updateID uint64, body []byte) {
	t.Helper()

	wire := wireu.NewWireUpdate(body, ctxID)
	wire.SetMessageID(updateID)
	wire.SetSourceID(fwdChurnSource)
	update := &ReceivedUpdate{
		WireUpdate:   wire,
		SourcePeerIP: netip.MustParseAddr(forwardSourceAddr),
		ReceivedAt:   time.Now(),
	}
	r.recentUpdates.Add(update)
	r.recentUpdates.Activate(updateID, 1)

	_, dispatched := reactorForwardRS(r, update, updateID, netip.MustParseAddr(forwardSourceAddr), src)
	require.Equal(t, 1, dispatched, "the destination must be relayed to, not dropped")

	require.NoError(t, r.recentUpdates.Ack(updateID, fwdChurnConsumer))
	require.False(t, r.recentUpdates.Contains(updateID),
		"the entry must be evicted by the ack, because eviction is where the release runs")
}

// fwdChurnSent flushes the destination's write buffer and returns the Path
// Identifiers it has announced and withdrawn, in the order they were written.
func fwdChurnSent(t *testing.T, dst *Peer, conn *recordingConn) (announced, withdrawn []uint32) {
	t.Helper()

	dst.mu.RLock()
	session := dst.session
	dst.mu.RUnlock()
	require.NotNil(t, session, "guard: the destination must hold a session")

	session.mu.Lock()
	writer := session.bufWriter
	session.mu.Unlock()
	require.NoError(t, writer.Flush())

	raw := conn.written()
	for len(raw) >= message.HeaderLen {
		length := int(binary.BigEndian.Uint16(raw[16:18]))
		if length < message.HeaderLen || length > len(raw) {
			break
		}
		body := raw[message.HeaderLen:length]
		msgType := msgtype.MessageType(raw[18])
		raw = raw[length:]
		if msgType != msgtype.TypeUPDATE {
			continue
		}
		update, err := message.UnpackUpdate(body)
		require.NoError(t, err)
		announced = append(announced, fwdChurnPathIDs(t, update.NLRI)...)
		withdrawn = append(withdrawn, fwdChurnPathIDs(t, update.WithdrawnRoutes)...)
	}
	return announced, withdrawn
}

// fwdChurnPathIDs reads the Path Identifier of every NLRI in an ADD-PATH framed
// section.
func fwdChurnPathIDs(t *testing.T, section []byte) []uint32 {
	t.Helper()
	var out []uint32
	iter := nlri.NewNLRIIterator(section, true)
	for _, pathID, ok := iter.Next(); ok; _, pathID, ok = iter.Next() {
		out = append(out, pathID)
	}
	require.Zero(t, iter.Remaining(), "the destination section is malformed")
	return out
}

// TestForwardPathIDsFreedOnRelayedWithdraw is the memory bound.
//
// VALIDATES: AC-4 -- an identifier returns to the pool once ze has relayed the
// withdraw of the path that held it, so a client that churns identifiers costs
// the table nothing across a cycle.
// PREVENTS: the leak this spec's Review Gate opened on. Every distinct
// identifier a peer sends buys a table entry, and the only release was peer
// removal, so an established route-server client grew the daemon from the socket
// until the process was restarted.
// RFC requirement: RFC7911-2-2 positive -- "the Path Identifier MUST be assigned
// in such a way that the BGP speaker is able to use the (Prefix, Path
// Identifier) to uniquely identify a path advertised to a neighbor". A value is
// free to name a second path once no advertisement of the first one is
// outstanding, and the relayed withdraw is that point.
func TestForwardPathIDsFreedOnRelayedWithdraw(t *testing.T) {
	r, src, _, _, ctxID := fwdChurnRail(t)

	updateID := uint64(9100)
	for cycle := range 8 {
		// A new path from the same client arrives under a new identifier, which
		// is what a client with ADD-PATH send capability does as its paths
		// change. The table must not remember the ones it has stopped using.
		received := uint32(cycle) + 1

		updateID++
		fwdChurnRelay(t, r, src, ctxID, updateID, pathIDTestBody(t, fwdChurnSourceAS, received))
		require.Equal(t, 1, fwdPathEntries(),
			"cycle %d: the announced path must hold exactly one identifier", cycle)

		updateID++
		fwdChurnRelay(t, r, src, ctxID, updateID, fwdShapeBody(fwdPathIDNLRI(received), nil, nil))
		require.Equal(t, 0, fwdPathEntries(),
			"cycle %d: the withdrawn path kept its identifier, so the table grows by one for every identifier the client ever uses", cycle)
	}
}

// TestForwardPathIDWithdrawCarriesTheAnnouncedValue keeps the bound from
// stranding a route.
//
// VALIDATES: AC-3, AC-4 -- a path re-advertised while it is still advertised
// keeps its identifier, and the withdraw that ends it carries that same value.
// PREVENTS: a release that renumbers. A destination matches a withdraw on
// (prefix, Path Identifier), so an identifier that moved between the
// announcement and the withdraw leaves the route in that destination's table
// with nothing left to remove it.
// RFC requirement: RFC7911-2-2 positive -- ze's identifier names the path for as
// long as ze advertises it, the withdraw included.
func TestForwardPathIDWithdrawCarriesTheAnnouncedValue(t *testing.T) {
	r, src, dst, conn, ctxID := fwdChurnRail(t)

	const received = 0x0BADC0DE
	fwdChurnRelay(t, r, src, ctxID, 9200, pathIDTestBody(t, fwdChurnSourceAS, received))
	// The same path again, with a different AS_PATH: a replacement, not a second
	// path (RFC 7911 Section 5).
	fwdChurnRelay(t, r, src, ctxID, 9201, pathIDTestBody(t, fwdChurnSourceAS+9, received))
	require.Equal(t, 1, fwdPathEntries(),
		"a re-advertised path must reuse its entry rather than buy a second one")

	fwdChurnRelay(t, r, src, ctxID, 9202, fwdShapeBody(fwdPathIDNLRI(received), nil, nil))
	require.Equal(t, 0, fwdPathEntries())

	announced, withdrawn := fwdChurnSent(t, dst, conn)
	require.Len(t, announced, 2, "the destination must have received both advertisements")
	require.Len(t, withdrawn, 1, "the destination must have received one withdraw")
	assert.Equal(t, announced[0], announced[1],
		"the re-advertised path left under a second identifier, so the destination keeps the superseded one forever")
	assert.Equal(t, announced[0], withdrawn[0],
		"the withdraw left under an identifier the destination never received, so the route stays")
	assert.NotEqual(t, uint32(received), withdrawn[0],
		"the withdraw carries the source's identifier, which ze does not own")
}

// TestForwardPathIDWithdrawOfUnknownPathLeavesNothing closes the cheapest growth
// vector.
//
// VALIDATES: AC-4 -- a withdraw for a path ze never advertised costs the table
// nothing once the UPDATE that carried it is evicted.
// PREVENTS: growth from withdrawn NLRI. The relay never consults a RIB, so it
// rewrites the identifiers of a withdrawn section exactly as it rewrites an
// announced one, and each distinct value in it used to buy a permanent entry. A
// full-size UPDATE carries hundreds of them and needs no announcement first.
// RFC requirement: RFC7911-5-4 positive -- "If a BGP speaker receives a message
// to withdraw a prefix with a Path Identifier not seen before, it SHOULD
// silently ignore it", which is what the destination does with the identifier ze
// mints for such a withdraw and immediately frees.
func TestForwardPathIDWithdrawOfUnknownPathLeavesNothing(t *testing.T) {
	r, src, _, _, ctxID := fwdChurnRail(t)

	updateID := uint64(9300)
	for cycle := range 8 {
		updateID++
		fwdChurnRelay(t, r, src, ctxID, updateID, fwdShapeBody(fwdPathIDNLRI(uint32(cycle)+1), nil, nil))
		require.Equal(t, 0, fwdPathEntries(),
			"withdraw %d for a path ze never advertised left an entry behind", cycle)
	}
}

// TestForwardPathIDKeepsTheSourceOfARebuiltFrame guards the key itself.
//
// VALIDATES: AC-2, AC-7 -- a destination that reads bytes ze rebuilt gets the
// identifier of the path's real source, so two clients still separate and the
// entry is still the one the withdraw frees.
// PREVENTS: the route-server rail dropping the source when it rebuilds a frame.
// The identifier is keyed on the ingress path, so a rebuilt wire that lost its
// source keys every client's paths under the singleton config source: two
// clients that both chose identifier 1 for different prefixes reach the
// destination under ONE identifier and RFC 7911 Section 5 makes the second
// replace the first. The withdraw then frees a key nothing holds, and the entry
// the rebuild made lives until the peer is removed.
// RFC requirement: RFC7911-2-2 positive -- "A BGP speaker that re-advertises a
// route MUST generate its own Path Identifier", and the path it generates one
// for is the one the source sent, whatever ze did to the bytes on the way out.
func TestForwardPathIDKeepsTheSourceOfARebuiltFrame(t *testing.T) {
	r, src, _, _, ctxID := fwdChurnRailWith(t, true)

	const received = 7
	fwdChurnRelay(t, r, src, ctxID, 9400, pathIDTestBody(t, fwdChurnSourceAS, received))
	require.Equal(t, 1, fwdPathEntries(),
		"the rebuilt frame keyed its path under another source, so this client's paths share one identifier with every other client's")

	fwdChurnRelay(t, r, src, ctxID, 9401, fwdShapeBody(fwdPathIDNLRI(received), nil, nil))
	require.Equal(t, 0, fwdPathEntries(),
		"the withdraw freed a key this path never held")
}

// TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt guards the one
// exception the release makes.
//
// VALIDATES: AC-3, AC-4 -- a pair the same UPDATE withdraws and announces stays
// advertised, so it keeps the identifier that names it at the destination.
// PREVENTS: a release that frees a pair the destination still holds. RFC 7606
// Section 5.1 forbids a conforming sender to put both fields in one UPDATE, and
// ze must still accept one that does: the destination ends holding the pair,
// because the announcement of a pair supersedes its withdraw. Freeing ze's
// identifier there mints a fresh one at the next re-advertisement, and the
// destination keeps the superseded (prefix, identifier) with nothing left that
// can remove it.
// RFC requirement: RFC7911-2-2 positive -- "the Path Identifier MUST be assigned
// in such a way that the BGP speaker is able to use the (Prefix, Path
// Identifier) to uniquely identify a path advertised to a neighbor". A pair that
// is still advertised is still ze's to name, whatever else the UPDATE carrying
// it withdrew.
func TestForwardPathIDKeptWhenOneUpdateWithdrawsAndAnnouncesIt(t *testing.T) {
	r, src, dst, conn, ctxID := fwdChurnRail(t)

	// Two paths for one prefix, which is what ADD-PATH exists to carry.
	const kept = 0x11111111
	const gone = 0x22222222

	fwdChurnRelay(t, r, src, ctxID, 9500, pathIDTestBody(t, fwdChurnSourceAS, kept))
	fwdChurnRelay(t, r, src, ctxID, 9501, pathIDTestBody(t, fwdChurnSourceAS, gone))
	require.Equal(t, 2, fwdPathEntries(), "guard: the source advertises two paths for the prefix")

	// One UPDATE withdrawing both pairs and announcing one of them back.
	both := append(fwdPathIDNLRI(kept), fwdPathIDNLRI(gone)...)
	fwdChurnRelay(t, r, src, ctxID, 9502,
		fwdShapeBody(both, forwardBodyBaseAttrs(t, fwdChurnSourceAS), fwdPathIDNLRI(kept)))
	require.Equal(t, 1, fwdPathEntries(),
		"the pair the same UPDATE announced was freed, so ze no longer names a path the destination holds")

	// The re-advertisement is what makes the loss visible: it must leave under
	// the identifier the destination already has for this pair.
	fwdChurnRelay(t, r, src, ctxID, 9503, pathIDTestBody(t, fwdChurnSourceAS, kept))

	announced, withdrawn := fwdChurnSent(t, dst, conn)
	require.Len(t, announced, 4, "the destination must have received four announcements")
	require.Len(t, withdrawn, 2, "the destination must have received two withdraws")
	assert.Equal(t, announced[0], announced[2],
		"the pair survived its own UPDATE's withdraw, so it must not be renumbered inside it")
	assert.Equal(t, announced[0], announced[3],
		"the re-advertised pair left under a second identifier, so the destination keeps the first one forever")
	assert.NotEqual(t, announced[0], announced[1],
		"guard: the two paths of one prefix must reach the destination under different identifiers")
}
