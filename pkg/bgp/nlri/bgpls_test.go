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
