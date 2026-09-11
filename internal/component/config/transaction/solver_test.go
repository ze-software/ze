package transaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTopologicalSort verifies that the solver returns operations in an order
// that respects the graph edges, here the ones derived from what each
// operation produces and consumes.
//
// VALIDATES: Topological sort respects add-interface -> add-address -> add-peer.
// PREVENTS: Executor receiving operation order based on input slice order.
func TestTopologicalSort(t *testing.T) {
	t.Parallel()

	graph, err := BuildOperationGraph([]ConfigOperation{
		{ID: "peer-add", Type: testOpAddPeer, Verb: VerbCreate, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1"},
			Consumes: []ResourceRef{{Kind: ResourceAddress, Address: "192.0.2.1"}},
			Params:   ConfigOperationParams{Address: "192.0.2.1"}},
		{ID: "addr-add", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1"},
			Produces: []ResourceRef{{Kind: ResourceAddress, Address: "192.0.2.1"}},
			Consumes: []ResourceRef{{Kind: ResourceInterface, Name: "eth0"}}},
		{ID: "iface-add", Type: testOpAddInterface, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"},
			Produces: []ResourceRef{{Kind: ResourceInterface, Name: "eth0"}}},
	}, nil)
	require.NoError(t, err)

	sorted, err := TopologicalSort(graph)
	require.NoError(t, err)
	assert.Equal(t, []string{"iface-add", "addr-add", "peer-add"}, operationIDs(sorted))
}

// TestTopologicalSortCycle verifies that irreconcilable cycles are rejected by
// the solver instead of producing a partial or arbitrary order.
//
// VALIDATES: Cycles in the operation graph return ErrOperationCycle.
// PREVENTS: Executor applying operations when dependencies are impossible.
func TestTopologicalSortCycle(t *testing.T) {
	t.Parallel()

	graph, err := BuildOperationGraph([]ConfigOperation{
		{ID: "addr-add", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1"}},
		{ID: "addr-remove", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1"}},
	}, []ConstraintRule{
		{ID: "test-add-before-remove", Before: OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationAny},
		{ID: "test-remove-before-add", Before: OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress}, After: OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress}, Relation: ResourceRelationAny},
	})
	require.NoError(t, err)

	_, err = TopologicalSort(graph)
	require.ErrorIs(t, err, ErrOperationCycle)
}

// TestTopologicalSortSwapsAddressesBreakBeforeMake verifies that two
// interfaces trading addresses sort with every removal before every addition,
// and with no cycle to break.
//
// It replaces TestTopologicalSortCycleResolution, which asserted the opposite
// policy: that the swap was a cycle, that the solver relaxed it by dropping
// the cross-interface edges, and that the address creations came back marked
// AllowDual so both addresses sat on the host at once. That was
// make-before-break. The requirement stops the binder, removes the disturbed
// address and adds it again (docs/architecture/config/apply-ordering.md).
//
// VALIDATES: phases 3 and 4. An address leaves before the same address arrives
// on another interface, and no window holds both.
// PREVENTS: the relaxation coming back, which it would as a silent reordering
// rather than as an error: a graph that no longer closes a cycle would take
// the relaxed branch for nothing and still leave both addresses present.
func TestTopologicalSortSwapsAddressesBreakBeforeMake(t *testing.T) {
	t.Parallel()

	// ethA currently holds 10.0.0.1 and ethB holds 10.0.0.2. The candidate
	// gives each address to the other interface.
	ops := []ConfigOperation{
		{ID: "add-A-2", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.2/32"}},
		{ID: "add-B-1", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.1/32"}},
		{ID: "remove-B-2", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.2/32"}},
		{ID: "remove-A-1", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
	}

	graph, err := BuildOperationGraph(ops, survivingAddressRule())
	require.NoError(t, err)

	sorted, err := TopologicalSort(graph)
	require.NoError(t, err, "a swap carries one destroy and one create for each address, and no cycle")
	assert.Equal(t, []string{"remove-B-2", "remove-A-1", "add-A-2", "add-B-1"}, operationIDs(sorted))
}

// TestTopologicalSortRotatesAddressesBreakBeforeMake verifies the three-way
// rotation, which was the other cycle the relaxation existed for.
//
// VALIDATES: phases 3 and 4 over three interfaces.
// PREVENTS: a rotation rejected as an unrelaxable cycle, which is what a rule
// ordering an addition ahead of a removal would make it again.
func TestTopologicalSortRotatesAddressesBreakBeforeMake(t *testing.T) {
	t.Parallel()

	// A:1->2, B:2->3, C:3->1
	ops := []ConfigOperation{
		{ID: "add-A-2", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.2/32"}},
		{ID: "add-B-3", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.3/32"}},
		{ID: "add-C-1", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethC", Address: "10.0.0.1/32"}},
		{ID: "remove-A-1", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
		{ID: "remove-B-2", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.2/32"}},
		{ID: "remove-C-3", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethC", Address: "10.0.0.3/32"}},
	}

	graph, err := BuildOperationGraph(ops, survivingAddressRule())
	require.NoError(t, err)

	sorted, err := TopologicalSort(graph)
	require.NoError(t, err)
	assertRemovalsBeforeAdditions(t, sorted)
}

// survivingAddressRule is the one constraint rule the `interface` root still
// registers, under the test package's own operation labels: every address
// removal runs before every address addition, which is phase 3 before phase 4.
func survivingAddressRule() []ConstraintRule {
	return []ConstraintRule{{
		ID:       "iface-remove-address-before-add-address",
		Before:   OperationSelector{Type: testOpRemoveAddress, ResourceKind: ResourceAddress},
		After:    OperationSelector{Type: testOpAddAddress, ResourceKind: ResourceAddress},
		Relation: ResourceRelationAny,
	}}
}

// assertRemovalsBeforeAdditions reads the sorted order back as the two phases
// it must hold: no address is added before the last address is removed.
func assertRemovalsBeforeAdditions(t *testing.T, sorted []ConfigOperation) {
	t.Helper()

	firstAdd := len(sorted)
	lastRemove := -1
	for i := range sorted {
		switch sorted[i].Verb {
		case VerbCreate:
			if i < firstAdd {
				firstAdd = i
			}
		case VerbDestroy:
			lastRemove = i
		case VerbModify:
		}
	}
	assert.Less(t, lastRemove, firstAdd, "every removal runs before every addition; got %v", operationIDs(sorted))
}

// TestTopologicalSortNonAddressCycleFails verifies that cycles involving
// non-address operations are not relaxable and still return ErrOperationCycle.
//
// VALIDATES: Non-address cycles are rejected, not silently relaxed.
// PREVENTS: Dual-presence fallback applied to non-address resource types.
func TestTopologicalSortNonAddressCycleFails(t *testing.T) {
	t.Parallel()

	ops := []ConfigOperation{
		{ID: "add-peer", Type: testOpAddPeer, Verb: VerbCreate, Target: ResourceRef{Kind: ResourcePeer, Peer: "edge"}},
		{ID: "remove-peer", Type: testOpRemovePeer, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourcePeer, Peer: "edge"}},
	}
	rules := []ConstraintRule{
		{ID: "add-before-remove", Before: OperationSelector{Type: testOpAddPeer, ResourceKind: ResourcePeer}, After: OperationSelector{Type: testOpRemovePeer, ResourceKind: ResourcePeer}, Relation: ResourceRelationAny},
		{ID: "remove-before-add", Before: OperationSelector{Type: testOpRemovePeer, ResourceKind: ResourcePeer}, After: OperationSelector{Type: testOpAddPeer, ResourceKind: ResourcePeer}, Relation: ResourceRelationAny},
	}

	graph, err := BuildOperationGraph(ops, rules)
	require.NoError(t, err)

	_, err = TopologicalSort(graph)
	require.ErrorIs(t, err, ErrOperationCycle)
}

func operationIDs(ops []ConfigOperation) []string {
	ids := make([]string, 0, len(ops))
	for i := range ops {
		ids = append(ids, ops[i].ID)
	}
	return ids
}

// TestTopologicalSortOrdersASwapWhoseLabelsItDoesNotKnow verifies the swap
// ordering decides on the verb plus the target resource kind, and never on the
// operation label. The operations below carry labels no package in this
// repository names, which is what a root that owns its own vocabulary emits.
//
// VALIDATES: AC-5. The solver orders an operation whose label it does not know.
// PREVENTS: the ordering working for the two roots whose labels the solver was
// written against, and leaving every other root's swap in input order.
func TestTopologicalSortOrdersASwapWhoseLabelsItDoesNotKnow(t *testing.T) {
	t.Parallel()

	const (
		bindVIP    ConfigOperationType = "provision-bind-vip"
		releaseVIP ConfigOperationType = "provision-release-vip"
	)

	// The same swap as TestTopologicalSortSwapsAddressesBreakBeforeMake:
	// ethA and ethB exchange 10.0.0.1 and 10.0.0.2.
	ops := []ConfigOperation{
		{ID: "bind-A-2", Type: bindVIP, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.2/32"}},
		{ID: "bind-B-1", Type: bindVIP, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.1/32"}},
		{ID: "release-B-2", Type: releaseVIP, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.2/32"}},
		{ID: "release-A-1", Type: releaseVIP, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
	}
	rules := []ConstraintRule{
		{ID: "release-before-bind", Before: OperationSelector{Type: releaseVIP, ResourceKind: ResourceAddress}, After: OperationSelector{Type: bindVIP, ResourceKind: ResourceAddress}, Relation: ResourceRelationAny},
	}

	graph, err := BuildOperationGraph(ops, rules)
	require.NoError(t, err)

	sorted, err := TopologicalSort(graph)
	require.NoError(t, err, "a swap of addresses orders whatever the operations are labeled")
	require.Len(t, sorted, 4)
	assertRemovalsBeforeAdditions(t, sorted)
}

// TestTopologicalSortRejectsEveryCycle holds the rejection over the verb and
// kind vocabulary. A cycle whose members modify an address, and a cycle over a
// kind this package does not name, are both refused, and so is every other
// shape: nothing relaxes a cycle.
//
// VALIDATES: "Every cycle is rejected" (docs/architecture/config/apply-ordering.md).
// PREVENTS: a relaxation coming back for the kinds somebody happens to know,
// which is how the deleted one applied two conflicting operations at once.
func TestTopologicalSortRejectsEveryCycle(t *testing.T) {
	t.Parallel()

	const (
		claim   ConfigOperationType = "provision-claim"
		release ConfigOperationType = "provision-release"
		retune  ConfigOperationType = "provision-retune"
	)

	cases := []struct {
		name string
		ops  []ConfigOperation
	}{
		{
			name: "address kind but neither member creates or destroys",
			ops: []ConfigOperation{
				{ID: "retune-1", Type: retune, Verb: VerbModify, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
				{ID: "retune-2", Type: retune, Verb: VerbModify, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.1/32"}},
			},
		},
		{
			name: "create and destroy but not an address",
			ops: []ConfigOperation{
				{ID: "claim-1", Type: claim, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceKind("vlan"), Interface: "ethA", Name: "v10"}},
				{ID: "release-1", Type: release, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceKind("vlan"), Interface: "ethB", Name: "v10"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			first := tc.ops[0].Type
			second := tc.ops[1].Type
			rules := []ConstraintRule{
				{ID: "first-before-second", Before: OperationSelector{Type: first, ResourceKind: tc.ops[0].Target.Kind}, After: OperationSelector{Type: second, ResourceKind: tc.ops[1].Target.Kind}, Relation: ResourceRelationAny},
				{ID: "second-before-first", Before: OperationSelector{Type: second, ResourceKind: tc.ops[1].Target.Kind}, After: OperationSelector{Type: first, ResourceKind: tc.ops[0].Target.Kind}, Relation: ResourceRelationAny},
			}

			graph, err := BuildOperationGraph(tc.ops, rules)
			require.NoError(t, err)

			_, err = TopologicalSort(graph)
			require.ErrorIs(t, err, ErrOperationCycle)
		})
	}
}

// TestTopologicalSortPlacesSectionNodeBetweenAddressingAndBinders reads the
// position the solver gives a coarse section-apply node. The node carries no
// verb, no target and no produce or consume set, so it earns no edge from
// either mechanism. Its place in the order is a decision the solver takes,
// rather than a constraint the graph states.
//
// The decision is the requirement's own sequence: stop what binds, remove the
// disturbed addresses, add them back, then start what binds them. The core
// cannot tell whether a root nobody decomposes binds an address, so the
// fail-safe default reads it as one, which puts it after the additions and
// before the starts.
//
// VALIDATES: phases 4 and 5. A coarse node runs after the addressing this
// commit adds and before the first operation that binds it.
// PREVENTS: a static route installed before the address it binds exists, which
// is what the slice tie-break produced (the kernel answers "network is
// unreachable" and the route is lost while the transaction reports committed),
// and a peer started before the plugins that configure its RIB have applied.
func TestTopologicalSortPlacesSectionNodeBetweenAddressingAndBinders(t *testing.T) {
	t.Parallel()

	section := ConfigOperation{ID: "section-apply-static", Owner: "static", Type: OperationSectionApply}

	cases := []struct {
		name  string
		ops   []ConfigOperation
		rules []ConstraintRule
		want  []string
	}{
		{
			name: "after every addressing addition",
			ops: []ConfigOperation{
				{ID: "iface-add-zx", Type: testOpAddInterface, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceInterface, Name: "zx"},
					Produces: []ResourceRef{{Kind: ResourceInterface, Name: "zx"}}},
				{ID: "iface-add-address-zx", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "zx", Address: "10.93.0.1/24"},
					Produces: []ResourceRef{{Kind: ResourceAddress, Address: "10.93.0.1/24"}},
					Consumes: []ResourceRef{{Kind: ResourceInterface, Name: "zx"}}},
				section,
			},
			want: []string{"iface-add-zx", "iface-add-address-zx", "section-apply-static"},
		},
		{
			name: "after the destructions, which are phases 1 to 3",
			ops: []ConfigOperation{
				{ID: "iface-remove-zold", Type: testOpRemoveInterface, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceInterface, Name: "zold"},
					Produces: []ResourceRef{{Kind: ResourceInterface, Name: "zold"}}},
				section,
			},
			want: []string{"iface-remove-zold", "section-apply-static"},
		},
		{
			name: "after the renumber, which removes before it adds",
			ops: []ConfigOperation{
				{ID: "iface-add-address-new", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "zmix0", Address: "10.93.1.1/24"},
					Produces: []ResourceRef{{Kind: ResourceAddress, Address: "10.93.1.1/24"}}},
				{ID: "iface-remove-address-old", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "zmix0", Address: "10.93.0.1/24"},
					Produces: []ResourceRef{{Kind: ResourceAddress, Address: "10.93.0.1/24"}}},
				section,
			},
			rules: survivingAddressRule(),
			want:  []string{"iface-remove-address-old", "iface-add-address-new", "section-apply-static"},
		},
		{
			name: "before the peer that binds the address, and after the address",
			ops: []ConfigOperation{
				{ID: "bgp-add-peer", Type: testOpAddPeer, Verb: VerbCreate, Target: ResourceRef{Kind: ResourcePeer, Peer: "edge"},
					Produces: []ResourceRef{{Kind: ResourcePeer, Peer: "edge"}},
					Consumes: []ResourceRef{{Kind: ResourceAddress, Address: "10.93.0.1/24"}}},
				{ID: "iface-add-address", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "zmix0", Address: "10.93.0.1/24"},
					Produces: []ResourceRef{{Kind: ResourceAddress, Address: "10.93.0.1/24"}}},
				section,
			},
			want: []string{"iface-add-address", "section-apply-static", "bgp-add-peer"},
		},
		{
			// `interface` applies everything it has no create primitive for
			// with one modify that declares no target resource, and a tunnel,
			// a wireguard device or an xfrm device and the addresses on it
			// arrive through it (ifaceConfigureOperation). The engine cannot
			// read what it does, so the fail-safe reads it as addressing and
			// the coarse sections wait for it.
			name: "after an operation whose kind the engine cannot read",
			ops: []ConfigOperation{
				{ID: "iface-remove-address-old", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "zmix0", Address: "10.93.0.1/24"},
					Produces: []ResourceRef{{Kind: ResourceAddress, Address: "10.93.0.1/24"}}},
				{ID: "iface-configure", Type: testOpSetProperty, Verb: VerbModify},
				section,
			},
			want: []string{"iface-remove-address-old", "iface-configure", "section-apply-static"},
		},
		{
			name: "before a peer whose address this commit does not touch",
			ops: []ConfigOperation{
				{ID: "bgp-modify-peer", Type: testOpAddPeer, Verb: VerbModify, Target: ResourceRef{Kind: ResourcePeer, Peer: "edge"},
					Produces: []ResourceRef{{Kind: ResourcePeer, Peer: "edge"}}},
				section,
			},
			want: []string{"section-apply-static", "bgp-modify-peer"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			graph, err := BuildOperationGraph(tc.ops, tc.rules)
			require.NoError(t, err)

			sorted, err := TopologicalSort(graph)
			require.NoError(t, err)
			assert.Equal(t, tc.want, operationIDs(sorted))
		})
	}
}

// TestTopologicalSortSectionNodeJoinsAnAddressSwapWithoutACycle holds the
// coarse node's position to a placement and away from an edge. A coarse node
// stands for a whole participant section, so it names no resource and nothing
// in the graph can state where it goes. An edge invented for it would join
// cycles the operator never wrote. The reload below is ordinary: two
// interfaces swap addresses while one uncovered root has a diff.
//
// VALIDATES: R-2. Total coverage aborts no reload that worked before it.
// PREVENTS: a swap reload answering "operation dependency cycle" because the
// core's own synthesized node joined a cycle the operator never wrote.
func TestTopologicalSortSectionNodeJoinsAnAddressSwapWithoutACycle(t *testing.T) {
	t.Parallel()

	ops := []ConfigOperation{
		{ID: "add-A-2", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.2/32"}},
		{ID: "add-B-1", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.1/32"}},
		{ID: "remove-B-2", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethB", Address: "10.0.0.2/32"}},
		{ID: "remove-A-1", Type: testOpRemoveAddress, Verb: VerbDestroy, Target: ResourceRef{Kind: ResourceAddress, Interface: "ethA", Address: "10.0.0.1/32"}},
		{ID: "section-apply-firewall", Owner: "firewall", Type: OperationSectionApply},
	}

	graph, err := BuildOperationGraph(ops, survivingAddressRule())
	require.NoError(t, err)

	sorted, err := TopologicalSort(graph)
	require.NoError(t, err)
	assert.Equal(t, []string{"remove-B-2", "remove-A-1", "add-A-2", "add-B-1", "section-apply-firewall"}, operationIDs(sorted))
}
