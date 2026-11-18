package cli

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompletionOrderUnchanged: the editor's completion menu stays
// alphabetical while every printer follows YANG definition order. Method:
// complete `set ` inside bgp, whose module declares router-id (l.47), then
// session (l.148), then peer (l.302); the menu must list them peer,
// router-id, session.
//
// VALIDATES: AC-12, D-5 of spec-config-yang-definition-order: completion is
// a menu of what may be typed, read from goyang and sorted
// (getSortedChildren), never the schema's print order.
// PREVENTS: the menu silently taking the print order, or a later change to
// it passing unnoticed (R-4).
func TestCompletionOrderUnchanged(t *testing.T) {
	c := newTestCompleter(t)
	texts := completionTexts(c.Complete(nil, "set ", []string{"bgp"}))

	positions := make([]int, 0, 3)
	for _, name := range []string{"peer", "router-id", "session"} {
		at := slices.Index(texts, name)
		require.NotEqual(t, -1, at, "the bgp menu lacks %q: %v", name, texts)
		positions = append(positions, at)
	}
	require.True(t, slices.IsSorted(positions),
		"the menu lists peer, router-id, session out of alphabetical order: %v", texts)
}
