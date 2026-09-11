// Design: docs/architecture/config/apply-ordering.md -- phase 2, stop the binder whose address moves
// Related: operation.go -- the decomposer under test

package plugin

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/core/bgp/configop"

	// The real `interface` decomposer produces the address operations the
	// disturbed set is computed from. Reading it out of the registry, rather
	// than writing address operations by hand, is what keeps this test honest
	// about the shape the other root really emits.
	_ "github.com/ze-software/ze/internal/component/iface"
)

// The two trees the `interface` root holds on each side of a commit that moves
// one address from zdiag0 to zdiag1. Every other address stays where it is, so
// the commit disturbs 10.90.0.1 and nothing else.
const (
	ifaceRootAddressOnZdiag0 = `{"interface":{"backend":"netlink","dummy":{` +
		`"zdiag0":{"unit":{"0":{"ipv4":{"address":["10.80.0.1/24","10.90.0.1/24"]}}}},` +
		`"zdiag1":{"unit":{"0":{"ipv4":{"address":["10.91.0.1/24"]}}}}}}}`
	ifaceRootAddressOnZdiag1 = `{"interface":{"backend":"netlink","dummy":{` +
		`"zdiag0":{"unit":{"0":{"ipv4":{"address":["10.80.0.1/24"]}}}},` +
		`"zdiag1":{"unit":{"0":{"ipv4":{"address":["10.91.0.1/24","10.90.0.1/24"]}}}}}}}`
	ifaceAddressMoveDiff = `{"interface/dummy/zdiag0/unit/0/ipv4/address/1":{"old":"10.90.0.1/24","new":null},` +
		`"interface/dummy/zdiag1/unit/0/ipv4/address/1":{"old":null,"new":"10.90.0.1/24"}}`

	// One peer, bound to the address that moves. The bgp root is identical on
	// both sides of this commit: the operator edited the interface root alone.
	bgpRootPeerOn109001 = `{"bgp":{"session":{"asn":{"local":"65000"}},"peer":{"edge":` +
		`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"10.90.0.1"}},` +
		`"session":{"asn":{"remote":"65001"}}}}}}`
	bgpRootPeerOnAutoSource = `{"bgp":{"session":{"asn":{"local":"65000"}},"peer":{"edge":` +
		`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"auto"}},` +
		`"session":{"asn":{"remote":"65001"}}}}}}`
)

// decomposeIfaceRoot runs the registered `interface` decomposer over one
// commit and returns the address operations it owns.
func decomposeIfaceRoot(t *testing.T, active, candidate, changed string) []tx.ConfigOperation {
	t.Helper()
	decomposer, ok := tx.OperationDecomposerFor("interface")
	require.True(t, ok, "the interface root registers its decomposer in init()")
	ops, err := decomposer(context.Background(), tx.DecomposeRequest{
		TransactionID: "tx-phase2",
		Root:          "interface",
		ActiveRoot:    active,
		CandidateRoot: candidate,
		Diff:          tx.DiffSection{Root: "interface", Changed: changed},
	})
	require.NoError(t, err)
	return ops
}

// sortOperations orders one plan the way the executor applies it.
func sortOperations(t *testing.T, ops []tx.ConfigOperation) []string {
	t.Helper()
	graph, err := tx.BuildOperationGraph(ops, tx.ConstraintRules())
	require.NoError(t, err)
	sorted, err := tx.TopologicalSort(graph)
	require.NoError(t, err)
	ids := make([]string, 0, len(sorted))
	for i := range sorted {
		ids = append(ids, sorted[i].ID)
	}
	return ids
}

// TestBGPStopsThePeerBoundToAnAddressThatChangesInterface drives the commit the
// requirement is written for: one address keeps its value and moves to another
// interface, and the peer bound to it has no config change of its own.
//
// The `interface` decomposer emits the create and the destroy, the core reads
// the disturbed address out of them, and the bgp decomposer answers with the
// stop and the start of a session whose config bytes did not change. The
// derived edges then place all four: the session stops before its address
// leaves the host and starts after the new one arrives.
//
// VALIDATES: phase 2 and phase 5 for BGP -- remove-peer, remove-address, add-address, add-peer.
// PREVENTS: a session left holding a binding the commit destroyed, which is what
// happened while the decomposer emitted operations only for a peer whose own
// config changed (docs/architecture/config/apply-ordering.md).
func TestBGPStopsThePeerBoundToAnAddressThatChangesInterface(t *testing.T) {
	ifaceOps := decomposeIfaceRoot(t, ifaceRootAddressOnZdiag0, ifaceRootAddressOnZdiag1, ifaceAddressMoveDiff)
	require.Len(t, ifaceOps, 2, "one address moving is one create and one destroy; got %v", ifaceOps)

	disturbed := tx.DisturbedAddresses(ifaceOps)
	assert.Equal(t, []string{"10.90.0.1"}, disturbed,
		"the address the commit takes off zdiag0 is disturbed, and the two that stay put are not")

	bgpOps, err := decomposeBGPOperations(context.Background(), tx.DecomposeRequest{
		TransactionID:      "tx-phase2",
		Root:               configRootBGP,
		ActiveRoot:         bgpRootPeerOn109001,
		CandidateRoot:      bgpRootPeerOn109001,
		DisturbedAddresses: disturbed,
	})
	require.NoError(t, err)
	require.Len(t, bgpOps, 2, "the session bound to the moving address stops and starts; got %v", bgpOps)
	assert.Equal(t, configop.RemovePeer, bgpOps[0].Type)
	assert.Equal(t, tx.VerbDestroy, bgpOps[0].Verb)
	assert.Equal(t, configop.AddPeer, bgpOps[1].Type)
	assert.Equal(t, tx.VerbCreate, bgpOps[1].Verb)

	order := sortOperations(t, append(slices.Clone(ifaceOps), bgpOps...))
	require.Equal(t, []string{
		"bgp-remove-peer-edge",
		"interface-remove-address-zdiag0-10.90.0.1_24",
		"interface-add-address-zdiag1-10.90.0.1_24",
		"bgp-add-peer-edge",
	}, order, "the stop runs before the address leaves and the start after it arrives")
}

// TestBGPLeavesThePeerAloneWhenTheAddressRowIsIntact drives the case the owner
// named as the one that must NOT stop anything: an interface attribute change
// that leaves every address where it is.
//
// VALIDATES: an MTU edit disturbs no address, so no peer operation is emitted.
// PREVENTS: a fail-safe default so wide that every interface edit bounces every session.
func TestBGPLeavesThePeerAloneWhenTheAddressRowIsIntact(t *testing.T) {
	const (
		activeMTU    = `{"interface":{"backend":"netlink","dummy":{"zdiag0":{"mtu":"1500","unit":{"0":{"ipv4":{"address":["10.90.0.1/24"]}}}}}}}`
		candidateMTU = `{"interface":{"backend":"netlink","dummy":{"zdiag0":{"mtu":"9000","unit":{"0":{"ipv4":{"address":["10.90.0.1/24"]}}}}}}}`
		mtuDiff      = `{"interface/dummy/zdiag0/mtu":{"old":"1500","new":"9000"}}`
	)

	ifaceOps := decomposeIfaceRoot(t, activeMTU, candidateMTU, mtuDiff)
	disturbed := tx.DisturbedAddresses(ifaceOps)
	assert.Empty(t, disturbed, "an MTU edit leaves the address row intact, so it disturbs nothing")

	bgpOps, err := decomposeBGPOperations(context.Background(), tx.DecomposeRequest{
		TransactionID:      "tx-phase2-mtu",
		Root:               configRootBGP,
		ActiveRoot:         bgpRootPeerOn109001,
		CandidateRoot:      bgpRootPeerOn109001,
		DisturbedAddresses: disturbed,
	})
	require.NoError(t, err)
	assert.Empty(t, bgpOps, "nothing is disturbed and the peer did not change, so the session is left running")
}

// TestBGPStopsThePeerWhoseSourceAddressTheKernelPicks drives the fail-safe
// default. The peer's `connection.local.ip` is `auto`, so Ze cannot say which
// address the session holds, and the requirement answers that case with "be
// safe and deconf/reconf".
//
// The two operations declare every disturbed address in Consumes, which is
// what keeps them ordered: a peer that declared nothing would be stopped in
// the plan and placed anywhere in it.
//
// VALIDATES: a peer with no configured local address stops and starts when any address moves, ordered against the move.
// PREVENTS: a kernel-chosen source address silently surviving the removal of the address it was chosen from.
func TestBGPStopsThePeerWhoseSourceAddressTheKernelPicks(t *testing.T) {
	ifaceOps := decomposeIfaceRoot(t, ifaceRootAddressOnZdiag0, ifaceRootAddressOnZdiag1, ifaceAddressMoveDiff)
	disturbed := tx.DisturbedAddresses(ifaceOps)
	require.Equal(t, []string{"10.90.0.1"}, disturbed)

	bgpOps, err := decomposeBGPOperations(context.Background(), tx.DecomposeRequest{
		TransactionID:      "tx-phase2-auto",
		Root:               configRootBGP,
		ActiveRoot:         bgpRootPeerOnAutoSource,
		CandidateRoot:      bgpRootPeerOnAutoSource,
		DisturbedAddresses: disturbed,
	})
	require.NoError(t, err)
	require.Len(t, bgpOps, 2, "Ze cannot establish which address the kernel picked, so the session stops; got %v", bgpOps)
	assert.Equal(t, []tx.ResourceRef{{Kind: tx.ResourceAddress, Address: "10.90.0.1"}}, bgpOps[0].Consumes,
		"the peer could be sourced from any disturbed address, so it declares each one")

	order := sortOperations(t, append(slices.Clone(ifaceOps), bgpOps...))
	require.Equal(t, []string{
		"bgp-remove-peer-edge",
		"interface-remove-address-zdiag0-10.90.0.1_24",
		"interface-add-address-zdiag1-10.90.0.1_24",
		"bgp-add-peer-edge",
	}, order, "the declared addresses order the stop and the start around the move")
}
