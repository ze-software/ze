package context_test

// Encoding Tests for EncodingContext
//
// These tests verify that the EncodingContext correctly influences wire encoding.
// The context controls how NLRI and attributes are serialized based on
// negotiated capabilities.
//
// ASN4 Encoding Permutations (RFC 6793):
//
//	Test Case | ctx.ASN4 | ASN Value  | Encoded As
//	----------|----------|------------|---------------------------
//	1         | true     | 65001      | 4 bytes: 0x00 0x00 0xFD 0xE9
//	2         | true     | 4200000001 | 4 bytes: 0xFA 0x56 0xEA 0x01
//	3         | true     | 65535      | 4 bytes: 0x00 0x00 0xFF 0xFF
//	4         | true     | 65536      | 4 bytes: 0x00 0x01 0x00 0x00
//	5         | false    | 65001      | 2 bytes: 0xFD 0xE9
//	6         | false    | 4200000001 | 2 bytes: 0x5B 0xA0 (AS_TRANS=23456)
//	7         | false    | 65535      | 2 bytes: 0xFF 0xFF
//	8         | false    | 65536      | 2 bytes: 0x5B 0xA0 (AS_TRANS=23456)
//
// ADD-PATH Encoding Permutations (RFC 7911):
//
//	Test Case | ctx.AddPath | Path ID    | NLRI Wire Format
//	----------|-------------|------------|----------------------------------
//	1         | true        | 0          | [00 00 00 00][prefixLen][prefix]
//	2         | true        | 1          | [00 00 00 01][prefixLen][prefix]
//	3         | true        | 256        | [00 00 01 00][prefixLen][prefix]
//	4         | true        | 0xFFFFFFFF | [FF FF FF FF][prefixLen][prefix]
//	5         | false       | (any)      | [prefixLen][prefix] (no pathID)
//
// ADD-PATH Prefix Length Permutations:
//
//	Prefix    | AddPath | Wire Length | Format
//	----------|---------|-------------|----------------------------------
//	/0        | true    | 5 bytes     | [pathID:4][len:1]
//	/8        | true    | 6 bytes     | [pathID:4][len:1][prefix:1]
//	/16       | true    | 7 bytes     | [pathID:4][len:1][prefix:2]
//	/24       | true    | 8 bytes     | [pathID:4][len:1][prefix:3]
//	/32       | true    | 9 bytes     | [pathID:4][len:1][prefix:4]
//
// IPv6 ADD-PATH Permutations:
//
//	Prefix    | AddPath | Wire Length | Format
//	----------|---------|-------------|----------------------------------
//	/64       | true    | 13 bytes    | [pathID:4][len:1][prefix:8]
//	/128      | true    | 21 bytes    | [pathID:4][len:1][prefix:16]
//	/64       | false   | 9 bytes     | [len:1][prefix:8]

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/attribute"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// =============================================================================
// ASN4 Encoding Tests (RFC 6793)
// =============================================================================

// TestASPathEncodingWithASN4 verifies AS_PATH encoding with 4-byte ASNs.
//
// VALIDATES: When ASN4=true, ASNs are encoded as 4-byte values.
//
// PREVENTS: Wrong AS_PATH encoding between NEW BGP speakers.
func TestASPathEncodingWithASN4(t *testing.T) {
	tests := []struct {
		name     string
		asn      uint32
		expected []byte // Expected 4-byte encoding
	}{
		{
			name:     "2-byte range ASN",
			asn:      65001,
			expected: []byte{0x00, 0x00, 0xFD, 0xE9}, // 65001 as 4 bytes
		},
		{
			name:     "4-byte ASN",
			asn:      4200000001,
			expected: []byte{0xFA, 0x56, 0xEA, 0x01}, // 4200000001 as 4 bytes
		},
		{
			name:     "max 2-byte ASN",
			asn:      65535,
			expected: []byte{0x00, 0x00, 0xFF, 0xFF}, // 65535 as 4 bytes
		},
		{
			name:     "min 4-byte ASN",
			asn:      65536,
			expected: []byte{0x00, 0x01, 0x00, 0x00}, // 65536 as 4 bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asPath := &attribute.ASPath{
				Segments: []attribute.ASPathSegment{
					{Type: attribute.ASSequence, ASNs: []uint32{tt.asn}},
				},
			}

			// Encode with ASN4=true (context indicates 4-byte AS speaker)
			buf := make([]byte, 64)
			n := asPath.WriteToWithASN4(buf, 0, true)
			packed := buf[:n]

			// Format: [type:1][count:1][asn:4]
			require.Len(t, packed, 6, "should be 6 bytes: type(1)+count(1)+asn(4)")
			require.Equal(t, byte(attribute.ASSequence), packed[0], "segment type")
			require.Equal(t, byte(1), packed[1], "ASN count")
			require.Equal(t, tt.expected, packed[2:6], "ASN encoding")
		})
	}
}

// TestASPathEncodingWithoutASN4 verifies AS_PATH encoding with 2-byte ASNs.
//
// VALIDATES: When ASN4=false, ASNs >65535 become AS_TRANS (23456).
//
// PREVENTS: Wrong AS_PATH encoding to OLD BGP speakers.
func TestASPathEncodingWithoutASN4(t *testing.T) {
	tests := []struct {
		name     string
		asn      uint32
		expected []byte // Expected 2-byte encoding
	}{
		{
			name:     "2-byte range ASN preserved",
			asn:      65001,
			expected: []byte{0xFD, 0xE9}, // 65001 as 2 bytes
		},
		{
			name:     "4-byte ASN becomes AS_TRANS",
			asn:      4200000001,
			expected: []byte{0x5B, 0xA0}, // 23456 (AS_TRANS) as 2 bytes
		},
		{
			name:     "max 2-byte ASN preserved",
			asn:      65535,
			expected: []byte{0xFF, 0xFF}, // 65535 as 2 bytes
		},
		{
			name:     "min 4-byte ASN becomes AS_TRANS",
			asn:      65536,
			expected: []byte{0x5B, 0xA0}, // 23456 (AS_TRANS) as 2 bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asPath := &attribute.ASPath{
				Segments: []attribute.ASPathSegment{
					{Type: attribute.ASSequence, ASNs: []uint32{tt.asn}},
				},
			}

			// Encode with ASN4=false (context indicates 2-byte AS speaker)
			buf := make([]byte, 64)
			n := asPath.WriteToWithASN4(buf, 0, false)
			packed := buf[:n]

			// Format: [type:1][count:1][asn:2]
			require.Len(t, packed, 4, "should be 4 bytes: type(1)+count(1)+asn(2)")
			require.Equal(t, byte(attribute.ASSequence), packed[0], "segment type")
			require.Equal(t, byte(1), packed[1], "ASN count")
			require.Equal(t, tt.expected, packed[2:4], "ASN encoding")
		})
	}
}

// TestASPathEncodingContextIntegration verifies context.ASN4 drives encoding.
//
// VALIDATES: EncodingContext.ASN4 correctly passed to attribute packing.
//
// PREVENTS: Context not being used for encoding decisions.
func TestASPathEncodingContextIntegration(t *testing.T) {
	// 4-byte ASN that should become AS_TRANS when ASN4=false
	largeASN := uint32(4200000001)
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{largeASN}},
		},
	}

	// Context with ASN4=true (NEW speaker)
	ctxASN4 := bgpctx.EncodingContextForASN4(true)
	bufASN4 := make([]byte, 64)
	nASN4 := asPath.WriteToWithASN4(bufASN4, 0, ctxASN4.ASN4())
	packedASN4 := bufASN4[:nASN4]

	// Context with ASN4=false (OLD speaker)
	ctxNoASN4 := bgpctx.EncodingContextForASN4(false)
	bufNoASN4 := make([]byte, 64)
	nNoASN4 := asPath.WriteToWithASN4(bufNoASN4, 0, ctxNoASN4.ASN4())
	packedNoASN4 := bufNoASN4[:nNoASN4]

	// ASN4 encoding should be 6 bytes (4-byte ASN)
	require.Len(t, packedASN4, 6, "ASN4=true should produce 6-byte encoding")
	// Verify actual ASN value is preserved
	require.Equal(t, largeASN, binary.BigEndian.Uint32(packedASN4[2:6]))

	// Non-ASN4 encoding should be 4 bytes (2-byte ASN)
	require.Len(t, packedNoASN4, 4, "ASN4=false should produce 4-byte encoding")
	// Verify AS_TRANS is used
	require.Equal(t, uint16(23456), binary.BigEndian.Uint16(packedNoASN4[2:4]))
}

// =============================================================================
// ADD-PATH Encoding Tests (RFC 7911)
// =============================================================================

// TestNLRIEncodingWithAddPath verifies NLRI includes path ID when AddPath=true.
//
// VALIDATES: When AddPath=true, NLRI wire format includes 4-byte path ID.
//
// PREVENTS: Missing path ID in ADD-PATH enabled sessions.
func TestNLRIEncodingWithAddPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	pathID := uint32(42)

	n := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, pathID)

	// Context with AddPath=true
	addPath := true
	packed := func() []byte {
		b := make([]byte, nlri.LenWithContext(n, addPath))
		nlri.WriteNLRI(n, b, 0, addPath)
		return b
	}()

	// Format with AddPath: [pathID:4][prefixLen:1][prefix:3]
	require.Len(t, packed, 8, "should be 8 bytes: pathID(4)+prefixLen(1)+prefix(3)")

	// Verify path ID is first
	gotPathID := binary.BigEndian.Uint32(packed[0:4])
	require.Equal(t, pathID, gotPathID, "path ID should be encoded first")

	// Verify prefix length
	require.Equal(t, byte(24), packed[4], "prefix length")

	// Verify prefix bytes
	require.Equal(t, []byte{10, 0, 0}, packed[5:8], "prefix bytes")
}

// TestNLRIEncodingWithoutAddPath verifies NLRI excludes path ID when AddPath=false.
//
// VALIDATES: When AddPath=false, NLRI wire format has no path ID.
//
// PREVENTS: Spurious path ID in non-ADD-PATH sessions.
func TestNLRIEncodingWithoutAddPath(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	pathID := uint32(42) // Even though set, should not appear in wire format

	n := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, pathID)

	// Context with AddPath=false
	addPath := false
	packed := func() []byte {
		b := make([]byte, nlri.LenWithContext(n, addPath))
		nlri.WriteNLRI(n, b, 0, addPath)
		return b
	}()

	// Format without AddPath: [prefixLen:1][prefix:3]
	require.Len(t, packed, 4, "should be 4 bytes: prefixLen(1)+prefix(3)")

	// Verify prefix length is first (no path ID)
	require.Equal(t, byte(24), packed[0], "prefix length should be first byte")

	// Verify prefix bytes
	require.Equal(t, []byte{10, 0, 0}, packed[1:4], "prefix bytes")
}

// TestNLRIEncodingContextIntegration verifies context.AddPath drives encoding.
//
// VALIDATES: EncodingContext.AddPath correctly passed to NLRI packing.
//
// PREVENTS: Context not being used for ADD-PATH decisions.
func TestNLRIEncodingContextIntegration(t *testing.T) {
	prefix := netip.MustParsePrefix("192.168.1.0/24")
	pathID := uint32(1)
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	n := nlri.NewINET(family, prefix, pathID)

	// Context with AddPath=true
	ctxAddPath := bgpctx.EncodingContextWithAddPath(true, map[bgpctx.Family]bool{{AFI: 1, SAFI: 1}: true})
	addPathTrue := ctxAddPath.AddPath(family)
	packedWithPath := make([]byte, nlri.LenWithContext(n, addPathTrue))
	nlri.WriteNLRI(n, packedWithPath, 0, addPathTrue)

	// Context with AddPath=false
	ctxNoAddPath := bgpctx.EncodingContextWithAddPath(true, map[bgpctx.Family]bool{{AFI: 1, SAFI: 1}: false})
	addPathFalse := ctxNoAddPath.AddPath(family)
	packedWithoutPath := make([]byte, nlri.LenWithContext(n, addPathFalse))
	nlri.WriteNLRI(n, packedWithoutPath, 0, addPathFalse)

	// With AddPath: 4 bytes longer due to path ID
	require.Equal(t, len(packedWithPath), len(packedWithoutPath)+4,
		"AddPath encoding should be 4 bytes longer")

	// Verify path ID in AddPath encoding
	require.Equal(t, pathID, binary.BigEndian.Uint32(packedWithPath[0:4]),
		"path ID should be at start of AddPath encoding")
}

// TestAddPathEncodingPermutations verifies ADD-PATH encoding for all path ID values.
//
// ADD-PATH Path ID Encoding Permutations (RFC 7911):
//
//	Test Case | Path ID    | AddPath | Wire Format
//	----------|------------|---------|----------------------------------------
//	1         | 0          | true    | [00 00 00 00][prefixLen][prefix...]
//	2         | 1          | true    | [00 00 00 01][prefixLen][prefix...]
//	3         | 0xFFFFFFFF | true    | [FF FF FF FF][prefixLen][prefix...]
//	4         | 42         | false   | [prefixLen][prefix...] (no path ID)
//
// VALIDATES: Path IDs are correctly encoded as big-endian 4-byte values.
//
// PREVENTS: Wrong byte order or missing path IDs.
func TestAddPathEncodingPermutations(t *testing.T) {
	tests := []struct {
		name     string
		pathID   uint32
		addPath  bool
		expected []byte // Expected path ID bytes (empty if addPath=false)
	}{
		{
			name:     "path ID 0",
			pathID:   0,
			addPath:  true,
			expected: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			name:     "path ID 1",
			pathID:   1,
			addPath:  true,
			expected: []byte{0x00, 0x00, 0x00, 0x01},
		},
		{
			name:     "path ID 256",
			pathID:   256,
			addPath:  true,
			expected: []byte{0x00, 0x00, 0x01, 0x00},
		},
		{
			name:     "path ID max uint32",
			pathID:   0xFFFFFFFF,
			addPath:  true,
			expected: []byte{0xFF, 0xFF, 0xFF, 0xFF},
		},
		{
			name:     "path ID ignored when addPath=false",
			pathID:   42,
			addPath:  false,
			expected: nil, // No path ID in wire format
		},
	}

	prefix := netip.MustParsePrefix("10.0.0.0/24")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, tt.pathID)

			packed := func() []byte {
				b := make([]byte, nlri.LenWithContext(n, tt.addPath))
				nlri.WriteNLRI(n, b, 0, tt.addPath)
				return b
			}()

			if tt.addPath {
				// With AddPath: [pathID:4][prefixLen:1][prefix:3] = 8 bytes
				require.Len(t, packed, 8, "should be 8 bytes with path ID")
				require.Equal(t, tt.expected, packed[0:4], "path ID encoding")
				require.Equal(t, byte(24), packed[4], "prefix length after path ID")
			} else {
				// Without AddPath: [prefixLen:1][prefix:3] = 4 bytes
				require.Len(t, packed, 4, "should be 4 bytes without path ID")
				require.Equal(t, byte(24), packed[0], "prefix length first")
			}
		})
	}
}

// TestAddPathEncodingPrefixLengths verifies ADD-PATH with various prefix lengths.
//
// VALIDATES: Path ID + prefix length combinations encode correctly.
//
// PREVENTS: Prefix length calculation errors with ADD-PATH.
func TestAddPathEncodingPrefixLengths(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		pathID      uint32
		expectedLen int // Total wire length with AddPath=true
		prefixBytes int // Number of prefix bytes
	}{
		{
			name:        "/8 prefix",
			prefix:      "10.0.0.0/8",
			pathID:      1,
			expectedLen: 6, // 4 (pathID) + 1 (len) + 1 (prefix)
			prefixBytes: 1,
		},
		{
			name:        "/16 prefix",
			prefix:      "10.0.0.0/16",
			pathID:      2,
			expectedLen: 7, // 4 (pathID) + 1 (len) + 2 (prefix)
			prefixBytes: 2,
		},
		{
			name:        "/24 prefix",
			prefix:      "10.0.0.0/24",
			pathID:      3,
			expectedLen: 8, // 4 (pathID) + 1 (len) + 3 (prefix)
			prefixBytes: 3,
		},
		{
			name:        "/32 prefix",
			prefix:      "10.0.0.1/32",
			pathID:      4,
			expectedLen: 9, // 4 (pathID) + 1 (len) + 4 (prefix)
			prefixBytes: 4,
		},
		{
			name:        "/0 default route",
			prefix:      "0.0.0.0/0",
			pathID:      5,
			expectedLen: 5, // 4 (pathID) + 1 (len) + 0 (prefix)
			prefixBytes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tt.prefix)
			n := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, tt.pathID)
			addPath := true // All tests in this function use ADD-PATH
			packed := func() []byte {
				b := make([]byte, nlri.LenWithContext(n, addPath))
				nlri.WriteNLRI(n, b, 0, addPath)
				return b
			}()

			require.Len(t, packed, tt.expectedLen, "total wire length")

			// Verify path ID
			gotPathID := binary.BigEndian.Uint32(packed[0:4])
			require.Equal(t, tt.pathID, gotPathID, "path ID")

			// Verify prefix length byte
			require.Equal(t, byte(prefix.Bits()), packed[4], "prefix length byte")
		})
	}
}

// TestAddPathEncodingIPv6 verifies ADD-PATH encoding for IPv6 prefixes.
//
// VALIDATES: IPv6 NLRI correctly includes path ID when ADD-PATH enabled.
//
// PREVENTS: Wrong encoding for IPv6 with ADD-PATH.
func TestAddPathEncodingIPv6(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		pathID      uint32
		addPath     bool
		expectedLen int
	}{
		{
			name:        "IPv6 /64 with AddPath",
			prefix:      "2001:db8::/64",
			pathID:      100,
			addPath:     true,
			expectedLen: 13, // 4 (pathID) + 1 (len) + 8 (prefix)
		},
		{
			name:        "IPv6 /128 with AddPath",
			prefix:      "2001:db8::1/128",
			pathID:      200,
			addPath:     true,
			expectedLen: 21, // 4 (pathID) + 1 (len) + 16 (prefix)
		},
		{
			name:        "IPv6 /64 without AddPath",
			prefix:      "2001:db8::/64",
			pathID:      100,
			addPath:     false,
			expectedLen: 9, // 1 (len) + 8 (prefix)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tt.prefix)
			n := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, prefix, tt.pathID)

			packed := func() []byte {
				b := make([]byte, nlri.LenWithContext(n, tt.addPath))
				nlri.WriteNLRI(n, b, 0, tt.addPath)
				return b
			}()

			require.Len(t, packed, tt.expectedLen, "total wire length")

			if tt.addPath {
				// Verify path ID at start
				gotPathID := binary.BigEndian.Uint32(packed[0:4])
				require.Equal(t, tt.pathID, gotPathID, "path ID")
				// Verify prefix length after path ID
				require.Equal(t, byte(prefix.Bits()), packed[4], "prefix length")
			} else {
				// Verify prefix length is first byte
				require.Equal(t, byte(prefix.Bits()), packed[0], "prefix length")
			}
		})
	}
}
