// Design: docs/features/interfaces.md -- plugin-owned devices (macvlan)
// Related: macvlan.go -- MacvlanSpec and ComposeOwnedDeviceName
// Related: address_owner.go -- the sibling plugin-owned ADDRESS registry
// Related: config_apply.go -- reconcileOwnedDevices consumes ownedMacvlans()
// Related: cmd/owned_devices.go -- the engine operations that check the grant

package iface

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
)

// ownedDeviceAliasPrefix marks a kernel link as owned by ze's owned-device
// registry. It is written as the link's IFLA_IFALIAS at create time, as
// "ze:owned:<owner>". The reconcile orphan scan deletes any macvlan carrying
// this prefix that has no matching registration -- covering owner release AND
// crash leftovers without in-memory history (the kernel remembers ownership,
// the process need not). This is the one structural difference from the
// address registry (address_owner.go), which must remember what it did
// (staleIfaces) because an address carries no owner.
const ownedDeviceAliasPrefix = "ze:owned:"

// deviceOwnerMu protects deviceOwners, deviceOwnerTrigger, and gaugeOwners.
var deviceOwnerMu sync.Mutex

// deviceOwners maps owner name -> device name -> desired MacvlanSpec. Device
// names are unique across owners (RegisterOwnedMacvlan's conflict check), so
// ownedMacvlans() can key by device name. Empty by default: a process with no
// registered owner has zero owned devices and the reconcile device pass is a
// no-op.
var deviceOwners = map[string]map[string]MacvlanSpec{}

// deviceOwnerTrigger is invoked after every registry mutation, wired once at
// startup to the SAME reconcile channel the address registry uses
// (setDeviceOwnerReconcileTrigger). nil (a no-op) until then, keeping the
// registry usable standalone in unit tests.
var deviceOwnerTrigger func()

// setDeviceOwnerReconcileTrigger wires the callback invoked whenever the
// owned-device registry changes. Wired beside setAddressOwnerReconcileTrigger
// to the same registryReconcileCh so one worker serves both registries.
// Passing nil detaches the trigger (e.g. on shutdown).
func setDeviceOwnerReconcileTrigger(trigger func()) {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	deviceOwnerTrigger = trigger
}

// RegisterOwnedMacvlan declares that owner wants a bridge-mode macvlan named
// spec.Name on spec.Parent carrying spec.MAC. The reconcile device pass
// creates it (marked "ze:owned:<owner>") BEFORE the address passes, so a VIP
// registered on spec.Name via RegisterOwnedAddresses lands on an existing
// device in the same pass. Re-registering the same owner+name replaces that
// owner's spec (idempotent for the desired state when unchanged; the pass
// re-asserts only on drift).
//
// Returns an error naming the conflicting owner if a DIFFERENT owner already
// registered spec.Name -- the original registration is left unchanged.
func RegisterOwnedMacvlan(owner string, spec MacvlanSpec) error {
	if owner == "" {
		return fmt.Errorf("iface: owned macvlan owner is empty")
	}
	if err := spec.validate(); err != nil {
		return err
	}

	deviceOwnerMu.Lock()
	for otherOwner, devices := range deviceOwners {
		if otherOwner == owner {
			continue
		}
		if _, exists := devices[spec.Name]; exists {
			deviceOwnerMu.Unlock()
			return fmt.Errorf("iface: macvlan device %q already owned by %q", spec.Name, otherOwner)
		}
	}
	if deviceOwners[owner] == nil {
		deviceOwners[owner] = make(map[string]MacvlanSpec)
	}
	deviceOwners[owner][spec.Name] = spec
	trigger := deviceOwnerTrigger
	deviceOwnerMu.Unlock()

	updateOwnedDeviceGauge()
	if trigger != nil {
		trigger()
	}
	return nil
}

// UnregisterOwnedMacvlan removes one device registration for owner. A no-op
// (no reconcile trigger) if that owner never registered the name. The next
// reconcile pass sees an aliased kernel macvlan with no registration and
// deletes it (which removes its addresses with it).
func UnregisterOwnedMacvlan(owner, name string) {
	deviceOwnerMu.Lock()
	devices, ok := deviceOwners[owner]
	_, existed := devices[name]
	if ok {
		delete(devices, name)
		if len(devices) == 0 {
			delete(deviceOwners, owner)
		}
	}
	trigger := deviceOwnerTrigger
	deviceOwnerMu.Unlock()

	if !existed {
		return
	}
	updateOwnedDeviceGauge()
	if trigger != nil {
		trigger()
	}
}

// ownedMacvlans returns a fresh snapshot merging every registered owner:
// specs maps device name -> desired MacvlanSpec, owners maps device name ->
// owner (for the "ze:owned:<owner>" alias). Consumed by the reconcile device
// pass. Device names are unique across owners (conflict check), so no key
// collides. The returned maps are copies the caller may mutate.
func ownedMacvlans() (specs map[string]MacvlanSpec, owners map[string]string) {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	specs = make(map[string]MacvlanSpec)
	owners = make(map[string]string)
	for owner, devices := range deviceOwners {
		for name, spec := range devices {
			specs[name] = spec
			owners[name] = owner
		}
	}
	return specs, owners
}

// isRegisteredMacvlanParent reports whether name is the parent of any
// currently-registered owned macvlan. The monitor wiring in register.go calls
// this so a parent appearing (interface/created) or coming up (interface/up)
// after a device was registered re-triggers the reconcile pass, creating the
// device with no further plugin calls (holo bug 12: fire-and-forget create is
// replaced by an event-driven retry).
func isRegisteredMacvlanParent(name string) bool {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	for _, devices := range deviceOwners {
		for _, spec := range devices {
			if spec.Parent == name {
				return true
			}
		}
	}
	return false
}

// gaugeOwners records the owner label values currently present in the
// ze_iface_owned_devices GaugeVec, so an owner that drops to zero devices has
// its series deleted (not left stale at its last value). Guarded by
// deviceOwnerMu.
var gaugeOwners = map[string]bool{}

// updateOwnedDeviceGauge refreshes ze_iface_owned_devices{owner} from the live
// registry: one gauge per owner set to that owner's device count, and any
// previously-present owner no longer registered has its series deleted. Called
// after every registry mutation and by the reconcile device pass. No-op until
// bindMetricsRegistry has installed the gauge.
func updateOwnedDeviceGauge() {
	m := ifaceMetricsPtr.Load()
	if m == nil || m.ownedDevices == nil {
		return
	}
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	seen := make(map[string]bool, len(deviceOwners))
	for owner, devices := range deviceOwners {
		m.ownedDevices.With(owner).Set(float64(len(devices)))
		seen[owner] = true
	}
	for owner := range gaugeOwners {
		if !seen[owner] {
			m.ownedDevices.Delete(owner)
		}
	}
	gaugeOwners = seen
}

// ownedDevicePrefixMax bounds a granted device name prefix. A kernel link name
// holds at most 15 bytes, so a prefix of 15 leaves no room for the device's
// own part.
const ownedDevicePrefixMax = 14

// deviceNameGrants maps an owner to the device name prefixes it granted itself
// from init(). The grants are filled from init() only, so every process of the
// ze binary holds the same set, a forked plugin's included, and a plugin built
// outside the binary holds none. Safe for concurrent use.
var deviceNameGrants = struct {
	mu     sync.RWMutex
	owners map[string][]string
}{
	owners: make(map[string][]string),
}

// MustGrantOwnedDeviceNames records that owner may ask the interface component,
// through its engine operations, for devices whose names start with one of
// prefixes. owner is the plugin's registry name, which is the authenticated
// name of its process. An empty owner, an empty or over-long prefix, and a
// prefix that overlaps one another owner holds are defects in the calling
// package, so it panics. MUST be called from init().
func MustGrantOwnedDeviceNames(owner string, prefixes ...string) {
	if owner == "" {
		panic("BUG: iface: owned device names granted to an empty owner")
	}
	if len(prefixes) == 0 {
		panic("BUG: iface: owner " + owner + " granted itself no owned device name prefix")
	}
	deviceNameGrants.mu.Lock()
	defer deviceNameGrants.mu.Unlock()
	for _, prefix := range prefixes {
		if prefix == "" {
			panic("BUG: iface: owner " + owner + " granted itself an empty owned device name prefix")
		}
		if len(prefix) > ownedDevicePrefixMax {
			panic("BUG: iface: owner " + owner + " granted itself owned device name prefix " + prefix + ", longer than a link name allows")
		}
		refuseOverlappingGrant(owner, prefix)
		deviceNameGrants.owners[owner] = append(deviceNameGrants.owners[owner], prefix)
	}
}

// refuseOverlappingGrant panics when another owner holds a prefix that starts
// prefix or that prefix starts: two such owners could ask for one device name.
// The caller MUST hold deviceNameGrants.mu.
func refuseOverlappingGrant(owner, prefix string) {
	for holder, held := range deviceNameGrants.owners {
		if holder == owner {
			continue
		}
		for _, other := range held {
			if strings.HasPrefix(prefix, other) {
				panic("BUG: iface: owner " + owner + " granted itself owned device name prefix " + prefix + ", inside prefix " + other + " that owner " + holder + " holds")
			}
			if strings.HasPrefix(other, prefix) {
				panic("BUG: iface: owner " + owner + " granted itself owned device name prefix " + prefix + ", which holds prefix " + other + " that owner " + holder + " holds")
			}
		}
	}
}

// RefuseUngrantedDeviceOwner answers an operator error when owner made no
// grant with MustGrantOwnedDeviceNames, and nil when it did. The engine
// operations call it with the authenticated process name before they change
// any state. Safe for concurrent use.
func RefuseUngrantedDeviceOwner(owner string) error {
	deviceNameGrants.mu.RLock()
	_, granted := deviceNameGrants.owners[owner]
	deviceNameGrants.mu.RUnlock()
	if granted {
		return nil
	}
	return opererr.New(
		"Plugin "+owner+" may not ask for owned interface devices",
		"Only a plugin compiled into this ze binary that granted itself device names may create bridges and links, so no outside process can place a device in any network namespace",
		"Run the plugin compiled into ze under its own name: set plugin external <name> run \"ze plugin <name>\"",
	)
}
