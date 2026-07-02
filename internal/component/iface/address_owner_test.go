package iface

import (
	"strings"
	"sync"
	"testing"
)

// resetAddressOwners clears package-level registry state between tests so
// they don't leak into each other (address_owner.go's map/trigger are
// package-level, mirroring backend.go's backends map).
func resetAddressOwners(t *testing.T) {
	t.Helper()
	reset := func() {
		addressOwnerMu.Lock()
		addressOwners = map[string]map[string]map[string]bool{}
		staleIfaces = map[string]bool{}
		addressOwnerTrigger = nil
		addressOwnerMu.Unlock()
		lastRegistryReconcile.Store(nil)
	}
	reset()
	t.Cleanup(reset)
}

// VALIDATES: registration appears in the registry, readable via ownedAddresses().
func TestRegisterOwnedAddresses_Basic(t *testing.T) {
	resetAddressOwners(t)

	if err := RegisterOwnedAddresses("lo", "test-owner", []string{"192.175.48.1/32", "192.31.196.1/32"}); err != nil {
		t.Fatalf("RegisterOwnedAddresses: unexpected error: %v", err)
	}

	got, _ := ownedAddresses()
	want := map[string]bool{"192.175.48.1/32": true, "192.31.196.1/32": true}
	if len(got["lo"]) != len(want) {
		t.Fatalf("ownedAddresses()[lo] = %v, want %v", got["lo"], want)
	}
	for a := range want {
		if !got["lo"][a] {
			t.Errorf("ownedAddresses()[lo] missing %q", a)
		}
	}
}

// VALIDATES: AC-4 -- a second owner registering an address already owned by
// a different owner on the same interface is rejected, naming the
// conflicting owner, and the original registration is unchanged.
// PREVENTS: silent overwrite of one plugin's address by another's.
func TestRegisterOwnedAddresses_ConflictRejected(t *testing.T) {
	resetAddressOwners(t)

	if err := RegisterOwnedAddresses("lo", "owner-a", []string{"10.99.0.1/32"}); err != nil {
		t.Fatalf("first registration: unexpected error: %v", err)
	}

	err := RegisterOwnedAddresses("lo", "owner-b", []string{"10.99.0.1/32"})
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !containsAll(err.Error(), "10.99.0.1/32", "owner-a") {
		t.Errorf("error %q does not name the address and conflicting owner", err.Error())
	}

	got, _ := ownedAddresses()
	if !got["lo"]["10.99.0.1/32"] {
		t.Fatal("original registration was lost after rejected conflict")
	}
	if len(got) != 1 || len(got["lo"]) != 1 {
		t.Fatalf("owner-b's rejected registration leaked into ownedAddresses(): %v", got)
	}
}

// VALIDATES: AC-5 -- re-registering the identical address set for the same
// owner is a no-op: no error, no duplicate entries.
func TestRegisterOwnedAddresses_Idempotent(t *testing.T) {
	resetAddressOwners(t)

	addrs := []string{"192.175.48.1/32", "192.31.196.1/32"}
	if err := RegisterOwnedAddresses("lo", "test-owner", addrs); err != nil {
		t.Fatalf("first registration: unexpected error: %v", err)
	}
	if err := RegisterOwnedAddresses("lo", "test-owner", addrs); err != nil {
		t.Fatalf("re-registration: unexpected error: %v", err)
	}

	got, _ := ownedAddresses()
	if len(got["lo"]) != 2 {
		t.Fatalf("ownedAddresses()[lo] = %v, want exactly 2 entries (no duplicates)", got["lo"])
	}
}

// VALIDATES: AC-2 -- unregistering an owner removes its entries. The
// interface itself stays a key in ownedAddresses() (with an empty address
// set) after the last owner leaves, so reconcileOnReadyWithJournal's
// "remove extra addresses" pass keeps visiting it and prunes the kernel
// address instead of silently losing track of that interface (see the
// everOwnedIfaces doc comment and this spec's Mistake Log).
func TestUnregisterOwnedAddresses(t *testing.T) {
	resetAddressOwners(t)

	if err := RegisterOwnedAddresses("lo", "test-owner", []string{"192.175.48.1/32"}); err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}
	UnregisterOwnedAddresses("test-owner")

	got, _ := ownedAddresses()
	if len(got) != 1 || len(got["lo"]) != 0 {
		t.Fatalf("ownedAddresses() after unregister = %v, want {\"lo\": {}} (interface still tracked, no addresses)", got)
	}

	// Unregistering an owner with no registrations is a documented no-op.
	UnregisterOwnedAddresses("never-registered")
}

// VALIDATES: AC-6 -- concurrent register/unregister/read is race-free.
// Run with -race.
func TestAddressOwnerRegistry_Race(t *testing.T) {
	resetAddressOwners(t)

	var wg sync.WaitGroup
	for i := range 8 {
		owner := "owner-" + string(rune('a'+i))
		wg.Add(3)
		go func(owner string) {
			defer wg.Done()
			_ = RegisterOwnedAddresses("lo", owner, []string{"10.0.0.1/32"})
		}(owner)
		go func(owner string) {
			defer wg.Done()
			UnregisterOwnedAddresses(owner)
		}(owner)
		go func() {
			defer wg.Done()
			ownedAddresses()
		}()
	}
	wg.Wait()
}

// VALIDATES: AC-7 / finding B1 -- RegisterOwnedAddresses and
// UnregisterOwnedAddresses fire the reconcile-trigger synchronously with no
// following config commit.
// PREVENTS: an address registered mid-session sitting unapplied until an
// unrelated later commit re-runs reconciliation.
func TestRegister_TriggersReconcile(t *testing.T) {
	resetAddressOwners(t)

	var registerCalls, unregisterCalls int
	setAddressOwnerReconcileTrigger(func() { registerCalls++ })

	if err := RegisterOwnedAddresses("lo", "test-owner", []string{"192.175.48.1/32"}); err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}
	if registerCalls != 1 {
		t.Fatalf("trigger called %d times after Register, want 1", registerCalls)
	}

	setAddressOwnerReconcileTrigger(func() { unregisterCalls++ })
	UnregisterOwnedAddresses("test-owner")
	if unregisterCalls != 1 {
		t.Fatalf("trigger called %d times after Unregister, want 1", unregisterCalls)
	}

	// Unregistering an owner with nothing registered must NOT fire the trigger.
	setAddressOwnerReconcileTrigger(func() { t.Fatal("trigger fired for no-op unregister") })
	UnregisterOwnedAddresses("never-registered")
}

// TestClearStaleIfaces_OnlyClearsGivenNames is a deterministic regression
// test for a race an adversarial re-review found in the first staleIfaces
// design: reconcileOnReadyWithJournal takes an ownedAddresses() snapshot,
// then does real (slow) kernel I/O, then calls clearStaleIfaces with that
// SAME snapshot's names -- never a fresh read of the live map. A version
// that instead wiped the whole live staleIfaces map would silently discard
// an entry a concurrent UnregisterOwnedAddresses call (for a completely
// different interface) added after the snapshot was taken but before the
// clear ran, permanently leaking that interface's stale kernel address.
//
// VALIDATES: clearStaleIfaces(names) removes exactly `names`, leaving any
// OTHER entry currently in staleIfaces untouched.
// PREVENTS: a reconcile pass's cleanup silently discarding a different,
// concurrently-added pending cleanup it never processed.
func TestClearStaleIfaces_OnlyClearsGivenNames(t *testing.T) {
	resetAddressOwners(t)

	if err := RegisterOwnedAddresses("ifaceA", "owner-a", []string{"10.0.0.1/32"}); err != nil {
		t.Fatalf("register ifaceA: unexpected error: %v", err)
	}
	UnregisterOwnedAddresses("owner-a")

	// A reconcile pass for ifaceA takes its snapshot here.
	_, staleNames := ownedAddresses()
	if len(staleNames) != 1 || staleNames[0] != "ifaceA" {
		t.Fatalf("staleNames = %v, want [ifaceA]", staleNames)
	}

	// While that pass is still "in flight" (real reconciles do slow kernel
	// I/O between the snapshot and the clear), a concurrent unregister on a
	// DIFFERENT interface adds its own staleIfaces entry.
	if err := RegisterOwnedAddresses("ifaceB", "owner-b", []string{"10.0.0.2/32"}); err != nil {
		t.Fatalf("register ifaceB: unexpected error: %v", err)
	}
	UnregisterOwnedAddresses("owner-b")

	// The first pass finishes and clears only the names its own snapshot saw.
	clearStaleIfaces(staleNames)

	after, _ := ownedAddresses()
	if _, stillPending := after["ifaceB"]; !stillPending {
		t.Fatal("clearStaleIfaces wiped ifaceB, which was never in the snapshot it was given")
	}
	if _, stillPending := after["ifaceA"]; stillPending {
		t.Fatal("clearStaleIfaces did not clear ifaceA, which WAS in the snapshot it was given")
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
