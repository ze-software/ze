package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToWireBytesKeepsLargeCommunities verifies the rebuilt attributes carry
// the LARGE_COMMUNITIES attribute in its type-code place.
//
// VALIDATES: an UPDATE received with LARGE_COMMUNITIES (code 32) between an
// unknown code 30 and an unknown code 250 is rebuilt with all three, in
// type-code order, the large community written optional transitive.
//
// PREVENTS: ParseAttributes interning code 32 into its own pooled field while
// ToWireBytes writes only OtherAttrs past the extended communities, so every
// rebuilt UPDATE (the fast path replay, recovery, the flowspec revalidation)
// silently drops the operator's large communities.
//
// Method: parse the attributes, rebuild them, compare the exact bytes.
func TestToWireBytesKeepsLargeCommunities(t *testing.T) {
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	lowUnknown := []byte{0xC0, 0x1E, 0x02, 0xAA, 0xBB}
	large := []byte{0xC0, 0x20, 0x0C, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02}
	highUnknown := []byte{0xC0, 0xFA, 0x01, 0xCC}

	var raw []byte
	raw = append(raw, origin...)
	raw = append(raw, lowUnknown...)
	raw = append(raw, large...)
	raw = append(raw, highUnknown...)

	entry, err := ParseAttributes(raw)
	require.NoError(t, err)
	defer entry.Release()
	require.True(t, entry.GetBundle().hasLargeCommunities(), "code 32 is interned into its own pooled field")

	wire, err := entry.ToWireBytes()
	require.NoError(t, err)
	assert.Equal(t, raw, wire, "the large community is rebuilt between code 30 and code 250")
}
