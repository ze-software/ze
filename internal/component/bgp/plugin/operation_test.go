package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/configop"

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

	assert.Equal(t, configop.RemovePeer, ops[0].Type)
	assert.Equal(t, tx.VerbDestroy, ops[0].Verb, "the engine orders by the verb, so every emitted operation carries one")
	assert.Equal(t, "bgp", ops[0].Owner)
	assert.Equal(t, tx.ResourcePeer, ops[0].Target.Kind)
	assert.Equal(t, "edge", ops[0].Params.Peer)
	assert.Equal(t, "192.0.2.1", ops[0].Params.Address)
	assert.NotEmpty(t, ops[0].Params.OldConfig)

	assert.Equal(t, configop.AddPeer, ops[1].Type)
	assert.Equal(t, tx.VerbCreate, ops[1].Verb)
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

	assert.Equal(t, configop.ModifyPeer, ops[0].Type)
	assert.Equal(t, tx.VerbModify, ops[0].Verb)
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

	assert.Equal(t, configop.RemovePeer, ops[0].Type)
	assert.Equal(t, "peer1", ops[0].Params.Peer)
	assert.Equal(t, configop.RemovePeer, ops[1].Type)
	assert.Equal(t, "peer2", ops[1].Params.Peer)
	assert.Equal(t, configop.AddPeer, ops[2].Type)
	assert.Equal(t, "peer1", ops[2].Params.Peer)
	assert.Equal(t, configop.AddPeer, ops[3].Type)
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

// The `interface` root's address operations, as this test needs them to stand
// beside the peer operations in one graph. The labels are the interface root's
// own and this package no longer names them: the four rules that had to spell
// another root's vocabulary are deleted, and the ordering comes from the
// address these operations produce and the peer operations consume.
func testAddressOperation(id string, verb tx.OperationVerb, ifaceName, cidr string) tx.ConfigOperation {
	label := tx.ConfigOperationType("add-address")
	if verb == tx.VerbDestroy {
		label = "remove-address"
	}
	return tx.ConfigOperation{
		ID: id, Root: "interface", Owner: "interface", Type: label, Verb: verb,
		Target:   tx.ResourceRef{Kind: tx.ResourceAddress, Interface: ifaceName, Address: cidr},
		Produces: []tx.ResourceRef{{Kind: tx.ResourceAddress, Address: cidr}},
		Consumes: []tx.ResourceRef{{Kind: tx.ResourceInterface, Name: ifaceName}},
	}
}

// TestBGPOperationsDeclareConsumeAddress verifies that a peer operation
// declares the local address it binds, and that the declaration is what orders
// it against the address operations of the `interface` root.
//
// This replaces TestBGPConstraintRulesOrderPeerAgainstAddress, which asserted
// the same two edges over the two constraint rules this spec deleted. The
// edges are unchanged. What produces them is the pair of declarations, so this
// root registers no rule at all.
//
// VALIDATES: add-address -> add-peer and remove-peer -> remove-address, derived from Consumes.
// PREVENTS: a peer started before its local address exists, or an address removed under a live session.
func TestBGPOperationsDeclareConsumeAddress(t *testing.T) {
	addPeer := bgpPeerOperation(configop.AddPeer, "edge", "192.0.2.2", nil, nil, nil)
	removePeer := bgpPeerOperation(configop.RemovePeer, "edge-old", "192.0.2.1", nil, nil, nil)
	modifyPeer := bgpModifyPeerOperation("edge-same", "192.0.2.3", nil, nil, nil)

	for _, op := range []tx.ConfigOperation{addPeer, removePeer, modifyPeer} {
		assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceAddress, Address: op.Params.Address}}, op.Consumes,
			"%s binds a local address, so it declares it", op.ID)
		assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourcePeer, Peer: op.Params.Peer}}, op.Produces,
			"%s owns the session it names", op.ID)
	}

	assert.Nil(t, bgpPeerOperation(configop.AddPeer, "auto", "", nil, nil, nil).Consumes,
		"a peer that lets the kernel pick its source address waits for no address, and declares no entry rather than a blank one")

	ops := []tx.ConfigOperation{
		testAddressOperation("addr-add", tx.VerbCreate, "dum0", "192.0.2.2/32"),
		addPeer,
		removePeer,
		testAddressOperation("addr-remove", tx.VerbDestroy, "dum0", "192.0.2.1/32"),
	}

	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)
	assert.True(t, graph.HasEdge("addr-add", addPeer.ID), "new local address must exist before adding peer")
	assert.True(t, graph.HasEdge(removePeer.ID, "addr-remove"), "peer must be removed before deleting its local address")
}

// TestBGPOperationsOrderAgainstAnAddressOnAnyInterface verifies that a peer is
// ordered against its address wherever that address sits, because an address is
// identified by its IP and never by the device carrying it.
//
// VALIDATES: the derived edge holds when the address moves to another interface.
// PREVENTS: an identity that includes the interface, which no peer knows.
func TestBGPOperationsOrderAgainstAnAddressOnAnyInterface(t *testing.T) {
	addPeer := bgpPeerOperation(configop.AddPeer, "edge", "192.0.2.2", nil, nil, nil)

	ops := []tx.ConfigOperation{
		testAddressOperation("addr-add-elsewhere", tx.VerbCreate, "dum7", "192.0.2.2/24"),
		addPeer,
	}

	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)
	assert.True(t, graph.HasEdge("addr-add-elsewhere", addPeer.ID),
		"the peer binds 192.0.2.2 without knowing which device holds it, and the prefix length is no part of its name")
}

// TestBGPSettlementRulesWaitForListenerReady verifies BGP registers the
// readiness event needed after adding a local-address peer.
//
// VALIDATES: ADD_PEER operations wait for bgp/listener-ready settlement.
// PREVENTS: Removing old addresses before the replacement BGP listener is observable.
func TestBGPSettlementRulesWaitForListenerReady(t *testing.T) {
	op := tx.ConfigOperation{
		ID:     "peer-add",
		Type:   configop.AddPeer,
		Target: tx.ResourceRef{Kind: tx.ResourcePeer, Peer: "edge"},
		Params: tx.ConfigOperationParams{Address: "192.0.2.2"},
	}

	rules := tx.SettlementRulesFor(&op)
	require.NotEmpty(t, rules)
	assert.Contains(t, rules, tx.SettlementRule{
		ID:           "bgp-add-peer-settles-listener-ready",
		Operation:    tx.OperationSelector{Type: configop.AddPeer, ResourceKind: tx.ResourcePeer},
		Readiness:    tx.ConfigOperationReadiness{Namespace: "bgp", EventType: "listener-ready"},
		ResourceFrom: tx.SettlementResourceAddress,
		Timeout:      10 * time.Second,
	})
}
