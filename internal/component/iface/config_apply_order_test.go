// Design: docs/features/interfaces.md -- the first configured address is the primary.
// Related: config_apply.go -- desiredState, reconcileOnReadyWithJournal, addDesiredAddresses.
// Related: operation.go -- decomposeIfaceOperations.

package iface

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	tx "github.com/ze-software/ze/internal/component/config/transaction"
)

// orderedUnitAddrs is a unit's address leaf-list in the order the operator
// wrote it. It is neither sorted nor reverse sorted, so neither a lexical sort
// nor a map walk can produce it by accident: a map walk over eight entries
// matches one given order with a probability of about one in eight per run,
// and every test below runs its path orderRuns times.
var orderedUnitAddrs = []string{
	"192.168.50.1/24",
	"10.99.0.1/24",
	"172.16.5.1/24",
	"100.64.0.1/24",
	"198.51.100.1/24",
	"10.0.0.1/24",
	"203.0.113.1/24",
	"192.0.2.1/24",
}

const orderRuns = 20

func orderedDummyConfig() *ifaceConfig {
	return &ifaceConfig{
		Backend: "fake",
		Dummy: []ifaceEntry{{
			Name:  "dum0",
			Units: []unitEntry{{Label: "default", Addresses: orderedUnitAddrs}},
		}},
	}
}

// TestDesiredStateKeepsConfiguredAddressOrder checks that desiredState hands
// back each device's addresses in the order the config lists them, with a
// plugin-registered address after them and a duplicate counted once.
//
// VALIDATES: the desired address list is ordered: configured order first,
// registry addresses after, in a fixed order.
// PREVENTS: the primary address (the first one the kernel holds, which
// resolveSegment in the DHCP server and source-address selection read) being
// decided by map iteration.
func TestDesiredStateKeepsConfiguredAddressOrder(t *testing.T) {
	resetAddressOwners(t)
	require.NoError(t, RegisterOwnedAddresses("dum0", "test-owner", []string{"192.0.2.9/32", "10.99.0.1/24"}))

	cfg := orderedDummyConfig()
	for range orderRuns {
		addrs, _, _ := cfg.desiredState(nil)
		want := append(append([]string{}, orderedUnitAddrs...), "192.0.2.9/32")
		require.Equal(t, want, addrs["dum0"])
	}
}

// TestApplyInstallsAddressesInConfiguredOrder applies a config whose unit
// lists eight addresses to a recording backend and checks the exact sequence
// of AddAddress calls.
//
// VALIDATES: the reconcile installs a unit's addresses in configured order, so
// the first configured address is the interface's primary.
// PREVENTS: the home-lab defect where br0 came up with 10.99.0.1 before the
// configured first address 192.168.50.1, and the LAN client was leased from
// the wrong subnet. On the old code `for addr := range desired` walked a map,
// so the sequence differed from run to run.
func TestApplyInstallsAddressesInConfiguredOrder(t *testing.T) {
	for run := range orderRuns {
		fb := &fakeBackend{ifaces: map[string]fakeIface{}}
		errs := applyConfig(orderedDummyConfig(), nil, fb)
		require.Empty(t, errs)
		require.Equal(t, orderedUnitAddrs, fb.addrs["dum0"], fmt.Sprintf("run %d", run))
	}
}

// TestDeferredApplyInstallsAddressesInConfiguredOrder checks the additive
// fallback a not-ready backend takes (addDesiredAddresses) in the same way.
//
// VALIDATES: addresses installed before the full reconcile can run keep the
// configured order too.
// PREVENTS: a VPP start where the primary depends on map iteration.
func TestDeferredApplyInstallsAddressesInConfiguredOrder(t *testing.T) {
	clearOwedApply(t)
	for run := range orderRuns {
		fb := &fakeBackend{
			ifaces:  map[string]fakeIface{},
			listErr: fmt.Errorf("ifacevpp: VPP connector not ready: %w", ErrBackendNotReady),
		}
		errs := applyConfig(orderedDummyConfig(), nil, fb)
		require.Empty(t, errs)
		require.Equal(t, orderedUnitAddrs, fb.addrs["dum0"], fmt.Sprintf("run %d", run))
	}
}

// TestDecomposerEmitsAddressAddsInConfiguredOrder checks the transaction
// path: the add-address operations come out in configured order, which the
// solver keeps among operations of one phase (popLowestPhase).
//
// VALIDATES: an address added through a transaction lands in configured
// order.
// PREVENTS: the decomposer sorting the addresses lexically, which installs
// 10.99.0.1/24 before 192.168.50.1/24 on every run.
func TestDecomposerEmitsAddressAddsInConfiguredOrder(t *testing.T) {
	active := `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{}}}}}}`
	candidate := `{"interface":{"backend":"test","dummy":{"dum0":{"unit":{"default":{"ipv4":{"address":[` +
		`"192.168.50.1/24","10.99.0.1/24","172.16.5.1/24","100.64.0.1/24",` +
		`"198.51.100.1/24","10.0.0.1/24","203.0.113.1/24","192.0.2.1/24"]}}}}}}}`

	for run := range orderRuns {
		ops, err := decomposeIfaceOperations(context.Background(), tx.DecomposeRequest{
			TransactionID: "tx-iface-order",
			Root:          configRootInterface,
			ActiveRoot:    active,
			CandidateRoot: candidate,
			Diff: tx.DiffSection{
				Root:  configRootInterface,
				Added: `{"interface/dummy/dum0/unit/default/ipv4/address/0":"192.168.50.1/24"}`,
			},
		})
		require.NoError(t, err)
		var added []string
		for i := range ops {
			if ops[i].Type == operationAddAddress {
				added = append(added, ops[i].Params.CIDR)
			}
		}
		require.Equal(t, orderedUnitAddrs, added, fmt.Sprintf("run %d", run))
	}
}
