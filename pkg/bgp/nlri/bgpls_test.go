package nlri

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBGPLSNLRITypes verifies NLRI type constants.
func TestBGPLSNLRITypes(t *testing.T) {
	assert.Equal(t, BGPLSNLRIType(1), BGPLSNodeNLRI)
	assert.Equal(t, BGPLSNLRIType(2), BGPLSLinkNLRI)
	assert.Equal(t, BGPLSNLRIType(3), BGPLSPrefixV4NLRI)
	assert.Equal(t, BGPLSNLRIType(4), BGPLSPrefixV6NLRI)
	assert.Equal(t, BGPLSNLRIType(6), BGPLSSRv6SIDNLRI)
}

// TestBGPLSProtocolIDs verifies protocol ID constants.
func TestBGPLSProtocolIDs(t *testing.T) {
	assert.Equal(t, BGPLSProtocolID(1), ProtoISISL1)
	assert.Equal(t, BGPLSProtocolID(2), ProtoISISL2)
	assert.Equal(t, BGPLSProtocolID(3), ProtoOSPFv2)
	assert.Equal(t, BGPLSProtocolID(4), ProtoDirect)
	assert.Equal(t, BGPLSProtocolID(5), ProtoStatic)
	assert.Equal(t, BGPLSProtocolID(6), ProtoOSPFv3)
	assert.Equal(t, BGPLSProtocolID(7), ProtoBGP)
}

// TestBGPLSNodeDescriptor verifies node descriptor creation.
func TestBGPLSNodeDescriptor(t *testing.T) {
	nd := NodeDescriptor{
		ASN:             65000,
		BGPLSIdentifier: 0x12345678,
		OSPFAreaID:      0,
		IGPRouterID:     []byte{10, 0, 0, 1},
	}

	assert.Equal(t, uint32(65000), nd.ASN)
	assert.Equal(t, uint32(0x12345678), nd.BGPLSIdentifier)
	assert.Equal(t, []byte{10, 0, 0, 1}, nd.IGPRouterID)
}

// TestBGPLSNodeNLRI verifies node NLRI creation.
func TestBGPLSNodeNLRI(t *testing.T) {
	node := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	})

	assert.Equal(t, BGPLSNodeNLRI, node.NLRIType())
	assert.Equal(t, ProtoOSPFv2, node.ProtocolID())
	assert.Equal(t, uint64(0x100), node.Identifier())
}

// TestBGPLSLinkNLRI verifies link NLRI creation.
func TestBGPLSLinkNLRI(t *testing.T) {
	localNode := NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	}
	remoteNode := NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{2, 2, 2, 2},
	}
	linkDesc := LinkDescriptor{
		LocalInterfaceAddr: []byte{10, 0, 0, 1},
	}

	link := NewBGPLSLink(ProtoOSPFv2, 0x100, localNode, remoteNode, linkDesc)

	assert.Equal(t, BGPLSLinkNLRI, link.NLRIType())
	assert.Equal(t, ProtoOSPFv2, link.ProtocolID())
}

// TestBGPLSPrefixV4NLRI verifies IPv4 prefix NLRI creation.
func TestBGPLSPrefixV4NLRI(t *testing.T) {
	node := NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	}
	prefixDesc := PrefixDescriptor{
		IPReachabilityInfo: []byte{24, 10, 0, 0}, // 10.0.0.0/24
	}

	prefix := NewBGPLSPrefixV4(ProtoOSPFv2, 0x100, node, prefixDesc)

	assert.Equal(t, BGPLSPrefixV4NLRI, prefix.NLRIType())
}

// TestBGPLSPrefixV6NLRI verifies IPv6 prefix NLRI creation.
func TestBGPLSPrefixV6NLRI(t *testing.T) {
	node := NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	}
	prefixDesc := PrefixDescriptor{
		IPReachabilityInfo: []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}, // 2001:db8::/64
	}

	prefix := NewBGPLSPrefixV6(ProtoOSPFv2, 0x100, node, prefixDesc)

	assert.Equal(t, BGPLSPrefixV6NLRI, prefix.NLRIType())
}

// TestBGPLSFamily verifies BGP-LS address family.
func TestBGPLSFamily(t *testing.T) {
	node := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN: 65001,
	})

	// BGP-LS uses AFI 16388, SAFI 71
	assert.Equal(t, AFIBGPLS, node.Family().AFI)
}

// TestBGPLSNodeBytes verifies wire format encoding.
func TestBGPLSNodeBytes(t *testing.T) {
	node := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	})

	data := node.Bytes()
	require.NotEmpty(t, data)

	// Verify structure:
	// - NLRI type (2 bytes)
	// - Total length (2 bytes)
	// - Protocol ID (1 byte)
	// - Identifier (8 bytes)
	// - Local Node Descriptors (TLVs)
	assert.GreaterOrEqual(t, len(data), 13)
}

// TestBGPLSNodeString verifies string representation.
func TestBGPLSNodeString(t *testing.T) {
	node := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	})

	s := node.String()
	assert.Contains(t, s, "node")
	assert.Contains(t, s, "65001")
}

// TestBGPLSLinkBytes verifies link wire format.
func TestBGPLSLinkBytes(t *testing.T) {
	link := NewBGPLSLink(
		ProtoOSPFv2, 0x100,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{2, 2, 2, 2}},
		LinkDescriptor{},
	)

	data := link.Bytes()
	require.NotEmpty(t, data)
}

// TestBGPLSLinkDescriptorNotWrapped verifies link descriptors appear directly in NLRI.
//
// VALIDATES: RFC 7752 Section 3.2.2 - Link descriptor TLVs (258-263) appear
// directly in the Link NLRI body after Remote Node Descriptors, NOT wrapped
// in a container TLV.
//
// PREVENTS: Encoding violation where link descriptors are wrapped in TLV 258.
func TestBGPLSLinkDescriptorNotWrapped(t *testing.T) {
	link := NewBGPLSLink(
		ProtoOSPFv2, 0x100,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
		NodeDescriptor{ASN: 65002, IGPRouterID: []byte{2, 2, 2, 2}},
		LinkDescriptor{
			LocalInterfaceAddr: []byte{10, 0, 0, 1}, // Should produce TLV 259 directly
		},
	)

	data := link.Bytes()
	require.NotEmpty(t, data)

	// Scan TLVs after header (4 bytes) and protocol/identifier (9 bytes)
	// Format: type(2) + length(2) + proto(1) + id(8) + TLVs
	offset := 4 + 9 // Start of TLVs
	require.Greater(t, len(data), offset)

	// Scan for TLV types - we should see 256, 257, then 259 directly (not 258 as container)
	foundTLVTypes := []uint16{}
	for offset+4 <= len(data) {
		tlvType := uint16(data[offset])<<8 | uint16(data[offset+1])
		tlvLen := int(uint16(data[offset+2])<<8 | uint16(data[offset+3]))
		foundTLVTypes = append(foundTLVTypes, tlvType)
		offset += 4 + tlvLen
	}

	// RFC 7752: TLV 259 (IPv4 Interface Address) should appear directly
	// NOT wrapped inside a container TLV 258
	assert.Contains(t, foundTLVTypes, uint16(256), "should have Local Node Descriptor TLV")
	assert.Contains(t, foundTLVTypes, uint16(257), "should have Remote Node Descriptor TLV")
	assert.Contains(t, foundTLVTypes, uint16(259), "should have IPv4 Interface Address TLV directly")

	// Verify no duplicate TLV 258 used as container (258 may appear for Link Local/Remote ID, but not as wrapper)
	count258 := 0
	for _, tlvType := range foundTLVTypes {
		if tlvType == 258 {
			count258++
		}
	}
	// If there's a TLV 258, it should only be for actual Link Local/Remote ID, not as container
	// Since our test doesn't set LinkLocalID/LinkRemoteID, there should be no TLV 258
	assert.Equal(t, 0, count258, "TLV 258 should not appear as container wrapper")
}

// TestBGPLSPrefixDescriptorNotWrapped verifies prefix descriptors appear directly in NLRI.
//
// VALIDATES: RFC 7752 Section 3.2.3 - Prefix descriptor TLVs (263-265) appear
// directly in the Prefix NLRI body, NOT wrapped in a container TLV.
//
// PREVENTS: Encoding violation where prefix descriptors are wrapped in TLV 264.
func TestBGPLSPrefixDescriptorNotWrapped(t *testing.T) {
	prefix := NewBGPLSPrefixV4(
		ProtoOSPFv2, 0x100,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
		PrefixDescriptor{
			IPReachabilityInfo: []byte{24, 10, 0, 0}, // Should produce TLV 265 directly
		},
	)

	data := prefix.Bytes()
	require.NotEmpty(t, data)

	// Scan TLVs after header and protocol/identifier
	offset := 4 + 9
	require.Greater(t, len(data), offset)

	foundTLVTypes := []uint16{}
	for offset+4 <= len(data) {
		tlvType := uint16(data[offset])<<8 | uint16(data[offset+1])
		tlvLen := int(uint16(data[offset+2])<<8 | uint16(data[offset+3]))
		foundTLVTypes = append(foundTLVTypes, tlvType)
		offset += 4 + tlvLen
	}

	// RFC 7752: TLV 265 (IP Reachability Info) should appear directly
	// NOT wrapped inside a container TLV 264
	assert.Contains(t, foundTLVTypes, uint16(256), "should have Local Node Descriptor TLV")
	assert.Contains(t, foundTLVTypes, uint16(265), "should have IP Reachability Info TLV directly")

	// Verify TLV 264 is not used as container wrapper
	for _, tlvType := range foundTLVTypes {
		assert.NotEqual(t, uint16(264), tlvType, "TLV 264 should not appear as container wrapper")
	}
}

// TestParseBGPLSNode verifies parsing node NLRI.
func TestParseBGPLSNode(t *testing.T) {
	original := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	})

	data := original.Bytes()

	parsed, err := ParseBGPLS(data)
	require.NoError(t, err)
	require.NotNil(t, parsed)

	assert.Equal(t, BGPLSNodeNLRI, parsed.NLRIType())
}

// TestParseBGPLSErrors verifies error handling.
func TestParseBGPLSErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated type", []byte{0x00}},
		{"truncated length", []byte{0x00, 0x01, 0x00}},
		{"invalid type", []byte{0x00, 0xFF, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBGPLS(tt.data)
			assert.Error(t, err)
		})
	}
}

// TestBGPLSRoundTrip verifies encode/decode cycle.
func TestBGPLSRoundTrip(t *testing.T) {
	testCases := []struct {
		name string
		nlri BGPLSNLRI
	}{
		{
			name: "node",
			nlri: NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
				ASN:         65001,
				IGPRouterID: []byte{1, 1, 1, 1},
			}),
		},
		{
			name: "link",
			nlri: NewBGPLSLink(ProtoOSPFv2, 0x100,
				NodeDescriptor{ASN: 65001},
				NodeDescriptor{ASN: 65002},
				LinkDescriptor{},
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.nlri.Bytes()
			parsed, err := ParseBGPLS(data)
			require.NoError(t, err)

			assert.Equal(t, tc.nlri.NLRIType(), parsed.NLRIType())
		})
	}
}

// TestBGPLSSRv6SID verifies SRv6 SID NLRI creation.
func TestBGPLSSRv6SID(t *testing.T) {
	node := NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	}
	sid := SRv6SIDDescriptor{
		SRv6SID: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
	}

	srv6 := NewBGPLSSRv6SID(ProtoSegment, 0x200, node, sid)

	assert.Equal(t, BGPLSSRv6SIDNLRI, srv6.NLRIType())
	assert.Equal(t, ProtoSegment, srv6.ProtocolID())
	assert.Equal(t, uint64(0x200), srv6.Identifier())
}

// TestBGPLSSRv6SIDBytes verifies SRv6 SID wire format.
func TestBGPLSSRv6SIDBytes(t *testing.T) {
	node := NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	}
	sid := SRv6SIDDescriptor{
		SRv6SID: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
	}

	srv6 := NewBGPLSSRv6SID(ProtoSegment, 0x200, node, sid)
	data := srv6.Bytes()

	require.NotEmpty(t, data)
	assert.GreaterOrEqual(t, len(data), 13) // Minimum header size
}

// TestBGPLSSRv6SIDRoundTrip verifies SRv6 SID encode/decode cycle.
func TestBGPLSSRv6SIDRoundTrip(t *testing.T) {
	node := NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	}
	sid := SRv6SIDDescriptor{
		SRv6SID: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
	}

	original := NewBGPLSSRv6SID(ProtoSegment, 0x200, node, sid)
	data := original.Bytes()

	parsed, err := ParseBGPLS(data)
	require.NoError(t, err)
	require.NotNil(t, parsed)

	assert.Equal(t, BGPLSSRv6SIDNLRI, parsed.NLRIType())
	assert.Equal(t, original.ProtocolID(), parsed.ProtocolID())
	assert.Equal(t, original.Identifier(), parsed.Identifier())
}
