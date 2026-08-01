// Tests for the multi-occurrence case of ATTR_DISCARD.
//
// RFC 7606 Section 2 says "the attribute MUST be discarded". RFC 8669 Section 4 says the
// same of the Prefix-SID attribute at the EBGP boundary. Both name the ATTRIBUTE, not its
// first occurrence: a discard that rewrites one copy and leaves a second is a discard that
// did not happen. These tests hold ApplyAttrDiscard to that, independently of any caller.

package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestApplyAttrDiscardRemovesEveryOccurrence is the discriminator for the first of the two
// causes behind the RFC 8669 Section 4 leak: applyInPlace located the attribute with
// AttrFind, which returns the FIRST match, and rewrote only that one.
//
// It drives ApplyAttrDiscard directly, so it fails on that cause alone and cannot be
// closed by any change to the reactor's enforcement layer.
//
// VALIDATES: one discard entry removes EVERY occurrence of that code, and leaves exactly
// one ATTR_TOMBSTONE (draft-mangin-idr-attr-tombstone-00 Section 5.1 permits one merged
// marker, and a second marker would itself be the duplicate code the attribute index
// rejects as a hard error).
// PREVENTS: a repeated attribute surviving a discard, which for Prefix-SID is a
// wire-visible RFC 8669 Section 4 violation and for any code is a duplicate the RIB drops.
func TestApplyAttrDiscardRemovesEveryOccurrence(t *testing.T) {
	tests := []struct {
		name        string
		occurrences int
	}{
		{"one", 1},
		{"two", 2},
		{"three", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := concatBytes(
				makeAttr(0x40, 1, []byte{0x00}),          // ORIGIN
				makeAttr(0x40, 2, []byte{}),              // AS_PATH (empty)
				makeAttr(0x40, 3, []byte{0xC0, 0, 2, 1}), // NEXT_HOP
			)
			for i := range tt.occurrences {
				// Distinct Label-Index per copy, so a survivor names itself.
				attrs = concatBytes(attrs, makeAttr(0xC0, uint8(attribute.AttrPrefixSID),
					[]byte{1, 0, 7, 0, 0, 0, 0x00, 0x00, 0x03, byte(0x09 + i)}))
			}

			entries := []DiscardEntry{{
				Code:   uint8(attribute.AttrPrefixSID),
				Reason: DiscardReasonEBGPInvalid,
			}}
			result, _ := ApplyAttrDiscard(attrs, entries)

			assert.Equal(t, 0, countAttrByCode(result, uint8(attribute.AttrPrefixSID)),
				"every occurrence of a discarded attribute must be gone, %d were sent", tt.occurrences)
			assert.Equal(t, 1, countAttrByCode(result, uint8(attribute.AttrTombstone)),
				"exactly one merged ATTR_TOMBSTONE records the discard")

			// The attributes that were not discarded are untouched.
			require.Equal(t, 1, countAttrByCode(result, 1), "ORIGIN must survive")
			require.Equal(t, 1, countAttrByCode(result, 3), "NEXT_HOP must survive")
		})
	}
}

// TestApplyAttrDiscardTombstoneRecordsTheCodeOnce proves the marker for a repeated
// attribute names the discarded code exactly once, rather than once per copy removed.
//
// VALIDATES: draft-mangin-idr-attr-tombstone-00 Section 4.1 — the marker's value is a list
// of (code, reason) pairs describing which attributes were discarded. Three copies of one
// attribute are one discarded attribute, so one pair.
// PREVENTS: a marker that grows with the peer's repetition count, which would let a remote
// peer inflate the attribute an implementation must re-parse.
func TestApplyAttrDiscardTombstoneRecordsTheCodeOnce(t *testing.T) {
	attrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),
		makeAttr(0xC0, uint8(attribute.AttrPrefixSID), []byte{1, 0, 7, 0, 0, 0, 0, 0, 3, 9}),
		makeAttr(0xC0, uint8(attribute.AttrPrefixSID), []byte{1, 0, 7, 0, 0, 0, 0, 0, 3, 10}),
		makeAttr(0xC0, uint8(attribute.AttrPrefixSID), []byte{1, 0, 7, 0, 0, 0, 0, 0, 3, 11}),
	)

	result, _ := ApplyAttrDiscard(attrs, []DiscardEntry{{
		Code:   uint8(attribute.AttrPrefixSID),
		Reason: DiscardReasonEBGPInvalid,
	}})

	marker := findAttrByCode(result, uint8(attribute.AttrTombstone))
	require.NotNil(t, marker, "the discard must leave a marker")

	entries := ExtractUpstreamAttrDiscard(result)
	require.Len(t, entries, 1, "three copies of one attribute are one discarded attribute")
	assert.Equal(t, uint8(attribute.AttrPrefixSID), entries[0].Code)
	assert.Equal(t, DiscardReasonEBGPInvalid, entries[0].Reason)
}
