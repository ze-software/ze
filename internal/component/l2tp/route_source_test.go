// Design: docs/guide/l2tp.md -- bounded original native route contribution tokens

package l2tp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRouteSourceSequenceNeverReusesTokens checks the cold allocator's first
// useful values without changing the process-wide production sequence.
// MUTATION: omitting the increment returns zero or repeats a contribution token.
func TestRouteSourceSequenceNeverReusesTokens(t *testing.T) {
	var source routeSourceSequence
	first, err := source.next()
	require.NoError(t, err)
	require.Equal(t, uint32(1), first)
	second, err := source.next()
	require.NoError(t, err)
	require.Equal(t, uint32(2), second)
}

// TestRouteSourceSequenceExhaustionRefusesWithoutWrapping takes the final
// usable token, then requires repeatable refusal and an unchanged counter.
// A separately constructed sequence remains useful; no global state is edited.
// MUTATION: removing the exhaustion guard wraps to zero and loses the refusal.
func TestRouteSourceSequenceExhaustionRefusesWithoutWrapping(t *testing.T) {
	source := routeSourceSequence{last: ^uint32(0) - 1}
	last, err := source.next()
	require.NoError(t, err)
	require.Equal(t, ^uint32(0), last)
	for range 2 {
		refused, err := source.next()
		require.ErrorIs(t, err, errRouteSourceExhausted)
		require.Zero(t, refused)
		require.Equal(t, ^uint32(0), source.last)
	}
	var fresh routeSourceSequence
	first, err := fresh.next()
	require.NoError(t, err)
	require.Equal(t, uint32(1), first)
}
