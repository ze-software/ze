package web

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
)

// TestBuildAdminCommandTree_FromYANG verifies that AdminTreeFromYANG
// converts a merged YANG operational command tree into the children-map
// format consumed by HandleAdminView. Each level lists its children sorted
// alphabetically so the rendered finder columns are deterministic.
//
// VALIDATES: Phase 6 spec deliverable -- admin command tree derived from
// YANG, not from the static BuildAdminCommandTree map.
// PREVENTS: Plugin-contributed commands silently disappearing from the
// admin nav because someone forgot to update the static map.
func TestBuildAdminCommandTree_FromYANG(t *testing.T) {
	// Synthesize a small command tree that mirrors what the merged YANG
	// modules produce: root has `peer` and `show` subtrees with their own
	// children. The test verifies adapter shape, not YANG parsing.
	tree := &command.Node{
		Children: map[string]*command.Node{
			"peer": {
				Name: "peer",
				Children: map[string]*command.Node{
					"detail":       {Name: "detail", WireMethod: "ze-bgp:peer-detail"},
					"capabilities": {Name: "capabilities", WireMethod: "ze-bgp:peer-capabilities"},
					"teardown":     {Name: "teardown", WireMethod: "ze-bgp:peer-teardown"},
				},
			},
			"show": {
				Name: "show",
				Children: map[string]*command.Node{
					"version":  {Name: "version", WireMethod: "ze-show:version"},
					"warnings": {Name: "warnings", WireMethod: "ze-show:warnings"},
				},
			},
			"summary": {Name: "summary", WireMethod: "ze-bgp:summary"},
		},
	}

	got := AdminTreeFromYANG(tree)

	root := got[""]
	require.NotEmpty(t, root, "root must list every top-level command")
	assert.Equal(t, []string{"peer", "show", "summary"}, root, "top-level children must be alphabetical")

	peerKids := got["peer"]
	assert.True(t, sort.StringsAreSorted(peerKids), "peer subtree must be sorted")
	assert.Equal(t, []string{"capabilities", "detail", "teardown"}, peerKids)

	showKids := got["show"]
	assert.Equal(t, []string{"version", "warnings"}, showKids)

	// Leaf commands have no further children -- they are absent from the
	// children map (the FragmentData builder treats missing keys as leaves).
	_, hasLeaf := got["peer/detail"]
	assert.False(t, hasLeaf, "leaves must not appear with an empty child slice")
	_, hasSummary := got["summary"]
	assert.False(t, hasSummary, "leaves at the root must not appear")
}

// TestAdminTreeFromYANG_NilTree verifies that a nil command tree produces
// an empty map without panicking. This is the loader-failure fallback
// safety net.
func TestAdminTreeFromYANG_NilTree(t *testing.T) {
	got := AdminTreeFromYANG(nil)
	assert.Empty(t, got)
}

// TestAdminTreeFromYANG_EmptyTree verifies that a tree with no children
// produces an empty map (the empty-key entry is omitted because there is
// nothing to list).
func TestAdminTreeFromYANG_EmptyTree(t *testing.T) {
	got := AdminTreeFromYANG(&command.Node{})
	assert.Empty(t, got)
}

// TestAdminTreeFromYANG_DeepNesting verifies that grandchildren and deeper
// levels are reachable via slash-joined keys.
func TestAdminTreeFromYANG_DeepNesting(t *testing.T) {
	tree := &command.Node{
		Children: map[string]*command.Node{
			"show": {
				Children: map[string]*command.Node{
					"system": {
						Children: map[string]*command.Node{
							"memory": {WireMethod: "ze-show:system-memory"},
							"cpu":    {WireMethod: "ze-show:system-cpu"},
						},
					},
				},
			},
		},
	}
	got := AdminTreeFromYANG(tree)
	assert.Equal(t, []string{"show"}, got[""])
	assert.Equal(t, []string{"system"}, got["show"])
	assert.Equal(t, []string{"cpu", "memory"}, got["show/system"])
}
