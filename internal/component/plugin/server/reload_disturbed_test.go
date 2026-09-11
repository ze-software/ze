// Design: docs/architecture/config/apply-ordering.md -- phase 2, stop the binder whose address moves
// Related: reload_tx.go -- the planner under test
// Related: reload.go -- appendDecomposingPlugins, which puts the binder in the transaction

package server

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The operation labels this test's two roots own. Each root spells its own,
// the way `interface` and `bgp` spell theirs: the engine orders by the verb
// and the resource kind and reads no label.
const (
	testOpMoveAddressOff rpc.ConfigOperationType = "test-remove-address"
	testOpMoveAddressOn  rpc.ConfigOperationType = "test-add-address"
	testOpStopBinder     rpc.ConfigOperationType = "test-stop-binder"
	testOpStartBinder    rpc.ConfigOperationType = "test-start-binder"
)

// testMovedAddress is the address the address root takes off one interface and
// puts on another, and the address the binder root binds.
const testMovedAddress = "198.51.100.7"

// TestReloadStopsABinderWhoseAddressMovesAndItsOwnConfigDidNot drives the
// requirement's phase 2 from the reload entry point, over a root that has no
// diff at all.
//
// The address root changes, and the binder root does not. Under the ordering
// as it stood, a root with no diff was never asked for operations and its
// plugin was not even a participant, so the binder kept running while the
// address it binds was removed and added underneath it.
//
// VALIDATES: the planner computes the disturbed address from the first pass,
// asks every root that decomposes in a second pass, and the graph places the
// binder's stop before the address destroy and its start after the create.
// PREVENTS: a binder left holding a binding the commit destroyed, and a
// binder that cannot be reached at all because its plugin joined no
// transaction.
func TestReloadStopsABinderWhoseAddressMovesAndItsOwnConfigDidNot(t *testing.T) {
	stamp := time.Now().UnixNano()
	addressRoot := fmt.Sprintf("oproot-address-%d", stamp)
	binderRoot := fmt.Sprintf("oproot-binder-%d", stamp)
	addressOwner := "addressowner"
	binderOwner := "binderowner"

	require.NoError(t, transaction.RegisterOperationDecomposer(addressRoot, func(_ context.Context, req transaction.DecomposeRequest) ([]transaction.ConfigOperation, error) {
		return []transaction.ConfigOperation{
			testAddressOperation("addr-remove", addressRoot, addressOwner, testOpMoveAddressOff, transaction.VerbDestroy, "zt0"),
			testAddressOperation("addr-add", addressRoot, addressOwner, testOpMoveAddressOn, transaction.VerbCreate, "zt1"),
		}, nil
	}))

	var seenDisturbed []string
	require.NoError(t, transaction.RegisterOperationDecomposer(binderRoot, func(_ context.Context, req transaction.DecomposeRequest) ([]transaction.ConfigOperation, error) {
		seenDisturbed = slices.Clone(req.DisturbedAddresses)
		if !slices.Contains(req.DisturbedAddresses, testMovedAddress) {
			return nil, nil
		}
		return []transaction.ConfigOperation{
			testBinderOperation("binder-stop", binderRoot, binderOwner, testOpStopBinder, transaction.VerbDestroy),
			testBinderOperation("binder-start", binderRoot, binderOwner, testOpStartBinder, transaction.VerbCreate),
		}, nil
	}))

	// The binder root is identical on both sides. Only the address root moves.
	oldTree := map[string]any{
		addressRoot: map[string]any{"holder": "zt0"},
		binderRoot:  map[string]any{"binds": testMovedAddress},
	}
	newTree := map[string]any{
		addressRoot: map[string]any{"holder": "zt1"},
		binderRoot:  map[string]any{"binds": testMovedAddress},
	}

	order := &orderRecorder{}
	reactor := &mockReloadReactor{tree: oldTree}
	plugins := []pluginDef{
		{
			name:  addressOwner,
			roots: []string{addressRoot},
			order: order,
			configOps: []rpc.ConfigOperationDecl{{
				Root:       addressRoot,
				Decompose:  true,
				Operations: []rpc.ConfigOperationType{testOpMoveAddressOff, testOpMoveAddressOn},
			}},
		},
		{
			name:  binderOwner,
			roots: []string{binderRoot},
			order: order,
			configOps: []rpc.ConfigOperationDecl{{
				Root:       binderRoot,
				Decompose:  true,
				Operations: []rpc.ConfigOperationType{testOpStopBinder, testOpStartBinder},
			}},
		},
	}
	s := newTestReloadServer(t, reactor, plugins)

	require.NoError(t, s.ReloadConfig(context.Background(), newTree))
	require.Eventually(t, func() bool { return len(order.snapshot()) == 4 }, 2*time.Second, 10*time.Millisecond,
		"four operations apply: the stop, the two address halves and the start; got %v", order.snapshot())

	assert.Equal(t, []string{testMovedAddress}, seenDisturbed,
		"the binder is told which addresses this commit takes off the host")

	applied := order.snapshot()
	stop := slices.Index(applied, "binder-stop")
	start := slices.Index(applied, "binder-start")
	remove := slices.Index(applied, "addr-remove")
	add := slices.Index(applied, "addr-add")
	require.NotEqual(t, -1, stop, "the binder was asked for operations; got %v", applied)
	assert.Less(t, stop, remove, "the binder stops before its address leaves; got %v", applied)
	assert.Less(t, add, start, "the binder starts after the address arrives; got %v", applied)
}

// TestReloadAsksNoUnchangedRootWhenNoAddressMoves is the other half of the
// rule above: a commit that disturbs no address costs the binder nothing. Its
// decomposer is not called, its plugin receives no operation, and the planner
// makes one pass.
//
// VALIDATES: a root with no diff is asked only when an address is disturbed.
// PREVENTS: every reload asking every root that decomposes, which turns a
// one-line MTU edit into a decompose round trip for each of them.
func TestReloadAsksNoUnchangedRootWhenNoAddressMoves(t *testing.T) {
	stamp := time.Now().UnixNano()
	changedRoot := fmt.Sprintf("oproot-quiet-%d", stamp)
	binderRoot := fmt.Sprintf("oproot-quiet-binder-%d", stamp)
	changedOwner := "quietowner"
	binderOwner := "quietbinder"

	require.NoError(t, transaction.RegisterOperationDecomposer(changedRoot, func(context.Context, transaction.DecomposeRequest) ([]transaction.ConfigOperation, error) {
		return []transaction.ConfigOperation{{
			ID:       "quiet-modify",
			Root:     changedRoot,
			Owner:    changedOwner,
			Type:     testOpSetProperty,
			Verb:     transaction.VerbModify,
			Target:   transaction.ResourceRef{Kind: testResourceSysctl, Name: "mtu"},
			Produces: []transaction.ResourceRef{{Kind: testResourceSysctl, Name: "mtu"}},
		}}, nil
	}))

	binderCalls := 0
	require.NoError(t, transaction.RegisterOperationDecomposer(binderRoot, func(context.Context, transaction.DecomposeRequest) ([]transaction.ConfigOperation, error) {
		binderCalls++
		return nil, nil
	}))

	oldTree := map[string]any{
		changedRoot: map[string]any{"mtu": "1500"},
		binderRoot:  map[string]any{"binds": testMovedAddress},
	}
	newTree := map[string]any{
		changedRoot: map[string]any{"mtu": "9000"},
		binderRoot:  map[string]any{"binds": testMovedAddress},
	}

	reactor := &mockReloadReactor{tree: oldTree}
	plugins := []pluginDef{
		{
			name:  changedOwner,
			roots: []string{changedRoot},
			configOps: []rpc.ConfigOperationDecl{{
				Root:       changedRoot,
				Decompose:  true,
				Operations: []rpc.ConfigOperationType{testOpSetProperty},
			}},
		},
		{
			name:  binderOwner,
			roots: []string{binderRoot},
			configOps: []rpc.ConfigOperationDecl{{
				Root:       binderRoot,
				Decompose:  true,
				Operations: []rpc.ConfigOperationType{testOpStopBinder, testOpStartBinder},
			}},
		},
	}
	s := newTestReloadServer(t, reactor, plugins)

	require.NoError(t, s.ReloadConfig(context.Background(), newTree))
	require.Eventually(t, func() bool { return plugins[0].responder.getOperationApplyCalls() == 1 }, 2*time.Second, 10*time.Millisecond)

	assert.Zero(t, binderCalls, "no address moved, so the root with no diff has nothing to answer for")
	assert.Zero(t, plugins[1].responder.getOperationApplyCalls(), "the binder receives no operation")
	assert.Zero(t, plugins[1].responder.getApplyCalls(), "and no section apply either: it has no diff")
}

// testAddressOperation is one half of an address move: the destroy that takes
// the address off one interface, or the create that puts it on another. It is
// shaped the way the `interface` root shapes its own, because the disturbed
// set is read out of exactly these two declarations.
func testAddressOperation(id, root, owner string, label rpc.ConfigOperationType, verb transaction.OperationVerb, ifaceName string) transaction.ConfigOperation {
	return transaction.ConfigOperation{
		ID:       id,
		Root:     root,
		Owner:    owner,
		Type:     label,
		Verb:     verb,
		Target:   transaction.ResourceRef{Kind: transaction.ResourceAddress, Interface: ifaceName, Address: testMovedAddress + "/24"},
		Produces: []transaction.ResourceRef{{Kind: transaction.ResourceAddress, Address: testMovedAddress + "/24"}},
		Consumes: []transaction.ResourceRef{{Kind: transaction.ResourceInterface, Name: ifaceName}},
	}
}

// testBinderOperation is one half of a binder's stop and start. It declares
// the address it binds, which is the only thing that orders it against the
// move.
func testBinderOperation(id, root, owner string, label rpc.ConfigOperationType, verb transaction.OperationVerb) transaction.ConfigOperation {
	return transaction.ConfigOperation{
		ID:       id,
		Root:     root,
		Owner:    owner,
		Type:     label,
		Verb:     verb,
		Target:   transaction.ResourceRef{Kind: transaction.ResourceListener, Address: testMovedAddress, Port: 179},
		Produces: []transaction.ResourceRef{{Kind: transaction.ResourceListener, Address: testMovedAddress, Port: 179}},
		Consumes: []transaction.ResourceRef{{Kind: transaction.ResourceAddress, Address: testMovedAddress}},
	}
}
