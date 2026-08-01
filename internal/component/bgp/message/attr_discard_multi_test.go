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

// TestApplyAttrDiscardMergedFlagsUseEveryOccurrence pins the Section 5.7 rule
// against the multi-occurrence case the multi-occurrence guard newly exposes.
//
// Before that guard landed, a single-entry discard was absorbed by applyInPlace
// and never reached rebuildWithAttrDiscard, so the merged-marker derivation was
// only ever asked about codes present once. It now always reaches the rebuild,
// and the derivation read the flags of the FIRST occurrence alone
// (findAttrFlags -> attribute.AttrFind). A peer sending one code twice with
// different Transitive bits therefore inverted the MUST.
//
// VALIDATES: draft-mangin-idr-attr-tombstone-00 Section 5.7 -- "if all discarded
// attributes were transitive, the result is transitive (0xC0); if all were
// non-transitive, non-transitive (0x80); if mixed, the result MUST be
// non-transitive (0x80)". Every occurrence counts, not the first.
// PREVENTS: a transitive marker for a mixed set, which propagates a discard
// record past the EBGP boundary the RFC keeps it inside. The "all transitive"
// and "all non-transitive" rows are here so a fix that simply hardcodes 0x80
// fails: the derivation must still answer 0xC0 when it should.
func TestApplyAttrDiscardMergedFlagsUseEveryOccurrence(t *testing.T) {
	const (
		optionalTransitive    = 0xC0
		optionalNonTransitive = 0x80
	)
	tests := []struct {
		name      string
		occFlags  []uint8
		wantFlags uint8
		why       string
	}{
		{
			name:      "all transitive",
			occFlags:  []uint8{optionalTransitive, optionalTransitive},
			wantFlags: optionalTransitive,
			why:       "every occurrence carried Transitive, so the merged marker is transitive",
		},
		{
			name:      "mixed, transitive first",
			occFlags:  []uint8{optionalTransitive, optionalNonTransitive},
			wantFlags: optionalNonTransitive,
			why:       "a mixed set MUST be non-transitive; reading only the first occurrence answered 0xC0",
		},
		{
			name:      "mixed, non-transitive first",
			occFlags:  []uint8{optionalNonTransitive, optionalTransitive},
			wantFlags: optionalNonTransitive,
			why:       "a mixed set MUST be non-transitive whichever copy arrives first",
		},
		{
			name:      "all non-transitive",
			occFlags:  []uint8{optionalNonTransitive, optionalNonTransitive},
			wantFlags: optionalNonTransitive,
			why:       "no occurrence carried Transitive",
		},
		{
			name:      "mixed across three occurrences",
			occFlags:  []uint8{optionalTransitive, optionalTransitive, optionalNonTransitive},
			wantFlags: optionalNonTransitive,
			why:       "the divergent copy is last, so a scan that stops early misses it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := makeAttr(0x40, 1, []byte{0x00}) // ORIGIN, not discarded
			for i, f := range tt.occFlags {
				attrs = concatBytes(attrs, makeAttr(f, uint8(attribute.AttrPrefixSID),
					[]byte{1, 0, 7, 0, 0, 0, 0x00, 0x00, 0x03, byte(0x09 + i)}))
			}

			result, rebuilt := ApplyAttrDiscard(attrs, []DiscardEntry{{
				Code:   uint8(attribute.AttrPrefixSID),
				Reason: DiscardReasonEBGPInvalid,
			}})

			require.True(t, rebuilt,
				"a multi-occurrence discard cannot be expressed in place, so it must rebuild")
			marker := findAttrByCode(result, uint8(attribute.AttrTombstone))
			require.NotNil(t, marker, "the discard must leave a marker")

			assert.Equal(t, tt.wantFlags, marker[0], "%s", tt.why)
			assert.Zero(t, countAttrByCode(result, uint8(attribute.AttrPrefixSID)),
				"every occurrence must still be removed")
		})
	}
}

// TestApplyAttrDiscardMergedFlagsAcrossCodes pins the same rule for the other
// mixed shape: two DIFFERENT codes whose Transitive bits disagree. This one was
// already correct, and it is here so the every-occurrence walk cannot regress it
// while fixing the repeated-code case.
//
// VALIDATES: draft-mangin-idr-attr-tombstone-00 Section 5.7 across codes.
// PREVENTS: the per-code walk answering from whichever code it saw last.
func TestApplyAttrDiscardMergedFlagsAcrossCodes(t *testing.T) {
	attrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),  // ORIGIN, not discarded
		makeAttr(0xC0, 40, []byte{1, 2}), // discarded, transitive
		makeAttr(0x80, 41, []byte{3, 4}), // discarded, non-transitive
	)

	result, rebuilt := ApplyAttrDiscard(attrs, []DiscardEntry{
		{Code: 40, Reason: DiscardReasonEBGPInvalid},
		{Code: 41, Reason: DiscardReasonMalformedValue},
	})

	require.True(t, rebuilt, "two entries always rebuild")
	marker := findAttrByCode(result, uint8(attribute.AttrTombstone))
	require.NotNil(t, marker)
	assert.Equal(t, uint8(0x80), marker[0],
		"one transitive and one non-transitive attribute is a mixed set: MUST be non-transitive")
}

// TestApplyAttrDiscardMergedFlagsAbsentCodeIsNotTransitive pins the miss branch.
//
// VALIDATES: a discarded code that is not in the section contributes no
// transitivity evidence, so the merged marker stays non-transitive. This is the
// answer the previous findAttrFlags shape gave by returning 0 for a miss, and it
// is the fail-closed direction (see localSetTransitive).
// PREVENTS: the every-occurrence walk defaulting an unseen code to transitive,
// which would emit 0xC0 on no evidence at all.
func TestApplyAttrDiscardMergedFlagsAbsentCodeIsNotTransitive(t *testing.T) {
	attrs := concatBytes(
		makeAttr(0x40, 1, []byte{0x00}),  // ORIGIN
		makeAttr(0xC0, 40, []byte{1, 2}), // present and transitive
	)

	result, rebuilt := ApplyAttrDiscard(attrs, []DiscardEntry{
		{Code: 40, Reason: DiscardReasonEBGPInvalid},
		{Code: 99, Reason: DiscardReasonMalformedValue}, // never present
	})

	require.True(t, rebuilt)
	marker := findAttrByCode(result, uint8(attribute.AttrTombstone))
	require.NotNil(t, marker)
	assert.Equal(t, uint8(0x80), marker[0],
		"a discarded code absent from the section is no evidence of transitivity")
}
