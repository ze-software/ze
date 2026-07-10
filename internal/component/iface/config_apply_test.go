package iface

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// VALIDATES: AC-1 -- a plugin-registered address for an interface with no
// matching YANG config appears in desiredState()'s output.
func TestDesiredState_IncludesRegisteredOwnerAddresses(t *testing.T) {
	resetAddressOwners(t)

	if err := RegisterOwnedAddresses("lo", "test-owner", []string{"192.175.48.1/32"}); err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}

	cfg := &ifaceConfig{}
	addrs, _, _ := cfg.desiredState()

	if !addrs["lo"]["192.175.48.1/32"] {
		t.Fatalf("desiredState() addrs[lo] = %v, want it to include the registered address", addrs["lo"])
	}
}

// VALIDATES: AC-2 -- unregistering a plugin's only claim on an address (not
// separately YANG-declared) drops it from desiredState()'s output.
func TestDesiredState_DropsAddressAfterUnregister(t *testing.T) {
	resetAddressOwners(t)

	if err := RegisterOwnedAddresses("lo", "test-owner", []string{"192.175.48.1/32"}); err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}
	UnregisterOwnedAddresses("test-owner")

	cfg := &ifaceConfig{}
	addrs, _, _ := cfg.desiredState()

	if addrs["lo"]["192.175.48.1/32"] {
		t.Fatalf("desiredState() addrs[lo] still contains the unregistered address: %v", addrs["lo"])
	}
}

// VALIDATES: AC-3 -- an address that is both YANG-declared and
// plugin-registered appears exactly once, and unregistering the plugin's
// claim alone does not remove it (still YANG-declared). Also covers R-2:
// no flap when only one source drops the address.
func TestDesiredState_YangAndRegistryOverlap(t *testing.T) {
	resetAddressOwners(t)

	const shared = "192.175.48.1/32"
	if err := RegisterOwnedAddresses("lo", "test-owner", []string{shared}); err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}

	cfg := &ifaceConfig{
		Loopback: &loopbackEntry{
			Units: []unitEntry{{Addresses: []string{shared}}},
		},
	}

	addrs, _, _ := cfg.desiredState()
	if got := len(addrs["lo"]); got != 1 {
		t.Fatalf("desiredState() addrs[lo] has %d entries, want exactly 1 (deduplicated): %v", got, addrs["lo"])
	}
	if !addrs["lo"][shared] {
		t.Fatalf("desiredState() addrs[lo] = %v, want it to include %q", addrs["lo"], shared)
	}

	// Plugin unregisters; YANG config still declares the address.
	UnregisterOwnedAddresses("test-owner")
	addrs, _, _ = cfg.desiredState()
	if !addrs["lo"][shared] {
		t.Fatalf("desiredState() dropped %q after unregister even though YANG config still declares it", shared)
	}
}

// VALIDATES: existing behavior is unchanged when the registry is empty --
// desiredState() output is a pure function of YANG config only.
func TestDesiredState_EmptyRegistryUnchanged(t *testing.T) {
	resetAddressOwners(t)

	cfg := &ifaceConfig{
		Loopback: &loopbackEntry{
			Units: []unitEntry{{Addresses: []string{"127.0.0.1/8"}}},
		},
	}

	addrs, managed, _ := cfg.desiredState()
	if len(addrs) != 1 || len(addrs["lo"]) != 1 || !addrs["lo"]["127.0.0.1/8"] {
		t.Fatalf("desiredState() with empty registry = %v, want only the YANG-declared address", addrs)
	}
	if len(managed) != 0 {
		t.Fatalf("desiredState() managed = %v, want empty (loopback is never in the managed set)", managed)
	}
}

// TestReconcileOnRegistryChange_AppliesAddressToBackend verifies AC-7 /
// finding B1 end-to-end: address_owner.go's reconcile-trigger, wired to
// reconcileOnRegistryChange the same way runEngine wires it, actually
// applies a registered address to the kernel backend with no config commit
// in between -- the third Wiring Test row.
//
// VALIDATES: AC-7 -- "the address is applied to ... the kernel backend
// within the same operation", not merely that the trigger closure fires.
// PREVENTS: a trigger that fires (mechanism) without the reconcile it
// triggers actually reaching AddAddress (behavior).
func TestReconcileOnRegistryChange_AppliesAddressToBackend(t *testing.T) {
	resetAddressOwners(t)
	fb := setupFakeBackendForTest(t)
	fb.ifaces["lo"] = fakeIface{name: "lo", linkType: "loopback"}

	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(&ifaceConfig{})
	setAddressOwnerReconcileTrigger(func() { reconcileOnRegistryChange(&activeCfg) })
	t.Cleanup(func() { setAddressOwnerReconcileTrigger(nil) })

	if err := RegisterOwnedAddresses("lo", "test-owner", []string{"192.175.48.1/32"}); err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}
	require.ElementsMatch(t, []string{"192.175.48.1/32"}, fb.addrs["lo"])

	UnregisterOwnedAddresses("test-owner")
	require.Empty(t, fb.addrs["lo"], "unregister must remove the address from the kernel backend")
}

// TestReconcileOnRegistryChange_RecordsOutcome verifies the fix for fork
// finding 2 (spec-as112 review): a registry-triggered reconcile pass records
// its outcome via RegistryReconcileStatus, both on a clean pass and on one
// that hits a kernel-apply error -- previously this trigger was fire-and-
// forget with only a log.Warn line, invisible to any caller.
//
// VALIDATES: RegistryReconcileStatus reflects the most recent
// reconcileOnRegistryChange pass, success or failure.
// PREVENTS: a stuck address-registration failure being visible only in logs.
func TestReconcileOnRegistryChange_RecordsOutcome(t *testing.T) {
	resetAddressOwners(t)
	fb := setupFakeBackendForTest(t)
	fb.ifaces["lo"] = fakeIface{name: "lo", linkType: "loopback"}

	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(&ifaceConfig{})
	setAddressOwnerReconcileTrigger(func() { reconcileOnRegistryChange(&activeCfg) })
	t.Cleanup(func() { setAddressOwnerReconcileTrigger(nil) })

	if err := RegisterOwnedAddresses("lo", "test-owner", []string{"192.175.48.1/32"}); err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}
	ok, at, errMsg := RegistryReconcileStatus()
	if !ok || errMsg != "" {
		t.Fatalf("RegistryReconcileStatus after clean pass = (%v, %v, %q), want (true, _, \"\")", ok, at, errMsg)
	}
	if at.IsZero() {
		t.Fatal("RegistryReconcileStatus at = zero time after a pass ran, want non-zero")
	}

	fb.addAddressErr = map[string]error{addressErrKey("lo", "2620:4f:8000::1/128"): errors.New("fake add-address failure")}
	if err := RegisterOwnedAddresses("lo", "test-owner", []string{"192.175.48.1/32", "2620:4f:8000::1/128"}); err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}
	ok, _, errMsg = RegistryReconcileStatus()
	if ok || errMsg == "" {
		t.Fatalf("RegistryReconcileStatus after failing pass = (%v, _, %q), want (false, _, non-empty)", ok, errMsg)
	}
}

// TestReconcileOnRegistryChange_NoOpWhenActiveCfgNil verifies the defensive
// path: a registry mutation before any config has been applied yet must not
// panic (mirrors TestReconcileOnVPPReady_NoOpWhenActiveCfgNil).
func TestReconcileOnRegistryChange_NoOpWhenActiveCfgNil(t *testing.T) {
	var activeCfg atomic.Pointer[ifaceConfig]
	reconcileOnRegistryChange(&activeCfg)
}

// TestReconcileOnReady_PreservesLoWhenNoLoopbackConfigAndNoRegistry guards
// against a regression caught during review: an earlier version of
// desiredState() unconditionally created an addrs["lo"] entry to make the
// registry-unregister-prunes-the-kernel path work (see Mistake Log), which
// had the side effect of treating "lo" as ze-managed even with zero YANG
// loopback config AND zero registry activity -- stripping the kernel's own
// auto-assigned 127.0.0.1/::1 on every reconcile. The fix (staleIfaces in
// address_owner.go) scopes "lo is reconcile-tracked" to interfaces the
// registry has actually, currently or recently, touched -- not
// unconditionally and not forever (see next test).
//
// VALIDATES: an interface with neither YANG config nor any registry
// activity is left alone by reconciliation -- ze must not manage addresses
// it was never told about.
// PREVENTS: stripping kernel-native loopback addresses on every commit.
func TestReconcileOnReady_PreservesLoWhenNoLoopbackConfigAndNoRegistry(t *testing.T) {
	resetAddressOwners(t)
	fb := setupFakeBackendForTest(t)
	fb.ifaces["lo"] = fakeIface{name: "lo", linkType: "loopback"}
	fb.addrs = map[string][]string{"lo": {"127.0.0.1/8", "::1/128"}}

	cfg := &ifaceConfig{} // no Loopback config at all, no registry activity

	errs, deferred := reconcileOnReady(cfg, fb)
	if len(errs) != 0 || deferred {
		t.Fatalf("reconcileOnReady() = (%v, %v), want (nil, false)", errs, deferred)
	}
	require.ElementsMatch(t, []string{"127.0.0.1/8", "::1/128"}, fb.addrs["lo"], "kernel-native lo addresses must survive when ze was never told to manage lo")
}

// TestReconcileOnReady_StopsTrackingInterfaceAfterCleanupReconcile guards
// against the second, more severe form of the same regression class,
// caught by independent review passes: a naive "remember every interface
// the registry ever touched" fix (an earlier version of staleIfaces) never
// forgot an interface once registered, so ANY plugin registering then
// fully unregistering on "lo" -- even once, e.g. an enable/disable cycle --
// would make every future, completely unrelated reconcile (a config commit
// touching a different interface, a vpp reconnect) strip lo's kernel-native
// addresses forever, for the remaining life of the process, EVEN THOUGH
// neither YANG config nor the registry claims "lo" anymore by that point.
//
// While the registry actively claims "lo" (steps 1-2 below), declarative
// reconciliation correctly strips anything not in the desired set -- the
// same behavior YANG-declared addresses already have (see
// TestDesiredState_EmptyRegistryUnchanged). The bug this test targets is
// specifically what happens AFTER the registry lets go: does "lo" revert to
// "not ze's business," or does it stay permanently managed?
//
// VALIDATES: once staleIfaces is cleared by a clean cleanup pass, a LATER,
// unrelated reconcile pass must not touch "lo" at all -- an address that
// reappears there afterward (kernel-native, or externally managed) must
// survive.
// PREVENTS: permanent, silent loopback address stripping after any
// one-time registry use.
func TestReconcileOnReady_StopsTrackingInterfaceAfterCleanupReconcile(t *testing.T) {
	resetAddressOwners(t)
	fb := setupFakeBackendForTest(t)
	fb.ifaces["lo"] = fakeIface{name: "lo", linkType: "loopback"}
	fb.addrs = map[string][]string{"lo": {"127.0.0.1/8"}}
	cfg := &ifaceConfig{} // no YANG loopback config throughout

	// 1. Register: "lo" becomes registry-managed. Declarative reconcile
	// correctly strips 127.0.0.1/8 since only the registered address is
	// desired -- expected, matches existing YANG-driven semantics, not the
	// regression under test.
	if err := RegisterOwnedAddresses("lo", "test-owner", []string{"192.175.48.1/32"}); err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}
	if _, deferred := reconcileOnReady(cfg, fb); deferred {
		t.Fatal("reconcile deferred unexpectedly")
	}
	require.ElementsMatch(t, []string{"192.175.48.1/32"}, fb.addrs["lo"], "only the registered address should be desired while it is the sole claimant of lo")

	// 2. Unregister: the cleanup reconcile prunes the registry's own
	// address and (on a clean pass) clears staleIfaces.
	UnregisterOwnedAddresses("test-owner")
	if errs, deferred := reconcileOnReady(cfg, fb); len(errs) != 0 || deferred {
		t.Fatalf("cleanup reconcile = (%v, %v), want (nil, false)", errs, deferred)
	}
	require.Empty(t, fb.addrs["lo"], "unregistered address must be pruned")

	// 3. Something outside ze's registry/YANG config adds an address back
	// to lo (kernel-native 127.0.0.1/8 reappearing, or any other
	// externally-managed address) -- represents the "cleanup already
	// happened, life goes on" state.
	fb.addrs["lo"] = []string{"127.0.0.1/8"}

	// 4. A LATER, completely unrelated reconcile (e.g. an unrelated config
	// commit) must leave it alone -- "lo" must no longer be forced into
	// desiredState() as an empty key now that staleIfaces was cleared.
	if errs, deferred := reconcileOnReady(cfg, fb); len(errs) != 0 || deferred {
		t.Fatalf("later unrelated reconcile = (%v, %v), want (nil, false)", errs, deferred)
	}
	require.ElementsMatch(t, []string{"127.0.0.1/8"}, fb.addrs["lo"], "a later unrelated reconcile must not strip an address that reappeared on lo after cleanup")
}

// TestApplyLoopbackLCPPairOnVPP verifies AC-6 wiring: applying a loopback under
// the vpp backend shadows it into Linux via SetupLCPPair, while the same config
// under a non-vpp backend does not (the LCP pair is a vpp-only concept).
// VALIDATES: AC-6 -- config -> Backend.SetupLCPPair on the vpp backend.
// PREVENTS: the LCP Backend method landing without a config-apply caller.
func TestApplyLoopbackLCPPairOnVPP(t *testing.T) {
	t.Run("vpp_backend_creates_pair", func(t *testing.T) {
		b := &fakeBackend{}
		cfg := &ifaceConfig{
			Backend: vppBackendName,
			Dummy:   []ifaceEntry{{Name: "loop0"}},
		}
		if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
			t.Fatalf("applyConfig: %v", errs)
		}
		if got := b.lcpPairs["loop0"]; got != "loop0" {
			t.Errorf("lcpPairs[loop0] = %q, want loop0", got)
		}
	})
	t.Run("netlink_backend_no_pair", func(t *testing.T) {
		b := &fakeBackend{}
		cfg := &ifaceConfig{
			Backend: "netlink",
			Dummy:   []ifaceEntry{{Name: "loop0"}},
		}
		if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
			t.Fatalf("applyConfig: %v", errs)
		}
		if _, ok := b.lcpPairs["loop0"]; ok {
			t.Error("netlink backend must not create an LCP pair")
		}
	})
}
