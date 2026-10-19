package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// RFC 7606 Section 5.1, first bullet: "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if
// present) SHALL be encoded as the very first path attribute in an UPDATE message."
//
// Ze is the sender of the bytes it relays. A received UPDATE whose MP attribute trails is
// relayed with the MP attribute moved to the front and every other byte unchanged; a
// compliant one keeps the zero-copy forward. These tests pin the three relay producers:
// the same-context raw branch and the re-encode branch of buildFwdBody, and the
// per-destination rebuild (attrEmitter.emitMP).

// fwdMPFirstReach is an IPv6 unicast MP_REACH_NLRI announcing 2001:db8:1::/48 via
// 2001:db8::1.
var fwdMPFirstReach = []byte{
	0x80, 0x0E, 0x1C,
	0x00, 0x02, 0x01, 0x10,
	0x20, 0x01, 0x0D, 0xB8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01,
	0x00,
	0x30, 0x20, 0x01, 0x0D, 0xB8, 0x00, 0x01,
}

// fwdMPFirstBodies returns the same MP-only announcement twice: once as a sender that
// put MP_REACH_NLRI last, and once as RFC 7606 Section 5.1 orders it.
func fwdMPFirstBodies(t *testing.T, asn uint32) (trailing, leading []byte) {
	t.Helper()
	base := forwardBodyBaseAttrs(t, asn)
	trailing = fwdShapeBody(nil, append(append([]byte{}, base...), fwdMPFirstReach...), nil)
	leading = fwdShapeBody(nil, append(append([]byte{}, fwdMPFirstReach...), base...), nil)
	return trailing, leading
}

// fwdMPFirstCode returns the type code of the first path attribute in attrs.
func fwdMPFirstCode(t *testing.T, attrs []byte) attribute.AttributeCode {
	t.Helper()
	iter := attribute.NewAttrIterator(attrs)
	code, _, _, ok := iter.Next()
	require.True(t, ok, "the attribute section must hold an attribute")
	return code
}

// VALIDATES: a same-context relay of an UPDATE whose sender put MP_REACH_NLRI after
// ORIGIN, AS_PATH and NEXT_HOP leaves Ze with MP_REACH_NLRI first, and is otherwise the
// byte string a compliant sender's UPDATE relays as.
// PREVENTS: the zero-copy branch handing a trailing MP attribute on unchanged.
//
// RFC requirement: RFC7606-5.1-1 positive -- a same-context relay of a received UPDATE whose MP_REACH_NLRI follows other path attributes emits MP_REACH_NLRI as the first path attribute, with every other byte equal to the received UPDATE's.
func TestForwardMovesTrailingMPAttributeFirstSameContext(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, false)
	peer := forwardBodyTestPeer(ctx, ctxID)
	trailing, leading := fwdMPFirstBodies(t, 65000)

	result, ok := buildFwdBody(wireu.NewWireUpdate(trailing, ctxID), message.MaxMsgLen, ctxID, peer,
		netip.MustParseAddr("192.0.2.21"), &fwdParseCache{})
	// Stand in for the caller's adoptFwdHandle: the reorder borrows a read-pool buffer.
	defer returnReadBuffer(result.transcodeBuf)
	require.True(t, ok)
	require.Len(t, result.rawBodies, 1)
	assert.Equal(t, leading, result.rawBodies[0], "the relay must move MP_REACH_NLRI first and change nothing else")
	assert.NotSame(t, &trailing[0], &result.rawBodies[0][0], "the received bytes must not be rewritten in place")
	assert.Equal(t, fwdMPFirstReach[:3], trailing[len(trailing)-len(fwdMPFirstReach):][:3],
		"the received UPDATE is left as it arrived")
}

// VALIDATES: an UPDATE that already leads with its MP attribute is relayed as the
// received bytes themselves.
// PREVENTS: the MP position check costing a copy on every forward. The assertion is
// backing-array identity, which a byte-equal copy would fail.
//
// RFC requirement: RFC7606-5.1-1 negative -- a same-context relay of a received UPDATE whose MP_REACH_NLRI is already the first path attribute hands on the received bytes unchanged, without a reorder or a copy.
func TestForwardLeadingMPAttributeKeepsZeroCopy(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, false)
	peer := forwardBodyTestPeer(ctx, ctxID)
	_, leading := fwdMPFirstBodies(t, 65000)

	result, ok := buildFwdBody(wireu.NewWireUpdate(leading, ctxID), message.MaxMsgLen, ctxID, peer,
		netip.MustParseAddr("192.0.2.22"), &fwdParseCache{})
	defer returnReadBuffer(result.transcodeBuf)
	require.True(t, ok)
	require.Len(t, result.rawBodies, 1)
	require.Same(t, &leading[0], &result.rawBodies[0][0],
		"a compliant UPDATE must be forwarded as the received bytes, not a copy")
}

// VALIDATES: a relay re-encoded for a destination with another ASN width also leads with
// MP_REACH_NLRI, and equals the re-encode of the same UPDATE from a compliant sender.
// PREVENTS: the re-encode branch keeping the sender's order.
//
// RFC requirement: RFC7606-5.1-1 positive -- a relay re-encoded for a destination context of a received UPDATE whose MP_REACH_NLRI follows other path attributes emits MP_REACH_NLRI as the first path attribute, with the same attribute bytes the re-encode of a compliant sender's UPDATE carries.
func TestForwardMovesTrailingMPAttributeFirstAcrossContexts(t *testing.T) {
	_, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)
	trailing, leading := fwdMPFirstBodies(t, 200000)

	got, ok := buildFwdBody(wireu.NewWireUpdate(trailing, srcCtxID), message.MaxMsgLen, destCtxID, peer,
		netip.MustParseAddr("192.0.2.23"), &fwdParseCache{})
	defer returnReadBuffer(got.transcodeBuf)
	require.True(t, ok)
	require.Empty(t, got.rawBodies, "mismatched contexts must not reuse the source bytes")
	require.Len(t, got.updates, 1)

	want, ok := buildFwdBody(wireu.NewWireUpdate(leading, srcCtxID), message.MaxMsgLen, destCtxID, peer,
		netip.MustParseAddr("192.0.2.24"), &fwdParseCache{})
	defer returnReadBuffer(want.transcodeBuf)
	require.True(t, ok)
	require.Len(t, want.updates, 1)

	assert.Equal(t, attribute.AttrMPReachNLRI, fwdMPFirstCode(t, got.updates[0].PathAttributes))
	assert.Equal(t, want.updates[0].PathAttributes, got.updates[0].PathAttributes)
	assert.Empty(t, got.updates[0].WithdrawnRoutes)
	assert.Empty(t, got.updates[0].NLRI)
}

// VALIDATES: a per-destination rebuild (an added OTC here) of an UPDATE whose sender put
// MP_REACH_NLRI after ORIGIN writes the sourced MP_REACH_NLRI first, and a rebuild of a
// compliant UPDATE writes the same bytes.
// PREVENTS: attrEmitter keeping a sourced MP attribute in its source position, which also
// made the rebuild and the zero-copy forward of one UPDATE two different byte strings.
//
// RFC requirement: RFC7606-5.1-1 positive -- a relayed UPDATE rebuilt for a destination by buildModifiedPayload, whose source carries MP_REACH_NLRI after ORIGIN, emits the sourced MP_REACH_NLRI as the first path attribute.
func TestForwardRebuildMovesSourcedMPAttributeFirst(t *testing.T) {
	origin := makeAttr(0x40, 1, []byte{0x00})
	otc := []byte{0xC0, 35, 4, 0, 0, 0xFD, 0xE8}
	otcHandler := filterapi.AttrModHandler(func(p *filterapi.AttrPlan) {
		if p.Source() != nil {
			p.KeepAll()
			return
		}
		for i, op := range p.Ops() {
			if op.Action != filterapi.AttrModSet || len(op.Buf) != 4 {
				continue
			}
			p.Op(i)
			p.Emit(0xC0, 35)
			return
		}
		p.Drop()
	})
	rebuild := func(attrs []byte) []byte {
		var mods filterapi.ModAccumulator
		mods.Op(35, filterapi.AttrModSet, otc[3:])
		out, _, fail := buildModifiedPayload(buildModTestPayload(attrs, nil), &mods,
			map[uint8]filterapi.AttrModHandler{35: otcHandler}, nil, nil)
		require.False(t, fail.failed())
		require.NotNil(t, out)
		return append([]byte(nil), out...)
	}

	got := rebuild(append(append([]byte{}, origin...), fwdMPFirstReach...))
	want := buildModTestPayload(append(append(append([]byte{}, fwdMPFirstReach...), origin...), otc...), nil)
	assert.Equal(t, want, got, "MP_REACH_NLRI, then ORIGIN, then the added OTC")
	assert.Equal(t, want, rebuild(append(append([]byte{}, fwdMPFirstReach...), origin...)),
		"a compliant source rebuilds to the same bytes")
}
