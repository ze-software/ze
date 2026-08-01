// Design: docs/architecture/core-design.md -- tests for mpReachNextHopHandler
// Related: filter_delta_handlers.go -- handler under test

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// planOutcome records what one handler's plan resolved to.
type planOutcome struct {
	out     []byte // the emitted attribute bytes; nil when nothing is emitted
	failed  bool   // the handler refused: the caller suppresses the route
	dropped bool   // the attribute leaves the UPDATE
}

// planHandler drives an AttrModHandler through one attribute plan and
// materializes exactly what that plan describes.
//
// It is the test-side equivalent of planAttr plus attrEmitter.emitSlot
// (forward_build.go). A handler PLANS and writes no bytes, so a test that
// asserts on wire bytes has to run the plan to see them. srcAttr is the whole
// source attribute, header included, and it doubles as the attribute section the
// plan's offsets resolve against.
func planHandler(h filterapi.AttrModHandler, code uint8, srcAttr []byte, ops []filterapi.AttrOp) planOutcome {
	var mods filterapi.ModAccumulator
	edit := mods.EditSet()
	edit.Begin()

	var src []byte
	srcOff, srcLen, valOff, valLen := 0, 0, 0, 0
	if len(srcAttr) > 0 {
		hdr := 3
		if srcAttr[0]&0x10 != 0 { // RFC 4271 Section 4.3 Extended Length
			hdr = 4
		}
		src = srcAttr
		srcLen = len(srcAttr)
		valOff, valLen = hdr, len(srcAttr)-hdr
	}

	p := edit.Attr(code, src, srcOff, srcLen, valOff, valLen, ops, 0)
	h(p)
	id := edit.Commit(p)
	if edit.SlotFailed(id) {
		return planOutcome{failed: true}
	}
	if edit.SlotDropped(id) {
		return planOutcome{dropped: true}
	}
	n := edit.SlotSize(id)
	buf := make([]byte, n)
	if got := edit.SlotWrite(id, srcAttr, ops, buf, 0); got != n {
		return planOutcome{failed: true}
	}
	return planOutcome{out: buf}
}

// planHandlerBytes returns the bytes a handler's plan emits. ok is false when
// the plan emitted nothing, whether by refusing or by dropping the attribute.
func planHandlerBytes(h filterapi.AttrModHandler, code uint8, srcAttr []byte, ops []filterapi.AttrOp) (out []byte, ok bool) {
	res := planHandler(h, code, srcAttr, ops)
	if res.failed || res.dropped {
		return nil, false
	}
	return res.out, true
}

// buildMPReachSource constructs a raw MP_REACH_NLRI attribute (header + value).
// value layout: AFI(2) | SAFI(1) | NHLen(1) | NH(nhLen) | Reserved(1) | NLRI.
func buildMPReachSource(afi uint16, safi byte, nh, nlri []byte) []byte {
	valLen := 2 + 1 + 1 + len(nh) + 1 + len(nlri)
	attr := make([]byte, 0, 4+valLen)
	if valLen > 255 {
		var lenBuf [2]byte
		binary.BigEndian.PutUint16(lenBuf[:], uint16(valLen))
		attr = append(attr, 0x90, 14, lenBuf[0], lenBuf[1])
	} else {
		attr = append(attr, 0x80, 14, byte(valLen))
	}
	var afiBuf [2]byte
	binary.BigEndian.PutUint16(afiBuf[:], afi)
	attr = append(attr, afiBuf[0], afiBuf[1], safi, byte(len(nh)))
	attr = append(attr, nh...)
	attr = append(attr, 0x00) // Reserved
	attr = append(attr, nlri...)
	return attr
}

// TestMPReachNextHopHandler_Rewrite16Bytes verifies that a single-global
// IPv6 next-hop is rewritten and AFI/SAFI/Reserved/NLRI are preserved.
//
// VALIDATES: cmd-1 IPv6 next-hop rewrite for a 16-byte MP_REACH next-hop.
// PREVENTS: Silent corruption of NLRI bytes when rewriting the next-hop.
func TestMPReachNextHopHandler_Rewrite16Bytes(t *testing.T) {
	oldNH := []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0x00, 0x01,
	}
	nlri := []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0} // /64 prefix 2001:db8::/64
	src := buildMPReachSource(2 /*AFI IPv6*/, 1 /*SAFI unicast*/, oldNH, nlri)

	newNH := []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0x00, 0x02,
	}
	ops := []filterapi.AttrOp{
		{Code: 14, Action: filterapi.AttrModSet, Buf: newNH},
	}

	out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, ops)
	require.True(t, ok, "handler planned an emitted attribute")
	n := len(out)
	require.Equal(t, len(src), n, "length unchanged for 16->16 rewrite")

	// Parse the output and verify layout.
	assert.Equal(t, byte(0x80), out[0], "flags preserved (optional, not extended)")
	assert.Equal(t, byte(14), out[1], "attribute code")
	assert.Equal(t, byte(len(src)-3), out[2], "one-byte length = valLen")

	// Value starts at offset 3.
	val := out[3:n]
	assert.Equal(t, byte(0), val[0], "AFI high byte")
	assert.Equal(t, byte(2), val[1], "AFI low byte = IPv6")
	assert.Equal(t, byte(1), val[2], "SAFI = unicast")
	assert.Equal(t, byte(16), val[3], "NH length preserved")
	assert.Equal(t, newNH, val[4:20], "next-hop rewritten")
	assert.Equal(t, byte(0), val[20], "reserved byte preserved")
	assert.Equal(t, nlri, val[21:], "NLRI preserved")
}

// TestMPReachNextHopHandler_Rewrite32Bytes verifies that a global+link-local
// IPv6 next-hop is rewritten and the surrounding bytes are preserved.
//
// VALIDATES: cmd-1 RFC 2545 Section 3 dual-address IPv6 next-hop rewrite.
// PREVENTS: Link-local scope dropped when the rewriter only supports 16-byte NH.
func TestMPReachNextHopHandler_Rewrite32Bytes(t *testing.T) {
	// Source starts with a 16-byte NH; we rewrite to 32-byte NH (global + LL).
	oldNH := []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0x00, 0x01,
	}
	nlri := []byte{0x30, 0xfc, 0x00, 0x00} // /48 prefix fc00::/48
	src := buildMPReachSource(2, 1, oldNH, nlri)

	newNH := make([]byte, 32)
	// global
	copy(newNH, []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0x00, 0x02,
	})
	// link-local
	copy(newNH[16:], []byte{
		0xfe, 0x80, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0x00, 0x02,
	})
	ops := []filterapi.AttrOp{
		{Code: 14, Action: filterapi.AttrModSet, Buf: newNH},
	}

	out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, ops)
	require.True(t, ok, "handler planned an emitted attribute")
	n := len(out)
	require.Equal(t, len(src)+16, n, "length grew by 16 bytes (old 16 -> new 32)")

	// Parse the output and verify layout.
	assert.Equal(t, byte(0x80), out[0], "flags: optional, non-extended (still < 255)")
	assert.Equal(t, byte(14), out[1])
	newValLen := int(out[2])
	assert.Equal(t, n-3, newValLen)

	val := out[3:n]
	assert.Equal(t, byte(32), val[3], "NH length updated to 32")
	assert.Equal(t, newNH, val[4:36], "32-byte next-hop written")
	assert.Equal(t, byte(0), val[36], "reserved byte preserved")
	assert.Equal(t, nlri, val[37:], "NLRI preserved")
}

// TestMPReachNextHopHandler_CopyWhenNoOps verifies the handler is a no-op
// when no ops are provided.
//
// VALIDATES: Pass-through behavior -- no rewrite means source bytes copied verbatim.
// PREVENTS: Spurious attribute corruption for peers that have next-hop-auto.
func TestMPReachNextHopHandler_CopyWhenNoOps(t *testing.T) {
	oldNH := []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0x00, 0x01,
	}
	nlri := []byte{0x30, 0xfc, 0x00, 0x00}
	src := buildMPReachSource(2, 1, oldNH, nlri)

	out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, nil)
	require.True(t, ok, "handler planned an emitted attribute")
	n := len(out)
	require.Equal(t, len(src), n)
	assert.Equal(t, src, out)
}

// TestMPReachNextHopHandler_InvalidOpLength verifies that an op with a
// wrong-size next-hop buffer is rejected and the source is copied unchanged.
//
// VALIDATES: Fail-safe when a caller produces an invalid rewrite op.
// PREVENTS: Mangled MP_REACH bytes reaching the wire.
func TestMPReachNextHopHandler_InvalidOpLength(t *testing.T) {
	oldNH := []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0x00, 0x01,
	}
	nlri := []byte{0x30, 0xfc, 0x00, 0x00}
	src := buildMPReachSource(2, 1, oldNH, nlri)

	ops := []filterapi.AttrOp{
		{Code: 14, Action: filterapi.AttrModSet, Buf: []byte{1, 2, 3}}, // 3 bytes -- invalid
	}
	out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, ops)
	require.True(t, ok, "handler planned an emitted attribute")
	n := len(out)
	require.Equal(t, len(src), n)
	assert.Equal(t, src, out, "source preserved when op length is invalid")
}

// TestMPReachNextHopHandler_IPv4NextHopThroughMPReach verifies that a 4-byte
// next-hop op targeting MP_REACH (used by labeled-unicast / VPN families where
// the IPv4 NH lives inside MP_REACH) patches correctly.
//
// VALIDATES: cmd-1 IPv4-over-MP_REACH rewrite (RFC 8277 labeled unicast).
// PREVENTS: Label/VPN families getting the wrong next-hop when next-hop-self
// is enabled on an IPv4 peer carrying non-unicast families.
func TestMPReachNextHopHandler_IPv4NextHopThroughMPReach(t *testing.T) {
	oldNH := []byte{10, 0, 0, 1} // 10.0.0.1
	nlri := []byte{0x18, 10, 0, 0}
	src := buildMPReachSource(1 /*AFI IPv4*/, 4 /*SAFI labeled-unicast*/, oldNH, nlri)

	newNH := []byte{10, 0, 0, 2}
	ops := []filterapi.AttrOp{
		{Code: 14, Action: filterapi.AttrModSet, Buf: newNH},
	}
	out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, ops)
	require.True(t, ok, "handler planned an emitted attribute")
	n := len(out)
	require.Equal(t, len(src), n)

	val := out[3:n]
	assert.Equal(t, byte(4), val[3], "NH length = 4")
	assert.Equal(t, newNH, val[4:8], "IPv4 NH rewritten")
	assert.Equal(t, nlri, val[9:], "NLRI preserved")
}

// TestMPReachNextHopHandler_RejectsOverflow verifies that a rewrite that
// would push the new attribute value past the BGP 65535 cap is refused,
// preserving the source rather than silently truncating the length field.
//
// VALIDATES: cmd-1 review finding #1 -- uint16 truncation at the wire
// boundary. Catches the case where srcValLen ~ 65535 and new NH growth
// pushes newValLen over the cap.
// PREVENTS: Silent wire corruption -- handler writes N bytes but emits
// "N mod 65536" in the length field, poisoning downstream attribute parsing.
//
// The MECHANISM changed with the plan-based AttrModHandler contract, and the
// assertions are re-pointed at the new one. A handler used to answer an
// overflow by copying the source through, which forwarded the route carrying
// exactly the next-hop the policy existed to rewrite. RFC 4271 Section 4.3 caps
// an attribute value at 65535 octets, so the plan now REFUSES and the caller
// suppresses the route for this destination (ai/rules/fail-closed-guards.md).
// Both outcomes prevent a truncated length field reaching the wire, which is
// what this test is for; only refusal also stops the unmodified route.
func TestMPReachNextHopHandler_RejectsOverflow(t *testing.T) {
	// Build a near-full MP_REACH: 0-byte next-hop, and a giant NLRI padding
	// so that srcValLen reaches just under 65535. Growing NH to 32 bytes
	// would push newValLen past 65535 and must be refused.
	padding := make([]byte, 65500) // large NLRI blob, arbitrary bytes
	src := buildMPReachSource(2, 1, nil, padding)
	// Sanity: the source value length should be >=65504 so that a 32-byte
	// next-hop would push it past 65535.
	require.GreaterOrEqual(t, len(src), 65504)

	newNH := make([]byte, 32)
	ops := []filterapi.AttrOp{
		{Code: 14, Action: filterapi.AttrModSet, Buf: newNH},
	}

	res := planHandler(mpReachNextHopHandler(), 14, src, ops)

	// Must have refused, not planned a value whose length field would wrap.
	require.True(t, res.failed, "overflow rewrite must refuse so the caller suppresses the route")
	assert.Nil(t, res.out, "overflow rewrite must emit no bytes rather than a truncated header")
	assert.False(t, res.dropped, "refusal, not a drop: dropping MP_REACH would forward a route without its NLRI")
}

// TestBuildModifiedPayload_MPReachNextHopSelf verifies that the full
// buildModifiedPayload pipeline rewrites a forwarded IPv6 UPDATE's MP_REACH
// next-hop when a peer is configured with next-hop self.
//
// VALIDATES: cmd-1 end-to-end integration -- applyNextHopMod emits op 14,
// buildModifiedPayload dispatches to mpReachNextHopHandler, and the NLRI
// bytes survive the patch intact.
// PREVENTS: A future refactor that silently drops MP_REACH mods from the
// attrModHandlers map.
func TestBuildModifiedPayload_MPReachNextHopSelf(t *testing.T) {
	// Build a source UPDATE body:
	//   withdrawn_len(2)=0 | attr_len(2) | origin + aspath + mpreach | no legacy NLRI.
	oldNH := []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0x00, 0x01,
	}
	nlri := []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}
	mpReach := buildMPReachSource(2, 1, oldNH, nlri)

	origin := []byte{0x40, 0x01, 0x01, 0x00}                         // ORIGIN IGP
	aspath := []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0, 0, 0xfd, 0xe9} // AS_PATH [65001]
	attrs := make([]byte, 0, len(origin)+len(aspath)+len(mpReach))
	attrs = append(attrs, origin...)
	attrs = append(attrs, aspath...)
	attrs = append(attrs, mpReach...)

	payload := make([]byte, 0, 4+len(attrs))
	var lenBuf [2]byte
	// withdrawn_len = 0
	payload = append(payload, 0x00, 0x00)
	// attr_len
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(attrs)))
	payload = append(payload, lenBuf[0], lenBuf[1])
	payload = append(payload, attrs...)

	// Build the accumulator with a next-hop-self rewrite for IPv6.
	newNH := []byte{
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0x00, 0x02,
	}
	var mods filterapi.ModAccumulator
	mods.Op(14, filterapi.AttrModSet, newNH)

	handlers := attrModHandlersWithDefaults()
	modified, _, _ := buildModifiedPayload(payload, &mods, handlers, nil, nil)
	require.NotNil(t, modified, "payload was modified")
	require.Equal(t, len(payload), len(modified), "length unchanged for 16->16 rewrite")

	// Find the MP_REACH attribute in the modified payload.
	attrLen := int(binary.BigEndian.Uint16(modified[2:4]))
	mod := modified[4 : 4+attrLen]
	idx := 0
	var foundNH []byte
	for idx < len(mod) {
		flags := mod[idx]
		code := mod[idx+1]
		hdrLen := 3
		valLen := int(mod[idx+2])
		if flags&0x10 != 0 {
			hdrLen = 4
			valLen = int(binary.BigEndian.Uint16(mod[idx+2 : idx+4]))
		}
		if code == 14 {
			// MP_REACH: skip AFI(2)+SAFI(1), read NHLen(1), grab next-hop bytes.
			val := mod[idx+hdrLen : idx+hdrLen+valLen]
			nhLen := int(val[3])
			foundNH = val[4 : 4+nhLen]
			break
		}
		idx += hdrLen + valLen
	}
	require.NotNil(t, foundNH, "MP_REACH still present after rewrite")
	assert.Equal(t, newNH, foundNH, "MP_REACH next-hop was rewritten to the self address")
}

// TestApplyNextHopMod_IPv4EmitsBothOps verifies that applyNextHopMod with
// an IPv4 local address emits both a legacy NEXT_HOP (type 3) op and an
// MP_REACH_NLRI (type 14) op with IPv4-mapped IPv6 bytes.
//
// VALIDATES: Mixed-family next-hop rewrite for IPv4 local + IPv6 routes.
// PREVENTS: IPv6 routes forwarded with stale next-hop when session is IPv4.
func TestApplyNextHopMod_IPv4EmitsBothOps(t *testing.T) {
	t.Parallel()

	dest := &PeerSettings{
		NextHopMode:  NextHopSelf,
		LocalAddress: netip.MustParseAddr("127.0.0.2"),
	}
	var mods filterapi.ModAccumulator
	applyNextHopMod(dest, &mods)

	ops := mods.Ops()
	require.Len(t, ops, 2, "IPv4 local must emit both type 3 and type 14 ops")

	// Op 0: legacy NEXT_HOP (type 3) with 4-byte IPv4.
	assert.Equal(t, uint8(3), ops[0].Code, "first op is legacy NEXT_HOP")
	assert.Equal(t, []byte{127, 0, 0, 2}, ops[0].Buf, "NEXT_HOP = 127.0.0.2")

	// Op 1: MP_REACH_NLRI (type 14) with 16-byte IPv4-mapped IPv6.
	assert.Equal(t, uint8(14), ops[1].Code, "second op is MP_REACH_NLRI")
	assert.Len(t, ops[1].Buf, 16, "IPv4-mapped IPv6 is 16 bytes")
	// ::ffff:127.0.0.2 = 00 00 00 00 00 00 00 00 00 00 FF FF 7F 00 00 02
	assert.Equal(t, byte(0xFF), ops[1].Buf[10], "mapped sentinel byte 10")
	assert.Equal(t, byte(0xFF), ops[1].Buf[11], "mapped sentinel byte 11")
	assert.Equal(t, []byte{127, 0, 0, 2}, ops[1].Buf[12:16], "embedded IPv4 in mapped address")
}

// TestApplyNextHopMod_IPv6EmitsOnlyMPReach verifies that applyNextHopMod
// with an IPv6 local address emits only an MP_REACH_NLRI (type 14) op.
//
// VALIDATES: IPv6 local does not emit a legacy NEXT_HOP op.
// PREVENTS: Invalid 16-byte value in a 4-byte NEXT_HOP attribute.
func TestApplyNextHopMod_IPv6EmitsOnlyMPReach(t *testing.T) {
	t.Parallel()

	dest := &PeerSettings{
		NextHopMode:  NextHopSelf,
		LocalAddress: netip.MustParseAddr("2001:db8::1"),
	}
	var mods filterapi.ModAccumulator
	applyNextHopMod(dest, &mods)

	ops := mods.Ops()
	require.Len(t, ops, 2, "IPv6 local emits MP_REACH op + PrefixSID suppress")
	assert.Equal(t, uint8(14), ops[0].Code, "op is MP_REACH_NLRI")
	assert.Len(t, ops[0].Buf, 16, "IPv6 next-hop is 16 bytes")
	assert.Equal(t, uint8(40), ops[1].Code, "RFC 9252 S3.3: PrefixSID suppress")
	assert.Equal(t, filterapi.AttrModSuppress, ops[1].Action)
}

// TestApplyNextHopModIPv6LinkLocalBytes verifies that IPv6 next-hop self with a
// link-local address emits the RFC 2545 global plus link-local 32-byte value.
//
// VALIDATES: AC-1, IPv6 link-local next-hop value is global IPv6 followed by link-local IPv6.
// PREVENTS: Forwarding IPv6 next-hop-self routes with only half of the required next-hop value.
func TestApplyNextHopModIPv6LinkLocalBytes(t *testing.T) {
	t.Parallel()

	globalAddr := netip.MustParseAddr("2001:db8::1")
	linkLocalAddr := netip.MustParseAddr("fe80::1")
	dest := &PeerSettings{
		NextHopMode:  NextHopSelf,
		LocalAddress: globalAddr,
		LinkLocal:    linkLocalAddr,
	}
	var mods filterapi.ModAccumulator
	applyNextHopMod(dest, &mods)

	ops := mods.Ops()
	require.Len(t, ops, 2, "IPv6 link-local emits MP_REACH op + PrefixSID suppress")
	require.Equal(t, uint8(14), ops[0].Code, "op is MP_REACH_NLRI")
	require.Len(t, ops[0].Buf, 32, "RFC 2545 global plus link-local next-hop is 32 bytes")

	global := globalAddr.As16()
	linkLocal := linkLocalAddr.As16()
	assert.Equal(t, global[:], ops[0].Buf[:16], "first half is global IPv6 next-hop")
	assert.Equal(t, linkLocal[:], ops[0].Buf[16:], "second half is link-local IPv6 next-hop")
	assert.Equal(t, uint8(40), ops[1].Code, "RFC 9252 S3.3: PrefixSID suppress")
	assert.Equal(t, filterapi.AttrModSuppress, ops[1].Action)
}

// TestApplyNextHopModIPv6LinkLocalNoAlloc verifies that the fixed-size
// global plus link-local value does not allocate after accumulator storage is warm.
//
// VALIDATES: AC-2 and AC-4, IPv6 link-local next-hop construction uses caller-owned storage.
// PREVENTS: Reintroducing per-forward heap allocation for fixed-size next-hop bytes.
func TestApplyNextHopModIPv6LinkLocalNoAlloc(t *testing.T) {

	dest := &PeerSettings{
		NextHopMode:  NextHopSelf,
		LocalAddress: netip.MustParseAddr("2001:db8::1"),
		LinkLocal:    netip.MustParseAddr("fe80::1"),
	}
	var mods filterapi.ModAccumulator
	mods.Op(0, filterapi.AttrModSuppress, nil)
	mods.Op(0, filterapi.AttrModSuppress, nil)
	mods.Reset()

	allocs := testing.AllocsPerRun(1000, func() {
		mods.Reset()
		applyNextHopMod(dest, &mods)
	})
	assert.Equal(t, 0.0, allocs, "next-hop value construction should not allocate")
}
