package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
)

// TestBGPOperationDecomposerPeerLocalAddressChange verifies that BGP owns
// decomposition of peer changes that depend on interface addresses.
//
// VALIDATES: a peer local-address change decomposes to REMOVE_PEER(old) then ADD_PEER(new).
// PREVENTS: generic transaction code parsing BGP peer semantics or dropping peer changes.
func TestBGPOperationDecomposerPeerLocalAddressChange(t *testing.T) {
	active := `{"bgp":{"session":{"asn":{"local":"65000"}},"peer":{"edge":{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"remote":"65001"}}}}}}`
	candidate := `{"bgp":{"session":{"asn":{"local":"65000"}},"peer":{"edge":{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.2"}},"session":{"asn":{"remote":"65001"}}}}}}`

	ops, err := decomposeBGPOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-bgp-op",
		Root:          configRootBGP,
		ActiveRoot:    active,
		CandidateRoot: candidate,
		Diff: tx.DiffSection{
			Root:    configRootBGP,
			Changed: `{"bgp/peer/edge/connection/local/ip":{"old":"192.0.2.1","new":"192.0.2.2"}}`,
		},
	})
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, tx.OperationRemovePeer, ops[0].Type)
	assert.Equal(t, "bgp", ops[0].Owner)
	assert.Equal(t, tx.ResourcePeer, ops[0].Target.Kind)
	assert.Equal(t, "edge", ops[0].Params.Peer)
	assert.Equal(t, "192.0.2.1", ops[0].Params.Address)
	assert.NotEmpty(t, ops[0].Params.OldConfig)

	assert.Equal(t, tx.OperationAddPeer, ops[1].Type)
	assert.Equal(t, "edge", ops[1].Params.Peer)
	assert.Equal(t, "192.0.2.2", ops[1].Params.Address)
	assert.NotEmpty(t, ops[1].Params.Config)
}

// TestBGPOperationDecomposerPeerModifySameAddress verifies that a peer config
// change without local-address change decomposes to MODIFY_PEER instead of
// REMOVE_PEER + ADD_PEER.
//
// VALIDATES: same-address peer change uses MODIFY_PEER.
// PREVENTS: unnecessary address settlement when only non-address peer fields change.
func TestBGPOperationDecomposerPeerModifySameAddress(t *testing.T) {
	active := `{"bgp":{"session":{"asn":{"local":"65000"}},"peer":{"edge":{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"remote":"65001"}}}}}}`
	candidate := `{"bgp":{"session":{"asn":{"local":"65000"}},"peer":{"edge":{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"remote":"65002"}}}}}}`

	ops, err := decomposeBGPOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-bgp-modify",
		Root:          configRootBGP,
		ActiveRoot:    active,
		CandidateRoot: candidate,
		Diff: tx.DiffSection{
			Root:    configRootBGP,
			Changed: `{"bgp/peer/edge/session/asn/remote":{"old":"65001","new":"65002"}}`,
		},
	})
	require.NoError(t, err)
	require.Len(t, ops, 1)

	assert.Equal(t, tx.OperationModifyPeer, ops[0].Type)
	assert.Equal(t, "edge", ops[0].Params.Peer)
	assert.Equal(t, "192.0.2.1", ops[0].Params.Address)
	assert.NotEmpty(t, ops[0].Params.Config)
	assert.NotEmpty(t, ops[0].Params.OldConfig)
}

// TestBGPOperationDecomposerRouterIDRotationSplitsPeers verifies that a
// cross-peer router-id rotation removes every affected peer before adding any
// replacement peer.
//
// VALIDATES: router-id rotations decompose to REMOVE_PEER batch then ADD_PEER batch.
// PREVENTS: adding a replacement while another old session still owns that router-id.
func TestBGPOperationDecomposerRouterIDRotationSplitsPeers(t *testing.T) {
	active := `{"bgp":{"session":{"asn":{"local":"65000"}},"peer":{"peer1":{"connection":{"remote":{"ip":"127.0.0.1"},"local":{"ip":"127.0.0.1"}},"session":{"asn":{"remote":"65000"},"router-id":"1.2.3.4"}},"peer2":{"connection":{"remote":{"ip":"127.0.0.2"},"local":{"ip":"127.0.0.2"}},"session":{"asn":{"remote":"65000"},"router-id":"5.6.7.8"}}}}}`
	candidate := `{"bgp":{"session":{"asn":{"local":"65000"}},"peer":{"peer1":{"connection":{"remote":{"ip":"127.0.0.1"},"local":{"ip":"127.0.0.1"}},"session":{"asn":{"remote":"65000"},"router-id":"5.6.7.8"}},"peer2":{"connection":{"remote":{"ip":"127.0.0.2"},"local":{"ip":"127.0.0.2"}},"session":{"asn":{"remote":"65000"},"router-id":"1.2.3.4"}}}}}`

	ops, err := decomposeBGPOperations(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-bgp-router-id-rotation",
		Root:          configRootBGP,
		ActiveRoot:    active,
		CandidateRoot: candidate,
		Diff: tx.DiffSection{
			Root:    configRootBGP,
			Changed: `{"bgp/peer/peer1/session/router-id":{"old":"1.2.3.4","new":"5.6.7.8"},"bgp/peer/peer2/session/router-id":{"old":"5.6.7.8","new":"1.2.3.4"}}`,
		},
	})
	require.NoError(t, err)
	require.Len(t, ops, 4)

	assert.Equal(t, tx.OperationRemovePeer, ops[0].Type)
	assert.Equal(t, "peer1", ops[0].Params.Peer)
	assert.Equal(t, tx.OperationRemovePeer, ops[1].Type)
	assert.Equal(t, "peer2", ops[1].Params.Peer)
	assert.Equal(t, tx.OperationAddPeer, ops[2].Type)
	assert.Equal(t, "peer1", ops[2].Params.Peer)
	assert.Equal(t, tx.OperationAddPeer, ops[3].Type)
	assert.Equal(t, "peer2", ops[3].Params.Peer)
}

// TestBGPOperationDecomposerNoPeerChangesFallsBack verifies that non-peer BGP
// changes do not enter the operation path until they have exact operation support.
//
// VALIDATES: BGP router-id-only changes return no operations for legacy fallback.
// PREVENTS: partial BGP operation decomposition silently approximating unsupported changes.
func TestBGPOperationDecomposerNoPeerChangesFallsBack(t *testing.T) {
	ops, err := decomposeBGPOperations(context.Background(), tx.DecomposeRequest{
		Root:          configRootBGP,
		ActiveRoot:    `{"bgp":{"router-id":"1.2.3.4"}}`,
		CandidateRoot: `{"bgp":{"router-id":"5.6.7.8"}}`,
		Diff: tx.DiffSection{
			Root:    configRootBGP,
			Changed: `{"bgp/router-id":{"old":"1.2.3.4","new":"5.6.7.8"}}`,
		},
	})
	require.NoError(t, err)
	assert.Empty(t, ops)
}

// TestBGPConstraintRulesOrderPeerAgainstAddress verifies BGP registers its
// address dependency rules as data consumed by the generic graph builder.
//
// VALIDATES: ADD_ADDRESS -> ADD_PEER and REMOVE_PEER -> REMOVE_ADDRESS edges are registry rules.
// PREVENTS: hardcoded BGP ordering logic in the transaction graph solver.
func TestBGPConstraintRulesOrderPeerAgainstAddress(t *testing.T) {
	ops := []tx.ConfigOperation{
		{ID: "addr-add", Type: tx.OperationAddAddress, Target: tx.ResourceRef{Kind: tx.ResourceAddress, Interface: "dum0", Address: "192.0.2.2/32"}},
		{ID: "peer-add", Type: tx.OperationAddPeer, Target: tx.ResourceRef{Kind: tx.ResourcePeer, Peer: "edge"}, Params: tx.ConfigOperationParams{Address: "192.0.2.2"}},
		{ID: "peer-remove", Type: tx.OperationRemovePeer, Target: tx.ResourceRef{Kind: tx.ResourcePeer, Peer: "edge"}, Params: tx.ConfigOperationParams{Address: "192.0.2.1"}},
		{ID: "addr-remove", Type: tx.OperationRemoveAddress, Target: tx.ResourceRef{Kind: tx.ResourceAddress, Interface: "dum0", Address: "192.0.2.1/32"}},
	}

	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)
	assert.True(t, graph.HasEdge("addr-add", "peer-add"), "new local address must exist before adding peer")
	assert.True(t, graph.HasEdge("peer-remove", "addr-remove"), "peer must be removed before deleting its local address")
}

// TestBGPListenerConstraintRules verifies BGP registers O6 and O7 listener
// ordering rules as data consumed by the generic graph builder.
//
// VALIDATES: ADD_ADDRESS -> ADD_LISTENER and REMOVE_LISTENER -> REMOVE_ADDRESS edges are registry rules.
// PREVENTS: future listener operations executing without address ordering constraints.
func TestBGPListenerConstraintRules(t *testing.T) {
	ops := []tx.ConfigOperation{
		{ID: "addr-add", Type: tx.OperationAddAddress, Target: tx.ResourceRef{Kind: tx.ResourceAddress, Interface: "dum0", Address: "192.0.2.2/32"}},
		{ID: "listener-add", Type: tx.OperationAddListener, Target: tx.ResourceRef{Kind: tx.ResourceListener, Address: "192.0.2.2", Port: 179}},
		{ID: "listener-remove", Type: tx.OperationRemoveListener, Target: tx.ResourceRef{Kind: tx.ResourceListener, Address: "192.0.2.1", Port: 179}},
		{ID: "addr-remove", Type: tx.OperationRemoveAddress, Target: tx.ResourceRef{Kind: tx.ResourceAddress, Interface: "dum0", Address: "192.0.2.1/32"}},
	}

	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)
	assert.True(t, graph.HasEdge("addr-add", "listener-add"), "address must exist before starting listener")
	assert.True(t, graph.HasEdge("listener-remove", "addr-remove"), "listener must stop before removing address")
}

// TestBGPSettlementRulesWaitForListenerReady verifies BGP registers the
// readiness event needed after adding a local-address peer.
//
// VALIDATES: ADD_PEER operations wait for bgp/listener-ready settlement.
// PREVENTS: Removing old addresses before the replacement BGP listener is observable.
func TestBGPSettlementRulesWaitForListenerReady(t *testing.T) {
	op := tx.ConfigOperation{
		ID:     "peer-add",
		Type:   tx.OperationAddPeer,
		Target: tx.ResourceRef{Kind: tx.ResourcePeer, Peer: "edge"},
		Params: tx.ConfigOperationParams{Address: "192.0.2.2"},
	}

	rules := tx.SettlementRulesFor(&op)
	require.NotEmpty(t, rules)
	assert.Contains(t, rules, tx.SettlementRule{
		ID:           "bgp-add-peer-settles-listener-ready",
		Operation:    tx.OperationSelector{Type: tx.OperationAddPeer, ResourceKind: tx.ResourcePeer},
		Readiness:    tx.ConfigOperationReadiness{Namespace: "bgp", EventType: "listener-ready"},
		ResourceFrom: tx.SettlementResourceAddress,
		Timeout:      10 * time.Second,
	})
}
