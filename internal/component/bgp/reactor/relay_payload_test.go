package reactor

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
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
	require.LessOrEqual(t, n, size, "writer must not exceed the sized bound")
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

// hexByte renders a single byte as two hex digits for fixture assembly.
func hexByte(n int) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[(n>>4)&0xF], digits[n&0xF]})
}
