package bgp_ls

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBGPLSPluginDecodeMode verifies the plugin's decode mode protocol.
//
// VALIDATES: Plugin responds correctly to "decode nlri bgp-ls/bgp-ls <hex>" requests.
// PREVENTS: Protocol mismatch with engine decode dispatcher.
func TestBGPLSPluginDecodeMode(t *testing.T) {
	// Create a node NLRI for testing
	node := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	})
	hexData := strings.ToUpper(hex.EncodeToString(node.Bytes()))

	input := bytes.NewBufferString("decode nlri bgp-ls/bgp-ls " + hexData + "\n")
	output := &bytes.Buffer{}

	RunBGPLSDecode(input, output)

	result := output.String()
	assert.True(t, strings.HasPrefix(result, "decoded json "), "should return decoded json prefix")

	// Parse JSON to verify structure
	jsonStr := strings.TrimPrefix(strings.TrimSpace(result), "decoded json ")
	var parsed map[string]any
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "bgpls-node", parsed["ls-nlri-type"])
}

// TestBGPLSPluginInvalidFamily verifies unknown family handling.
//
// VALIDATES: Plugin returns "decoded unknown" for non-bgp-ls families.
// PREVENTS: Plugin crashes on unexpected family strings.
func TestBGPLSPluginInvalidFamily(t *testing.T) {
	input := bytes.NewBufferString("decode nlri ipv4/unicast 00000000\n")
	output := &bytes.Buffer{}

	RunBGPLSDecode(input, output)

	result := strings.TrimSpace(output.String())
	assert.Equal(t, "decoded unknown", result)
}

// TestBGPLSPluginInvalidHex verifies malformed hex handling.
//
// VALIDATES: Plugin returns "decoded unknown" for invalid hex input.
// PREVENTS: Plugin crashes on malformed input.
func TestBGPLSPluginInvalidHex(t *testing.T) {
	input := bytes.NewBufferString("decode nlri bgp-ls/bgp-ls GGGG\n")
	output := &bytes.Buffer{}

	RunBGPLSDecode(input, output)

	result := strings.TrimSpace(output.String())
	assert.Equal(t, "decoded unknown", result)
}

// TestBGPLSCLIDecode verifies CLI decode mode.
//
// VALIDATES: CLI mode produces valid JSON output.
// PREVENTS: CLI mode output incompatible with downstream tools.
func TestBGPLSCLIDecode(t *testing.T) {
	node := NewBGPLSNode(ProtoISISL2, 0x200, NodeDescriptor{
		ASN:         65500,
		IGPRouterID: []byte{10, 0, 0, 1},
	})
	hexData := strings.ToUpper(hex.EncodeToString(node.Bytes()))

	output := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	exitCode := RunBGPLSCLIDecode(hexData, "bgp-ls/bgp-ls", false, output, errOut)

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, errOut.String())

	// Verify JSON output
	var result map[string]any
	err := json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, "bgpls-node", result["ls-nlri-type"])
}

// TestBGPLSCLIDecodeText verifies CLI text output mode.
//
// VALIDATES: CLI mode with --text produces human-readable output.
// PREVENTS: Text mode crashes or produces garbled output.
func TestBGPLSCLIDecodeText(t *testing.T) {
	node := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	})
	hexData := strings.ToUpper(hex.EncodeToString(node.Bytes()))

	output := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	exitCode := RunBGPLSCLIDecode(hexData, "bgp-ls/bgp-ls", true, output, errOut)

	assert.Equal(t, 0, exitCode)
	assert.Empty(t, errOut.String())
	assert.Contains(t, output.String(), "bgpls-node")
}

// TestBGPLSCLIDecodeInvalidFamily verifies CLI error handling.
//
// VALIDATES: CLI returns error for invalid family.
// PREVENTS: Silent failures on bad input.
func TestBGPLSCLIDecodeInvalidFamily(t *testing.T) {
	output := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	exitCode := RunBGPLSCLIDecode("00000000", "ipv4/unicast", false, output, errOut)

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, errOut.String(), "invalid family")
}

// TestBGPLSNodeNLRIDecode verifies Node NLRI decoding.
//
// VALIDATES: Node NLRI (type 1) decodes with correct fields.
// PREVENTS: Node-specific fields missing from output.
func TestBGPLSNodeNLRIDecode(t *testing.T) {
	node := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:             65001,
		BGPLSIdentifier: 0x12345678,
		IGPRouterID:     []byte{1, 1, 1, 1},
	})

	results := decodeBGPLSNLRI(node.Bytes())
	require.Len(t, results, 1)
	result := results[0]

	assert.Equal(t, "bgpls-node", result["ls-nlri-type"])
	assert.Equal(t, 3, result["protocol-id"]) // ProtoOSPFv2 = 3
	assert.Equal(t, uint64(0x100), result["l3-routing-topology"])

	// Check node descriptors
	nodeDescs, ok := result["node-descriptors"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, nodeDescs)
}

// TestBGPLSLinkNLRIDecode verifies Link NLRI decoding.
//
// VALIDATES: Link NLRI (type 2) decodes with local/remote node descriptors.
// PREVENTS: Link-specific fields missing from output.
func TestBGPLSLinkNLRIDecode(t *testing.T) {
	link := NewBGPLSLink(
		ProtoISISL2, 0x200,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
		NodeDescriptor{ASN: 65002, IGPRouterID: []byte{2, 2, 2, 2}},
		LinkDescriptor{LinkLocalID: 100, LinkRemoteID: 200},
	)

	results := decodeBGPLSNLRI(link.Bytes())
	require.Len(t, results, 1)
	result := results[0]

	assert.Equal(t, "bgpls-link", result["ls-nlri-type"])
	assert.Equal(t, 2, result["protocol-id"]) // ProtoISISL2 = 2

	// Check local and remote node descriptors
	_, hasLocal := result["local-node-descriptors"]
	_, hasRemote := result["remote-node-descriptors"]
	assert.True(t, hasLocal)
	assert.True(t, hasRemote)
}

// TestBGPLSPrefixV4NLRIDecode verifies IPv4 Prefix NLRI decoding.
//
// VALIDATES: IPv4 Prefix NLRI (type 3) decodes correctly.
// PREVENTS: Prefix-specific fields missing from output.
func TestBGPLSPrefixV4NLRIDecode(t *testing.T) {
	prefix := NewBGPLSPrefixV4(
		ProtoOSPFv2, 0x100,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
		PrefixDescriptor{IPReachabilityInfo: []byte{24, 10, 0, 0}},
	)

	results := decodeBGPLSNLRI(prefix.Bytes())
	require.Len(t, results, 1)

	assert.Equal(t, "bgpls-prefix-v4", results[0]["ls-nlri-type"])
}

// TestBGPLSPrefixV6NLRIDecode verifies IPv6 Prefix NLRI decoding.
//
// VALIDATES: IPv6 Prefix NLRI (type 4) decodes correctly.
// PREVENTS: IPv6-specific handling issues.
func TestBGPLSPrefixV6NLRIDecode(t *testing.T) {
	prefix := NewBGPLSPrefixV6(
		ProtoOSPFv3, 0x200,
		NodeDescriptor{ASN: 65002},
		PrefixDescriptor{IPReachabilityInfo: []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}},
	)

	results := decodeBGPLSNLRI(prefix.Bytes())
	require.Len(t, results, 1)

	assert.Equal(t, "bgpls-prefix-v6", results[0]["ls-nlri-type"])
}

// TestBGPLSSRv6SIDNLRIDecode verifies SRv6 SID NLRI decoding.
//
// VALIDATES: SRv6 SID NLRI (type 6, RFC 9514) decodes correctly.
// PREVENTS: Wrong NLRI type detection.
func TestBGPLSSRv6SIDNLRIDecode(t *testing.T) {
	srv6 := NewBGPLSSRv6SID(
		ProtoSegment, 0x300,
		NodeDescriptor{ASN: 65001, IGPRouterID: []byte{1, 1, 1, 1}},
		SRv6SIDDescriptor{SRv6SID: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
	)

	results := decodeBGPLSNLRI(srv6.Bytes())
	require.Len(t, results, 1)
	result := results[0]

	assert.Equal(t, "bgpls-srv6-sid", result["ls-nlri-type"])
	assert.Equal(t, 9, result["protocol-id"]) // ProtoSegment = 9

	// Note: SRv6SID descriptor parsing from wire is not fully implemented in nlri package.
	// The type detection and basic parsing works, but SRv6SID.SRv6SID may not be populated.
}

// TestBGPLSProtocolIDs verifies all protocol IDs are handled.
//
// VALIDATES: All RFC 7752 protocol IDs produce valid output.
// PREVENTS: Unknown protocol ID crashes.
func TestBGPLSProtocolIDs(t *testing.T) {
	protocols := []BGPLSProtocolID{
		ProtoISISL1,
		ProtoISISL2,
		ProtoOSPFv2,
		ProtoDirect,
		ProtoStatic,
		ProtoOSPFv3,
		ProtoBGP,
	}

	for _, proto := range protocols {
		t.Run(proto.String(), func(t *testing.T) {
			node := NewBGPLSNode(proto, 0x100, NodeDescriptor{ASN: 65001})
			results := decodeBGPLSNLRI(node.Bytes())
			require.Len(t, results, 1)
		})
	}
}

// TestBGPLSMalformedInput verifies error handling for truncated data.
//
// VALIDATES: Truncated data returns nil.
// PREVENTS: Panic on malformed wire bytes.
func TestBGPLSMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"truncated_type", []byte{0x00}},
		{"truncated_length", []byte{0x00, 0x01, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := decodeBGPLSNLRI(tt.data)
			// Malformed input returns a result with parsed=false
			require.Len(t, results, 1)
			assert.Equal(t, false, results[0]["parsed"])
		})
	}
}

// TestIsValidBGPLSFamily verifies family validation.
//
// VALIDATES: "bgp-ls/bgp-ls" and "bgp-ls/bgp-ls-vpn" are accepted.
// PREVENTS: Incorrect family strings being processed.
func TestIsValidBGPLSFamily(t *testing.T) {
	tests := []struct {
		family string
		valid  bool
	}{
		{"bgp-ls/bgp-ls", true},
		{"bgp-ls/bgp-ls-vpn", true},
		{"BGP-LS/BGP-LS", false}, // case-sensitive
		{"bgpls/bgpls", false},
		{"ipv4/unicast", false},
		{"l2vpn/evpn", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.family, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidBGPLSFamily(tt.family))
		})
	}
}

// TestFormatRouterID verifies router ID formatting for different lengths.
//
// VALIDATES: Router ID formatted correctly for OSPF (4B), IS-IS (6/7B), pseudonode (8B).
// PREVENTS: Incorrect router ID display in JSON output.
func TestFormatRouterID(t *testing.T) {
	tests := []struct {
		name     string
		id       []byte
		expected string
	}{
		{"ospf_4byte", []byte{1, 2, 3, 4}, "1.2.3.4"},
		{"isis_6byte", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, "010203040506"},
		{"isis_7byte_psn", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, "01020304050607"},
		{"ospf_8byte_psn", []byte{1, 2, 3, 4, 5, 6, 7, 8}, "1.2.3.4,5.6.7.8"},
		{"unknown_3byte", []byte{0xAB, 0xCD, 0xEF}, "ABCDEF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatRouterID(tt.id))
		})
	}
}

// TestFormatIPv6Compressed verifies IPv6 address formatting.
//
// VALIDATES: IPv6 addresses formatted with zero compression via netip.
// PREVENTS: Garbled IPv6 output in JSON.
func TestFormatIPv6Compressed(t *testing.T) {
	// 2001:db8::1
	addr := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	result := formatIPv6Compressed(addr)
	assert.Equal(t, "2001:db8::1", result)
}

// TestBGPLSNLRITypeString verifies NLRI type name formatting.
//
// VALIDATES: Known types return named strings, unknown return "bgpls-type-N".
// PREVENTS: Panic on unknown NLRI types.
func TestBGPLSNLRITypeString(t *testing.T) {
	tests := []struct {
		nlriType uint16
		expected string
	}{
		{1, "bgpls-node"},
		{2, "bgpls-link"},
		{3, "bgpls-prefix-v4"},
		{4, "bgpls-prefix-v6"},
		{6, "bgpls-srv6-sid"},
		{5, "bgpls-type-5"},   // Unknown type
		{99, "bgpls-type-99"}, // Unknown type
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, bgplsNLRITypeString(tt.nlriType))
		})
	}
}

// TestBGPLSMultipleNLRIDecode verifies decoding multiple packed NLRIs.
//
// VALIDATES: Multiple NLRIs in single buffer are all decoded.
// PREVENTS: Only first NLRI being decoded when multiple are packed.
func TestBGPLSMultipleNLRIDecode(t *testing.T) {
	// Create two Node NLRIs with different ASNs
	node1 := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	})
	node2 := NewBGPLSNode(ProtoISISL2, 0x200, NodeDescriptor{
		ASN:         65002,
		IGPRouterID: []byte{2, 2, 2, 2},
	})

	// Concatenate both NLRIs
	combined := append(node1.Bytes(), node2.Bytes()...)

	results := decodeBGPLSNLRI(combined)

	// Should decode both NLRIs
	require.Len(t, results, 2, "should decode both packed NLRIs")

	// First NLRI
	assert.Equal(t, "bgpls-node", results[0]["ls-nlri-type"])
	assert.Equal(t, 3, results[0]["protocol-id"]) // ProtoOSPFv2

	// Second NLRI
	assert.Equal(t, "bgpls-node", results[1]["ls-nlri-type"])
	assert.Equal(t, 2, results[1]["protocol-id"]) // ProtoISISL2
}

// TestBGPLSVPNFamilyDecode verifies bgp-ls-vpn family is accepted.
//
// VALIDATES: Plugin handles bgp-ls/bgp-ls-vpn family correctly.
// PREVENTS: VPN family rejected despite being valid.
func TestBGPLSVPNFamilyDecode(t *testing.T) {
	node := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	})
	hexData := strings.ToUpper(hex.EncodeToString(node.Bytes()))

	// Test with bgp-ls-vpn family
	input := bytes.NewBufferString("decode nlri bgp-ls/bgp-ls-vpn " + hexData + "\n")
	output := &bytes.Buffer{}

	RunBGPLSDecode(input, output)

	result := output.String()
	assert.True(t, strings.HasPrefix(result, "decoded json "), "bgp-ls-vpn should return decoded json")

	// Verify JSON parses correctly
	jsonStr := strings.TrimPrefix(strings.TrimSpace(result), "decoded json ")
	var parsed map[string]any
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "bgpls-node", parsed["ls-nlri-type"])
}

// TestBGPLSCLIDecodeVPNFamily verifies CLI accepts bgp-ls-vpn family.
//
// VALIDATES: CLI mode works with bgp-ls/bgp-ls-vpn family.
// PREVENTS: CLI rejecting valid VPN family.
func TestBGPLSCLIDecodeVPNFamily(t *testing.T) {
	node := NewBGPLSNode(ProtoOSPFv2, 0x100, NodeDescriptor{
		ASN:         65001,
		IGPRouterID: []byte{1, 1, 1, 1},
	})
	hexData := strings.ToUpper(hex.EncodeToString(node.Bytes()))

	output := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	exitCode := RunBGPLSCLIDecode(hexData, "bgp-ls/bgp-ls-vpn", false, output, errOut)

	assert.Equal(t, 0, exitCode, "bgp-ls-vpn should succeed")
	assert.Empty(t, errOut.String(), "no errors expected")

	var result map[string]any
	err := json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "bgpls-node", result["ls-nlri-type"])
}
