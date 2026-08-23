package reactor

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// mustHex decodes a hex fixture or fails the test.
func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err, "fixture hex must decode")
	return b
}

// buildRelay is the test-side driver: it scans the stored attribute block, sizes
// the payload, and writes it, returning the reconstructed UPDATE body.
func buildRelay(t *testing.T, attrs, nextHop, nlri []byte, fam family.Family) []byte {
	t.Helper()
	spans, ok := scanAttrBlock(nil, attrs)
	require.True(t, ok, "attribute block must scan")
	need := relayNeedsNextHopAttr(spans, fam)
	size, ok := relayPayloadLen(spans, nextHop, nlri, fam, need)
	require.True(t, ok, "payload must be sizeable")
	buf := make([]byte, size+16) // slack proves the writer returns the exact length
	n := writeRelayPayload(buf, 0, spans, attrs, nextHop, nlri, fam, need)
	// EXACT, not a bound. relayPayloadLen's contract is "the exact byte count
	// writeRelayPayload will produce". buildRelayUpdate spends it as one: it
	// takes a pool buffer sized against it, then hands the WireUpdate buf[:n].
	// So a SHORT write overflows nothing. It publishes a truncated UPDATE body.
	// The writer back-fills the attribute-length field from its own offset, so
	// the truncation agrees with itself and nothing downstream can tell.
	// `n <= size` accepted exactly that (ai/rules/evidence.md).
	require.Equal(t, size, n, "writer must produce the sized length exactly")
	return buf[:n]
}

// TestRelayPayloadIPv4ReproducesReceivedWire verifies the reconstructed body is
// byte-identical to the body the source peer sent, for the ordinary IPv4 unicast
// shape.
//
// VALIDATES: a replayed IPv4 unicast route re-enters the forward rail with the
// SAME wire the source sent, so forwardUpdateCore's egress steps see exactly what
// a live forward would see (spec AC-1 depends on this).
// PREVENTS: attribute re-ordering or a re-synthesized NEXT_HOP changing the bytes,
// which would break every exact-hex functional expectation.
func TestRelayPayloadIPv4ReproducesReceivedWire(t *testing.T) {
	// The received UPDATE body from test/plugin/remove-private-as-replace-peer.ci:
	// withdrawn-len=0000, attr-len=001C, ORIGIN/AS_PATH/NEXT_HOP, NLRI 10.0.0.0/24.
	body := mustHex(t, "0000001C4001010040020E02030000FBF00000FC000000FBF140030401010101180A0000")

	wu := wireu.NewWireUpdate(body, 0)
	attrsWire, err := wu.Attrs()
	require.NoError(t, err)
	nlriBytes, err := wu.NLRI()
	require.NoError(t, err)

	got := buildRelay(t, attrsWire.Packed(), mustHex(t, "01010101"), nlriBytes, family.IPv4Unicast)

	require.Equal(t, hex.EncodeToString(body), hex.EncodeToString(got),
		"reconstructed IPv4 unicast body must equal the received body byte for byte")
}

// TestRelayPayloadStripsStoredMPReach verifies the stored attribute block's
// MP_REACH_NLRI is removed and replaced by a single-NLRI one.
//
// VALIDATES: spec A-1 (BROKEN) -- the stored block carries the source UPDATE's
// whole MP_REACH, so replay must strip it and synthesize a per-route attribute.
// PREVENTS: a replayed IPv6 route re-announcing every prefix of the originating
// UPDATE, and emitting two MP_REACH attributes in one message.
func TestRelayPayloadStripsStoredMPReach(t *testing.T) {
	// Source UPDATE body: ORIGIN, AS_PATH, then MP_REACH (IPv6 unicast) carrying
	// TWO prefixes -- 2001:db8::/32 and 2001:db9::/32 -- with next-hop 2001:db8::1.
	nh := "20010db8000000000000000000000001"
	mpValue := "0002" + "01" + "10" + nh + "00" + "202001" + "0db8" + "202001" + "0db9"
	mpValueBytes := mustHex(t, mpValue)
	mpAttr := "800e" + hexByte(len(mpValueBytes)) + mpValue
	attrs := mustHex(t, "4001010040020602010000FBF1"+mpAttr)

	spans, ok := scanAttrBlock(nil, attrs)
	require.True(t, ok, "attribute block with MP_REACH must scan")
	require.Len(t, spans, 3, "ORIGIN, AS_PATH, MP_REACH")

	// Replay only the FIRST prefix, as adj-rib-in stores routes one NLRI at a time.
	oneNLRI := mustHex(t, "20"+"20010db8")
	got := buildRelay(t, attrs, mustHex(t, nh), oneNLRI, family.IPv6Unicast)

	out := wireu.NewWireUpdate(got, 0)
	outAttrs, err := out.Attrs()
	require.NoError(t, err)

	outSpans, ok := scanAttrBlock(nil, outAttrs.Packed())
	require.True(t, ok)
	mpCount := 0
	for _, s := range outSpans {
		if s.code == attribute.AttrMPReachNLRI {
			mpCount++
		}
	}
	require.Equal(t, 1, mpCount, "exactly one MP_REACH_NLRI must survive reconstruction")

	mp, err := out.MPReach()
	require.NoError(t, err)
	require.NotNil(t, mp, "reconstructed body must carry MP_REACH")
	require.Equal(t, family.IPv6Unicast, mp.Family())
	require.Equal(t, hex.EncodeToString(oneNLRI), hex.EncodeToString(mp.NLRIBytes()),
		"MP_REACH must carry only the replayed route's NLRI, not the source UPDATE's whole set")

	body, err := out.NLRI()
	require.NoError(t, err)
	require.Empty(t, body, "a non-IPv4-unicast replay must not put NLRI in the body field")
}

// TestRelayPayloadAddsNextHopWhenIPv4CameViaMPReach verifies the IPv4-unicast
// reconstruction re-adds a legacy NEXT_HOP when the source used MP_REACH.
//
// VALIDATES: RFC 4271 Section 5.1.3 -- NEXT_HOP is well-known mandatory on the
// legacy IPv4 unicast encoding the relay emits.
// PREVENTS: emitting an IPv4 unicast UPDATE with no NEXT_HOP after stripping the
// MP_REACH that carried it, which peers reject.
func TestRelayPayloadAddsNextHopWhenIPv4CameViaMPReach(t *testing.T) {
	// ORIGIN, AS_PATH, MP_REACH(ipv4/unicast, nh 1.1.1.1, 10.0.0.0/24). No type-3.
	mpValue := "0001" + "01" + "04" + "01010101" + "00" + "180A0000"
	mpValueBytes := mustHex(t, mpValue)
	attrs := mustHex(t, "4001010040020602010000FBF1"+"800e"+hexByte(len(mpValueBytes))+mpValue)

	got := buildRelay(t, attrs, mustHex(t, "01010101"), mustHex(t, "180A0000"), family.IPv4Unicast)

	out := wireu.NewWireUpdate(got, 0)
	outAttrs, err := out.Attrs()
	require.NoError(t, err)

	raw, err := outAttrs.GetRaw(attribute.AttrNextHop)
	require.NoError(t, err)
	require.NotEmpty(t, raw, "IPv4 unicast reconstruction must carry a legacy NEXT_HOP")

	nlriBytes, err := out.NLRI()
	require.NoError(t, err)
	require.Equal(t, "180a0000", hex.EncodeToString(nlriBytes),
		"IPv4 unicast NLRI belongs in the body field")

	outSpans, ok := scanAttrBlock(nil, outAttrs.Packed())
	require.True(t, ok)
	for _, s := range outSpans {
		require.NotEqual(t, attribute.AttrMPReachNLRI, s.code,
			"the stored MP_REACH must not survive an IPv4 unicast reconstruction")
	}
}

// TestRelayPayloadKeepsSourceAttributeOrder verifies the synthesized MP_REACH
// takes the POSITION of the stripped one instead of being appended.
//
// VALIDATES: writeRelayPayload's contract that "attribute order from the source
// is preserved for every surviving attribute" holds for attributes the source
// placed AFTER MP_REACH.
// PREVENTS: the two-rail byte divergence. A live forward relays the source's own
// bytes; this rail reconstructs. Appending the synthesized MP_REACH moved every
// later attribute in front of it, so the same route left the daemon as two
// different byte strings depending only on whether the destination peer was an
// established forward target yet. Caught as test/plugin/role-otc-unicast-scope.ci
// receiving ORIGIN, AS_PATH, OTC, MP_REACH against the ORIGIN, AS_PATH, MP_REACH,
// OTC its source peer sent.
func TestRelayPayloadKeepsSourceAttributeOrder(t *testing.T) {
	// The source UPDATE from test/plugin/role-otc-unicast-scope.ci: ORIGIN,
	// AS_PATH, MP_REACH (ipv4/multicast, nh 1.1.1.1, 10.0.0.0/24), then OTC(35)
	// AFTER the MP_REACH.
	mpValue := "0001" + "02" + "04" + "01010101" + "00" + "180A0000"
	otc := "C023040000FDE9"
	attrs := mustHex(t, "4001010040020602010000FDE9"+"800E0D"+mpValue+otc)

	got := buildRelay(t, attrs, mustHex(t, "01010101"), mustHex(t, "180A0000"), family.IPv4Multicast)

	out := wireu.NewWireUpdate(got, 0)
	outAttrs, err := out.Attrs()
	require.NoError(t, err)
	outSpans, ok := scanAttrBlock(nil, outAttrs.Packed())
	require.True(t, ok)

	codes := make([]attribute.AttributeCode, 0, len(outSpans))
	for _, s := range outSpans {
		codes = append(codes, s.code)
	}
	require.Equal(t,
		[]attribute.AttributeCode{
			attribute.AttrOrigin,
			attribute.AttrASPath,
			attribute.AttrMPReachNLRI,
			attribute.AttributeCode(35),
		},
		codes,
		"the synthesized MP_REACH must sit where the source's did, before OTC")
}

// TestScanAttrBlockRejectsMalformed verifies the walker fails closed.
//
// VALIDATES: spec S-1/S-2 -- attacker-shaped attribute bytes must be rejected
// whole, never partially re-emitted onto a peer session.
// PREVENTS: a truncated length field producing an out-of-bounds read or a
// half-copied attribute block on the wire.
func TestScanAttrBlockRejectsMalformed(t *testing.T) {
	cases := []struct {
		name  string
		block string
	}{
		{"header truncated", "4001"},
		{"value truncated", "40010A00"},
		{"extended length truncated", "50010100"},
		{"extended length overruns", "5001FFFF00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := scanAttrBlock(nil, mustHex(t, tc.block))
			require.False(t, ok, "malformed attribute block must be rejected")
		})
	}
}

// TestStoredHexDecodeRejectsMalformed verifies a malformed stored-route hex
// field fails the route rather than being partially accepted.
//
// the subject function decodeHexInto was REMOVED, so its two tests
// cannot remain as written. Its justification was false and was disproved by
// measurement: hex.Decode does not leak src, so hex.Decode(dst, []byte(s)) is
// elided to a zero-copy conversion and allocates ZERO per call (AllocsPerRun and
// -gcflags=-m). TestDecodeHexIntoDoesNotAllocate is DELETED rather than ported
// because it could not fail -- the form it claimed to guard against allocates
// nothing either, so it pinned nothing. The malformed-input boundaries are NOT
// dropped: they are re-pointed at encoding/hex, the call buildRelayUpdate now
// makes, and strengthened with a high-bit case the old table never reached (its
// "unicode digit" case was rejected on length, never on alphabet).
//
// VALIDATES: spec S-5 -- stored-route hex crosses the plugin RPC boundary and is
// untrusted, so a length or alphabet defect must fail the route.
// PREVENTS: an odd-length or non-hex field being accepted and reaching the wire.
func TestStoredHexDecodeRejectsMalformed(t *testing.T) {
	// decodeField mirrors buildRelayUpdate: size with DecodedLen, decode, and
	// treat any error as "reject this route".
	decodeField := func(s string) bool {
		dst := make([]byte, hex.DecodedLen(len(s)))
		_, err := hex.Decode(dst, []byte(s))
		return err == nil
	}

	t.Run("round-trips valid hex in both cases", func(t *testing.T) {
		dst := make([]byte, 4)
		_, err := hex.Decode(dst, []byte("0a0B0c0D"))
		require.NoError(t, err)
		require.Equal(t, []byte{0x0a, 0x0b, 0x0c, 0x0d}, dst)
	})

	t.Run("empty decodes to empty", func(t *testing.T) {
		require.True(t, decodeField(""), "a zero-length field is valid")
	})

	cases := []struct {
		name string
		s    string
	}{
		{"odd length", "0a0"},
		{"non-hex alphabet", "0azz"},
		{"leading space", " a0b"},
		{"high-bit byte", "0a\xff\xfe"},
		{"multi-byte utf8 digit", "0a0٠"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.False(t, decodeField(tc.s), "malformed hex must fail the route")
		})
	}
}

// hexByte renders a single byte as two hex digits for fixture assembly.
func hexByte(n int) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[(n>>4)&0xF], digits[n&0xF]})
}
