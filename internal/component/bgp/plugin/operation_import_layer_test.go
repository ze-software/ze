package plugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/core/bgp/configop"
)

// rfc8212EdgePeer is a peer body both layers of these tests attach.
const rfc8212EdgePeer = `{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"remote":"65001"}}}`

// decomposeImportLayer runs the decomposer over one active and candidate bgp
// root with the diff the reload computes for it.
func decomposeImportLayer(t *testing.T, active, candidate, changed string) []tx.ConfigOperation {
	t.Helper()
	ops, err := decomposeBGPOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-import-layer",
		Root:          configRootBGP,
		ActiveRoot:    active,
		CandidateRoot: candidate,
		Diff:          tx.DiffSection{Root: configRootBGP, Changed: changed},
	})
	require.NoError(t, err)
	return ops
}

// requireModifyEdge checks ops is exactly one in-place modify of peer edge,
// carrying the peer's own body, so the reactor swaps it under the operation's
// journal and transaction.
func requireModifyEdge(t *testing.T, ops []tx.ConfigOperation) {
	t.Helper()
	require.Len(t, ops, 1, "ops %+v", ops)
	assert.Equal(t, configop.ModifyPeer, ops[0].Type)
	assert.Equal(t, tx.VerbModify, ops[0].Verb)
	assert.Equal(t, "edge", ops[0].Params.Peer)
	assert.Equal(t, "192.0.2.1", ops[0].Params.Address)
	var body map[string]any
	require.NoError(t, json.Unmarshal(ops[0].Params.Config, &body))
	assert.Contains(t, body, "connection", "operation config is not the peer's body")
	assert.NotEmpty(t, ops[0].Params.OldConfig)
}

// requireModifyEdgeAndResidual checks ops is the in-place modify of peer edge
// (requireModifyEdge) followed by the reconcile-peers residual: a change at the
// bgp or group level also reaches what no peer operation carries, the dynamic
// groups' templates among them (bgpResidualOperation).
func requireModifyEdgeAndResidual(t *testing.T, ops []tx.ConfigOperation) {
	t.Helper()
	require.Len(t, ops, 2, "ops %+v", ops)
	requireModifyEdge(t, ops[:1])
	assert.Equal(t, configop.ReconcilePeers, ops[1].Type, "ops %+v", ops)
}

// TestBGPOperationDecomposerInheritedImportChangeModifiesPeer proves RFC 8212
// D-2's routing half: a change to an import policy layer a peer inherits (the
// bgp level, its group, or a grouped peer's own policy) reaches the BGP plugin
// as a modify-peer operation for every peer it reaches (and, for a bgp or
// group level change, the reconcile-peers residual), so the swap runs under
// the operation's journal and transaction and is invalidated in place, rather
// than falling to the whole-section apply, which has no transaction and so
// refuses the change.
//
// VALIDATES: decomposeBGPOperations emits modify-peer for a peer whose
// inherited or grouped import layer changed.
// PREVENTS: an operator's `final reject` at bgp or group level being refused
// with no way to apply it.
func TestBGPOperationDecomposerInheritedImportChangeModifiesPeer(t *testing.T) {
	t.Run("bgp-level", func(t *testing.T) {
		active := `{"bgp":{"filter":{"import":{"final":"accept"}},"peer":{"edge":` + rfc8212EdgePeer + `}}}`
		candidate := `{"bgp":{"filter":{"import":{"final":"reject"}},"peer":{"edge":` + rfc8212EdgePeer + `}}}`
		ops := decomposeImportLayer(t, active, candidate, `{"bgp/filter/import/final":{"old":"accept","new":"reject"}}`)
		requireModifyEdgeAndResidual(t, ops)
	})
	t.Run("group-level", func(t *testing.T) {
		active := `{"bgp":{"group":{"ixp":{"filter":{"import":{"final":"accept"}},"peer":{"edge":` + rfc8212EdgePeer + `}}}}}`
		candidate := `{"bgp":{"group":{"ixp":{"peer":{"edge":` + rfc8212EdgePeer + `}}}}}`
		ops := decomposeImportLayer(t, active, candidate, `{"bgp/group/ixp/filter/import/final":{"old":"accept","new":""}}`)
		requireModifyEdgeAndResidual(t, ops)
	})
	t.Run("grouped-peer", func(t *testing.T) {
		accepting := `{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"remote":"65001"}},"filter":{"import":{"final":"accept"}}}`
		active := `{"bgp":{"group":{"ixp":{"peer":{"edge":` + accepting + `}}}}}`
		candidate := `{"bgp":{"group":{"ixp":{"peer":{"edge":` + rfc8212EdgePeer + `}}}}}`
		ops := decomposeImportLayer(t, active, candidate, `{"bgp/group/ixp/peer/edge/filter/import/final":{"old":"accept","new":""}}`)
		requireModifyEdge(t, ops)
	})
}

// TestBGPOperationDecomposerInheritedNonImportChangeKeepsSection proves the
// routing stays narrow: an inherited change that leaves every import layer as
// it was keeps the whole-section apply it has today.
//
// VALIDATES: no operation for a group-level change outside the import layers.
// PREVENTS: the import routing pulling unrelated group changes onto the
// operation path.
func TestBGPOperationDecomposerInheritedNonImportChangeKeepsSection(t *testing.T) {
	active := `{"bgp":{"group":{"ixp":{"description":"old","peer":{"edge":` + rfc8212EdgePeer + `}}}}}`
	candidate := `{"bgp":{"group":{"ixp":{"description":"new","peer":{"edge":` + rfc8212EdgePeer + `}}}}}`
	ops := decomposeImportLayer(t, active, candidate, `{"bgp/group/ixp/description":{"old":"old","new":"new"}}`)
	assert.Empty(t, ops)
}

// TestBGPOperationCollectorsRefuseMalformedSubtree proves a bgp root whose
// filter, peer or group subtree is not a container is refused by both
// collectors rather than read as configuring nothing: an import layer skipped
// that way would hide an accepting import policy from the RFC 8212 change
// detection. Method: each malformed root through collectBGPImportLayers and
// collectBGPEffectivePeers, then a well-formed root that still collects.
func TestBGPOperationCollectorsRefuseMalformedSubtree(t *testing.T) {
	malformed := map[string]string{
		"bgp filter":   `{"filter":"accept","peer":{"edge":` + rfc8212EdgePeer + `}}`,
		"bgp peer":     `{"peer":"edge"}`,
		"bgp group":    `{"group":"edges"}`,
		"group filter": `{"group":{"edges":{"filter":["import"],"peer":{"edge":` + rfc8212EdgePeer + `}}}}`,
		"group peer":   `{"group":{"edges":{"peer":"edge"}}}`,
		"peer filter":  `{"peer":{"edge":{"filter":7}}}`,
	}
	for name, raw := range malformed {
		t.Run(name, func(t *testing.T) {
			var root map[string]any
			require.NoError(t, json.Unmarshal([]byte(raw), &root))
			_, err := collectBGPImportLayers(root)
			require.Error(t, err, "import layers of %s", raw)
			assert.Contains(t, err.Error(), "has type")
		})
	}
	for _, name := range []string{"bgp peer", "bgp group", "group peer"} {
		t.Run("effective "+name, func(t *testing.T) {
			var root map[string]any
			require.NoError(t, json.Unmarshal([]byte(malformed[name]), &root))
			_, err := collectBGPEffectivePeers(root)
			require.Error(t, err, "effective peers of %s", malformed[name])
		})
	}
	var root map[string]any
	require.NoError(t, json.Unmarshal([]byte(`{"filter":{"import":["p:a"]},"group":{"edges":{"peer":{"edge":`+rfc8212EdgePeer+`}}}}`), &root))
	layers, err := collectBGPImportLayers(root)
	require.NoError(t, err)
	assert.Contains(t, layers["edge"].layers, "p:a")
	peers, err := collectBGPEffectivePeers(root)
	require.NoError(t, err)
	assert.Contains(t, peers, "edge")
}
