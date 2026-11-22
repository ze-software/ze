package pool

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
)

// TestAppendLabelsReadsTheStackIntoTheCallersBuffer verifies AppendLabels
// yields the interned stack, top first, after what dst already holds.
//
// VALIDATES: a two-label stack comes back in order behind dst's own element,
// and ResolveLabels answers the same stack as an owned slice.
// PREVENTS: a walk that reads one label of a stack, or a reversed stack.
func TestAppendLabelsReadsTheStackIntoTheCallersBuffer(t *testing.T) {
	h := InternLabels([]uint32{100, 200})
	require.True(t, h.IsValid())
	t.Cleanup(func() { require.NoError(t, Labels.Release(h)) })

	var stack [4]uint32
	stack[0] = 7

	require.Equal(t, []uint32{7, 100, 200}, AppendLabels(stack[:1], h))
	require.Equal(t, []uint32{100, 200}, ResolveLabels(h))
}

// TestAppendLabelsAppendsNothingWithoutAStack verifies the answer a caller
// reads as "no label stack": dst comes back at its own length.
//
// VALIDATES: InvalidHandle appends nothing, and ResolveLabels answers nil.
// PREVENTS: a zero label invented for a route that has no label binding.
func TestAppendLabelsAppendsNothingWithoutAStack(t *testing.T) {
	var stack [4]uint32

	require.Empty(t, AppendLabels(stack[:0], attrpool.InvalidHandle))
	require.Nil(t, ResolveLabels(attrpool.InvalidHandle))
}
