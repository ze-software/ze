// Design: docs/architecture/wire/attributes.md — AGGREGATOR survival across an AS_PATH prepend
// RFC: rfc/short/rfc4271.md — AGGREGATOR is optional transitive (Section 5.1.7)
// RFC: rfc/short/rfc6793.md — AGGREGATOR/AS4_AGGREGATOR transcoding (Section 4.2.2)

package wireu

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// buildProbePayload assembles an UPDATE body from a packed attribute section.
func buildProbePayload(attrs, nlri []byte) []byte {
	body := make([]byte, 0, 4+len(attrs)+len(nlri))
	// withdrawn routes length = 0, then the attribute section length.
	body = append(body, 0, 0, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlri...)
	return body
}

// probeAttr packs one attribute with a 3-octet header.
func probeAttr(flags byte, code attribute.AttributeCode, value []byte) []byte {
	out := []byte{flags, byte(code), byte(len(value))}
	return append(out, value...)
}

// probeASPath2 packs a two-octet AS_SEQUENCE value.
func probeASPath2(asns ...uint16) []byte {
	val := []byte{byte(attribute.ASSequence), byte(len(asns))}
	for _, a := range asns {
		val = append(val, byte(a>>8), byte(a))
	}
	return val
}

// probeAggregator6 packs a two-octet-ASN AGGREGATOR value (ASN(2) + IPv4(4)).
func probeAggregator6(asn uint16, ip [4]byte) []byte {
	val := []byte{byte(asn >> 8), byte(asn)}
	return append(val, ip[:]...)
}

// findProbeAttr returns the first attribute with the given code in a packed
// UPDATE body, or nil.
func findProbeAttr(t *testing.T, payload []byte, code attribute.AttributeCode) []byte {
	t.Helper()
	require.GreaterOrEqual(t, len(payload), 4)
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	require.GreaterOrEqual(t, len(payload), attrLenOff+2)
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	start := attrLenOff + 2
	end := start + attrLen
	require.GreaterOrEqual(t, len(payload), end)
	for off := start; off < end; {
		require.LessOrEqual(t, off+3, end)
		flags := attribute.AttributeFlags(payload[off])
		got := attribute.AttributeCode(payload[off+1])
		length := int(payload[off+2])
		hdr := 3
		if flags.IsExtLength() {
			length = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
			hdr = 4
		}
		require.LessOrEqual(t, off+hdr+length, end)
		if got == code {
			return payload[off : off+hdr+length]
		}
		off += hdr + length
	}
	return nil
}

// VALIDATES: RFC 4271 Section 5.1.7 -- AGGREGATOR is optional TRANSITIVE, so a
// speaker that propagates a route carries it through unchanged when nothing
// about it needs to change.
// PREVENTS: the slow-path AGGREGATOR destruction described below silently
// returning.
//
// rewritePrependASPathFull computes newAggValueLen only when the source and
// destination ASN widths DIFFER. It then treated a zero newAggValueLen as "this
// AGGREGATOR is malformed" and overwrote it with an ATTR_TOMBSTONE marker
// carrying TombstoneInvalidLength -- so every same-encoding prepend that reached
// the slow path destroyed a perfectly valid AGGREGATOR. A dual-AS local-as
// prepend is the ordinary trigger, and whether the attribute survived depended
// only on whether the prepend took the byte-shifting fast path.
//
// The zero now means what it says: no re-encoding is required, so the attribute
// travels on untouched. Only a length that is neither 6 nor 8 is malformed.
func TestPrependKeepsValidAggregatorOnEveryPath(t *testing.T) {
	ip := [4]byte{192, 0, 2, 1}
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	attrs = append(attrs, probeAttr(0x40, attribute.AttrASPath, probeASPath2(64500, 64501))...)
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAggregator, probeAggregator6(64500, ip))...)
	payload := buildProbePayload(attrs, []byte{24, 192, 0, 2})

	// Sanity: the AGGREGATOR is present and well formed on the way in.
	require.NotNil(t, findProbeAttr(t, payload, attribute.AttrAggregator))

	// Separate buffers: the extracted attribute is a WINDOW into dst, so reusing
	// one buffer would leave the first result aliasing the second run's bytes.
	dstFast := make([]byte, 4096)
	dstSlow := make([]byte, 4096)

	// Fast path: single mappable ASN, same encoding, leading AS_SEQUENCE.
	nFast, err := RewriteASPath(dstFast, payload, 64510, false, false)
	require.NoError(t, err)
	fast := findProbeAttr(t, dstFast[:nFast], attribute.AttrAggregator)
	require.NotNil(t, fast, "fast path must carry the AGGREGATOR through")
	require.Equal(t, byte(0xC0), fast[0], "fast path must not change the AGGREGATOR flags")

	// Slow path, SAME encoding: a dual-AS prepend is two ASNs, which
	// tryDirectPrepend refuses, so rewritePrependASPathFull runs.
	nSlow, err := RewriteASPathDual(dstSlow, payload, 64510, 64500, false, false)
	require.NoError(t, err)
	dst := dstSlow

	slow := findProbeAttr(t, dst[:nSlow], attribute.AttrAggregator)
	require.NotNil(t, slow,
		"the slow path must carry a valid AGGREGATOR through: it is optional "+
			"transitive and nothing about it needed re-encoding")
	require.Equal(t, byte(0xC0), slow[0], "the AGGREGATOR flags are unchanged")
	require.Equal(t, fast, slow,
		"the same route must emit the same AGGREGATOR whichever prepend path it took")

	require.Nil(t, findProbeAttr(t, dst[:nSlow], attribute.AttrTombstone),
		"a well-formed AGGREGATOR must never be replaced by an ATTR_TOMBSTONE")
}

// VALIDATES: an AGGREGATOR whose length is genuinely unreadable is still
// tombstoned, so the fix above narrowed the branch rather than removing it.
// PREVENTS: the correction being read as "never tombstone an AGGREGATOR", which
// would forward a malformed attribute to a peer.
func TestPrependTombstonesMalformedAggregator(t *testing.T) {
	// Five octets: neither the two-octet-ASN (6) nor four-octet-ASN (8) form.
	attrs := probeAttr(0x40, attribute.AttrASPath, probeASPath2(64500, 64501))
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAggregator, []byte{1, 2, 3, 4, 5})...)
	payload := buildProbePayload(attrs, nil)

	dst := make([]byte, 4096)
	n, err := RewriteASPathDual(dst, payload, 64510, 64500, false, false)
	require.NoError(t, err)

	require.Nil(t, findProbeAttr(t, dst[:n], attribute.AttrAggregator),
		"a malformed AGGREGATOR does not travel on")
	tomb := findProbeAttr(t, dst[:n], attribute.AttrTombstone)
	require.NotNil(t, tomb, "it is replaced by a marker")
	require.Equal(t, byte(attribute.AttrAggregator), tomb[3])
	require.Equal(t, TombstoneInvalidLength, tomb[4])
}

// VALIDATES: draft-mangin-idr-attr-tombstone-00 Section 5.3 -- a recognizing
// EBGP speaker MUST clear the Transitive bit before forwarding the marker.
// PREVENTS: the clear riding on one prepend path only. It used to fire inside
// rewritePrependASPathFull alone, so a plain single-ASN prepend (the common EBGP
// case, which takes the byte-shifting fast path) forwarded the marker with its
// Transitive bit intact and let the peer propagate it further.
func TestTombstoneTransitiveClearedOnEveryPrependPath(t *testing.T) {
	// Value is (original code, reason, padding): at least two octets.
	marker := probeAttr(0xC0, attribute.AttrTombstone, []byte{byte(attribute.AttrMED), 1, 0, 0})

	cases := []struct {
		name string
		run  func(dst, payload []byte) (int, error)
		with []byte
	}{
		{
			name: "fast path: one mappable ASN, matching widths",
			run: func(dst, payload []byte) (int, error) {
				return RewriteASPath(dst, payload, 64510, false, false)
			},
			with: probeAttr(0x40, attribute.AttrASPath, probeASPath2(64500)),
		},
		{
			name: "slow path: dual-AS prepend",
			run: func(dst, payload []byte) (int, error) {
				return RewriteASPathDual(dst, payload, 64510, 64500, false, false)
			},
			with: probeAttr(0x40, attribute.AttrASPath, probeASPath2(64500)),
		},
		{
			name: "insert path: no AS_PATH present",
			run: func(dst, payload []byte) (int, error) {
				return RewriteASPath(dst, payload, 64510, false, false)
			},
			with: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
			attrs = append(attrs, tc.with...)
			attrs = append(attrs, marker...)
			payload := buildProbePayload(attrs, nil)

			// The marker arrives WITH the Transitive bit set.
			in := findProbeAttr(t, payload, attribute.AttrTombstone)
			require.NotNil(t, in)
			require.NotZero(t, in[0]&byte(attribute.FlagTransitive),
				"guard: the fixture must carry a transitive marker, or this proves nothing")

			dst := make([]byte, 4096)
			n, err := tc.run(dst, payload)
			require.NoError(t, err)

			out := findProbeAttr(t, dst[:n], attribute.AttrTombstone)
			require.NotNil(t, out, "the marker itself still travels to the peer")
			require.Zero(t, out[0]&byte(attribute.FlagTransitive),
				"the Transitive bit MUST be cleared at the EBGP boundary")
		})
	}
}

// VALIDATES: the same AGGREGATOR destruction is reachable through the
// transcode-only rail whenever the widths already match.
// PREVENTS: reading the defect as specific to the dual-AS prepend.
//
// TranscodeASPath returns 0 when srcASN4 == dstASN4, so the transcode rail never
// reaches its own AGGREGATOR branch with newAggValueLen == 0 in that case. This
// pins that the transcode rail is CLEAN where the prepend rail is not, which is
// what makes the prepend rail's branch a defect rather than a shared convention.
func TestTranscodeNoOpLeavesAggregatorAlone(t *testing.T) {
	ip := [4]byte{192, 0, 2, 1}
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	attrs = append(attrs, probeAttr(0x40, attribute.AttrASPath, probeASPath2(64500))...)
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAggregator, probeAggregator6(64500, ip))...)
	payload := buildProbePayload(attrs, nil)

	dst := make([]byte, 4096)
	n, err := TranscodeASPath(dst, payload, false, false)
	require.NoError(t, err)
	require.Zero(t, n, "matching widths need no transcode, so nothing is rewritten")
}
