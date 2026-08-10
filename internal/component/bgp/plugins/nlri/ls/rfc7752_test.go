// RFC: rfc/short/rfc7752.md — BGP-LS obligations gated by make ze-rfc-check
//
// Ze is a BGP-LS consumer and transit speaker: it decodes AFI 16388 NLRI and
// the type-29 attribute, and forwards received UPDATEs verbatim. It originates
// no BGP-LS from an IGP, so the tests here pin the decode side and the encoding
// form of the only BGP-LS encoders ze owns (types_nlri.go, types_descriptor.go,
// attr_*.go).

package ls

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unknownAttrTLVType is a TLV type code with no registered decoder. Chosen from
// the unassigned range so an IANA allocation that ze someday registers cannot
// turn this test green for the wrong reason.
const unknownAttrTLVType uint16 = 0x7FFF

// buildAttrTLV appends a TLV (type, length, value) to dst.
func buildAttrTLV(dst []byte, tlvType uint16, value []byte) []byte {
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint16(hdr[0:], tlvType)
	binary.BigEndian.PutUint16(hdr[2:], uint16(len(value)))
	dst = append(dst, hdr...)
	return append(dst, value...)
}

// nodeNLRIWithUnknownSubTLV builds a Node NLRI (type 1) whose Local Node
// Descriptors carry a known sub-TLV (512, AS) followed by a sub-TLV ze has no
// decoder for.
func nodeNLRIWithUnknownSubTLV() []byte {
	var descs []byte
	descs = buildAttrTLV(descs, TLVAutonomousSystem, []byte{0x00, 0x00, 0xFD, 0xE8}) // AS 65000
	descs = buildAttrTLV(descs, unknownAttrTLVType, []byte{0xDE, 0xAD})

	var container []byte
	container = buildAttrTLV(container, TLVLocalNodeDesc, descs)

	body := make([]byte, 9, 9+len(container))
	body[0] = byte(ProtoOSPFv2)
	binary.BigEndian.PutUint64(body[1:], 7)
	body = append(body, container...)

	nlri := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint16(nlri[0:], uint16(BGPLSNodeNLRI))
	binary.BigEndian.PutUint16(nlri[2:], uint16(len(body)))
	return append(nlri, body...)
}

// TestRFC7752UnknownTLVPreservedAndPropagated proves an unrecognized TLV
// survives both halves of ze's BGP-LS path: the attribute decoder keeps its
// bytes under a generic key, and a parsed NLRI re-encodes byte-identically
// because ParseBGPLS caches the wire slice (types.go:333) and WriteTo copies it
// back out (types_nlri.go:63).
//
// VALIDATES: forward compatibility -- ze never drops TLVs it cannot name.
// PREVENTS: an unknown TLV being silently stripped on re-advertisement.
func TestRFC7752UnknownTLVPreservedAndPropagated(t *testing.T) {
	// RFC requirement: RFC7752-3.1-1 positive -- an unrecognized attribute TLV is kept verbatim and an unrecognized NLRI sub-TLV survives decode/re-encode unchanged (§3.1)
	// RFC requirement: RFC9552-5.1-3 positive -- the unknown type is preserved and propagated in both halves: the attribute keeps its bytes and the NLRI re-encodes identically (§5.1)
	var attr []byte
	attr = buildAttrTLV(attr, TLVIPv4RouterIDLocal, []byte{10, 0, 0, 1})
	attr = buildAttrTLV(attr, unknownAttrTLVType, []byte{0xAA, 0xBB, 0xCC})

	js := AttrTLVsToJSON(attr)
	assert.Equal(t, []string{"10.0.0.1"}, js["local-router-ids"], "known TLV decodes")
	assert.Equal(t, []string{"0xAABBCC"}, js[genericTLVKey(unknownAttrTLVType)],
		"unknown TLV value is preserved verbatim under a generic key")

	tlvs, err := decodeAllAttrTLVs(attr)
	require.NoError(t, err, "an unknown TLV is not an error")
	require.Len(t, tlvs, 1, "only the recognized TLV is typed")

	wire := nodeNLRIWithUnknownSubTLV()
	parsed, err := ParseBGPLS(wire)
	require.NoError(t, err)
	node, ok := parsed.(*BGPLSNode)
	require.True(t, ok, "type 1 NLRI parses as a Node NLRI")
	assert.Equal(t, uint32(65000), node.LocalNode.ASN, "known sub-TLV decodes")

	out := make([]byte, parsed.Len())
	n := parsed.WriteTo(out, 0)
	assert.Equal(t, len(wire), n)
	assert.Equal(t, wire, out[:n],
		"re-encoded NLRI is byte-identical, so the unknown sub-TLV propagates")
}

// TestRFC7752MalformedTLVNotPreserved is the counter-case: preservation stops
// at the syntax boundary. A TLV whose declared length runs past the buffer, and
// a fixed-length TLV of the wrong size, are rejected rather than carried along.
//
// VALIDATES: IterateAttrTLVs (attr.go:84) and the fixed-length decoders reject
// malformed input instead of preserving it.
// PREVENTS: a length-overrun TLV being treated as an opaque unknown and echoed.
func TestRFC7752MalformedTLVNotPreserved(t *testing.T) {
	// RFC requirement: RFC7752-3.1-1 negative -- a TLV whose declared length overruns the buffer is rejected, not preserved as an unknown TLV (§3.1)
	// RFC requirement: RFC7752-6.2.2-2 negative -- a TLV length sum exceeding the attribute length and a wrong-sized fixed-length TLV are both refused (§6.2.2)
	// RFC requirement: RFC9552-5.1-3 negative -- preservation covers unknown types, not broken framing: a TLV whose declared length overruns the attribute is refused rather than propagated (§5.1)
	truncated := make([]byte, 6)
	binary.BigEndian.PutUint16(truncated[0:], unknownAttrTLVType)
	binary.BigEndian.PutUint16(truncated[2:], 10) // claims 10 value octets, 2 present
	truncated[4], truncated[5] = 0xFF, 0xFF

	err := iterateAttrTLVs(truncated, func(attrTLVEntry) bool {
		t.Fatal("a truncated TLV must not be yielded")
		return false
	})
	assert.ErrorIs(t, err, ErrBGPLSTruncated, "TLV sum beyond the attribute length is refused")

	_, err = decodeAllAttrTLVs(truncated)
	assert.ErrorIs(t, err, ErrBGPLSTruncated)

	assert.Empty(t, AttrTLVsToJSON(truncated),
		"a malformed TLV is not preserved under a generic key")

	// Fixed-length TLV 1028 (IPv4 Router-ID) with three value octets.
	_, err = decodeAttrTLV(attrTLVEntry{Type: TLVIPv4RouterIDLocal, Value: []byte{10, 0, 0}})
	assert.ErrorIs(t, err, ErrBGPLSTruncated, "fixed-length TLV of the wrong size is refused")
}

// TestRFC7752AttrSyntacticChecks proves the positive half of the Section 6.2.2
// syntactic checks: a TLV sequence whose lengths sum exactly to the attribute
// length is walked completely, and a fixed-length TLV of the specified size is
// accepted.
//
// VALIDATES: IterateAttrTLVs consumes an exact-sum TLV sequence.
// PREVENTS: an off-by-one in the TLV walk silently dropping the last TLV.
func TestRFC7752AttrSyntacticChecks(t *testing.T) {
	// RFC requirement: RFC7752-6.2.2-2 positive -- a TLV sequence whose lengths sum exactly to the attribute length, with fixed-length TLVs at their specified size, decodes completely (§6.2.2)
	var attr []byte
	attr = buildAttrTLV(attr, TLVIPv4RouterIDLocal, []byte{192, 0, 2, 1}) // fixed 4
	attr = buildAttrTLV(attr, TLVNodeName, []byte("core1"))               // variable
	attr = buildAttrTLV(attr, TLVIGPMetric, []byte{0x00, 0x0A})           // 2-octet OSPF metric

	var types []uint16
	var consumed int
	err := iterateAttrTLVs(attr, func(e attrTLVEntry) bool {
		types = append(types, e.Type)
		consumed += 4 + len(e.Value)
		return true
	})
	require.NoError(t, err)
	assert.Equal(t, []uint16{TLVIPv4RouterIDLocal, TLVNodeName, TLVIGPMetric}, types)
	assert.Equal(t, len(attr), consumed, "TLV lengths sum exactly to the attribute length")

	tlvs, err := decodeAllAttrTLVs(attr)
	require.NoError(t, err)
	assert.Len(t, tlvs, 3)
}

// TestRFC7752NodeDescriptorSubTLVsAscending pins the sub-TLV order emitted by
// NodeDescriptor.WriteTo (types_descriptor.go:98): 512, 513, 514, 515, 516, 517
// in strictly ascending type order, independent of struct field order.
//
// VALIDATES: node descriptor sub-TLV ordering on encode.
// PREVENTS: a reordered write sequence breaking the binary comparability that
// RFC 7752 Section 3.2.1.4 relies on.
func TestRFC7752NodeDescriptorSubTLVsAscending(t *testing.T) {
	// RFC requirement: RFC7752-3.2.1.4-3 positive -- node descriptor sub-TLVs are emitted in ascending sub-TLV type order (§3.2.1.4)
	// RFC requirement: RFC9552-5.2.1.4-2 positive -- the sub-TLVs within a Node Descriptor are arranged in ascending order by sub-TLV type (§5.2.1.4)
	nd := NodeDescriptor{
		ASN:             65001,
		BGPLSIdentifier: 1,
		OSPFAreaID:      2,
		IGPRouterID:     []byte{1, 2, 3, 4},
		BGPRouterID:     3,
		ConfedMember:    4,
	}
	buf := make([]byte, nd.Len())
	n := nd.WriteTo(buf, 0)
	require.Equal(t, nd.Len(), n)

	var types []uint16
	for off := 0; off+4 <= n; {
		tlvType := binary.BigEndian.Uint16(buf[off:])
		tlvLen := int(binary.BigEndian.Uint16(buf[off+2:]))
		types = append(types, tlvType)
		off += 4 + tlvLen
	}
	require.Len(t, types, 6)
	for i := 1; i < len(types); i++ {
		assert.Less(t, types[i-1], types[i],
			"sub-TLV %d (%d) must precede %d", i-1, types[i-1], types[i])
	}
	assert.Equal(t, []uint16{
		TLVAutonomousSystem, TLVBGPLSIdentifier, TLVOSPFAreaID,
		TLVIGPRouterID, TLVBGPRouterID, TLVConfedMember,
	}, types)
}

// TestRFC7752TEDefaultMetricZeroPadded proves ze's TE Default Metric encoder
// always emits the full 32-bit field, so a metric sourced from a narrower IGP
// width lands zero-padded in the high-order octets.
//
// VALIDATES: LsTEDefaultMetric.WriteTo (attr_link.go:226) writes 4 octets.
// PREVENTS: a width-guessing encoder emitting a short TE metric.
func TestRFC7752TEDefaultMetricZeroPadded(t *testing.T) {
	// RFC requirement: RFC7752-3.3.2.3-1 positive -- a metric narrower than 32 bits is emitted with zero-padded high-order octets in a 4-octet TE Default Metric TLV (§3.3.2.3)
	// RFC requirement: RFC9552-5.3.2.3-1 positive -- the high-order bits of the TE Default Metric are padded with zero when the source metric is narrower than 32 bits (§5.3.2.3)
	for _, metric := range []uint32{0, 1, 0x3F, 0xFFFF, 0xFFFFFF} {
		tlv := &LsTEDefaultMetric{Metric: metric}
		buf := make([]byte, tlv.Len())
		n := tlv.WriteTo(buf, 0)
		require.Equal(t, 8, n, "4 octet header + 4 octet value")
		assert.Equal(t, TLVTEDefaultMetric, binary.BigEndian.Uint16(buf[0:]))
		assert.Equal(t, uint16(4), binary.BigEndian.Uint16(buf[2:]), "value is always 4 octets")
		assert.Equal(t, metric, binary.BigEndian.Uint32(buf[4:]))

		// High-order octets carry only the zero padding for narrow metrics.
		if metric <= 0xFFFF {
			assert.Equal(t, byte(0), buf[4], "high-order octet is zero padding")
			assert.Equal(t, byte(0), buf[5], "high-order octet is zero padding")
		}
	}
}

// TestRFC7752NonVPNFamilyIsAFI16388SAFI71 pins the non-VPN BGP-LS family to
// AFI 16388 / SAFI 71, both in the family registry (types.go:40) and in the
// decode-mode declaration the plugin publishes (plugin.go:70).
//
// VALIDATES: link, node and prefix information travels as (16388, 71).
// PREVENTS: a registry edit silently moving BGP-LS to another SAFI.
func TestRFC7752NonVPNFamilyIsAFI16388SAFI71(t *testing.T) {
	// RFC requirement: RFC7752-3.2-5 positive -- non-VPN link-state information uses AFI 16388 with SAFI 71 (§3.2)
	// RFC requirement: RFC9552-5.2-1 positive -- all non-VPN link, node and prefix information is encoded using AFI 16388 / SAFI 71 (§5.2)
	assert.Equal(t, AFI(16388), BGPLSFamily.AFI)
	assert.Equal(t, SAFI(71), BGPLSFamily.SAFI)
	assert.Equal(t, "bgp-ls/bgp-ls", BGPLSFamily.String())

	node := NewBGPLSNode(ProtoISISL2, 0, NodeDescriptor{ASN: 65001})
	assert.Equal(t, BGPLSFamily, node.Family(), "a Node NLRI belongs to (16388, 71)")

	assert.True(t, isValidBGPLSFamily("bgp-ls/bgp-ls"))
}

// TestRFC7752VPNFamilyIsAFI16388SAFI72 is the VPN counterpart: BGP-LS VPN
// information travels as AFI 16388 / SAFI 72 (types.go:41, plugin.go:71).
//
// VALIDATES: the VPN link-state family is a distinct SAFI, not a variant of 71.
// PREVENTS: VPN link-state being folded into the non-VPN SAFI.
func TestRFC7752VPNFamilyIsAFI16388SAFI72(t *testing.T) {
	// RFC requirement: RFC7752-3.2-6 positive -- VPN link-state information uses AFI 16388 with SAFI 72 (§3.2)
	// RFC requirement: RFC9552-5.2-2 positive -- VPN link, node and prefix information is encoded using AFI 16388 / SAFI 72 (§5.2)
	assert.Equal(t, AFI(16388), BGPLSVPNFamily.AFI)
	assert.Equal(t, SAFI(72), BGPLSVPNFamily.SAFI)
	assert.Equal(t, "bgp-ls/bgp-ls-vpn", BGPLSVPNFamily.String())
	assert.NotEqual(t, BGPLSFamily, BGPLSVPNFamily, "VPN and non-VPN are separate families")

	assert.True(t, isValidBGPLSFamily("bgp-ls/bgp-ls-vpn"))
}

// TestRFC7752NonLinkStateFamilyRefused is the negative for both SHALL clauses:
// the BGP-LS decoder entry points accept only the two registered link-state
// family names, so link-state NLRI cannot be carried under any other AFI/SAFI
// pair through ze.
//
// VALIDATES: isValidBGPLSFamily (plugin.go:245) gates every decode entry point.
// PREVENTS: a BGP-LS payload being decoded under, say, ipv4/unicast.
func TestRFC7752NonLinkStateFamilyRefused(t *testing.T) {
	// RFC requirement: RFC7752-3.2-5 negative -- a family that is not (16388, 71) is refused as a carrier of non-VPN link-state NLRI (§3.2)
	// RFC requirement: RFC7752-3.2-6 negative -- a family that is not (16388, 72) is refused as a carrier of VPN link-state NLRI (§3.2)
	// RFC requirement: RFC9552-5.2-1 negative -- a family that is not (16388, 71) is refused as a carrier of non-VPN link-state NLRI (§5.2)
	// RFC requirement: RFC9552-5.2-2 negative -- a family that is not (16388, 72) is refused as a carrier of VPN link-state NLRI (§5.2)
	for _, fam := range []string{"ipv4/unicast", "ipv6/unicast", "l2vpn/evpn", "bgp-ls/bgp-ls-vpn2", ""} {
		assert.False(t, isValidBGPLSFamily(fam), "family %q must not be treated as BGP-LS", fam)

		var out, errOut strBuf
		rc := runBGPLSCLIDecode("0002001b0300000000000000000100", fam, false, &out, &errOut)
		assert.Equal(t, 1, rc, "decode under family %q is refused", fam)
		assert.Empty(t, out.String(), "no NLRI is emitted for family %q", fam)
	}
}

// strBuf is a minimal io.Writer used to capture CLI output without pulling in
// bytes.Buffer semantics the assertions do not need.
type strBuf struct{ b []byte }

func (s *strBuf) Write(p []byte) (int, error) { s.b = append(s.b, p...); return len(p), nil }
func (s *strBuf) String() string              { return string(s.b) }
