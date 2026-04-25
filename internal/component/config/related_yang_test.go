package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestYANGSchemaRelatedTools_BGPPeer verifies that the BGP peer YANG list
// carries the day-one workbench tool descriptors specified in
// spec-web-2-operator-workbench (Day-One BGP Related Tools table).
//
// VALIDATES: YANG `ze:related` extraction wires through to ListNode.Related;
// the standalone bgp/peer list ships peer-detail/capabilities/statistics/
// flush/teardown with row placement and the correct command templates.
// PREVENTS: A schema-build regression silently dropping all per-row tools
// from the workbench, leaving peer rows with no operator actions.
func TestYANGSchemaRelatedTools_BGPPeer(t *testing.T) {
	schema, err := YANGSchema()
	require.NoError(t, err, "YANGSchema build must succeed with ze:related annotations")

	bgpNode := schema.root.Get("bgp")
	require.NotNil(t, bgpNode, "schema must contain bgp container")
	bgp, ok := bgpNode.(*ContainerNode)
	require.True(t, ok, "bgp must be a ContainerNode, got %T", bgpNode)

	// Global BGP tools live on the bgp container.
	globalIDs := relatedIDs(bgp.Related)
	for _, want := range []string{"bgp-health", "bgp-warnings", "bgp-errors"} {
		assert.Contains(t, globalIDs, want, "bgp container missing global tool %q", want)
	}

	peerNode := bgp.Get("peer")
	require.NotNil(t, peerNode, "bgp container must hold standalone peer list")
	peerList, ok := peerNode.(*ListNode)
	require.True(t, ok, "bgp/peer must be a ListNode, got %T", peerNode)

	rowIDs := relatedIDs(peerList.Related)
	for _, want := range []string{"peer-detail", "peer-capabilities", "peer-statistics", "peer-flush", "peer-teardown"} {
		assert.Contains(t, rowIDs, want, "bgp/peer missing row tool %q", want)
	}

	// Spot-check command template carries the expected placeholder so the
	// resolver later (Phase 3) can substitute the peer's remote IP.
	for _, tool := range peerList.Related {
		if tool.ID == "peer-detail" {
			assert.Contains(t, tool.Command, "${path:connection/remote/ip|key}",
				"peer-detail must use the connection/remote/ip|key placeholder")
			assert.Equal(t, RelatedPlacementRow, tool.Placement)
			assert.Equal(t, RelatedPresentationDrawer, tool.Presentation)
			assert.Equal(t, RelatedClassInspect, tool.Class)
		}
		if tool.ID == "peer-teardown" {
			assert.NotEmpty(t, tool.Confirm, "peer-teardown must require confirmation (D8)")
			assert.Equal(t, RelatedClassDanger, tool.Class)
		}
	}

	// The bgp/group/peer list mirrors the same tool ids but with the
	// path-inherit placeholder so the group's remote IP serves as fallback.
	groupsNode := bgp.Get("group")
	require.NotNil(t, groupsNode, "bgp must have a group list")
	groupList, ok := groupsNode.(*ListNode)
	require.True(t, ok, "bgp/group must be a ListNode")
	groupPeerNode := groupList.Get("peer")
	require.NotNil(t, groupPeerNode, "bgp/group must hold a peer list")
	groupPeer, ok := groupPeerNode.(*ListNode)
	require.True(t, ok, "bgp/group/peer must be a ListNode")

	groupRowIDs := relatedIDs(groupPeer.Related)
	for _, want := range []string{"peer-detail", "peer-capabilities", "peer-statistics", "peer-flush", "peer-teardown"} {
		assert.Contains(t, groupRowIDs, want, "bgp/group/peer missing row tool %q", want)
	}
	for _, tool := range groupPeer.Related {
		if tool.ID == "peer-detail" {
			assert.Contains(t, tool.Command, "${path-inherit:connection/remote/ip|key}",
				"bgp/group/peer peer-detail must use path-inherit (D7)")
		}
	}
}

func relatedIDs(tools []*RelatedTool) []string {
	ids := make([]string, 0, len(tools))
	for _, t := range tools {
		ids = append(ids, t.ID)
	}
	return ids
}
