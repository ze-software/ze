// RFC: rfc/short/rfc7911.md — Section 2, a re-advertised route carries the speaker's own Path Identifier
package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The generator's tests. The reproduction that opened
// plan/spec-rfc7911-generate-own-path-id.md lives beside this file in
// forward_path_id_test.go and drives the same entry point; these cover the
// halves a reproduction does not: the withdraw, the source that negotiated no
// ADD-PATH, the destination that negotiated none, the boundary values, and the
// agreement between destinations that the replay rail inherits.
//
// Every fixture sets a SourceID on its WireUpdate, which is what the receive
// path does to every UPDATE it accepts (session_read.go, sourceID is set before
// the update is published). The identifier ze assigns is keyed on that source
// and on the identifier the source used, so a fixture that leaves it unset is
// not two peers, it is one peer replacing its own path.

const (
	fwdTestSourceA source.SourceID = 4101
	fwdTestSourceB source.SourceID = 4102
)

// TestForwardPathIDDiffersForTwoSourcePeers is the route-server route loss.
//
// VALIDATES: AC-2 -- two peers that both chose Path Identifier 1 for one prefix
// reach a third ADD-PATH peer under DIFFERENT identifiers, so it keeps both.
// PREVENTS: relaying the source's identifier, which makes the two paths one
// (prefix, Path Identifier) pair and the second a replacement for the first
// (RFC 7911 Section 5).
// RFC requirement: RFC7911-2-2 positive -- "A BGP speaker that re-advertises a
// route MUST generate its own Path Identifier to be associated with the
// re-advertised route." Path Identifiers are chosen by each peer alone, so two
// peers choosing 1 is ordinary rather than unlucky.
func TestForwardPathIDDiffersForTwoSourcePeers(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	require.True(t, ctx.AddPath(family.IPv4Unicast), "destination must have ADD-PATH negotiated")
	peer := forwardBodyTestPeer(ctx, ctxID)

	const collidingPathID = 1
	fromA := fwdPathIDWire(pathIDTestBody(t, 65001, collidingPathID), ctxID, fwdTestSourceA)
	fromB := fwdPathIDWire(pathIDTestBody(t, 65002, collidingPathID), ctxID, fwdTestSourceB)

	first := fwdForwardOnePathID(t, fromA, ctxID, peer)
	second := fwdForwardOnePathID(t, fromB, ctxID, peer)

	assert.NotEqual(t, first, second,
		"both peers' paths for 10.0.0.0/24 left under Path Identifier %d, so the destination sees one path where two were sent", first)
}

// TestForwardPathIDMatchesAnnounceAndWithdraw is the other half of the fix.
//
// VALIDATES: AC-4 -- the identifier a withdrawn route carries is the one its
// announcement carried.
// PREVENTS: a generator wired into the announcement alone. The receiver matches
// a withdraw on (prefix, Path Identifier) (RFC 7911 Section 5), so a withdraw
// carrying the SOURCE's identifier names a path the receiver never heard of and
// SHOULD be silently ignored: the route never leaves, and the route loss this
// spec fixes is traded for a route that cannot be removed.
// RFC requirement: RFC7911-2-2 positive -- the identifier ze generates
// identifies the path for as long as ze advertises it, withdraw included.
func TestForwardPathIDMatchesAnnounceAndWithdraw(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	peer := forwardBodyTestPeer(ctx, ctxID)

	const receivedPathID = 0x0BADC0DE
	announce := fwdPathIDWire(pathIDTestBody(t, 65001, receivedPathID), ctxID, fwdTestSourceA)
	withdraw := fwdPathIDWire(fwdShapeBody(fwdPathIDNLRI(receivedPathID), nil, nil), ctxID, fwdTestSourceA)

	announced := fwdForwardOnePathID(t, announce, ctxID, peer)
	withdrawn := fwdForwardOneWithdrawnPathID(t, withdraw, ctxID, peer)

	assert.Equal(t, announced, withdrawn,
		"the withdraw left under a different Path Identifier from the announcement, so the receiver cannot match it and the route stays")
	assert.NotEqual(t, uint32(receivedPathID), withdrawn,
		"the withdraw carries the source's identifier, which ze does not own")
}

// TestForwardPathIDSurvivesAttributeChange guards the replacement.
//
// VALIDATES: AC-3 -- one path re-advertised with changed attributes keeps its
// identifier.
// PREVENTS: keying the identifier on the attribute bytes. RFC 7911 Section 5:
// "a new advertisement for a given address prefix and a given Path Identifier
// replaces a previous advertisement for the same address prefix and Path
// Identifier". A source that re-announces a path under the identifier it
// already used is replacing that path, so a fresh identifier here would leave
// the superseded path in the destination's table with nothing left to withdraw
// it.
// RFC requirement: RFC7911-2-2 positive -- ze's identifier names the path, and
// the path is what the source identified, not the attributes it last carried.
func TestForwardPathIDSurvivesAttributeChange(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	peer := forwardBodyTestPeer(ctx, ctxID)

	const receivedPathID = 77
	firstAttrs := fwdPathIDWire(pathIDTestBody(t, 65001, receivedPathID), ctxID, fwdTestSourceA)
	// Same peer, same prefix, same identifier, a different AS_PATH: the ordinary
	// shape of a path whose attributes moved.
	changedAttrs := fwdPathIDWire(pathIDTestBody(t, 65010, receivedPathID), ctxID, fwdTestSourceA)

	before := fwdForwardOnePathID(t, firstAttrs, ctxID, peer)
	after := fwdForwardOnePathID(t, changedAttrs, ctxID, peer)

	assert.Equal(t, before, after,
		"the same path left under two Path Identifiers, so the destination keeps the superseded one forever")
}

// TestForwardPathIDSeparatesNonAddPathSources covers the ordinary mixed
// deployment.
//
// VALIDATES: AC-1, AC-5 -- a source that negotiated no ADD-PATH sends every
// path with no identifier at all, and each such source's paths still reach an
// ADD-PATH destination under an identifier of their own.
// PREVENTS: the re-encode rail writing 0 for every prefix of every such source,
// which is what nlri.NLRIIterator reports when it is not reading ADD-PATH
// framing. Every path from every non-ADD-PATH client then shares the pair
// (prefix, 0) at the destination, and only one survives.
// RFC requirement: RFC7911-2-2 positive -- the re-advertised identifier is ze's
// own for a source that never sent one.
func TestForwardPathIDSeparatesNonAddPathSources(t *testing.T) {
	_, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, true, true)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	body := buildRawUpdateBody(nil, forwardBodyBaseAttrs(t, 65001), [][]byte{fwdPathIDBareNLRI()})
	fromA := fwdPathIDWire(body, srcCtxID, fwdTestSourceA)
	fromB := fwdPathIDWire(body, srcCtxID, fwdTestSourceB)

	first := fwdForwardOnePathID(t, fromA, destCtxID, peer)
	second := fwdForwardOnePathID(t, fromB, destCtxID, peer)
	again := fwdForwardOnePathID(t, fwdPathIDWire(body, srcCtxID, fwdTestSourceA), destCtxID, peer)

	assert.NotEqual(t, first, second,
		"two clients that sent no Path Identifier both left under %d, so the destination sees one path", first)
	assert.Equal(t, first, again,
		"one client's path left under two identifiers, so a refresh reads as a new path")
}

// TestForwardPathIDBoundaryReceivedValues covers the numeric edges of the
// received field.
//
// VALIDATES: AC-5 -- 0 and 2^32-1 are legal received values and neither is
// treated as unset or reserved.
// PREVENTS: a generator that reads 0 as "no identifier" and passes it through,
// or that overflows on the maximum. RFC 7911 Section 3 gives the field four
// octets and reserves no value in it.
// RFC requirement: RFC7911-2-2 positive -- every received value, edges
// included, is replaced by ze's own.
func TestForwardPathIDBoundaryReceivedValues(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	peer := forwardBodyTestPeer(ctx, ctxID)

	for name, received := range map[string]uint32{"zero": 0, "max_uint32": ^uint32(0)} {
		t.Run(name, func(t *testing.T) {
			wire := fwdPathIDWire(pathIDTestBody(t, 65001, received), ctxID, fwdTestSourceB)
			got := fwdForwardOnePathID(t, wire, ctxID, peer)
			assert.NotEqual(t, received, got,
				"the received Path Identifier %d was relayed rather than replaced", received)

			same := fwdForwardOnePathID(t, fwdPathIDWire(pathIDTestBody(t, 65001, received), ctxID, fwdTestSourceB), ctxID, peer)
			assert.Equal(t, got, same, "the identifier for one path must not move between UPDATEs")
		})
	}
}

// TestForwardPathIDIdenticalForEveryDestination is the rail-agreement
// invariant.
//
// VALIDATES: AC-7 -- one path leaves under one identifier whatever destination
// reads it, so a peer-up replay and a live forward of that path are the same
// bytes.
// PREVENTS: keying the identifier on the destination. The replay rail and the
// live rail reach this function with the same source UPDATE and different
// destinations, so a destination-keyed identifier would make a replayed route
// differ from the live one for the same path -- the divergence
// spec-fixit-bgp-egress-rail-divergence closed.
// RFC requirement: RFC7911-2-2 positive -- ze's identifier belongs to the path,
// not to the conversation it is sent in.
func TestForwardPathIDIdenticalForEveryDestination(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	first := forwardBodyTestPeer(ctx, ctxID)
	second := forwardBodyTestPeer(ctx, ctxID)

	wire := fwdPathIDWire(pathIDTestBody(t, 65001, 12), ctxID, fwdTestSourceA)

	toFirst, ok := buildFwdBody(wire, message.MaxMsgLen, ctxID, first, netip.MustParseAddr("192.0.2.40"), &fwdParseCache{})
	require.True(t, ok)
	defer ReturnReadBuffer(toFirst.transcodeBuf)
	toSecond, ok := buildFwdBody(wire, message.MaxMsgLen, ctxID, second, netip.MustParseAddr("192.0.2.41"), &fwdParseCache{})
	require.True(t, ok)
	defer ReturnReadBuffer(toSecond.transcodeBuf)

	require.Len(t, toFirst.rawBodies, 1, "guard: the fixture must produce one frame per destination")
	require.Len(t, toSecond.rawBodies, 1)
	assert.Equal(t, toFirst.rawBodies[0], toSecond.rawBodies[0],
		"one path reached two destinations as different bytes, so a replay cannot match a live forward")
}

// TestForwardPathIDLeavesNonAddPathDestinationAlone keeps the cost off the
// sessions that cannot use it.
//
// VALIDATES: AC-6 -- a destination that negotiated no ADD-PATH receives the
// source frame unchanged, and the forward borrows nothing to do it.
// PREVENTS: paying the rewrite, and its pooled copy, on every session in a
// deployment that negotiated ADD-PATH nowhere.
// RFC requirement: RFC7911-5-3 negative -- with no ADD-PATH negotiated for the
// <AFI, SAFI>, ze follows the RFC 4271 procedures and emits no Path Identifier
// field.
func TestForwardPathIDLeavesNonAddPathDestinationAlone(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, false)
	peer := forwardBodyTestPeer(ctx, ctxID)

	body := buildRawUpdateBody(nil, forwardBodyBaseAttrs(t, 65001), [][]byte{fwdPathIDBareNLRI()})
	wire := fwdPathIDWire(body, ctxID, fwdTestSourceA)

	result, ok := buildFwdBody(wire, message.MaxMsgLen, ctxID, peer, netip.MustParseAddr("192.0.2.42"), &fwdParseCache{})
	require.True(t, ok)

	require.Len(t, result.rawBodies, 1, "a same-context forward must stay on the raw path")
	assert.Equal(t, &body[0], &result.rawBodies[0][0],
		"the frame was copied for a destination that reads no Path Identifiers, so the zero-copy forward was lost")
	assert.Nil(t, result.transcodeBuf.Buf, "nothing may be borrowed when nothing is rewritten")
}

// TestForwardPathIDReleaseReturnsValues covers the table itself.
//
// VALIDATES: AC-4 -- an identifier returns to the pool when its source's paths
// are gone, and not before.
// PREVENTS: unbounded growth from a peer that is removed and re-added, and the
// reverse failure of handing a live path's identifier to a second path.
// RFC requirement: RFC7911-2-2 positive -- "the Path Identifier MUST be
// assigned in such a way that the BGP speaker is able to use the (Prefix, Path
// Identifier) to uniquely identify a path advertised to a neighbor", which a
// value issued twice at once would break.
func TestForwardPathIDReleaseReturnsValues(t *testing.T) {
	table := newFwdPathIDTable()

	held := table.generate(fwdTestSourceA, 1)
	other := table.generate(fwdTestSourceB, 1)
	require.NotEqual(t, held, other, "two sources must not share one identifier")
	assert.Equal(t, held, table.generate(fwdTestSourceA, 1), "a known path keeps its identifier")

	table.releaseSource(fwdTestSourceA)
	assert.NotContains(t, table.bySource, fwdTestSourceA, "a released source keeps no entries")
	assert.NotContains(t, table.used, held, "a released identifier stays out of the live set")
	assert.Contains(t, table.used, other, "releasing one source must not free another's identifier")

	// A wrapped counter must step over the identifier the surviving source still
	// holds rather than issue it twice.
	table.next = other
	assert.NotEqual(t, other, table.generate(fwdTestSourceA, 2),
		"the counter reissued an identifier a live path holds")
}

// fwdPathIDWire wraps a body as the receive path does: the source that sent it
// is on the WireUpdate before anything forwards it (session_read.go).
func fwdPathIDWire(body []byte, ctxID bgpctx.ContextID, src source.SourceID) *wireu.WireUpdate {
	wire := wireu.NewWireUpdate(body, ctxID)
	wire.SetSourceID(src)
	return wire
}

// fwdPathIDNLRI builds one ADD-PATH framed 10.0.0.0/24 entry under the given
// Path Identifier.
func fwdPathIDNLRI(pathID uint32) []byte {
	var id [4]byte
	binary.BigEndian.PutUint32(id[:], pathID)
	return append(id[:], fwdPathIDBareNLRI()...)
}

// fwdPathIDBareNLRI builds 10.0.0.0/24 with no Path Identifier, which is what a
// source that negotiated no ADD-PATH sends.
func fwdPathIDBareNLRI() []byte {
	return nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0).Bytes()
}

// fwdForwardOnePathID forwards one UPDATE and returns the single Path
// Identifier the destination frame announces.
func fwdForwardOnePathID(t *testing.T, wire *wireu.WireUpdate, destCtxID bgpctx.ContextID, peer *Peer) uint32 {
	t.Helper()
	result, ok := buildFwdBody(wire, message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.43"), &fwdParseCache{})
	require.True(t, ok, "the UPDATE must forward")
	t.Cleanup(func() { ReturnReadBuffer(result.transcodeBuf) })
	return forwardedPathID(t, result)
}

// fwdForwardOneWithdrawnPathID forwards one UPDATE and returns the single Path
// Identifier the destination frame withdraws.
func fwdForwardOneWithdrawnPathID(t *testing.T, wire *wireu.WireUpdate, destCtxID bgpctx.ContextID, peer *Peer) uint32 {
	t.Helper()
	result, ok := buildFwdBody(wire, message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.44"), &fwdParseCache{})
	require.True(t, ok, "the withdraw must forward")
	t.Cleanup(func() { ReturnReadBuffer(result.transcodeBuf) })

	var withdrawn []byte
	switch {
	case len(result.rawBodies) > 0:
		require.Len(t, result.rawBodies, 1, "the fixture must produce one destination frame")
		update, err := message.UnpackUpdate(result.rawBodies[0])
		require.NoError(t, err)
		withdrawn = update.WithdrawnRoutes
	default:
		require.Len(t, result.updates, 1, "the fixture must produce one destination frame")
		withdrawn = result.updates[0].WithdrawnRoutes
	}

	iter := nlri.NewNLRIIterator(withdrawn, true)
	_, pathID, ok := iter.Next()
	require.True(t, ok, "the destination frame withdraws nothing")
	_, _, more := iter.Next()
	require.False(t, more, "the fixture must withdraw one prefix")
	require.Zero(t, iter.Remaining(), "the destination withdrawn section is malformed")
	return pathID
}
