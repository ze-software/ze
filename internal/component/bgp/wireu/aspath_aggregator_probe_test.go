// Design: docs/architecture/wire/attributes.md — AGGREGATOR survival across an AS_PATH prepend
// RFC: rfc/short/rfc4271.md — AGGREGATOR is optional transitive (Section 5.1.7)
// RFC: rfc/short/rfc6793.md — AGGREGATOR/AS4_AGGREGATOR transcoding (Section 4.2.2)

package wireu

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
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
// about it needs to change. Both prepend paths agree.
// PREVENTS: the defect this file was written for, in the shape the live rail
// could still take it: a same-width prepend deciding the AGGREGATOR needs
// re-encoding, and destroying a valid one on whichever path it reached.
//
// The whole-payload rewrite this now drives through ASPathEdit computed a new
// AGGREGATOR length only when the widths DIFFERED, then read a zero length as
// "malformed" and overwrote the attribute with an ATTR_TOMBSTONE. So every
// same-encoding prepend that reached its slow path destroyed a valid AGGREGATOR,
// and survival depended only on which prepend path the route took. ASPathEdit
// answers the width question FIRST, in recordAggregator, and returns before any
// of that when the widths match -- so the attribute is not named in the edit set
// at all and the writer copies it verbatim.
func TestPrependKeepsValidAggregatorOnEveryPath(t *testing.T) {
	ip := [4]byte{192, 0, 2, 1}
	attrs := probeAttr(0x40, attribute.AttrOrigin, []byte{0})
	attrs = append(attrs, probeAttr(0x40, attribute.AttrASPath, probeASPath2(64500, 64501))...)
	attrs = append(attrs, probeAttr(0xC0, attribute.AttrAggregator, probeAggregator6(64500, ip))...)
	payload := buildProbePayload(attrs, []byte{24, 192, 0, 2})

	// Sanity: the AGGREGATOR is present and well formed on the way in.
	require.NotNil(t, findProbeAttr(t, payload, attribute.AttrAggregator))

	cases := []struct {
		name    string
		prepend []uint32
	}{
		// One mappable ASN onto a leading AS_SEQUENCE with matching widths: the
		// byte-shifting fast path (tryShift).
		{name: "fast path: one mappable ASN, matching widths", prepend: []uint32{64510}},
		// Two ASNs, which tryShift refuses, so the full prepend runs.
		{name: "slow path: dual-AS prepend", prepend: []uint32{64500, 64510}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mods filterapi.ModAccumulator
			var edit ASPathEdit
			_, err := edit.Record(&mods, payload, ASPathIntent{Prepend: tc.prepend})
			require.NoError(t, err)

			for _, op := range mods.Ops() {
				require.NotEqual(t, byte(attribute.AttrAggregator), op.Code,
					"a valid AGGREGATOR at matching widths needs no operation: it travels verbatim")
				require.NotEqual(t, byte(attribute.AttrTombstone), op.Code,
					"a well-formed AGGREGATOR must never be replaced by an ATTR_TOMBSTONE")
			}
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
