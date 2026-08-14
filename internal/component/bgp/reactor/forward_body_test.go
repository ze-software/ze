package reactor

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForwardSplitConvertsAddPathContext(t *testing.T) {
	// VALIDATES: AC-1, AC-4, AC-5 -- mismatched ADD-PATH contexts are converted before shared helper splitting.
	// PREVENTS: Forwarding source-context path IDs to a destination peer that did not negotiate RFC 7911 ADD-PATH.
	// RFC requirement: RFC7911-5-3 negative -- the destination peer has not negotiated ADD-PATH (dest ctx addPath=false) while the source has, so buildFwdBody strips the 4-octet Path IDs and emits plain RFC 4271 NLRI (each destination prefix is 4 bytes, no path id).
	srcCtx, srcCtxID := registerForwardBodyTestContext(t, true, true)
	destCtx, destCtxID := registerForwardBodyTestContext(t, true, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	attrs := forwardBodyBaseAttrs(t, 65000)
	sourceNLRI := forwardBodyNLRIs(80, true)
	rawBody := buildRawUpdateBody(nil, attrs, [][]byte{sourceNLRI})
	require.Greater(t, message.HeaderLen+len(rawBody), 180, "source UPDATE must force split")

	result, ok := buildFwdBody(wireu.NewWireUpdate(rawBody, srcCtxID), 180, destCtxID, peer, netip.MustParseAddr("192.0.2.2"), &fwdParseCache{})
	require.True(t, ok)
	require.Empty(t, result.rawBodies, "mismatched contexts must not use raw source bodies")
	require.Greater(t, len(result.updates), 1, "destination-encoded update should split")

	var gotNLRI []byte
	for i, update := range result.updates {
		require.LessOrEqual(t, update.Len(nil), 180, "chunk %d exceeds destination max", i)
		gotNLRI = append(gotNLRI, update.NLRI...)
		iter := nlri.NewNLRIIterator(update.NLRI, false)
		for prefix, _, ok := iter.Next(); ok; prefix, _, ok = iter.Next() {
			require.Len(t, prefix, 4, "destination NLRI should be plain /24 without a path ID")
		}
		require.Zero(t, iter.Remaining(), "chunk %d has malformed destination NLRI", i)
	}
	assert.Equal(t, forwardBodyNLRIs(80, false), gotNLRI)
	assert.NotEqual(t, srcCtx.AddPath(family.IPv4Unicast), destCtx.AddPath(family.IPv4Unicast))
}

func TestForwardSplitConvertsASN4Context(t *testing.T) {
	// VALIDATES: AC-2, AC-4, AC-5 -- shared forwarding helper transcodes ASN4 source attributes before destination splitting.
	// PREVENTS: Sending four-octet AS_PATH wire bytes to a two-octet-AS destination peer.
	_, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	attrs := forwardBodyBaseAttrs(t, 200000)
	rawBody := buildRawUpdateBody(nil, attrs, [][]byte{forwardBodyNLRIs(70, false)})

	result, ok := buildFwdBody(wireu.NewWireUpdate(rawBody, srcCtxID), 170, destCtxID, peer, netip.MustParseAddr("192.0.2.3"), &fwdParseCache{})
	// Stand in for the caller's adoptFwdHandle: the ASN4 mismatch borrows a
	// read-pool buffer that production returns at cache eviction.
	defer ReturnReadBuffer(result.transcodeBuf)
	require.True(t, ok)
	require.Empty(t, result.rawBodies)
	require.Greater(t, len(result.updates), 1)

	for i, update := range result.updates {
		require.LessOrEqual(t, update.Len(nil), 170, "chunk %d exceeds destination max", i)
		asPath := forwardBodyFindASPath(t, update.PathAttributes, false)
		require.NotNil(t, asPath, "chunk %d missing AS_PATH", i)
		require.Len(t, asPath.Segments, 1)
		assert.Equal(t, []uint32{attribute.ASTrans}, asPath.Segments[0].ASNs)
		as4Path := forwardBodyFindAS4Path(t, update.PathAttributes)
		require.NotNil(t, as4Path, "chunk %d missing AS4_PATH", i)
		require.Len(t, as4Path.Segments, 1)
		assert.Equal(t, []uint32{200000}, as4Path.Segments[0].ASNs)
	}
}

func TestForwardDoesNotRetranscodeASN2RewrittenWire(t *testing.T) {
	// VALIDATES: EBGP ASN4->ASN2 rewrites update the wire context before buildFwdBody.
	// PREVENTS: Re-parsing an already 2-octet AS_PATH as 4-octet and dropping the destination.
	_, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	attrs := forwardBodyBaseAttrs(t, 65001)
	rawBody := buildRawUpdateBody(nil, attrs, [][]byte{forwardBodyNLRIs(1, false)})
	received := &ReceivedUpdate{WireUpdate: wireu.NewWireUpdate(rawBody, srcCtxID)}
	rewritten, err := received.EBGPWire(65000, true, false)
	require.NoError(t, err)

	result, ok := buildFwdBody(rewritten, message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.5"), &fwdParseCache{})
	require.True(t, ok, "already rewritten ASN2 wire must not be transcoded again")

	var forwarded *message.Update
	if len(result.rawBodies) > 0 {
		var err error
		forwarded, err = message.UnpackUpdate(result.rawBodies[0])
		require.NoError(t, err)
	} else {
		require.Len(t, result.updates, 1)
		forwarded = result.updates[0]
	}

	asPath := forwardBodyFindASPath(t, forwarded.PathAttributes, false)
	require.NotNil(t, asPath)
	require.Len(t, asPath.Segments, 1)
	assert.Equal(t, []uint32{65000, 65001}, asPath.Segments[0].ASNs)
}

func TestForwardSplitSameContextKeepsRawSplit(t *testing.T) {
	// VALIDATES: AC-3 -- same source/destination ContextID keeps the raw split branch before parsing.
	// PREVENTS: Regressing same-context forwarding into parsed UPDATE allocation and SendUpdate dispatch.
	// RFC requirement: RFC7911-5-3 positive -- source and destination share an ADD-PATH-negotiated context (both addPath=true), so ze generates the route update from the combination of the address prefix and the Path Identifier and emits the extended NLRI encoding: every emitted NLRI carries a 4-octet Path Identifier field ahead of the prefix the source sent.
	//
	// rfc-test-change-approved: 2026-08-14 Thomas approved correcting the
	// assertion and the tag text of this test. It read "the emitted NLRI bytes
	// equal the source NLRI including their 4-octet Path IDs", which pinned the
	// RFC 7911 Section 2 violation this file's sibling reproduces: a speaker that
	// re-advertises a route MUST generate its own Path Identifier. RFC7911-5-3
	// says a speaker follows RFC 4271 procedures unless ADD-PATH is negotiated
	// both ways, and says nothing about preserving a received identifier, so the
	// old text read a requirement the RFC does not carry. What this test
	// legitimately proves is that the same-context branch stays on the raw-split
	// fast path, and it still proves it: the assertions on rawBodies are
	// untouched, and the identifiers are checked as ze's own instead of the
	// source's.
	ctx, ctxID := registerForwardBodyTestContext(t, true, true)
	peer := forwardBodyTestPeer(ctx, ctxID)

	attrs := forwardBodyBaseAttrs(t, 65000)
	sourceNLRI := forwardBodyNLRIs(80, true)
	rawBody := buildRawUpdateBody(nil, attrs, [][]byte{sourceNLRI})

	result, ok := buildFwdBody(wireu.NewWireUpdate(rawBody, ctxID), 180, ctxID, peer, netip.MustParseAddr("192.0.2.4"), &fwdParseCache{})
	require.True(t, ok)
	require.Empty(t, result.updates, "same-context oversized forwarding must keep rawBodies")
	require.Greater(t, len(result.rawBodies), 1)

	var gotNLRI []byte
	for i, body := range result.rawBodies {
		require.LessOrEqual(t, message.HeaderLen+len(body), 180, "raw chunk %d exceeds max", i)
		update, err := message.UnpackUpdate(body)
		require.NoError(t, err)
		gotNLRI = append(gotNLRI, update.NLRI...)
	}

	// rfc-test-change-approved: 2026-08-14 Thomas approved replacing a byte
	// equality between the emitted NLRI and the source NLRI. That equality
	// required the emitted Path Identifiers to be the source's, and so pinned
	// the RFC 7911 Section 2 violation: "A BGP speaker that re-advertises a
	// route MUST generate its own Path Identifier to be associated with the
	// re-advertised route". The extended encoding claim RFC7911-5-3 does make is
	// kept, split in two: the prefixes are the source's, in order, and each
	// still carries a 4-octet identifier field ahead of it, now holding ze's
	// value rather than the source's.
	gotIDs, gotPrefixes := forwardBodySplitNLRI(t, gotNLRI)
	sourceIDs, sourcePrefixes := forwardBodySplitNLRI(t, sourceNLRI)
	assert.Equal(t, sourcePrefixes, gotPrefixes,
		"the extended encoding must carry the source's prefixes, in order, one per Path Identifier")
	assert.NotEqual(t, sourceIDs, gotIDs,
		"the emitted Path Identifiers are the source's, so ze is advertising values it does not own (RFC 7911 Section 2)")
	assert.Len(t, forwardBodyUniqueIDs(gotIDs), len(gotIDs),
		"two paths left under one Path Identifier, so the destination sees fewer paths than were sent")
}

// forwardBodySplitNLRI splits ADD-PATH framed NLRI bytes into the Path
// Identifiers and the prefixes they carry, so a test can assert on each half
// without the other.
func forwardBodySplitNLRI(t *testing.T, data []byte) (ids []uint32, prefixes [][]byte) {
	t.Helper()
	iter := nlri.NewNLRIIterator(data, true)
	for prefix, pathID, ok := iter.Next(); ok; prefix, pathID, ok = iter.Next() {
		ids = append(ids, pathID)
		prefixes = append(prefixes, prefix)
	}
	require.Zero(t, iter.Remaining(), "malformed ADD-PATH NLRI")
	require.NotEmpty(t, ids, "guard: the fixture carried no NLRI, so nothing was compared")
	return ids, prefixes
}

// forwardBodyUniqueIDs collapses Path Identifiers to the set of values used, so
// a caller can compare that count against the number of paths sent.
func forwardBodyUniqueIDs(ids []uint32) map[uint32]struct{} {
	seen := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return seen
}

// crossContextTranscodeBody builds a source UPDATE that fits, carries a 4-byte
// ASN, and therefore drives the RFC 6793 transcode branch of
// fwdUpdateForDestination when forwarded to a 2-byte-ASN destination context.
func crossContextTranscodeBody(t *testing.T) []byte {
	t.Helper()
	body := fwdShapeBody(nil, forwardBodyBaseAttrs(t, 200000), forwardBodyNLRIs(4, false))
	require.LessOrEqual(t, message.HeaderLen+len(body), message.MaxMsgLen,
		"guard: the body must FIT, so the transcode buffer is carried out rather than copied by the splitter")
	return body
}

// VALIDATES: AC-4 -- the cross-context transcode takes its buffer from the shared
// read pool instead of allocating a fresh one per forward, and takes none at all
// when the destination context needs no transcode.
// PREVENTS: a silent return to `make([]byte, len(payload)*2+1024)` per forward,
// which is the ai/rules/performance.md violation this item exists to remove. The
// pool in-use delta is the only observable that separates the two: both produce
// identical bytes.
func TestTranscodeBufferPooled(t *testing.T) {
	t.Run("cross_context_transcode_borrows_from_pool", func(t *testing.T) {
		_, srcCtxID := registerForwardBodyTestContext(t, true, false)
		destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
		peer := forwardBodyTestPeer(destCtx, destCtxID)

		_, before := bufMuxStd.Stats()

		result, ok := buildFwdBody(wireu.NewWireUpdate(crossContextTranscodeBody(t), srcCtxID),
			message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.20"), &fwdParseCache{})
		require.True(t, ok)
		require.Len(t, result.updates, 1, "guard: a fitting UPDATE must not be split")

		_, borrowed := bufMuxStd.Stats()
		assert.Equal(t, before+1, borrowed,
			"the transcode must borrow exactly one read-pool buffer, not allocate one")
		require.NotNil(t, result.transcodeBuf.Buf,
			"the borrowed handle must travel out for the caller to adopt")

		// The result aliases the buffer, so releasing is the caller's job. Stand in
		// for it here so the pool is left as it was found.
		ReturnReadBuffer(result.transcodeBuf)
		_, after := bufMuxStd.Stats()
		require.Equal(t, before, after)
	})

	t.Run("matching_context_borrows_nothing", func(t *testing.T) {
		// Same ASN width, different ADD-PATH: the re-encode branch runs but the
		// RFC 6793 transcode does not, so no buffer may be taken.
		_, srcCtxID := registerForwardBodyTestContext(t, true, true)
		destCtx, destCtxID := registerForwardBodyTestContext(t, true, false)
		peer := forwardBodyTestPeer(destCtx, destCtxID)

		body := fwdShapeBody(nil, forwardBodyBaseAttrs(t, 65000), forwardBodyNLRIs(4, true))

		_, before := bufMuxStd.Stats()
		result, ok := buildFwdBody(wireu.NewWireUpdate(body, srcCtxID),
			message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.21"), &fwdParseCache{})
		require.True(t, ok)
		_, after := bufMuxStd.Stats()

		assert.Equal(t, before, after, "no ASN width change means no transcode buffer")
		assert.Nil(t, result.transcodeBuf.Buf, "nothing to adopt when nothing was borrowed")
	})
}

// VALIDATES: AC-10, AC-13 -- the cross-context RFC 6793 transcode emits exactly
// the bytes the pre-T1-2 unpooled `make([]byte, len(payload)*2+1024)` produced.
// PREVENTS: the half of AC-10 that was claimed and not covered. The Tier 1
// golden corpus drives buildModifiedPayload only, so nothing compared the
// transcode's OUTPUT before and after it started borrowing from the read pool.
// TestTranscodeBufferPooled counts pool borrows and asserts nothing about bytes,
// so a pooled buffer handed back short, or still holding another route's UPDATE,
// would have passed it.
//
// The reference is computed by the removed code path itself: a fresh make of the
// same size, transcoded by the same wireu.TranscodeASPath. That is the exact
// claim AC-10 makes for T1-2 -- only where the buffer comes from changed -- so
// it needs no pinned hex constant that would rot when a fixture moves.
func TestGoldenBytesUnchangedCrossContextTranscode(t *testing.T) {
	srcCtx, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	body := crossContextTranscodeBody(t)

	// Reference: the pre-T1-2 shape, verbatim -- a fresh make of the size this
	// call site has always asked for, transcoded by the same function.
	refDst := make([]byte, len(body)*2+1024)
	refN, err := wireu.TranscodeASPath(refDst, body, srcCtx.ASN4(), destCtx.ASN4())
	require.NoError(t, err)
	require.Positive(t, refN, "guard: the fixture must actually transcode, or this test compares nothing")
	refUpdate, err := message.UnpackUpdate(refDst[:refN])
	require.NoError(t, err)
	// Both sides go through the same packer, so a difference can only come from
	// the bytes, never from how they were serialized for comparison.
	want := hex.EncodeToString(fwdPackUpdateBody(refUpdate))

	// Poison every free read-pool buffer, so a transcode that reads past what it
	// wrote carries 0xEE rather than a plausible zero.
	poisonReadPool(t)

	result, ok := buildFwdBody(wireu.NewWireUpdate(body, srcCtxID),
		message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.30"), &fwdParseCache{})
	require.True(t, ok)
	require.Len(t, result.updates, 1, "guard: a fitting UPDATE must not be split")
	require.NotNil(t, result.transcodeBuf.Buf,
		"guard: this test is about the POOLED transcode; a nil handle means it took the make fallback")

	assert.Equal(t, want, hex.EncodeToString(fwdPackUpdateBody(result.updates[0])),
		"pooling the transcode buffer must not move a byte on the wire")

	ReturnReadBuffer(result.transcodeBuf)
}

// poisonReadPool fills every currently-free standard read-pool buffer with a
// byte no valid UPDATE body starts with, then returns them. A transcode that
// emits stale bytes then fails a byte comparison instead of reading as zeros.
func poisonReadPool(t *testing.T) {
	t.Helper()
	var held []BufHandle
	for range 8 {
		h := getReadBuf(false)
		if h.Buf == nil {
			break
		}
		for i := range h.Buf {
			h.Buf[i] = 0xEE
		}
		held = append(held, h)
	}
	require.NotEmpty(t, held, "guard: the read pool handed out nothing, so nothing was poisoned")
	for _, h := range held {
		ReturnReadBuffer(h)
	}
}

// VALIDATES: AC-5 -- every borrowed transcode buffer is returned exactly once:
// through adoption plus eviction on the success path, and inside the call on a
// failure after acquisition, which hands the caller nothing to adopt.
// PREVENTS: both halves of the lifetime bug. Returning the buffer at end of call
// recycles it under a pending async write (another route's bytes on the wire);
// never returning it leaks a pooled buffer per forward, which is the defect
// spec-fixit-forward-readbuf-leak already fixed once for the sibling sites.
func TestTranscodeBufferSingleReturn(t *testing.T) {
	_, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	t.Run("adopt_then_drain_returns_exactly_once", func(t *testing.T) {
		_, before := bufMuxStd.Stats()

		result, ok := buildFwdBody(wireu.NewWireUpdate(crossContextTranscodeBody(t), srcCtxID),
			message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.22"), &fwdParseCache{})
		require.True(t, ok)

		// The production ownership chain: the caller adopts onto the entry, and the
		// cache drains adopted handles once at eviction.
		update := &ReceivedUpdate{WireUpdate: wireu.NewWireUpdate(nil, srcCtxID)}
		update.adoptFwdHandle(result.transcodeBuf)

		_, held := bufMuxStd.Stats()
		require.Equal(t, before+1, held,
			"the buffer must still be in use while the entry holds it: the updates alias it")

		update.returnFwdHandles()
		_, afterDrain := bufMuxStd.Stats()
		assert.Equal(t, before, afterDrain, "eviction must return the adopted buffer")

		// A second drain must not return it again. A double return would hand the
		// same slot to two writers.
		update.returnFwdHandles()
		_, afterSecondDrain := bufMuxStd.Stats()
		assert.Equal(t, before, afterSecondDrain, "the handle must be returned once, not twice")
	})

	t.Run("failure_after_acquisition_returns_and_adopts_nothing", func(t *testing.T) {
		// ASN4 mismatch drives the transcode (buffer acquired), then an ADD-PATH
		// mismatch sends the NLRI through the re-encoder, where a trailing byte
		// fails it -- an error raised AFTER the buffer was taken.
		_, badSrcCtxID := registerForwardBodyTestContext(t, true, true)
		badDestCtx, badDestCtxID := registerForwardBodyTestContext(t, false, false)
		badPeer := forwardBodyTestPeer(badDestCtx, badDestCtxID)

		malformed := append(forwardBodyNLRIs(2, true), 0x18)
		body := fwdShapeBody(nil, forwardBodyBaseAttrs(t, 200000), malformed)

		_, before := bufMuxStd.Stats()
		result, ok := buildFwdBody(wireu.NewWireUpdate(body, badSrcCtxID),
			message.MaxMsgLen, badDestCtxID, badPeer, netip.MustParseAddr("192.0.2.23"), &fwdParseCache{})
		require.False(t, ok, "guard: malformed NLRI must fail the re-encode")

		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after, "a failure after acquisition must return the buffer")
		assert.Nil(t, result.transcodeBuf.Buf, "a failed body must hand the caller nothing to adopt")
	})

	t.Run("repeated_cycles_stay_at_baseline", func(t *testing.T) {
		_, before := bufMuxStd.Stats()
		for range 8 {
			result, ok := buildFwdBody(wireu.NewWireUpdate(crossContextTranscodeBody(t), srcCtxID),
				message.MaxMsgLen, destCtxID, peer, netip.MustParseAddr("192.0.2.24"), &fwdParseCache{})
			require.True(t, ok)
			update := &ReceivedUpdate{WireUpdate: wireu.NewWireUpdate(nil, srcCtxID)}
			update.adoptFwdHandle(result.transcodeBuf)
			update.returnFwdHandles()
		}
		_, after := bufMuxStd.Stats()
		assert.Equal(t, before, after,
			"repeated borrow/adopt/drain cycles must neither leak nor over-return")
	})
}

func registerForwardBodyTestContext(t *testing.T, asn4, addPath bool) (*bgpctx.EncodingContext, bgpctx.ContextID) {
	t.Helper()
	ctx := bgpctx.EncodingContextWithAddPath(asn4, map[family.Family]bool{family.IPv4Unicast: addPath})
	id, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	return ctx, id
}

func forwardBodyTestPeer(ctx *bgpctx.EncodingContext, ctxID bgpctx.ContextID) *Peer {
	peer := NewPeer(&PeerSettings{Address: netip.MustParseAddr("192.0.2.1"), LocalAS: 65000, PeerAS: 65001})
	peer.sendCtx.Store(ctx)
	peer.sendCtxID = ctxID
	return peer
}

func forwardBodyBaseAttrs(t *testing.T, asn uint32) []byte {
	t.Helper()
	asPath := &attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{asn}}}}
	asPathValueLen := asPath.LenWithContext(nil, bgpctx.EncodingContextForASN4(true))
	asPathAttr := make([]byte, 4+asPathValueLen)
	hdrLen := attribute.WriteHeaderTo(asPathAttr, 0, asPath.Flags(), asPath.Code(), uint16(asPathValueLen))
	asPath.WriteToWithASN4(asPathAttr, hdrLen, true)
	asPathAttr = asPathAttr[:hdrLen+asPathValueLen]

	origin := []byte{0x40, 0x01, 0x01, 0x00}
	nextHop := []byte{0x40, 0x03, 0x04, 192, 0, 2, 1}
	attrs := make([]byte, 0, len(origin)+len(asPathAttr)+len(nextHop))
	attrs = append(attrs, origin...)
	attrs = append(attrs, asPathAttr...)
	attrs = append(attrs, nextHop...)
	return attrs
}

func forwardBodyNLRIs(count int, addPath bool) []byte {
	out := make([]byte, 0, count*8)
	for i := range count {
		if addPath {
			var pathID [4]byte
			binary.BigEndian.PutUint32(pathID[:], uint32(i+1))
			out = append(out, pathID[:]...)
		}
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i / 256), byte(i), 0}), 24)
		out = append(out, nlri.NewINET(family.IPv4Unicast, prefix, 0).Bytes()...)
	}
	return out
}

func forwardBodyFindASPath(t *testing.T, attrs []byte, asn4 bool) *attribute.ASPath {
	t.Helper()
	for off := 0; off < len(attrs); {
		_, code, length, hdrLen, err := attribute.ParseHeader(attrs[off:])
		require.NoError(t, err)
		end := off + hdrLen + int(length)
		require.LessOrEqual(t, end, len(attrs))
		if code == attribute.AttrASPath {
			path, err := attribute.ParseASPath(attrs[off+hdrLen:end], asn4)
			require.NoError(t, err)
			return path
		}
		off = end
	}
	return nil
}

func forwardBodyFindAS4Path(t *testing.T, attrs []byte) *attribute.AS4Path {
	t.Helper()
	for off := 0; off < len(attrs); {
		_, code, length, hdrLen, err := attribute.ParseHeader(attrs[off:])
		require.NoError(t, err)
		end := off + hdrLen + int(length)
		require.LessOrEqual(t, end, len(attrs))
		if code == attribute.AttrAS4Path {
			path, err := attribute.ParseAS4Path(attrs[off+hdrLen : end])
			require.NoError(t, err)
			return path
		}
		off = end
	}
	return nil
}
