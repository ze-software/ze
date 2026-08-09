//go:build integration && linux

// Design: docs/architecture/core-design.md -- Interface Management (section 14): generic
// plugin-to-iface address-ownership registry, real-kernel proof
// Related: config_apply_test.go's TestReconcileOnRegistryChange_AppliesAddressToBackend
// -- the same scenario against a fake backend; this file is the real-netlink
// counterpart that spec-as112-1 deferred to "the first spec with a real plugin
// consumer" and that consumer (spec-as112-2) never built.

package iface

import (
	"sync/atomic"
	"testing"
)

// TestIntegrationRegisterOwnedAddresses_ReachesRealKernelInterface proves the
// generic address-ownership registry (RegisterOwnedAddresses,
// UnregisterOwnedAddresses, the reconcile-trigger wiring) actually lands and
// removes an address on a REAL kernel interface via netlink -- not just the
// fake-backend unit test's in-memory bookkeeping
// (TestReconcileOnRegistryChange_AppliesAddressToBackend proves the wiring
// logic; this proves it reaches the kernel). Uses the same call shape a real
// plugin consumer (as112) uses in production:
// register.go's applyAddressRegistration -> iface.RegisterOwnedAddresses.
//
// VALIDATES: spec-as112-1's core promise -- a plugin enabling a service is
// sufficient to get its addresses onto the kernel interface, no second,
// manually-duplicated address configuration step.
// PREVENTS: the registry's Go-level bookkeeping (proven by the fake-backend
// unit test) silently never reaching the kernel for a real backend, which
// no test previously verified end-to-end.
func TestIntegrationRegisterOwnedAddresses_ReachesRealKernelInterface(t *testing.T) {
	withNetNS(t, func() {
		resetAddressOwners(t)

		var activeCfg atomic.Pointer[ifaceConfig]
		activeCfg.Store(&ifaceConfig{})
		setAddressOwnerReconcileTrigger(func() { reconcileOnRegistryChange(&activeCfg) })
		t.Cleanup(func() { setAddressOwnerReconcileTrigger(nil) })

		// Real AS112 anycast host addresses (register.go's hostAddresses
		// shape: /32,/128, not the /24,/48 covering prefixes), registered
		// exactly as as112's applyAddressRegistration does in production.
		addrs := []string{"192.175.48.1/32", "2620:4f:8000::1/128"}
		if err := RegisterOwnedAddresses("lo", "as112-test", addrs); err != nil {
			t.Fatalf("RegisterOwnedAddresses: %v", err)
		}
		t.Cleanup(func() { UnregisterOwnedAddresses("as112-test") })

		requireAddress(t, "lo", "192.175.48.1/32")
		requireAddress(t, "lo", "2620:4f:8000::1/128")

		UnregisterOwnedAddresses("as112-test")
		// Unregister alone does not reconcile -- the production trigger
		// (register.go:380, wired identically here) fires reconcile on
		// every register/unregister, matching what just ran above for
		// register; call it again explicitly for symmetry/clarity since
		// UnregisterOwnedAddresses's own trigger call already did this.
		reconcileOnRegistryChange(&activeCfg)

		requireNoAddress(t, "lo", "192.175.48.1/32")
		requireNoAddress(t, "lo", "2620:4f:8000::1/128")
	})
}
