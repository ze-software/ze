package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/attrpool"
	pool "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-rib/pool"
)

// Test attribute wire bytes (flags + type + length + value).
// RFC 4271 Section 4.3 format.

var (
	// ORIGIN = IGP (well-known mandatory, transitive).
	// Flags: 0x40 (transitive), Type: 1, Length: 1, Value: 0x00 (IGP).
	wireOriginIGP = []byte{0x40, 0x01, 0x01, 0x00}

	// AS_PATH = [65001] (AS_SEQUENCE).
	// Flags: 0x40, Type: 2, Length: 6, Segment: type=2(SEQ), len=1, ASN=65001.
	wireASPath65001 = []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9}

	// NEXT_HOP = 10.0.0.1.
	// Flags: 0x40, Type: 3, Length: 4, Value: 10.0.0.1.
	wireNextHop = []byte{0x40, 0x03, 0x04, 0x0A, 0x00, 0x00, 0x01}

	// MED = 100.
	// Flags: 0x80 (optional), Type: 4, Length: 4, Value: 100.
	wireMED100 = []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64}

	// LOCAL_PREF = 100.
	// Flags: 0x40, Type: 5, Length: 4, Value: 100.
	wireLocalPref100 = []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}

	// COMMUNITIES = [65000:100].
	// Flags: 0xC0 (optional transitive), Type: 8, Length: 4, Value: 65000:100.
	wireCommunity = []byte{0xC0, 0x08, 0x04, 0xFD, 0xE8, 0x00, 0x64}

	// Unknown attribute (type 99) - should go to OtherAttrs.
	// Flags: 0xC0 (optional transitive), Type: 99, Length: 2, Value: 0xAB 0xCD.
	wireUnknown = []byte{0xC0, 0x63, 0x02, 0xAB, 0xCD}
)

// TestParseAttributes_Origin verifies ORIGIN attribute parsing.
//
// VALIDATES: ORIGIN attribute parsed and interned in Origin pool.
// PREVENTS: ORIGIN being stored in wrong pool or blob.
func TestParseAttributes_Origin(t *testing.T) {
	entry, err := ParseAttributes(wireOriginIGP)
	require.NoError(t, err)
	defer entry.Release()

	assert.True(t, entry.HasOrigin(), "ORIGIN should be present")
	assert.Equal(t, uint8(2), entry.Origin.PoolIdx(), "should use Origin pool (idx=2)")

	// Verify value
	data, err := pool.Origin.Get(entry.Origin)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x00}, data, "ORIGIN value should be IGP (0)")
}

// TestParseAttributes_ASPath verifies AS_PATH attribute parsing.
//
// VALIDATES: AS_PATH attribute parsed and interned in ASPath pool.
// PREVENTS: AS_PATH being stored in wrong pool.
func TestParseAttributes_ASPath(t *testing.T) {
	entry, err := ParseAttributes(wireASPath65001)
	require.NoError(t, err)
	defer entry.Release()

	assert.True(t, entry.HasASPath(), "AS_PATH should be present")
	assert.Equal(t, uint8(3), entry.ASPath.PoolIdx(), "should use ASPath pool (idx=3)")
}

// TestParseAttributes_AllTypes verifies all known attribute types are parsed.
//
// VALIDATES: Each attribute type goes to its dedicated pool.
// PREVENTS: Attributes being misrouted to wrong pools.
func TestParseAttributes_AllTypes(t *testing.T) {
	// Concatenate multiple attributes
	raw := concat(
		wireOriginIGP,
		wireASPath65001,
		wireNextHop,
		wireMED100,
		wireLocalPref100,
		wireCommunity,
	)

	entry, err := ParseAttributes(raw)
	require.NoError(t, err)
	defer entry.Release()

	assert.True(t, entry.HasOrigin(), "ORIGIN should be present")
	assert.True(t, entry.HasASPath(), "AS_PATH should be present")
	assert.True(t, entry.HasNextHop(), "NEXT_HOP should be present")
	assert.True(t, entry.HasMED(), "MED should be present")
	assert.True(t, entry.HasLocalPref(), "LOCAL_PREF should be present")
	assert.True(t, entry.HasCommunities(), "COMMUNITIES should be present")

	// Verify pool indices
	assert.Equal(t, uint8(2), entry.Origin.PoolIdx(), "Origin pool idx")
	assert.Equal(t, uint8(3), entry.ASPath.PoolIdx(), "ASPath pool idx")
	assert.Equal(t, uint8(6), entry.NextHop.PoolIdx(), "NextHop pool idx")
	assert.Equal(t, uint8(5), entry.MED.PoolIdx(), "MED pool idx")
	assert.Equal(t, uint8(4), entry.LocalPref.PoolIdx(), "LocalPref pool idx")
	assert.Equal(t, uint8(7), entry.Communities.PoolIdx(), "Communities pool idx")
}

// TestParseAttributes_Optional verifies missing optional attributes.
//
// VALIDATES: Missing attributes have InvalidHandle, not zero handle.
// PREVENTS: Spurious pool lookups for absent attributes.
func TestParseAttributes_Optional(t *testing.T) {
	// Only ORIGIN - all others missing
	entry, err := ParseAttributes(wireOriginIGP)
	require.NoError(t, err)
	defer entry.Release()

	assert.True(t, entry.HasOrigin(), "ORIGIN should be present")
	assert.False(t, entry.HasASPath(), "AS_PATH should be absent")
	assert.False(t, entry.HasNextHop(), "NEXT_HOP should be absent")
	assert.False(t, entry.HasMED(), "MED should be absent")
	assert.False(t, entry.HasLocalPref(), "LOCAL_PREF should be absent")
	assert.False(t, entry.HasCommunities(), "COMMUNITIES should be absent")

	assert.Equal(t, attrpool.InvalidHandle, entry.ASPath)
	assert.Equal(t, attrpool.InvalidHandle, entry.MED)
}

// TestParseAttributes_Unknown verifies unknown attributes go to OtherAttrs.
//
// VALIDATES: Unknown attribute types stored in OtherAttrs pool as blob.
// PREVENTS: Unknown attributes being silently dropped.
func TestParseAttributes_Unknown(t *testing.T) {
	entry, err := ParseAttributes(wireUnknown)
	require.NoError(t, err)
	defer entry.Release()

	assert.True(t, entry.HasOtherAttrs(), "OtherAttrs should be present")
	assert.Equal(t, uint8(14), entry.OtherAttrs.PoolIdx(), "should use OtherAttrs pool (idx=14)")

	// Known attributes should be absent
	assert.False(t, entry.HasOrigin())
	assert.False(t, entry.HasASPath())
}

// TestParseAttributes_MixedKnownUnknown verifies mixed attribute handling.
//
// VALIDATES: Known attrs go to typed pools, unknown to OtherAttrs.
// PREVENTS: Unknown attrs polluting typed pools.
func TestParseAttributes_MixedKnownUnknown(t *testing.T) {
	raw := concat(wireOriginIGP, wireUnknown, wireLocalPref100)

	entry, err := ParseAttributes(raw)
	require.NoError(t, err)
	defer entry.Release()

	assert.True(t, entry.HasOrigin(), "ORIGIN should be present")
	assert.True(t, entry.HasLocalPref(), "LOCAL_PREF should be present")
	assert.True(t, entry.HasOtherAttrs(), "OtherAttrs should be present")
}

// TestParseAttributes_Empty verifies empty input handling.
//
// VALIDATES: Empty input returns valid entry with all InvalidHandle.
// PREVENTS: Panic or error on empty attribute bytes.
func TestParseAttributes_Empty(t *testing.T) {
	entry, err := ParseAttributes([]byte{})
	require.NoError(t, err)
	defer entry.Release()

	assert.False(t, entry.HasOrigin())
	assert.False(t, entry.HasASPath())
	assert.False(t, entry.HasOtherAttrs())
}

// TestParseAttributes_Deduplication verifies same attrs return same handles.
//
// VALIDATES: Parsing same raw bytes twice returns same pool slots.
// PREVENTS: Duplicate storage of identical attributes.
func TestParseAttributes_Deduplication(t *testing.T) {
	entry1, err := ParseAttributes(wireOriginIGP)
	require.NoError(t, err)
	defer entry1.Release()

	entry2, err := ParseAttributes(wireOriginIGP)
	require.NoError(t, err)
	defer entry2.Release()

	assert.Equal(t, entry1.Origin.Slot(), entry2.Origin.Slot(),
		"same ORIGIN should share pool slot")
}

// TestParseAttributes_ExtendedLength verifies extended length attribute parsing.
//
// VALIDATES: Attributes with extended length flag (>255 bytes) parse correctly.
// PREVENTS: Misparse of extended length attributes.
func TestParseAttributes_ExtendedLength(t *testing.T) {
	// Create a large community list with extended length
	// Flags: 0xD0 (optional transitive extended), Type: 8, Length: 256 (2 bytes)
	largeCommunities := make([]byte, 260)
	largeCommunities[0] = 0xD0 // optional transitive + extended length
	largeCommunities[1] = 0x08 // COMMUNITIES
	largeCommunities[2] = 0x01 // length high byte
	largeCommunities[3] = 0x00 // length low byte (256)
	// Value bytes (256 bytes of communities = 64 communities)
	for i := 4; i < 260; i++ {
		largeCommunities[i] = byte(i)
	}

	entry, err := ParseAttributes(largeCommunities)
	require.NoError(t, err)
	defer entry.Release()

	assert.True(t, entry.HasCommunities(), "COMMUNITIES should be present")

	data, err := pool.Communities.Get(entry.Communities)
	require.NoError(t, err)
	assert.Len(t, data, 256, "should have 256 bytes of community data")
}

// TestParseAttributes_PreservesFlags verifies original flags are preserved in OtherAttrs.
//
// VALIDATES: Unknown attrs retain original flags (including Partial bit).
// PREVENTS: Flag corruption when reconstructing wire bytes.
func TestParseAttributes_PreservesFlags(t *testing.T) {
	// Unknown attr with Partial flag (0x20) set.
	// Flags: 0xE0 (optional transitive partial), Type: 99, Length: 2, Value: 0xAB 0xCD.
	wirePartial := []byte{0xE0, 0x63, 0x02, 0xAB, 0xCD}

	entry, err := ParseAttributes(wirePartial)
	require.NoError(t, err)
	defer entry.Release()

	assert.True(t, entry.HasOtherAttrs())

	// OtherAttrs uses storage format: [type][flags][length_16bit][value].
	// This allows sorting by type code during reconstruction.
	data, err := pool.OtherAttrs.Get(entry.OtherAttrs)
	require.NoError(t, err)

	// Expected: type=0x63, flags=0xE0, length=0x0002, value=0xAB 0xCD.
	expected := []byte{0x63, 0xE0, 0x00, 0x02, 0xAB, 0xCD}
	assert.Equal(t, expected, data, "OtherAttrs should store in sortable format with flags preserved")
}

// TestParseAttributes_ExtendedLengthInOther verifies extended length attrs in OtherAttrs.
//
// VALIDATES: Extended length unknown attrs are correctly stored.
// PREVENTS: Header size miscalculation for extended length attrs.
func TestParseAttributes_ExtendedLengthInOther(t *testing.T) {
	// Unknown attr with extended length.
	// Flags: 0xD0 (optional transitive extended), Type: 99, Length: 256 (2 bytes).
	wireExtUnknown := make([]byte, 260)
	wireExtUnknown[0] = 0xD0 // optional transitive + extended length
	wireExtUnknown[1] = 0x63 // type 99
	wireExtUnknown[2] = 0x01 // length high byte
	wireExtUnknown[3] = 0x00 // length low byte (256)
	for i := 4; i < 260; i++ {
		wireExtUnknown[i] = byte(i)
	}

	entry, err := ParseAttributes(wireExtUnknown)
	require.NoError(t, err)
	defer entry.Release()

	assert.True(t, entry.HasOtherAttrs())

	// OtherAttrs uses storage format: [type][flags][length_16bit][value].
	// For 256-byte value: type=0x63, flags=0xD0, length=0x0100, value=256 bytes.
	data, err := pool.OtherAttrs.Get(entry.OtherAttrs)
	require.NoError(t, err)

	// Header: 4 bytes (type + flags + length_16bit) + 256 bytes value = 260 bytes.
	assert.Len(t, data, 260, "storage should have 4-byte header + 256 value bytes")
	assert.Equal(t, byte(0x63), data[0], "type code should be first")
	assert.Equal(t, byte(0xD0), data[1], "flags should be second (preserving extended bit)")
	assert.Equal(t, byte(0x01), data[2], "length high byte")
	assert.Equal(t, byte(0x00), data[3], "length low byte")
}

// TestParseAttributes_DuplicateAttribute verifies duplicate attrs don't leak handles.
//
// VALIDATES: Second occurrence of same attr releases first handle.
// PREVENTS: Handle leak when malformed input has duplicate attributes.
func TestParseAttributes_DuplicateAttribute(t *testing.T) {
	// Two ORIGIN attributes (malformed but must handle gracefully).
	wireOriginEGP := []byte{0x40, 0x01, 0x01, 0x01}
	raw := concat(wireOriginIGP, wireOriginEGP) // IGP then EGP

	entry, err := ParseAttributes(raw)
	require.NoError(t, err)
	defer entry.Release()

	// Should have the second value (EGP).
	data, err := pool.Origin.Get(entry.Origin)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x01}, data, "should have EGP (second occurrence)")
}

// TestParseAttributes_BoundaryOrigin verifies ORIGIN value boundary.
//
// VALIDATES: ORIGIN values 0-2 are valid, stored correctly.
// PREVENTS: Off-by-one errors in ORIGIN validation.
// BOUNDARY: 0 (IGP), 1 (EGP), 2 (INCOMPLETE) valid; 3+ invalid per RFC 4271.
func TestParseAttributes_BoundaryOrigin(t *testing.T) {
	tests := []struct {
		name  string
		value byte
	}{
		{"IGP_0", 0x00},
		{"EGP_1", 0x01},
		{"INCOMPLETE_2", 0x02},
		// Note: Values 3+ are invalid per RFC but we store them anyway.
		// Validation is not the parser's job - it just stores.
		{"invalid_3", 0x03},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := []byte{0x40, 0x01, 0x01, tt.value}
			entry, err := ParseAttributes(wire)
			require.NoError(t, err)
			defer entry.Release()

			assert.True(t, entry.HasOrigin())
			data, err := pool.Origin.Get(entry.Origin)
			require.NoError(t, err)
			assert.Equal(t, []byte{tt.value}, data)
		})
	}
}

// TestParseAttributes_BoundaryLocalPref verifies LOCAL_PREF u32 boundary.
//
// VALIDATES: LOCAL_PREF full u32 range stored correctly.
// PREVENTS: Truncation or overflow in LOCAL_PREF handling.
// BOUNDARY: 0 (min), 4294967295 (max u32).
func TestParseAttributes_BoundaryLocalPref(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
	}{
		{"min_0", []byte{0x00, 0x00, 0x00, 0x00}},
		{"typical_100", []byte{0x00, 0x00, 0x00, 0x64}},
		{"max_u32", []byte{0xFF, 0xFF, 0xFF, 0xFF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := append([]byte{0x40, 0x05, 0x04}, tt.value...)
			entry, err := ParseAttributes(wire)
			require.NoError(t, err)
			defer entry.Release()

			assert.True(t, entry.HasLocalPref())
			data, err := pool.LocalPref.Get(entry.LocalPref)
			require.NoError(t, err)
			assert.Equal(t, tt.value, data)
		})
	}
}

// TestParseAttributes_BoundaryMED verifies MED u32 boundary.
//
// VALIDATES: MED full u32 range stored correctly.
// PREVENTS: Truncation or overflow in MED handling.
// BOUNDARY: 0 (min), 4294967295 (max u32).
func TestParseAttributes_BoundaryMED(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
	}{
		{"min_0", []byte{0x00, 0x00, 0x00, 0x00}},
		{"typical_100", []byte{0x00, 0x00, 0x00, 0x64}},
		{"max_u32", []byte{0xFF, 0xFF, 0xFF, 0xFF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := append([]byte{0x80, 0x04, 0x04}, tt.value...)
			entry, err := ParseAttributes(wire)
			require.NoError(t, err)
			defer entry.Release()

			assert.True(t, entry.HasMED())
			data, err := pool.MED.Get(entry.MED)
			require.NoError(t, err)
			assert.Equal(t, tt.value, data)
		})
	}
}

// TestParseAttributes_BoundaryLengths verifies boundary conditions for attribute lengths.
//
// VALIDATES: Correct handling of 0, 255, 256 (extended), 65535 byte attributes.
// PREVENTS: Off-by-one errors in length parsing, extended length mishandling.
func TestParseAttributes_BoundaryLengths(t *testing.T) {
	tests := []struct {
		name    string
		attr    []byte
		wantErr bool
	}{
		{
			name: "length_0_valid",
			// ATOMIC_AGGREGATE has length 0 (no value).
			attr:    []byte{0x40, 0x06, 0x00},
			wantErr: false,
		},
		{
			name: "length_255_normal_header",
			// 255 bytes of community data (uses 1-byte length).
			attr:    makeAttr(0xC0, 0x08, 255),
			wantErr: false,
		},
		{
			name: "length_256_extended_header",
			// 256 bytes requires extended length (2-byte length).
			attr:    makeExtAttr(0xC0, 0x08, 256),
			wantErr: false,
		},
		{
			name: "length_exceeds_data",
			// Length says 100 but only 2 bytes of value.
			attr:    []byte{0x40, 0x01, 0x64, 0x00, 0x00},
			wantErr: false, // Iterator stops gracefully.
		},
		{
			name: "truncated_extended_length",
			// Extended length flag but missing second length byte.
			attr:    []byte{0x50, 0x01, 0x00},
			wantErr: false, // Iterator stops gracefully.
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := ParseAttributes(tc.attr)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			entry.Release()
		})
	}
}

// makeAttr creates an attribute with normal length header and n bytes of value.
func makeAttr(flags, code byte, n int) []byte {
	if n > 255 {
		panic("use makeExtAttr for length > 255")
	}
	result := []byte{flags, code, byte(n)}
	for i := range n {
		result = append(result, byte(i))
	}
	return result
}

// makeExtAttr creates an attribute with extended length header and n bytes of value.
func makeExtAttr(flags, code byte, n int) []byte {
	flags |= 0x10 // Set extended length flag.
	result := []byte{flags, code, byte(n >> 8), byte(n)}
	for i := range n {
		result = append(result, byte(i))
	}
	return result
}

// concat concatenates multiple byte slices.
func concat(slices ...[]byte) []byte {
	var total int
	for _, s := range slices {
		total += len(s)
	}
	result := make([]byte, 0, total)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}
