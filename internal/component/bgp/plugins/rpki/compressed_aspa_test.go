package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// draft-ietf-sidrops-aspa-verification Section 5.1: "Let the sequence COMPRESSED_AS_PATH
// {AS(N), AS(N-1),..., AS(2), AS(1)} represent the AS_PATH after removing consecutive
// duplicate ASNs". normalizeASPath builds that sequence from the AS_PATH segments.

// TestASPACompressedASPath pins the consecutive-duplicate removal.
//
// VALIDATES: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1, prepends collapse to one hop and
// a non-consecutive repeat is kept.
// PREVENTS: counting a prepend as an extra hop, or removing a legitimate repeat that is
// not adjacent to its twin.
func TestASPACompressedASPath(t *testing.T) {
	t.Run("consecutive duplicates are removed", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1 positive -- an AS_PATH with
		// prepends {65001 65001 65002 65002 65002 65003} compresses to {65001 65002 65003}.
		segments := []attribute.ASPathSegment{{
			Type: attribute.ASSequence,
			ASNs: []uint32{65001, 65001, 65002, 65002, 65002, 65003},
		}}

		hops, hasSet := normalizeASPath(segments)
		require.False(t, hasSet)
		assert.Equal(t, []uint32{65001, 65002, 65003}, hops)
	})

	t.Run("a non-consecutive repeat is kept", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1 negative -- only CONSECUTIVE
		// duplicates are removed: {65001 65002 65001} keeps both 65001 hops, so the sequence is
		// never collapsed to a set of distinct ASNs.
		segments := []attribute.ASPathSegment{{
			Type: attribute.ASSequence,
			ASNs: []uint32{65001, 65002, 65001},
		}}

		hops, hasSet := normalizeASPath(segments)
		require.False(t, hasSet)
		assert.Equal(t, []uint32{65001, 65002, 65001}, hops)
		assert.NotEqual(t, []uint32{65001, 65002}, hops, "a repeat across another AS is not a duplicate")
	})
}
