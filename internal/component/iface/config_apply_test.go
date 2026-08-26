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
	addrs, _, _ := cfg.desiredState(nil)

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
	addrs, _, _ := cfg.desiredState(nil)

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

	addrs, _, _ := cfg.desiredState(nil)
	if got := len(addrs["lo"]); got != 1 {
		t.Fatalf("desiredState() addrs[lo] has %d entries, want exactly 1 (deduplicated): %v", got, addrs["lo"])
	}
	if !addrs["lo"][shared] {
		t.Fatalf("desiredState() addrs[lo] = %v, want it to include %q", addrs["lo"], shared)
	}

	// Plugin unregisters; YANG config still declares the address.
	UnregisterOwnedAddresses("test-owner")
	addrs, _, _ = cfg.desiredState(nil)
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

	addrs, managed, _ := cfg.desiredState(nil)
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
	// The deferred vpp-ready / post-crash recreate path must also re-establish
	// the LCP shadow, matching applyConfig's Phase 1.
	t.Run("recreate_path_reshadows", func(t *testing.T) {
		b := &fakeBackend{}
		cfg := &ifaceConfig{
			Backend: vppBackendName,
			Dummy:   []ifaceEntry{{Name: "loop0"}},
		}
		if err := recreateManagedInterface(cfg, nil, "loop0", b); err != nil {
			t.Fatalf("recreateManagedInterface: %v", err)
		}
		if got := b.lcpPairs["loop0"]; got != "loop0" {
			t.Errorf("recreate lcpPairs[loop0] = %q, want loop0", got)
		}
	})
}

// VALIDATES: AC-1 (spec-iface-absent-link-graceful) -- a configured physical
// (Ethernet) interface whose link is absent is warned + skipped, and the present
// interface is still fully applied. The apply must NOT roll back everything just
// because one configured NIC is missing (portable image / unplugged cable).
func TestApplyConfigSkipsAbsentEthernet(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{ifaces: map[string]fakeIface{
		"eth-present": {name: "eth-present", linkType: zeTypeEthernet},
	}}
	cfg := &ifaceConfig{
		Ethernet: []ifaceEntry{
			{Name: "eth-present", MACAddress: "02:00:00:00:00:01"},
			{Name: "eth-absent", MACAddress: "02:00:00:00:00:02"},
		},
	}
	errs := applyConfig(cfg, nil, b)
	require.Empty(t, errs, "absent physical interface must be skipped, not abort the whole apply")
	require.Equal(t, "02:00:00:00:00:01", b.macSet["eth-present"], "present interface MAC must still be applied")
	_, absentApplied := b.macSet["eth-absent"]
	require.False(t, absentApplied, "absent interface must be skipped (no MAC applied)")
}

// VALIDATES: AC-2 -- a genuine (non-absent-link) backend error still aborts the
// apply and rolls back; the absent-interface skip must not swallow real errors.
func TestApplyConfigRollsBackGenuineError(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{
		ifaces:         map[string]fakeIface{},
		createDummyErr: map[string]error{"dum0": errors.New("boom")},
	}
	cfg := &ifaceConfig{
		Dummy: []ifaceEntry{{Name: "dum0"}},
	}
	errs := applyConfig(cfg, nil, b)
	require.NotEmpty(t, errs, "a genuine backend error must still abort and roll back")
}

// orderIndex returns the position of want in the backend's ordered call log,
// or -1 when absent.
func orderIndex(order []string, want string) int {
	for i, s := range order {
		if s == want {
			return i
		}
	}
	return -1
}

// TestReconcileCreatesOwnedMacvlanBeforeAddressAdd proves the device pass
// creates a plugin-owned macvlan BEFORE the address loop installs a VIP
// registered on that device, in one reconcile pass.
//
// VALIDATES: Wiring row 1 + AC-1/AC-2 ordering -- CreateMacvlanDevice precedes
// AddAddress so the VIP lands on an existing device (a missing device would
// fail "not found" and roll back the whole commit).
// PREVENTS: a VIP add on a not-yet-created macvlan aborting an unrelated commit.
func TestReconcileCreatesOwnedMacvlanBeforeAddressAdd(t *testing.T) {
	resetAddressOwners(t)
	resetDeviceOwners(t)
	fb := &fakeBackend{ifaces: map[string]fakeIface{"eth0": {name: "eth0", linkType: "ethernet", index: 2, mtu: 1500}}}

	if err := RegisterOwnedMacvlan("redund", MacvlanSpec{Name: "zv4-2-10", Parent: "eth0", MAC: "00:00:5e:00:01:0a"}); err != nil {
		t.Fatalf("register macvlan: %v", err)
	}
	if err := RegisterOwnedAddresses("zv4-2-10", "redund", []string{"192.0.2.1/32"}); err != nil {
		t.Fatalf("register address: %v", err)
	}

	errs, deferred := reconcileOnReady(&ifaceConfig{}, fb)
	if len(errs) != 0 || deferred {
		t.Fatalf("reconcileOnReady = (%v, %v), want (nil, false)", errs, deferred)
	}

	createIdx := orderIndex(fb.callOrder, "create-macvlan:zv4-2-10")
	addIdx := orderIndex(fb.callOrder, "add-address:zv4-2-10:192.0.2.1/32")
	if createIdx < 0 {
		t.Fatalf("CreateMacvlanDevice was not called; order=%v", fb.callOrder)
	}
	if addIdx < 0 {
		t.Fatalf("AddAddress on the macvlan was not called; order=%v", fb.callOrder)
	}
	if createIdx >= addIdx {
		t.Errorf("create (%d) must precede add (%d); order=%v", createIdx, addIdx, fb.callOrder)
	}
	if got := fb.macvlans["zv4-2-10"].Alias; got != "ze:owned:redund" {
		t.Errorf("macvlan alias = %q, want ze:owned:redund", got)
	}
}

// TestReconcileDeletesOrphanAliasedMacvlan proves the orphan scan deletes an
// aliased macvlan with no registration and leaves an UNaliased macvlan alone.
//
// VALIDATES: AC-4 + R-2 -- deletion requires BOTH kind macvlan AND the
// "ze:owned:" alias; an operator's unaliased macvlan is never touched.
// PREVENTS: leaking owned devices after a crash / owner release, and destroying
// an operator's own macvlan.
func TestReconcileDeletesOrphanAliasedMacvlan(t *testing.T) {
	resetAddressOwners(t)
	resetDeviceOwners(t)
	fb := &fakeBackend{ifaces: map[string]fakeIface{
		"zv4-9-1": {name: "zv4-9-1", linkType: zeTypeMacvlan, alias: "ze:owned:redund"},
		"opmv0":   {name: "opmv0", linkType: zeTypeMacvlan}, // operator device, no alias
	}}

	errs, deferred := reconcileOnReady(&ifaceConfig{}, fb)
	if len(errs) != 0 || deferred {
		t.Fatalf("reconcileOnReady = (%v, %v), want (nil, false)", errs, deferred)
	}
	if !fb.deleted["zv4-9-1"] {
		t.Error("orphan aliased macvlan should be deleted")
	}
	if fb.deleted["opmv0"] {
		t.Error("operator's unaliased macvlan must NOT be deleted")
	}
}

// TestReconcileReassertsDriftedMacvlan proves a drifted owned macvlan (MAC
// changed out of band) is deleted and recreated to spec in the same pass.
//
// VALIDATES: AC-10 -- MAC drift on a registered owned macvlan -> delete +
// recreate.
// PREVENTS: an owned device silently keeping an out-of-band MAC.
func TestReconcileReassertsDriftedMacvlan(t *testing.T) {
	resetAddressOwners(t)
	resetDeviceOwners(t)
	// Existing owned macvlan with the WRONG MAC; parent absent so only MAC +
	// alias are compared (parent/MTU drift guarded on parent resolvability).
	fb := &fakeBackend{ifaces: map[string]fakeIface{
		"zv4-2-10": {name: "zv4-2-10", linkType: zeTypeMacvlan, alias: "ze:owned:redund", mac: "00:00:5e:00:01:ff"},
	}}
	if err := RegisterOwnedMacvlan("redund", MacvlanSpec{Name: "zv4-2-10", Parent: "eth0", MAC: "00:00:5e:00:01:0a"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	errs, deferred := reconcileOnReady(&ifaceConfig{}, fb)
	if len(errs) != 0 || deferred {
		t.Fatalf("reconcileOnReady = (%v, %v), want (nil, false)", errs, deferred)
	}
	if !fb.deleted["zv4-2-10"] {
		t.Error("drifted macvlan should be deleted")
	}
	if got := fb.macvlans["zv4-2-10"].MAC; got != "00:00:5e:00:01:0a" {
		t.Errorf("recreated macvlan MAC = %q, want the spec MAC 00:00:5e:00:01:0a", got)
	}
}

// TestReconcileAdoptsUnmarkedRegisteredMacvlan proves that a macvlan holding a
// REGISTERED name but missing the ownership alias (the crash window between
// LinkAdd and LinkSetAlias -- the kernel ignores IFLA_IFALIAS at create, A-2
// fallback) is adopted: deleted and recreated to spec with the alias set,
// instead of failing closed forever.
//
// VALIDATES: A-2 fallback ("orphan scan additionally tolerates a missing alias
// on an exactly-registered name (adopt + re-mark)").
// PREVENTS: a crash between create and alias-set permanently blocking the
// registration with a name-conflict error.
func TestReconcileAdoptsUnmarkedRegisteredMacvlan(t *testing.T) {
	resetAddressOwners(t)
	resetDeviceOwners(t)
	fb := &fakeBackend{ifaces: map[string]fakeIface{
		// Correct spec but NO alias: exactly what a crash between LinkAdd and
		// LinkSetAlias leaves behind.
		"zv4-2-10": {name: "zv4-2-10", linkType: zeTypeMacvlan, mac: "00:00:5e:00:01:0a"},
	}}
	if err := RegisterOwnedMacvlan("redund", MacvlanSpec{Name: "zv4-2-10", Parent: "eth0", MAC: "00:00:5e:00:01:0a"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	errs, deferred := reconcileOnReady(&ifaceConfig{}, fb)
	if len(errs) != 0 || deferred {
		t.Fatalf("reconcileOnReady = (%v, %v), want (nil, false) -- must adopt, not fail closed", errs, deferred)
	}
	if !fb.deleted["zv4-2-10"] {
		t.Error("unmarked registered macvlan should be deleted for re-mark")
	}
	if got := fb.macvlans["zv4-2-10"].Alias; got != "ze:owned:redund" {
		t.Errorf("recreated macvlan alias = %q, want ze:owned:redund", got)
	}
}

// TestReconcileFailsClosedOnForeignKindHoldingRegisteredName proves a
// NON-macvlan device occupying a registered name is never deleted: the pass
// records an error and aborts (fail closed, R-2).
func TestReconcileFailsClosedOnForeignKindHoldingRegisteredName(t *testing.T) {
	resetAddressOwners(t)
	resetDeviceOwners(t)
	fb := &fakeBackend{ifaces: map[string]fakeIface{
		"zv4-2-10": {name: "zv4-2-10", linkType: "dummy"},
	}}
	if err := RegisterOwnedMacvlan("redund", MacvlanSpec{Name: "zv4-2-10", Parent: "eth0", MAC: "00:00:5e:00:01:0a"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	errs, _ := reconcileOnReady(&ifaceConfig{}, fb)
	if len(errs) == 0 {
		t.Fatal("pass must record an error for a foreign kind holding a registered name")
	}
	if fb.deleted["zv4-2-10"] {
		t.Error("a non-macvlan device must NEVER be deleted by the owned-device pass")
	}
}

// TestSharedTriggerServesBothRegistries proves one reconcile trigger wiring
// serves BOTH the owned-device and the owned-address registry: a device
// mutation and an address mutation each provoke a pass whose effects the fake
// backend observes.
//
// VALIDATES: A-6 -- shared registryReconcileCh worker reconciles both
// registries from snapshots; no second channel needed.
// PREVENTS: a registry mutation that never reaches the kernel because only the
// other registry's trigger was wired.
func TestSharedTriggerServesBothRegistries(t *testing.T) {
	resetAddressOwners(t)
	resetDeviceOwners(t)
	fb := setupFakeBackendForTest(t)
	fb.ifaces["eth0"] = fakeIface{name: "eth0", linkType: "ethernet", index: 2, mtu: 1500}

	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(&ifaceConfig{})
	shared := func() { reconcileOnRegistryChange(&activeCfg) }
	setAddressOwnerReconcileTrigger(shared)
	setDeviceOwnerReconcileTrigger(shared)
	t.Cleanup(func() {
		setAddressOwnerReconcileTrigger(nil)
		setDeviceOwnerReconcileTrigger(nil)
	})

	// Device mutation via the shared trigger -> device created.
	if err := RegisterOwnedMacvlan("redund", MacvlanSpec{Name: "zv4-2-10", Parent: "eth0", MAC: "00:00:5e:00:01:0a"}); err != nil {
		t.Fatalf("register macvlan: %v", err)
	}
	if _, ok := fb.macvlans["zv4-2-10"]; !ok {
		t.Error("device mutation through the shared trigger did not create the macvlan")
	}

	// Address mutation via the shared trigger -> address installed on eth0.
	if err := RegisterOwnedAddresses("eth0", "redund", []string{"192.0.2.9/32"}); err != nil {
		t.Fatalf("register address: %v", err)
	}
	found := false
	for _, a := range fb.addrs["eth0"] {
		if a == "192.0.2.9/32" {
			found = true
		}
	}
	if !found {
		t.Errorf("address mutation through the shared trigger did not install the address; addrs=%v", fb.addrs["eth0"])
	}
}

// TestOwnedMacvlanMatchesSpec_ModeDrift proves the reconcile drift check treats
// a macvlan in the wrong delivery mode as drift (so VRRP's private-mode device
// is re-created if found as bridge), while an unknown/absent mode is tolerated.
func TestOwnedMacvlanMatchesSpec_ModeDrift(t *testing.T) {
	desired := MacvlanSpec{Name: "zv4-2-10", Parent: "eth0", MAC: "00:00:5e:00:01:0a", Mode: MacvlanModePrivate, Alias: "ze:owned:vrrp"}
	base := InterfaceInfo{MAC: "00:00:5e:00:01:0a", Alias: "ze:owned:vrrp"}

	priv := base
	priv.MacvlanMode = "private"
	if !ownedMacvlanMatchesSpec(priv, desired, nil) {
		t.Error("private device matching a private spec should NOT be drift")
	}

	bridge := base
	bridge.MacvlanMode = "bridge"
	if ownedMacvlanMatchesSpec(bridge, desired, nil) {
		t.Error("bridge device for a private spec MUST be drift (re-create)")
	}

	unknown := base // MacvlanMode == "" (backend did not report it)
	if !ownedMacvlanMatchesSpec(unknown, desired, nil) {
		t.Error("unknown mode must be tolerated, not force a needless re-create")
	}
}

// tunnelTestConfig returns a one-tunnel GRE config carrying one address, so an
// apply exercises the create step and the address step over the same netdev.
func tunnelTestConfig(remote string) *ifaceConfig {
	return tunnelTestConfigOfKind(TunnelKindGRE, remote)
}

// tunnelTestConfigOfKind is tunnelTestConfig with the encapsulation kind under
// the caller's control, for the cases where the kind of the device holding the
// name is what the assertion is about.
func tunnelTestConfigOfKind(kind TunnelKind, remote string) *ifaceConfig {
	return &ifaceConfig{
		Backend: "fake",
		Tunnel: []tunnelEntry{
			{
				Name:  "tgre0",
				Units: []unitEntry{{Label: "default", Addresses: []string{"10.0.0.1/30"}}},
				Spec: TunnelSpec{
					Kind:          kind,
					Name:          "tgre0",
					LocalAddress:  "192.0.2.1",
					RemoteAddress: remote,
				},
			},
		},
	}
}

// TestApplyTunnelKeepsExistingNetdevWhenPreviousSpecIsLost proves an apply that
// holds no previous spec for a tunnel succeeds when a tunnel of the configured
// kind already holds the name, and leaves that netdev alone.
//
// The case is a ze RESTART. previous is nil at every plugin start, nothing in
// this package deletes tunnels when the daemon stops, and netlink.LinkAdd sends
// NLM_F_CREATE|NLM_F_EXCL (vendor/github.com/vishvananda/netlink/link_linux.go),
// so the second start meets EEXIST over ze's own netdevs. Before this branch
// that failed the whole interface apply, and the fail-closed startup cascade
// then refused to start the daemon.
//
// It is NOT the QEMU failure first recorded here. That one was a collision
// between two tests: test/plugin/iface-tunnel-kinds.ci used the endpoint pairs
// of test/reload/tx-iface-tunnel-create.ci, ran first in the shared VM,
// and left its links behind. The kernel answers EEXIST on a duplicate
// local/remote pair whatever the device is named, so moving the endpoints made
// all three tests pass with no product change.
//
// VALIDATES: a second plugin start over ze's own leftover tunnel netdevs
// applies cleanly.
// PREVENTS: a restart failing the whole interface apply on EEXIST, and a
// delete-then-create "fix" breaking the traffic crossing an unchanged tunnel.
func TestApplyTunnelKeepsExistingNetdevWhenPreviousSpecIsLost(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	require.Empty(t, applyConfig(tunnelTestConfig("198.51.100.1"), nil, b), "first apply must create the tunnel")
	require.Contains(t, b.tunnels, "tgre0", "first apply must reach the backend")

	// Second apply with no previous view: what OnConfigure does at start, and
	// what the reload did in QEMU after the plugin had just created the links.
	errs := applyConfig(tunnelTestConfig("198.51.100.1"), nil, b)
	require.Empty(t, errs, "an apply meeting a netdev ze already created must succeed")
	require.Contains(t, b.ifaces, "tgre0", "the netdev must survive the second apply")
	require.False(t, b.deleted["tgre0"], "an unchanged tunnel must not be deleted and rebuilt")
	require.Equal(t, "198.51.100.1", b.tunnels["tgre0"].RemoteAddress, "the kept netdev must not be re-created")
}

// TestApplyTunnelChangedSpecStillReachesTheKernel proves the kept-netdev branch
// above does not swallow an operator edit: a tunnel whose Spec changed is
// deleted and re-created, so the new encapsulation reaches the kernel.
//
// VALIDATES: a reload is idempotent for an unchanged tunnel WITHOUT becoming
// blind to a changed one.
// PREVENTS: "create failed, a link exists, carry on" silently keeping a netdev
// built from the previous config.
func TestApplyTunnelChangedSpecStillReachesTheKernel(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	first := tunnelTestConfig("198.51.100.1")
	require.Empty(t, applyConfig(first, nil, b), "first apply must create the tunnel")

	errs := applyConfig(tunnelTestConfig("198.51.100.9"), first, b)
	require.Empty(t, errs, "a changed tunnel must apply cleanly")
	require.True(t, b.deleted["tgre0"], "a changed tunnel must be deleted before it is re-created")
	require.Equal(t, "198.51.100.9", b.tunnels["tgre0"].RemoteAddress, "the edited remote must reach the backend")
}

// TestApplyTunnelRollbackDoesNotDeleteAKeptNetdev proves the rollback of a
// later failing step never deletes a netdev this pass did not create.
//
// The create step's rollback deletes the link it made. A pass that KEPT an
// existing tunnel made nothing, so an unrelated failure downstream (here an
// address add) must leave the operator's tunnel standing.
//
// VALIDATES: the kept-netdev branch leaves `created` false.
// PREVENTS: a failed reload destroying a tunnel that predates it.
func TestApplyTunnelRollbackDoesNotDeleteAKeptNetdev(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{
		ifaces:        map[string]fakeIface{"tgre0": {name: "tgre0", linkType: "gre"}},
		addAddressErr: map[string]error{addressErrKey("tgre0", "10.0.0.1/30"): errors.New("boom")},
	}

	errs := applyConfig(tunnelTestConfig("198.51.100.1"), nil, b)
	require.NotEmpty(t, errs, "the address failure must abort the apply")
	require.False(t, b.deleted["tgre0"], "rollback must not delete a netdev this pass did not create")
	require.Contains(t, b.ifaces, "tgre0", "the pre-existing tunnel must survive the rollback")
}

// TestEveryTunnelKindHasAKernelLinkType proves the guard above can identify a
// device of every kind Ze models.
//
// kernelLinkType returning false makes the apply fail closed, which is the safe
// direction but the wrong answer for a kind Ze can create: a restart would
// refuse a tunnel it made itself. A kind added to tunnelKindNames with no entry
// in kernelLinkTypes fails here rather than in the field.
func TestEveryTunnelKindHasAKernelLinkType(t *testing.T) {
	for kind, name := range tunnelKindNames {
		linkType, known := kind.kernelLinkType()
		require.True(t, known, "tunnel kind %q has no entry in kernelLinkTypes", name)
		require.Equal(t, fakeKernelLinkTypes[kind], linkType,
			"tunnel kind %q must map to the link type the kernel reports", name)
	}
}

// TestApplyTunnelFailsWhenAnotherDeviceKindHoldsTheName proves the kept-netdev
// branch reads the kind of the device it found and fails the apply when that
// device is not a tunnel at all.
//
// EEXIST says a name is taken; it does not say by what. A dummy, a bridge or a
// physical NIC carrying the configured name is not this tunnel, and keeping it
// hands Phase 2 its MTU, Phase 2c its admin state and Phase 3 its addresses.
// reconcileOwnedDevices fails closed on the same state ("name occupied by a
// non-owned %s device"), and this step now matches it.
//
// VALIDATES: the guard is on the device KIND, not on the name being taken.
// PREVENTS: an operator's dummy0 silently becoming the carrier of a tunnel's
// addresses because the two share a name.
func TestApplyTunnelFailsWhenAnotherDeviceKindHoldsTheName(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{ifaces: map[string]fakeIface{"tgre0": {name: "tgre0", linkType: "dummy"}}}

	errs := applyConfig(tunnelTestConfig("198.51.100.1"), nil, b)
	require.NotEmpty(t, errs, "a foreign device holding the name must fail the apply")
	require.ErrorContains(t, errors.Join(errs...), `"tgre0" is held by a device of type dummy, not by a gre tunnel`,
		"the error must name what holds it and what was wanted")
	require.False(t, b.deleted["tgre0"], "a device ze did not create must never be deleted")
	require.Empty(t, b.addrs["tgre0"], "the tunnel's addresses must not reach a foreign device")
}

// TestApplyTunnelFailsWhenTheNameHoldsAnotherTunnelKind proves the same guard
// separates one encapsulation from another.
//
// This is the operator who edits `encapsulation` while ze is down. previous is
// nil at plugin start, so the changed Spec never reaches the delete-then-create
// branch: it reaches the create, gets EEXIST, and finds an ipip link under a
// name the config now calls gre. That config failed loudly before the
// kept-netdev branch existed and it still does, rather than starting the daemon
// on the old encapsulation with the new addresses.
//
// VALIDATES: kernelLinkTypes is compared against the read-back, not ignored.
// PREVENTS: an encapsulation edit applied while ze was down being silently
// dropped, with a WARN as the only trace.
func TestApplyTunnelFailsWhenTheNameHoldsAnotherTunnelKind(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{ifaces: map[string]fakeIface{"tgre0": {name: "tgre0", linkType: "ipip"}}}

	errs := applyConfig(tunnelTestConfigOfKind(TunnelKindGRE, "198.51.100.1"), nil, b)
	require.NotEmpty(t, errs, "a tunnel of another kind under the name must fail the apply")
	require.ErrorContains(t, errors.Join(errs...), `"tgre0" is held by a device of type ipip, not by a gre tunnel`,
		"the error must name both encapsulations")
	require.False(t, b.deleted["tgre0"], "the existing tunnel must not be deleted on the failing path")
}

// TestApplyTunnelReportsTheCreateErrorWhenTheNameIsFree proves the deny path:
// when the create fails and NOTHING holds the name, the apply reports the
// kernel's own error.
//
// This is the real-world failure the kept-netdev branch must not swallow: a
// kernel with no driver for the kind answers `operation not supported` and
// leaves no netdev, which is exactly the state this spec's kernel work was
// written to remove. The branch is reached only through a successful read-back,
// so a free name must fall through to the error.
//
// VALIDATES: the branch keys on a device being found, never on the create
// having failed.
// PREVENTS: a create refused by the kernel being reported as success, which
// would leave the daemon running with a tunnel that does not exist.
func TestApplyTunnelReportsTheCreateErrorWhenTheNameIsFree(t *testing.T) {
	resetAddressOwners(t)
	b := &fakeBackend{
		ifaces:          map[string]fakeIface{},
		createTunnelErr: map[string]error{"tgre0": errors.New("operation not supported")},
	}

	errs := applyConfig(tunnelTestConfig("198.51.100.1"), nil, b)
	require.NotEmpty(t, errs, "a refused create must fail the apply")
	require.ErrorContains(t, errors.Join(errs...), "operation not supported",
		"the kernel's own error must survive to the caller")
	require.NotContains(t, errors.Join(errs...).Error(), "is held by",
		"a free name is not a name conflict")
	require.NotContains(t, b.ifaces, "tgre0", "no netdev may be reported for a refused create")
}
