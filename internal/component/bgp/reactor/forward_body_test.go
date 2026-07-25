package reactor

import (
	"encoding/binary"
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
	// RFC requirement: RFC7911-5-3 positive -- source and destination share an ADD-PATH-negotiated context (both addPath=true), so forwarding preserves the extended NLRI encoding: the emitted NLRI bytes equal the source NLRI including their 4-octet Path IDs.
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
	assert.Equal(t, sourceNLRI, gotNLRI)
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
