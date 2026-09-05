// Design: docs/architecture/perf-round-3.md -- filter-delta allocation reduction
package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// newTestScratch gives one test the arena a modify block would acquire, and
// returns it to the pool when the test ends. A test reads carved windows for as
// long as it runs, so the release belongs at the end of the test rather than at
// the end of the call that carved.
func newTestScratch(t *testing.T) *valueScratch {
	t.Helper()

	scratch := acquireValueScratch()
	t.Cleanup(func() { releaseValueScratch(scratch) })
	return scratch
}

// TestValueScratchCarvesWindowsThatDoNotOverlap covers the arena itself: the
// property every filter-delta encoder relies on is that two carves never share
// a byte, so writing through one window cannot be seen through another.
//
// VALIDATES: the append-only invariant filter_scratch.go states.
// PREVENTS: a carve that rewinds, reuses, or overlaps an earlier region.
func TestValueScratchCarvesWindowsThatDoNotOverlap(t *testing.T) {
	t.Run("writes_through_one_window_stay_in_it", func(t *testing.T) {
		scratch := newTestScratch(t)

		first := scratch.carveBytes(4)
		second := scratch.carveBytes(4)
		third := scratch.carveBytes(2)

		for i := range first {
			first[i] = 0x11
		}
		for i := range second {
			second[i] = 0x22
		}
		for i := range third {
			third[i] = 0x33
		}

		assert.Equal(t, []byte{0x11, 0x11, 0x11, 0x11}, first)
		assert.Equal(t, []byte{0x22, 0x22, 0x22, 0x22}, second)
		assert.Equal(t, []byte{0x33, 0x33}, third)
	})

	t.Run("a_window_is_capped_so_an_append_cannot_reach_the_next_one", func(t *testing.T) {
		scratch := newTestScratch(t)

		first := scratch.carveBytes(2)
		second := scratch.carveBytes(2)
		second[0], second[1] = 0xAA, 0xBB

		// The append leaves the arena rather than writing over the next window.
		grown := append(first, 0xCC) //nolint:gocritic // the point of the case is that first has no spare capacity
		assert.Equal(t, byte(0xCC), grown[2])
		assert.Equal(t, []byte{0xAA, 0xBB}, second, "the next window is untouched")
	})

	t.Run("a_carve_is_zeroed_so_a_reused_arena_leaks_no_bytes", func(t *testing.T) {
		scratch := acquireValueScratch()
		written := scratch.carveBytes(8)
		for i := range written {
			written[i] = 0xFF
		}
		releaseValueScratch(scratch)

		reused := acquireValueScratch()
		defer releaseValueScratch(reused)
		assert.Equal(t, make([]byte, 8), reused.carveBytes(8),
			"a block must not read the bytes the block before it wrote")
	})

	t.Run("a_zero_length_carve_owes_no_bytes", func(t *testing.T) {
		scratch := newTestScratch(t)

		assert.Nil(t, scratch.carveBytes(0))
		assert.Nil(t, scratch.carveSegments(0))
		assert.Nil(t, scratch.carveASNs(0))
	})

	t.Run("segment_and_asn_reservations_do_not_overlap_either", func(t *testing.T) {
		scratch := newTestScratch(t)

		firstASNs := append(scratch.carveASNs(2), 64496, 64497)
		secondASNs := append(scratch.carveASNs(2), 65000, 65001)

		segments := scratch.carveSegments(2)
		segments = append(segments,
			attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: firstASNs},
			attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: secondASNs})

		require.Len(t, segments, 2)
		assert.Equal(t, []uint32{64496, 64497}, segments[0].ASNs)
		assert.Equal(t, []uint32{65000, 65001}, segments[1].ASNs)
	})
}
