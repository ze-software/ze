package message

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAttrDiscardFlags verifies the flags computation formula.
//
// VALIDATES: new_flags = 0x80 | (original_flags & 0x50) per draft-mangin-idr-attr-discard-00 §4.2.
// PREVENTS: Wrong flag bits — Optional not set, Partial leaked, Transitive lost.
func TestAttrDiscardFlags(t *testing.T) {
	tests := []struct {
		name     string
		original uint8
		want     uint8
	}{
		{
			name:     "optional_transitive_0xC0",
			original: 0xC0, // Optional + Transitive
			want:     0xC0, // 0x80 | (0xC0 & 0x50) = 0x80 | 0x40 = 0xC0
		},
		{
			name:     "optional_nontransitive_0x80",
			original: 0x80, // Optional only
			want:     0x80, // 0x80 | (0x80 & 0x50) = 0x80 | 0x00 = 0x80
		},
		{
			name:     "wellknown_transitive_0x40",
			original: 0x40, // Well-known transitive (e.g., LOCAL_PREF)
			want:     0xC0, // 0x80 | (0x40 & 0x50) = 0x80 | 0x40 = 0xC0
		},
		{
			name:     "optional_nontransitive_extlen_0x90",
			original: 0x90, // Optional + Extended Length
			want:     0x90, // 0x80 | (0x90 & 0x50) = 0x80 | 0x10 = 0x90
		},
		{
			name:     "optional_transitive_partial_0xE0",
			original: 0xE0, // Optional + Transitive + Partial
			want:     0xC0, // 0x80 | (0xE0 & 0x50) = 0x80 | 0x40 = 0xC0 (Partial cleared)
		},
		{
			name:     "all_bits_0xF0",
			original: 0xF0, // All flag bits set
			want:     0xD0, // 0x80 | (0xF0 & 0x50) = 0x80 | 0x50 = 0xD0
		},
		{
			name:     "no_bits_0x00",
			original: 0x00, // No bits set
			want:     0x80, // 0x80 | (0x00 & 0x50) = 0x80
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attrDiscardFlags(tt.original)
			assert.Equal(t, tt.want, got, "attrDiscardFlags(0x%02X) = 0x%02X, want 0x%02X",
				tt.original, got, tt.want)
		})
	}
}

// makeAttr constructs a single attribute in wire format for testing.
func makeAttr(flags uint8, code uint8, value []byte) []byte {
	if len(value) > 255 {
		buf := make([]byte, 4+len(value))
		buf[0] = flags | 0x10 // Set extended length
		buf[1] = code
		binary.BigEndian.PutUint16(buf[2:4], uint16(len(value)))
		copy(buf[4:], value)
		return buf
	}
	buf := make([]byte, 3+len(value))
	buf[0] = flags
	buf[1] = code
	buf[2] = byte(len(value))
	copy(buf[3:], value)
	return buf
}

// concatBytes concatenates multiple byte slices.
func concatBytes(slices ...[]byte) []byte {
	var total int
	for _, s := range slices {
		total += len(s)
	}
	result := make([]byte, total)
	pos := 0
	for _, s := range slices {
		copy(result[pos:], s)
		pos += len(s)
	}
	return result
}

// TestApplyAttrDiscardInPlace verifies in-place overwrite of a single malformed attribute.
//
// VALIDATES: ATTR_DISCARD replaces malformed attribute in wire bytes without changing length.
// PREVENTS: Incorrect flags computation, missing zeroing of trailing bytes, wrong type code.
func TestApplyAttrDiscardInPlace(t *testing.T) {
	tests := []struct {
		name      string
		pathAttrs []byte
		entries   []DiscardEntry
		wantAttrs []byte
		wantBuilt bool // true if rebuild expected (not in-place)
	}{
		{
			name: "transitive_aggregator_invalid_length",
			// ORIGIN + AS_PATH + NEXT_HOP + AGGREGATOR(wrong length 5, expects 6)
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),                         // ORIGIN=IGP
				makeAttr(0x40, 2, []byte{}),                             // AS_PATH empty
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),                // NEXT_HOP
				makeAttr(0xC0, 7, []byte{0x01, 0x02, 0x03, 0x04, 0x05}), // AGGREGATOR len=5
			),
			entries: []DiscardEntry{{Code: 7, Reason: DiscardReasonInvalidLength}},
			// AGGREGATOR flags 0xC0 → 0x80|(0xC0&0x50) = 0xC0 (unchanged)
			// Type code → attrCodeAttrDiscard (253)
			// Value: [0x07, 0x02, 0x00, 0x00, 0x00] (orig code, reason, zeroed)
			wantAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0xC0, attrCodeAttrDiscard, []byte{0x07, 0x02, 0x00, 0x00, 0x00}),
			),
			wantBuilt: false,
		},
		{
			name: "nontransitive_originator_id_ebgp",
			// ORIGIN + AS_PATH + NEXT_HOP + ORIGINATOR_ID(from EBGP)
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0x80, 9, []byte{0x0A, 0x00, 0x00, 0x01}), // ORIGINATOR_ID
			),
			entries: []DiscardEntry{{Code: 9, Reason: DiscardReasonEBGPInvalid}},
			// Flags 0x80 → 0x80|(0x80&0x50) = 0x80 (unchanged)
			wantAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0x80, attrCodeAttrDiscard, []byte{0x09, 0x01, 0x00, 0x00}),
			),
			wantBuilt: false,
		},
		{
			name: "wellknown_local_pref_ebgp",
			// LOCAL_PREF from EBGP: flags 0x40 (well-known transitive)
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0x40, 5, []byte{0x00, 0x00, 0x00, 0x64}), // LOCAL_PREF=100
			),
			entries: []DiscardEntry{{Code: 5, Reason: DiscardReasonEBGPInvalid}},
			// Flags 0x40 → 0x80|(0x40&0x50) = 0xC0 (Optional bit set)
			wantAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0xC0, attrCodeAttrDiscard, []byte{0x05, 0x01, 0x00, 0x00}),
			),
			wantBuilt: false,
		},
		{
			name: "extended_length_attribute",
			// Attribute with extended length (256 bytes value)
			pathAttrs: func() []byte {
				value := make([]byte, 256)
				for i := range value {
					value[i] = byte(i)
				}
				return makeAttr(0x90, 0x1A, value) // AIGP, optional non-transitive ext-len
			}(),
			entries: []DiscardEntry{{Code: 0x1A, Reason: DiscardReasonMalformedValue}},
			// Flags 0x90 → 0x80|(0x90&0x50) = 0x90 (unchanged)
			wantAttrs: func() []byte {
				value := make([]byte, 256)
				value[0] = 0x1A // Original code
				value[1] = 0x03 // Reason: malformed value
				// Remaining 254 bytes zeroed (default from make)
				return makeAttr(0x90, attrCodeAttrDiscard, value)
			}(),
			wantBuilt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]byte, len(tt.pathAttrs))
			copy(input, tt.pathAttrs)

			result, rebuilt := ApplyAttrDiscard(input, tt.entries)
			assert.Equal(t, tt.wantBuilt, rebuilt, "rebuilt mismatch")
			assert.Equal(t, tt.wantAttrs, result, "attrs mismatch")
		})
	}
}

// TestApplyAttrDiscardValueTooShort verifies rebuild when attribute value < 2 bytes.
//
// VALIDATES: Attributes with value length < 2 trigger rebuild instead of in-place.
// PREVENTS: Buffer overwrite when value is too short for (code, reason) pair.
func TestApplyAttrDiscardValueTooShort(t *testing.T) {
	tests := []struct {
		name string
		attr []byte // Single attribute with short value
	}{
		{
			name: "value_length_1",
			attr: makeAttr(0x40, 6, []byte{0xFF}), // ATOMIC_AGGREGATE len=1
		},
		{
			name: "value_length_0",
			attr: makeAttr(0xC0, 7, []byte{}), // AGGREGATOR len=0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathAttrs := concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),          // ORIGIN
				makeAttr(0x40, 2, []byte{}),              // AS_PATH
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}), // NEXT_HOP
				tt.attr,
			)

			entries := []DiscardEntry{{Code: tt.attr[1], Reason: DiscardReasonInvalidLength}}
			result, rebuilt := ApplyAttrDiscard(pathAttrs, entries)
			assert.True(t, rebuilt, "should trigger rebuild for value < 2")
			// Verify the result contains an ATTR_DISCARD attribute.
			found := ExtractUpstreamAttrDiscard(result)
			require.Len(t, found, 1)
			assert.Equal(t, tt.attr[1], found[0].Code)
			assert.Equal(t, DiscardReasonInvalidLength, found[0].Reason)
		})
	}
}

// TestApplyAttrDiscardMultipleEntries verifies rebuild for multiple discards.
//
// VALIDATES: Multiple discarded attributes produce a single ATTR_DISCARD with all pairs.
// PREVENTS: Multiple ATTR_DISCARD instances violating RFC 4271 Section 5.
func TestApplyAttrDiscardMultipleEntries(t *testing.T) {
	pathAttrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),                                           // ORIGIN
		makeAttr(0x40, 2, []byte{}),                                               // AS_PATH
		makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),                                  // NEXT_HOP
		makeAttr(0x40, 6, []byte{0x01, 0x02}),                                     // ATOMIC_AGG len=2 (wrong, should be 0)
		makeAttr(0xC0, 7, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}), // AGGREGATOR len=8 (wrong for asn2)
	)

	entries := []DiscardEntry{
		{Code: 6, Reason: DiscardReasonInvalidLength},
		{Code: 7, Reason: DiscardReasonInvalidLength},
	}

	result, rebuilt := ApplyAttrDiscard(pathAttrs, entries)
	require.True(t, rebuilt, "multiple entries must trigger rebuild")

	// Verify single ATTR_DISCARD with 2 pairs.
	found := ExtractUpstreamAttrDiscard(result)
	require.Len(t, found, 2)
	assert.Equal(t, uint8(6), found[0].Code)
	assert.Equal(t, DiscardReasonInvalidLength, found[0].Reason)
	assert.Equal(t, uint8(7), found[1].Code)
	assert.Equal(t, DiscardReasonInvalidLength, found[1].Reason)

	// Verify removed attributes are not in the result.
	assert.Nil(t, findAttrByCode(result, 6), "ATOMIC_AGG should be removed")
	assert.Nil(t, findAttrByCode(result, 7), "AGGREGATOR should be removed")

	// Verify kept attributes are preserved.
	assert.NotNil(t, findAttrByCode(result, 1), "ORIGIN should be kept")
	assert.NotNil(t, findAttrByCode(result, 2), "AS_PATH should be kept")
	assert.NotNil(t, findAttrByCode(result, 3), "NEXT_HOP should be kept")
}

// TestApplyAttrDiscardMergeUpstream verifies merging with upstream ATTR_DISCARD.
//
// VALIDATES: Upstream + local discards produce a single merged ATTR_DISCARD.
// PREVENTS: Duplicate ATTR_DISCARD violating RFC 4271 Section 5.
func TestApplyAttrDiscardMergeUpstream(t *testing.T) {
	// Path attrs with existing upstream ATTR_DISCARD (code 26/AIGP, reason 3/malformed).
	upstreamDiscard := makeAttr(0x80, attrCodeAttrDiscard, []byte{0x1A, 0x03})

	pathAttrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),                   // ORIGIN
		makeAttr(0x40, 2, []byte{}),                       // AS_PATH
		makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),          // NEXT_HOP
		upstreamDiscard,                                   // Upstream ATTR_DISCARD
		makeAttr(0x80, 9, []byte{0x0A, 0x00, 0x00, 0x01}), // ORIGINATOR_ID (to discard)
	)

	entries := []DiscardEntry{{Code: 9, Reason: DiscardReasonEBGPInvalid}}

	result, rebuilt := ApplyAttrDiscard(pathAttrs, entries)
	require.True(t, rebuilt, "upstream merge must trigger rebuild")

	// Verify single ATTR_DISCARD with upstream + local pairs.
	found := ExtractUpstreamAttrDiscard(result)
	require.Len(t, found, 2, "should have upstream + local entries")
	// Upstream pair first.
	assert.Equal(t, uint8(0x1A), found[0].Code, "upstream AIGP code")
	assert.Equal(t, DiscardReasonMalformedValue, found[0].Reason, "upstream AIGP reason")
	// Local pair second.
	assert.Equal(t, uint8(9), found[1].Code, "local ORIGINATOR_ID code")
	assert.Equal(t, DiscardReasonEBGPInvalid, found[1].Reason, "local ORIGINATOR_ID reason")

	// Verify old upstream ATTR_DISCARD is removed (replaced by merged one).
	discardCount := countAttrByCode(result, attrCodeAttrDiscard)
	assert.Equal(t, 1, discardCount, "should have exactly one ATTR_DISCARD")
}

// TestApplyAttrDiscardEmptyEntries verifies no-op for empty entries.
//
// VALIDATES: Empty discard entries returns path attrs unchanged.
// PREVENTS: Unnecessary buffer allocation or modification.
func TestApplyAttrDiscardEmptyEntries(t *testing.T) {
	pathAttrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),
		makeAttr(0x40, 2, []byte{}),
	)

	result, rebuilt := ApplyAttrDiscard(pathAttrs, nil)
	assert.False(t, rebuilt)
	assert.Equal(t, pathAttrs, result)
}

// TestRebuildUpdateBody verifies UPDATE body reconstruction.
//
// VALIDATES: New path attrs correctly integrated into UPDATE body layout.
// PREVENTS: Corrupted withdrawn/NLRI sections after rebuild.
func TestRebuildUpdateBody(t *testing.T) {
	// Build an UPDATE body: withdrawn(0) + pathattrs + nlri
	origAttrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),
		makeAttr(0x40, 2, []byte{}),
		makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
		makeAttr(0x80, 9, []byte{0x0A, 0x00, 0x00, 0x01}), // to be discarded
	)
	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24

	body := make([]byte, 2+0+2+len(origAttrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], 0) // withdrawn length = 0
	binary.BigEndian.PutUint16(body[2:4], uint16(len(origAttrs)))
	copy(body[4:], origAttrs)
	copy(body[4+len(origAttrs):], nlri)

	// Build new attrs (without ORIGINATOR_ID, with ATTR_DISCARD)
	newAttrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),
		makeAttr(0x40, 2, []byte{}),
		makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
		makeAttr(0x80, attrCodeAttrDiscard, []byte{0x09, 0x01}),
	)

	result := RebuildUpdateBody(body, newAttrs)

	// Verify structure.
	require.True(t, len(result) >= 4)
	wdLen := int(binary.BigEndian.Uint16(result[0:2]))
	assert.Equal(t, 0, wdLen, "withdrawn length preserved")

	attrLen := int(binary.BigEndian.Uint16(result[2:4]))
	assert.Equal(t, len(newAttrs), attrLen, "new attr length")

	// Verify attrs section.
	resultAttrs := result[4 : 4+attrLen]
	assert.Equal(t, newAttrs, resultAttrs, "attrs match")

	// Verify NLRI preserved.
	resultNLRI := result[4+attrLen:]
	assert.Equal(t, nlri, resultNLRI, "NLRI preserved")
}

// TestRFC7606DiscardEntryReasonCodes verifies reason codes from validation.
//
// VALIDATES: RFC 7606 validation populates DiscardEntries with correct reason codes.
// PREVENTS: Wrong reason code in ATTR_DISCARD marker.
func TestRFC7606DiscardEntryReasonCodes(t *testing.T) {
	tests := []struct {
		name       string
		pathAttrs  []byte
		isIBGP     bool
		wantCode   uint8
		wantReason uint8
	}{
		{
			name: "local_pref_ebgp_reason_1",
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0x40, 5, []byte{0x00, 0x00, 0x00, 0x64}), // LOCAL_PREF from EBGP
			),
			isIBGP:     false,
			wantCode:   5,
			wantReason: DiscardReasonEBGPInvalid,
		},
		{
			name: "atomic_agg_wrong_length_reason_2",
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0x40, 6, []byte{0x01, 0x02}), // ATOMIC_AGG len=2 (should be 0)
			),
			isIBGP:     true,
			wantCode:   6,
			wantReason: DiscardReasonInvalidLength,
		},
		{
			name: "aggregator_wrong_length_reason_2",
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				// AGGREGATOR len=8 but asn4=false expects 6
				makeAttr(0xC0, 7, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}),
			),
			isIBGP:     true,
			wantCode:   7,
			wantReason: DiscardReasonInvalidLength,
		},
		{
			name: "originator_id_ebgp_reason_1",
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0x80, 9, []byte{0x0A, 0x00, 0x00, 0x01}), // ORIGINATOR_ID from EBGP
			),
			isIBGP:     false,
			wantCode:   9,
			wantReason: DiscardReasonEBGPInvalid,
		},
		{
			name: "cluster_list_ebgp_reason_1",
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0x80, 10, []byte{0x0A, 0x00, 0x00, 0x01}), // CLUSTER_LIST from EBGP
			),
			isIBGP:     false,
			wantCode:   10,
			wantReason: DiscardReasonEBGPInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateUpdateRFC7606(tt.pathAttrs, true, tt.isIBGP, false)
			require.Equal(t, RFC7606ActionAttributeDiscard, result.Action)
			require.Len(t, result.DiscardEntries, 1)
			assert.Equal(t, tt.wantCode, result.DiscardEntries[0].Code)
			assert.Equal(t, tt.wantReason, result.DiscardEntries[0].Reason)
		})
	}
}

// TestApplyAttrDiscardTransitivity verifies transitivity rules in rebuild mode.
//
// VALIDATES: draft-mangin-idr-attr-discard-00 Section 5.10:
// ALL transitive → 0xC0, ALL non-transitive → 0x80, mixed → SHOULD 0x80.
// PREVENTS: Mixed transitivity propagating across AS boundaries.
func TestApplyAttrDiscardTransitivity(t *testing.T) {
	tests := []struct {
		name      string
		pathAttrs []byte
		entries   []DiscardEntry
		wantFlags uint8
	}{
		{
			name: "mixed_transitive_and_nontransitive",
			// AGGREGATOR (0xC0 transitive) + ORIGINATOR_ID (0x80 non-transitive)
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0xC0, 7, []byte{0x01, 0x02, 0x03}),       // AGGREGATOR (transitive)
				makeAttr(0x80, 9, []byte{0x0A, 0x00, 0x00, 0x01}), // ORIGINATOR_ID (non-transitive)
			),
			entries: []DiscardEntry{
				{Code: 7, Reason: DiscardReasonInvalidLength},
				{Code: 9, Reason: DiscardReasonEBGPInvalid},
			},
			wantFlags: 0x80, // Mixed → conservative non-transitive.
		},
		{
			name: "all_transitive",
			// AGGREGATOR (0xC0) + COMMUNITY (0xC0) — both transitive
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0xC0, 7, []byte{0x01, 0x02, 0x03}),             // AGGREGATOR (transitive)
				makeAttr(0xC0, 8, []byte{0xFF, 0xFF, 0xFF, 0x01, 0x02}), // COMMUNITY (transitive)
			),
			entries: []DiscardEntry{
				{Code: 7, Reason: DiscardReasonInvalidLength},
				{Code: 8, Reason: DiscardReasonMalformedValue},
			},
			wantFlags: 0xC0, // All transitive → MUST 0xC0.
		},
		{
			name: "all_nontransitive",
			// ORIGINATOR_ID (0x80) + CLUSTER_LIST (0x80) — both non-transitive
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0x80, 9, []byte{0x0A, 0x00, 0x00, 0x01}),  // ORIGINATOR_ID (non-transitive)
				makeAttr(0x80, 10, []byte{0x0A, 0x00, 0x00, 0x02}), // CLUSTER_LIST (non-transitive)
			),
			entries: []DiscardEntry{
				{Code: 9, Reason: DiscardReasonEBGPInvalid},
				{Code: 10, Reason: DiscardReasonEBGPInvalid},
			},
			wantFlags: 0x80, // All non-transitive → MUST 0x80.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, rebuilt := ApplyAttrDiscard(tt.pathAttrs, tt.entries)
			require.True(t, rebuilt)

			// Find the ATTR_DISCARD and check its flags.
			pos := 0
			for pos < len(result) {
				if pos+2 > len(result) {
					break
				}
				flags := result[pos]
				code := result[pos+1]
				if code == attrCodeAttrDiscard {
					assert.Equal(t, tt.wantFlags, flags,
						"ATTR_DISCARD flags 0x%02X, want 0x%02X", flags, tt.wantFlags)
					return
				}
				pos += 2
				if flags&0x10 != 0 {
					if pos+2 > len(result) {
						break
					}
					vl := int(binary.BigEndian.Uint16(result[pos : pos+2]))
					pos += 2 + vl
				} else {
					if pos >= len(result) {
						break
					}
					vl := int(result[pos])
					pos += 1 + vl
				}
			}
			t.Fatal("ATTR_DISCARD not found in result")
		})
	}
}

// TestApplyAttrDiscardNotFound verifies fallback when target attribute is absent.
//
// VALIDATES: When the attribute code to discard is not in pathAttrs, in-place fails
// and ApplyAttrDiscard falls through to rebuild, producing a valid ATTR_DISCARD.
// PREVENTS: Silent no-op when discarding an attribute that doesn't exist in the wire.
func TestApplyAttrDiscardNotFound(t *testing.T) {
	pathAttrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),          // ORIGIN
		makeAttr(0x40, 2, []byte{}),              // AS_PATH
		makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}), // NEXT_HOP
	)

	// Discard code 9 (ORIGINATOR_ID) which is NOT present.
	entries := []DiscardEntry{{Code: 9, Reason: DiscardReasonEBGPInvalid}}

	result, rebuilt := ApplyAttrDiscard(pathAttrs, entries)
	require.True(t, rebuilt, "attribute-not-found should trigger rebuild")

	// Verify the ATTR_DISCARD was created with the entry.
	found := ExtractUpstreamAttrDiscard(result)
	require.Len(t, found, 1)
	assert.Equal(t, uint8(9), found[0].Code)
	assert.Equal(t, DiscardReasonEBGPInvalid, found[0].Reason)

	// Verify the original kept attributes are still present.
	assert.NotNil(t, findAttrByCode(result, 1), "ORIGIN should be kept")
	assert.NotNil(t, findAttrByCode(result, 2), "AS_PATH should be kept")
	assert.NotNil(t, findAttrByCode(result, 3), "NEXT_HOP should be kept")
}

// TestApplyAttrDiscardUpstreamTransitivityMerge verifies upstream transitivity merge rules.
//
// VALIDATES: draft-mangin-idr-attr-discard-00 Section 5.10 applied during upstream merge.
// Upstream transitive + local non-transitive = mixed → SHOULD 0x80.
// Upstream transitive + local transitive = all transitive → MUST 0xC0.
// PREVENTS: Unconditional transitivity inheritance from upstream ignoring local entries.
func TestApplyAttrDiscardUpstreamTransitivityMerge(t *testing.T) {
	tests := []struct {
		name      string
		pathAttrs []byte
		entries   []DiscardEntry
		wantFlags uint8
	}{
		{
			name: "upstream_transitive_local_nontransitive_mixed",
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0xC0, attrCodeAttrDiscard, []byte{0x1A, 0x03}), // Upstream transitive
				makeAttr(0x80, 9, []byte{0x0A, 0x00, 0x00, 0x01}),       // ORIGINATOR_ID (non-transitive)
			),
			entries:   []DiscardEntry{{Code: 9, Reason: DiscardReasonEBGPInvalid}},
			wantFlags: 0x80, // Mixed → conservative non-transitive.
		},
		{
			name: "upstream_transitive_local_transitive_all",
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0xC0, attrCodeAttrDiscard, []byte{0x1A, 0x03}), // Upstream transitive
				makeAttr(0xC0, 7, []byte{0x01, 0x02, 0x03}),             // AGGREGATOR (transitive)
			),
			entries:   []DiscardEntry{{Code: 7, Reason: DiscardReasonInvalidLength}},
			wantFlags: 0xC0, // All transitive → MUST 0xC0.
		},
		{
			name: "upstream_nontransitive_local_transitive_mixed",
			pathAttrs: concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),
				makeAttr(0x40, 2, []byte{}),
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),
				makeAttr(0x80, attrCodeAttrDiscard, []byte{0x1A, 0x03}), // Upstream non-transitive
				makeAttr(0xC0, 7, []byte{0x01, 0x02, 0x03}),             // AGGREGATOR (transitive)
			),
			entries:   []DiscardEntry{{Code: 7, Reason: DiscardReasonInvalidLength}},
			wantFlags: 0x80, // Mixed → conservative non-transitive.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, rebuilt := ApplyAttrDiscard(tt.pathAttrs, tt.entries)
			require.True(t, rebuilt)

			pos := 0
			for pos < len(result) {
				if pos+2 > len(result) {
					break
				}
				flags := result[pos]
				code := result[pos+1]
				if code == attrCodeAttrDiscard {
					assert.Equal(t, tt.wantFlags, flags,
						"ATTR_DISCARD flags 0x%02X, want 0x%02X", flags, tt.wantFlags)
					return
				}
				pos += 2
				if flags&0x10 != 0 {
					if pos+2 > len(result) {
						break
					}
					vl := int(binary.BigEndian.Uint16(result[pos : pos+2]))
					pos += 2 + vl
				} else {
					if pos >= len(result) {
						break
					}
					vl := int(result[pos])
					pos += 1 + vl
				}
			}
			t.Fatal("ATTR_DISCARD not found in result")
		})
	}
}

// findAttrByCode returns the raw attribute bytes for a given code, or nil if not found.
func findAttrByCode(pathAttrs []byte, code uint8) []byte {
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			return nil
		}
		flags := pathAttrs[pos]
		attrCode := pathAttrs[pos+1]
		attrStart := pos
		pos += 2

		var valueLen int
		var hdrLen int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				return nil
			}
			valueLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			hdrLen = 4
			pos += 2
		} else {
			if pos >= len(pathAttrs) {
				return nil
			}
			valueLen = int(pathAttrs[pos])
			hdrLen = 3
			pos++
		}

		if attrCode == code {
			return pathAttrs[attrStart : attrStart+hdrLen+valueLen]
		}

		pos += valueLen
	}
	return nil
}

// countAttrByCode counts how many attributes have the given code.
func countAttrByCode(pathAttrs []byte, code uint8) int {
	count := 0
	pos := 0
	for pos < len(pathAttrs) {
		if pos+2 > len(pathAttrs) {
			return count
		}
		flags := pathAttrs[pos]
		attrCode := pathAttrs[pos+1]
		pos += 2

		var valueLen int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				return count
			}
			valueLen = int(binary.BigEndian.Uint16(pathAttrs[pos : pos+2]))
			pos += 2
		} else {
			if pos >= len(pathAttrs) {
				return count
			}
			valueLen = int(pathAttrs[pos])
			pos++
		}

		if attrCode == code {
			count++
		}

		pos += valueLen
	}
	return count
}
