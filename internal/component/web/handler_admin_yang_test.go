package web

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// TestAdminNavFromYANGTree verifies that the admin console reads its navigation
// off the merged YANG operational command tree. It reads one level at a time,
// and each level is sorted, so the rendered finder columns are deterministic.
//
// The tree used to be flattened into a children map before it reached the
// handler. That map carried no help text, so the command form had nothing to
// show. The handler now takes the tree itself.
//
// VALIDATES: Phase 6 spec deliverable -- admin command tree derived from
// YANG, not from a static map.
// PREVENTS: Plugin-contributed commands silently disappearing from the
// admin nav because someone forgot to update a static map.
func TestAdminNavFromYANGTree(t *testing.T) {
	// Synthesize a small command tree that mirrors what the merged YANG
	// modules produce: root has `peer` and `show` subtrees with their own
	// children. The test verifies navigation shape, not YANG parsing.
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
			"overview": {Name: "overview", WireMethod: "ze-bgp:overview"},
		},
	}

	root := adminChildNames(adminNodeAt(tree, nil))
	require.NotEmpty(t, root, "root must list every top-level command")
	assert.Equal(t, []string{"overview", "peer", "show"}, root, "top-level children must be alphabetical")

	peerKids := adminChildNames(adminNodeAt(tree, []string{"peer"}))
	assert.True(t, sort.StringsAreSorted(peerKids), "peer subtree must be sorted")
	assert.Equal(t, []string{"capabilities", "detail", "teardown"}, peerKids)

	showKids := adminChildNames(adminNodeAt(tree, []string{"show"}))
	assert.Equal(t, []string{"version", "warnings"}, showKids)

	// A leaf command has no children, which is what makes the detail panel
	// render its form rather than another column.
	assert.Empty(t, adminChildNames(adminNodeAt(tree, []string{"peer", "detail"})))
	assert.Empty(t, adminChildNames(adminNodeAt(tree, []string{"overview"})))

	// The finder shows one column for each level down to the selected node.
	fragData := buildAdminFragmentData([]string{"peer"}, tree)
	require.Len(t, fragData.Columns, 2, "root column plus the peer column")
	assert.Nil(t, fragData.CommandForm, "peer has children, so it is a container")
}

// TestAdminNavNilTree verifies that a nil command tree serves an empty console
// without panicking. This is the loader-failure fallback safety net: the hub
// passes the tree it has, and it has none when the YANG loader failed.
func TestAdminNavNilTree(t *testing.T) {
	assert.Empty(t, adminChildNames(adminNodeAt(nil, nil)))
	assert.Nil(t, adminNodeAt(nil, []string{"peer"}))

	fragData := buildAdminFragmentData(nil, nil)
	assert.Empty(t, fragData.Columns)
	assert.Nil(t, fragData.CommandForm)
}

// TestAdminNavEmptyTree verifies that a tree with no children lists nothing.
func TestAdminNavEmptyTree(t *testing.T) {
	assert.Empty(t, adminChildNames(adminNodeAt(&command.Node{}, nil)))
	assert.Empty(t, buildAdminFragmentData(nil, &command.Node{}).Columns)
}

// TestAdminNavDeepNesting verifies that grandchildren and deeper levels are
// reachable. A path leaving the tree answers nil, never the last node the walk
// passed through.
func TestAdminNavDeepNesting(t *testing.T) {
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

	assert.Equal(t, []string{"show"}, adminChildNames(adminNodeAt(tree, nil)))
	assert.Equal(t, []string{"system"}, adminChildNames(adminNodeAt(tree, []string{"show"})))
	assert.Equal(t, []string{"cpu", "memory"},
		adminChildNames(adminNodeAt(tree, []string{"show", "system"})))

	assert.Nil(t, adminNodeAt(tree, []string{"show", "system", "memory", "deeper"}),
		"a path past a leaf holds no node")
	assert.Nil(t, adminNodeAt(tree, []string{"nosuch"}))
}
