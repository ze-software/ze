package message

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ChunkMPNLRI Comprehensive Tests
// =============================================================================
//
// NLRI Formats by Family:
//
// IPv4/IPv6 Unicast (no Add-Path):
//   [prefix-len:1][prefix-bytes:var]
//
// IPv4/IPv6 Unicast with Add-Path:
//   [path-id:4][prefix-len:1][prefix-bytes:var]
//
// Labeled Unicast (SAFI 4):
//   [total-bits:1][labels:3*N][prefix-bytes:var]
//   where total-bits = 24*N + prefix_bits
//
// VPN (SAFI 128):
//   [total-bits:1][labels:3*N][RD:8][prefix-bytes:var]
//   where total-bits = 24*N + 64 + prefix_bits
//
// EVPN (AFI 25, SAFI 70):
//   [route-type:1][length:1][payload:length]
//
// FlowSpec (SAFI 133/134):
//   [length:1-2][components:...]
//   length >= 240 uses 2-byte encoding
//
// BGP-LS (AFI 16388, SAFI 71):
//   [nlri-type:2][total-length:2][payload:total-length]

// =============================================================================
// IPv6 Unicast Tests (no Add-Path)
// =============================================================================

// TestChunkMPNLRI_IPv6_SmallFits verifies small IPv6 NLRI passthrough.
//
// VALIDATES: Single /64 that fits returns unchanged.
// PREVENTS: Unnecessary chunking overhead.
func TestChunkMPNLRI_IPv6_SmallFits(t *testing.T) {
	// 2001:db8::/64 = [64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00]
	nlri := []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00}

	chunks, err := ChunkMPNLRI(nlri, 2, 1, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_IPv6_MultiplePrefixes verifies multiple IPv6 prefixes.
//
// VALIDATES: Multiple /64s chunked correctly at prefix boundaries.
// PREVENTS: Mid-prefix split corruption.
func TestChunkMPNLRI_IPv6_MultiplePrefixes(t *testing.T) {
	// 10 /64 prefixes, each 9 bytes
	var nlri []byte
	for i := 0; i < 10; i++ {
		nlri = append(nlri, 64) // /64
		nlri = append(nlri, 0x20, 0x01, 0x0d, 0xb8, byte(i), 0x00, 0x00, 0x00)
	}
	// Total: 90 bytes

	// maxSize = 30, so ~3 prefixes per chunk (27 bytes)
	chunks, err := ChunkMPNLRI(nlri, 2, 1, false, 30)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	// Verify no chunk exceeds maxSize
	for i, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), 30, "chunk %d exceeds maxSize", i)
	}

	// Verify all bytes preserved
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}

// TestChunkMPNLRI_IPv6_VariableLengths verifies mixed prefix lengths.
//
// VALIDATES: /48, /64, /128 all parsed correctly.
// PREVENTS: Size miscalculation for non-/64 prefixes.
func TestChunkMPNLRI_IPv6_VariableLengths(t *testing.T) {
	var nlri []byte
	// /48 = 1 + 6 = 7 bytes
	nlri = append(nlri, 48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01)
	// /64 = 1 + 8 = 9 bytes
	nlri = append(nlri, 64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02, 0x00, 0x00)
	// /128 = 1 + 16 = 17 bytes
	nlri = append(nlri, 128, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x03, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01)
	// Total: 33 bytes

	// maxSize = 20, split between /64 and /128
	chunks, err := ChunkMPNLRI(nlri, 2, 1, false, 20)
	require.NoError(t, err)
	require.Len(t, chunks, 2)

	// First chunk: /48 (7) + /64 (9) = 16 bytes
	assert.Equal(t, 16, len(chunks[0]))
	// Second chunk: /128 (17) bytes
	assert.Equal(t, 17, len(chunks[1]))
}

// TestChunkMPNLRI_IPv6_EdgePrefixLengths verifies edge case prefix lengths.
//
// VALIDATES: /0, /1, /127, /128 all handled correctly.
// PREVENTS: Off-by-one in bit-to-byte calculation.
func TestChunkMPNLRI_IPv6_EdgePrefixLengths(t *testing.T) {
	testCases := []struct {
		name      string
		prefixLen byte
		dataBytes int
	}{
		{"zero", 0, 0},   // /0 = 1 byte total
		{"one", 1, 1},    // /1 = 2 bytes total
		{"127", 127, 16}, // /127 = 17 bytes total
		{"128", 128, 16}, // /128 = 17 bytes total
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nlri := make([]byte, 1+tc.dataBytes)
			nlri[0] = tc.prefixLen
			// Fill with non-zero to detect corruption
			for i := 1; i < len(nlri); i++ {
				nlri[i] = byte(i)
			}

			chunks, err := ChunkMPNLRI(nlri, 2, 1, false, 100)
			require.NoError(t, err)
			require.Len(t, chunks, 1)
			assert.Equal(t, nlri, chunks[0])
		})
	}
}

// =============================================================================
// Add-Path Tests (critical failure mode for ChunkNLRI)
// =============================================================================

// TestChunkMPNLRI_AddPath_SinglePrefix verifies Add-Path prefix parsing.
//
// VALIDATES: [path-id:4][prefix-len:1][prefix] parsed as single unit.
// PREVENTS: ChunkNLRI bug: path-id[0] interpreted as prefix-len.
func TestChunkMPNLRI_AddPath_SinglePrefix(t *testing.T) {
	// Path-ID=1, 192.168.1.0/24
	// [0x00, 0x00, 0x00, 0x01, 24, 192, 168, 1]
	nlri := []byte{0x00, 0x00, 0x00, 0x01, 24, 192, 168, 1}

	chunks, err := ChunkMPNLRI(nlri, 1, 1, true, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_AddPath_MultipleNoSplit verifies multiple Add-Path prefixes.
//
// VALIDATES: Multiple [path-id:4][prefix] kept together when they fit.
// PREVENTS: Path-ID separated from prefix.
func TestChunkMPNLRI_AddPath_MultipleNoSplit(t *testing.T) {
	var nlri []byte
	for i := 1; i <= 5; i++ {
		// Path-ID = i, 10.0.i.0/24
		nlri = append(nlri, 0, 0, 0, byte(i)) // path-id
		nlri = append(nlri, 24, 10, 0, byte(i))
	}
	// Each: 8 bytes, total: 40 bytes

	chunks, err := ChunkMPNLRI(nlri, 1, 1, true, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_AddPath_Split verifies Add-Path splitting at correct boundaries.
//
// VALIDATES: Split between complete [path-id][prefix] units.
// PREVENTS: Split between path-id and prefix-len.
func TestChunkMPNLRI_AddPath_Split(t *testing.T) {
	var nlri []byte
	for i := 1; i <= 10; i++ {
		nlri = append(nlri, 0, 0, 0, byte(i))   // path-id
		nlri = append(nlri, 24, 10, 0, byte(i)) // /24
	}
	// Each: 8 bytes, total: 80 bytes

	// maxSize = 20, fits 2 prefixes (16 bytes)
	chunks, err := ChunkMPNLRI(nlri, 1, 1, true, 20)
	require.NoError(t, err)

	// Should split into 5 chunks of 2 prefixes each
	require.Len(t, chunks, 5)

	for i, chunk := range chunks {
		// Each chunk should be exactly 16 bytes (2 prefixes)
		assert.Equal(t, 16, len(chunk), "chunk %d wrong size", i)

		// Verify each chunk starts with valid path-id (non-zero)
		pathID := binary.BigEndian.Uint32(chunk[0:4])
		assert.NotEqual(t, uint32(0), pathID, "chunk %d starts with zero path-id", i)
	}
}

// TestChunkMPNLRI_AddPath_IPv6 verifies IPv6 Add-Path.
//
// VALIDATES: IPv6 /64 with path-id parsed correctly.
// PREVENTS: AFI-specific parsing errors with Add-Path.
func TestChunkMPNLRI_AddPath_IPv6(t *testing.T) {
	// Path-ID=100, 2001:db8::/64
	var nlri []byte
	nlri = append(nlri, 0, 0, 0, 100) // path-id
	nlri = append(nlri, 64)           // prefix-len
	nlri = append(nlri, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00)
	// Total: 13 bytes

	chunks, err := ChunkMPNLRI(nlri, 2, 1, true, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_AddPath_LargePathID verifies large path-id values.
//
// VALIDATES: Path-ID 0xFFFFFFFF doesn't corrupt parsing.
// PREVENTS: Path-ID bytes interpreted as length.
func TestChunkMPNLRI_AddPath_LargePathID(t *testing.T) {
	// Path-ID = 0xFFFFFFFF (max), 10.0.0.0/8
	nlri := []byte{0xFF, 0xFF, 0xFF, 0xFF, 8, 10}

	chunks, err := ChunkMPNLRI(nlri, 1, 1, true, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)

	// Verify path-id preserved
	pathID := binary.BigEndian.Uint32(chunks[0][0:4])
	assert.Equal(t, uint32(0xFFFFFFFF), pathID)
}

// =============================================================================
// VPN Tests (SAFI 128)
// =============================================================================

// TestChunkMPNLRI_VPN_SinglePrefix verifies VPN NLRI parsing.
//
// VALIDATES: [total-bits][label][RD][prefix] parsed as single unit.
// PREVENTS: Label or RD split from prefix.
func TestChunkMPNLRI_VPN_SinglePrefix(t *testing.T) {
	// VPNv4: label=100, RD=65000:1, 10.0.0.0/24
	// total-bits = 24 (label) + 64 (RD) + 24 (prefix) = 112
	var nlri []byte
	nlri = append(nlri, 112)                          // total-bits
	nlri = append(nlri, 0x00, 0x06, 0x41)             // label 100 with BoS
	nlri = append(nlri, 0, 0, 0xFD, 0xE8, 0, 0, 0, 1) // RD type 0, ASN 65000, assigned 1
	nlri = append(nlri, 10, 0, 0)                     // 10.0.0.0/24
	// Total: 15 bytes

	chunks, err := ChunkMPNLRI(nlri, 1, 128, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_VPN_MultipleWithSplit verifies VPN chunking.
//
// VALIDATES: Multiple VPN prefixes split at NLRI boundaries.
// PREVENTS: Split in middle of label/RD/prefix.
func TestChunkMPNLRI_VPN_MultipleWithSplit(t *testing.T) {
	var nlri []byte
	for i := 0; i < 10; i++ {
		// VPNv4: label=i, RD=65000:i, 10.0.i.0/24
		nlri = append(nlri, 112)                                // total-bits
		nlri = append(nlri, 0x00, byte(i>>4), byte(i<<4)|0x01)  // label with BoS
		nlri = append(nlri, 0, 0, 0xFD, 0xE8, 0, 0, 0, byte(i)) // RD
		nlri = append(nlri, 10, 0, byte(i))                     // prefix
	}
	// Each: 15 bytes, total: 150 bytes

	// maxSize = 40, fits 2 prefixes (30 bytes)
	chunks, err := ChunkMPNLRI(nlri, 1, 128, false, 40)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	// Verify all bytes preserved
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}

// TestChunkMPNLRI_VPN_LabelStack verifies multi-label VPN.
//
// VALIDATES: Label stack (multiple labels) kept together.
// PREVENTS: Label stack split.
func TestChunkMPNLRI_VPN_LabelStack(t *testing.T) {
	// VPNv4 with 2 labels: total-bits = 48 (2 labels) + 64 (RD) + 24 = 136
	var nlri []byte
	nlri = append(nlri, 136)                          // total-bits
	nlri = append(nlri, 0x00, 0x06, 0x40)             // label 100, no BoS
	nlri = append(nlri, 0x00, 0x0C, 0x81)             // label 200, BoS
	nlri = append(nlri, 0, 0, 0xFD, 0xE8, 0, 0, 0, 1) // RD
	nlri = append(nlri, 10, 0, 0)                     // prefix
	// Total: 18 bytes

	chunks, err := ChunkMPNLRI(nlri, 1, 128, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_VPN_IPv6 verifies VPNv6.
//
// VALIDATES: VPNv6 /64 parsed correctly.
// PREVENTS: AFI=2 + SAFI=128 combination errors.
func TestChunkMPNLRI_VPN_IPv6(t *testing.T) {
	// VPNv6: label=100, RD=65000:1, 2001:db8::/64
	// total-bits = 24 + 64 + 64 = 152
	var nlri []byte
	nlri = append(nlri, 152)                                // total-bits
	nlri = append(nlri, 0x00, 0x06, 0x41)                   // label 100 with BoS
	nlri = append(nlri, 0, 0, 0xFD, 0xE8, 0, 0, 0, 1)       // RD
	nlri = append(nlri, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0) // 2001:db8::/64
	// Total: 20 bytes

	chunks, err := ChunkMPNLRI(nlri, 2, 128, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// =============================================================================
// Labeled Unicast Tests (SAFI 4)
// =============================================================================

// TestChunkMPNLRI_Labeled_Single verifies labeled unicast parsing.
//
// VALIDATES: [total-bits][label][prefix] parsed correctly.
// PREVENTS: Label split from prefix.
func TestChunkMPNLRI_Labeled_Single(t *testing.T) {
	// Labeled IPv4: label=100, 10.0.0.0/24
	// total-bits = 24 (label) + 24 (prefix) = 48
	var nlri []byte
	nlri = append(nlri, 48)               // total-bits
	nlri = append(nlri, 0x00, 0x06, 0x41) // label 100 with BoS
	nlri = append(nlri, 10, 0, 0)         // 10.0.0.0/24
	// Total: 7 bytes

	chunks, err := ChunkMPNLRI(nlri, 1, 4, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_Labeled_IPv6 verifies labeled IPv6 unicast.
//
// VALIDATES: Labeled IPv6 /64 parsed correctly.
// PREVENTS: AFI-specific labeled parsing errors.
func TestChunkMPNLRI_Labeled_IPv6(t *testing.T) {
	// Labeled IPv6: label=100, 2001:db8::/64
	// total-bits = 24 + 64 = 88
	var nlri []byte
	nlri = append(nlri, 88)                                 // total-bits
	nlri = append(nlri, 0x00, 0x06, 0x41)                   // label 100 with BoS
	nlri = append(nlri, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0) // 2001:db8::/64
	// Total: 12 bytes

	chunks, err := ChunkMPNLRI(nlri, 2, 4, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// =============================================================================
// EVPN Tests (AFI 25, SAFI 70)
// =============================================================================

// TestChunkMPNLRI_EVPN_Type2 verifies EVPN MAC/IP route.
//
// VALIDATES: [route-type:1][length:1][payload] parsed correctly.
// PREVENTS: ChunkNLRI bug: route-type (2) as prefix-len.
func TestChunkMPNLRI_EVPN_Type2(t *testing.T) {
	// EVPN Type 2 (MAC/IP Advertisement)
	// route-type=2, length=33 (typical MAC+IPv4)
	var nlri []byte
	nlri = append(nlri, 2)                   // route-type
	nlri = append(nlri, 33)                  // length
	nlri = append(nlri, make([]byte, 33)...) // payload (zeros for test)
	// Total: 35 bytes

	chunks, err := ChunkMPNLRI(nlri, 25, 70, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_EVPN_Type5 verifies EVPN IP Prefix route.
//
// VALIDATES: Type 5 route parsed correctly.
// PREVENTS: Different route types have different lengths.
func TestChunkMPNLRI_EVPN_Type5(t *testing.T) {
	// EVPN Type 5 (IP Prefix)
	// route-type=5, length=34
	var nlri []byte
	nlri = append(nlri, 5)                   // route-type
	nlri = append(nlri, 34)                  // length
	nlri = append(nlri, make([]byte, 34)...) // payload
	// Total: 36 bytes

	chunks, err := ChunkMPNLRI(nlri, 25, 70, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_EVPN_MultipleSplit verifies EVPN splitting.
//
// VALIDATES: Multiple EVPN routes split at route boundaries.
// PREVENTS: Split in middle of EVPN payload.
func TestChunkMPNLRI_EVPN_MultipleSplit(t *testing.T) {
	var nlri []byte
	for i := 0; i < 5; i++ {
		nlri = append(nlri, 2)  // route-type
		nlri = append(nlri, 20) // length
		payload := make([]byte, 20)
		payload[0] = byte(i) // Mark each for verification
		nlri = append(nlri, payload...)
	}
	// Each: 22 bytes, total: 110 bytes

	// maxSize = 50, fits 2 routes (44 bytes)
	chunks, err := ChunkMPNLRI(nlri, 25, 70, false, 50)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	// Verify all bytes preserved
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}

// =============================================================================
// FlowSpec Tests (SAFI 133/134)
// =============================================================================

// TestChunkMPNLRI_FlowSpec_ShortLength verifies FlowSpec short length.
//
// VALIDATES: Length < 240 uses 1-byte encoding.
// PREVENTS: Length misparse.
func TestChunkMPNLRI_FlowSpec_ShortLength(t *testing.T) {
	// FlowSpec with length=50 (1-byte encoding)
	var nlri []byte
	nlri = append(nlri, 50)                  // length (1 byte)
	nlri = append(nlri, make([]byte, 50)...) // flow components
	// Total: 51 bytes

	chunks, err := ChunkMPNLRI(nlri, 1, 133, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_FlowSpec_LongLength verifies FlowSpec long length.
//
// VALIDATES: Length >= 240 uses 2-byte encoding.
// PREVENTS: 2-byte length misparse.
func TestChunkMPNLRI_FlowSpec_LongLength(t *testing.T) {
	// FlowSpec with length=300 (2-byte encoding)
	// Encoding: 0xF0 | (300 >> 8) = 0xF1, 300 & 0xFF = 0x2C
	var nlri []byte
	nlri = append(nlri, 0xF1, 0x2C)           // length (2 bytes)
	nlri = append(nlri, make([]byte, 300)...) // flow components
	// Total: 302 bytes

	chunks, err := ChunkMPNLRI(nlri, 1, 133, false, 500)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_FlowSpec_Boundary verifies length=239 and 240.
//
// VALIDATES: Boundary between 1-byte and 2-byte encoding.
// PREVENTS: Off-by-one at encoding boundary.
func TestChunkMPNLRI_FlowSpec_Boundary(t *testing.T) {
	// Length=239 (last 1-byte)
	nlri239 := append([]byte{239}, make([]byte, 239)...)
	chunks, err := ChunkMPNLRI(nlri239, 1, 133, false, 500)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, 240, len(chunks[0])) // 1 + 239

	// Length=240 (first 2-byte: 0xF0, 0xF0)
	nlri240 := append([]byte{0xF0, 0xF0}, make([]byte, 240)...)
	chunks, err = ChunkMPNLRI(nlri240, 1, 133, false, 500)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, 242, len(chunks[0])) // 2 + 240
}

// =============================================================================
// BGP-LS Tests (AFI 16388, SAFI 71)
// =============================================================================

// TestChunkMPNLRI_BGPLS_Node verifies BGP-LS Node NLRI.
//
// VALIDATES: [type:2][length:2][payload] parsed correctly.
// PREVENTS: 2-byte type/length misparse.
func TestChunkMPNLRI_BGPLS_Node(t *testing.T) {
	// BGP-LS Node: type=1, length=20
	var nlri []byte
	nlri = append(nlri, 0, 1)                // type (2 bytes) = 1 (Node)
	nlri = append(nlri, 0, 20)               // length (2 bytes) = 20
	nlri = append(nlri, make([]byte, 20)...) // payload
	// Total: 24 bytes

	chunks, err := ChunkMPNLRI(nlri, 16388, 71, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_BGPLS_Link verifies BGP-LS Link NLRI.
//
// VALIDATES: Type=2 (Link) parsed correctly.
// PREVENTS: Type-specific parsing errors.
func TestChunkMPNLRI_BGPLS_Link(t *testing.T) {
	// BGP-LS Link: type=2, length=50
	var nlri []byte
	nlri = append(nlri, 0, 2)  // type = 2 (Link)
	nlri = append(nlri, 0, 50) // length = 50
	nlri = append(nlri, make([]byte, 50)...)
	// Total: 54 bytes

	chunks, err := ChunkMPNLRI(nlri, 16388, 71, false, 100)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_BGPLS_Split verifies BGP-LS splitting.
//
// VALIDATES: Multiple BGP-LS NLRIs split correctly.
// PREVENTS: Split in middle of BGP-LS TLV.
func TestChunkMPNLRI_BGPLS_Split(t *testing.T) {
	var nlri []byte
	for i := 0; i < 5; i++ {
		nlri = append(nlri, 0, 1)  // type = Node
		nlri = append(nlri, 0, 30) // length = 30
		payload := make([]byte, 30)
		payload[0] = byte(i)
		nlri = append(nlri, payload...)
	}
	// Each: 34 bytes, total: 170 bytes

	// maxSize = 70, fits 2 NLRIs (68 bytes)
	chunks, err := ChunkMPNLRI(nlri, 16388, 71, false, 70)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	// Verify all bytes preserved
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}

// =============================================================================
// Add-Path Tests for EVPN, FlowSpec, BGP-LS
// =============================================================================

// TestChunkMPNLRI_EVPN_AddPath verifies EVPN with Add-Path.
//
// VALIDATES: EVPN NLRIs with path-id prefix parsed correctly.
// PREVENTS: Path-id corruption in EVPN Add-Path scenarios.
func TestChunkMPNLRI_EVPN_AddPath(t *testing.T) {
	// EVPN Add-Path: [path-id:4][route-type:1][length:1][payload]
	// Type 2 MAC/IP route with path-id
	var nlri []byte
	for i := 0; i < 5; i++ {
		// Path-id (4 bytes)
		nlri = append(nlri, 0x00, 0x00, 0x00, byte(i+1))
		// Route type 2, length 33
		nlri = append(nlri, 0x02, 33)
		// Payload (33 bytes of dummy data)
		for j := 0; j < 33; j++ {
			nlri = append(nlri, byte(j))
		}
	}

	// Each EVPN Add-Path NLRI = 4 + 2 + 33 = 39 bytes
	// 5 * 39 = 195 bytes total
	chunks, err := ChunkMPNLRI(nlri, 25, 70, true, 80) // AFI=L2VPN(25), SAFI=EVPN(70)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split")

	// Verify all bytes preserved
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}

// TestChunkMPNLRI_FlowSpec_AddPath verifies FlowSpec with Add-Path.
//
// VALIDATES: FlowSpec NLRIs with path-id prefix parsed correctly.
// PREVENTS: Path-id corruption in FlowSpec Add-Path scenarios.
func TestChunkMPNLRI_FlowSpec_AddPath(t *testing.T) {
	// FlowSpec Add-Path: [path-id:4][length:1][components]
	var nlri []byte
	for i := 0; i < 10; i++ {
		// Path-id (4 bytes)
		nlri = append(nlri, 0x00, 0x00, 0x00, byte(i+1))
		// Length (1 byte) + components
		nlri = append(nlri, 10) // 10 bytes of components
		for j := 0; j < 10; j++ {
			nlri = append(nlri, byte(j))
		}
	}

	// Each FlowSpec Add-Path NLRI = 4 + 1 + 10 = 15 bytes
	// 10 * 15 = 150 bytes total
	chunks, err := ChunkMPNLRI(nlri, 1, 133, true, 50) // AFI=IPv4(1), SAFI=FlowSpec(133)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split")

	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}

// TestChunkMPNLRI_FlowSpec_AddPath_LongLength verifies FlowSpec Add-Path with 2-byte length.
//
// VALIDATES: FlowSpec with length >= 240 uses 2-byte encoding with Add-Path.
// PREVENTS: Parsing errors at the 240-byte boundary.
func TestChunkMPNLRI_FlowSpec_AddPath_LongLength(t *testing.T) {
	// FlowSpec Add-Path with 2-byte length: [path-id:4][0xF0|high:1][low:1][components]
	nlri := make([]byte, 0)
	// Path-id
	nlri = append(nlri, 0x00, 0x00, 0x00, 0x01)
	// 2-byte length encoding for 250 bytes: 0xF0 | (250 >> 8), 250 & 0xFF = 0xF0, 0xFA
	nlri = append(nlri, 0xF0, 0xFA)
	// 250 bytes of components
	for i := 0; i < 250; i++ {
		nlri = append(nlri, byte(i))
	}

	// Total = 4 + 2 + 250 = 256 bytes
	chunks, err := ChunkMPNLRI(nlri, 1, 133, true, 300)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, nlri, chunks[0])
}

// TestChunkMPNLRI_BGPLS_AddPath verifies BGP-LS with Add-Path.
//
// VALIDATES: BGP-LS NLRIs with path-id prefix parsed correctly.
// PREVENTS: Path-id corruption in BGP-LS Add-Path scenarios.
func TestChunkMPNLRI_BGPLS_AddPath(t *testing.T) {
	// BGP-LS Add-Path: [path-id:4][nlri-type:2][length:2][payload]
	var nlri []byte
	for i := 0; i < 5; i++ {
		// Path-id (4 bytes)
		nlri = append(nlri, 0x00, 0x00, 0x00, byte(i+1))
		// NLRI type (2 bytes) - Node NLRI = 1
		nlri = append(nlri, 0x00, 0x01)
		// Length (2 bytes) - 20 bytes payload
		nlri = append(nlri, 0x00, 0x14)
		// Payload (20 bytes)
		for j := 0; j < 20; j++ {
			nlri = append(nlri, byte(j))
		}
	}

	// Each BGP-LS Add-Path NLRI = 4 + 4 + 20 = 28 bytes
	// 5 * 28 = 140 bytes total
	chunks, err := ChunkMPNLRI(nlri, 16388, 71, true, 60) // AFI=BGP-LS(16388), SAFI=71
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split")

	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}

// =============================================================================
// SplitMPNLRI Tests (Subslice-Based Splitting)
// =============================================================================

// TestSplitMPNLRI_Subslice verifies SplitMPNLRI returns subslices of original buffer.
//
// VALIDATES: fitting and remaining are subslices of original nlriData (not copies).
// PREVENTS: Unnecessary allocation/copying in hot path.
func TestSplitMPNLRI_Subslice(t *testing.T) {
	// Create NLRI buffer with 10 /24 prefixes (4 bytes each = 40 bytes)
	nlri := make([]byte, 40)
	for i := 0; i < 10; i++ {
		nlri[i*4] = 24 // /24
		nlri[i*4+1] = 10
		nlri[i*4+2] = 0
		nlri[i*4+3] = byte(i)
	}

	// maxSize = 20, fits 5 prefixes
	fitting, remaining, err := SplitMPNLRI(nlri, 1, 1, false, 20)
	require.NoError(t, err)

	// Verify fitting is a subslice (same underlying array)
	require.NotNil(t, fitting)
	require.NotNil(t, remaining)

	// Check that fitting points to start of original buffer
	assert.Equal(t, &nlri[0], &fitting[0], "fitting should be subslice of original")

	// Check that remaining points to middle of original buffer
	assert.Equal(t, &nlri[len(fitting)], &remaining[0], "remaining should be subslice of original")

	// Verify no gap between fitting and remaining
	assert.Equal(t, len(nlri), len(fitting)+len(remaining), "fitting+remaining should cover all data")
}

// TestSplitMPNLRI_NoAllocHotPath verifies no allocations in split loop.
//
// VALIDATES: testing.AllocsPerRun returns 0 for SplitMPNLRI calls.
// PREVENTS: Hidden allocations degrading forwarding performance.
func TestSplitMPNLRI_NoAllocHotPath(t *testing.T) {
	// Create NLRI buffer with 10 /24 prefixes
	nlri := make([]byte, 40)
	for i := 0; i < 10; i++ {
		nlri[i*4] = 24
		nlri[i*4+1] = 10
		nlri[i*4+2] = 0
		nlri[i*4+3] = byte(i)
	}

	// Warm up
	_, _, _ = SplitMPNLRI(nlri, 1, 1, false, 20)

	// Measure allocations
	allocs := testing.AllocsPerRun(100, func() {
		_, _, _ = SplitMPNLRI(nlri, 1, 1, false, 20)
	})

	assert.Equal(t, float64(0), allocs, "SplitMPNLRI should not allocate in hot path")
}

// TestSplitMPNLRI_BoundaryExact verifies exact maxSize boundary handling.
//
// VALIDATES: NLRI data exactly at maxSize returns all as fitting, nil remaining.
// PREVENTS: Off-by-one errors at exact boundaries.
func TestSplitMPNLRI_BoundaryExact(t *testing.T) {
	// 5 /24 prefixes = exactly 20 bytes
	nlri := make([]byte, 20)
	for i := 0; i < 5; i++ {
		nlri[i*4] = 24
		nlri[i*4+1] = 10
		nlri[i*4+2] = 0
		nlri[i*4+3] = byte(i)
	}

	// maxSize = 20, exactly fits all
	fitting, remaining, err := SplitMPNLRI(nlri, 1, 1, false, 20)
	require.NoError(t, err)

	assert.Equal(t, nlri, fitting, "should return all data as fitting")
	assert.Nil(t, remaining, "remaining should be nil when all fits")
}

// TestSplitMPNLRI_SingleNLRIFillsBuffer verifies single NLRI at exact limit.
//
// VALIDATES: Single NLRI exactly at maxSize handled correctly.
// PREVENTS: Edge case where one NLRI equals maxSize.
func TestSplitMPNLRI_SingleNLRIFillsBuffer(t *testing.T) {
	// Single /128 IPv6 = 17 bytes (1 len + 16 prefix)
	nlri := make([]byte, 17)
	nlri[0] = 128
	for i := 1; i < 17; i++ {
		nlri[i] = byte(i)
	}

	// maxSize = 17, exactly fits one /128
	fitting, remaining, err := SplitMPNLRI(nlri, 2, 1, false, 17)
	require.NoError(t, err)

	assert.Equal(t, nlri, fitting, "should return single NLRI as fitting")
	assert.Nil(t, remaining, "remaining should be nil")
}

// TestSplitMPNLRI_InvalidMaxSize verifies error on invalid maxSize.
//
// VALIDATES: maxSize <= 0 returns error.
// PREVENTS: Panic or infinite loop on invalid input.
func TestSplitMPNLRI_InvalidMaxSize(t *testing.T) {
	nlri := []byte{24, 10, 0, 0}

	_, _, err := SplitMPNLRI(nlri, 1, 1, false, 0)
	require.Error(t, err)

	_, _, err = SplitMPNLRI(nlri, 1, 1, false, -1)
	require.Error(t, err)
}

// TestSplitMPNLRI_EmptyInput verifies empty/nil handling.
//
// VALIDATES: Empty input returns nil, nil, nil.
// PREVENTS: Panic on empty input.
func TestSplitMPNLRI_EmptyInput(t *testing.T) {
	fitting, remaining, err := SplitMPNLRI(nil, 1, 1, false, 100)
	require.NoError(t, err)
	assert.Nil(t, fitting)
	assert.Nil(t, remaining)

	fitting, remaining, err = SplitMPNLRI([]byte{}, 1, 1, false, 100)
	require.NoError(t, err)
	assert.Nil(t, fitting)
	assert.Nil(t, remaining)
}

// TestSplitMPNLRI_SingleTooLarge verifies error on oversized single NLRI.
//
// VALIDATES: Single NLRI > maxSize returns error.
// PREVENTS: Silent truncation or infinite loop.
func TestSplitMPNLRI_SingleTooLarge(t *testing.T) {
	// /128 IPv6 = 17 bytes
	nlri := make([]byte, 17)
	nlri[0] = 128

	_, _, err := SplitMPNLRI(nlri, 2, 1, false, 10)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNLRITooLarge)
}

// TestSplitMPNLRI_MultipleSplits verifies iterative splitting.
//
// VALIDATES: Repeated SplitMPNLRI calls correctly split large buffer.
// PREVENTS: Incorrect offset tracking across multiple splits.
func TestSplitMPNLRI_MultipleSplits(t *testing.T) {
	// 20 /24 prefixes = 80 bytes
	nlri := make([]byte, 80)
	for i := 0; i < 20; i++ {
		nlri[i*4] = 24
		nlri[i*4+1] = 10
		nlri[i*4+2] = 0
		nlri[i*4+3] = byte(i)
	}

	// maxSize = 20, fits 5 prefixes per chunk
	var chunks [][]byte
	remaining := nlri
	for len(remaining) > 0 {
		fitting, rem, err := SplitMPNLRI(remaining, 1, 1, false, 20)
		require.NoError(t, err)
		require.NotNil(t, fitting)
		chunks = append(chunks, fitting)
		remaining = rem
	}

	// Should have 4 chunks
	require.Len(t, chunks, 4)

	// Reassemble and verify
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk...)
	}
	assert.Equal(t, nlri, reassembled)
}

// =============================================================================
// Error Cases
// =============================================================================

// TestChunkMPNLRI_Empty verifies empty NLRI handling.
//
// VALIDATES: Empty input returns nil/empty without error.
// PREVENTS: Panic on empty input.
func TestChunkMPNLRI_Empty(t *testing.T) {
	chunks, err := ChunkMPNLRI(nil, 1, 1, false, 100)
	require.NoError(t, err)
	assert.Nil(t, chunks)

	chunks, err = ChunkMPNLRI([]byte{}, 1, 1, false, 100)
	require.NoError(t, err)
	assert.Nil(t, chunks)
}

// TestChunkMPNLRI_Truncated verifies truncated NLRI detection.
//
// VALIDATES: Error returned for malformed NLRI.
// PREVENTS: Out-of-bounds panic.
func TestChunkMPNLRI_Truncated(t *testing.T) {
	// IPv6 /64 should be 9 bytes, but only 5 provided
	nlri := []byte{64, 0x20, 0x01, 0x0d, 0xb8}

	_, err := ChunkMPNLRI(nlri, 2, 1, false, 100)
	require.Error(t, err)
}

// TestChunkMPNLRI_SingleTooLarge verifies single NLRI overflow.
//
// VALIDATES: Error when single NLRI > maxSize.
// PREVENTS: Infinite loop or silent truncation.
func TestChunkMPNLRI_SingleTooLarge(t *testing.T) {
	// /128 IPv6 = 17 bytes
	nlri := make([]byte, 17)
	nlri[0] = 128

	_, err := ChunkMPNLRI(nlri, 2, 1, false, 10)
	require.Error(t, err)
}

// =============================================================================
// Integration Tests: SplitUpdate with MP families
// =============================================================================

// TestSplitUpdate_DetectsMPReach verifies MP_REACH_NLRI detection.
//
// VALIDATES: SplitUpdate finds and splits MP_REACH_NLRI in PathAttributes.
// PREVENTS: MP families passing through unsplit.
func TestSplitUpdate_DetectsMPReach(t *testing.T) {
	// Build UPDATE with MP_REACH_NLRI containing many IPv6 prefixes
	var mpNLRI []byte
	for i := 0; i < 100; i++ {
		mpNLRI = append(mpNLRI, 64) // /64
		mpNLRI = append(mpNLRI, 0x20, 0x01, 0x0d, 0xb8, byte(i>>8), byte(i), 0, 0)
	}

	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(2),
		SAFI:     attribute.SAFI(1),
		NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
		NLRI:     mpNLRI,
	}

	// Pack MP_REACH_NLRI as attribute
	pathAttrs := make([]byte, 3+mpReach.Len())
	if mpReach.Len() > 255 {
		pathAttrs = make([]byte, 4+mpReach.Len())
	}
	attribute.WriteAttrTo(mpReach, pathAttrs, 0)

	u := &Update{
		PathAttributes: pathAttrs,
		// NLRI empty (IPv6 uses MP_REACH_NLRI)
	}

	// Split with small maxSize to force splitting
	chunks, err := SplitUpdate(u, 200)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "MP_REACH_NLRI should be split")

	// Verify each chunk is valid and within size
	for i, chunk := range chunks {
		size := HeaderLen + 4 + len(chunk.PathAttributes) + len(chunk.NLRI)
		assert.LessOrEqual(t, size, 200, "chunk %d exceeds maxSize", i)
	}
}

// TestSplitUpdate_DetectsMPUnreach verifies MP_UNREACH_NLRI detection.
//
// VALIDATES: SplitUpdate finds and splits MP_UNREACH_NLRI.
// PREVENTS: MP withdrawals passing through unsplit.
func TestSplitUpdate_DetectsMPUnreach(t *testing.T) {
	// Build UPDATE with MP_UNREACH_NLRI containing many prefixes
	var mpNLRI []byte
	for i := 0; i < 100; i++ {
		mpNLRI = append(mpNLRI, 64)
		mpNLRI = append(mpNLRI, 0x20, 0x01, 0x0d, 0xb8, byte(i>>8), byte(i), 0, 0)
	}

	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(2),
		SAFI: attribute.SAFI(1),
		NLRI: mpNLRI,
	}

	attrLen := mpUnreach.Len()
	hdrLen := 3
	if attrLen > 255 {
		hdrLen = 4
	}
	pathAttrs := make([]byte, hdrLen+attrLen)
	attribute.WriteAttrTo(mpUnreach, pathAttrs, 0)

	u := &Update{
		PathAttributes: pathAttrs,
	}

	chunks, err := SplitUpdate(u, 200)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "MP_UNREACH_NLRI should be split")
}

// TestSplitUpdate_PreservesOtherAttrs verifies non-MP attributes preserved.
//
// VALIDATES: ORIGIN, AS_PATH, etc. identical in all split chunks.
// PREVENTS: Attribute loss or corruption during MP split.
func TestSplitUpdate_PreservesOtherAttrs(t *testing.T) {
	// Build UPDATE with ORIGIN + AS_PATH + MP_REACH_NLRI
	var attrs []byte

	// ORIGIN = IGP
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00)

	// AS_PATH = [65001]
	attrs = append(attrs, 0x40, 0x02, 0x04, 0x02, 0x01, 0xFD, 0xE9)

	// MP_REACH_NLRI with many prefixes
	var mpNLRI []byte
	for i := 0; i < 50; i++ {
		mpNLRI = append(mpNLRI, 64)
		mpNLRI = append(mpNLRI, 0x20, 0x01, 0x0d, 0xb8, byte(i), 0, 0, 0)
	}
	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(2),
		SAFI:     attribute.SAFI(1),
		NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
		NLRI:     mpNLRI,
	}
	mpReachLen := mpReach.Len()
	mpReachHdr := 3
	if mpReachLen > 255 {
		mpReachHdr = 4
	}
	mpReachBuf := make([]byte, mpReachHdr+mpReachLen)
	attribute.WriteAttrTo(mpReach, mpReachBuf, 0)
	attrs = append(attrs, mpReachBuf...)

	u := &Update{PathAttributes: attrs}

	chunks, err := SplitUpdate(u, 150)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	// All chunks should have ORIGIN and AS_PATH
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	asPath := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0xFD, 0xE9}
	for i, chunk := range chunks {
		// Check ORIGIN present (use bytes.Contains for subsequence matching)
		assert.True(t, bytes.Contains(chunk.PathAttributes, origin),
			"chunk %d missing ORIGIN", i)

		// Check AS_PATH present
		assert.True(t, bytes.Contains(chunk.PathAttributes, asPath),
			"chunk %d missing AS_PATH", i)
	}
}

// TestSplitUpdate_BothMPReachAndUnreach verifies mixed MP_REACH + MP_UNREACH handling.
//
// VALIDATES: UPDATE with both MP_REACH_NLRI and MP_UNREACH_NLRI splits correctly.
// PREVENTS: Index corruption when both MP attributes are present and need removal.
func TestSplitUpdate_BothMPReachAndUnreach(t *testing.T) {
	// Build UPDATE with ORIGIN + AS_PATH + MP_UNREACH_NLRI + MP_REACH_NLRI
	var attrs []byte

	// ORIGIN = IGP
	attrs = append(attrs, 0x40, 0x01, 0x01, 0x00)

	// AS_PATH = [65001]
	attrs = append(attrs, 0x40, 0x02, 0x04, 0x02, 0x01, 0xFD, 0xE9)

	// MP_UNREACH_NLRI with many prefixes (withdrawals)
	var unreachNLRI []byte
	for i := 0; i < 20; i++ {
		unreachNLRI = append(unreachNLRI, 64)
		unreachNLRI = append(unreachNLRI, 0x20, 0x01, 0x0d, 0xb8, 0xFF, byte(i), 0, 0)
	}
	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(2),
		SAFI: attribute.SAFI(1),
		NLRI: unreachNLRI,
	}
	mpUnreachLen := mpUnreach.Len()
	mpUnreachHdr := 3
	if mpUnreachLen > 255 {
		mpUnreachHdr = 4
	}
	mpUnreachBuf := make([]byte, mpUnreachHdr+mpUnreachLen)
	attribute.WriteAttrTo(mpUnreach, mpUnreachBuf, 0)
	attrs = append(attrs, mpUnreachBuf...)

	// MP_REACH_NLRI with many prefixes (announcements)
	var reachNLRI []byte
	for i := 0; i < 30; i++ {
		reachNLRI = append(reachNLRI, 64)
		reachNLRI = append(reachNLRI, 0x20, 0x01, 0x0d, 0xb8, byte(i), 0, 0, 0)
	}
	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(2),
		SAFI:     attribute.SAFI(1),
		NextHops: []netip.Addr{netip.MustParseAddr("2001:db8::1")},
		NLRI:     reachNLRI,
	}
	mpReachLen := mpReach.Len()
	mpReachHdr := 3
	if mpReachLen > 255 {
		mpReachHdr = 4
	}
	mpReachBuf := make([]byte, mpReachHdr+mpReachLen)
	attribute.WriteAttrTo(mpReach, mpReachBuf, 0)
	attrs = append(attrs, mpReachBuf...)

	u := &Update{PathAttributes: attrs}

	// maxSize that allows splitting but is small enough to force multiple chunks
	// Overhead = 23, ORIGIN = 4, AS_PATH = 7 = 34 base
	// MP_REACH overhead: AFI(2) + SAFI(1) + NH_LEN(1) + NH(16) + Reserved(1) + header(4) = 25
	// MP_UNREACH overhead: AFI(2) + SAFI(1) + header(4) = 7
	// Each /64 NLRI = 9 bytes
	// Use maxSize = 300 to allow ~20 NLRIs per MP chunk
	chunks, err := SplitUpdate(u, 300)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split into multiple chunks")

	// Verify all chunks have valid structure
	for i, chunk := range chunks {
		// All chunks should have ORIGIN and AS_PATH
		origin := []byte{0x40, 0x01, 0x01, 0x00}
		asPath := []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0xFD, 0xE9}
		assert.True(t, bytes.Contains(chunk.PathAttributes, origin),
			"chunk %d missing ORIGIN", i)
		assert.True(t, bytes.Contains(chunk.PathAttributes, asPath),
			"chunk %d missing AS_PATH", i)
	}

	// Count total NLRIs across chunks
	totalReachNLRI := 0
	totalUnreachNLRI := 0
	for _, chunk := range chunks {
		// Check for MP_REACH_NLRI (type 14)
		mpReachInfo := findMPAttribute(chunk.PathAttributes, attribute.AttrMPReachNLRI)
		if mpReachInfo.found {
			// Count /64 prefixes (9 bytes each)
			totalReachNLRI += len(mpReachInfo.value) / 9 // Approximate
		}

		// Check for MP_UNREACH_NLRI (type 15)
		mpUnreachInfo := findMPAttribute(chunk.PathAttributes, attribute.AttrMPUnreachNLRI)
		if mpUnreachInfo.found {
			// Count /64 prefixes (9 bytes each, after AFI/SAFI overhead)
			nlriBytes := len(mpUnreachInfo.value) - 3 // Subtract AFI(2) + SAFI(1)
			if nlriBytes > 0 {
				totalUnreachNLRI += nlriBytes / 9
			}
		}
	}

	// Verify no data loss (should have close to original counts)
	assert.GreaterOrEqual(t, totalReachNLRI, 20, "should preserve most reach NLRIs")
	assert.GreaterOrEqual(t, totalUnreachNLRI, 15, "should preserve most unreach NLRIs")
}

// TestSplitUpdate_AddPath verifies Add-Path NLRI splitting.
//
// VALIDATES: Add-Path NLRIs (4-byte path-id prefix) split correctly.
// PREVENTS: Corruption when splitting Add-Path enabled UPDATEs.
func TestSplitUpdate_AddPath(t *testing.T) {
	// Build IPv4 Add-Path NLRIs
	// Format: [path-id:4][prefix-len:1][prefix-bytes]
	var nlri []byte
	for i := 0; i < 20; i++ {
		// Path ID (4 bytes)
		nlri = append(nlri, 0x00, 0x00, 0x00, byte(i+1))
		// /24 prefix (4 bytes total: 1 len + 3 prefix)
		nlri = append(nlri, 0x18, 0x0A, byte(i), 0x00)
	}

	u := &Update{
		PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, // ORIGIN
		NLRI:           nlri,
	}

	// Small maxSize to force splitting
	chunks, err := SplitUpdateWithAddPath(u, 80, true)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "should split Add-Path NLRIs")

	// Verify all NLRIs preserved
	var totalNLRIBytes int
	for _, chunk := range chunks {
		totalNLRIBytes += len(chunk.NLRI)
	}
	assert.Equal(t, len(nlri), totalNLRIBytes, "all Add-Path NLRIs should be preserved")
}
