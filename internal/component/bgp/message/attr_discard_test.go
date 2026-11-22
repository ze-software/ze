// Design: docs/architecture/wire/messages.md -- ATTR_TOMBSTONE upstream merge edge cases
// RFC: rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt -- Section 5.1 rebuild on upstream marker

package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestApplyAttrDiscardMergesZeroEntryUpstreamTombstone verifies that an upstream
// ATTR_TOMBSTONE (code 252) with zero valid (code, reason) entries still forces
// the rebuild path instead of an in-place overwrite.
//
// VALIDATES: Empty (0-byte), 1-byte, and all-zero-padded upstream ATTR_TOMBSTONE
// markers combined with a local discard (value length >= 2) are replaced by a
// single rebuilt ATTR_TOMBSTONE that passes SpanIndex and AttributesWire indexing.
// PREVENTS: Duplicate ATTR_TOMBSTONE (code 252) poisoning the UPDATE when
// ExtractUpstreamAttrDiscard returns zero entries and applyInPlace stamps a
// second code 252 alongside the upstream one.
func TestApplyAttrDiscardMergesZeroEntryUpstreamTombstone(t *testing.T) {
	tests := []struct {
		name          string
		upstreamFlags uint8
		upstreamValue []byte
		wantFlags     uint8
	}{
		{
			name:          "empty 0-byte upstream tombstone (non-transitive)",
			upstreamFlags: 0x80,
			upstreamValue: []byte{},
			wantFlags:     0x80,
		},
		{
			name:          "empty 0-byte upstream tombstone (transitive)",
			upstreamFlags: 0xC0,
			upstreamValue: []byte{},
			wantFlags:     0xC0,
		},
		{
			name:          "1-byte upstream tombstone",
			upstreamFlags: 0x80,
			upstreamValue: []byte{0x07},
			wantFlags:     0x80,
		},
		{
			name:          "all-zero-padded upstream tombstone",
			upstreamFlags: 0xC0,
			upstreamValue: []byte{0x00, 0x00, 0x00, 0x00},
			wantFlags:     0xC0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathAttrs := concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),                         // ORIGIN
				makeAttr(0x40, 2, []byte{}),                             // AS_PATH
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}),                // NEXT_HOP
				makeAttr(tt.upstreamFlags, uint8(attribute.AttrTombstone), tt.upstreamValue),
				makeAttr(0xC0, 7, []byte{0x01, 0x02, 0x03, 0x04, 0x05}), // malformed AGGREGATOR (len=5 >= 2)
			)

			// Confirm the upstream marker yields zero parsed entries.
			require.Empty(t, ExtractUpstreamAttrDiscard(pathAttrs))

			result, rebuilt := ApplyAttrDiscard(pathAttrs, []DiscardEntry{
				{Code: 7, Reason: DiscardReasonInvalidLength},
			})
			require.True(t, rebuilt, "presence of upstream ATTR_TOMBSTONE must force rebuild")

			assert.Equal(t, 1, countAttrByCode(result, uint8(attribute.AttrTombstone)),
				"output must contain exactly one ATTR_TOMBSTONE")
			assert.Equal(t, 0, countAttrByCode(result, 7),
				"locally discarded attribute must be removed")

			marker := findAttrByCode(result, uint8(attribute.AttrTombstone))
			require.NotNil(t, marker)
			assert.Equal(t, tt.wantFlags, marker[0])

			entries := ExtractUpstreamAttrDiscard(result)
			require.Len(t, entries, 1)
			assert.Equal(t, uint8(7), entries[0].Code)
			assert.Equal(t, DiscardReasonInvalidLength, entries[0].Reason)

			// Both SpanIndex and AttributesWire reject duplicate attribute codes.
			idx, err := attribute.BuildSpanIndex(result)
			require.NoError(t, err, "BuildSpanIndex must accept the rebuilt attributes")
			assert.True(t, idx.Has(attribute.AttrTombstone))
			assert.False(t, idx.Has(attribute.AttrAggregator))

			aw := attribute.NewAttributesWire(result, 0)
			hasTombstone, err := aw.Has(attribute.AttrTombstone)
			require.NoError(t, err, "NewAttributesWire must index without duplicate-code error")
			assert.True(t, hasTombstone)
		})
	}
}
