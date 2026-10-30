package plugin

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/core/bgp/configop"
)

// rfc8212TransitPeer is a second peer, in another group than the edge peer.
const rfc8212TransitPeer = `{"connection":{"remote":{"ip":"203.0.113.2"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"remote":"65002"}}}`

// mixedGroupRoot is a bgp root with the edge peer in group ixp, whose import
// layer is importLayer, and the transit peer in group transit at holdTime.
func mixedGroupRoot(importLayer, holdTime string) string {
	return `{"bgp":{"group":{` +
		`"ixp":{` + importLayer + `"peer":{"edge":` + rfc8212EdgePeer + `}},` +
		`"transit":{"timer":{"receive-hold-time":"` + holdTime + `"},"peer":{"transit":` + rfc8212TransitPeer + `}}}}}`
}

// TestBGPOperationDecomposerCoversMixedDiff proves the decomposer leaves no
// change of a diff behind: a group's import-layer change (a modify-peer) and
// another group's hold-time change in the same commit. The operation path
// gives a participant that owns operations no section apply, so whatever its
// operations do not carry is never applied unless an operation carries it.
// VALIDATES: the decomposition holds the modify-peer for the import change and
// a reconcile-peers operation carrying the candidate root, ordered after the
// modify-peer by the graph.
// PREVENTS: another group's hold-time silently dropped once an import-layer
// change sends the diff down the operation path.
func TestBGPOperationDecomposerCoversMixedDiff(t *testing.T) {
	active := mixedGroupRoot(`"filter":{"import":{"final":"accept"}},`, "90")
	candidate := mixedGroupRoot("", "30")
	ops := decomposeImportLayer(t, active, candidate,
		`{"bgp/group/ixp/filter/import/final":{"old":"accept","new":""},"bgp/group/transit/timer/receive-hold-time":{"old":"90","new":"30"}}`)
	modify := slices.IndexFunc(ops, func(op tx.ConfigOperation) bool {
		return op.Type == configop.ModifyPeer && op.Params.Peer == "edge"
	})
	require.GreaterOrEqual(t, modify, 0, "no modify-peer for the import-layer change: %+v", ops)
	residual := slices.IndexFunc(ops, func(op tx.ConfigOperation) bool { return op.Type == configop.ReconcilePeers })
	require.GreaterOrEqual(t, residual, 0, "the hold-time change is carried by no operation: %+v", ops)
	require.Len(t, ops, 2, "ops %+v", ops)

	var carried map[string]any
	require.NoError(t, json.Unmarshal(ops[residual].Params.Config, &carried))
	want, err := parseBGPSection(candidate)
	require.NoError(t, err)
	require.Equal(t, want, carried, "reconcile-peers does not carry the candidate root")

	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)
	sorted, err := tx.TopologicalSort(graph)
	require.NoError(t, err)
	order := make([]string, 0, len(sorted))
	for i := range sorted {
		order = append(order, sorted[i].ID)
	}
	require.Less(t, slices.Index(order, ops[modify].ID), slices.Index(order, ops[residual].ID),
		"reconcile-peers would run before the modify-peer it must follow: %v", order)
}

// TestBGPOperationDecomposerCoveredDiffHasNoResidual proves the residual is
// emitted only for a change no operation carries.
// VALIDATES: a diff whose only change is one grouped peer's own import layer,
// with every group level and the bgp level unchanged, yields the modify-peer
// alone.
// PREVENTS: a second reconcile of every peer on each operation-path commit.
func TestBGPOperationDecomposerCoveredDiffHasNoResidual(t *testing.T) {
	accepting := `{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"remote":"65001"}},"filter":{"import":{"final":"accept"}}}`
	active := `{"bgp":{"group":{"ixp":{"timer":{"receive-hold-time":"90"},"peer":{"edge":` + accepting + `}}}}}`
	candidate := `{"bgp":{"group":{"ixp":{"timer":{"receive-hold-time":"90"},"peer":{"edge":` + rfc8212EdgePeer + `}}}}}`
	ops := decomposeImportLayer(t, active, candidate, `{"bgp/group/ixp/peer/edge/filter/import/final":{"old":"accept","new":""}}`)
	requireModifyEdge(t, ops)
}

// TestBGPOperationDecomposerResidualCarriesPeerlessChanges proves the residual
// is emitted for a change at the bgp level or in a group level even when every
// static peer is covered by an operation: the router-id beside a bgp-level
// import change, and a dynamic group's range beside a static group's import
// change.
// VALIDATES: one reconcile-peers operation, carrying the candidate root,
// ordered after the modify-peer.
// PREVENTS: the router-id or the dynamic group silently left unapplied
// because the commit also carried a peer operation.
func TestBGPOperationDecomposerResidualCarriesPeerlessChanges(t *testing.T) {
	cases := []struct{ name, active, candidate, changed string }{
		{
			name:      "router-id",
			active:    `{"bgp":{"router-id":"10.0.0.1","filter":{"import":{"final":"accept"}},"peer":{"edge":` + rfc8212EdgePeer + `}}}`,
			candidate: `{"bgp":{"router-id":"10.0.0.2","peer":{"edge":` + rfc8212EdgePeer + `}}}`,
			changed:   `{"bgp/router-id":{"old":"10.0.0.1","new":"10.0.0.2"},"bgp/filter/import/final":{"old":"accept","new":""}}`,
		},
		{
			name:      "dynamic-group",
			active:    `{"bgp":{"group":{"ixp":{"filter":{"import":{"final":"accept"}},"peer":{"edge":` + rfc8212EdgePeer + `}},"dyn":` + residualDynamicGroup("198.51.100.0/24") + `}}}`,
			candidate: `{"bgp":{"group":{"ixp":{"peer":{"edge":` + rfc8212EdgePeer + `}},"dyn":` + residualDynamicGroup("198.51.101.0/24") + `}}}`,
			changed:   `{"bgp/group/ixp/filter/import/final":{"old":"accept","new":""},"bgp/group/dyn/connection/remote/range":{"old":"198.51.100.0/24","new":"198.51.101.0/24"}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := decomposeImportLayer(t, tc.active, tc.candidate, tc.changed)
			requireModifyEdgeAndResidual(t, ops)
			var carried map[string]any
			require.NoError(t, json.Unmarshal(ops[1].Params.Config, &carried))
			want, err := parseBGPSection(tc.candidate)
			require.NoError(t, err)
			require.Equal(t, want, carried, "reconcile-peers does not carry the candidate root")
			graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
			require.NoError(t, err)
			sorted, err := tx.TopologicalSort(graph)
			require.NoError(t, err)
			require.Equal(t, configop.ReconcilePeers, sorted[len(sorted)-1].Type, "reconcile-peers runs before the modify-peer")
		})
	}
}

// residualDynamicGroup is a dynamic group body accepting connections from
// prefix: no static peer, so only the reconcile applies a change to it.
func residualDynamicGroup(prefix string) string {
	return `{"connection":{"remote":{"ip":"dynamic","connect":"false","range":{"` + prefix + `":{}}},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"remote":"65009"}}}`
}
