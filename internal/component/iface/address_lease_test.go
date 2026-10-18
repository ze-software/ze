// Design: docs/features/interfaces.md -- what the address reconcile removes.
// Related: address_owner.go -- RegisterLeasedAddresses; config_apply.go -- reconcileOnReadyWithJournal.

package iface

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReconcileSparesLeasedAddresses runs the reconcile over a unit holding a
// registered lease the kernel reports as permanent, and a registered lease
// that is no longer installed.
//
// VALIDATES: a leased address survives the reconcile whatever its lifetime,
// a leased address that is absent is not installed, and once the owner
// unregisters, the same address is removed like any address the config does
// not name.
// PREVENTS: a commit removing a DHCPv4 lease with an infinite lifetime (the
// kernel holds it permanent, as a static address), and the reconcile putting
// back, as a permanent address, a lease its client already removed.
func TestReconcileSparesLeasedAddresses(t *testing.T) {
	resetAddressOwners(t)
	const (
		infinite = "10.0.9.9/24"
		expired  = "10.0.8.8/24"
	)
	fb := &fakeBackend{
		ifaces: map[string]fakeIface{"dum0": {name: "dum0", linkType: "dummy"}},
		addrs:  map[string][]string{"dum0": {"10.0.0.1/24", "10.0.0.2/24", infinite}},
	}
	cfg := testConfigWithAddresses() // desires 10.0.0.1/24 + 10.0.0.2/24

	require.NoError(t, RegisterLeasedAddresses("dum0", "dhcp-test", []string{infinite, expired}))
	errs, deferred := reconcileOnReady(cfg, fb)
	require.False(t, deferred)
	require.Empty(t, errs)
	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24", infinite}, fb.addrs["dum0"],
		"the registered lease must survive and the absent one must not be installed")

	UnregisterLeasedAddresses("dhcp-test")
	errs, deferred = reconcileOnReady(cfg, fb)
	require.False(t, deferred)
	require.Empty(t, errs)
	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, fb.addrs["dum0"],
		"an unregistered, permanent, unnamed address is removed")
}

// TestRegisterLeasedAddresses_SeparateFromDesired checks the registry contract.
//
// VALIDATES: a leased address never enters the desired state, a lease
// registration fires no reconcile, an address has one owner across both
// registries, and an empty set forgets the interface.
// PREVENTS: the reconcile installing a lease, and two owners holding one
// address.
func TestRegisterLeasedAddresses_SeparateFromDesired(t *testing.T) {
	resetAddressOwners(t)
	fired := 0
	setAddressOwnerReconcileTrigger(func() { fired++ })

	require.NoError(t, RegisterLeasedAddresses("eth0", "dhcp", []string{"10.1.0.5/24"}))
	require.Zero(t, fired, "a lease changes nothing the reconcile installs")
	owned, _ := ownedAddresses()
	require.Empty(t, owned, "a leased address is not desired")
	require.Equal(t, map[string]map[string]bool{"eth0": {"10.1.0.5/24": true}}, leasedAddressSet())

	err := RegisterOwnedAddresses("eth0", "as112", []string{"10.1.0.5/24"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), `"dhcp"`), "error names the lease owner: %v", err)

	require.NoError(t, RegisterOwnedAddresses("eth0", "as112", []string{"10.1.0.6/24"}))
	err = RegisterLeasedAddresses("eth0", "dhcp", []string{"10.1.0.6/24"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), `"as112"`), "error names the owned-address owner: %v", err)
	require.Equal(t, map[string]map[string]bool{"eth0": {"10.1.0.5/24": true}}, leasedAddressSet(),
		"a refused registration leaves the previous one")

	require.NoError(t, RegisterLeasedAddresses("eth0", "dhcp", nil))
	require.Empty(t, leasedAddressSet())
}
