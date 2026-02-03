package wire

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateSectionsParse verifies ParseUpdateSections extracts correct offsets.
//
// VALIDATES: ParseUpdateSections correctly parses UPDATE body into section offsets.
// PREVENTS: Off-by-one errors in offset calculations.
func TestUpdateSectionsParse(t *testing.T) {
	// Build UPDATE payload:
	// WithdrawnLen(2) + Withdrawn(variable) + AttrLen(2) + Attrs(variable) + NLRI(variable)
	//
	// Test case: 2 bytes withdrawn, 4 bytes attrs, 4 bytes NLRI
	withdrawn := []byte{0x10, 0x0a} // /16 prefix
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	nlri := []byte{0x18, 0xc0, 0xa8, 0x01} // /24 prefix

	payload := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn))) //nolint:gosec // G115: test data
	copy(payload[2:], withdrawn)
	offset := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(payload[offset:], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[offset+2:], attrs)
	copy(payload[offset+2+len(attrs):], nlri)

	sections, err := ParseUpdateSections(payload)
	require.NoError(t, err)

	// Verify offsets
	assert.True(t, sections.Valid())
	assert.Equal(t, len(withdrawn), sections.WithdrawnLen())
	assert.Equal(t, len(attrs), sections.AttrsLen())

	// Verify slice accessors return correct data
	gotWithdrawn := sections.Withdrawn(payload)
	assert.Equal(t, withdrawn, gotWithdrawn)

	gotAttrs := sections.Attrs(payload)
	assert.Equal(t, attrs, gotAttrs)

	gotNLRI := sections.NLRI(payload)
	assert.Equal(t, nlri, gotNLRI)
}

// TestUpdateSectionsEmpty verifies handling of empty sections.
//
// VALIDATES: Empty withdrawn/attrs/NLRI return nil slices.
// PREVENTS: False errors on valid empty UPDATE (End-of-RIB).
func TestUpdateSectionsEmpty(t *testing.T) {
	// Minimal UPDATE: WithdrawnLen=0, AttrLen=0, no NLRI
	payload := []byte{0x00, 0x00, 0x00, 0x00}

	sections, err := ParseUpdateSections(payload)
	require.NoError(t, err)
	assert.True(t, sections.Valid())

	wd := sections.Withdrawn(payload)
	assert.Nil(t, wd)

	attrs := sections.Attrs(payload)
	assert.Nil(t, attrs)

	nlri := sections.NLRI(payload)
	assert.Nil(t, nlri)
}

// TestUpdateSectionsTruncated verifies error handling for truncated payloads.
//
// VALIDATES: Truncated payloads return error, not garbage data.
// PREVENTS: Buffer overread from malformed UPDATE.
func TestUpdateSectionsTruncated(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"too_short_1", []byte{0x00}},
		{"too_short_3", []byte{0x00, 0x00, 0x00}},
		{"withdrawn_truncated", []byte{0x00, 0x05, 0x01}},         // claims 5 bytes, only 1
		{"attrs_truncated", []byte{0x00, 0x00, 0x00, 0x10, 0x40}}, // claims 16 bytes attrs, only 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseUpdateSections(tt.payload)
			assert.Error(t, err)
		})
	}
}

// TestUpdateSectionsZeroCopy verifies slice accessors return views into original buffer.
//
// VALIDATES: Accessors return slices sharing underlying array with input.
// PREVENTS: Unintended copies that break zero-copy semantics.
func TestUpdateSectionsZeroCopy(t *testing.T) {
	// Build payload with all sections
	withdrawn := []byte{0x10, 0x0a}
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	nlri := []byte{0x18, 0xc0, 0xa8, 0x01}

	payload := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn))) //nolint:gosec // G115: test data
	copy(payload[2:], withdrawn)
	offset := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(payload[offset:], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[offset+2:], attrs)
	copy(payload[offset+2+len(attrs):], nlri)

	sections, err := ParseUpdateSections(payload)
	require.NoError(t, err)

	// Get slices
	gotWithdrawn := sections.Withdrawn(payload)
	gotAttrs := sections.Attrs(payload)
	gotNLRI := sections.NLRI(payload)

	// Verify zero-copy by checking that modifying returned slice modifies original.
	// This proves they share the same underlying array.
	if len(gotWithdrawn) > 0 {
		original := payload[2]
		gotWithdrawn[0] = 0xEE
		assert.Equal(t, byte(0xEE), payload[2], "Withdrawn should share memory with payload")
		payload[2] = original // restore
	}

	if len(gotAttrs) > 0 {
		attrOffset := 2 + len(withdrawn) + 2
		original := payload[attrOffset]
		gotAttrs[0] = 0xEE
		assert.Equal(t, byte(0xEE), payload[attrOffset], "Attrs should share memory with payload")
		payload[attrOffset] = original // restore
	}

	if len(gotNLRI) > 0 {
		nlriOffset := 2 + len(withdrawn) + 2 + len(attrs)
		original := payload[nlriOffset]
		gotNLRI[0] = 0xEE
		assert.Equal(t, byte(0xEE), payload[nlriOffset], "NLRI should share memory with payload")
		payload[nlriOffset] = original // restore
	}
}

// TestUpdateSectionsExtendedMessage verifies handling of large payloads (RFC 8654).
//
// VALIDATES: Extended Message sizes (up to 65535 - 19 = 65516 body) are supported.
// PREVENTS: Overflow when handling Extended Message UPDATE.
func TestUpdateSectionsExtendedMessage(t *testing.T) {
	// RFC 8654: Extended message max = 65535 total, minus 19 byte header = 65516 body
	// Build a large payload (but reasonable for test)
	largeAttrs := make([]byte, 10000)
	for i := range largeAttrs {
		largeAttrs[i] = byte(i % 256)
	}

	payload := make([]byte, 2+0+2+len(largeAttrs)+0)
	binary.BigEndian.PutUint16(payload[0:2], 0)                       // no withdrawn
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(largeAttrs))) //nolint:gosec // G115: test data
	copy(payload[4:], largeAttrs)

	sections, err := ParseUpdateSections(payload)
	require.NoError(t, err)

	assert.Equal(t, 0, sections.WithdrawnLen())
	assert.Equal(t, 10000, sections.AttrsLen())

	gotAttrs := sections.Attrs(payload)
	assert.Len(t, gotAttrs, 10000)
}

// TestUpdateSectionsExtendedMessageMaximum verifies handling at RFC 8654 maximum.
//
// VALIDATES: Maximum Extended Message body (65516 bytes) is handled correctly.
// PREVENTS: Integer overflow at maximum sizes with int type.
func TestUpdateSectionsExtendedMessageMaximum(t *testing.T) {
	// RFC 8654: max body = 65535 - 19 = 65516 bytes
	// Body format: wdLen(2) + withdrawn + attrLen(2) + attrs + nlri
	// Test with maximum attrs section: 65516 - 4 = 65512 bytes for attrs
	const maxBodySize = 65516
	const headerOverhead = 4 // 2 bytes wdLen + 2 bytes attrLen
	const maxAttrsSize = maxBodySize - headerOverhead

	largeAttrs := make([]byte, maxAttrsSize)
	// Fill with pattern to verify correctness
	for i := range largeAttrs {
		largeAttrs[i] = byte(i % 256)
	}

	payload := make([]byte, maxBodySize)
	binary.BigEndian.PutUint16(payload[0:2], 0)            // no withdrawn
	binary.BigEndian.PutUint16(payload[2:4], maxAttrsSize) // max attrs
	copy(payload[4:], largeAttrs)

	sections, err := ParseUpdateSections(payload)
	require.NoError(t, err)

	// Verify offsets are calculated correctly at maximum sizes
	assert.Equal(t, 0, sections.WithdrawnLen())
	assert.Equal(t, maxAttrsSize, sections.AttrsLen())
	assert.Equal(t, 0, sections.NLRILen(payload)) // no NLRI (attrs fill entire body)

	// Verify accessors return correct slices
	assert.Nil(t, sections.Withdrawn(payload))
	gotAttrs := sections.Attrs(payload)
	assert.Len(t, gotAttrs, maxAttrsSize)
	assert.Nil(t, sections.NLRI(payload))

	// Verify first and last bytes to confirm zero-copy
	assert.Equal(t, byte(0), gotAttrs[0])
	assert.Equal(t, byte((maxAttrsSize-1)%256), gotAttrs[maxAttrsSize-1])
}

// TestUpdateSectionsExtendedMessageAllSections tests max body split across sections.
//
// VALIDATES: Maximum body with all sections populated works correctly.
// PREVENTS: Offset calculation errors when all sections have data.
func TestUpdateSectionsExtendedMessageAllSections(t *testing.T) {
	// Split max body across all three sections
	const maxBodySize = 65516
	const wdSize = 20000   // ~30% for withdrawn
	const attrSize = 30000 // ~46% for attrs
	// nlriSize = 65516 - 4 - 20000 - 30000 = 15512 (remaining)
	const nlriSize = maxBodySize - 4 - wdSize - attrSize

	withdrawn := make([]byte, wdSize)
	attrs := make([]byte, attrSize)
	nlri := make([]byte, nlriSize)

	// Fill with distinct patterns
	for i := range withdrawn {
		withdrawn[i] = 0xAA
	}
	for i := range attrs {
		attrs[i] = 0xBB
	}
	for i := range nlri {
		nlri[i] = 0xCC
	}

	payload := make([]byte, maxBodySize)
	binary.BigEndian.PutUint16(payload[0:2], wdSize)
	copy(payload[2:], withdrawn)
	offset := 2 + wdSize
	binary.BigEndian.PutUint16(payload[offset:], attrSize)
	copy(payload[offset+2:], attrs)
	copy(payload[offset+2+attrSize:], nlri)

	sections, err := ParseUpdateSections(payload)
	require.NoError(t, err)

	// Verify all lengths
	assert.Equal(t, wdSize, sections.WithdrawnLen())
	assert.Equal(t, attrSize, sections.AttrsLen())
	assert.Equal(t, nlriSize, sections.NLRILen(payload))

	// Verify accessors return correct data
	gotWd := sections.Withdrawn(payload)
	gotAttrs := sections.Attrs(payload)
	gotNLRI := sections.NLRI(payload)

	assert.Len(t, gotWd, wdSize)
	assert.Len(t, gotAttrs, attrSize)
	assert.Len(t, gotNLRI, nlriSize)

	// Verify patterns (confirms correct offsets)
	assert.Equal(t, byte(0xAA), gotWd[0])
	assert.Equal(t, byte(0xBB), gotAttrs[0])
	assert.Equal(t, byte(0xCC), gotNLRI[0])
}

// TestUpdateSectionsBoundary verifies boundary conditions.
//
// VALIDATES: Edge cases at section boundaries are handled correctly.
// PREVENTS: Off-by-one errors at boundaries.
func TestUpdateSectionsBoundary(t *testing.T) {
	tests := []struct {
		name      string
		payload   []byte
		wantErr   bool
		wantWdLen int
		wantAtLen int
	}{
		{
			name:      "minimum_valid",
			payload:   []byte{0x00, 0x00, 0x00, 0x00},
			wantErr:   false,
			wantWdLen: 0,
			wantAtLen: 0,
		},
		{
			name:      "withdrawn_exactly_fills",
			payload:   []byte{0x00, 0x02, 0xAA, 0xBB, 0x00, 0x00},
			wantErr:   false,
			wantWdLen: 2,
			wantAtLen: 0,
		},
		{
			name:    "withdrawn_one_short",
			payload: []byte{0x00, 0x03, 0xAA, 0xBB}, // claims 3, has 2
			wantErr: true,
		},
		{
			name:      "attrs_exactly_fills",
			payload:   []byte{0x00, 0x00, 0x00, 0x02, 0xAA, 0xBB},
			wantErr:   false,
			wantWdLen: 0,
			wantAtLen: 2,
		},
		{
			name:    "attrs_one_short",
			payload: []byte{0x00, 0x00, 0x00, 0x03, 0xAA, 0xBB}, // claims 3, has 2
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections, err := ParseUpdateSections(tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantWdLen, sections.WithdrawnLen())
			assert.Equal(t, tt.wantAtLen, sections.AttrsLen())
		})
	}
}

// TestUpdateSectionsWithNLRI verifies NLRI extraction when present.
//
// VALIDATES: NLRI bytes after attrs section are correctly identified.
// PREVENTS: Missing NLRI when attrs section is empty but NLRI follows.
func TestUpdateSectionsWithNLRI(t *testing.T) {
	// wdLen=0, attrLen=0, then NLRI bytes
	nlriBytes := []byte{0x18, 0x0A, 0x00, 0x00} // /24 prefix 10.0.0.x
	payload := append([]byte{0x00, 0x00, 0x00, 0x00}, nlriBytes...)

	sections, err := ParseUpdateSections(payload)
	require.NoError(t, err)

	gotNLRI := sections.NLRI(payload)
	assert.Equal(t, nlriBytes, gotNLRI)
}

// TestUpdateSectionsNLRILength verifies NLRILen calculation.
//
// VALIDATES: NLRILen() returns correct length of trailing NLRI.
// PREVENTS: Wrong NLRI length affecting iteration.
func TestUpdateSectionsNLRILength(t *testing.T) {
	withdrawn := []byte{0x10, 0x0a}
	attrs := []byte{0x40, 0x01, 0x01, 0x00}
	nlri := []byte{0x18, 0xc0, 0xa8, 0x01, 0x08, 0x0a} // two prefixes

	payload := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn))) //nolint:gosec // G115: test data
	copy(payload[2:], withdrawn)
	offset := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(payload[offset:], uint16(len(attrs))) //nolint:gosec // G115: test data
	copy(payload[offset+2:], attrs)
	copy(payload[offset+2+len(attrs):], nlri)

	sections, err := ParseUpdateSections(payload)
	require.NoError(t, err)

	assert.Equal(t, len(nlri), sections.NLRILen(payload))
}

// TestUpdateSectionsNotParsed verifies zero-value sections indicate not parsed.
//
// VALIDATES: Zero-value UpdateSections is distinguishable from parsed empty.
// PREVENTS: Confusion between "not parsed" and "parsed with all empty sections".
func TestUpdateSectionsNotParsed(t *testing.T) {
	var sections UpdateSections

	// Zero-value should not be valid
	assert.False(t, sections.Valid())
}

// TestUpdateSectionsAccessorsOnZeroValue verifies accessors return nil on zero-value.
//
// VALIDATES: Accessors safely handle zero-value UpdateSections.
// PREVENTS: Panic or wrong data when accessors called on uninitialized struct.
func TestUpdateSectionsAccessorsOnZeroValue(t *testing.T) {
	var sections UpdateSections
	data := []byte{0x00, 0x00, 0x00, 0x00, 0xAA, 0xBB, 0xCC, 0xDD}

	// All accessors should return nil on zero-value (not valid)
	assert.Nil(t, sections.Withdrawn(data), "Withdrawn should return nil on zero-value")
	assert.Nil(t, sections.Attrs(data), "Attrs should return nil on zero-value")
	assert.Nil(t, sections.NLRI(data), "NLRI should return nil on zero-value")
	assert.Equal(t, 0, sections.NLRILen(data), "NLRILen should return 0 on zero-value")
}
