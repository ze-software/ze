// Design: docs/features/interfaces.md -- generic plugin-owned address registry
// Related: backend.go -- RegisterBackend pattern this mirrors; config_apply.go -- desiredState() merge point

package iface

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// addressOwnerMu protects addressOwners, staleIfaces, and addressOwnerTrigger.
var addressOwnerMu sync.Mutex

// addressOwners maps owner name -> OS interface name -> set of CIDR
// addresses that owner has registered. Consulted by desiredState() so a
// plugin's addresses are treated as desired without the operator
// separately declaring them under the interface's YANG config. Empty by
// default: an interface with no registered owner behaves exactly as
// before this registry existed.
var addressOwners = map[string]map[string]map[string]bool{}

// staleIfaces tracks interface names whose last registered owner has just
// departed (UnregisterOwnedAddresses) but that still need one more
// reconcile pass to prune any kernel address the registry previously
// caused to be added. Without this, once the last owner leaves,
// ownedAddresses() would stop mentioning the interface at all, and
// reconcileOnReadyWithJournal's "remove extra addresses" pass only visits
// interfaces present in its desired-state map (config_apply.go) -- so a
// stale address on an interface with no YANG config of its own would never
// be pruned.
//
// This is NOT a permanent "ever owned" set: config_apply.go's
// clearStaleIfaces() forgets an entry once a full reconcile pass completes
// with zero errors, proof the pending cleanup was applied. An earlier
// version of this design tracked every interface ever registered forever,
// which permanently treated it as ze-managed and stripped kernel-native
// addresses (e.g. lo's 127.0.0.1) on every later, unrelated reconcile once
// any plugin had been enabled and disabled even once -- see this package's
// design spec history for the full incident writeup.
var staleIfaces = map[string]bool{}

// addressOwnerTrigger is invoked after every RegisterOwnedAddresses or
// UnregisterOwnedAddresses call that changes the registry's contents, so
// iface reconciliation re-runs promptly instead of waiting for an
// unrelated config commit (design finding B1). Wired once at plugin
// startup via setAddressOwnerReconcileTrigger; nil (a no-op) until then,
// which keeps the registry usable standalone in unit tests.
var addressOwnerTrigger func()

// setAddressOwnerReconcileTrigger wires the callback invoked whenever the
// registry's contents change. Called once from runEngine, reusing the same
// reconcile-on-ready shape as subscribeReconcileOnReady's vpp trigger.
// Passing nil detaches the trigger (e.g. on shutdown).
func setAddressOwnerReconcileTrigger(trigger func()) {
	addressOwnerMu.Lock()
	defer addressOwnerMu.Unlock()
	addressOwnerTrigger = trigger
}

// RegisterOwnedAddresses declares that owner owns addrs on ifaceName.
// desiredState() includes these addresses for ifaceName until a matching
// UnregisterOwnedAddresses(owner) call. Re-registering the same owner for
// the same interface replaces that owner's previous address set for that
// interface (idempotent when the set is unchanged; other interfaces or
// owners are untouched).
//
// Returns an error naming the conflicting owner if any address in addrs is
// already registered by a different owner on the same interface -- the
// original registration is left unchanged.
//
// addrs may be empty: the call still records owner against ifaceName, but
// since ifaceHasOwnerLocked and ownedAddresses only ever look at address
// COUNTS (len(...) > 0), an owner with zero addresses is indistinguishable
// from having no registration at all -- it contributes nothing to
// desiredState() and never blocks staleIfaces cleanup once every owner on
// ifaceName is empty or gone. No caller currently registers an empty set.
func RegisterOwnedAddresses(ifaceName, owner string, addrs []string) error {
	addressOwnerMu.Lock()

	for otherOwner, ifaces := range addressOwners {
		if otherOwner == owner {
			continue
		}
		existing := ifaces[ifaceName]
		for _, a := range addrs {
			if existing[a] {
				addressOwnerMu.Unlock()
				return fmt.Errorf("iface: address %q on %q already owned by %q", a, ifaceName, otherOwner)
			}
		}
	}

	set := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		set[a] = true
	}
	if addressOwners[owner] == nil {
		addressOwners[owner] = make(map[string]map[string]bool)
	}
	addressOwners[owner][ifaceName] = set
	// Actively owned again: no longer merely "pending cleanup" (harmless
	// no-op if it was never in staleIfaces).
	delete(staleIfaces, ifaceName)

	trigger := addressOwnerTrigger
	addressOwnerMu.Unlock()

	if trigger != nil {
		trigger()
	}
	return nil
}

// UnregisterOwnedAddresses removes every address owner previously
// registered on any interface. A no-op (including no reconcile-trigger
// call) if owner has no registrations. Any interface that no longer has
// any owner after this call is marked in staleIfaces so the next reconcile
// pass still prunes its kernel address (see staleIfaces doc comment).
func UnregisterOwnedAddresses(owner string) {
	addressOwnerMu.Lock()
	ifaces, existed := addressOwners[owner]
	delete(addressOwners, owner)
	for ifaceName := range ifaces {
		if !ifaceHasOwnerLocked(ifaceName) {
			staleIfaces[ifaceName] = true
		}
	}
	trigger := addressOwnerTrigger
	addressOwnerMu.Unlock()

	if existed && trigger != nil {
		trigger()
	}
}

// ifaceHasOwnerLocked reports whether any remaining owner in addressOwners
// still claims ifaceName. Caller must hold addressOwnerMu.
func ifaceHasOwnerLocked(ifaceName string) bool {
	for _, ifaces := range addressOwners {
		if len(ifaces[ifaceName]) > 0 {
			return true
		}
	}
	return false
}

// ownedAddresses returns a fresh snapshot -- OS interface name -> set of
// CIDR addresses -- merging every registered owner, plus an empty entry for
// every interface in staleIfaces (interfaces that just lost their last
// owner and still need a cleanup pass). Consulted by desiredState(); the
// returned map is a copy the caller is free to mutate.
//
// Also returns staleNames, the exact staleIfaces entries included in THIS
// snapshot. A caller that successfully reconciles using this snapshot must
// pass staleNames (not the live staleIfaces map) to clearStaleIfaces, so a
// concurrent UnregisterOwnedAddresses call that adds a DIFFERENT interface
// to staleIfaces after this snapshot was taken -- while a reconcile pass
// using an older snapshot is still in flight, since reconcileMu guards the
// whole pass but addressOwnerMu is only held for this brief snapshot -- is
// never silently discarded by that older pass's cleanup. Caught by a
// dedicated adversarial re-review of the first staleIfaces design; see
// Mistake Log.
func ownedAddresses() (result map[string]map[string]bool, staleNames []string) {
	addressOwnerMu.Lock()
	defer addressOwnerMu.Unlock()

	result = make(map[string]map[string]bool, len(staleIfaces))
	staleNames = make([]string, 0, len(staleIfaces))
	for ifaceName := range staleIfaces {
		result[ifaceName] = make(map[string]bool)
		staleNames = append(staleNames, ifaceName)
	}
	for _, ifaces := range addressOwners {
		for ifaceName, addrs := range ifaces {
			if result[ifaceName] == nil {
				result[ifaceName] = make(map[string]bool, len(addrs))
			}
			for a := range addrs {
				result[ifaceName][a] = true
			}
		}
	}
	return result, staleNames
}

// lastRegistryReconcile holds the outcome of the most recent
// reconcileOnRegistryChange pass (config_apply.go), so any registry caller
// can surface a stuck misconfiguration without tailing logs -- that trigger
// is fire-and-forget by design (RegisterOwnedAddresses/UnregisterOwnedAddresses
// only signal a channel; the actual kernel work runs later on a separate
// goroutine, per the reconcileMu doc comment in config_apply.go), so nothing
// else observes its outcome.
var lastRegistryReconcile atomic.Pointer[registryReconcileOutcome]

type registryReconcileOutcome struct {
	at  time.Time
	err string // empty on a clean pass
}

// recordRegistryReconcileOutcome stores the outcome of one
// reconcileOnRegistryChange pass. Called only from config_apply.go.
func recordRegistryReconcileOutcome(errs []error) {
	o := &registryReconcileOutcome{at: time.Now()}
	if len(errs) > 0 {
		o.err = errors.Join(errs...).Error()
	}
	lastRegistryReconcile.Store(o)
}

// RegistryReconcileStatus reports the outcome of the most recent
// registry-triggered reconcile pass. ok is true and at is the zero Time
// before any registry-triggered reconcile has ever run (nothing to report
// yet -- e.g. no plugin has called RegisterOwnedAddresses in this process).
// Reflects the registry as a whole, not any single owner: if multiple
// plugins register addresses, a failure from one shows here regardless of
// which owner's call triggered the pass that hit it.
func RegistryReconcileStatus() (ok bool, at time.Time, errSummary string) {
	o := lastRegistryReconcile.Load()
	if o == nil {
		return true, time.Time{}, ""
	}
	return o.err == "", o.at, o.err
}

// clearStaleIfaces forgets exactly the given interface names from
// staleIfaces -- NOT the whole map. Called by config_apply.go's
// reconcileOnReadyWithJournal, with the staleNames a specific reconcile
// pass's own ownedAddresses() snapshot returned, after that pass completes
// with zero errors: a clean pass over a desired-address map that included
// exactly these names as empty keys is proof any stale address on them was
// already pruned. Deleting only these specific names (not reassigning the
// whole map) leaves untouched any entry a concurrent
// UnregisterOwnedAddresses call added after this pass's snapshot was taken.
func clearStaleIfaces(names []string) {
	if len(names) == 0 {
		return
	}
	addressOwnerMu.Lock()
	defer addressOwnerMu.Unlock()
	for _, name := range names {
		delete(staleIfaces, name)
	}
}
