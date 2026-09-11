// Design: docs/architecture/config/apply-ordering.md -- phase 2, stop the binder whose address moves
// Related: reload_tx.go -- the planner under test
// Related: internal/component/iface/operation.go -- the decomposer these tests drive

package server

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The `interface` root's own operation labels, spelled here because this
// package cannot reach the constants that declare them. A rename there fails
// this test loudly: the planner refuses an operation its emitter did not
// declare (validateOperationDeclarations), so the reload aborts.
const (
	ifaceOpAddInterface    rpc.ConfigOperationType = "add-interface"
	ifaceOpRemoveInterface rpc.ConfigOperationType = "remove-interface"
	ifaceOpAddAddress      rpc.ConfigOperationType = "add-address"
	ifaceOpRemoveAddress   rpc.ConfigOperationType = "remove-address"
	ifaceOpConfigure       rpc.ConfigOperationType = "configure-interfaces"
)

// The operation ids the `interface` decomposer builds for the address these
// tests move, and for the operation that applies the rest of the section.
const (
	ifaceIDAddAddress    = "interface-add-address-dum1-10.0.0.1_24"
	ifaceIDRemoveAddress = "interface-remove-address-dum0-10.0.0.1_24"
	ifaceIDConfigure     = "interface-configure"
)

// ifaceMixedAddress is the address the commit moves from dum0 to dum1, and the
// address the binder root binds.
const ifaceMixedAddress = "10.0.0.1"

// The binder root of this file. One root serves all three tests, and its
// decomposer is registered once: a decomposer stays registered for the life of
// the test binary, and the planner asks EVERY registered root once an address
// is disturbed. A root registered per test would therefore answer inside the
// next test, where its owner is not a participant and the planner refuses the
// operation it emits.
const (
	ifaceMixedBinderRoot  = "oproot-iface-mixed-binder"
	ifaceMixedBinderOwner = "ifacemixedbinder"

	// The readiness event the `interface` root settles an address create on.
	ifaceEventAddrAdded = "addr-added"
)

var (
	ifaceMixedBinderOnce sync.Once
	ifaceMixedBinderMu   sync.Mutex
	ifaceMixedBinderSeen []string
)

// ifaceMixedTrees returns the running and candidate trees for one commit
// against the real `interface` root, beside a binder root that binds the
// address and never changes.
//
// mtu is the value dum0 carries in the candidate tree. Pass the running value
// to change nothing but the address, and a new value for the mixed commit this
// file is named for.
func ifaceMixedTrees(runningMTU, candidateMTU string, moveAddress bool) (running, candidate map[string]any) {
	address := map[string]any{"unit": map[string]any{"default": map[string]any{"ipv4": map[string]any{"address": ifaceMixedAddress + "/24"}}}}
	running = map[string]any{
		"interface": map[string]any{
			"backend": "test",
			"dummy": map[string]any{
				"dum0": map[string]any{"mtu": runningMTU, "unit": address["unit"]},
				"dum1": map[string]any{},
			},
		},
		ifaceMixedBinderRoot: map[string]any{"binds": ifaceMixedAddress},
	}
	candidateDum0 := map[string]any{"mtu": candidateMTU, "unit": address["unit"]}
	candidateDum1 := map[string]any{}
	if moveAddress {
		candidateDum0 = map[string]any{"mtu": candidateMTU}
		candidateDum1 = map[string]any{"unit": address["unit"]}
	}
	candidate = map[string]any{
		"interface": map[string]any{
			"backend": "test",
			"dummy": map[string]any{
				"dum0": candidateDum0,
				"dum1": candidateDum1,
			},
		},
		ifaceMixedBinderRoot: map[string]any{"binds": ifaceMixedAddress},
	}
	return running, candidate
}

// registerMixedBinder registers, once for this file, a binder root that stops
// and starts itself whenever the commit takes the address it binds off the
// host. It clears what the last test saw and returns the addresses this one is
// told about.
func registerMixedBinder(t *testing.T) func() []string {
	t.Helper()
	ifaceMixedBinderOnce.Do(func() {
		require.NoError(t, transaction.RegisterOperationDecomposer(ifaceMixedBinderRoot, func(_ context.Context, req transaction.DecomposeRequest) ([]transaction.ConfigOperation, error) {
			ifaceMixedBinderMu.Lock()
			ifaceMixedBinderSeen = slices.Clone(req.DisturbedAddresses)
			ifaceMixedBinderMu.Unlock()
			if !slices.Contains(req.DisturbedAddresses, ifaceMixedAddress) {
				return nil, nil
			}
			return []transaction.ConfigOperation{
				mixedBinderOperation("binder-stop", testOpStopBinder, transaction.VerbDestroy),
				mixedBinderOperation("binder-start", testOpStartBinder, transaction.VerbCreate),
			}, nil
		}))
	})
	ifaceMixedBinderMu.Lock()
	ifaceMixedBinderSeen = nil
	ifaceMixedBinderMu.Unlock()
	return func() []string {
		ifaceMixedBinderMu.Lock()
		defer ifaceMixedBinderMu.Unlock()
		return slices.Clone(ifaceMixedBinderSeen)
	}
}

// mixedBinderOperation is one half of the binder's stop and start. It declares
// the address it binds, which is the only thing that orders it against the
// move: the stop is a destroy that consumes the address, so it runs before the
// destroy that produces it, and the start is a create that consumes it, so it
// runs after the create that produces it.
func mixedBinderOperation(id string, label rpc.ConfigOperationType, verb transaction.OperationVerb) transaction.ConfigOperation {
	return transaction.ConfigOperation{
		ID:       id,
		Root:     ifaceMixedBinderRoot,
		Owner:    ifaceMixedBinderOwner,
		Type:     label,
		Verb:     verb,
		Target:   transaction.ResourceRef{Kind: transaction.ResourceListener, Address: ifaceMixedAddress, Port: 179},
		Produces: []transaction.ResourceRef{{Kind: transaction.ResourceListener, Address: ifaceMixedAddress, Port: 179}},
		Consumes: []transaction.ResourceRef{{Kind: transaction.ResourceAddress, Address: ifaceMixedAddress}},
	}
}

// reportAddressAdded stands in for the kernel monitor. The `interface` root
// makes an address create settle on the addr-added event its backend reports
// (the settlement rule in internal/component/iface/operation.go), and no
// backend runs here, so nothing would ever report the address and the apply
// would fail on a five second timeout.
//
// It reports on a ticker rather than in answer to the operation apply,
// because the per-plugin operation event types are interned when the
// transaction registers them, and a subscription taken before that keys on an
// id the emit never uses. A waiter armed at any point in the run catches the
// next tick.
func reportAddressAdded(t *testing.T, s *Server) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _ = s.EmitEngineEvent(ifaceevents.Namespace, ifaceEventAddrAdded, `{"address":"`+ifaceMixedAddress+`"}`) //nolint:errcheck // the settlement timeout is what reports a failure to emit
			}
		}
	}()
	t.Cleanup(func() { close(done) })
}

// mixedIfacePlugins returns the two participants: the real `interface` root,
// and the binder that binds the address it moves.
func mixedIfacePlugins(order *orderRecorder) []pluginDef {
	return []pluginDef{
		{
			name:  "interface",
			roots: []string{"interface"},
			order: order,
			configOps: []rpc.ConfigOperationDecl{{
				Root:      "interface",
				Decompose: true,
				Operations: []rpc.ConfigOperationType{
					ifaceOpAddInterface, ifaceOpRemoveInterface,
					ifaceOpAddAddress, ifaceOpRemoveAddress, ifaceOpConfigure,
				},
			}},
		},
		{
			name:  ifaceMixedBinderOwner,
			roots: []string{ifaceMixedBinderRoot},
			order: order,
			configOps: []rpc.ConfigOperationDecl{{
				Root:       ifaceMixedBinderRoot,
				Decompose:  true,
				Operations: []rpc.ConfigOperationType{testOpStopBinder, testOpStartBinder},
			}},
		},
	}
}

// TestReloadStopsABinderWhenTheCommitAlsoEditsAnMTU drives the defect from the
// reload entry point, over the real `interface` decomposer.
//
// One commit edits dum0's MTU and moves the address from dum0 to dum1. The
// decomposer used to answer NOTHING for that commit, because one key in the
// diff was one it had no primitive for, and it refused the whole root rather
// than part of it. The plan then carried no address destroy, so the core read
// no disturbance, no binder was asked to stop, and the coarse section apply
// moved the address while the session bound to it stayed up.
//
// VALIDATES: a mixed interface diff emits its address operations, the binder
// stops before the address leaves and starts after it arrives, and the MTU
// still reaches the component.
// PREVENTS: an undecomposable key in the diff taking the whole root's address
// operations with it, which reintroduces the failure phase 2 exists to
// prevent through a second door.
func TestReloadStopsABinderWhenTheCommitAlsoEditsAnMTU(t *testing.T) {
	seenDisturbed := registerMixedBinder(t)
	running, candidate := ifaceMixedTrees("1500", "9000", true)

	order := &orderRecorder{}
	reactor := &mockReloadReactor{tree: running}
	plugins := mixedIfacePlugins(order)
	s := newTestReloadServer(t, reactor, plugins)
	reportAddressAdded(t, s)

	require.NoError(t, s.ReloadConfig(context.Background(), candidate))
	require.Eventually(t, func() bool { return len(order.snapshot()) == 5 }, 2*time.Second, 10*time.Millisecond,
		"five operations apply: the stop, the two address halves, the start and the configure; got %v", order.snapshot())

	assert.Equal(t, []string{ifaceMixedAddress}, seenDisturbed(),
		"the binder is told the address this commit takes off the host, MTU edit or not")

	applied := order.snapshot()
	stop := slices.Index(applied, "binder-stop")
	start := slices.Index(applied, "binder-start")
	remove := slices.Index(applied, ifaceIDRemoveAddress)
	add := slices.Index(applied, ifaceIDAddAddress)
	configure := slices.Index(applied, ifaceIDConfigure)
	require.NotEqual(t, -1, stop, "the binder was asked for operations; got %v", applied)
	require.NotEqual(t, -1, configure, "the MTU reaches the component; got %v", applied)
	assert.Less(t, stop, remove, "the binder stops before its address leaves; got %v", applied)
	assert.Less(t, add, start, "the binder starts after the address arrives; got %v", applied)
	assert.Less(t, remove, configure, "the section is applied once every address is where the plan leaves it; got %v", applied)
	assert.Less(t, add, configure, "and once every address it adds is on the host; got %v", applied)
}

// TestReloadMovesTheAddressWithNoOtherInterfaceChange is the commit phase 2
// was built for, driven over the real decomposer rather than a stand-in.
//
// VALIDATES: an address-only interface commit still stops and starts the
// binder bound to it.
// PREVENTS: the repair for the mixed commit changing the case that already
// worked.
func TestReloadMovesTheAddressWithNoOtherInterfaceChange(t *testing.T) {
	seenDisturbed := registerMixedBinder(t)
	running, candidate := ifaceMixedTrees("1500", "1500", true)

	order := &orderRecorder{}
	reactor := &mockReloadReactor{tree: running}
	plugins := mixedIfacePlugins(order)
	s := newTestReloadServer(t, reactor, plugins)
	reportAddressAdded(t, s)

	require.NoError(t, s.ReloadConfig(context.Background(), candidate))
	require.Eventually(t, func() bool { return len(order.snapshot()) == 5 }, 2*time.Second, 10*time.Millisecond,
		"five operations apply; got %v", order.snapshot())

	assert.Equal(t, []string{ifaceMixedAddress}, seenDisturbed())

	applied := order.snapshot()
	assert.Less(t, slices.Index(applied, "binder-stop"), slices.Index(applied, ifaceIDRemoveAddress),
		"the binder stops before its address leaves; got %v", applied)
	assert.Less(t, slices.Index(applied, ifaceIDAddAddress), slices.Index(applied, "binder-start"),
		"the binder starts after the address arrives; got %v", applied)
}

// TestReloadDisturbsNothingWhenOnlyTheMTUChanges is the other half of the rule
// the mixed commit tests. An MTU edit moves no address, so nothing is
// disturbed and no binder is touched.
//
// VALIDATES: an MTU-only interface commit emits the configure operation alone,
// disturbs no address, and sends the binder nothing.
// PREVENTS: a decomposer that answers the address question for every diff
// bouncing a session that had no reason to restart.
func TestReloadDisturbsNothingWhenOnlyTheMTUChanges(t *testing.T) {
	seenDisturbed := registerMixedBinder(t)
	running, candidate := ifaceMixedTrees("1500", "9000", false)

	order := &orderRecorder{}
	reactor := &mockReloadReactor{tree: running}
	plugins := mixedIfacePlugins(order)
	s := newTestReloadServer(t, reactor, plugins)
	reportAddressAdded(t, s)

	require.NoError(t, s.ReloadConfig(context.Background(), candidate))
	require.Eventually(t, func() bool { return len(order.snapshot()) == 1 }, 2*time.Second, 10*time.Millisecond,
		"one operation applies, the configure; got %v", order.snapshot())

	assert.Equal(t, []string{ifaceIDConfigure}, order.snapshot(),
		"the MTU is applied by the operation that carries the section, and nothing else runs")
	assert.Empty(t, seenDisturbed(), "no address moved, so the binder answers for nothing")
	assert.Zero(t, plugins[1].responder.getOperationApplyCalls(), "the binder receives no operation")
	assert.Zero(t, plugins[1].responder.getApplyCalls(), "and no section apply either: it has no diff")
}
